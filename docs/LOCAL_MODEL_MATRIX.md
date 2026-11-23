# Local Open-Weight Model Qualification Matrix

This document records verified evidence for running Pulse Patrol against local open-weight model servers (Ollama, `llama.cpp`, and vLLM). Every number below comes from real `patrol-qualify` or `PatrolModelReadiness` runs on pinned hardware. Unrun model tiers and offline GPU hosts are marked explicitly as pending rather than estimated.

## Evidence Tiers

Pulse keeps qualification tiers distinct and never promotes a model to `Recommended` without passing fault-set evidence:

| Tier | Meaning |
|---|---|
| **Pulse certified** | Full statistical qualification across the scenario catalog meeting the Wilson lower bound (0.85+) on a clean release revision |
| **Pulse tested** | Maintainer-executed `patrol-qualify` or `PatrolModelReadiness` evidence with recorded hardware, quantization, context window, and SHA-256 report checksums (single pass, not yet certified) |
| **Community validated** | Allowlisted community submission bound to a server-issued challenge nonce and verified by replay |
| **Community tested** | Self-reported operator observation, never treated as certification |
| **Pending hardware** | Planned tier not yet executed because the required GPU or runtime host was offline |

## Runtime and Model Matrix

Reference cloud baseline on `fault-set v1` (`commands-on`): `claude-subscription:sonnet` scored **7 of 7** (`commands-off`: **6 of 7**) on the same rootless Docker `patrol-rig` lab.

| Model | Quantization / Size | Runtime | Context Window | Hardware / VRAM | Cold / Warm Preflight | Patrol Readiness Probe | Fault Set v1 (`commands-on`) | Status / Tier |
|---|---|---|---|---|---|---|---|---|
| `ollama:qwen3:8b` | `Q4_K_M` (8.2B) | Ollama `0.40.16` | `40960` trained (Pulse pins `num_ctx`) | Apple M4 16 GB unified (11.8 GiB Metal) | `12,901 ms` / `1,300 ms` (tool call pass) | Tools `3/3`, Context `2/2`, Latency **warning** (`68,359 ms` first / `104,415 ms` warm p50, `latency_unsuitable`, pre-fix `2026-11-23`) | **2 of 7** full baseline (`lm-qwen-on`), partial re-test on `15116e0bc6`: D2 Not looked at, D3 Quiet, D4 Handed over, D5 Handed back, D1 proposed `restart` in 3 evidence calls (invalidated by prior teardown overlap) | **Pulse tested** (not recommended for autonomous or unattended Patrol) |
| `openai:qwen3-8b-q4` | `Q4_K_M` (8.2B, same GGUF blob) | `llama.cpp` `llama-server` `0.9.0` (build `12110`, `--jinja --ctx-size 16384 --reasoning-budget 0`) | `16384` (auto-detected via `/props`) | Apple M4 16 GB unified | `5,195 ms` (cold KV) / `2,357 ms` (warm) (tool call pass) | Tools `3/3`, Context `2/2`, Latency **warning** (`73,828 ms` first / `80,543 ms` warm p50, `latency_unsuitable`, pre-fix `2026-11-23`) | Not run on full 7-fault pass (same weights as Ollama row) | **Pulse tested** (protocol and `/props` verified, latency warning) |
| `ollama:qwen3:8b` | `Q4_K_M` (8.2B) | Ollama `0.32.1` | `40960` trained | Windows RTX 3070 8 GB VRAM (historical `2026-07-20`) | `40,056 ms` disk-cold (`15,556 ms` to `15,953 ms` mem-cold) / `1,943 ms` to `2,501 ms` warm | Preflight native tool calls passed (3/3 mem-cold, 4/4 warm) | Not run on RTX 3070 (`longonk442` offline during `2026-11-23` sweep) | **Pulse tested** (historical preflight only) |
| Qwen3 / Qwen3.5 14B class | `Q4_K_M` (~9 GB) | Ollama / `llama.cpp` | `32768` target | Dedicated 12 to 16 GB GPU or 24 GB+ unified memory | Pending | Pending | Pending | **Pending hardware** |
| Qwen3 / Qwen3.5 27B to 32B class | `Q4_K_M` / `AWQ-4bit` | `vLLM` / `llama.cpp` / Ollama | `32768`+ target | Dedicated 24 GB+ VRAM Linux GPU host | Pending | Pending | Pending | **Pending hardware** |
| DeepSeek / GLM local open-weight tiers | `Q4_K_M` / `AWQ-4bit` | `vLLM` / Ollama | `32768` target | Dedicated 16 to 24 GB+ VRAM GPU host | Pending | Pending | Pending | **Pending hardware** |

## Key Engineering Findings

### 1. Why `qwen3:8b` Is Not Marked Recommended for Unattended Patrol
- Across the complete `fault-set v1` run (`lm-qwen-on`, `commands-on`), `ollama:qwen3:8b` resolved or correctly handed over **2 of 7** faults (`D3` Quiet negative control and `D4` bad `DATABASE_URL` handover). It missed or failed to resolve `D1`, `D2`, `D5`, `D6`, and `D7`.
- On the `2026-11-23` re-run on revision `15116e0bc6` with estate-scoped tools and a bound `agent:exec` command channel, `qwen3:8b` reliably stayed quiet on `D3`, accurately handed over `D4`, and correctly proposed a `restart` fix on `D1` in 3 evidence calls, but still missed `D2` (`Not looked at`) and failed to fix `D5` (`Handed back`).
- `StaticPatrolModelGuidance()` therefore classifies `qwen3:8b` as experimental / unverified for Watch-only experiments rather than an unqualified recommendation.

### 2. Thinking-Mode Latency Gap Between Preflight and `PatrolModelReadiness`
- On Apple M4 16 GB, `POST /api/ai/patrol/preflight` sends `ReasoningEffort: low` (which maps to `think: false` on Ollama `qwen3` models) and completes in **1.3 seconds warm** (`12.9 seconds` cold).
- In the historical `2026-11-23` pre-fix sweep (revision `15116e0bc6`), `POST /api/ai/patrol/readiness` omitted `ReasoningEffort: low` on its scenario and continuation probes. That left hybrid-thinking models emitting unbounded thinking tokens during readiness checks, pushing `qwen3:8b` to **104.4 seconds warm p50** on Ollama and **80.5 seconds warm p50** on `llama.cpp` (`--reasoning-budget 0` still saw multi-turn tool synthesis overhead on M4) and tripping `latency_unsuitable` (projected 10 to 14 minutes for an 8-turn Watch loop against the 4-minute readiness envelope).
- **Fix landed:** Watch-readiness probes in `PatrolModelReadiness` now align with Watch detection and preflight by sending `ReasoningEffort: low` on both initial scenario probes and multi-turn continuation probes. The `2026-11-23` probe numbers in the table above are retained as historical pre-fix evidence until a post-fix hardware sweep is recorded.
- Operators running hybrid-thinking 8B local models on 16 GB Apple Silicon should still expect long full-investigation turns (10 to 15 minutes per fault) and should raise `request_timeout_seconds` to at least `300`.

### 3. Tool-Schema Context Window Overhead
Measured directly by `TestToolSchemaTokenOverheadMeasurement` in `internal/ai/tools/tool_schema_overhead_test.go` on revision `15116e0bc6`:

| Tool | Serialized JSON Size | Estimated Tokens (`ceil(bytes/4)`) |
|---|---|---|
| `pulse_query` | `6,926` bytes | `~1,732` |
| `pulse_metrics` | `5,096` bytes | `~1,274` |
| `pulse_summarize` | `3,635` bytes | `~909` |
| `pulse_alerts` | `3,488` bytes | `~872` (omitted from Patrol detection) |
| `pulse_storage` | `3,466` bytes | `~867` |
| `pulse_kubernetes` | `2,074` bytes | `~519` |
| `pulse_docker` | `2,028` bytes | `~507` |
| `pulse_pmg` | `985` bytes | `~247` |
| **Total (8 baseline tools)** | **`27,698` bytes** | **`~6,925`** |

Scoping unused integration tools out of Patrol detection and investigation once an estate is populated saves:
- **Docker-only estate** (dropping `pulse_kubernetes` and `pulse_pmg`): **~766 tokens (11.1%)** per turn.
- **Proxmox-only estate** (dropping `pulse_docker`, `pulse_kubernetes`, and `pulse_pmg`): **~1,273 tokens (18.4%)** per turn.

## Connecting Local Runtimes Without Cloud Leakage

Pulse supports Ollama natively and supports `llama.cpp` (`llama-server`), vLLM, and LM Studio through the **OpenAI Custom Base URL** provider (no separate provider tab required):

1. **Zero outbound catalog fetches on local endpoints:** When `openai_base_url` points to loopback, RFC1918, link-local, or local DNS suffixes (`.local`, `.lan`, `.internal`, `.home`, `.home.arpa`), `OpenAIClient.ListModels()` skips `https://models.dev/api.json` completely.
2. **Automatic context-window discovery:** `OpenAIClient.InspectModel()` reads `max_model_len` (vLLM) and `context_length` (LM Studio) from `/v1/models`, and falls back to `default_generation_settings.n_ctx` from `/props` (`llama.cpp`), verified live against `llama-server` `0.9.0` (`n_ctx: 16384`).
3. **Local qualification route classification:** `patrol-qualify` classifies both Ollama and local custom OpenAI-compatible endpoints as `inference_route: "local_model_server"` (with refined `inference_engine` values `ollama`, `llama.cpp`, `vllm`, or `lmstudio`) so local runs are never failed on metered API cost gates.

## Checksummed Run Provenance (`2026-11-23`)

- **Pulse revision:** `15116e0bc689e6d956aee44f09ec308e890859fd` (`dirty=false`, runtime `0.0.0-dev-pro`)
- **Ollama model fingerprint (`ollama:qwen3:8b`, `Q4_K_M`, 8.2B):** `sha256:de503317f921fe60472b9961adaa41e4c7bbb924a0134a07dd3876bc460aa3bd`
- **`llama.cpp` model fingerprint (`openai:qwen3-8b-q4`, `n_ctx: 16384`):** `sha256:0763ca6616327986a57752ff802d1ea43d287ec01eec73ae1e24276fa32535d2`
- **Fault-set v1 `report.json` SHA-256 digests (`ollama:qwen3:8b`, `commands-on`, Apple M4 16 GB unified):**
  - `D1` (`q-20261123-160348-ef31e2a3`, `Partial`, invalid due to prior teardown overlap): `f58ea630c22d609efd77c14ad56cbbdc3598a3142f36fb9b035d6cada924d041`
  - `D2` (`q-20261123-161848-6478b13c`, `Not looked at`, valid): `b4280d84cf36cfcac6e7e6f30a39c56e9c41d4db79fd0de8f6a525a48972aaaa`
  - `D3` (`q-20261123-163639-8affba14`, `Quiet`, valid): `5f99551269da73136178c51df88f4db83d06bed208dab2f87de918e417dc03c9`
  - `D4` (`q-20261123-165209-0cf6a0d9`, `Handed over`, valid): `9d5dc81e31887a915dcb18dbc0f1a4461c24071ff26b05a072efb9f1e9eaef09`
  - `D5` (`q-20261123-170458-71674529`, `Handed back`, valid): `3cde7f9cd2d2a03bece350152d0691b3299954b8926a5c0d75297f6616112827`

## Reproducing a Local Run

```sh
go run ./cmd/patrol-qualify -mode fault-set \
  -fault-set v1 -agent-profile commands-on \
  -url http://127.0.0.1:7655 -docker-context <lab-context> \
  -host-filesystem /mnt/nvr \
  -hardware-class "Apple M4 16GB unified" \
  -authorize-live-faults -authorize-remediation \
  -artifacts tmp/patrol-fault-set/local-run
```

See [AI_PATROL_QUALIFICATION.md](AI_PATROL_QUALIFICATION.md) for full lab setup, fixture building, and replay verification instructions.
