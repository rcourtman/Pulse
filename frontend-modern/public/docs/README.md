# Pulse documentation

Start here for installation, platform setup, security, operations, and Pulse
Intelligence. Commands, configuration keys, image names, API fields, and
product identifiers remain untranslated in localized guides.

## Start here

- [Install Pulse](INSTALL.md) — signed Proxmox/Linux installation, Docker,
  Docker Compose, Kubernetes, and first-run setup.
- [Upgrade from Pulse v5](UPGRADE_v6.md) — migration prerequisites, rollback,
  agent continuity, and post-upgrade checks.
- [Configure Pulse](CONFIGURATION.md) — authentication, notifications,
  discovery, retention, and system settings.
- [Deployment models](DEPLOYMENT_MODELS.md) — data locations, lifecycle, and
  differences between supported deployment paths.
- [Troubleshooting](TROUBLESHOOTING.md) and [FAQ](FAQ.md) — common failures,
  diagnostics, and operator questions.

Localized getting started guides: [Deutsch](i18n/de/README.md) ·
[Español](i18n/es/README.md)

## Platforms and agents

- [Proxmox Backup Server](PBS.md)
- [Proxmox Mail Gateway](MAIL_GATEWAY.md)
- [Docker and Podman](DOCKER.md)
- [Kubernetes and Helm](KUBERNETES.md)
- [TrueNAS SCALE and CORE](TRUENAS.md)
- [Unified Agent](UNIFIED_AGENT.md)
- [Agent security](AGENT_SECURITY.md)
- [VM disk monitoring](VM_DISK_MONITORING.md)
- [ZFS monitoring](ZFS_MONITORING.md)
- [Temperature monitoring](TEMPERATURE_MONITORING.md)

VMware vSphere support is early access. Current builds expose dedicated
vSphere inventory and recovery context, but operators should validate the
integration against their own vCenter before production use.

## Monitoring and operations

- [Metrics history](METRICS_HISTORY.md)
- [Recovery data](RECOVERY.md)
- [Webhooks](WEBHOOKS.md)
- [Automatic updates](AUTO_UPDATE.md)
- [Centralized agent management](CENTRALIZED_MANAGEMENT.md) (Pro)
- [Operational trust model](OPERATIONAL_TRUST.md)
- [Current product screenshots](SCREENSHOTS.md)

## Pulse Intelligence

- [Assistant, Patrol, and external-agent overview](AI.md)
- [Patrol modes and safety](AI_AUTONOMY.md)
- [Assistant safety model](ASSISTANT_SAFETY.md)
- [External agent HTTP and MCP substrate](AGENT_SUBSTRATE.md)

Patrol watch-only analysis is available on Community with a local model or the
operator's own provider. Investigation and governed fixes require the relevant
Pulse Pro capabilities.

## Security, privacy, and access

- [Security guide](../SECURITY.md)
- [Privacy and telemetry disclosure](PRIVACY.md)
- [OIDC and SSO](OIDC.md)
- [Proxy authentication](PROXY_AUTH.md)
- [Role-based access control](RBAC.md) (Pro)
- [Audit logging](AUDIT_LOGGING.md) (Pro)
- [Reverse proxy configuration](REVERSE_PROXY.md)
- [Code-signing policy](CODE_SIGNING_POLICY.md)

## Plans and managed access

- [Community, Relay, and Pro capabilities](PULSE_PRO.md)
- [Relay and Pulse Mobile handoff](RELAY.md)
- [Multi-tenant organizations](MULTI_TENANT.md) (Enterprise/custom)
- [Provider-hosted MSP operations](MSP.md) (request-assisted)

Pulse Cloud is not generally available. Ordinary self-hosted Pulse remains the
primary installation path; MSP and Enterprise access are explicit commercial
paths rather than defaults in self-hosted setup.

## Development and reference

- [REST API](API.md)
- [Architecture](../ARCHITECTURE.md)
- [Contributing](../CONTRIBUTING.md)
- [Release notes index](RELEASE_NOTES.md)
- [Development transparency disclosure](AI_TRANSPARENCY.md)

Detailed design notes and dated migration specifications may remain in this
directory for maintainers, but they are not operator setup guides unless they
are linked from the sections above.

## Previous versions and migrations

- [Upgrade from v4 to v5](UPGRADE_v5.md)
- [Retired unified-navigation migration](MIGRATION_UNIFIED_NAV.md) — historical
  context only; current Pulse uses platform-shaped navigation.
- [Move a Pulse installation](MIGRATION.md)

Found a bug? Use the [issue forms](https://github.com/rcourtman/Pulse/issues/new/choose).
For setup questions, use [GitHub Discussions](https://github.com/rcourtman/Pulse/discussions).
