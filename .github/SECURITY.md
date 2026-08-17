# Security Policy | Chính sách bảo mật

<!--
EN: This file is what GitHub shows behind the "Report a vulnerability" button and
    in the repository's Security tab.
VI: File này là nội dung GitHub hiển thị sau nút "Report a vulnerability" và ở tab
    Security của repo.
-->

## Supported versions | Phiên bản được hỗ trợ

**EN** — This is a pre-1.0 project: the CRD API is `v1alpha1` and only the latest
release plus `main` receive fixes. There are no backports to older tags.

**VI** — Đây là dự án trước 1.0: API của CRD là `v1alpha1`, và chỉ bản phát hành
mới nhất cùng nhánh `main` được sửa lỗi. Không backport về các tag cũ.

| Version | Supported | Hỗ trợ |
| --- | --- | --- |
| `main` | ✅ | Đang phát triển |
| latest `v*` release | ✅ | Bản phát hành hiện hành |
| older tags | ❌ | Nâng lên bản mới nhất |

## Reporting a vulnerability | Báo cáo lỗ hổng

**EN** — Use
[GitHub's private vulnerability reporting](https://github.com/ngtukien/sandbox-operator/security/advisories/new).
Do **not** open a public issue, and do not attach a working exploit against a
cluster you do not own. Expect a first response within **7 days**; a fix for a
confirmed sandbox-escape or cross-lab access issue is prioritised over every
feature in flight.

**VI** — Hãy dùng
[cơ chế báo cáo riêng tư của GitHub](https://github.com/ngtukien/sandbox-operator/security/advisories/new).
**Đừng** mở issue công khai, và đừng kèm exploit đang hoạt động nhắm vào cụm
không thuộc quyền sở hữu của bạn. Bạn sẽ nhận phản hồi đầu tiên trong vòng
**7 ngày**; bản sửa cho lỗi thoát sandbox hoặc truy cập chéo giữa các lab đã được
xác nhận sẽ được ưu tiên trên mọi tính năng đang làm.

Please include, as far as you can | Vui lòng kèm theo, trong khả năng của bạn:

- The CRs you applied (`ClusterLab` / `InstanceLab`), with secrets redacted.
  Các CR bạn đã apply (`ClusterLab` / `InstanceLab`), đã che thông tin bí mật.
- Kubernetes distribution and version, Sysbox version, RuntimeClass in use.
  Bản phân phối và phiên bản Kubernetes, phiên bản Sysbox, RuntimeClass đang dùng.
- The operator image tag or commit SHA. | Tag image hoặc commit SHA của operator.
- What an attacker gains — data, credentials, host access.
  Kẻ tấn công đạt được gì — dữ liệu, thông tin đăng nhập, quyền trên máy chủ.

## In scope | Trong phạm vi

**EN** — This platform's core promise is that a student inside a lab container
cannot reach the host, another lab, or the cluster's control plane. Anything that
breaks that promise is a vulnerability here:

**VI** — Cam kết cốt lõi của nền tảng này là một sinh viên bên trong container lab
không thể chạm tới máy chủ, lab khác, hay control plane của cụm. Bất cứ điều gì
phá vỡ cam kết đó đều là lỗ hổng:

- Escaping an `InstanceLab` container to the node (Sysbox/`sysbox-runc` bypass,
  an unintended `privileged`, `hostPath` or capability grant in the generated Pod).
  Thoát khỏi container `InstanceLab` ra node (vượt Sysbox/`sysbox-runc`, hoặc Pod
  sinh ra vô tình có `privileged`, `hostPath`, capability không mong muốn).
- Reaching another `ClusterLab`'s namespace, Pods or PVCs — the ingress-blocking
  `NetworkPolicy` or the namespace isolation failing to apply.
  Chạm tới namespace, Pod hay PVC của một `ClusterLab` khác — `NetworkPolicy` chặn
  ingress hoặc việc cách ly namespace không được áp dụng.
- Privilege escalation through the operator's ServiceAccount: RBAC in
  `config/rbac/` granting more than the controller needs, or a reconcile path that
  lets a user-supplied CR field create an arbitrary cluster-scoped object.
  Leo thang đặc quyền qua ServiceAccount của operator: RBAC trong `config/rbac/`
  cấp nhiều hơn mức controller cần, hoặc một nhánh reconcile cho phép trường do
  người dùng cung cấp trong CR tạo ra object phạm vi cụm bất kỳ.
- Escaping the `ResourceQuota` or TTL cleanup so one lab can starve the cluster.
  Vượt `ResourceQuota` hoặc cơ chế dọn dẹp theo TTL, khiến một lab có thể làm cạn
  tài nguyên của cả cụm.
- Secrets leaking into logs, CR status, generated manifests or `dist/install.yaml`
  — including a kubeconfig or Terraform state committed by mistake.
  Thông tin bí mật lọt vào log, status của CR, manifest sinh ra hay
  `dist/install.yaml` — kể cả kubeconfig hoặc state của Terraform bị commit nhầm.
- A CVE in the operator's own dependency tree (`go.sum`) or in its
  distroless/nonroot image.
  CVE trong cây dependency của chính operator (`go.sum`) hoặc trong image
  distroless/nonroot của nó.

## Out of scope | Ngoài phạm vi

**EN** — The following are known, deliberate properties of a teaching lab, not
vulnerabilities. Report them as issues if they cause you trouble, but they will
not receive an advisory:

**VI** — Những điều sau là đặc tính đã biết và có chủ ý của một phòng lab dạy học,
không phải lỗ hổng. Nếu chúng gây khó cho bạn, hãy mở issue thường — nhưng chúng
sẽ không được cấp advisory:

- **Root and systemd inside a lab container.** That is the product: a system
  container is supposed to behave like a machine. Sysbox is what makes it safe.
  **Quyền root và systemd bên trong container lab.** Đó chính là tính năng: một
  system container phải hoạt động như một cái máy. Sysbox là thứ giữ cho nó an toàn.
- **Unrestricted egress from a lab.** Students need `apt install`, `git clone`
  and `docker pull`. Egress filtering is a deployment-time decision, not a bug.
  **Egress không giới hạn từ lab.** Sinh viên cần `apt install`, `git clone`,
  `docker pull`. Chặn egress là quyết định lúc triển khai, không phải lỗi.
- **OS CVEs in the `images/` lab base images.** They inherit a systemd/Sysbox base
  whose backlog is upstream; the scan is report-only for exactly this reason (see
  the gate policy in [`.github/workflows/_security.yml`](workflows/_security.yml)).
  **CVE hệ điều hành trong các lab base image ở `images/`.** Chúng kế thừa base
  systemd/Sysbox có tồn đọng thuộc upstream; đây chính là lý do lượt quét chỉ ở
  chế độ báo cáo (xem chính sách gate trong
  [`.github/workflows/_security.yml`](workflows/_security.yml)).
- **The host-obfuscation layer** (hiding `/sys/block`, wrapping `df`/`lsblk`) is
  anti-cheating comfort, not a security boundary. Seeing through it is expected.
  **Lớp che giấu phần cứng máy chủ** (ẩn `/sys/block`, bọc `df`/`lsblk`) là tiện
  ích chống gian lận, không phải ranh giới bảo mật. Xuyên qua được là điều dự kiến.
- Findings from a scanner with no demonstrated impact on this repository's code.
  Kết quả từ máy quét mà không chứng minh được ảnh hưởng tới mã của repo này.

## What runs automatically | Những gì đã được kiểm tra tự động

**EN** — Before reporting a dependency or misconfiguration finding, note that CI
already scans for it on every pull request and again every Monday at 03:17 UTC
(see [`_security.yml`](workflows/_security.yml) and
[`security-schedule.yml`](workflows/security-schedule.yml)):

**VI** — Trước khi báo một phát hiện về dependency hay sai cấu hình, hãy biết rằng
CI đã quét trên mọi pull request và quét lại mỗi thứ Hai 03:17 UTC (xem
[`_security.yml`](workflows/_security.yml) và
[`security-schedule.yml`](workflows/security-schedule.yml)):

| Scan | Target | Gate |
| --- | --- | --- |
| Trivy config | `config/`, `infra/terraform/`, `Dockerfile` | fails on CRITICAL + HIGH |
| Trivy config | `images/` | report only |
| Trivy fs (vuln + secret) | repository root | fails on CRITICAL |
| Trivy image | operator image | fails on CRITICAL + HIGH |

All results are uploaded as SARIF to the repository's Security tab. Suppressions
live in [`.trivyignore`](../.trivyignore) and each one carries a justification.

Mọi kết quả được upload dưới dạng SARIF lên tab Security của repo. Các mục bỏ qua
nằm trong [`.trivyignore`](../.trivyignore) và mỗi mục đều kèm lý do.

## Disclosure | Công bố

**EN** — Fix first, publish after: a patched release goes out, then the advisory
is published with credit to the reporter unless they ask to stay anonymous.

**VI** — Sửa trước, công bố sau: phát hành bản đã vá, rồi công bố advisory kèm ghi
nhận người báo cáo, trừ khi họ muốn ẩn danh.
