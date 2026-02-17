# Privacy

Pulse is designed to run locally. By default, your monitoring data stays on your server.

## No Outbound Telemetry By Default

- Pulse does not send your infrastructure data to Pulse-controlled servers by default.
- There is no third-party analytics SDK in the frontend.

## Optional Outbound Connections (Explicitly Enabled)

Pulse can make outbound connections when you enable specific features:

- **AI (BYOK)**: when AI features are enabled, Pulse sends only the context required for your request to the provider you configured (OpenAI, Anthropic, etc.). See `docs/AI.md`.
- **Relay / Remote Access**: when relay is enabled, Pulse connects to the configured relay endpoint to enable mobile access. See Settings → Remote Access.
- **Update checks**: Pulse can check for new releases/updates (for example via GitHub release metadata) depending on your deployment and configuration.

## Local Upgrade Metrics (Can Be Disabled)

Pulse can record local-only events such as "paywall viewed" or "trial started" to improve and debug in-app upgrade flows.

- These events are stored locally and are not exported to third parties.
- Disable via **Settings → System → General → Disable local upgrade metrics** or set:
  - `PULSE_DISABLE_LOCAL_UPGRADE_METRICS=true`

If you prefer fewer upgrade prompts, you can also enable:
- **Settings → System → General → Reduce Pro prompts**
