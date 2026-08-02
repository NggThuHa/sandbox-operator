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

// VirtualInstanceResourceLimit định nghĩa CPU/Memory request và limit
type VirtualInstanceResourceLimit struct {
	Request resource.Quantity `json:"request,omitempty"`
	Limit   resource.Quantity `json:"limit,omitempty"`
}

// VirtualInstanceResources gom nhóm tài nguyên tính toán của VirtualInstance
type VirtualInstanceResources struct {
	CPU    VirtualInstanceResourceLimit `json:"cpu,omitempty"`
	Memory VirtualInstanceResourceLimit `json:"memory,omitempty"`
}

// VirtualInstanceRootVolume định nghĩa ổ đĩa chứa hệ điều hành của Sysbox
type VirtualInstanceRootVolume struct {
	Size resource.Quantity `json:"size"`
	// +kubebuilder:validation:Enum=local;network
	Type string `json:"type"`
}

// VirtualInstanceDataVolume định nghĩa các ổ đĩa mount thêm vào (như /var/lib/docker)
type VirtualInstanceDataVolume struct {
	Name      string            `json:"name"`
	MountPath string            `json:"mountPath"`
	Size      resource.Quantity `json:"size"`
	// +kubebuilder:validation:Enum=local;network
	Type string `json:"type"`
}

// VirtualInstanceStorage gom cấu hình lưu trữ
type VirtualInstanceStorage struct {
	Root        VirtualInstanceRootVolume   `json:"root"`
	DataVolumes []VirtualInstanceDataVolume `json:"dataVolumes,omitempty"`
}

// VirtualInstancePort định nghĩa các cổng mạng cần mở
type VirtualInstancePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Expose     bool   `json:"expose"` // Nếu true -> Operator tự động tạo Ingress
}

// VirtualInstanceSpec định nghĩa cấu hình của một máy ảo
type VirtualInstanceSpec struct {
	VirtualClusterRef string                   `json:"virtualClusterRef"`
	Image             string                   `json:"image"`
	RuntimeClassName  string                   `json:"runtimeClassName,omitempty"`
	Resources         VirtualInstanceResources `json:"resources,omitempty"`
	Storage           VirtualInstanceStorage   `json:"storage,omitempty"`
	Ports             []VirtualInstancePort    `json:"ports,omitempty"`
}

// ============================================================================
// 2. STATUS STRUCTS (Trạng thái thực tế trả về cho UI)
// ============================================================================

// VirtualInstanceAccessEndpoint chứa thông tin URL để sinh viên bấm vào truy cập ngay
type VirtualInstanceAccessEndpoint struct {
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	URL             string `json:"url,omitempty"`   // URL external qua Ingress (nếu Expose=true)
	InternalAddress string `json:"internalAddress"` // Địa chỉ dùng để gọi nội bộ trong VirtualCluster
}

// VirtualInstanceVolumeStatusDetail chứa trạng thái của từng PVC
type VirtualInstanceVolumeStatusDetail struct {
	Name    string `json:"name,omitempty"` // Tên volume (trống nếu là RootVolume)
	PVCName string `json:"pvcName"`
	// +kubebuilder:validation:Enum=Pending;Bound;Lost;Failed
	Status string `json:"status"`
}

// VirtualInstanceVolumesStatus gom nhóm trạng thái toàn bộ ổ đĩa của máy ảo
type VirtualInstanceVolumesStatus struct {
	RootVolume  VirtualInstanceVolumeStatusDetail   `json:"rootVolume,omitempty"`
	DataVolumes []VirtualInstanceVolumeStatusDetail `json:"dataVolumes,omitempty"`
}

// VirtualInstanceStatus định nghĩa trạng thái quan sát được của VirtualInstance
type VirtualInstanceStatus struct {
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Stopped;Failed
	Phase string `json:"phase,omitempty"`

	// Tên Pod và IP thực tế dưới K8s
	PodName string `json:"podName,omitempty"`
	PodIP   string `json:"podIP,omitempty"`

	// Danh sách các link/endpoint cho UI hiển thị
	AccessEndpoints []VirtualInstanceAccessEndpoint `json:"accessEndpoints,omitempty"`

	// Trạng thái đính kèm của các ổ đĩa
	VolumesStatus VirtualInstanceVolumesStatus `json:"volumesStatus,omitempty"`

	// Chuẩn báo cáo điều kiện K8s
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
