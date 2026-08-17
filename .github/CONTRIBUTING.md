# Contributing | Hướng dẫn đóng góp

**EN** — Thanks for wanting to help. This document is the short version of what CI
enforces, so you can find out on your laptop what would otherwise take a red pull
request to learn.

**VI** — Cảm ơn bạn đã muốn đóng góp. Tài liệu này là bản ngắn của những gì CI kiểm
tra, để bạn phát hiện vấn đề ngay trên máy mình thay vì phải chờ một pull request
đỏ mới biết.

---

## 1. Prerequisites | Yêu cầu môi trường

| Tool | Version | Needed for | Cần cho |
| --- | --- | --- | --- |
| Go | as in [`go.mod`](../go.mod) (1.26+) | everything Go | mọi thứ liên quan Go |
| `make` + `bash` | — | every target below | mọi target dưới đây |
| Docker | with Buildx | images, e2e, Molecule | image, e2e, Molecule |
| [Kind](https://kind.sigs.k8s.io/) | recent | `make test-e2e` | `make test-e2e` |
| Python 3 + `venv` | 3.10+ | `infra/ansible/` | `infra/ansible/` |
| Terraform | 1.5+ | `infra/terraform/` | `infra/terraform/` |

**EN** — Do **not** install controller-gen, kustomize, setup-envtest or
golangci-lint by hand. The Makefile pins their versions and installs them into
`bin/` on first use; a system-wide copy at a different version is the most common
cause of "it generates different files on my machine".

**VI** — **Đừng** tự cài controller-gen, kustomize, setup-envtest hay golangci-lint.
Makefile đã ghim phiên bản và tự cài vào `bin/` khi dùng lần đầu; một bản cài toàn
hệ thống ở phiên bản khác là nguyên nhân phổ biến nhất của kiểu lỗi "máy tôi sinh
ra file khác".

```bash
git clone https://github.com/ngtukien/sandbox-operator.git
cd sandbox-operator
make help          # every target, grouped | mọi target, đã phân nhóm
```

> **`vendor/` is not tracked.** Reproducibility comes from `go.sum`. Run
> `go mod vendor` locally only if you need an offline build, and never commit it.
>
> **`vendor/` không được theo dõi.** Khả năng tái lập đến từ `go.sum`. Chỉ chạy
> `go mod vendor` ở máy bạn khi cần build offline, và tuyệt đối không commit nó.

---

## 2. Never edit generated files | Không sửa file sinh tự động

**EN** — These are rewritten by `make manifests generate`, and the
`Go / verify` CI job fails the build if the committed copy does not match what the
generator produces. Edit the Go markers, then regenerate.

**VI** — Những file này do `make manifests generate` viết lại, và job CI
`Go / verify` sẽ fail nếu bản đã commit không khớp với đầu ra của generator. Hãy sửa
marker trong Go rồi sinh lại.

- `api/v1alpha1/zz_generated.*.go`
- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `PROJECT` (managed by the `kubebuilder` CLI | do CLI `kubebuilder` quản lý)

**EN** — Also: never delete a `// +kubebuilder:scaffold:*` comment. The CLI injects
code at those markers.

**VI** — Ngoài ra: đừng bao giờ xoá comment `// +kubebuilder:scaffold:*`. CLI chèn
mã tại chính các marker đó.

---

## 3. Run what CI runs | Chạy đúng những gì CI chạy

**EN** — CI decides which of these areas to run using
[`hack/ci/detect-changes.sh`](../hack/ci/detect-changes.sh), then calls the
matching reusable workflow. Every job has a local equivalent:

**VI** — CI dùng [`hack/ci/detect-changes.sh`](../hack/ci/detect-changes.sh) để
quyết định chạy khu vực nào, rồi gọi workflow tái sử dụng tương ứng. Mỗi job đều có
lệnh tương đương ở máy:

| CI job | Workflow | Lệnh tương đương ở máy \| Local equivalent |
| --- | --- | --- |
| `Go / verify` | [`_go-verify.yml`](workflows/_go-verify.yml) | `make manifests generate && git status --porcelain`, `go mod tidy`, `go mod verify` |
| `Go / lint` | [`_go-lint.yml`](workflows/_go-lint.yml) | `make lint-config lint` (auto-fix: `make lint-fix`) |
| `Go / unit tests` | [`_go-test.yml`](workflows/_go-test.yml) | `make test-coverage` |
| `Go / e2e tests` | [`_go-e2e.yml`](workflows/_go-e2e.yml) | `make test-e2e` (needs Kind \| cần Kind) |
| `Ansible` | [`_ansible.yml`](workflows/_ansible.yml) | see §3.3 \| xem §3.3 |
| `Terraform` | [`_terraform.yml`](workflows/_terraform.yml) | see §3.4 \| xem §3.4 |
| `Lab images` | [`_images.yml`](workflows/_images.yml) | `docker build -f images/Dockerfile.<name> ./images` |
| `Security` | [`_security.yml`](workflows/_security.yml) | see §3.5 \| xem §3.5 |

**EN** — Want to know what CI *would* run for your branch? Ask the same script:

**VI** — Muốn biết CI *sẽ* chạy gì cho nhánh của bạn? Hỏi đúng script đó:

```bash
hack/ci/detect-changes.sh origin/main
```

### 3.1. The usual Go loop | Vòng lặp Go thường ngày

```bash
# After editing api/**/*_types.go or any +kubebuilder marker
# Sau khi sửa api/**/*_types.go hoặc bất kỳ marker +kubebuilder
make manifests generate

# Before pushing | Trước khi push
make lint-fix          # gofmt + auto-fixable lint | sửa tự động được
make test-coverage     # unit tests + the coverage floor | unit test + ngưỡng coverage
```

### 3.2. Coverage floor | Ngưỡng sàn coverage

**EN** — `make test-coverage` runs the suite and then
[`hack/ci/coverage-gate.sh`](../hack/ci/coverage-gate.sh), the exact script CI
uses. The floor is `COVERAGE_MIN` in the [Makefile](../Makefile), mirrored by the
`coverage-min` input in [`ci.yml`](workflows/ci.yml) and
[`release.yml`](workflows/release.yml). Raise it when you add tests; lowering it
requires a sentence in the pull request explaining why.

**VI** — `make test-coverage` chạy bộ test rồi gọi
[`hack/ci/coverage-gate.sh`](../hack/ci/coverage-gate.sh) — đúng script CI dùng.
Ngưỡng là `COVERAGE_MIN` trong [Makefile](../Makefile), được phản chiếu ở input
`coverage-min` của [`ci.yml`](workflows/ci.yml) và
[`release.yml`](workflows/release.yml). Hãy nâng nó khi bạn thêm test; muốn hạ thì
phải có một câu giải thích trong pull request.

```bash
make coverage-gate COVERAGE_MIN=30   # try a higher floor | thử ngưỡng cao hơn
```

### 3.3. Ansible | Ansible

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install -r infra/ansible/requirements-dev.txt

cd infra/ansible
ansible-playbook cluster.yml -i "localhost," -e ansible_connection=local --syntax-check
ansible-playbook cluster.yml -i "localhost," -e ansible_connection=local \
  -e kubernetes_distro=k3s --syntax-check     # the other distro path | nhánh distro còn lại
ansible-lint cluster.yml
molecule test -s default                      # needs Docker | cần Docker
```

**EN** — `requirements-dev.txt` is unpinned on purpose (the file says why). If a
new `ansible-lint` release breaks the build, pin it there in the same pull request
that works around the finding.

**VI** — `requirements-dev.txt` cố ý không ghim phiên bản (lý do ghi trong file).
Nếu một bản `ansible-lint` mới làm vỡ build, hãy ghim ngay trong pull request xử lý
phát hiện đó.

### 3.4. Terraform | Terraform

```bash
cd infra/terraform
terraform fmt -check -recursive -diff
terraform init -backend=false -lockfile=readonly   # fails if the lockfile drifted
terraform validate
```

**EN** — `.terraform.lock.hcl` **is** tracked. If you change a provider constraint
in `versions.tf`, re-run `terraform init -upgrade` and commit the new lockfile.

**VI** — `.terraform.lock.hcl` **có** được theo dõi. Nếu bạn đổi ràng buộc provider
trong `versions.tf`, hãy chạy lại `terraform init -upgrade` và commit lockfile mới.

### 3.5. Security scans | Quét bảo mật

```bash
hack/ci/trivy-scan.sh --type config --target config --name iac-k8s-manifests \
  --gate on --severity CRITICAL,HIGH
hack/ci/trivy-scan.sh --type fs --target . --name deps-and-secrets \
  --scanners vuln,secret --gate on --severity CRITICAL
```

**EN** — The script downloads Trivy into `bin/` if it is not on your `PATH`, writes
SARIF plus a readable table into `trivy-results/`, and prints a suppression hint
when the gate fires. Suppressions go in [`.trivyignore`](../.trivyignore) **with a
comment saying why**; an unjustified entry will be rejected in review. The full
gate policy is documented at the top of [`_security.yml`](workflows/_security.yml).

**VI** — Script tự tải Trivy vào `bin/` nếu chưa có trong `PATH`, ghi SARIF kèm bảng
dễ đọc vào `trivy-results/`, và in gợi ý khi gate chặn. Các mục bỏ qua đặt trong
[`.trivyignore`](../.trivyignore) **kèm comment nêu lý do**; mục không có lý do sẽ
bị từ chối khi review. Chính sách gate đầy đủ được ghi ở đầu
[`_security.yml`](workflows/_security.yml).

---

## 4. Commits and pull requests | Commit và pull request

**EN** — Commits follow [Conventional Commits](https://www.conventionalcommits.org):
`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`, with an optional scope
(`feat(hami):`, `chore(deps):`). Releases are cut from tags, so a readable history
is what the generated release notes are made of.

**VI** — Commit theo [Conventional Commits](https://www.conventionalcommits.org):
`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`, kèm scope tuỳ chọn
(`feat(hami):`, `chore(deps):`). Bản phát hành được cắt từ tag, nên lịch sử dễ đọc
chính là nguyên liệu của release notes sinh tự động.

Then | Sau đó:

1. **Open the pull request against `main`** and fill in
   [the template](PULL_REQUEST_TEMPLATE.md) — the *Why* and *Verification*
   sections are the ones reviewers actually read.
   **Mở pull request vào `main`** và điền
   [template](PULL_REQUEST_TEMPLATE.md) — phần *Why* và *Verification* là phần
   người review thực sự đọc.
2. **Wait for `CI / All checks passed`.** That single check aggregates every job;
   jobs shown as *skipped* are change detection working as intended, not a problem.
   **Chờ `CI / All checks passed`.** Check duy nhất này tổng hợp mọi job; job hiện
   *skipped* là change detection hoạt động đúng ý đồ, không phải vấn đề.
3. **A review from [@ngtukien](https://github.com/ngtukien) is requested
   automatically** via [`CODEOWNERS`](CODEOWNERS).
   **Yêu cầu review tới [@ngtukien](https://github.com/ngtukien) được tạo tự động**
   qua [`CODEOWNERS`](CODEOWNERS).

**EN** — Changing an area's CI definition (anything under `.github/`) re-runs the
whole matrix on purpose: a CI change cannot be validated by the previous green run.

**VI** — Thay đổi định nghĩa CI của một khu vực (bất kỳ thứ gì dưới `.github/`) sẽ
chạy lại toàn bộ matrix một cách có chủ ý: một thay đổi CI không thể được xác nhận
bởi lượt chạy xanh trước đó.

### Dependency updates | Cập nhật dependency

**EN** — Dependabot opens grouped pull requests weekly
([`dependabot.yml`](dependabot.yml)); minor and patch updates are approved and
queued for auto-merge automatically, majors are left for a human
([`dependabot-auto-merge.yml`](workflows/dependabot-auto-merge.yml)). Tool versions
in the Makefile and `TRIVY_VERSION` are plain make variables — bump those by hand.

**VI** — Dependabot mở pull request theo nhóm mỗi tuần
([`dependabot.yml`](dependabot.yml)); bản minor và patch được tự approve và đưa vào
hàng đợi auto-merge, bản major để người tự xử lý
([`dependabot-auto-merge.yml`](workflows/dependabot-auto-merge.yml)). Phiên bản công
cụ trong Makefile và `TRIVY_VERSION` là biến make thuần — hãy nâng tay.

### Pinning actions | Ghim phiên bản action

**EN** — Every third-party action is pinned to a full commit SHA with the version in
a trailing comment. Keep that shape; Dependabot updates both parts together, and an
unpinned `@v4` in a diff will be flagged in review.

**VI** — Mọi action của bên thứ ba đều được ghim theo commit SHA đầy đủ, kèm phiên
bản trong comment cuối dòng. Hãy giữ đúng dạng đó; Dependabot cập nhật cả hai phần
cùng lúc, và một `@v4` chưa ghim xuất hiện trong diff sẽ bị nhắc khi review.

```yaml
uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
```

---

## 5. Adding things | Khi thêm mới

### A new lab image | Một lab image mới

**EN** — Add `images/Dockerfile.<name>`. Nothing else: the build matrix is derived
from the files on disk by
[`hack/ci/detect-changed-images.sh`](../hack/ci/detect-changed-images.sh), so the
image is built, scanned and (on `main`) pushed to
`ghcr.io/ngtukien/sandbox-operator/<name>` with no workflow edit. Changing a shared
input under `images/` (the `motd`, anything in `script/`) rebuilds every image,
which is intended.

**VI** — Thêm `images/Dockerfile.<name>`. Không cần gì khác: matrix build được suy
ra từ các file trên đĩa bởi
[`hack/ci/detect-changed-images.sh`](../hack/ci/detect-changed-images.sh), nên image
sẽ được build, quét và (trên `main`) push lên
`ghcr.io/ngtukien/sandbox-operator/<name>` mà không phải sửa workflow. Thay đổi một
đầu vào dùng chung trong `images/` (`motd`, bất cứ gì trong `script/`) sẽ build lại
mọi image — đó là hành vi mong muốn.

### A new API field | Một trường API mới

```bash
# 1. Edit api/v1alpha1/*_types.go with validation markers
#    Sửa api/v1alpha1/*_types.go kèm marker validation
# 2. Regenerate | Sinh lại
make manifests generate
# 3. Update the sample CR and cover the field in a test
#    Cập nhật CR mẫu và viết test cho trường mới
# 4. Say in the pull request what happens to existing objects on upgrade
#    Nói rõ trong pull request các object đang tồn tại sẽ thế nào khi nâng cấp
```

### A new CI area | Một khu vực CI mới

**EN** — Add a reusable `_<area>.yml` (only `on: workflow_call`), call it from
[`ci.yml`](workflows/ci.yml) behind a `changes` output, and teach
`detect-changes.sh` which paths belong to it. Job definitions live in exactly one
file so branch protection keeps needing only one required check.

**VI** — Thêm một workflow tái sử dụng `_<area>.yml` (chỉ `on: workflow_call`), gọi
nó từ [`ci.yml`](workflows/ci.yml) sau một output của `changes`, và dạy
`detect-changes.sh` biết những đường dẫn nào thuộc khu vực đó. Mỗi định nghĩa job
chỉ nằm ở một file để branch protection vẫn chỉ cần một check bắt buộc duy nhất.

---

## 6. Reporting problems | Báo cáo vấn đề

- Bug or feature: use the [issue templates](ISSUE_TEMPLATE) — they ask for the CR,
  the version and the distro because that is what makes a report reproducible.
  Lỗi hoặc tính năng: dùng [issue template](ISSUE_TEMPLATE) — chúng hỏi CR, phiên
  bản và bản phân phối vì đó là những gì giúp tái hiện được báo cáo.
- Anything that lets a lab escape its sandbox: **do not open an issue**, follow
  [`SECURITY.md`](SECURITY.md).
  Bất cứ điều gì cho phép một lab thoát khỏi sandbox: **đừng mở issue**, hãy làm
  theo [`SECURITY.md`](SECURITY.md).
