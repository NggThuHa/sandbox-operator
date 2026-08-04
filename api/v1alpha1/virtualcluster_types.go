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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ============================================================================
// 1. SPEC STRUCTS (Trạng thái mong muốn từ người dùng)
// ============================================================================

// VirtualClusterNetworkConfig định nghĩa chế độ mạng của toàn bộ VirtualCluster
type VirtualClusterNetworkConfig struct {
	// Type có thể là: isolate, internal, hoặc external
	// +kubebuilder:validation:Enum=isolate;internal;external
	Type string `json:"type"`
}

// VirtualClusterStorageQuota định nghĩa giới hạn dung lượng ephemeral storage (emptyDir) của toàn VirtualCluster
// Được map tới K8s ResourceQuota field: limits.ephemeral-storage
type VirtualClusterStorageQuota struct {
	Limit resource.Quantity `json:"limit"`
}

// VirtualClusterResourceLimit định nghĩa cấu trúc có chứa trường limit cho CPU/Memory
type VirtualClusterResourceLimit struct {
	Limit resource.Quantity `json:"limit"`
}

// VirtualClusterComputeQuota định nghĩa giới hạn tài nguyên tính toán
type VirtualClusterComputeQuota struct {
	CPU    VirtualClusterResourceLimit `json:"cpu"`
	Memory VirtualClusterResourceLimit `json:"memory"`
}

// VirtualClusterObjectsQuota định nghĩa giới hạn số lượng tài nguyên K8s
type VirtualClusterObjectsQuota struct {
	PodsLimit     int32 `json:"podsLimit"`
	ServicesLimit int32 `json:"servicesLimit"`
}

// VirtualClusterQuotaConfig gom nhóm toàn bộ các giới hạn tài nguyên
type VirtualClusterQuotaConfig struct {
	Storage VirtualClusterStorageQuota `json:"storage"`
	Compute VirtualClusterComputeQuota `json:"compute"`
	Objects VirtualClusterObjectsQuota `json:"objects"`
}

// VirtualClusterSpec định nghĩa các thông số cấu hình mong muốn cho VirtualCluster
type VirtualClusterSpec struct {
	// TTL định nghĩa thời gian sống của cụm Lab, sau thời gian này toàn bộ cụm và VirtualInstance bên trong sẽ bị thu hồi (vd: "4h", "120m")
	// +optional
	TTL     string                      `json:"ttl,omitempty"`
	Network VirtualClusterNetworkConfig `json:"network"`
	Quota   VirtualClusterQuotaConfig   `json:"quota"`
}

// ============================================================================
// 2. STATUS STRUCTS (Trạng thái thực tế do Operator tổng hợp trả về)
// ============================================================================

// VirtualClusterInstanceCount thống kê số lượng máy ảo trong cụm
type VirtualClusterInstanceCount struct {
	Total int32 `json:"total"`
	Ready int32 `json:"ready"`
}

// VirtualClusterStorageUsage thống kê ephemeral storage đã sử dụng trong VirtualCluster
type VirtualClusterStorageUsage struct {
	Used resource.Quantity `json:"used,omitempty"`
}

// VirtualClusterComputeUsage thống kê CPU và RAM đã sử dụng
type VirtualClusterComputeUsage struct {
	CPUUsed    resource.Quantity `json:"cpuUsed,omitempty"`
	MemoryUsed resource.Quantity `json:"memoryUsed,omitempty"`
}

// VirtualClusterObjectsUsage thống kê số lượng Pod/Service đã tạo
type VirtualClusterObjectsUsage struct {
	PodsUsed     int32 `json:"podsUsed,omitempty"`
	ServicesUsed int32 `json:"servicesUsed,omitempty"`
}

// VirtualClusterQuotaUsage gom nhóm thống kê sử dụng thực tế (dùng cho UI hiển thị %)
type VirtualClusterQuotaUsage struct {
	Compute VirtualClusterComputeUsage `json:"compute,omitempty"`
	Storage VirtualClusterStorageUsage `json:"storage,omitempty"`
	Objects VirtualClusterObjectsUsage `json:"objects,omitempty"`
}

// VirtualClusterStatus định nghĩa trạng thái quan sát được của cụm
type VirtualClusterStatus struct {
	// Phase thể hiện trạng thái tổng thể của cụm
	// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Degraded;Failed;Terminating
	Phase string `json:"phase,omitempty"`

	// TargetNamespace là tên Namespace thực tế được cấp phát trên K8s
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// ExpiresAt là thời điểm chính xác cụm Lab này sẽ hết hạn và bị tự động xóa bỏ
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// InstanceCount đếm số máy ảo hiện tại
	InstanceCount VirtualClusterInstanceCount `json:"instanceCount,omitempty"`

	// QuotaUsage thống kê real-time tài nguyên
	QuotaUsage VirtualClusterQuotaUsage `json:"quotaUsage,omitempty"`

	// Conditions dùng chuẩn metav1.Condition của K8s để báo cáo trạng thái chi tiết
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ============================================================================
// 3. ROOT OBJECTS (Khai báo CRD với Kubernetes)
// ============================================================================

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The phase of the VirtualCluster"
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".status.targetNamespace",description="The target k8s namespace"
// +kubebuilder:printcolumn:name="Expires At",type="string",JSONPath=".status.expiresAt",description="Expiration timestamp of the lab cluster"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualCluster is the Schema for the virtualclusters API
type VirtualCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualClusterSpec   `json:"spec,omitempty"`
	Status VirtualClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualClusterList contains a list of VirtualCluster
type VirtualClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &VirtualCluster{}, &VirtualClusterList{})
		return nil
	})
}
