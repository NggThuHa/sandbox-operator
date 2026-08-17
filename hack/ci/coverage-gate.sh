#!/usr/bin/env bash
#
# Purpose | Mục đích
#   EN: Report total statement coverage from a Go cover profile and fail when it
#       drops below an explicit floor. Runs identically on a laptop and in CI,
#       so the gate can be reproduced locally with `make coverage-gate`.
#   VI: Báo cáo tổng coverage từ cover profile của Go và fail khi tụt xuống dưới
#       ngưỡng sàn đã khai báo. Chạy giống nhau ở máy cá nhân và trên CI, nên có
#       thể tái hiện gate bằng `make coverage-gate`.
#
# Usage | Cách dùng:
#   coverage-gate.sh [COVER_PROFILE]
#
#   COVER_PROFILE   Path to the profile written by `go test -coverprofile`.
#                   Default: cover.out
#
# Environment | Biến môi trường:
#   COVERAGE_MIN         Minimum total coverage in percent. Empty or unset turns
#                        the gate into a report-only step.
#   GITHUB_STEP_SUMMARY  When set (on GitHub Actions), the per-package table is
#                        appended there as well.
#
# Why a floor and not "must increase" | Vì sao dùng sàn thay vì "phải tăng":
#   EN: A ratchet needs the base branch profile, which means running the whole
#       suite twice. The floor is cheap, deterministic, and Codecov already
#       reports the relative delta on pull requests (see .github/codecov.yml).
#   VI: Kiểu "ratchet" cần profile của nhánh gốc, tức phải chạy toàn bộ test hai
#       lần. Sàn thì rẻ, tất định, và Codecov đã báo phần chênh lệch tương đối
#       trên pull request (xem .github/codecov.yml).

set -euo pipefail

profile="${1:-cover.out}"
coverage_min="${COVERAGE_MIN:-}"

if [[ ! -s "$profile" ]]; then
  echo "::error::Cover profile '${profile}' is missing or empty. Run 'make test' first." >&2
  exit 1
fi

# `go tool cover -func` ends with a "total:\t(statements)\t26.3%" line.
func_report="$(go tool cover -func="$profile")"
total_line="$(printf '%s\n' "$func_report" | tail -n1)"
total="$(printf '%s\n' "$total_line" | grep -oE '[0-9]+\.[0-9]+' | tail -n1 || true)"

if [[ -z "$total" ]]; then
  echo "::error::Could not parse the total coverage out of: ${total_line}" >&2
  exit 1
fi

# Per-package rollup: the func report is one line per function, which is too
# noisy for a summary. Fold it into "package -> covered/total statements".
package_table="$(
  printf '%s\n' "$func_report" \
    | awk -F'\t+' '
        $1 ~ /:[0-9]+:$/ {
          # $1 looks like "internal/controller/foo.go:120:", strip file+line to
          # get the directory, which is the package path.
          path = $1
          sub(/\/[^\/]*:[0-9]+:$/, "", path)
          pct = $NF
          sub(/%$/, "", pct)
          sum[path] += pct
          n[path] += 1
        }
        END {
          for (p in sum) printf "%s\t%.1f%%\n", p, sum[p] / n[p]
        }' \
    | sort
)"

echo "Total coverage: ${total}%"
echo
printf 'Per-package (mean of function coverage):\n%s\n' "$package_table"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## 🧪 Coverage"
    echo
    echo "**Total: \`${total}%\`** — floor: \`${coverage_min:-none}\`"
    echo
    echo "| Package | Coverage |"
    echo "| --- | --- |"
    printf '%s\n' "$package_table" | awk -F'\t' 'NF==2 {printf "| `%s` | %s |\n", $1, $2}'
  } >> "$GITHUB_STEP_SUMMARY"
fi

if [[ -z "$coverage_min" ]]; then
  echo "COVERAGE_MIN is not set; reporting only."
  exit 0
fi

# awk instead of bc: bc is not installed on every runner image.
if awk -v total="$total" -v min="$coverage_min" 'BEGIN { exit !(total + 0 < min + 0) }'; then
  echo "::error::Total coverage ${total}% is below the ${coverage_min}% floor." >&2
  echo "Add tests, or lower the floor with a note explaining why: COVERAGE_MIN in" \
       "the Makefile (local runs) and the coverage-min input in" \
       ".github/workflows/ci.yml and release.yml (CI)." >&2
  exit 1
fi

echo "Coverage gate passed (${total}% >= ${coverage_min}%)."
