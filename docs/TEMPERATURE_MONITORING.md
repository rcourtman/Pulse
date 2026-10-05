# Temperature Monitoring

Pulse collects temperatures exposed by the host's existing drivers. For Linux
hosts, prefer the local agent; SSH is a legacy fallback, not a prerequisite:

- Pulse agent on Proxmox hosts (recommended)
- SSH-based collection from the Pulse server (fallback or for non-agent hosts)

If you are upgrading from older releases that used `pulse-sensor-proxy`, see the legacy cleanup section below. The sensor proxy is no longer supported in Pulse.

## Recommended: Pulse Agent (Proxmox)

The unified agent runs on each Proxmox host and reports temperatures locally
with no SSH keys needed. First complete the
[private-file preparation](UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas)
on that host: protect the Pulse agent token, download the installer from your
Pulse server over verified HTTPS, and inspect it. Then use the same Pulse
address and add the Proxmox collector flag:

```bash
bash "$HOME/.config/pulse/agent-install.sh" \
  --url https://pulse.example.com \
  --token-file "$HOME/.config/pulse/agent-token" --enable-proxmox
```

The Pulse agent token is not a Proxmox API token. Keep the secret out of
command arguments and leave command execution disabled for monitoring.

Notes:
- On Linux, the agent reads `sensors -j` when available and supplements missing
  CPU readings from recognised CPU/SoC thermal sysfs sources. `lm-sensors` is
  not required for that fallback; empty output does not mean a temperature of
  zero. Other sensor types need their own supported driver/provider.
- Check the active agent's version, last report and the affected host's reading
  before changing its setup. A current server does not upgrade every agent.
- When a Proxmox host has recent usable agent temperature data, Pulse treats the agent as the source of truth and does not also try SSH temperature collection for that host.

### Check existing Linux readings safely

Use the monitored host's trusted local console, not the Pulse LXC or container.
Do not run `sensors-detect --auto`, bus scans, driver loading or a reboot merely
because Pulse shows no temperature: hardware probing can disrupt a running
host. Any necessary driver setup belongs in that host's normal maintenance
window, following its hardware documentation, not in a diagnostic retry loop.

If `sensors` is already installed, this Bash check reads it once, limits the
command to five seconds (with a two-second forced-stop grace), and saves both
output streams privately. It installs nothing and prints only status and paths:

```bash
(
set -eu
umask 077
command -v sensors >/dev/null || { printf 'sensors is not installed; check the agent sysfs fallback.\n'; exit 1; }
command -v timeout >/dev/null || { printf 'GNU timeout is required for this bounded check.\n'; exit 1; }
result_dir=$(mktemp -d "${TMPDIR:-/tmp}/pulse-temperature.XXXXXX") || exit $?
check_exit=0
timeout --signal=TERM --kill-after=2s 5s sensors -j \
  > "$result_dir/sensors.json" 2> "$result_dir/sensors.err" || check_exit=$?
printf 'Sensor command exit %s; private output in %s\n' "$check_exit" "$result_dir"
exit "$check_exit"
)
```

Inspect those files locally; non-zero exit can accompany partial sensor data,
and exit zero or `{}` does not establish a usable CPU reading. A timeout is a
failed check, not an empty reading or permission to repeat it. Do not post the
whole output: it can identify hardware and disks.

Without usable CPU output, inspect the already exposed sysfs files locally:
pair `/sys/class/thermal/thermal_zone*/type` with its `temp`, or
`/sys/class/hwmon/hwmon*/name` with its `temp1_input`. Values are millidegrees
Celsius: `57000` is **57 °C**. `thermal_zone0` is not necessarily the CPU;
compare the sensor type/name, not just its number. The agent accepts recognised
CPU/SoC names, not every thermal zone as a CPU. Missing, unreadable, invalid or
unsupported readings remain unavailable; do not change permissions or mount
host device trees into a container just to make this check pass.

Compare an existing local reading with the **same host and sensor**, current
Machines value and CPU-temperature History at the corresponding time. CPU/SoC
temperature is not physical-disk SMART temperature. A plausible value for one
sensor does not verify all sensors or historical continuity.

## Windows temperatures

On Windows hosts with an NVIDIA driver, the unified agent uses the driver's
`nvidia-smi` executable to report GPU temperature, utilization, and VRAM
usage. No extra Pulse configuration is required; `nvidia-smi.exe` must be
available on the agent service's `PATH`.

Supported physical disks are read separately through Windows Storage
reliability counters. Missing counters or unsupported devices are simply
omitted.

Windows does not provide a dependable built-in API for CPU or motherboard
temperatures. For those readings, install and run
[LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor)
as Administrator, leave its port at `8085`, and enable
**Options → Remote Web Server → Run**. Do not allow inbound network access to
port `8085` in Windows Firewall; Pulse reads only the local
`http://127.0.0.1:8085/data.json` endpoint. Web authentication must be disabled
for this loopback integration.

Pulse accepts only bounded CPU and motherboard Celsius readings from that
endpoint. It ignores LibreHardwareMonitor disk and GPU nodes so the native
Windows Storage and `nvidia-smi` providers remain authoritative. If the helper
is absent or unavailable, the rest of the Windows report is unaffected.

## SSH-Based Collection (Fallback)

Pulse can also collect temperatures by SSHing into each host that does not have usable agent temperature data. The SSH path runs the Pulse sensor wrapper when present, falls back to `sensors -j`, and can fall back again to `/sys/class/thermal/thermal_zone0/temp` when available (for example, on Raspberry Pi).

### Requirements

- SSH connectivity from the Pulse server to each host
- Existing usable sensor data on the host; normally `sensors -j` JSON
- A restricted SSH key entry that only allows the Pulse sensor wrapper

### Setup

1. Generate the node setup command from the UI:
   **Settings -> Infrastructure -> Add Node**
2. Run the command on each Proxmox host. The setup script can:
   - Create the required API user and permissions
   - Add a restricted SSH key entry for temperature collection
   - Install `lm-sensors` (optional)

The SSH entry added to `authorized_keys` is restricted to the Pulse sensor wrapper, for example:

```text
command="/usr/local/sbin/pulse-sensors",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <public-key> # pulse-sensors
```

If you use a non-standard SSH port, set `SSH_PORT` (system-wide) or configure it in **Settings -> System**.

### Containerized Pulse

Prefer the local agent. A legacy configured key can still cause SSH fallback
from a container, with a production-security warning; the development override
changes that warning, not whether sensor data exists. Do not mount root SSH
keys into the Pulse container or enable a development override to repair a
missing reading.

### Verification

Only for an already configured SSH fallback, use its existing restricted key
and separately verified host-key trust from the Pulse server's account. Do not
accept a new or changed host key merely to obtain a temperature. This Bash
check requests the sensor wrapper once, with a fifteen-second overall bound
and two-second forced-stop grace, and keeps output private:

```bash
(
set -eu
umask 077
command -v timeout >/dev/null || { printf 'GNU timeout is required for this bounded check.\n'; exit 1; }
result_dir=$(mktemp -d "${TMPDIR:-/tmp}/pulse-temperature-ssh.XXXXXX") || exit $?
check_exit=0
timeout --signal=TERM --kill-after=2s 15s \
  ssh -T -i /path/to/pulse-key -o BatchMode=yes -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=yes -o ConnectTimeout=5 -o ConnectionAttempts=1 \
    -o ServerAliveInterval=5 -o ServerAliveCountMax=1 \
    root@node /usr/local/sbin/pulse-sensors \
    > "$result_dir/wrapper.json" 2> "$result_dir/ssh.err" || check_exit=$?
printf 'SSH command exit %s; private output in %s\n' "$check_exit" "$result_dir"
exit "$check_exit"
)
```

Replace only the key path and node with the configured ones. The forced key
command can ignore the requested command: current wrappers return `sensors`
and `smart` data, while older keys can return bare `sensors -j` JSON. Do not
remove the key restrictions to run another probe, or treat wrapper JSON as a
raw thermal-zone number. An SSH failure, partial output or timeout does not
prove missing hardware. Keep the wrapper output and error private; share only
the exit, relevant redacted error and affected sensor if help is needed.

### Troubleshooting

- If temperatures are unavailable, use the [local reading check](#check-existing-linux-readings-safely) before considering setup changes. An empty reading is not a reason for automatic hardware detection.
- If the unified agent is already reporting temperatures for a Proxmox host, SSH collection is not required for that host.
- Ensure the SSH key entry is present and restricted to `/usr/local/sbin/pulse-sensors`.

## Legacy Cleanup (If Upgrading)

If you still have the old sensor proxy installed from prior releases, remove it
from each **Proxmox host** (not the Pulse container) with the supported cleanup
helper. In that host's trusted administrative shell, download it privately.
This refuses an existing target, symlinked configuration directory or
failed/partial download; it does not overwrite an earlier helper or run a
response as a command:

```bash
(
set -eu
umask 077
config_dir="$HOME/.config/pulse"
helper_file="$config_dir/sensor-proxy-uninstall.sh"
if [ -L "$HOME/.config" ] || [ -L "$config_dir" ]; then
  printf 'Refusing a symlinked configuration directory.\n'; exit 1
fi
mkdir -p "$config_dir" || exit $?
chmod 700 "$config_dir" || exit $?
if [ -e "$helper_file" ] || [ -L "$helper_file" ]; then
  printf 'Helper path already exists; inspect it locally before choosing a replacement.\n'; exit 1
fi
download_file=$(mktemp "$config_dir/sensor-cleanup-download.XXXXXX") || exit $?
curl_exit=0
status=$(curl --disable --fail --silent --show-error --proto '=https' \
  --connect-timeout 5 --max-time 60 --output "$download_file" --write-out '%{http_code}' \
  https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/uninstall-sensor-proxy.sh) || curl_exit=$?
printf 'HTTP %s\n' "$status"
[ "$curl_exit" -eq 0 ] || exit "$curl_exit"
[ "$status" = 200 ] || exit 1
mv -n "$download_file" "$helper_file" || exit $?
[ ! -e "$download_file" ] || exit 1
printf 'Inspect the complete private helper at %s before running it.\n' "$helper_file"
)
```

Stop if preparation or download fails; a temporary response is not the helper.
Curl's local configuration is ignored, redirects are not followed and TLS
verification stays enabled. This is an HTTPS download from the repository's
current main, not a signed release asset. Inspect the complete saved script
and choose the cleanup scope before running it.
Do not pipe a web response into a privileged shell:

```bash
bash "$HOME/.config/pulse/sensor-proxy-uninstall.sh" \
  --uninstall --purge --local-only
```

`--local-only` avoids cluster SSH entirely; run the command once on every
Proxmox node that carried the proxy. For one cluster-wide pass instead, omit
`--local-only`. Remote nodes must already have trusted host keys in root's
normal OpenSSH user/system known_hosts files, or supply a separately
provisioned file with `--ssh-known-hosts /path/to/known_hosts`. The helper never
accepts or enrolls an unknown host key, and a missing or changed key makes the
remote portion fail after local cleanup completes.

If you also want to remove the old `pulse-monitor@pam` API user and tokens before re-adding the node, include `--remove-proxmox-access`:

```bash
bash "$HOME/.config/pulse/sensor-proxy-uninstall.sh" \
  --uninstall --purge --remove-proxmox-access --local-only
```

Reinstalling or upgrading the Pulse container does **not** remove the sensor proxy from the host — they are separate installations. If you skip this cleanup, the selfheal timer will keep running and may generate recurring `TASK ERROR` entries in the Proxmox task log.
