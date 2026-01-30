# Pulse Assistant Eval Harness

This is a live, end-to-end eval harness that exercises the AI chat API, tool calls, and safety gates.
It requires a running Pulse instance and valid credentials.

## Quickstart

List scenarios:
```
go run ./cmd/eval -list
```

Run the full suite:
```
go run ./cmd/eval -scenario full
```

Run a single scenario:
```
go run ./cmd/eval -scenario readonly
```

Run the model matrix quick set:
```
go run ./cmd/eval -scenario matrix
```

Auto-select models (latest per provider):
```
go run ./cmd/eval -scenario matrix -auto-models
```

## Environment Overrides

These env vars let you align the evals with your infrastructure naming:

```
EVAL_NODE
EVAL_NODE_CONTAINER
EVAL_DOCKER_HOST
EVAL_HOMEPAGE_CONTAINER
EVAL_JELLYFIN_CONTAINER
EVAL_GRAFANA_CONTAINER
EVAL_HOMEASSISTANT_CONTAINER
EVAL_MQTT_CONTAINER
EVAL_ZIGBEE_CONTAINER
EVAL_FRIGATE_CONTAINER
EVAL_MODEL                  (optional model override)
EVAL_MODEL_PROVIDERS        (optional comma-separated provider filter for auto selection; defaults to openai,anthropic,deepseek,gemini,ollama)
EVAL_MODEL_LIMIT            (optional per-provider limit for auto selection, default 2)
EVAL_MODEL_EXCLUDE_KEYWORDS (optional comma-separated keywords to skip models; default filters image/video/audio, codex, and specific pre-release IDs like openai:gpt-5.2-pro until chat support is live; set to "none" to disable)
```

Write/verify and strict-resolution controls:

```
EVAL_WRITE_HOST              (defaults to EVAL_NODE)
EVAL_WRITE_COMMAND           (defaults to "true")
EVAL_REQUIRE_WRITE_VERIFY    (set to 1 to assert pulse_control -> pulse_read)
EVAL_STRICT_RESOLUTION       (set to 1 to expect STRICT_RESOLUTION block)
EVAL_REQUIRE_STRICT_RECOVERY (set to 1 to require pulse_query -> pulse_control)
EVAL_EXPECT_APPROVAL         (set to 1 to assert approval_needed event)
```

Retry controls and reports:

```
EVAL_HTTP_TIMEOUT           (seconds, default 300)
EVAL_STEP_RETRIES            (default 2)
EVAL_RETRY_ON_PHANTOM        (default 1)
EVAL_RETRY_ON_EXPLICIT_TOOL  (default 1)
EVAL_RETRY_ON_STREAM_FAILURE (default 1)
EVAL_RETRY_ON_EMPTY_RESPONSE (default 1)
EVAL_RETRY_ON_TOOL_ERRORS    (default 1)
EVAL_RETRY_ON_RATE_LIMIT     (default 0)
EVAL_RATE_LIMIT_COOLDOWN     (seconds, optional backoff before retry)
EVAL_PREFLIGHT              (set to 1 to run a quick chat preflight)
EVAL_PREFLIGHT_TIMEOUT       (seconds, default 15)
EVAL_REPORT_DIR              (write JSON report per scenario)
```

## Recommended Runs

Full suite with custom resource names:
```
EVAL_NODE=delly EVAL_DOCKER_HOST=homepage-docker \
go run ./cmd/eval -scenario full
```

Strict-resolution block + recovery (requires server with PULSE_STRICT_RESOLUTION=true):
```
EVAL_STRICT_RESOLUTION=1 EVAL_REQUIRE_STRICT_RECOVERY=1 \
go run ./cmd/eval -scenario strict
```

Strict-resolution block only (no recovery):
```
EVAL_STRICT_RESOLUTION=1 \
go run ./cmd/eval -scenario strict-block
```

Strict-resolution recovery in a single step:
```
EVAL_STRICT_RESOLUTION=1 EVAL_REQUIRE_STRICT_RECOVERY=1 \
go run ./cmd/eval -scenario strict-recovery
```

Approval flow (requires Control Level = Controlled):
```
EVAL_EXPECT_APPROVAL=1 \
go run ./cmd/eval -scenario approval
```

Approval approve flow (auto-approves approvals during the step):
```
EVAL_EXPECT_APPROVAL=1 \
go run ./cmd/eval -scenario approval-approve
```

Approval deny flow (auto-denies approvals during the step):
```
EVAL_EXPECT_APPROVAL=1 \
go run ./cmd/eval -scenario approval-deny
```

Approval combo flow (approve + deny in one session):
```
EVAL_EXPECT_APPROVAL=1 \
go run ./cmd/eval -scenario approval-combo
```

Write then verify (safe no-op command by default):
```
EVAL_REQUIRE_WRITE_VERIFY=1 \
go run ./cmd/eval -scenario writeverify
```

## Model Matrix Workflow

Run the matrix and update the docs table in one step:
```
scripts/eval/run_model_matrix.sh
```

Key overrides:
```
PULSE_BASE_URL=http://127.0.0.1:7655
PULSE_EVAL_USER=admin
PULSE_EVAL_PASS=admin
EVAL_MODEL_PROVIDERS=openai,anthropic,gemini
EVAL_MODEL_LIMIT=2
EVAL_MODELS=anthropic:claude-haiku-4-5-20251001
EVAL_SCENARIO=matrix
EVAL_REPORT_DIR=tmp/eval-reports
EVAL_WRITE_DOC=1
```

## Notes

- The evals run against live infrastructure. Use safe commands or keep the default `EVAL_WRITE_COMMAND=true`.
- Scenario assertions are intentionally coarse; use stricter env flags to enforce write/verify or strict-recovery sequences.
- Live tests via `go test`:
  ```
  go test -v ./internal/ai/eval -run TestQuickSmokeTest -live
  ```
