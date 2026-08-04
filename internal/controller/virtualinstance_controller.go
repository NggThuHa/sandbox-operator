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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	labv1alpha1 "github.com/ngtukien/systemd-operator/api/v1alpha1"
	"github.com/ngtukien/systemd-operator/internal/utils"
)

const (
	SysboxRuntimeClass = "sysbox-runc"
)

// VirtualInstanceReconciler đóng vai trò là "Người vận hành máy ảo".
// Lắp ráp Pod (Sysbox runtime), chuẩn bị ổ cứng (PVC), cấu hình dịch vụ mạng và tổng hợp danh sách URL bảo mật HTTPS.
type VirtualInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ============================================================================
// KHAI BÁO RBAC MARKERS (Phân Quyền cho VirtualInstance Controller)
// ============================================================================

// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods;services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// ============================================================================
// VÒNG LẶP ĐIỀU KHIỂN TRUNG TÂM (Reconcile Loop)
// ============================================================================

func (r *VirtualInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation for VirtualInstance", "name", req.Name)

	// 1. TRÍCH XUẤT ĐỐI TƯỢNG VIRTUALINSTANCE
	virtualInstance := &labv1alpha1.VirtualInstance{}
	if err := r.Get(ctx, req.NamespacedName, virtualInstance); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("VirtualInstance resource not found, ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch VirtualInstance")
		return ctrl.Result{}, err
	}

	// 2. QUY TẮC ƯU TIÊN FINALIZER (Finalizer-First Rule)
	if virtualInstance.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(virtualInstance, utils.VirtualInstanceFinalizer) {
		log.Info("Adding Finalizer to VirtualInstance for safe cross-namespace resource cleanup")
		controllerutil.AddFinalizer(virtualInstance, utils.VirtualInstanceFinalizer)
		if err := r.Update(ctx, virtualInstance); err != nil {
			log.Error(err, "Failed to update VirtualInstance with Finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 3. XỬ LÝ QUY TRÌNH TIÊU HUỶ VÀ GIẢI PHÓNG TÀI NGUYÊN (Cross-namespace Cleanup)
	if virtualInstance.DeletionTimestamp != nil {
		return r.reconcileFinalizer(ctx, virtualInstance)
	}

	// 4. XÁC THỰC CỤM CHA VIRTUALCLUSTER VÀ THIẾT LẬП OWNER REFERENCE
	virtualCluster, isReady, err := r.resolveParentVirtualCluster(ctx, virtualInstance)
	if err != nil {
		log.Error(err, "Error querying parent VirtualCluster", "virtualCluster", virtualInstance.Spec.VirtualClusterRef)
		return ctrl.Result{}, err
	}
	if !isReady {
		log.Info("Parent VirtualCluster is not running yet or awaiting initial reconciliation, requeuing", "virtualCluster", virtualInstance.Spec.VirtualClusterRef)
		virtualInstance.Status.Phase = "Pending"
		utils.SetCondition(&virtualInstance.Status.Conditions, "WaitingForCluster", metav1.ConditionFalse, "ParentNotReady", fmt.Sprintf("Waiting for parent VirtualCluster %s to enter Running phase", virtualInstance.Spec.VirtualClusterRef))
		_ = r.Status().Update(ctx, virtualInstance)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	targetNamespace := virtualCluster.Status.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = utils.GenerateTargetNamespace(virtualCluster.Name)
	}

	// 5. KIẾN TẠO CẤU TRÚC Ổ ĐĨA LƯU TRỮ (PersistentVolumeClaims)
	pvcMap, volumesStatus, err := r.reconcilePVCs(ctx, virtualInstance, targetNamespace)
	if err != nil {
		log.Error(err, "Failed to reconcile PVCs for VirtualInstance")
		r.updateStatusFailed(ctx, virtualInstance, "PVCError", err.Error())
		return ctrl.Result{}, err
	}

	// 6. KIẾN TẠO MÁY ẢO DOCKER-IN-DOCKER VỚI SYSBOX RUNTIME (Pod)
	pod, err := r.reconcileSysboxPod(ctx, virtualInstance, targetNamespace, pvcMap)
	if err != nil {
		log.Error(err, "Failed to reconcile Sysbox Pod for VirtualInstance")
		r.updateStatusFailed(ctx, virtualInstance, "PodError", err.Error())
		return ctrl.Result{}, err
	}

	// 7. KIẾN TẠO DỊCH VỤ GOM LƯU LƯỢNG MẠNG (ClusterIP Service)
	svc, err := r.reconcileServices(ctx, virtualInstance, targetNamespace)
	if err != nil {
		log.Error(err, "Failed to reconcile Service for VirtualInstance")
		r.updateStatusFailed(ctx, virtualInstance, "ServiceError", err.Error())
		return ctrl.Result{}, err
	}

	// 8. KIẾN TẠO CỔNG TRUY CẬP BẢO MẬT HTTPS (Ingress with TLS & Cert-Manager)
	ingress, err := r.reconcileIngress(ctx, virtualInstance, virtualCluster, targetNamespace, svc)
	if err != nil {
		log.Error(err, "Failed to reconcile Ingress for VirtualInstance")
		r.updateStatusFailed(ctx, virtualInstance, "IngressError", err.Error())
		return ctrl.Result{}, err
	}

	// 9. CẬP NHẬT TRẠNG THÁI GIAO DIỆN (UI Endpoints, Pod IP, Volumes Status)
	if err := r.updateVirtualInstanceStatus(ctx, virtualInstance, pod, svc, ingress, volumesStatus); err != nil {
		log.Error(err, "Failed to update VirtualInstance status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled VirtualInstance", "name", virtualInstance.Name, "phase", virtualInstance.Status.Phase, "podIP", virtualInstance.Status.PodIP)
	return ctrl.Result{}, nil
}

// ============================================================================
// HỆ CÁC HÀM XỬ LÝ QUY TRÌNH KIẾN TẠO VÀ THANH MỘ THAI SẢN
// ============================================================================

func (r *VirtualInstanceReconciler) resolveParentVirtualCluster(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance) (*labv1alpha1.VirtualCluster, bool, error) {
	virtualCluster := &labv1alpha1.VirtualCluster{}
	err := r.Get(ctx, types.NamespacedName{Name: virtualInstance.Spec.VirtualClusterRef, Namespace: virtualInstance.Namespace}, virtualCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if err := controllerutil.SetControllerReference(virtualCluster, virtualInstance, r.Scheme); err == nil {
		_ = r.Update(ctx, virtualInstance)
	}

	isReady := strings.EqualFold(virtualCluster.Status.Phase, "Running") || strings.EqualFold(virtualCluster.Status.Phase, "Ready")
	return virtualCluster, isReady, nil
}

func (r *VirtualInstanceReconciler) reconcileFinalizer(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(virtualInstance, utils.VirtualInstanceFinalizer) {
		return ctrl.Result{}, nil
	}

	log.Info("Executing Finalizer cross-namespace cleanup for VirtualInstance", "instance", virtualInstance.Name)

	virtualCluster := &labv1alpha1.VirtualCluster{}
	targetNs := ""
	if err := r.Get(ctx, types.NamespacedName{Name: virtualInstance.Spec.VirtualClusterRef, Namespace: virtualInstance.Namespace}, virtualCluster); err == nil {
		targetNs = virtualCluster.Status.TargetNamespace
	}
	if targetNs == "" {
		targetNs = utils.GenerateTargetNamespace(virtualInstance.Spec.VirtualClusterRef)
	}

	matchLabels := client.MatchingLabels{utils.LabelVirtualInstance: virtualInstance.Name}

	_ = r.DeleteAllOf(ctx, &networkingv1.Ingress{}, client.InNamespace(targetNs), matchLabels)
	_ = r.DeleteAllOf(ctx, &corev1.Service{}, client.InNamespace(targetNs), matchLabels)
	_ = r.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(targetNs), matchLabels)
	_ = r.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(targetNs), matchLabels)

	log.Info("Successfully cleaned up all workloads in target namespace. Removing Finalizer", "name", virtualInstance.Name)
	controllerutil.RemoveFinalizer(virtualInstance, utils.VirtualInstanceFinalizer)
	if err := r.Update(ctx, virtualInstance); err != nil {
		log.Error(err, "Failed to remove Finalizer from VirtualInstance")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *VirtualInstanceReconciler) reconcilePVCs(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance, targetNs string) (map[string]string, labv1alpha1.VirtualInstanceVolumesStatus, error) {
	log := logf.FromContext(ctx)
	pvcMap := make(map[string]string)
	var volStatus labv1alpha1.VirtualInstanceVolumesStatus

	createPVC := func(pvcName, volType string, size resource.Quantity, _ bool, labelName string) (labv1alpha1.VirtualInstanceVolumeStatusDetail, error) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: targetNs,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
			if pvc.Labels == nil {
				pvc.Labels = make(map[string]string)
			}
			pvc.Labels[utils.LabelManagedBy] = utils.LabelValueManagedBy
			pvc.Labels[utils.LabelVirtualCluster] = virtualInstance.Spec.VirtualClusterRef
			pvc.Labels[utils.LabelVirtualInstance] = virtualInstance.Name

			if len(pvc.Spec.AccessModes) == 0 {
				pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			}

			scName := utils.GetStorageClassName(volType)
			if scName != "" {
				pvc.Spec.StorageClassName = &scName
			} else {
				pvc.Spec.StorageClassName = nil
			}

			if pvc.Spec.Resources.Requests == nil {
				pvc.Spec.Resources.Requests = corev1.ResourceList{}
			}
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = size
			return nil
		})

		detail := labv1alpha1.VirtualInstanceVolumeStatusDetail{
			Name:    labelName,
			PVCName: pvcName,
			Status:  string(pvc.Status.Phase),
		}
		if detail.Status == "" {
			detail.Status = "Pending"
		}
		return detail, err
	}

	if !virtualInstance.Spec.Storage.Root.Size.IsZero() {
		rootPvcName := utils.SanitizeName(fmt.Sprintf("%s-root-pvc", virtualInstance.Name), 63)
		detail, err := createPVC(rootPvcName, virtualInstance.Spec.Storage.Root.Type, virtualInstance.Spec.Storage.Root.Size, true, "root")
		if err != nil {
			return nil, volStatus, err
		}
		pvcMap["/var/lib/docker"] = rootPvcName
		volStatus.RootVolume = detail
	}

	for _, dv := range virtualInstance.Spec.Storage.DataVolumes {
		dvPvcName := utils.SanitizeName(fmt.Sprintf("%s-%s-pvc", virtualInstance.Name, dv.Name), 63)
		detail, err := createPVC(dvPvcName, dv.Type, dv.Size, false, dv.Name)
		if err != nil {
			return nil, volStatus, err
		}
		pvcMap[dv.MountPath] = dvPvcName
		volStatus.DataVolumes = append(volStatus.DataVolumes, detail)
	}

	log.Info("Reconciled all volume PVCs successfully", "virtualInstance", virtualInstance.Name, "namespace", targetNs)
	return pvcMap, volStatus, nil
}

func (r *VirtualInstanceReconciler) reconcileSysboxPod(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance, targetNs string, pvcMap map[string]string) (*corev1.Pod, error) {
	log := logf.FromContext(ctx)
	podName := utils.SanitizeName(fmt.Sprintf("%s-sysbox", virtualInstance.Name), 63)
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
		pod.Labels[utils.LabelVirtualCluster] = virtualInstance.Spec.VirtualClusterRef
		pod.Labels[utils.LabelVirtualInstance] = virtualInstance.Name
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
		if virtualInstance.Spec.Hostname != "" {
			pod.Spec.Hostname = virtualInstance.Spec.Hostname
		}
		if len(virtualInstance.Spec.ImagePullSecrets) > 0 {
			pod.Spec.ImagePullSecrets = virtualInstance.Spec.ImagePullSecrets
		}

		var volumes []corev1.Volume
		var mounts []corev1.VolumeMount

		idx := 0
		for mountPath, pvcName := range pvcMap {
			volName := fmt.Sprintf("vol-%d", idx)
			volumes = append(volumes, corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			})
			mounts = append(mounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: mountPath,
			})
			idx++
		}

		// Mount lxcfs proc filesystem for accurate resource reporting and isolation in Sysbox containers
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
		for _, p := range virtualInstance.Spec.Ports {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				Name:          utils.SanitizeName(p.Name, 15),
				ContainerPort: p.TargetPort,
				Protocol:      corev1.ProtocolTCP,
			})
		}

		image := virtualInstance.Spec.Image
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
		if !virtualInstance.Spec.Resources.CPU.Request.IsZero() {
			resList.Requests[corev1.ResourceCPU] = virtualInstance.Spec.Resources.CPU.Request
		}
		if !virtualInstance.Spec.Resources.CPU.Limit.IsZero() {
			resList.Limits[corev1.ResourceCPU] = virtualInstance.Spec.Resources.CPU.Limit
		}
		if !virtualInstance.Spec.Resources.Memory.Request.IsZero() {
			resList.Requests[corev1.ResourceMemory] = virtualInstance.Spec.Resources.Memory.Request
		}
		if !virtualInstance.Spec.Resources.Memory.Limit.IsZero() {
			resList.Limits[corev1.ResourceMemory] = virtualInstance.Spec.Resources.Memory.Limit
		}
		if !virtualInstance.Spec.Resources.EphemeralStorage.Request.IsZero() {
			resList.Requests[corev1.ResourceEphemeralStorage] = virtualInstance.Spec.Resources.EphemeralStorage.Request
		}
		if !virtualInstance.Spec.Resources.EphemeralStorage.Limit.IsZero() {
			resList.Limits[corev1.ResourceEphemeralStorage] = virtualInstance.Spec.Resources.EphemeralStorage.Limit
		} else if !virtualInstance.Spec.Storage.Root.Size.IsZero() {
			resList.Limits[corev1.ResourceEphemeralStorage] = virtualInstance.Spec.Storage.Root.Size
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

func (r *VirtualInstanceReconciler) reconcileServices(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance, targetNs string) (*corev1.Service, error) {
	log := logf.FromContext(ctx)
	if len(virtualInstance.Spec.Ports) == 0 {
		return nil, nil
	}

	svcName := utils.SanitizeName(fmt.Sprintf("%s-svc", virtualInstance.Name), 63)
	podName := utils.SanitizeName(fmt.Sprintf("%s-sysbox", virtualInstance.Name), 63)

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
		svc.Labels[utils.LabelVirtualCluster] = virtualInstance.Spec.VirtualClusterRef
		svc.Labels[utils.LabelVirtualInstance] = virtualInstance.Name

		svc.Spec.Selector = map[string]string{"app": podName}
		svc.Spec.Type = corev1.ServiceTypeClusterIP

		var svcPorts []corev1.ServicePort
		for _, p := range virtualInstance.Spec.Ports {
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

func (r *VirtualInstanceReconciler) reconcileIngress(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance, virtualCluster *labv1alpha1.VirtualCluster, targetNs string, svc *corev1.Service) (*networkingv1.Ingress, error) {
	log := logf.FromContext(ctx)
	if svc == nil {
		return nil, nil
	}

	var exposePorts []labv1alpha1.VirtualInstancePort
	for _, p := range virtualInstance.Spec.Ports {
		if p.Expose {
			exposePorts = append(exposePorts, p)
		}
	}
	if len(exposePorts) == 0 {
		return nil, nil
	}

	ingName := utils.SanitizeName(fmt.Sprintf("%s-ingress", virtualInstance.Name), 63)
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
		ingress.Labels[utils.LabelVirtualCluster] = virtualInstance.Spec.VirtualClusterRef
		ingress.Labels[utils.LabelVirtualInstance] = virtualInstance.Name

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

		host := utils.GenerateIngressHost(virtualInstance.Name, virtualCluster.Name, "")

		secretName := utils.SanitizeName(fmt.Sprintf("%s-tls-secret", virtualInstance.Name), 63)
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
	log.Info("Reconciled Ingress successfully with HTTPS/TLS Enabled", "ingressName", ingName, "host", utils.GenerateIngressHost(virtualInstance.Name, virtualCluster.Name, ""))
	return ingress, nil
}

// ============================================================================
// HÀM CẬP NHẬT TRẠNG THÁI (Status Updating)
// ============================================================================

func (r *VirtualInstanceReconciler) updateVirtualInstanceStatus(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance, pod *corev1.Pod, svc *corev1.Service, ing *networkingv1.Ingress, volStatus labv1alpha1.VirtualInstanceVolumesStatus) error {
	virtualInstance.Status.VolumesStatus = volStatus

	if pod != nil {
		virtualInstance.Status.PodName = pod.Name
		virtualInstance.Status.PodIP = pod.Status.PodIP

		switch pod.Status.Phase {
		case corev1.PodRunning:
			virtualInstance.Status.Phase = PhaseRunning
			utils.SetCondition(&virtualInstance.Status.Conditions, "PodReady", metav1.ConditionTrue, "ContainerRunning", "Sysbox VM instance is running and responsive")
		case corev1.PodPending:
			virtualInstance.Status.Phase = PhaseCreating
			utils.SetCondition(&virtualInstance.Status.Conditions, "PodReady", metav1.ConditionFalse, "PodPending", "Waiting for K8s scheduler and volume mounting")
		case corev1.PodFailed:
			virtualInstance.Status.Phase = PhaseFailed
			utils.SetCondition(&virtualInstance.Status.Conditions, "PodReady", metav1.ConditionFalse, "PodFailed", "Sysbox VM Pod execution failed")
		default:
			virtualInstance.Status.Phase = PhaseCreating
		}
	} else {
		virtualInstance.Status.Phase = PhaseCreating
	}

	endpoints := make([]labv1alpha1.VirtualInstanceAccessEndpoint, 0, len(virtualInstance.Spec.Ports))
	for _, p := range virtualInstance.Spec.Ports {
		internalAddr := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, pod.Namespace, p.Port)
		ep := labv1alpha1.VirtualInstanceAccessEndpoint{
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
	virtualInstance.Status.AccessEndpoints = endpoints

	if strings.EqualFold(virtualInstance.Status.Phase, "Running") {
		utils.SetCondition(&virtualInstance.Status.Conditions, "Ready", metav1.ConditionTrue, "InstanceReady", "VirtualInstance is ready for student connectivity")
	} else {
		utils.SetCondition(&virtualInstance.Status.Conditions, "Ready", metav1.ConditionFalse, "InstanceProvisioning", "VirtualInstance is currently being provisioned")
	}

	return r.Status().Update(ctx, virtualInstance)
}

func (r *VirtualInstanceReconciler) updateStatusFailed(ctx context.Context, virtualInstance *labv1alpha1.VirtualInstance, reason, message string) {
	virtualInstance.Status.Phase = PhaseFailed
	utils.SetCondition(&virtualInstance.Status.Conditions, "Ready", metav1.ConditionFalse, reason, message)
	_ = r.Status().Update(ctx, virtualInstance)
}

// ============================================================================
// GIÁM SÁT THAO TÁC NGƯỜI DÙNG & SELF-HEALING (SetupWithManager)
// ============================================================================

func (r *VirtualInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&labv1alpha1.VirtualInstance{}).
		Named("virtualinstance").
		Complete(r)
}
