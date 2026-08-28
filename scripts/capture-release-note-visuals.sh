#!/usr/bin/env bash
set -euo pipefail

PLAN_FILE=${1:-}
COMPARISON_TAG=${2:-}
OUTPUT_DIR=${3:-}

if [ -z "$PLAN_FILE" ] || [ -z "$COMPARISON_TAG" ] || [ -z "$OUTPUT_DIR" ]; then
  echo "Usage: $0 <visual-plan.json> <comparison-tag> <output-directory>" >&2
  exit 2
fi

ROOT_DIR=$(git rev-parse --show-toplevel)
PLAN_FILE=$(cd "$(dirname "$PLAN_FILE")" && pwd)/$(basename "$PLAN_FILE")
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR=$(cd "$OUTPUT_DIR" && pwd)

python3 "$ROOT_DIR/scripts/release_control/release_note_visuals.py" \
  validate --plan "$PLAN_FILE" >/dev/null
CAPTURE_COUNT=$(python3 "$ROOT_DIR/scripts/release_control/release_note_visuals.py" \
  count --plan "$PLAN_FILE")
if [ "$CAPTURE_COUNT" = "0" ]; then
  exit 0
fi
BEFORE_CAPTURE_COUNT=$(python3 "$ROOT_DIR/scripts/release_control/release_note_visuals.py" \
  before-count --plan "$PLAN_FILE")

if ! git rev-parse -q --verify "${COMPARISON_TAG}^{commit}" >/dev/null; then
  echo "Comparison tag ${COMPARISON_TAG} does not exist" >&2
  exit 1
fi

TEMP_ROOT=$(mktemp -d)
PREVIOUS_TREE="$TEMP_ROOT/previous"
RUN_KEY="${GITHUB_RUN_ID:-$$}-${GITHUB_RUN_ATTEMPT:-1}"
BEFORE_IMAGE="pulse-release-visual-before-${RUN_KEY}"
AFTER_IMAGE="pulse-release-visual-after-${RUN_KEY}"
BEFORE_CONTAINER="pulse-release-visual-before-${RUN_KEY}"
AFTER_CONTAINER="pulse-release-visual-after-${RUN_KEY}"
BEFORE_PORT=${PULSE_RELEASE_VISUAL_BEFORE_PORT:-17655}
AFTER_PORT=${PULSE_RELEASE_VISUAL_AFTER_PORT:-17656}

cleanup() {
  docker rm -f "$BEFORE_CONTAINER" "$AFTER_CONTAINER" >/dev/null 2>&1 || true
  docker image rm -f "$BEFORE_IMAGE" "$AFTER_IMAGE" >/dev/null 2>&1 || true
  git -C "$ROOT_DIR" worktree remove --force "$PREVIOUS_TREE" >/dev/null 2>&1 || true
  rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT

build_visual_image() {
  local source_dir=$1
  local image_name=$2
  docker build \
    --target e2e_runtime \
    --build-arg BUILD_AGENT=0 \
    --build-arg GO_BUILD_TAGS= \
    --tag "$image_name" \
    "$source_dir"
}

if [ "$BEFORE_CAPTURE_COUNT" -gt 0 ]; then
  git -C "$ROOT_DIR" worktree add --detach "$PREVIOUS_TREE" "$COMPARISON_TAG"
  build_visual_image "$PREVIOUS_TREE" "$BEFORE_IMAGE"
fi
build_visual_image "$ROOT_DIR" "$AFTER_IMAGE"

start_visual_container() {
  local image_name=$1
  local container_name=$2
  local port=$3
  docker run --detach --name "$container_name" \
    --publish "127.0.0.1:${port}:7655" \
    --env PULSE_MOCK_MODE=true \
    --env PULSE_MOCK_RANDOM_METRICS=false \
    --env PULSE_MOCK_TRENDS_SEED_DURATION=24h \
    --env PULSE_AUTH_USER=admin \
    --env PULSE_AUTH_PASS=adminadminadmin \
    --env PULSE_DEV=true \
    "$image_name" >/dev/null
}

wait_for_runtime() {
  local port=$1
  local container_name=$2
  local attempts=60
  while [ "$attempts" -gt 0 ]; do
    if curl -fsS "http://127.0.0.1:${port}/api/health" >/dev/null 2>&1; then
      return 0
    fi
    if [ "$(docker inspect --format '{{.State.Running}}' "$container_name" 2>/dev/null || true)" != "true" ]; then
      docker logs "$container_name" >&2 || true
      return 1
    fi
    sleep 2
    attempts=$((attempts - 1))
  done
  docker logs "$container_name" >&2 || true
  echo "Timed out waiting for ${container_name}" >&2
  return 1
}

if [ "$BEFORE_CAPTURE_COUNT" -gt 0 ]; then
  start_visual_container "$BEFORE_IMAGE" "$BEFORE_CONTAINER" "$BEFORE_PORT"
fi
start_visual_container "$AFTER_IMAGE" "$AFTER_CONTAINER" "$AFTER_PORT"
if [ "$BEFORE_CAPTURE_COUNT" -gt 0 ]; then
  wait_for_runtime "$BEFORE_PORT" "$BEFORE_CONTAINER"
fi
wait_for_runtime "$AFTER_PORT" "$AFTER_CONTAINER"

node "$ROOT_DIR/scripts/release_control/capture_release_note_visuals.mjs" \
  "$PLAN_FILE" \
  "http://127.0.0.1:${BEFORE_PORT}" \
  "http://127.0.0.1:${AFTER_PORT}" \
  "$OUTPUT_DIR"

while IFS= read -r asset_name; do
  [ -f "$OUTPUT_DIR/$asset_name" ] || {
    echo "Expected visual asset was not produced: $asset_name" >&2
    exit 1
  }
done < <(python3 "$ROOT_DIR/scripts/release_control/release_note_visuals.py" \
  assets --plan "$PLAN_FILE")
