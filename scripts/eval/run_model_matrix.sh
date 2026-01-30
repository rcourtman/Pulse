#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${PULSE_BASE_URL:-http://127.0.0.1:7655}"
EVAL_USER="${PULSE_EVAL_USER:-admin}"
EVAL_PASS="${PULSE_EVAL_PASS:-admin}"
SCENARIO="${EVAL_SCENARIO:-matrix}"
REPORT_DIR="${EVAL_REPORT_DIR:-tmp/eval-reports}"
MODEL_LIST="${EVAL_MODELS:-}"
WRITE_DOC="${EVAL_WRITE_DOC:-1}"

MODEL_ARGS=("-auto-models")
if [[ -n "${MODEL_LIST}" ]]; then
  MODEL_ARGS=("-models" "${MODEL_LIST}")
fi

echo "Running eval scenario '${SCENARIO}' against ${BASE_URL}"
EVAL_REPORT_DIR="${REPORT_DIR}" \
go run ./cmd/eval \
  -scenario "${SCENARIO}" \
  "${MODEL_ARGS[@]}" \
  -url "${BASE_URL}" \
  -user "${EVAL_USER}" \
  -pass "${EVAL_PASS}"

if [[ "${WRITE_DOC}" != "0" ]]; then
  python3 scripts/eval/render_model_matrix.py "${REPORT_DIR}" --write-doc docs/AI.md
  echo "Updated docs/AI.md model matrix."
else
  echo "Skipping docs/AI.md update (EVAL_WRITE_DOC=0)."
fi
