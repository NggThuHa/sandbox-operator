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

	labv1alpha1 "github.com/NguyenTuKien/TYP-Operator/api/v1alpha1"
	"github.com/NguyenTuKien/TYP-Operator/internal/utils"
)

// VirtualClusterReconciler đóng vai trò là "Người quản lý cơ sở hạ tầng" của nền tảng Lab.
// Chi phối các vùng không gian con (Namespace), áp đặt chính sách mạng và giới hạn tài nguyên.
type VirtualClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ============================================================================
// KHAI BÁO RBAC MARKERS (Phân Quyền cho Controller)
// ============================================================================

// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// ============================================================================
// VÒNG LẶP ĐIỀU KHIỂN TRUNG TÂM (Reconcile Loop)
// ============================================================================

func (r *VirtualClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation for VirtualCluster", "name", req.Name)

	// 1. TRÍCH XUẤT (FETCH) ĐỐI TƯỢNG VIRTUALCLUSTER TỪ API KUBERNETES
	virtualCluster := &labv1alpha1.VirtualCluster{}
	if err := r.Get(ctx, req.NamespacedName, virtualCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("VirtualCluster resource not found, ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch VirtualCluster")
		return ctrl.Result{}, err
	}

	// 2. QUY TẮC ƯU TIÊN FINALIZER (Finalizer-First Rule)
	if virtualCluster.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(virtualCluster, utils.VirtualClusterFinalizer) {
		log.Info("Adding Finalizer to VirtualCluster to ensure safe lifecycle management")
		controllerutil.AddFinalizer(virtualCluster, utils.VirtualClusterFinalizer)
		if err := r.Update(ctx, virtualCluster); err != nil {
			log.Error(err, "Failed to update VirtualCluster with Finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 3. XỬ LÝ QUY TRÌNH TỬ VONG (Finalizer & Cascade Deletion)
	if virtualCluster.DeletionTimestamp != nil {
		return r.reconcileFinalizer(ctx, virtualCluster)
	}

	// 4. BỘ ĐỊNH DANH NAMESPACE THỰC TẾ (Không vượt quá 63 ký tự RFC 1123)
	targetNamespace := utils.GenerateTargetNamespace(virtualCluster.Name)
	if virtualCluster.Status.TargetNamespace == "" {
		virtualCluster.Status.TargetNamespace = targetNamespace
	}

	// 5. QUẢN VÂN VÒNG ĐỜI TTL (Time-To-Live & Chống trôi Requeue)
	isExpired, requeueAfter, err := r.reconcileTTL(ctx, virtualCluster)
	if err != nil {
		log.Error(err, "Failed to reconcile TTL")
		return ctrl.Result{}, err
	}
	if isExpired {
		log.Info("VirtualCluster TTL expired, issuing deletion command", "name", virtualCluster.Name)
		if err := r.Delete(ctx, virtualCluster); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to delete expired VirtualCluster")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 6. KIẾN TẠO CƠ BỘ VÀ TÀI NGUYÊN HẠ TẦNG (Resource Provisioning)
	if err := r.reconcileNamespace(ctx, virtualCluster, targetNamespace); err != nil {
		log.Error(err, "Failed to reconcile Namespace", "namespace", targetNamespace)
		r.updateStatusFailed(ctx, virtualCluster, "NamespaceError", err.Error())
		return ctrl.Result{}, err
	}

	if err := r.reconcileResourceQuota(ctx, virtualCluster, targetNamespace); err != nil {
		log.Error(err, "Failed to reconcile ResourceQuota", "namespace", targetNamespace)
		r.updateStatusFailed(ctx, virtualCluster, "QuotaError", err.Error())
		return ctrl.Result{}, err
	}

	if err := r.reconcileNetworkPolicy(ctx, virtualCluster, targetNamespace); err != nil {
		log.Error(err, "Failed to reconcile NetworkPolicy", "namespace", targetNamespace)
		r.updateStatusFailed(ctx, virtualCluster, "NetworkPolicyError", err.Error())
		return ctrl.Result{}, err
	}

	// 7. CẬP NHẬT TRẠNG THÁI HỆ THỐNG GIAO DIỆN WEB (Status Updating)
	if err := r.updateVirtualClusterStatus(ctx, virtualCluster, targetNamespace); err != nil {
		log.Error(err, "Failed to update VirtualCluster status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled VirtualCluster", "name", virtualCluster.Name, "phase", virtualCluster.Status.Phase)
	if requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

// ============================================================================
// HỆ CÁC HÀM XỬ LÝ TÀI NGUYÊN & VÒNG ĐỜI (Provisioning & Lifecycle Helper Functions)
// ============================================================================

func (r *VirtualClusterReconciler) reconcileFinalizer(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(virtualCluster, utils.VirtualClusterFinalizer) {
		return ctrl.Result{}, nil
	}

	targetNamespace := virtualCluster.Status.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = utils.GenerateTargetNamespace(virtualCluster.Name)
	}

	log.Info("Executing Cascade Deletion for VirtualCluster", "namespace", targetNamespace)

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

	log.Info("Namespace successfully deleted. Removing Finalizer from VirtualCluster", "name", virtualCluster.Name)
	controllerutil.RemoveFinalizer(virtualCluster, utils.VirtualClusterFinalizer)
	if err := r.Update(ctx, virtualCluster); err != nil {
		log.Error(err, "Failed to remove Finalizer from VirtualCluster")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VirtualClusterReconciler) reconcileTTL(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster) (bool, time.Duration, error) {
	if virtualCluster.Spec.TTL == "" {
		virtualCluster.Status.ExpiresAt = nil
		return false, 0, nil
	}

	duration, err := time.ParseDuration(virtualCluster.Spec.TTL)
	if err != nil {
		return false, 0, fmt.Errorf("invalid TTL string format %q: %v", virtualCluster.Spec.TTL, err)
	}

	expiresTime := virtualCluster.CreationTimestamp.Add(duration)
	virtualCluster.Status.ExpiresAt = &metav1.Time{Time: expiresTime}

	now := time.Now()
	if now.After(expiresTime) || now.Equal(expiresTime) {
		return true, 0, nil
	}

	return false, time.Until(expiresTime), nil
}

func (r *VirtualClusterReconciler) reconcileNamespace(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster, nsName string) error {
	log := logf.FromContext(ctx)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[utils.LabelManagedBy] = utils.LabelValueManagedBy
		ns.Labels[utils.LabelVirtualCluster] = virtualCluster.Name

		for k, v := range virtualCluster.Labels {
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

func (r *VirtualClusterReconciler) reconcileResourceQuota(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster, nsName string) error {
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

		if !virtualCluster.Spec.Quota.Compute.CPU.Limit.IsZero() {
			quota.Spec.Hard[corev1.ResourceLimitsCPU] = virtualCluster.Spec.Quota.Compute.CPU.Limit
			quota.Spec.Hard[corev1.ResourceRequestsCPU] = virtualCluster.Spec.Quota.Compute.CPU.Limit
		}
		if !virtualCluster.Spec.Quota.Compute.Memory.Limit.IsZero() {
			quota.Spec.Hard[corev1.ResourceLimitsMemory] = virtualCluster.Spec.Quota.Compute.Memory.Limit
			quota.Spec.Hard[corev1.ResourceRequestsMemory] = virtualCluster.Spec.Quota.Compute.Memory.Limit
		}

		if virtualCluster.Spec.Quota.Objects.PodsLimit > 0 {
			quota.Spec.Hard[corev1.ResourcePods] = *resource.NewQuantity(int64(virtualCluster.Spec.Quota.Objects.PodsLimit), resource.DecimalSI)
		}
		if virtualCluster.Spec.Quota.Objects.ServicesLimit > 0 {
			quota.Spec.Hard[corev1.ResourceServices] = *resource.NewQuantity(int64(virtualCluster.Spec.Quota.Objects.ServicesLimit), resource.DecimalSI)
		}

		if !virtualCluster.Spec.Quota.Storage.LocalLimit.IsZero() {
			localScKey := corev1.ResourceName(fmt.Sprintf("%s.storageclass.storage.k8s.io/requests.storage", utils.GetStorageClassName("local")))
			quota.Spec.Hard[localScKey] = virtualCluster.Spec.Quota.Storage.LocalLimit
		}
		if !virtualCluster.Spec.Quota.Storage.NetworkLimit.IsZero() {
			netScKey := corev1.ResourceName(fmt.Sprintf("%s.storageclass.storage.k8s.io/requests.storage", utils.GetStorageClassName("network")))
			quota.Spec.Hard[netScKey] = virtualCluster.Spec.Quota.Storage.NetworkLimit
		}
		return nil
	})

	if err != nil {
		return err
	}
	log.Info("Reconciled ResourceQuota successfully", "name", quotaName, "namespace", nsName)
	return nil
}

func (r *VirtualClusterReconciler) reconcileNetworkPolicy(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster, nsName string) error {
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

		switch strings.ToLower(virtualCluster.Spec.Network.Type) {
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
			log.Info("Unknown network policy type, defaulting to internal isolation", "type", virtualCluster.Spec.Network.Type)
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{}
		}
		return nil
	})

	if err != nil {
		return err
	}
	log.Info("Reconciled NetworkPolicy successfully", "type", virtualCluster.Spec.Network.Type, "namespace", nsName)
	return nil
}

// ============================================================================
// HÀM CẬP NHẬT TRẠNG THÁI VÀ BÁO CÁO GIAO DIỆN (Status Updating Functions)
// ============================================================================

func (r *VirtualClusterReconciler) updateVirtualClusterStatus(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster, nsName string) error {
	virtualCluster.Status.Phase = "Running"
	virtualCluster.Status.TargetNamespace = nsName

	quota := &corev1.ResourceQuota{}
	if err := r.Get(ctx, types.NamespacedName{Name: "lab-resource-quota", Namespace: nsName}, quota); err == nil {
		used := quota.Status.Used
		if cpu, ok := used[corev1.ResourceLimitsCPU]; ok {
			virtualCluster.Status.QuotaUsage.Compute.CPUUsed = cpu
		}
		if mem, ok := used[corev1.ResourceLimitsMemory]; ok {
			virtualCluster.Status.QuotaUsage.Compute.MemoryUsed = mem
		}
		if pods, ok := used[corev1.ResourcePods]; ok {
			virtualCluster.Status.QuotaUsage.Objects.PodsUsed = int32(pods.Value())
		}
		if svcs, ok := used[corev1.ResourceServices]; ok {
			virtualCluster.Status.QuotaUsage.Objects.ServicesUsed = int32(svcs.Value())
		}
		localScKey := corev1.ResourceName(fmt.Sprintf("%s.storageclass.storage.k8s.io/requests.storage", utils.GetStorageClassName("local")))
		if loc, ok := used[localScKey]; ok {
			virtualCluster.Status.QuotaUsage.Storage.LocalUsed = loc
		}
		netScKey := corev1.ResourceName(fmt.Sprintf("%s.storageclass.storage.k8s.io/requests.storage", utils.GetStorageClassName("network")))
		if net, ok := used[netScKey]; ok {
			virtualCluster.Status.QuotaUsage.Storage.NetworkUsed = net
		}
	}

	virtualInstanceList := &labv1alpha1.VirtualInstanceList{}
	if err := r.List(ctx, virtualInstanceList, client.InNamespace(virtualCluster.Namespace)); err == nil {
		var total, ready int32
		for _, inst := range virtualInstanceList.Items {
			if inst.Spec.VirtualClusterRef == virtualCluster.Name {
				total++
				if strings.ToLower(inst.Status.Phase) == "running" {
					ready++
				}
			}
		}
		virtualCluster.Status.InstanceCount.Total = total
		virtualCluster.Status.InstanceCount.Ready = ready
	}

	utils.SetCondition(&virtualCluster.Status.Conditions, "NamespaceReady", metav1.ConditionTrue, "NamespaceReconciled", fmt.Sprintf("Namespace %s is active and operational", nsName))
	utils.SetCondition(&virtualCluster.Status.Conditions, "QuotaReady", metav1.ConditionTrue, "QuotaReconciled", "Resource limits successfully enforced")
	utils.SetCondition(&virtualCluster.Status.Conditions, "NetworkPolicyReady", metav1.ConditionTrue, "PolicyReconciled", fmt.Sprintf("Network mode applied: %s", virtualCluster.Spec.Network.Type))
	utils.SetCondition(&virtualCluster.Status.Conditions, "Ready", metav1.ConditionTrue, "ClusterReady", "VirtualCluster is fully provisioned and operational")

	return r.Status().Update(ctx, virtualCluster)
}

func (r *VirtualClusterReconciler) updateStatusFailed(ctx context.Context, virtualCluster *labv1alpha1.VirtualCluster, reason, message string) {
	virtualCluster.Status.Phase = "Failed"
	utils.SetCondition(&virtualCluster.Status.Conditions, "Ready", metav1.ConditionFalse, reason, message)
	_ = r.Status().Update(ctx, virtualCluster)
}

// ============================================================================
// CÁC KÊNH GIÁM SÁT THỜI GIAN THỰC (Real-time Watch Setup)
// ============================================================================

func (r *VirtualClusterReconciler) findParentVirtualCluster(ctx context.Context, obj client.Object) []reconcile.Request {
	virtualInstance, ok := obj.(*labv1alpha1.VirtualInstance)
	if !ok || virtualInstance.Spec.VirtualClusterRef == "" {
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      virtualInstance.Spec.VirtualClusterRef,
				Namespace: virtualInstance.Namespace,
			},
		},
	}
}

func (r *VirtualClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&labv1alpha1.VirtualCluster{}).
		Watches(&labv1alpha1.VirtualInstance{}, handler.EnqueueRequestsFromMapFunc(r.findParentVirtualCluster)).
		Named("virtualcluster").
		Complete(r)
}
