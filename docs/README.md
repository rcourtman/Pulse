# Pulse Documentation Index

Use this index to navigate the documentation bundled with the repository. Each
section groups related guides so you can jump straight to the material you need.

---

## Getting Started

- [INSTALL.md](INSTALL.md) – Installation guide covering script, Docker, and Helm paths.
- [FAQ.md](FAQ.md) – Common questions and troubleshooting quick answers.
- [MIGRATION.md](MIGRATION.md) – Export/import process for moving between hosts.
- [DEV-QUICK-START.md](../DEV-QUICK-START.md) – Hot reload workflow for local development.

## Deployment Guides

- [DOCKER.md](DOCKER.md) – Container deployment walkthroughs and compose samples.
- [KUBERNETES.md](KUBERNETES.md) – Helm chart usage, ingress, persistence.
- [REVERSE_PROXY.md](REVERSE_PROXY.md) – nginx, Caddy, Apache, Traefik, HAProxy recipes.
- [DOCKER_MONITORING.md](DOCKER_MONITORING.md) – Docker/Podman agent installation.
- [HOST_AGENT.md](HOST_AGENT.md) – Host agent installers for Linux, macOS, Windows.
- [PORT_CONFIGURATION.md](PORT_CONFIGURATION.md) – Changing default ports and listeners.

## Operations & Monitoring

- [CONFIGURATION.md](CONFIGURATION.md) – Detailed breakdown of config files and env vars.
- [TEMPERATURE_MONITORING.md](TEMPERATURE_MONITORING.md) – Sensor proxy setup and hardening.
- [VM_DISK_MONITORING.md](VM_DISK_MONITORING.md) – Enabling guest-agent disk telemetry.
- [monitoring/](monitoring/) – Adaptive polling and Prometheus metric references.
- [WEBHOOKS.md](WEBHOOKS.md) – Notification providers and payload templates.
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) – Deep dive into common operational issues.

## Security

- [SECURITY.md](../SECURITY.md) – Canonical security policy (root-level document).
- [docs/security/](security/) – Sensor proxy network and hardening guidance.
- [PROXY_AUTH.md](PROXY_AUTH.md) – Authenticating via Authentik, Authelia, etc.
- [TEMPERATURE_MONITORING_SECURITY.md](TEMPERATURE_MONITORING_SECURITY.md) – Legacy SSH considerations.

## Reference

- [API.md](API.md) – REST API overview with examples.
- [api/SCHEDULER_HEALTH.md](api/SCHEDULER_HEALTH.md) – Adaptive scheduler API schema.
- [RELEASE_NOTES.md](RELEASE_NOTES.md) – Latest feature highlights and changes.
- [SCREENSHOTS.md](SCREENSHOTS.md) – UI tour with annotated screenshots.
- [DOCKER_HUB_README.md](DOCKER_HUB_README.md) – Summarised feature list for registries.

## Development & Contribution

- [CONTRIBUTING.md](../CONTRIBUTING.md) – Repository-wide contribution guide.
- [script-library-guide.md](script-library-guide.md) – Working with shared Bash modules.
- [development/MOCK_MODE.md](development/MOCK_MODE.md) – Using mock data while developing.
- [MIGRATION_SCAFFOLDING.md](../MIGRATION_SCAFFOLDING.md) – Tracking temporary migration code.

Have an idea for a new guide? Update this index when you add documentation so
discoverability stays high.
