package utils

import (
	"crypto/sha1"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ============================================================================
// 1. CÁC HẰNG SỐ CẤU HÌNH (Constants)
// ============================================================================

const (
	// VirtualClusterFinalizer là thẻ cẩu bảo vệ VirtualCluster khỏi việc bị bốc hơi trực tiếp khỏi Database
	// Đảm bảo Reconciler có thời gian tiêu hủy hạ tầng Namespace và workload bên trong
	VirtualClusterFinalizer = "lab.devops.toiyeuptit.com/virtualcluster-finalizer"

	// VirtualInstanceFinalizer bảo vệ VirtualInstance trong các kịch bản cần dọn dẹp đặc thù
	VirtualInstanceFinalizer = "lab.devops.toiyeuptit.com/virtualinstance-finalizer"

	// Các nhãn tiêu chuẩn của K8s để định danh đối tượng được sinh ra từ Operator này
	LabelManagedBy       = "app.kubernetes.io/managed-by"
	LabelValueManagedBy  = "typ-lab-operator"
	LabelVirtualCluster  = "lab.devops.toiyeuptit.com/virtualcluster"
	LabelVirtualInstance = "lab.devops.toiyeuptit.com/virtualinstance"

	// DefaultDomain là tên miền mặc định nếu người quản trị không truyền domain khác cho Ingress
	DefaultDomain = "lab.toiyeuptit.com"
)

// ============================================================================
// 2. KIỂM SOÁT ĐỘ DÀI VÀ HASH CHUỖI (RFC 1123 / Max 63 Ký tự)
// ============================================================================

// SanitizeName kiểm tra độ dài chuỗi tên. Nếu độ dài vượt quá maxLen (ví dụ 63 ký tự theo chuẩn K8s DNS Label),
// hàm sẽ cắt ngắn chuỗi gốc và đính kèm 7 ký tự đầu của chuỗi băm SHA-1 (hash).
func SanitizeName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}

	hash := sha1.Sum([]byte(name))
	hashStr := fmt.Sprintf("%x", hash)[:7]

	allowedPrefixLen := maxLen - 8
	if allowedPrefixLen <= 0 {
		return hashStr
	}

	prefix := strings.TrimRight(name[:allowedPrefixLen], "-.")

	return fmt.Sprintf("%s-%s", prefix, hashStr)
}

// GenerateTargetNamespace sinh tên Namespace thực tế cho một VirtualCluster.
// Đảm bảo không quá 63 ký tự chuẩn RFC 1123.
func GenerateTargetNamespace(virtualClusterName string) string {
	rawName := fmt.Sprintf("lab-%s", virtualClusterName)
	return SanitizeName(rawName, 63)
}

// GenerateIngressHost sinh tên miền truy cập mượt mà cho các ứng dụng thực tập của sinh viên.
func GenerateIngressHost(virtualInstanceName, virtualClusterName string, domain string) string {
	if domain == "" {
		domain = DefaultDomain
	}

	subdomainRaw := fmt.Sprintf("%s-%s", virtualInstanceName, virtualClusterName)
	subdomain := SanitizeName(strings.ToLower(subdomainRaw), 63)

	return fmt.Sprintf("%s.%s", subdomain, domain)
}

// ============================================================================
// 3. QUY TRÌNH HỖ TRỢ STORAGE, INGRESS & STATUS CONDITIONS
// ============================================================================

func GetStorageClassName(volumeType string) string {
	switch strings.ToLower(volumeType) {
	case "local":
		if envVal := os.Getenv("DEFAULT_LOCAL_STORAGE_CLASS"); envVal != "" {
			return envVal
		}
		return "local-path"
	case "network":
		if envVal := os.Getenv("DEFAULT_NETWORK_STORAGE_CLASS"); envVal != "" {
			return envVal
		}
		return "longhorn"
	case "":
		return ""
	default:
		return volumeType
	}
}

func GetIngressClassName() string {
	if envVal := os.Getenv("DEFAULT_INGRESS_CLASS"); envVal != "" {
		return envVal
	}
	return "nginx"
}

func SetCondition(conditions *[]metav1.Condition, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}
