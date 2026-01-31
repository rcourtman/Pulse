#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: patrol_e2e_matrix.sh [options]

Options:
  --url URL                 Pulse API base URL (default: http://127.0.0.1:7655)
  --models LIST             Comma-separated provider:model list (default: deepseek:deepseek-chat)
  --repeats N               Repeat count per model (default: 3)
  --scenario NAME           Eval scenario (default: patrol)
  --min-pass-rate FLOAT     Minimum pass rate per model (default: 1.0)
  --sleep SECONDS           Sleep between runs (default: 5)
  --quiet                   Pass -quiet to eval
  -h, --help                Show this help

Examples:
  ./scripts/patrol_e2e_matrix.sh --url http://192.168.0.98:7655
  ./scripts/patrol_e2e_matrix.sh --models deepseek:deepseek-chat,openai:gpt-4o-mini --repeats 5
USAGE
}

url="http://127.0.0.1:7655"
models="deepseek:deepseek-chat"
repeats=3
scenario="patrol"
min_pass_rate="1.0"
sleep_seconds=5
quiet=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)
      url="$2"; shift 2 ;;
    --models)
      models="$2"; shift 2 ;;
    --repeats)
      repeats="$2"; shift 2 ;;
    --scenario)
      scenario="$2"; shift 2 ;;
    --min-pass-rate)
      min_pass_rate="$2"; shift 2 ;;
    --sleep)
      sleep_seconds="$2"; shift 2 ;;
    --quiet)
      quiet=1; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown arg: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "go not found in PATH" >&2
  exit 1
fi

IFS=',' read -r -a model_list <<< "$models"
if [[ ${#model_list[@]} -eq 0 ]]; then
  echo "No models specified" >&2
  exit 1
fi

printf "Running scenario '%s' against %s (repeats=%s, min pass rate=%s)\n" "$scenario" "$url" "$repeats" "$min_pass_rate"

overall_fail=0

for model in "${model_list[@]}"; do
  model=$(echo "$model" | xargs)
  if [[ -z "$model" ]]; then
    continue
  fi

  pass=0
  fail=0
  echo "\n=== Model: $model ==="

  for i in $(seq 1 "$repeats"); do
    echo "Run $i/$repeats"
    if [[ $quiet -eq 1 ]]; then
      if EVAL_MODEL="$model" go run ./cmd/eval -scenario "$scenario" -url "$url" -quiet; then
        pass=$((pass + 1))
      else
        fail=$((fail + 1))
      fi
    else
      if EVAL_MODEL="$model" go run ./cmd/eval -scenario "$scenario" -url "$url"; then
        pass=$((pass + 1))
      else
        fail=$((fail + 1))
      fi
    fi

    if [[ $i -lt $repeats ]]; then
      sleep "$sleep_seconds"
    fi
  done

  total=$((pass + fail))
  if [[ $total -eq 0 ]]; then
    echo "No runs executed for model $model" >&2
    overall_fail=1
    continue
  fi

  pass_rate=$(python - <<PY
pass=$pass
fail=$fail
total=pass+fail
print(pass/total)
PY
)

  echo "Model $model pass rate: $pass/$total ($pass_rate)"

  meets=$(python - <<PY
pass_rate=float("$pass_rate")
min_rate=float("$min_pass_rate")
print("1" if pass_rate >= min_rate else "0")
PY
)

  if [[ "$meets" != "1" ]]; then
    echo "Model $model failed min pass rate ${min_pass_rate}" >&2
    overall_fail=1
  fi
done

if [[ $overall_fail -ne 0 ]]; then
  echo "\nE2E matrix FAILED" >&2
  exit 1
fi

echo "\nE2E matrix PASSED"
