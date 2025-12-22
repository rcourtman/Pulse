# Pulse AI

Pulse AI adds an optional assistant for troubleshooting, summarization, and proactive monitoring. It is **off by default** and can be enabled per instance.

## What Pulse AI Can Do

- **Interactive chat**: Ask questions about current cluster state and recent health signals.
- **Patrol**: Background checks that generate findings on a schedule.
- **Alert analysis**: Optional analysis when alerts fire (token-efficient).
- **Command proposals and execution**: When enabled, Pulse can propose commands and (optionally) execute them via connected agents.
- **Finding management**: Dismiss findings as expected behavior, resolve after fixing, with suppression rules to prevent recurrence.
- **Cost tracking**: Tracks usage and supports a monthly budget target.

## Configuration

Configure in the UI:

- **Settings → AI**

AI settings are stored encrypted at rest in `ai.enc` under the Pulse config directory (`/etc/pulse` for systemd installs, `/data` for Docker/Kubernetes).

### Supported Providers

Pulse supports multiple providers configured independently:

- **Anthropic** (API key or OAuth)
- **OpenAI**
- **DeepSeek**
- **Google Gemini**
- **Ollama** (self-hosted, with tool/function calling support)
- **OpenAI-compatible base URL** (for providers that implement the OpenAI API shape)

### Models

Pulse uses model identifiers in the form:

- `provider:model-name`

You can set separate models for:

- Chat (`chat_model`)
- Patrol (`patrol_model`)
- Auto-fix remediation (`auto_fix_model`)

### Testing and Model Discovery

- Test provider connectivity: `POST /api/ai/test` and `POST /api/ai/test/{provider}`
- List available models (queried live from the provider): `GET /api/ai/models`

## Patrol Service

Patrol runs automated health checks on a configurable schedule (default: 15 minutes). It analyzes:

- Proxmox nodes, VMs, and containers
- PBS backup status
- Host agent metrics
- Resource utilization trends

### Finding Severity

Patrol generates findings with severity levels:

- **Critical**: Immediate attention required
- **Warning**: Should be addressed soon

Note: `info` and `watch` level findings are filtered out by default to reduce noise.

### Managing Findings

Findings can be managed via the UI or API:

- **Resolve**: Mark as fixed (finding will reappear if the issue resurfaces)
- **Dismiss**: Mark as expected behavior with a reason (`not_an_issue`, `expected_behavior`, `will_fix_later`)
- **Suppress**: Create a rule to prevent similar findings from recurring

Dismissed and resolved findings are persisted across Pulse restarts.

### AI-Assisted Remediation

When chatting with AI about a patrol finding, the AI can:
- Run diagnostic commands on connected agents
- Propose fixes with explanations
- Automatically resolve findings after successful remediation
- Dismiss findings it determines are expected behavior

## Safety Controls

Pulse includes settings that control how "active" AI features are:

- **Autonomous mode** (`autonomous_mode`): when enabled, AI may execute actions without a separate approval step in the UI.
- **Patrol auto-fix** (`patrol_auto_fix`): allows patrol findings to trigger remediation attempts.
- **Alert-triggered analysis** (`alert_triggered_analysis`): limits AI to analyzing specific events when alerts occur.

If you enable execution features, ensure agent tokens and scopes are appropriately restricted and that audit logging is enabled.

## Troubleshooting

- **AI not responding**: verify provider credentials in **Settings → AI** and confirm `GET /api/ai/models` works.
- **OAuth issues (Anthropic)**: verify the OAuth flow is completing and that Pulse can reach the callback endpoint.
- **No execution capability**: confirm at least one compatible agent is connected and that the instance has execution enabled.
- **Findings not persisting**: check that Pulse has write access to its config directory.
- **Too many findings**: Adjust patrol thresholds in Settings, which derive from your alert thresholds.
