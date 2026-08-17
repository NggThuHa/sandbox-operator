#!/usr/bin/env bash
#
# Purpose | Mục đích
#   EN: Decide which areas of the repo a change touches, so CI can skip whole
#       job groups instead of running everything on every push.
#   VI: Xác định thay đổi chạm vào những khu vực nào của repo, để CI bỏ qua cả
#       nhóm job thay vì chạy tất cả mọi lần push.
#
# Usage | Cách dùng:
#   detect-changes.sh [BASE_REF]
#
#   BASE_REF   Commit/ref to compare against. Pass an empty string (or omit it)
#              to force every area on — that is the safe default for
#              workflow_dispatch, schedules and brand-new branches.
#
# Output (stdout): one `key=value` line per area, ready to be appended to
# $GITHUB_OUTPUT:
#   go=true
#   images=false
#   ansible=false
#   terraform=false
#
# Fail-open policy | Nguyên tắc fail-open:
#   EN: If the base commit is unreachable (shallow clone, force-push, deleted
#       ref) we enable every area. A false negative here silently skips tests;
#       a false positive only costs runner minutes.
#   VI: Nếu không tìm được commit gốc (clone shallow, force-push, ref đã xoá) thì
#       bật tất cả. Bỏ sót test là nguy hiểm, chạy thừa chỉ tốn phút runner.

set -euo pipefail

compare_ref="${1-}"

log() { printf '%s\n' "$*" >&2; }

# Areas are matched by path prefix (directories keep their trailing slash) or by
# exact file name. Keep these lists in sync with the reusable workflows in
# .github/workflows/.
#
# NOTE | LƯU Ý: images/script/ is Go code compiled into the lab images, so it
# belongs to BOTH the go area (it is part of `go list ./...`) and the images area.
readonly GO_PATHS=(
  "api/" "cmd/" "internal/" "test/" "config/" "hack/" "images/script/"
  "go.mod" "go.sum" "Makefile" "Dockerfile" ".dockerignore"
  ".golangci.yml" ".custom-gcl.yml" "PROJECT"
)
readonly IMAGE_PATHS=("images/")
readonly ANSIBLE_PATHS=("infra/ansible/")
readonly TERRAFORM_PATHS=("infra/terraform/")

# Anything under .github/ can change how every other job runs, so a CI change
# re-runs the full matrix instead of trusting the previous green run.
readonly CI_PATHS=(".github/")

go=false
images=false
ansible=false
terraform=false

emit() {
  printf 'go=%s\nimages=%s\nansible=%s\nterraform=%s\n' \
    "$go" "$images" "$ansible" "$terraform"
}

enable_all() {
  go=true
  images=true
  ansible=true
  terraform=true
}

# matches_any <file> <prefix-or-name>...
matches_any() {
  local file="$1"
  shift
  local pattern
  for pattern in "$@"; do
    case "$pattern" in
      */) [[ "$file" == "$pattern"* ]] && return 0 ;;
      *) [[ "$file" == "$pattern" ]] && return 0 ;;
    esac
  done
  return 1
}

main() {
  if [[ -z "$compare_ref" ]]; then
    log "No compare ref supplied; enabling every area."
    enable_all
    emit
    return
  fi

  # An all-zero SHA is what GitHub sends in github.event.before for the first
  # push to a new branch.
  if [[ "$compare_ref" =~ ^0{40}$ ]]; then
    log "Compare ref is the null SHA (new branch); enabling every area."
    enable_all
    emit
    return
  fi

  # Compare against the merge base so a PR is judged on its own commits only,
  # not on whatever landed on the target branch in the meantime.
  local base
  if ! base="$(git merge-base "$compare_ref" HEAD 2>/dev/null)"; then
    log "::warning::Cannot reach ${compare_ref} from HEAD; enabling every area."
    enable_all
    emit
    return
  fi

  local changed
  changed="$(git diff --name-only "$base" HEAD)"

  if [[ -z "$changed" ]]; then
    log "No file changes between ${base} and HEAD."
    emit
    return
  fi

  local file
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue

    if matches_any "$file" "${CI_PATHS[@]}"; then
      log "CI definition changed (${file}); enabling every area."
      enable_all
      break
    fi

    matches_any "$file" "${GO_PATHS[@]}" && go=true
    matches_any "$file" "${IMAGE_PATHS[@]}" && images=true
    matches_any "$file" "${ANSIBLE_PATHS[@]}" && ansible=true
    matches_any "$file" "${TERRAFORM_PATHS[@]}" && terraform=true
  done <<< "$changed"

  log "Areas: go=${go} images=${images} ansible=${ansible} terraform=${terraform}"
  emit
}

main
