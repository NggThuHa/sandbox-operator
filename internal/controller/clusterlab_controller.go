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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	labv1alpha1 "github.com/ngtukien/sandbox-operator/api/v1alpha1"
	"github.com/ngtukien/sandbox-operator/internal/utils"
)

const (
	PhaseRunning  = "Running"
	PhaseCreating = "Creating"
	PhaseFailed   = "Failed"
)

// ClusterLabReconciler đóng vai trò là "Người quản lý cơ sở hạ tầng" của nền tảng Lab.
// Chi phối các vùng không gian con (Namespace), áp đặt chính sách mạng và giới hạn tài nguyên.
type ClusterLabReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ============================================================================
// KHAI BÁO RBAC MARKERS (Phân Quyền cho Controller)
// ============================================================================

// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs/finalizers,verbs=update
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// ============================================================================
// VÒNG LẶP ĐIỀU KHIỂN TRUNG TÂM (Reconcile Loop)
// ============================================================================

func (r *ClusterLabReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation for ClusterLab", "name", req.Name)

	// 1. TRÍCH XUẤT (FETCH) ĐỐI TƯỢNG CLUSTERLAB TỪ API KUBERNETES
	clusterLab := &labv1alpha1.ClusterLab{}
	if err := r.Get(ctx, req.NamespacedName, clusterLab); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ClusterLab resource not found, ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch ClusterLab")
		return ctrl.Result{}, err
	}

	// 2. QUY TẮC ƯU TIÊN FINALIZER (Finalizer-First Rule)
	if clusterLab.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(clusterLab, utils.ClusterLabFinalizer) {
		log.Info("Adding Finalizer to ClusterLab to ensure safe lifecycle management")
		controllerutil.AddFinalizer(clusterLab, utils.ClusterLabFinalizer)
		if err := r.Update(ctx, clusterLab); err != nil {
			log.Error(err, "Failed to update ClusterLab with Finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 3. XỬ LÝ QUY TRÌNH TỬ VONG (Finalizer & Cascade Deletion)
	if clusterLab.DeletionTimestamp != nil {
		return r.reconcileFinalizer(ctx, clusterLab)
	}

	// 4. BỘ ĐỊNH DANH NAMESPACE THỰC TẾ (Không vượt quá 63 ký tự RFC 1123)
	targetNamespace := utils.GenerateTargetNamespace(clusterLab.Name)
	if clusterLab.Status.TargetNamespace == "" {
		clusterLab.Status.TargetNamespace = targetNamespace
	}

	// 5. QUẢN VÂN VÒNG ĐỜI TTL (Time-To-Live & Chống trôi Requeue)
	isExpired, requeueAfter, err := r.reconcileTTL(ctx, clusterLab)
	if err != nil {
		log.Error(err, "Failed to reconcile TTL")
		return ctrl.Result{}, err
	}
	if isExpired {
		log.Info("ClusterLab TTL expired, issuing deletion command", "name", clusterLab.Name)
		if err := r.Delete(ctx, clusterLab); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to delete expired ClusterLab")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 6. KIẾN TẠO CƠ BỘ VÀ TÀI NGUYÊN HẠ TẦNG (Resource Provisioning)
	if err := r.reconcileNamespace(ctx, clusterLab, targetNamespace); err != nil {
		log.Error(err, "Failed to reconcile Namespace", "namespace", targetNamespace)
		r.updateStatusFailed(ctx, clusterLab, "NamespaceError", err.Error())
		return ctrl.Result{}, err
	}

	if err := r.reconcileResourceQuota(ctx, clusterLab, targetNamespace); err != nil {
		log.Error(err, "Failed to reconcile ResourceQuota", "namespace", targetNamespace)
		r.updateStatusFailed(ctx, clusterLab, "QuotaError", err.Error())
		return ctrl.Result{}, err
	}

	if err := r.reconcileNetworkPolicy(ctx, clusterLab, targetNamespace); err != nil {
		log.Error(err, "Failed to reconcile NetworkPolicy", "namespace", targetNamespace)
		r.updateStatusFailed(ctx, clusterLab, "NetworkPolicyError", err.Error())
		return ctrl.Result{}, err
	}

	// 7. CẬP NHẬT TRẠNG THÁI HỆ THỐNG GIAO DIỆN WEB (Status Updating)
	if err := r.updateClusterLabStatus(ctx, clusterLab, targetNamespace); err != nil {
		log.Error(err, "Failed to update ClusterLab status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled ClusterLab", "name", clusterLab.Name, "phase", clusterLab.Status.Phase)
	if requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

// ============================================================================
// HỆ CÁC HÀM XỬ LÝ TÀI NGUYÊN & VÒNG ĐỜI (Provisioning & Lifecycle Helper Functions)
// ============================================================================

func (r *ClusterLabReconciler) reconcileFinalizer(ctx context.Context, clusterLab *labv1alpha1.ClusterLab) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(clusterLab, utils.ClusterLabFinalizer) {
		return ctrl.Result{}, nil
	}

	targetNamespace := clusterLab.Status.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = utils.GenerateTargetNamespace(clusterLab.Name)
	}

	log.Info("Executing Cascade Deletion for ClusterLab", "namespace", targetNamespace)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: targetNamespace}}
	if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "Failed to trigger Namespace deletion", "namespace", targetNamespace)
		return ctrl.Result{}, err
	}

	var checkNs corev1.Namespace
	err := r.Get(ctx, types.NamespacedName{Name: targetNamespace}, &checkNs)
	if err == nil || !apierrors.IsNotFound(err) {
		log.Info("Waiting for Namespace and child resources to be fully eradicated by K8s Garbage Collector", "namespace", targetNamespace)
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	log.Info("Namespace successfully deleted. Removing Finalizer from ClusterLab", "name", clusterLab.Name)
	controllerutil.RemoveFinalizer(clusterLab, utils.ClusterLabFinalizer)
	if err := r.Update(ctx, clusterLab); err != nil {
		log.Error(err, "Failed to remove Finalizer from ClusterLab")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterLabReconciler) reconcileTTL(_ context.Context, clusterLab *labv1alpha1.ClusterLab) (bool, time.Duration, error) {
	if clusterLab.Spec.TTL == "" {
		clusterLab.Status.ExpiresAt = nil
		return false, 0, nil
	}

	duration, err := time.ParseDuration(clusterLab.Spec.TTL)
	if err != nil {
		return false, 0, fmt.Errorf("invalid TTL string format %q: %v", clusterLab.Spec.TTL, err)
	}

	expiresTime := clusterLab.CreationTimestamp.Add(duration)
	clusterLab.Status.ExpiresAt = &metav1.Time{Time: expiresTime}

	now := time.Now()
	if now.After(expiresTime) || now.Equal(expiresTime) {
		return true, 0, nil
	}

	return false, time.Until(expiresTime), nil
}

func (r *ClusterLabReconciler) reconcileNamespace(ctx context.Context, clusterLab *labv1alpha1.ClusterLab, nsName string) error {
	log := logf.FromContext(ctx)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[utils.LabelManagedBy] = utils.LabelValueManagedBy
		ns.Labels[utils.LabelClusterLab] = clusterLab.Name

		for k, v := range clusterLab.Labels {
			if strings.HasPrefix(k, "student_") || strings.HasPrefix(k, "lab_") || strings.HasPrefix(k, "session_") {
				ns.Labels[k] = v
			}
		}
		return nil
	})

	if err != nil {
		return err
	}
	log.Info("Reconciled Namespace successfully", "namespace", nsName)
	return nil
}

func (r *ClusterLabReconciler) reconcileResourceQuota(ctx context.Context, clusterLab *labv1alpha1.ClusterLab, nsName string) error {
	log := logf.FromContext(ctx)
	quotaName := "lab-resource-quota"
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: nsName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, quota, func() error {
		if quota.Spec.Hard == nil {
			quota.Spec.Hard = corev1.ResourceList{}
		}

		if !clusterLab.Spec.Quota.Compute.CPU.Limit.IsZero() {
			quota.Spec.Hard[corev1.ResourceLimitsCPU] = clusterLab.Spec.Quota.Compute.CPU.Limit
			quota.Spec.Hard[corev1.ResourceRequestsCPU] = clusterLab.Spec.Quota.Compute.CPU.Limit
		}
		if !clusterLab.Spec.Quota.Compute.Memory.Limit.IsZero() {
			quota.Spec.Hard[corev1.ResourceLimitsMemory] = clusterLab.Spec.Quota.Compute.Memory.Limit
			quota.Spec.Hard[corev1.ResourceRequestsMemory] = clusterLab.Spec.Quota.Compute.Memory.Limit
		}

		if clusterLab.Spec.Quota.Objects.PodsLimit > 0 {
			quota.Spec.Hard[corev1.ResourcePods] = *resource.NewQuantity(int64(clusterLab.Spec.Quota.Objects.PodsLimit), resource.DecimalSI)
		}
		if clusterLab.Spec.Quota.Objects.ServicesLimit > 0 {
			quota.Spec.Hard[corev1.ResourceServices] = *resource.NewQuantity(int64(clusterLab.Spec.Quota.Objects.ServicesLimit), resource.DecimalSI)
		}

		if !clusterLab.Spec.Quota.Storage.Limit.IsZero() {
			quota.Spec.Hard[corev1.ResourceLimitsEphemeralStorage] = clusterLab.Spec.Quota.Storage.Limit
		}
		return nil
	})

	if err != nil {
		return err
	}
	log.Info("Reconciled ResourceQuota successfully", "name", quotaName, "namespace", nsName)
	return nil
}

func (r *ClusterLabReconciler) reconcileNetworkPolicy(ctx context.Context, clusterLab *labv1alpha1.ClusterLab, nsName string) error {
	log := logf.FromContext(ctx)
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lab-network-policy",
			Namespace: nsName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, netpol, func() error {
		netpol.Spec.PodSelector = metav1.LabelSelector{}
		netpol.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}

		switch strings.ToLower(clusterLab.Spec.Network.Type) {
		case "isolate":
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{}
		case "internal":
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
			}
		case "external":
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{},
				},
			}
		default:
			log.Info("Unknown network policy type, defaulting to internal isolation", "type", clusterLab.Spec.Network.Type)
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{}
		}
		return nil
	})

	if err != nil {
		return err
	}
	log.Info("Reconciled NetworkPolicy successfully", "type", clusterLab.Spec.Network.Type, "namespace", nsName)
	return nil
}

// ============================================================================
// HÀM CẬP NHẬT TRẠNG THÁI VÀ BÁO CÁO GIAO DIỆN (Status Updating Functions)
// ============================================================================

func (r *ClusterLabReconciler) updateClusterLabStatus(ctx context.Context, clusterLab *labv1alpha1.ClusterLab, nsName string) error {
	clusterLab.Status.Phase = PhaseRunning
	clusterLab.Status.TargetNamespace = nsName

	quota := &corev1.ResourceQuota{}
	if err := r.Get(ctx, types.NamespacedName{Name: "lab-resource-quota", Namespace: nsName}, quota); err == nil {
		used := quota.Status.Used
		if cpu, ok := used[corev1.ResourceLimitsCPU]; ok {
			clusterLab.Status.QuotaUsage.Compute.CPUUsed = cpu
		}
		if mem, ok := used[corev1.ResourceLimitsMemory]; ok {
			clusterLab.Status.QuotaUsage.Compute.MemoryUsed = mem
		}
		if pods, ok := used[corev1.ResourcePods]; ok {
			clusterLab.Status.QuotaUsage.Objects.PodsUsed = int32(pods.Value())
		}
		if svcs, ok := used[corev1.ResourceServices]; ok {
			clusterLab.Status.QuotaUsage.Objects.ServicesUsed = int32(svcs.Value())
		}
		if eph, ok := used[corev1.ResourceLimitsEphemeralStorage]; ok {
			clusterLab.Status.QuotaUsage.Storage.Used = eph
		}
	}

	instanceLabList := &labv1alpha1.InstanceLabList{}
	if err := r.List(ctx, instanceLabList, client.InNamespace(clusterLab.Namespace)); err == nil {
		var total, ready int32
		for _, inst := range instanceLabList.Items {
			if inst.Spec.ClusterLabRef == clusterLab.Name {
				total++
				if strings.ToLower(inst.Status.Phase) == "running" {
					ready++
				}
			}
		}
		clusterLab.Status.InstanceCount.Total = total
		clusterLab.Status.InstanceCount.Ready = ready
	}

	utils.SetCondition(&clusterLab.Status.Conditions, "NamespaceReady", metav1.ConditionTrue, "NamespaceReconciled", fmt.Sprintf("Namespace %s is active and operational", nsName))
	utils.SetCondition(&clusterLab.Status.Conditions, "QuotaReady", metav1.ConditionTrue, "QuotaReconciled", "Resource limits successfully enforced")
	utils.SetCondition(&clusterLab.Status.Conditions, "NetworkPolicyReady", metav1.ConditionTrue, "PolicyReconciled", fmt.Sprintf("Network mode applied: %s", clusterLab.Spec.Network.Type))
	utils.SetCondition(&clusterLab.Status.Conditions, "Ready", metav1.ConditionTrue, "ClusterReady", "ClusterLab is fully provisioned and operational")

	return r.Status().Update(ctx, clusterLab)
}

func (r *ClusterLabReconciler) updateStatusFailed(ctx context.Context, clusterLab *labv1alpha1.ClusterLab, reason, message string) {
	clusterLab.Status.Phase = PhaseFailed
	utils.SetCondition(&clusterLab.Status.Conditions, "Ready", metav1.ConditionFalse, reason, message)
	_ = r.Status().Update(ctx, clusterLab)
}

// ============================================================================
// CÁC KÊNH GIÁM SÁT THỜI GIAN THỰC (Real-time Watch Setup)
// ============================================================================

func (r *ClusterLabReconciler) findParentClusterLab(ctx context.Context, obj client.Object) []reconcile.Request {
	instanceLab, ok := obj.(*labv1alpha1.InstanceLab)
	if !ok || instanceLab.Spec.ClusterLabRef == "" {
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      instanceLab.Spec.ClusterLabRef,
				Namespace: instanceLab.Namespace,
			},
		},
	}
}

func (r *ClusterLabReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&labv1alpha1.ClusterLab{}).
		Watches(&labv1alpha1.InstanceLab{}, handler.EnqueueRequestsFromMapFunc(r.findParentClusterLab)).
		Named("clusterlab").
		Complete(r)
}
