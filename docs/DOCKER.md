# 🐳 Docker Guide

Pulse is distributed as an Alpine-based Docker image. Image storage and Pulse's
persistent data are separate; see [image size and disk use](#image-size-and-disk-use).

> **Paid Pulse Pro / Relay / legacy customers:** The public `rcourtman/pulse`
> Docker image is the community build. It can accept an activation key, but it
> does not include the private Pulse Pro runtime hooks. Use
> <https://pulserelay.pro/download.html> with your activation key, then run the
> private registry login and `PULSE_IMAGE=license.pulserelay.pro/pulse-pro:<version>`
> compose commands shown there. Those commands require the compose file image
> line to use the `PULSE_IMAGE` variable, as shown below. If your compose file
> hardcodes `image: rcourtman/pulse:...`, replace that line with the variable
> form or with the private image shown on the download page before restarting.

## 🚀 Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e PULSE_DEPLOYMENT_METHOD=docker_run \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Access at `http://<your-ip>:7655`.

---

## 📦 Docker Compose

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
      - TZ=Europe/London
      - PULSE_DEPLOYMENT_METHOD=docker_compose

volumes:
  pulse_data:
```

Run with: `docker compose up -d`

**Freeze-enabled VM backups:** API-only monitoring can still send QEMU Guest
Agent requests. For an affected installation, follow the
[planned server pause and checked restoration](VM_DISK_MONITORING.md#pause-a-docker-or-compose-server-for-a-planned-backup),
not container recreation or guest-agent probes. Monitoring and alerts are
unavailable while Pulse is stopped; an OK backup alone does not prove recovery.

Leave authentication overrides unset for a new install and complete
[bootstrap-token setup](INSTALL.md#step-1-get-the-token) in your browser. Do not add a
shared example password to the Compose file. If automation must skip setup,
use a private deployment-managed credential source; the
[private Docker authentication file](CONFIGURATION.md#private-docker-authentication-file)
example is for `docker run`, not Compose interpolation.

The `PULSE_IMAGE` variable lets the same compose file run either the public
community image or, for eligible paid customers, the private Pulse Pro image
shown on <https://pulserelay.pro/download.html>.

---

## Image size and disk use

A registry download, a local image and a container's data do not measure the
same storage. Before comparing sizes, keep the exact image reference or digest,
platform (for example, `linux/amd64` or `linux/arm64`), displayed units and the
tool's measurement label together. A mutable tag such as `latest` is not an
immutable comparison, and a multi-platform image index is not one platform's
image.

| Reading | What it measures | What it does not establish |
| --- | --- | --- |
| Registry or pull download size | Usually the compressed layers for the selected platform; cached layers may not need downloading again | The unpacked local image size or space used by Pulse's data |
| Local image size | The image's stored layers, with accounting depending on the Docker version and image store | Exclusive disk use: layers can be shared between images and tags, and some stores retain both compressed and unpacked content |
| Container writable-layer size | Changes made in that container above the image | Mounted volume or bind-mount contents; it is not the image's download size |
| Pulse's mounted `/data` | Persistent configuration, credentials and retained history in the deployment's volume or bind mount | Image size; replacing an image does not shrink or back up this data |

Image layers are retained even when a later layer replaces a file or changes
its ownership. The final visible filesystem can therefore be smaller than the
stored image. Conversely, adding the sizes of several local tags can count
shared layers more than once. Keep **GB** (decimal) and **GiB** (binary), or a
tool's rounded **MB** label, distinct; do not infer a leak or a reduction from
unmatched readings.

Use details already available in your deployment manager. The
[update checks](#-updates) show a narrow, read-only way to identify a container's
configured image and image ID; a Compose file or a pulled tag alone does not
establish which image the container is running. Existing observations are enough
for a report: include the size and its label, image/platform if known, and
whether it concerns the image, writable layer or persistent data. Unknown is
valid; do not pull or build another image, recreate a container or export full
inspect/Compose output just to complete the comparison.

**Do not prune images, delete volumes or clear `/data` to diagnose image size.**
Keep the previous image and consistent data backup needed for
[update recovery](#-updates). An image-layer repair does not reclaim old images,
shrink retained history or establish how much space an installed update will
free. For unexpected ongoing writes or growth, use the separate
[CPU and write-rate checks](TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth),
without changing retention or restarting to obtain a smaller reading.

## ⚙️ Configuration

Pulse is configured via the UI (`system.json`) with optional environment overrides.

| Variable | Description | Default |
|----------|-------------|---------|
| `TZ` | Timezone | `UTC` |
| `PULSE_AUTH_USER` | Admin Username | *(unset)* |
| `PULSE_AUTH_PASS` | Admin Password | *(unset)* |
| `DISCOVERY_SUBNET` | Custom CIDR to scan | *(auto)* |
| `ALLOWED_ORIGINS` | CORS allowed origin (`*` or a single origin). Empty = same-origin only. | *(unset)* |
| `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |
| `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS` | Disable container image-update actions; does not disable start/stop/restart | `false` |
| `PULSE_METRICS_DB_PATH` | Optional path for only `metrics.db`, useful with tmpfs | `/data/metrics.db` |
| `PULSE_METRICS_ROLLUP_INTERVAL` | Metrics aggregation cadence; minimum 5 minutes | `15m` |

> **Tip**: Set `LOG_LEVEL=warn` to reduce log volume while still capturing important events.
> **Note**: API tokens are managed in the UI and stored in `api_tokens.json`.
> **Note**: Plain text values in `PULSE_AUTH_PASS` are hashed for authentication
> at startup, but remain in the container environment and deployment file.
> Docker administrators can read those values. Prefer bootstrap setup; never
> share full `docker inspect` or resolved Compose output.

For SSD-sensitive installs, keep `/data` persistent and put only metrics
history on tmpfs:

```yaml
services:
  pulse:
    environment:
      PULSE_METRICS_DB_PATH: /metrics-tmpfs/metrics.db
    tmpfs:
      - /metrics-tmpfs:size=512m,uid=1000,gid=1000,mode=0700
```

Metrics history stored this way is lost on container restart.

### Healthcheck and resource sizing

Add this healthcheck to your **existing** `pulse` service; it is not a complete
Compose file. Preserve the image, persistent data mount, network and other
settings. Docker marks health failures as unhealthy; a healthcheck alone does
not restart the container or prove that monitored workloads are healthy.

```yaml
services:
  pulse:
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:7655/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

**There is no universal CPU or memory limit for Pulse.** This example does not
set one, and is not an instruction to remove an existing limit. Keep your
deployment's current resource policy while investigating a problem. An
undersized memory limit can cause an out-of-memory kill; a tight CPU quota can
delay polling and notification delivery. Neither repairs an increased baseline.

Choose limits from already available container readings during normal steady
and busy periods, leaving headroom for the workload. Pulse process RSS is only
part of the container's accounted memory, not a safe container limit by itself.
Use the [bounded Docker statistics](TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth)
and [memory interpretation](TROUBLESHOOTING.md#memory-use-keeps-growing) if help
is needed; an existing screenshot or unavailable reading is valid evidence.
Do not restart, induce load, shorten retention or slow polling just to size the
container or lower a reading. A higher steady baseline with unchanged monitoring
is worth reporting, even when memory no longer grows.

---

## Runtime and container alert scopes

In **Alerts → Thresholds → Docker**, runtime connectivity, per-container alert
rules and machine maintenance have different scopes. Turning off one check
is not a blanket mute for everything on that machine.

- **Runtime Offline Alerts** controls that runtime's connectivity alert. It
  does **not** turn off alerts for its containers. The runtime row's general
  **Alerts** switch is not a runtime-and-children control; do not rely on it
  to mute the containers below it.
- **One container's Alerts → Off** disables that container's threshold alerts,
  including state, health and resource checks. Find the container under the
  intended runtime/host group, not by name alone, then save the settings and
  check the saved row. Other containers and the machine keep their own rules.
  Turning off only the container's **Offline Alerts** leaves its other checks
  enabled. Neither setting stops collection or hides readings.
- **Machine maintenance** has a different scope. On a unified-agent host, the
  runtime's **Manage** controls apply to the machine too. A window covering
  the machine and its descendants also suppresses wanted machine alerts, not
  just Docker alerts. It is not equivalent to muting one runtime and its
  containers while
  keeping the machine monitored. Global or platform-wide Off controls also
  affect more than one runtime; do not use them for that narrower job.

There is no single inherited runtime-and-containers Off control here. For an
on-demand lab, the existing alternative is to disable each unwanted container's
alerts separately and, if wanted, the runtime's Offline Alerts. This does not
automatically cover containers added later; review their own settings too.
Keep wanted machine and other-runtime alerts enabled.

Use existing alert details to distinguish the runtime, container and machine;
do not stop a workload, induce an alert or send another Test to check a mute.
Already queued messages can reflect earlier settings: changing a rule is not
queue cancellation or proof of recovery. For those messages, use
[ordinary notification checks](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing).
Patrol findings are separate from threshold alerts; see
[metric Off and Patrol](FAQ.md#can-i-disable-alerts-for-specific-metrics).

## Container CPU readings

Pulse shows container CPU as a share of the **Docker/Podman host's total CPU
capacity**: 100% means all its logical CPUs. `docker stats` and `podman stats`
normally use **100% for one logical CPU**, so their readings can exceed 100%.
For that scale, divide the stats percentage by the CPU count reported for the
runtime host to compare it with Pulse.

If the engine runs inside a VM, use that VM's logical CPUs, not the physical
Proxmox host's CPUs. Pulse's percentage is not utilisation of the container's
own CPU quota or cpuset. If the reported CPU count differs from your configured
VM size, retain both in the comparison rather than guessing a divisor.

| Example host logical CPUs | Stats CPU | Pulse CPU before display rounding |
| --- | --- | --- |
| 4 | 240% | 60% |
| 6 | 16.52% | About 2.75% |
| 6 | 1.04% | About 0.17% |

These are examples, not an assumption about your host. A smaller Pulse value
alone does not establish a collection fault. Whole-percent table labels,
including those in **v6.5.0-rc.1**, can display a fractional reading below 0.5%
as 0%. Use **CPU History** rather than the rounded label; display rounding does
not change stored readings or the values used by CPU alert evaluation. A dash
or missing History is unavailable data, not a measured zero.

For a useful comparison:

1. Match the same container on the same runtime host. Compare a similar time
   interval in CPU History with `docker stats` or `podman stats`; separate
   sampling intervals can legitimately differ.
2. Apply the reported host CPU count only to a stats reading on the per-CPU
   scale. Do not divide Pulse History again, or compare with an average since
   container start as though it were a recent interval.
3. Check that observation times and History points advance during ordinary
   monitoring. A connection marked Active does not prove fresh CPU samples.
4. If the normalised readings still disagree, or History stops advancing,
   report the server and agent versions, reported CPU count, sample times and
   bounded readings. Do not restart workloads, create CPU load, lower alert
   thresholds or export diagnostics just to make this comparison.

CPU alerts use the same host-capacity scale, with the configured thresholds,
duration and suppression rules. Non-zero History proves a collected reading,
not that an alert should fire or that its notification was delivered. If a
normal workload already demonstrates an alert problem, retain the reading,
threshold, duration and observed result; no forced alert test is needed. See
[safe performance measurements](TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth)
when measuring the Pulse server itself rather than a monitored container.

---

## 🔄 Updates

These steps update the Pulse server, not the containers it monitors. Record the
current image and take a consistent private backup of the mounted Pulse data
before changing it. Keep the previous image and data backup for recovery; do
not delete volumes.

```bash
docker inspect pulse --format '{{.Config.Image}} {{.Image}}'
```

Run commands from the original Compose project directory. First set the exact
target in your Compose `image:` line, or persist `PULSE_IMAGE` in the project's
`.env` if the file uses `image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}`.
Setting `PULSE_IMAGE` has no effect on a hardcoded image line. Community uses
`rcourtman/pulse:vX.Y.Z`; paid Pro installs must keep the private image and
existing registry login from <https://pulserelay.pro/download.html>. Do not
replace Pro with the Community image or post registry credentials.

For the `pulse` service in the examples above:

```bash
(
set -e
docker compose pull pulse
docker compose up -d --no-deps pulse
)
```

If pulling fails, stop rather than recreating with an unverified or old image.
Pulling a different tag alone does not update your configured image. There is
no need to bring the whole Compose project down. Use your actual service and
container names if they differ; legacy `docker-compose` users can substitute
that command name.

For `docker run` or an app UI, change the image in the existing saved deployment
and recreate it with the same data mount, ports and settings; do not start a
second Pulse against that data. Check the running image again, Pulse's displayed
server version and service health after recreation. See
[rollback and backup scope](AUTO_UPDATE.md#rollback) before reverting a version:
an image change alone does not undo data migrations.

---

## 🔄 Docker / Podman Updates

These actions update **monitored containers**, not the Pulse server. Pulse can
detect image updates without command execution; applying an update requires a
separately enabled command channel and administrator review. Leave command
execution disabled when you only need monitoring.

### Before updating a workload

- Plan for downtime: a running container is stopped and replaced, not updated
  in place. Check the application's upgrade and data-migration requirements.
- Take an independent, consistent backup of its application data, including
  volumes and bind mounts, using the application's backup procedure. The
  renamed old container is **not a backup of mounted data**: the replacement
  uses those same mounts and can change their contents.
- Keep the previous image identity and your deployment definition privately.
  For Compose, Yacht or another deployment manager, prefer that manager's
  update procedure so its saved definition records the intended image. Pulse's
  recreation does not edit your Compose file or manager's desired state.
- Do not use an update as a diagnostic test or start another container against
  the same writable data to check a failure.

### How It Works

1. **Update Detection**: Pulse compares the local image digest with the latest digest from the container registry
2. **Visual Indicator**: Containers with available updates show a blue upward arrow icon
3. **Reviewed Update**: Review the target and approve the action, then verify the application yourself

### Updating a Container

1. Open **Docker → Overview** (`/docker/overview`) and find the host and container
2. Look for containers with a blue update arrow (⬆️)
3. Click the update button and approve the action in the review dialog (admin approval required)
4. Pulse attempts to:
   - Pull the latest image
   - Stop the current container if it was running
   - Rename the original container with the `_pulse_backup_` suffix
   - Recreate the container from its inspected configuration, reusing its data mounts
   - Start it only if the original was running, then perform a short runtime check
   - Schedule removal of the renamed old container after a successful update

Afterwards, check the actual image, container state, application readiness and
data through your normal administration tools. A successful action is not an
end-to-end application or data-integrity test.

### Batch Updates

Updates run as reviewed per-container actions, so there is currently no bulk update flow: update each container individually with its own update button. The **"Update all"** button in the host drawer only points you to the per-container buttons.

### Safety Features

- **Temporary old container**: the `_pulse_backup_` name retains the old
  container, not a separate copy of its volumes or bind mounts. Current agents
  schedule removal **five minutes after a successful update**; periodic cleanup
  also removes old backup containers. Do not rely on a 15-minute recovery window
  or on a remaining container as a durable backup.
- **Best-effort rollback**: when recreation, network attachment or the early
  runtime check fails, Pulse attempts to restore the original name and running
  state. Removal, rename or restart can fail too. A failed banner does not prove
  that the original container is running or that no change occurred.
- **Limited verification**: the running replacement is inspected after a short
  wait and rejected if stopped or explicitly unhealthy. A missing healthcheck
  or a `starting` health status is not proof that the application is ready.
- **Inspected configuration**: Pulse reuses container settings and mounts; it
  does not reconcile your deployment manager or roll back application data
  migrations. Verify networks, ports, mounts and the application after the change.

If the result is failed or uncertain, **check the current state before retrying**.
Retain the action error and time, old/new container and image identities, and
any rollback error. Inspect these locally; do not post full `docker inspect`
output, environments or registry credentials. Do not delete the old container
or volumes to clear a warning. Recover through the application's and deployment
manager's procedures using a verified matching data backup where required;
returning to the old image alone does not undo a data migration.

### Check a failed or pending workload update

Start with the **existing error and action stage**, not another click on Update:

- **Planning refused:** “Docker / Podman command agent is not connected” on the
  planning request means Pulse could not find a matching admitted live command
  channel and refused that plan before dispatch. The planning response can be
  HTTP 409, but 409 alone has other causes. This does **not** establish what an
  earlier action did; use the command-channel checks below for this refusal.
- **Action sent, receipt pending:** “The action was sent once” or a pending
  durable receipt means the outcome is uncertain, even beside a failed banner.
  Use the existing receipt and workload-state checks below, without dispatching
  another update.
- **Stage unknown:** a failed banner alone does not tell you whether the update
  ran. Retain that uncertainty; do not assume nothing changed or repeat the
  action to find out. Use existing observations where safe.

#### Command agent is not connected

Open **Settings → Infrastructure → Agent Doctor**, expand the **affected Docker
host**, and inspect the existing **Privilege** wording. This is the agent for
the workload, not necessarily the host running Pulse. `commands monitoring-only`
describes a local command ceiling; `commands command-capable` and `credential
grants exec` describe local authority and credential scope, **not a connected
session**. A missing Privilege line is unknown, not proof that commands are off.
Fresh monitoring or an available image update does not prove command transport.

`root` identifies the collector's OS user, not its server-side permission.
`commands legacy` identifies an older, unmarked compatibility profile; it does
not establish that commands are enabled. `monitoring credential` means the
known collector credential has no execution scope and cannot authenticate the
legacy command channel. Do not add execution rights to that token as a repair.

Inspect **Action runner** separately. `Credential not issued` and `Runner not
connected` describe the separate typed runner, not monitoring health. An absent
runner alone does not rule out an authorised legacy command connection. If
**Safe collector not confirmed** is shown, that is not proof of a broken
collector or permission to convert it merely to clear the update error.

For a command-enabled, execution-scoped legacy connection, follow the existing
[command-channel connection and admission checks](UNIFIED_AGENT.md#commands-enabled-but-remote-control-blocked).
A monitoring-only setup is intentional when you only need readings and update
detection; do not enable commands, broaden a token, change Proxmox permissions
or loosen TLS merely to diagnose the refusal. Keep saved identity and credentials
intact; do not reinstall or retry the workload update as a connection test.

You can keep working monitoring and manage any needed update through your usual
Docker or Podman administration tool, after checking the workload's existing
image/state and recovery path. Deliberately opting into Pulse-managed changes
is a separate trust decision: the
[agent privilege model](AGENT_SECURITY.md#agent-privilege-model) explains the
combined command profile behind **Allow Pulse to run commands on this host**, and the
[least-privilege profile](AGENT_SECURITY.md#least-privilege-agent-profile)
describes the supported separate runner. Neither is a diagnostic prerequisite;
do not change a working profile just to obtain evidence.

For help, retain the original error, time and stage plus the existing command
authority/scope and Action runner wording, or say it is unavailable. If relevant,
say whether command execution was deliberately selected at installation;
unknown is valid. Share only those distinctions, not a full Agent Doctor report,
token hint, agent ID or service configuration.
The connection refusal does not settle an earlier missing receipt.

#### Action was sent or its outcome is unknown

Use your normal Docker administration shell **on the host running the affected
container**, not a shell inside Pulse or on an unrelated Proxmox host. Use the
same Docker context as that workload; do not change socket permissions or enable
agent commands just to inspect it.

List container names, including stopped containers, without dumping their
configuration:

```bash
docker ps -a --format '{{.Names}}'
```

Replace `affected-container` with the original workload name (not `pulse` unless
Pulse itself was the target), then read only its state and image identity:

```bash
container='affected-container'
docker inspect --type container --format 'State={{.State.Status}} ImageRef={{.Config.Image}} ImageID={{.Image}} Started={{.State.StartedAt}}' "$container"
```

`State` says whether the container is running, stopped or restarting. `ImageRef`
is its configured image reference; a tag such as `latest` can stay unchanged
after an update. `ImageID` identifies the actual local image used by this
container: compare it with your privately recorded pre-update image or the
intended image in your deployment manager. It is not a registry manifest digest,
so do not compare it directly with Pulse's latest-registry digest. `Started` is
only a start time, not proof of an image update or application readiness. If the
old identity was not recorded, these readings alone cannot establish whether
the image changed.

If inspection fails or the original name is missing, retain that error and the
name listing locally; the workload may have been renamed during the attempt.
Do not start, rename, delete or update anything to make the check succeed.
Check the application's ordinary UI or health endpoint separately; a running
container is not proof that its application or data is healthy.

In Pulse, reopen that container's **Review action** and use **Check for receipt**
when available. This re-reads the existing action; it does not send another
container update. If the receipt is still missing, leave the update alone until
the actual workload state and recovery path are reconciled. For help, share only
the relevant state, receipt status and a redacted error, not the full inspection,
other container names, environments or registry credentials.

### Requirements

- **Unified agent** running on the Docker host with Docker monitoring enabled
- **Command execution enabled** on the agent (`--enable-commands` or `PULSE_ENABLE_COMMANDS=true`) — updates run as reviewed actions through the agent's command channel, the same as [container lifecycle actions](#️-container-lifecycle-actions), and share their requirements and limitations (admin approval, authorization-plugin block)
- Agent must have Docker socket access (`/var/run/docker.sock`)
- Registry must be accessible for update detection (public registries work automatically)

### Private Registries

Private-image update detection runs on the monitored host's unified agent,
not on the Pulse server. It uses the login visible to that agent process;
a successful pull in another user's shell does not prove the agent can
authenticate. Detection does not require enabling command execution.

**HTTPS is required for registry checks.** The agent contacts the registry
directly over HTTPS; Docker's `insecure-registries` setting does not apply, and
Pulse has no plain-HTTP registry option. A successful Docker pull from an
HTTP-only registry does not establish support for Pulse's update check.

If the existing error says `http: server gave HTTP response to HTTPS client`,
the selected endpoint is serving HTTP where the checker expects TLS. This is
not evidence of a missing login; changing registry credentials or adding them
to the Pulse server will not repair that protocol mismatch. The supported
route is an HTTPS registry endpoint, directly or through a TLS reverse proxy,
with a certificate trusted by the agent. Do not disable TLS verification or
send registry passwords over HTTP to test this.

**Native agent:** use its actual service account, home and service environment.
Root and least-privilege installations can use different accounts. Log in
interactively in the intended account and environment:

```bash
docker login registry.example.com
```

Use `sudo docker login` only for an agent that actually runs as root. Do not
switch the agent to root or make credentials world-readable to fix detection.
Use a dedicated pull-only registry account where the registry supports one.

The agent looks for credentials in these files, in order, using the first
usable login for the registry:

- `REGISTRY_AUTH_FILE`: an explicit Docker/Podman authentication file.
- `DOCKER_CONFIG`: a directory containing `config.json`, when set.
- `$HOME/.docker/config.json`: the agent process's home, not your interactive
  user's home.
- `$XDG_RUNTIME_DIR/containers/auth.json`: Podman's runtime login, when
  `XDG_RUNTIME_DIR` is set.

Docker `auths`, per-registry `credHelpers` and global `credsStore` are supported.
A configured credential helper needs its helper binary and its backing store
accessible in the agent's runtime, not just `config.json`.

**Containerised agent:** Docker socket access does not expose the host's login
files. Any credential file must be visible in the container's filesystem and
readable by its agent user. If supplying a dedicated authentication file, mount
only that file read-only and set `REGISTRY_AUTH_FILE` in the agent environment
to its container path. A read-only mount still lets the agent read its secrets.
Do not mount an entire home directory or copy logins into the Pulse server.
A helper-backed host login also needs the helper and backing store; mounting
`config.json` alone is not enough.

Credentials are sent directly to the registry or its authentication endpoint,
not reported to the Pulse server. Credential lookups are cached for up to
five minutes, including missing logins; a changed login may not be picked up
until the next ordinary scheduled check. Successful detection is not proof
that a workload update can pull or recreate the container: that is a separate
[reviewed action](#️-container-lifecycle-actions).

**“Check failed” is not evidence that the image is up to date.** Read its existing
short error before changing credentials: an authentication failure and a TLS or
network failure need different remedies. Do not disable TLS verification.
For help, give the agent version, native/container deployment and that short
error with registry/image names removed. Never share login files, helper output,
passwords or tokens.

To keep detection anonymous-only, set `PULSE_DISABLE_REGISTRY_CREDENTIALS=true`
(or pass `--disable-registry-credentials`) on the agent: no credential-file reads
or helper execution. Private images requiring authentication will then fail
the check rather than report a fabricated result.

Paid Pulse Pro Docker installs use the private Pulse Pro registry rather than
the public `rcourtman/pulse` image. Open <https://pulserelay.pro/download.html>,
enter your activation key in the page and use its registry login instructions
on the host that already runs Pulse. Persist the private image for both pull
and recreation as described in [server updates](#-updates); an image override
used for only the pull does not select it for the later recreation.

### Disabling Update Features

Pulse provides granular control over update features via environment variables on the **Pulse server**:

| Variable | Description |
|----------|-------------|
| `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS` | Disables image-update actions while still detecting updates. Start/stop/restart have separate controls. |

**Example - Disable image updates** (continue detecting updates):
```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    environment:
      - PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true
```

This setting is not a monitoring-only security boundary: it does not disable
container lifecycle actions or revoke agent command permissions. For
monitoring-only installs, leave command execution disabled on the agent and do
not grant command execution permission to monitoring tokens. See
[container lifecycle requirements](#️-container-lifecycle-actions).

To disable registry checks entirely, set `PULSE_DISABLE_DOCKER_UPDATE_CHECKS=true` on the **agent**.

You can also toggle **Hide update buttons** from the UI in **Settings → System → General** under **Docker / Podman updates**.

---

## ▶️ Container Lifecycle Actions

Pulse can start, stop, and restart Docker / Podman containers directly from the UI. Running containers offer **stop** and **restart**; stopped containers offer **start**.

### Requirements

- **Pulse Agent** installed on the container host (see [Unified Agent](UNIFIED_AGENT.md)) and currently connected
- **Command execution enabled** on the agent — it is disabled by default. Either:
  - start the agent with `--enable-commands` (or `PULSE_ENABLE_COMMANDS=true`), or
  - tick **Allow Pulse to run commands on this host** in the token step of **Settings → Infrastructure** before copying the install command, which adds the flag and grants the token the command execution permission. Pulse can then run commands as root on that host: this is the full-trust combined profile, not a least-privilege setting
  - an agent installed without it cannot be switched on from Pulse; run the new install command on the host
- **Admin approval**: every lifecycle action requires confirmation by an admin in the UI before it runs. The agent then verifies the container's state before the change and confirms it actually reached the requested state afterwards.

### Limitations

- If the Docker daemon has **authorization plugins** configured, Pulse blocks all daemon-mutating commands on that host (see advisory GO-2026-4887) and the lifecycle buttons are not offered. Podman hosts are not affected.
- Actions are unavailable while the host's Docker inventory is stale or the agent is disconnected.

---

## 🛠️ Troubleshooting

- **Forgot Password?**
  Follow the [password recovery guide](TROUBLESHOOTING.md#i-forgot-my-password).
  Update the active credential source; do not delete `.env`, remove the data
  volume or repeat setup. A deployment-supplied password overrides the generated
  file, and changing Docker's managed environment needs a recreate/redeploy,
  not just a restart. SSO accounts and temporary lockouts have separate paths.

- **Logs**
  Use the [bounded Docker log reader](TROUBLESHOOTING.md#inspect-notification-logs)
  on the host running the Pulse server, with its actual container name. It reads
  both output streams with a deadline, time window and record limit; a failed
  read is not an empty log. Review the excerpt privately and share only the
  relevant redacted error, not credentials, private host details or the full log.
  Do not follow logs indefinitely or repeat a failed update to collect evidence.

- **Shell Access**
  ```bash
  docker exec -it pulse /bin/sh
  ```
