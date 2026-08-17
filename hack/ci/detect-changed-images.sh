#!/usr/bin/env bash
#
# Emit the JSON matrix of lab images that need rebuilding.
#
# Usage:
#   detect-changed-images.sh [COMPARE_REF]
#
#   COMPARE_REF   Git ref to diff against. Pass an empty string (or omit it)
#                 to force a full rebuild of every image.
#
# Output (stdout): a compact JSON array of image suffixes, e.g.
#   ["docker-lab","k8s-lab"]
#   []                      <- nothing to build
#
# Rebuild rules:
#   - images/Dockerfile.<name> changed  -> rebuild just <name>
#   - any other file under images/ changed (motd, script/, ...) -> rebuild all,
#     because those are shared build inputs baked into every image
#   - no COMPARE_REF -> rebuild all
#
# Extracted from the old .github/workflows/build-image.yaml so the logic can be
# shellcheck'd and exercised locally. Consumed by .github/workflows/_images.yml.

set -euo pipefail

readonly IMAGES_DIR="images"
readonly DOCKERFILE_PREFIX="${IMAGES_DIR}/Dockerfile."

compare_ref="${1-}"

log() { printf '%s\n' "$*" >&2; }

# Every buildable image, derived from the Dockerfile.<name> files on disk.
all_images() {
  local file suffix
  while IFS= read -r -d '' file; do
    suffix="${file#"${DOCKERFILE_PREFIX}"}"
    printf '%s\n' "$suffix"
  done < <(find "${IMAGES_DIR}" -maxdepth 1 -type f -name 'Dockerfile.*' -print0)
}

# Encode newline-separated names as a compact, sorted JSON array.
# `awk NF` drops blank lines so a trailing newline in the input cannot leak an
# empty string into the matrix (which would spawn a job with no image to build).
# It also makes empty input collapse to [] on its own.
to_json_array() {
  local names="${1-}"
  printf '%s\n' "$names" | awk 'NF' | sort -u | jq -R . | jq -s -c .
}

main() {
  local images

  if [[ -z "$compare_ref" ]]; then
    log "No compare ref supplied; rebuilding all images."
    to_json_array "$(all_images)"
    return
  fi

  # --diff-filter=AMR skips deletions: a removed Dockerfile cannot be built.
  local changed
  changed="$(git diff --name-only --diff-filter=AMR "$compare_ref" -- "${IMAGES_DIR}/")"

  if [[ -z "$changed" ]]; then
    log "No changes under ${IMAGES_DIR}/ since ${compare_ref}."
    printf '[]\n'
    return
  fi

  # A change to a shared input (motd, script/, ...) invalidates every image.
  local file
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    if [[ "$file" != "${DOCKERFILE_PREFIX}"* ]]; then
      log "Shared build input changed (${file}); rebuilding all images."
      to_json_array "$(all_images)"
      return
    fi
  done <<< "$changed"

  # Only Dockerfiles changed: build exactly those that still exist.
  images=""
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    [[ -f "$file" ]] || continue
    images+="${file#"${DOCKERFILE_PREFIX}"}"$'\n'
  done <<< "$changed"

  log "Changed images: $(printf '%s' "$images" | tr '\n' ' ')"
  to_json_array "$images"
}

main
