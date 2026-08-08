/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	labv1alpha1 "github.com/ngtukien/sandbox-operator/api/v1alpha1"
	"github.com/ngtukien/sandbox-operator/internal/utils"
)

const (
	SysboxRuntimeClass = "sysbox-runc"
)

// InstanceLabReconciler đóng vai trò là "Người vận hành máy ảo".
// Lắp ráp Pod (Sysbox runtime), chuẩn bị ổ cứng (PVC), cấu hình dịch vụ mạng và tổng hợp danh sách URL bảo mật HTTPS.
type InstanceLabReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ============================================================================
// KHAI BÁO RBAC MARKERS (Phân Quyền cho InstanceLab Controller)
// ============================================================================

// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs/finalizers,verbs=update
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// ============================================================================
// VÒNG LẶP ĐIỀU KHIỂN TRUNG TÂM (Reconcile Loop)
// ============================================================================

func (r *InstanceLabReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation for InstanceLab", "name", req.Name)

	// 1. TRÍCH XUẤT ĐỐI TƯỢNG INSTANCELAB
	instanceLab := &labv1alpha1.InstanceLab{}
	if err := r.Get(ctx, req.NamespacedName, instanceLab); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("InstanceLab resource not found, ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch InstanceLab")
		return ctrl.Result{}, err
	}

	// 2. QUY TẮC ƯU TIÊN FINALIZER (Finalizer-First Rule)
	if instanceLab.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(instanceLab, utils.InstanceLabFinalizer) {
		log.Info("Adding Finalizer to InstanceLab for safe cross-namespace resource cleanup")
		controllerutil.AddFinalizer(instanceLab, utils.InstanceLabFinalizer)
		if err := r.Update(ctx, instanceLab); err != nil {
			log.Error(err, "Failed to update InstanceLab with Finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 3. XỬ LÝ QUY TRÌNH TIÊU HUỶ VÀ GIẢI PHÓNG TÀI NGUYÊN (Cross-namespace Cleanup)
	if instanceLab.DeletionTimestamp != nil {
		return r.reconcileFinalizer(ctx, instanceLab)
	}

	// 4. XÁC THỰC CỤM CHA CLUSTERLAB VÀ THIẾT LẬП OWNER REFERENCE
	clusterLab, isReady, err := r.resolveParentClusterLab(ctx, instanceLab)
	if err != nil {
		log.Error(err, "Error querying parent ClusterLab", "clusterLab", instanceLab.Spec.ClusterLabRef)
		return ctrl.Result{}, err
	}
	if !isReady {
		log.Info("Parent ClusterLab is not running yet or awaiting initial reconciliation, requeuing", "clusterLab", instanceLab.Spec.ClusterLabRef)
		instanceLab.Status.Phase = "Pending"
		utils.SetCondition(&instanceLab.Status.Conditions, "WaitingForCluster", metav1.ConditionFalse, "ParentNotReady", fmt.Sprintf("Waiting for parent ClusterLab %s to enter Running phase", instanceLab.Spec.ClusterLabRef))
		_ = r.Status().Update(ctx, instanceLab)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	targetNamespace := clusterLab.Status.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = utils.GenerateTargetNamespace(clusterLab.Name)
	}

	// 5. KIẾN TẠO MÁY ẢO DOCKER-IN-DOCKER VỚI SYSBOX RUNTIME (Pod)
	pod, err := r.reconcileSysboxPod(ctx, instanceLab, targetNamespace)
	if err != nil {
		log.Error(err, "Failed to reconcile Sysbox Pod for InstanceLab")
		r.updateStatusFailed(ctx, instanceLab, "PodError", err.Error())
		return ctrl.Result{}, err
	}

	// 6. KIẾN TẠO DỊCH VỤ GOM LƯU LƯỢNG MẠNG (ClusterIP Service)
	svc, err := r.reconcileServices(ctx, instanceLab, targetNamespace)
	if err != nil {
		log.Error(err, "Failed to reconcile Service for InstanceLab")
		r.updateStatusFailed(ctx, instanceLab, "ServiceError", err.Error())
		return ctrl.Result{}, err
	}

	// 7. KIẾN TẠO CỔNG TRUY CẬP BẢO MẬT HTTPS (Ingress with TLS & Cert-Manager)
	ingress, err := r.reconcileIngress(ctx, instanceLab, clusterLab, targetNamespace, svc)
	if err != nil {
		log.Error(err, "Failed to reconcile Ingress for InstanceLab")
		r.updateStatusFailed(ctx, instanceLab, "IngressError", err.Error())
		return ctrl.Result{}, err
	}

	// 8. CẬP NHẬT TRẠNG THÁI GIAO DIỆN (UI Endpoints, Pod IP)
	if err := r.updateInstanceLabStatus(ctx, instanceLab, pod, svc, ingress); err != nil {
		log.Error(err, "Failed to update InstanceLab status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled InstanceLab", "name", instanceLab.Name, "phase", instanceLab.Status.Phase, "podIP", instanceLab.Status.PodIP)
	return ctrl.Result{}, nil
}

// ============================================================================
// HỆ CÁC HÀM XỬ LÝ QUY TRÌNH KIẾN TẠO VÀ THANH MỘ THAI SẢN
// ============================================================================

func (r *InstanceLabReconciler) resolveParentClusterLab(ctx context.Context, instanceLab *labv1alpha1.InstanceLab) (*labv1alpha1.ClusterLab, bool, error) {
	clusterLab := &labv1alpha1.ClusterLab{}
	err := r.Get(ctx, types.NamespacedName{Name: instanceLab.Spec.ClusterLabRef, Namespace: instanceLab.Namespace}, clusterLab)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if err := controllerutil.SetControllerReference(clusterLab, instanceLab, r.Scheme); err == nil {
		_ = r.Update(ctx, instanceLab)
	}

	isReady := strings.EqualFold(clusterLab.Status.Phase, "Running") || strings.EqualFold(clusterLab.Status.Phase, "Ready")
	return clusterLab, isReady, nil
}

func (r *InstanceLabReconciler) reconcileFinalizer(ctx context.Context, instanceLab *labv1alpha1.InstanceLab) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(instanceLab, utils.InstanceLabFinalizer) {
		return ctrl.Result{}, nil
	}

	log.Info("Executing Finalizer cross-namespace cleanup for InstanceLab", "instance", instanceLab.Name)

	clusterLab := &labv1alpha1.ClusterLab{}
	targetNs := ""
	if err := r.Get(ctx, types.NamespacedName{Name: instanceLab.Spec.ClusterLabRef, Namespace: instanceLab.Namespace}, clusterLab); err == nil {
		targetNs = clusterLab.Status.TargetNamespace
	}
	if targetNs == "" {
		targetNs = utils.GenerateTargetNamespace(instanceLab.Spec.ClusterLabRef)
	}

	matchLabels := client.MatchingLabels{utils.LabelInstanceLab: instanceLab.Name}

	_ = r.DeleteAllOf(ctx, &networkingv1.Ingress{}, client.InNamespace(targetNs), matchLabels)
	_ = r.DeleteAllOf(ctx, &corev1.Service{}, client.InNamespace(targetNs), matchLabels)
	_ = r.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(targetNs), matchLabels)
	// Không cần xóa PVC vì không còn dùng PVC

	log.Info("Successfully cleaned up all workloads in target namespace. Removing Finalizer", "name", instanceLab.Name)
	controllerutil.RemoveFinalizer(instanceLab, utils.InstanceLabFinalizer)
	if err := r.Update(ctx, instanceLab); err != nil {
		log.Error(err, "Failed to remove Finalizer from InstanceLab")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *InstanceLabReconciler) reconcileSysboxPod(ctx context.Context, instanceLab *labv1alpha1.InstanceLab, targetNs string) (*corev1.Pod, error) {
	log := logf.FromContext(ctx)
	podName := utils.SanitizeName(fmt.Sprintf("%s-sysbox", instanceLab.Name), 63)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: targetNs,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pod, func() error {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels[utils.LabelManagedBy] = utils.LabelValueManagedBy
		pod.Labels[utils.LabelClusterLab] = instanceLab.Spec.ClusterLabRef
		pod.Labels[utils.LabelInstanceLab] = instanceLab.Name
		pod.Labels["app"] = podName

		enableServiceLinks := false
		pod.Spec.EnableServiceLinks = &enableServiceLinks

		hostUsers := false
		pod.Spec.HostUsers = &hostUsers

		if pod.Spec.NodeSelector == nil {
			pod.Spec.NodeSelector = make(map[string]string)
		}
		pod.Spec.NodeSelector["sysbox-install"] = "yes"

		runtimeClass := SysboxRuntimeClass
		pod.Spec.RuntimeClassName = &runtimeClass
		pod.Spec.RestartPolicy = corev1.RestartPolicyNever
		if instanceLab.Spec.Hostname != "" {
			pod.Spec.Hostname = instanceLab.Spec.Hostname
		}
		if len(instanceLab.Spec.ImagePullSecrets) > 0 {
			pod.Spec.ImagePullSecrets = instanceLab.Spec.ImagePullSecrets
		}

		var volumes []corev1.Volume
		var mounts []corev1.VolumeMount

		// lxcfs proc mounts — báo cáo resource chính xác cho Sysbox containers
		hostPathTypeFile := corev1.HostPathFile
		lxcfsMounts := []struct {
			name      string
			mountPath string
			hostPath  string
		}{
			{"lxcfs-proc-cpuinfo", "/proc/cpuinfo", "/var/lib/lxcfs/proc/cpuinfo"},
			{"lxcfs-proc-meminfo", "/proc/meminfo", "/var/lib/lxcfs/proc/meminfo"},
			{"lxcfs-proc-diskstats", "/proc/diskstats", "/var/lib/lxcfs/proc/diskstats"},
			{"lxcfs-proc-swaps", "/proc/swaps", "/var/lib/lxcfs/proc/swaps"},
			{"lxcfs-proc-uptime", "/proc/uptime", "/var/lib/lxcfs/proc/uptime"},
		}
		for _, l := range lxcfsMounts {
			volumes = append(volumes, corev1.Volume{
				Name: l.name,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: l.hostPath,
						Type: &hostPathTypeFile,
					},
				},
			})
			mounts = append(mounts, corev1.VolumeMount{
				Name:      l.name,
				MountPath: l.mountPath,
			})
		}

		var containerPorts []corev1.ContainerPort
		for _, p := range instanceLab.Spec.Ports {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				Name:          utils.SanitizeName(p.Name, 15),
				ContainerPort: p.TargetPort,
				Protocol:      corev1.ProtocolTCP,
			})
		}

		image := instanceLab.Spec.Image
		if image == "" {
			image = "ubuntu:24.04"
		}

		container := corev1.Container{
			Name:            "main",
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    mounts,
			Ports:           containerPorts,
			Command:         []string{"/sbin/init"},
		}

		resList := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}
		if !instanceLab.Spec.Resources.CPU.Request.IsZero() {
			resList.Requests[corev1.ResourceCPU] = instanceLab.Spec.Resources.CPU.Request
		}
		if !instanceLab.Spec.Resources.CPU.Limit.IsZero() {
			resList.Limits[corev1.ResourceCPU] = instanceLab.Spec.Resources.CPU.Limit
		}
		if !instanceLab.Spec.Resources.Memory.Request.IsZero() {
			resList.Requests[corev1.ResourceMemory] = instanceLab.Spec.Resources.Memory.Request
		}
		if !instanceLab.Spec.Resources.Memory.Limit.IsZero() {
			resList.Limits[corev1.ResourceMemory] = instanceLab.Spec.Resources.Memory.Limit
		}
		if !instanceLab.Spec.Resources.Storage.Limit.IsZero() {
			resList.Limits[corev1.ResourceEphemeralStorage] = instanceLab.Spec.Resources.Storage.Limit
		}
		if len(resList.Requests) > 0 || len(resList.Limits) > 0 {
			container.Resources = resList
		}

		pod.Spec.Volumes = volumes
		pod.Spec.Containers = []corev1.Container{container}
		return nil
	})

	if err != nil {
		return nil, err
	}
	log.Info("Reconciled Sysbox Pod successfully", "podName", podName, "namespace", targetNs)
	return pod, nil
}

func (r *InstanceLabReconciler) reconcileServices(ctx context.Context, instanceLab *labv1alpha1.InstanceLab, targetNs string) (*corev1.Service, error) {
	log := logf.FromContext(ctx)
	if len(instanceLab.Spec.Ports) == 0 {
		return nil, nil
	}

	svcName := utils.SanitizeName(fmt.Sprintf("%s-svc", instanceLab.Name), 63)
	podName := utils.SanitizeName(fmt.Sprintf("%s-sysbox", instanceLab.Name), 63)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: targetNs,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if svc.Labels == nil {
			svc.Labels = make(map[string]string)
		}
		svc.Labels[utils.LabelManagedBy] = utils.LabelValueManagedBy
		svc.Labels[utils.LabelClusterLab] = instanceLab.Spec.ClusterLabRef
		svc.Labels[utils.LabelInstanceLab] = instanceLab.Name

		svc.Spec.Selector = map[string]string{"app": podName}
		svc.Spec.Type = corev1.ServiceTypeClusterIP

		var svcPorts []corev1.ServicePort
		for _, p := range instanceLab.Spec.Ports {
			svcPorts = append(svcPorts, corev1.ServicePort{
				Name:       utils.SanitizeName(p.Name, 15),
				Port:       p.Port,
				TargetPort: intstr.FromInt(int(p.TargetPort)),
				Protocol:   corev1.ProtocolTCP,
			})
		}
		svc.Spec.Ports = svcPorts
		return nil
	})

	if err != nil {
		return nil, err
	}
	log.Info("Reconciled Service successfully", "serviceName", svcName, "namespace", targetNs)
	return svc, nil
}

func (r *InstanceLabReconciler) reconcileIngress(ctx context.Context, instanceLab *labv1alpha1.InstanceLab, clusterLab *labv1alpha1.ClusterLab, targetNs string, svc *corev1.Service) (*networkingv1.Ingress, error) {
	log := logf.FromContext(ctx)
	if svc == nil {
		return nil, nil
	}

	var exposePorts []labv1alpha1.InstanceLabPort
	for _, p := range instanceLab.Spec.Ports {
		if p.Expose {
			exposePorts = append(exposePorts, p)
		}
	}
	if len(exposePorts) == 0 {
		return nil, nil
	}

	ingName := utils.SanitizeName(fmt.Sprintf("%s-ingress", instanceLab.Name), 63)
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingName,
			Namespace: targetNs,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ingress, func() error {
		if ingress.Labels == nil {
			ingress.Labels = make(map[string]string)
		}
		ingress.Labels[utils.LabelManagedBy] = utils.LabelValueManagedBy
		ingress.Labels[utils.LabelClusterLab] = instanceLab.Spec.ClusterLabRef
		ingress.Labels[utils.LabelInstanceLab] = instanceLab.Name

		if ingress.Annotations == nil {
			ingress.Annotations = make(map[string]string)
		}
		ingress.Annotations["cert-manager.io/cluster-issuer"] = "letsencrypt-prod"
		ingress.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		ingress.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = "1000m"
		ingress.Annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] = "3600"
		ingress.Annotations["nginx.ingress.kubernetes.io/proxy-send-timeout"] = "3600"

		ingClass := utils.GetIngressClassName()
		ingress.Spec.IngressClassName = &ingClass

		host := utils.GenerateIngressHost(instanceLab.Name, clusterLab.Name, "")

		secretName := utils.SanitizeName(fmt.Sprintf("%s-tls-secret", instanceLab.Name), 63)
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: secretName,
			},
		}

		var httpPaths []networkingv1.HTTPIngressPath
		pathType := networkingv1.PathTypePrefix

		for _, ep := range exposePorts {
			httpPaths = append(httpPaths, networkingv1.HTTPIngressPath{
				Path:     "/",
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: svc.Name,
						Port: networkingv1.ServiceBackendPort{
							Number: ep.Port,
						},
					},
				},
			})
			break
		}

		ingress.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: httpPaths,
					},
				},
			},
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	log.Info("Reconciled Ingress successfully with HTTPS/TLS Enabled", "ingressName", ingName, "host", utils.GenerateIngressHost(instanceLab.Name, clusterLab.Name, ""))
	return ingress, nil
}

// ============================================================================
// HÀM CẬP NHẬT TRẠNG THÁI (Status Updating)
// ============================================================================

func (r *InstanceLabReconciler) updateInstanceLabStatus(ctx context.Context, instanceLab *labv1alpha1.InstanceLab, pod *corev1.Pod, svc *corev1.Service, ing *networkingv1.Ingress) error {
	if pod != nil {
		instanceLab.Status.PodName = pod.Name
		instanceLab.Status.PodIP = pod.Status.PodIP

		switch pod.Status.Phase {
		case corev1.PodRunning:
			instanceLab.Status.Phase = PhaseRunning
			utils.SetCondition(&instanceLab.Status.Conditions, "PodReady", metav1.ConditionTrue, "ContainerRunning", "Sysbox VM instance is running and responsive")
		case corev1.PodPending:
			instanceLab.Status.Phase = PhaseCreating
			utils.SetCondition(&instanceLab.Status.Conditions, "PodReady", metav1.ConditionFalse, "PodPending", "Waiting for K8s scheduler and volume mounting")
		case corev1.PodFailed:
			instanceLab.Status.Phase = PhaseFailed
			utils.SetCondition(&instanceLab.Status.Conditions, "PodReady", metav1.ConditionFalse, "PodFailed", "Sysbox VM Pod execution failed")
		default:
			instanceLab.Status.Phase = PhaseCreating
		}
	} else {
		instanceLab.Status.Phase = PhaseCreating
	}

	endpoints := make([]labv1alpha1.InstanceLabAccessEndpoint, 0, len(instanceLab.Spec.Ports))
	for _, p := range instanceLab.Spec.Ports {
		internalAddr := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, pod.Namespace, p.Port)
		ep := labv1alpha1.InstanceLabAccessEndpoint{
			Name:            p.Name,
			Protocol:        "TCP",
			InternalAddress: internalAddr,
		}
		if p.Expose && ing != nil && len(ing.Spec.Rules) > 0 {
			ep.Protocol = "HTTPS"
			ep.URL = fmt.Sprintf("https://%s", ing.Spec.Rules[0].Host)
		}
		endpoints = append(endpoints, ep)
	}
	instanceLab.Status.AccessEndpoints = endpoints

	if strings.EqualFold(instanceLab.Status.Phase, "Running") {
		utils.SetCondition(&instanceLab.Status.Conditions, "Ready", metav1.ConditionTrue, "InstanceReady", "InstanceLab is ready for student connectivity")
	} else {
		utils.SetCondition(&instanceLab.Status.Conditions, "Ready", metav1.ConditionFalse, "InstanceProvisioning", "InstanceLab is currently being provisioned")
	}

	return r.Status().Update(ctx, instanceLab)
}

func (r *InstanceLabReconciler) updateStatusFailed(ctx context.Context, instanceLab *labv1alpha1.InstanceLab, reason, message string) {
	instanceLab.Status.Phase = PhaseFailed
	utils.SetCondition(&instanceLab.Status.Conditions, "Ready", metav1.ConditionFalse, reason, message)
	_ = r.Status().Update(ctx, instanceLab)
}

// ============================================================================
// GIÁM SÁT THAO TÁC NGƯỜI DÙNG & SELF-HEALING (SetupWithManager)
// ============================================================================

func (r *InstanceLabReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&labv1alpha1.InstanceLab{}).
		Named("instancelab").
		Complete(r)
}
