#!/usr/bin/env bash
#
# Purpose | Mục đích
#   EN: Run one Trivy scan and produce BOTH a SARIF file (for the GitHub
#       Security tab) and a human-readable table (for the job summary), then
#       optionally fail the job on high-severity findings.
#   VI: Chạy một lượt quét Trivy, xuất ra ĐỒNG THỜI file SARIF (cho tab Security
#       của GitHub) và bảng dễ đọc (cho job summary), rồi tuỳ chọn fail job khi
#       có lỗi mức cao.
#
# Usage | Cách dùng:
#   trivy-scan.sh --type <config|fs|image> --target <path|image-ref> \
#                 --name <slug> [--severity CRITICAL,HIGH] [--gate on|off] \
#                 [--scanners vuln,secret,misconfig]
#
#   --type      Trivy subcommand: config (IaC/misconfig), fs (files), image.
#   --target    Directory to scan, or a container image reference.
#   --name      Slug used for the output file and the SARIF category, so several
#               scans can coexist in the Security tab without overwriting
#               each other (e.g. iac-config, image-docker-lab).
#   --severity  Severities the gate cares about. Default: CRITICAL,HIGH
#   --gate      off = report only (SARIF still uploaded). Default: on
#   --scanners  Comma-separated scanner list passed through to Trivy. Only
#               meaningful for `fs` and `image`; omit to use Trivy's defaults.
#
# Environment | Biến môi trường:
#   OUT_DIR              Where SARIF/table files land. Default: trivy-results
#   TRIVY_BIN            Use an existing trivy binary instead of downloading.
#   TRIVY_VERSION        Pin the downloaded version, e.g. v0.58.0. Default:
#                        latest, because a scanner and its vulnerability DB are
#                        one of the few dependencies you *want* moving forward.
#   TRIVY_IGNOREFILE     Default: .trivyignore at the repo root.
#   GITHUB_STEP_SUMMARY  Appended to when present.
#
# Why two passes | Vì sao quét hai lượt:
#   EN: Trivy writes one format per run. The first pass emits SARIF with no exit
#       code so every finding reaches the Security tab; the second pass reuses
#       the cached DB, prints the table and carries the gate's exit code.
#   VI: Trivy chỉ xuất một format mỗi lần chạy. Lượt đầu xuất SARIF và không đặt
#       exit code để mọi phát hiện đều lên tab Security; lượt sau dùng lại DB đã
#       cache, in bảng và mang exit code của gate.

set -euo pipefail

scan_type=""
target=""
name=""
severity="CRITICAL,HIGH"
gate="on"
scanners=""

usage() {
  sed -n '2,45p' "$0" >&2
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --type) scan_type="${2-}"; shift 2 ;;
    --target) target="${2-}"; shift 2 ;;
    --name) name="${2-}"; shift 2 ;;
    --severity) severity="${2-}"; shift 2 ;;
    --gate) gate="${2-}"; shift 2 ;;
    --scanners) scanners="${2-}"; shift 2 ;;
    -h | --help) usage ;;
    *) echo "Unknown argument: $1" >&2; usage ;;
  esac
done

[[ -n "$scan_type" && -n "$target" && -n "$name" ]] || usage
case "$scan_type" in
  config | fs | image) ;;
  *) echo "--type must be config, fs or image (got '${scan_type}')" >&2; exit 2 ;;
esac

out_dir="${OUT_DIR:-trivy-results}"
ignorefile="${TRIVY_IGNOREFILE:-.trivyignore}"
mkdir -p "$out_dir"

sarif_out="${out_dir}/${name}.sarif"
table_out="${out_dir}/${name}.txt"

# Resolve the trivy binary: an already-installed one wins, otherwise fetch it
# into bin/ next to the Makefile-managed tools.
resolve_trivy() {
  if [[ -n "${TRIVY_BIN:-}" ]]; then
    printf '%s\n' "$TRIVY_BIN"
    return
  fi
  if command -v trivy >/dev/null 2>&1; then
    command -v trivy
    return
  fi

  local bindir="${LOCALBIN:-$PWD/bin}"
  if [[ ! -x "${bindir}/trivy" ]]; then
    echo "Installing trivy into ${bindir} ..." >&2
    mkdir -p "$bindir"
    # install.sh takes the version as a positional argument; omitting it means
    # "latest". Build the argv explicitly so an unset TRIVY_VERSION cannot leak
    # an empty string that install.sh would read as a version.
    local -a install_args=(-b "$bindir")
    if [[ -n "${TRIVY_VERSION:-}" ]]; then
      install_args+=("$TRIVY_VERSION")
    fi
    curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh \
      | sh -s -- "${install_args[@]}" >&2
  fi
  printf '%s\n' "${bindir}/trivy"
}

trivy="$(resolve_trivy)"
"$trivy" --version >&2

common_args=(--cache-dir "${TRIVY_CACHE_DIR:-$HOME/.cache/trivy}")
[[ -f "$ignorefile" ]] && common_args+=(--ignorefile "$ignorefile")
[[ -n "$scanners" ]] && common_args+=(--scanners "$scanners")

echo "==> Trivy ${scan_type} scan of '${target}' (category: ${name})"

# Pass 1: SARIF for the Security tab. Every severity is reported here — the tab
# is a backlog, not a gate, and filtering it would hide MEDIUM findings forever.
"$trivy" "$scan_type" "${common_args[@]}" \
  --format sarif \
  --output "$sarif_out" \
  "$target"

# Trivy omits the SARIF file when it finds nothing in some versions; the upload
# step needs a valid document either way.
if [[ ! -s "$sarif_out" ]]; then
  echo "No SARIF produced (no findings); writing an empty run." >&2
  cat > "$sarif_out" <<'EOF'
{
  "version": "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "runs": [
    {
      "tool": { "driver": { "name": "Trivy", "rules": [] } },
      "results": []
    }
  ]
}
EOF
fi

# Pass 2: the table a human reads, plus the gate.
gate_status=0
table_args=("$scan_type" "${common_args[@]}" --format table --output "$table_out" --severity "$severity")
[[ "$gate" == "on" ]] && table_args+=(--exit-code 1)

"$trivy" "${table_args[@]}" "$target" || gate_status=$?

if [[ -s "$table_out" ]]; then
  cat "$table_out"
fi

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    if [[ $gate_status -eq 0 ]]; then
      echo "### ✅ Trivy ${scan_type}: \`${target}\`"
    else
      echo "### ❌ Trivy ${scan_type}: \`${target}\` (${severity})"
    fi
    echo
    if [[ -s "$table_out" ]]; then
      echo '```text'
      cat "$table_out"
      echo '```'
    else
      echo "No ${severity} findings."
    fi
    echo
  } >> "$GITHUB_STEP_SUMMARY"
fi

if [[ $gate_status -ne 0 ]]; then
  echo "::error::Trivy found ${severity} issues in '${target}'." \
       "Fix them, or add a justified entry to ${ignorefile}." >&2
fi

exit "$gate_status"
