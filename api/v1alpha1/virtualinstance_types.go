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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ============================================================================
// 1. SPEC STRUCTS (Trạng thái mong muốn từ người dùng)
// ============================================================================

// VirtualInstanceResourceLimit định nghĩa CPU/Memory request và limit
type VirtualInstanceResourceLimit struct {
	Request resource.Quantity `json:"request,omitempty"`
	Limit   resource.Quantity `json:"limit,omitempty"`
}

// VirtualInstanceStorageLimit định nghĩa giới hạn disk của container.
// Chỉ có Limit — không có Request vì ephemeral disk không cần scheduler reservation.
// Map sang Pod resources.limits.ephemeral-storage → kubelet evict pod nếu vượt quá.
type VirtualInstanceStorageLimit struct {
	Limit resource.Quantity `json:"limit,omitempty"`
}

// VirtualInstanceResources gom nhóm tài nguyên tính toán của VirtualInstance
type VirtualInstanceResources struct {
	CPU    VirtualInstanceResourceLimit `json:"cpu,omitempty"`
	Memory VirtualInstanceResourceLimit `json:"memory,omitempty"`
	// Storage giới hạn tổng disk container (writable layer + logs + /var/lib/docker).
	// Disk của container hoàn toàn ephemeral — xóa Pod là mất data.
	// +optional
	Storage VirtualInstanceStorageLimit `json:"storage,omitempty"`
}

// VirtualInstancePort định nghĩa các cổng mạng cần mở
type VirtualInstancePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Expose     bool   `json:"expose"` // Nếu true -> Operator tự động tạo Ingress HTTPS
}

// VirtualInstanceSpec định nghĩa cấu hình của một máy ảo Sysbox
type VirtualInstanceSpec struct {
	// VirtualClusterRef là tên VirtualCluster cha — Operator sẽ tạo Pod trong namespace của cụm đó
	VirtualClusterRef string `json:"virtualClusterRef"`

	// Image của container (mặc định: ubuntu:24.04)
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullSecrets để pull image từ private registry
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Hostname tùy chỉnh bên trong máy ảo (vd: "controlplane", "worker1")
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Resources định nghĩa giới hạn CPU, Memory và Ephemeral Storage
	// +optional
	Resources VirtualInstanceResources `json:"resources,omitempty"`

	// Ports khai báo các cổng mạng cần expose
	// +optional
	Ports []VirtualInstancePort `json:"ports,omitempty"`
}

// ============================================================================
// 2. STATUS STRUCTS (Trạng thái thực tế trả về cho UI)
// ============================================================================

// VirtualInstanceAccessEndpoint chứa thông tin URL để sinh viên bấm vào truy cập ngay
type VirtualInstanceAccessEndpoint struct {
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	URL             string `json:"url,omitempty"`             // URL external qua Ingress (nếu Expose=true)
	InternalAddress string `json:"internalAddress,omitempty"` // Địa chỉ gọi nội bộ trong VirtualCluster
}

// VirtualInstanceStatus định nghĩa trạng thái quan sát được của VirtualInstance
type VirtualInstanceStatus struct {
	// Phase thể hiện trạng thái máy ảo
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Stopped;Failed
	Phase string `json:"phase,omitempty"`

	// PodName và PodIP thực tế dưới K8s
	PodName string `json:"podName,omitempty"`
	PodIP   string `json:"podIP,omitempty"`

	// AccessEndpoints là danh sách link/endpoint cho UI hiển thị nút bấm kết nối
	AccessEndpoints []VirtualInstanceAccessEndpoint `json:"accessEndpoints,omitempty"`

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
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The phase of the VirtualInstance"
// +kubebuilder:printcolumn:name="VirtualCluster",type="string",JSONPath=".spec.virtualClusterRef",description="The VirtualCluster this instance belongs to"
// +kubebuilder:printcolumn:name="Pod IP",type="string",JSONPath=".status.podIP",description="The internal IP of the pod"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualInstance is the Schema for the virtualinstances API
type VirtualInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualInstanceSpec   `json:"spec,omitempty"`
	Status VirtualInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualInstanceList contains a list of VirtualInstance
type VirtualInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &VirtualInstance{}, &VirtualInstanceList{})
		return nil
	})
}
