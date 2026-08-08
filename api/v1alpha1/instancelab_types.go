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

// InstanceLabResourceLimit định nghĩa CPU/Memory request và limit
type InstanceLabResourceLimit struct {
	Request resource.Quantity `json:"request,omitempty"`
	Limit   resource.Quantity `json:"limit,omitempty"`
}

// InstanceLabStorageLimit định nghĩa giới hạn disk của container.
// Chỉ có Limit — không có Request vì ephemeral disk không cần scheduler reservation.
// Map sang Pod resources.limits.ephemeral-storage → kubelet evict pod nếu vượt quá.
type InstanceLabStorageLimit struct {
	Limit resource.Quantity `json:"limit,omitempty"`
}

// InstanceLabResources gom nhóm tài nguyên tính toán của InstanceLab
type InstanceLabResources struct {
	CPU    InstanceLabResourceLimit `json:"cpu,omitempty"`
	Memory InstanceLabResourceLimit `json:"memory,omitempty"`
	// Storage giới hạn tổng disk container (writable layer + logs + /var/lib/docker).
	// Disk của container hoàn toàn ephemeral — xóa Pod là mất data.
	// +optional
	Storage InstanceLabStorageLimit `json:"storage,omitempty"`
}

// InstanceLabPort định nghĩa các cổng mạng cần mở
type InstanceLabPort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Expose     bool   `json:"expose"` // Nếu true -> Operator tự động tạo Ingress HTTPS
}

// InstanceLabSpec định nghĩa cấu hình của một máy ảo Sysbox
type InstanceLabSpec struct {
	// ClusterLabRef là tên ClusterLab cha — Operator sẽ tạo Pod trong namespace của cụm đó
	ClusterLabRef string `json:"clusterLabRef"`

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
	Resources InstanceLabResources `json:"resources,omitempty"`

	// Ports khai báo các cổng mạng cần expose
	// +optional
	Ports []InstanceLabPort `json:"ports,omitempty"`
}

// ============================================================================
// 2. STATUS STRUCTS (Trạng thái thực tế trả về cho UI)
// ============================================================================

// InstanceLabAccessEndpoint chứa thông tin URL để sinh viên bấm vào truy cập ngay
type InstanceLabAccessEndpoint struct {
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	URL             string `json:"url,omitempty"`             // URL external qua Ingress (nếu Expose=true)
	InternalAddress string `json:"internalAddress,omitempty"` // Địa chỉ gọi nội bộ trong ClusterLab
}

// InstanceLabStatus định nghĩa trạng thái quan sát được của InstanceLab
type InstanceLabStatus struct {
	// Phase thể hiện trạng thái máy ảo
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Stopped;Failed
	Phase string `json:"phase,omitempty"`

	// PodName và PodIP thực tế dưới K8s
	PodName string `json:"podName,omitempty"`
	PodIP   string `json:"podIP,omitempty"`

	// AccessEndpoints là danh sách link/endpoint cho UI hiển thị nút bấm kết nối
	AccessEndpoints []InstanceLabAccessEndpoint `json:"accessEndpoints,omitempty"`

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
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The phase of the InstanceLab"
// +kubebuilder:printcolumn:name="ClusterLab",type="string",JSONPath=".spec.clusterLabRef",description="The ClusterLab this instance belongs to"
// +kubebuilder:printcolumn:name="Pod IP",type="string",JSONPath=".status.podIP",description="The internal IP of the pod"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// InstanceLab is the Schema for the instancelabs API
type InstanceLab struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceLabSpec   `json:"spec,omitempty"`
	Status InstanceLabStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceLabList contains a list of InstanceLab
type InstanceLabList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceLab `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &InstanceLab{}, &InstanceLabList{})
		return nil
	})
}
