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
<summary><strong>Manual or custom systemd services (advanced)</strong></summary>

For a **new Pulse server**, use the signed installer above rather than copying
one binary and writing a minimal service unit. The installer creates the
`pulse` service account, prepares the release tree and data ownership, and
installs the server service and update assets together. Its server unit uses
`User=pulse`, `Group=pulse`, `NoNewPrivileges=true`, `PrivateTmp=true`,
`ProtectSystem=strict` and `ProtectHome=true`, with writes limited to the
installation and data directories. Pulse's server does not need a root service;
the separate host **agent** has different privilege requirements (see
[Agent Security](AGENT_SECURITY.md)).

For an **existing manual or custom installation**, first inspect its effective
service settings locally. This read-only command does not print environment
values or credentials:

```bash
systemctl show pulse.service \
  --property=LoadState \
  --property=User \
  --property=Group \
  --property=NoNewPrivileges \
  --property=PrivateTmp \
  --property=ProtectSystem \
  --property=ProtectHome
```

Use your actual server unit name if it differs; older installations may use
`pulse-backend.service`. Interpret these settings only when
`LoadState=loaded`; `LoadState=not-found` means the selected unit is missing,
not that a root service is running. For a loaded service, an empty `User=`
means systemd runs it as root. Missing hardening is not repaired just by
downloading a new binary:
**the installer preserves existing service units during updates**, including
custom units.

Do not overwrite a working unit, change only its service user, or reinstall over
an existing data directory to match these settings. Before a migration, follow
[the recovery guide](RECOVERY.md), preserve the data directory and its encryption
key, and record the active data path and deployment-managed configuration
privately. Check that the intended service account can access that state and
that required integrations work with the chosen sandbox. Do not loosen data
permissions, remove authentication or share a full unit/environment dump as a
shortcut.

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

**API-only does not mean guest-agent-free.** VM filesystem and memory requests
through QEMU Guest Agent can share the channel used by freeze-enabled backups.
Read-only permissions do not prove backup safety. Do not add permissions,
enable or restart an agent, or send manual guest-agent probes during a backup,
freeze or thaw. Use the existing
[backup safety precaution](VM_DISK_MONITORING.md#backup-safety). Stopping Pulse
also stops its monitoring and alerts. An OK backup task does not prove
successful thaw.

> **Note**: If you configure authentication via environment variables (`PULSE_AUTH_USER`/`PULSE_AUTH_PASS`), the bootstrap token is automatically removed and this step is skipped.

---

## 🔄 Updates

The Pulse server and installed Pulse Agents have independent update paths.

### Pulse server updates

#### Automatic Updates (Systemd/LXC only)
Pulse can update the server runtime to the latest stable version.

**Enable via UI**: Settings → System → Updates

#### Manual Update

An update briefly interrupts monitoring and alert delivery. Before changing the
server, record its running version and edition, save the existing deployment
definition privately, and keep a consistent [full-state backup](MIGRATION.md#full-state-recovery)
of every effective data path with its matching keys. A configuration export or
an updater snapshot alone is not a complete data backup. See
[update preparation and recovery limits](DEPLOYMENT_MODELS.md#updates-by-model).

| Platform | Procedure |
|----------|---------|
| **Docker / Compose** | Follow [Docker server updates](DOCKER.md#-updates) from the original deployment, selecting the exact image and updating only the Pulse service. |
| **Kubernetes** | Follow [Helm update precautions](DEPLOYMENT_MODELS.md#kubernetes-helm) for the existing release, namespace, saved values and PVC; retain the chosen chart version and image edition. |
| **Systemd / Proxmox LXC with the Pulse-owned helper** | `sudo /bin/update` |

Use `/bin/update --version vX.Y.Z` for an exact target only when the helper was
installed by the Pulse server installer. On Proxmox community-scripts
containers, `/bin/update` can belong to a different updater that ignores
`--version`. If the helper is absent or its owner is unknown, use the
[signed server-installer flow](#2-bare-metal--systemd) with `PULSE_VERSION` set
to the exact target tag. The same ownership check applies to rollback. After
the service restarts, verify the installed version with `GET /api/version`.

For Docker without Compose, `docker restart` keeps the old image running.
Select the target in the existing saved deployment and pull it successfully
before stopping the current container. Recreate through that deployment with
the same data mount, ports, credentials and other settings, not a fresh example
command. If data is stored only in the container's writable layer, or an
anonymous volume could be removed by `--rm`, stop here until it has a consistent
backup and a checked persistent-data recovery path. Do not delete or prune
volumes, or start a second Pulse against the same writable data.

After rollout, check the running image, Pulse server version, service health
and ordinary monitoring and alert delivery. If the update fails or its result
is uncertain, check the current state before retrying; follow
[Rollback](#rollback) without assuming that reverting an image restores data.

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

Removing or stopping the Pulse server also stops monitoring and alert delivery.
This is not an update, rollback or password-reset procedure. Keep persistent
data by default; deleting it erases configuration, history, credentials and
the keys needed to decrypt that installation's data.

Before removing anything, identify the actual service, container or Helm
release and **every effective data path** privately. Keep a consistent,
private [full-state backup](MIGRATION.md#full-state-recovery), including the
matching encryption key and deployment configuration. A configuration export
alone is not a full backup. Let any in-progress Pulse update finish before
removing its installation. Do not post backups, environment files or full
container inspections in an issue.

### Docker and Compose: retain the data mount

The commands below assume `/data` is on a persistent named volume or bind
mount, as in this guide's examples. **Do not use them for state stored only in
the container's writable layer or temporary storage until it has a consistent
backup.** A container created with `--rm` can also delete anonymous volumes
when stopped. Container removal is not a backup.

For the `docker run` example, stop the container normally before removing it:

```bash
docker stop pulse
docker rm pulse
```

For Compose, run this from the **existing** project, using its actual Pulse
service name:

```bash
docker compose stop pulse
docker compose rm pulse
```

These commands do not request volume deletion. Keep the named volume or bind
directory, original image/runtime and deployment settings; a reinstall must
reattach the **same** data mount. Compose normally prefixes volume names with
its project name, so creating a new project or an empty `pulse_data` volume
can look like data loss. Do not add `-v`/`--volumes`, remove the volume or run
volume pruning as part of a data-preserving removal.

### Kubernetes: check claim ownership before uninstalling

Do not assume `helm uninstall pulse -n pulse` retains data. The default Pulse
chart creates a PersistentVolumeClaim without a keep policy; Helm removal can
delete that claim, and the storage reclaim policy can delete its backing data.
`persistence.existingClaim` refers to a separately managed claim, whose
lifecycle must be checked separately. With `persistence.enabled=false`, the
chart uses temporary `emptyDir` storage, lost when the pod is removed.

After verifying persistent storage and its backup, you can stop the default
deployment without uninstalling the chart:

```bash
kubectl scale deployment pulse \
  --namespace pulse \
  --replicas=0
```

Use the actual deployment and namespace. Record the previous replica count
and suspend any controller that would recreate pods. Before permanently
uninstalling, verify the live claim's ownership, retention and reclaim policy
and test recovery from the private backup; do not delete a PVC or namespace
as a troubleshooting step.

### Systemd / Proxmox LXC: disable without erasing data

Identify the active service and any updater first; legacy installs can use
`pulse-backend`, and custom installs can have other names. In an LXC, run
these steps **inside the Pulse container**, not on the Proxmox host.

For the default signed installation, disable its update timer **if present**:

```bash
sudo systemctl disable --now \
  pulse-update.timer
```

Then stop and disable the server:

```bash
sudo systemctl disable --now \
  pulse.service
```

Disable other deployment-managed updaters through their owner too. These
commands leave the binary, service account, units and data in place; they
disable Pulse, not fully uninstall it. Keep `/etc/pulse` (or the actual custom
data directory), its keys and authentication sources together. Do not remove
the service account while retained files still need its ownership.

**Complete removal is a separate, destructive choice.** The signed server
installer's `--uninstall` deletes its configuration/data directory without a
keep-data prompt; it is not a data-preserving alternative. Use it only after
checking the effective install/config paths and a tested private backup, when
you intend that erasure. Do not delete `/bin/update` unless you have verified
it belongs to Pulse; community-scripts containers can use a different helper.

Removing the server does not remove agents on monitored hosts. Use each
agent's [uninstall procedure](UNIFIED_AGENT.md#uninstall), or
[stop an orphaned agent](TROUBLESHOOTING.md#removed-pulse-server-but-pulse-agent-still-logs-connection-failures),
so it does not keep retrying the absent server.
