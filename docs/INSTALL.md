# 📦 Installation Guide

Pulse offers flexible installation options from Docker to enterprise-ready Kubernetes charts.

> **Paid Pulse Pro / Relay / legacy customers:** GitHub release assets and the
> public `rcourtman/pulse` Docker image are Community builds. They can accept an
> activation key, but they do not include the private Pulse Pro runtime hooks.
> Use <https://pulserelay.pro/download.html> with your activation key to get the
> private Pulse Pro Docker image or Linux archive. For Docker Compose, use the
> `PULSE_IMAGE`-aware image line shown below, or replace a hardcoded
> `rcourtman/pulse` image line with the private image shown on the download
> page.

## Windows code-signing status

Pulse was accepted into the SignPath Foundation open-source programme on
2026-08-06, and the public repository is connected to its SignPath project. The
production release certificate is still awaiting issuance (`CSR PENDING`), so
Windows community release artifacts remain unsigned until the certificate is
active and the non-publishing production proof run has passed. Release notes
identify Windows artifacts that are not Authenticode-signed; published
checksums and detached Pulse signatures remain mandatory. Test-signed artifacts
use an untrusted certificate and are never published as production releases.

See the [Code Signing Policy](CODE_SIGNING_POLICY.md) for build provenance,
approval roles, signing scope, and reporting requirements. Release downloads
are published on the [GitHub Releases page](https://github.com/rcourtman/Pulse/releases).

## Verify release build provenance

New release packets include `release-build-provenance.sigstore.json`, the
Sigstore bundle emitted by the hosted workflow that assembled and validated
the candidate. Verify a downloaded asset against that exact workflow and the
release source commit with GitHub CLI 2.97.0 or newer:

```bash
export PULSE_VERSION=vX.Y.Z
export PULSE_ASSET=pulse-vX.Y.Z-linux-amd64.tar.gz
gh release download "${PULSE_VERSION}" --repo rcourtman/Pulse \
  --pattern "${PULSE_ASSET}" \
  --pattern release-build-provenance.sigstore.json
SOURCE_SHA="$(gh api "repos/rcourtman/Pulse/releases/tags/${PULSE_VERSION}" \
  --jq .target_commitish)"
printf '%s\n' "${SOURCE_SHA}" > release-source-sha.txt
gh attestation verify "${PULSE_ASSET}" \
  --repo rcourtman/Pulse \
  --bundle release-build-provenance.sigstore.json \
  --signer-workflow github.com/rcourtman/Pulse/.github/workflows/build-release-candidate.yml \
  --source-digest "${SOURCE_SHA}" \
  --deny-self-hosted-runners \
  --predicate-type https://slsa.dev/provenance/v1
```

For an offline target, also run `gh attestation trusted-root >
trusted_root.jsonl` on the connected trusted machine and transfer that file
with the asset, bundle, and `release-source-sha.txt`. On the offline target,
restore `SOURCE_SHA="$(cat release-source-sha.txt)"` and add
`--custom-trusted-root trusted_root.jsonl` to the verification command. Refresh
the trusted root whenever importing newly signed material; an old copy cannot
report later key revocation or rotation.

## 🚀 Quick Start (Recommended)

### Proxmox VE (LXC installer)
If you run Proxmox VE, the easiest and most “Pulse-native” deployment is the official installer which creates and configures a lightweight LXC container.

Replace `vX.Y.Z` with the exact release tag you want, then run this on your Proxmox host:

```bash
(
set -e
export PULSE_VERSION=vX.Y.Z
pulse_installer_dir="$(mktemp -d)"
trap 'rm -rf "$pulse_installer_dir"' EXIT
cd "$pulse_installer_dir"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig"
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh
bash install.sh --version "${PULSE_VERSION}"
)
```

The signed server installer uses the `vX.Y.Z` Pulse release, which contains the
Linux server archive; it does not ask for a GitHub personal access token. A
`helm-chart-*` release contains a Kubernetes chart, not that archive. If a
different helper selects a Helm-chart release or asks for a GitHub token, stop
instead of supplying one or blindly retrying. On the Proxmox host, check
`pct list` for a partly created Pulse container first; do not run a fresh
installer over an existing container without checking its state.

> **Note**: The GitHub `install.sh` is the **server** installer. The agent installer is served from your Pulse server at `/install.sh` (see **Settings → Infrastructure → Install on a host**). Do not use the GitHub server installer to install or update `pulse-agent`.

### Docker
Ideal for containerized environments or testing.

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e PULSE_DEPLOYMENT_METHOD=docker_run \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

For a new container, continue with [bootstrap-token setup](#step-1-get-the-token)
in your browser; do not add an example password to the command.

### Docker Compose
Create a `docker-compose.yml` file:

```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - PULSE_DEPLOYMENT_METHOD=docker_compose

volumes:
  pulse_data:
```

The `PULSE_IMAGE` variable lets paid Docker users switch the same compose file
to the private Pulse Pro image shown on
<https://pulserelay.pro/download.html> without rebuilding the file around a
second deployment path.

Leave authentication overrides unset for a new install and complete
[bootstrap-token setup](#step-1-get-the-token) in your browser. Do not deploy a
shared example password. If automation must skip setup, use a private
deployment-managed credential source. The
[authentication guide](CONFIGURATION.md#private-docker-authentication-file)
explains the visibility and override limits; its `docker run --env-file`
example is not a Compose interpolation recipe.

For an existing installation, preserve its image, data mounts and managed
configuration; do not reset authentication to repeat first-time setup. If you
used a shared example password, replace it in your deployment's credential
source. A deployment-supplied password takes precedence over changes made in
Pulse's password-change UI. Hashing it inside Pulse does not remove the
original value from Docker's environment or your deployment file. Never share
full `docker inspect` or resolved Compose output.

> **Note**: Docker monitoring requires the unified agent on the Docker host with socket access; the Pulse server container does not need `/var/run/docker.sock`. See [UNIFIED_AGENT.md](UNIFIED_AGENT.md).

---

## 🛠️ Installation Methods

### 1. Kubernetes (Helm)
Deploy to your cluster using our Helm chart.

```bash
helm repo add pulse https://rcourtman.github.io/Pulse
helm repo update
helm upgrade --install pulse pulse/pulse \
  --namespace pulse \
  --create-namespace
```
See [KUBERNETES.md](KUBERNETES.md) for ingress and persistence configuration.

### 2. Bare Metal / Systemd
For Linux servers (VM or bare metal), use the official installer:

```bash
(
set -e
export PULSE_VERSION=vX.Y.Z
pulse_installer_dir="$(mktemp -d)"
trap 'rm -rf "$pulse_installer_dir"' EXIT
cd "$pulse_installer_dir"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig"
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh
sudo bash install.sh --version "${PULSE_VERSION}"
)
```

> **Note**: This installs the Pulse server. Use the `/install.sh` endpoint from **Settings → Infrastructure → Install on a host** for installing or upgrading `pulse-agent` on monitored hosts.

<details>
<summary><strong>Manual systemd install (advanced)</strong></summary>

```bash
# Download and extract the architecture-specific tarball from GitHub Releases:
#   https://github.com/rcourtman/Pulse/releases
# e.g.
#   curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/pulse-${PULSE_VERSION}-linux-amd64.tar.gz"
#   tar -xzf "pulse-${PULSE_VERSION}-linux-amd64.tar.gz"
# The extracted tree contains ./bin/pulse plus ./bin/pulse-agent-* and ./scripts/.

sudo install -m 0755 bin/pulse /usr/local/bin/pulse

# Create systemd service
sudo tee /etc/systemd/system/pulse.service > /dev/null << 'EOF'
[Unit]
Description=Pulse Monitoring
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse
Restart=always
RestartSec=10
Environment=PULSE_DATA_DIR=/etc/pulse

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo mkdir -p /etc/pulse
sudo systemctl daemon-reload
sudo systemctl enable --now pulse
```
</details>

---

## 🔐 First-Time Setup

Pulse is secure by default. On first launch, you must retrieve a **Bootstrap Token** to create your admin account.

### Step 1: Get the Token

| Platform | Command |
|----------|---------|
| **Docker** | `docker exec pulse /app/pulse bootstrap-token` |
| **Docker app UIs** (Unraid, Portainer, TrueNAS apps) | Open the Pulse container's console and run `/app/pulse bootstrap-token` |
| **Kubernetes** | `kubectl exec -it <pod> -- /app/pulse bootstrap-token` |
| **Systemd** | `sudo pulse bootstrap-token` |
| **Proxmox LXC** | `pct exec <ctid> -- /usr/local/bin/pulse bootstrap-token` (run on the Proxmox host; the installer prints this command with your container ID at the end of the install) |

The Proxmox path must be absolute. `pct exec` runs with `PATH=/sbin:/bin:/usr/sbin:/usr/bin`, which does not include `/usr/local/bin`, so a bare `pulse` fails with `No such file or directory`.

> **Important**: Paste the token string printed by the command above. Do not paste the raw `.bootstrap_token` file contents directly. In v6 that file may contain an encrypted JSON snapshot rather than the usable setup token.

### Step 2: Create Admin Account
1. Open `http://<your-ip>:7655`
2. Paste the **Bootstrap Token**.
3. Complete the **Quick Security Setup** wizard.
   - Set your **Admin Username** and **Password** (or let Pulse generate one).
   - Pulse generates an **API token** for agents and automations.
   - Copy the credentials before leaving the page.
4. Open **Settings → Infrastructure → Install on a host** and install the
   unified agent only on hosts where you need agent-provided telemetry. For
   Proxmox, start with API-only monitoring when inventory, node status,
   VM/container status, and storage metrics are enough; use agents for
   inside-guest Docker/Podman visibility, host SMART/temperature data, local
   ZFS/Ceph/mdadm detail, or other telemetry that requires local host access.
   See [Agent Security](AGENT_SECURITY.md).

> **Note**: If you configure authentication via environment variables (`PULSE_AUTH_USER`/`PULSE_AUTH_PASS`), the bootstrap token is automatically removed and this step is skipped.

---

## 🔄 Updates

The Pulse server and installed Pulse Agents have independent update paths.

### Pulse server updates

#### Automatic Updates (Systemd/LXC only)
Pulse can update the server runtime to the latest stable version.

**Enable via UI**: Settings → System → Updates

#### Manual Update

| Platform | Command |
|----------|---------|
| **Docker** | `docker compose pull && docker compose up -d` |
| **Kubernetes** | `helm repo update && helm upgrade pulse pulse/pulse -n pulse` |
| **Systemd / Proxmox LXC with the Pulse-owned helper** | `sudo /bin/update` |

Use `/bin/update --version vX.Y.Z` for an exact target only when the helper was
installed by the Pulse server installer. On Proxmox community-scripts
containers, `/bin/update` can belong to a different updater that ignores
`--version`. If the helper is absent or its owner is unknown, use the
[signed server-installer flow](#2-bare-metal--systemd) with `PULSE_VERSION` set
to the exact target tag. The same ownership check applies to rollback. After
the service restarts, verify the installed version with `GET /api/version`.

Docker without Compose: `docker restart` keeps the old image running. Run `docker pull rcourtman/pulse:vX.Y.Z`, then `docker stop pulse && docker rm pulse` and re-run your original `docker run` command.

The public image and commands above install the Community runtime. If the
instance uses the private Pro runtime, keep it on the private image or archive
shown by <https://pulserelay.pro/download.html>; replacing it with a public
GitHub asset or `rcourtman/pulse` image removes the private runtime hooks.

### Pulse Agent updates

Eligible v6 agents check the Pulse server for updates and apply them
asynchronously. A current server version therefore does not prove every agent is
current. v5 agents, PVE host agents, agents with auto-update disabled, and agents
whose authentication, connection state, download, trust, or self-test checks
fail require manual handling.

Open an outdated-agent notice, or use **Agent Doctor** at
`/settings/infrastructure?agentDoctor=1`, to review the agents Pulse currently
sees and copy the platform-specific command for each host. This surface provides
commands for the operator to run on the host; it does not remotely execute the
update. Use **Settings → Infrastructure → Install on a host** for a first install
or a v5-to-v6 in-place upgrade.

### Rollback
An update error does not establish which version is running. Check the running
version and service health before retrying or rolling back, and preserve the
failed installation and update logs.

In-app update snapshots and updater-script backups have different contents and
lifetimes. Neither is guaranteed to include all active data; an older binary
also may not understand data migrated by a newer version. Update History is
not a full-state recovery tool, and a recorded backup path is not proof of a
complete backup.

Check the [version-specific snapshot scope](AUTO_UPDATE.md#what-an-update-snapshot-contains)
and use the [stopped-service recovery procedure](AUTO_UPDATE.md#manual-rollback)
when data needs restoring. Keep matching data and keys together, verify the
backup privately and preserve reversible copies of the failed state. Do not
replace live runtime data or delete it to make a rollback fit.

---

## 🗑️ Uninstall

**Docker**:
```bash
docker rm -f pulse && docker volume rm pulse_data
```

**Kubernetes**:
```bash
helm uninstall pulse -n pulse
```

**Systemd**:
```bash
sudo systemctl disable --now pulse
sudo rm -rf /etc/pulse /etc/systemd/system/pulse.service /usr/local/bin/pulse
sudo systemctl daemon-reload
```
