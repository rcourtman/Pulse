# 💾 VM Disk Monitoring

Pulse can read filesystem usage inside a Proxmox VM through the QEMU Guest
Agent. Allocated virtual-disk size is not the same as used space inside the
guest. LXC filesystem collection uses a different path.

## Backup safety

**Do not run manual guest-agent probes during a backup, freeze or thaw.** A
guest-agent request is an active command on the same channel used by
freeze-enabled backups, even when it only reads disk or memory information.
There is a [report of a VM remaining frozen despite an OK backup task](https://github.com/rcourtman/Pulse/issues/2439).
The reported command-ID collision is not established as a reproduced cause.

If your installation is affected, stop the Pulse service before the backup
and keep it stopped until the backup has ended and independent checks confirm
**thaw, fresh successful writes to every filesystem covered by the backup, and
workload liveness**. Use your established guest-console or workload checks, not
Pulse readings or guest-agent probes. Restore only services and timers that were
active before the pause. **Pulse monitoring and alerts are unavailable while it
is stopped.** An OK backup task or an absent VM lock
does not prove thaw succeeded. This is a temporary precaution, not a claim
that the monitoring defect is fixed.

Do not test freeze/thaw commands, clear backup locks, disable backup freezing
or force-reset a guest as a disk-monitoring diagnostic. Those operations can
affect workloads or backup consistency. Preserve the existing backup task and
redacted error, rather than repeat a potentially harmful backup for a report.

### Pause Pulse for a planned freeze-enabled backup

This manual precaution is for an affected **systemd installation**, before a
scheduled backup starts. It is **not a recovery procedure** for a backup already
in progress or an unresponsive guest, and **not an automatic backup hook**.
For a Docker or Compose server, use the [container pause procedure](#pause-a-docker-or-compose-server-for-a-planned-backup) instead;
the systemd commands below do not stop a containerised Pulse server.

1. **Identify the Pulse server service.** Run these commands on the machine
   running the Pulse server: for Proxmox LXC, **inside the Pulse LXC**,
   **not the Proxmox host or the backed-up VM**. The examples use
   `pulse.service`; substitute your actual server unit throughout
   (`pulse-backend.service` on some older installs), **not pulse-agent.service**.

   ```bash
   systemctl show pulse.service --property=LoadState,ActiveState,MainPID
   ```

   Require `LoadState=loaded` and record whether Pulse was active. If the unit
   is missing, do not assume monitoring has stopped: identify your deployment's
   actual stop control. Docker and other supervisors need their own controls.

2. **Prevent an update from starting Pulse again.** Do not install or update
   Pulse during this window. If you use unattended updates, inspect the actual
   matching timer (the default is `pulse-update.timer`):

   ```bash
   systemctl show pulse-update.timer --property=LoadState,ActiveState
   ```

   Record whether the timer was active. For a loaded timer, pause it and check
   both it and its update service, substituting your actual unit names:

   ```bash
   sudo systemctl stop pulse-update.timer
   systemctl show pulse-update.timer pulse-update.service \
     --property=Id,LoadState,ActiveState,MainPID
   ```

   The timer must be inactive. If the update service is active, activating or
   deactivating, let it finish normally before continuing. Do not interrupt an
   installation. A missing update unit is not proof that a custom updater is
   idle; check your deployment's update controls too.

3. **Stop Pulse and confirm it is stopped, before the backup starts.**

   ```bash
   sudo systemctl stop pulse.service
   systemctl show pulse.service \
     --property=LoadState,ActiveState,MainPID,Result,ExecMainCode,ExecMainStatus
   ```

   Require `LoadState=loaded`, `ActiveState=inactive` and `MainPID=0` after a
   successful stop. Do not start the backup if the stop fails, the state is
   unknown or another Pulse server is still polling the affected guests.
   **Pulse monitoring and alerts are unavailable while stopped.** Do not run
   an update, start another Pulse instance or reboot its host during the pause.

   For a server stopped in this step, also require `Result=success`,
   `ExecMainCode=1` (normal process exit) and `ExecMainStatus=0`. A timeout,
   signal termination, non-zero exit or unavailable shutdown result needs
   normal maintenance before the backup. Do not force-kill Pulse or clear its
   failed state to satisfy this check. Leave a previously inactive server
   inactive; an old exit result is not evidence of a new shutdown.

   **Stopping Pulse does not cancel a guest-agent request already issued.**
   Even a successful process exit does not prove that an earlier request
   finished inside Proxmox or the guest. Let existing guest/backup operations
   finish normally before the planned backup, using your established task and
   workload checks, not new guest-agent probes. If their state is unknown,
   do not start the backup on the strength of a stopped service or an arbitrary
   waiting period. A disk dash or cooldown is not evidence of completion.

4. **Verify the guest independently, then restore only what you paused.**
   Keep Pulse stopped until the backup has ended **and** your established
   guest-console or workload checks confirm all three: **thaw**, **fresh successful
   workload writes to every filesystem covered by the backup**, and **workload
   liveness**. Use evidence from **after the backup ended**, independent of Pulse
   and the QEMU Guest Agent. A console connection or a successful read alone is
   not enough; neither is a write to only the OS disk when the backup covers
   other filesystems. Use the workload's normal safe checks, not forced writes,
   test-file commands or a new freeze/thaw cycle. If any check is unavailable or
   fails, leave Pulse stopped and use the guest/platform's recovery procedure;
   do not repeat the backup or use guest-agent probes to test it.

   Apply the recorded pre-pause states separately:

   | Pulse server before pause | Update timer before pause | Restore after all safety checks pass |
   | --- | --- | --- |
   | Active | Active | Start Pulse and confirm it is active; then restore and check the timer. |
   | Active | Inactive | Start Pulse and confirm it is active; leave the timer inactive. |
   | Inactive | Active | Leave Pulse inactive. Restore the timer only if its installed updater is confirmed not to start an inactive Pulse server; otherwise keep it paused until that behaviour is resolved through normal maintenance. |
   | Inactive | Inactive | Leave both inactive. |

   If either pre-pause state is unknown, do not guess or start either unit;
   establish the original states before restoring.

   Only after all safety checks pass, if Pulse was active beforehand:

   ```bash
   sudo systemctl start pulse.service
   systemctl is-active pulse.service
   ```

   `active` confirms service startup, not proof that polling or alerts have
   recovered. Check normal observation times in Pulse without Run Diagnostics
   or manual guest-agent probes. If startup fails or its state is unknown, keep
   the timer paused and resolve the startup failure through normal maintenance.
   Restore the update timer **only if it was active beforehand**, using the
   matching row above:

   ```bash
   sudo systemctl start pulse-update.timer
   systemctl is-active pulse-update.timer
   ```

   Require `active` for a timer you restored. A persistent timer may run a missed
   update immediately, so do not restore it before the safety checks. The current
   Pulse installer makes its updater skip an inactive Pulse server; an older or
   customised installed unit may differ. A timer's `active` state does not prove
   that an update or monitoring succeeded.

   Leave previously inactive services/timers inactive. Arrange independent
   outage coverage and repeat the precaution for each affected backup window;
   this manual sequence does not schedule future pauses or prove a fix.

### Pause a Docker or Compose server for a planned backup

This is a manual precaution for an affected **standalone Docker or plain Compose
Pulse server**, before a scheduled freeze-enabled backup. It is **not incident
recovery or an automatic backup hook**. Swarm, Kubernetes and other controllers
need their own supported pause controls: stopping a container alone does not
stop a controller from replacing it. Do not use this procedure if you cannot
prevent another server from polling the affected guests.

**Pulse monitoring and alerts are unavailable while stopped.** Arrange
independent outage coverage. A container stop does not cancel a guest-agent
request already issued; let any existing guest/backup operation finish normally
before the planned backup. If that state is unknown, do not start the backup on
the strength of a stopped container.

1. **Identify the existing Pulse server container on its Docker host**, not a
   monitored workload, the backed-up VM or a Pulse Agent container. For Compose,
   use the original project directory and the same project/file options used to
   deploy it. The example service is `pulse`; substitute your actual service:

   ```bash
   docker compose ps --all --quiet pulse
   ```

   Record every returned server container ID privately. An empty list or a
   failed command is not proof that monitoring has stopped. For a `docker run`
   installation, identify the existing server through its saved deployment.
   Set the ID explicitly, then read only its identity and state:

   ```bash
   PULSE_CONTAINER_ID='paste-the-recorded-server-container-id'
   docker inspect --format 'Id={{.Id}} Running={{.State.Running}} Paused={{.State.Paused}} Restarting={{.State.Restarting}} Status={{.State.Status}} Pid={{.State.Pid}} ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}' "$PULSE_CONTAINER_ID"
   ```

   Record whether this exact container was running before the pause. Require a
   stable state: either running and not paused/restarting, or already exited
   with PID 0. If it is paused, restarting, being replaced or its state cannot
   be read, resolve that through the deployment's normal controls first. Repeat
   the state check for every server; do not infer another container's state.
   Do not share full `docker inspect`, container environments or resolved
   Compose configuration: they can contain credentials.

2. **Prevent a deployment or updater from restarting/replacing Pulse.** Do not
   install, update, recreate the server, run `docker compose up` or reboot the
   Docker host during this window. Pause any automatic updater or deployment
   job using its supported controls, recording whether it was active first.
   Let an in-progress update finish normally; do not interrupt an installation.
   The supplied Compose example uses `restart: unless-stopped`, but that does
   not prevent an external updater or another operator from starting Pulse.
   Do not change the restart policy, image, mounts or saved deployment as a
   substitute for these checks.

3. **Stop each previously running server and verify the same ID is stopped.**
   Docker commands on the recorded existing container also work for a plain
   Compose deployment; they do not recreate its configuration or data volume:

   ```bash
   docker stop --timeout 60 "$PULSE_CONTAINER_ID"
   docker inspect --format 'Id={{.Id}} Running={{.State.Running}} Paused={{.State.Paused}} Restarting={{.State.Restarting}} Status={{.State.Status}} Pid={{.State.Pid}} ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}' "$PULSE_CONTAINER_ID"
   ```

   Require a successful stop and readback of the **same recorded ID** with
   `Running=false`, `Paused=false`, `Restarting=false`, `Status=exited` and
   `Pid=0`. For a server stopped in this step, also require `ExitCode=0` and
   `OOMKilled=false`; a non-zero exit (including 137 after forced termination)
   or unknown shutdown result needs normal maintenance before the backup.
   Leave an already stopped server stopped. If stopping times out,
   fails, the identity changes or any state is unknown, do not start the backup.
   Do not delete/recreate a container, remove volumes or force-kill it to clear
   this check. Docker can terminate a process after its stop timeout; stopped
   state alone does not prove graceful shutdown or completion of earlier guest
   requests. Preserve a shutdown failure for normal maintenance, not a forced
   backup test.

4. **Keep every server stopped until independent recovery checks pass.** The
   backup must have ended, and evidence from **after it ended**, independent of
   Pulse and the QEMU Guest Agent, must confirm **thaw, fresh successful workload
   writes to every filesystem covered by the backup, and workload liveness**.
   Use the workload's established safe checks, not forced writes, test files or
   another freeze/thaw cycle. An OK backup, an absent lock, a console connection
   or a successful read alone is not enough. If any check fails or is unavailable,
   leave Pulse and its automatic updater paused and use the platform's recovery
   procedure, without guest-agent probes or repeating the backup.

   Only for a container recorded as running before the pause, start that
   **same existing ID**, then repeat the bounded state readback from step 1:

   ```bash
   docker start "$PULSE_CONTAINER_ID"
   ```

   Require the original ID, `Running=true`, `Paused=false`, `Restarting=false`
   and `Status=running`. A previously stopped container stays stopped; if its
   original state is unknown, do not guess. Do not use a blanket Compose start
   or `up` to restore a mixture of previously running and stopped servers. If
   the original container is missing or startup fails, keep automation paused
   and resolve that through normal maintenance rather than creating another
   server against the same data.

   Running state is not proof of healthy collection or alert delivery. Check
   normal observation times in Pulse without Run Diagnostics or guest-agent
   probes. Restore only automatic jobs that were active before the pause, after
   verifying they will not start a previously inactive server. Unknown original
   job state or restart behaviour means leave that job paused. Repeat the
   precaution for each affected backup window; this is not proof of a fix.

## 🚀 Setup

Plan changes outside backup windows and follow the guest's normal maintenance
procedure; a reboot interrupts its workloads.

1. **Install the guest agent inside the VM.** Use the guest OS's supported
   package: `qemu-guest-agent` on Linux, or the QEMU Guest Agent service from
   the virtio-win distribution on Windows.
2. **Enable it in Proxmox.** Review **VM Options → QEMU Guest Agent** and apply
   any required VM restart in that maintenance window.
3. **Check the existing observations.** In Pulse, hover over the disk dash or
   open the resource details for the collection explanation and observation
   time. Check the guest-agent service from inside the guest, not by repeatedly
   sending `qm agent` commands from the hypervisor. A configured agent is not
   proof it responded, and a retained disk value is not proof of a fresh poll.

## ⚙️ Permissions

Use the dedicated account and token actually configured for that Pulse
connection, not a guessed account name. Review their effective privileges for
the affected VM; a host-root diagnostic does not test the API token's access.
With privilege separation enabled, access is the intersection of user and token
permissions: both must allow the required read on the affected VM. Do not disable
privilege separation or recreate the token to diagnose a missing reading. See
[Check permissions](TROUBLESHOOTING.md#check-permissions-proxmox) for the scoped
inspection and repair boundary. A permitted read does not prove disk freshness,
responsiveness or thaw.

| Collection | Proxmox VE 9+ | Proxmox VE 8 |
| --- | --- | --- |
| Filesystem usage and guest information | `VM.GuestAgent.Audit` | `VM.Monitor` |
| Reading guest memory via `/proc/meminfo` | Also `VM.GuestAgent.FileRead` | `VM.Monitor` |

Do not grant Administrator or guest execution/write permissions to diagnose a
missing disk value. The built-in `PVEAuditor` role cannot be modified; use a
custom monitoring role if the required read privileges are absent.

## 🔧 Troubleshooting

### A missing reading is not an installation diagnosis

Pulse's **“agent not running”** disk explanation means Proxmox could not query
the guest agent. It can also cover a general guest-agent HTTP 500 response; it
does not establish whether an agent is absent or stopped. An unknown explanation
establishes neither. **Do not install, enable or restart an agent solely to clear
a disk dash.**

Read [Backup safety](#backup-safety) before changing anything. During a backup,
freeze/thaw or an unresponsive-guest incident, defer setup and live probes and
use the guest/platform's established recovery procedure. A running VM, an absent
lock or an OK backup does not prove thaw. Keep the monitoring-outage precaution
until independent checks confirm thaw, fresh successful writes to **every
filesystem covered by the backup** and workload liveness. Restore only services
and timers that were previously active.

When the guest is responsive and outside those conditions, review its existing
agent configuration and guest-local service through your normal console. Use
[Setup](#-setup) only if it actually needs configuration and the guest OS has a
supported QEMU Guest Agent. The Linux package name is not a Windows or Android
installation instruction. If no supported agent is available, use the guest's
own filesystem tools; missing Pulse usage is unknown, not zero.

### Use existing Machines readings without changing the guest

The Proxmox disk view and an in-guest **Pulse Agent** use different collection
paths. A paused Proxmox guest-agent read does not mean the Pulse Agent has
stopped reporting. If the affected VM **already has a Pulse Agent** and an
existing entry in **Machines**, check that view's filesystem readings and
History before changing anything in the guest.

Confirm the Machines entry belongs to the **same guest**, using its existing
identity and host details, not a similar name alone. Check the filesystem and
its observation time: a retained History sample or an agent's recent contact
is not proof of a current disk reading. Use available, current readings there
as an interim view; this does not repair the missing Proxmox reading or prove
that the two entries are linked correctly. If the guest has no existing Pulse
Agent entry or no current filesystem readings, use its own filesystem tools
through your normal interface. **Do not install another agent, restart services,
restore a VM or force guest-agent checks just to recover these readings.**

This alternative does **not** relax [Backup safety](#backup-safety). A responsive
guest, running agent services, working Machines readings or an expired cooldown
is not proof of thaw and successful writes to every filesystem covered by the
backup. Keep the monitoring-outage precaution until those independent checks
pass; do not clear or bypass a guest-read pause to test recovery.

| Observation | Useful next check |
| --- | --- |
| **Disk shows “-”** | Read its explanation and observation time. Check the owning host, current VM options, guest-local service and configured API token access. |
| **Permission denied** | Review the token and account's effective read privileges on that VM, including privilege-separated token ACLs. A successful root command would not disprove this error. |
| **Timeout or backup lock** | Check the existing backup/task timeline and guest workload locally. Defer active guest-agent probes; increasing a timeout is not a contention or thaw repair. |
| **Rocky Linux / RHEL memory missing** | Review `/etc/sysconfig/qemu-ga` inside the guest. File-read restrictions can explain memory collection failure; they do not by themselves prove why filesystem usage is absent. Change the guest's allowlist or restart its agent only through your normal maintenance procedure, outside backups. |
| **Windows service stopped** | Check the QEMU Guest Agent service inside Windows. Schedule any restart outside backups. |

### Passive host preflight

The current `scripts/test-vm-disk.sh` helper reads only local VM status and
current configuration, with bounded timeouts and byte limits. It sends **no guest-agent
commands**, reads no guest files, and changes no service, lock or ACL. It requires
local `qm`, `timeout` and Python 3; it downloads nothing. Use it
on the VM's owning Proxmox host with an account permitted to read that local
configuration (normally through `sudo`).

Use a reviewed checkout containing this passive helper. Inspect the file
before running it: older copies send `ping` and `get-fsinfo` to the guest. Do
not pipe a network download into a root shell.

```bash
# From the reviewed Pulse checkout; replace 100 with the affected VM ID.
sudo bash ./scripts/test-vm-disk.sh 100
```

The output distinguishes the running/stopped state, agent configuration and
a reported backup/other lock without dumping the full VM configuration. A
missing or unreadable lock is not permission to probe the guest: a backup can
start immediately after the read. A non-zero exit means a required read failed
or the response could not be interpreted; it is not an empty or healthy result.
Status replies are limited to 256 bytes and configuration to 64 KiB. Oversized
or corrupt replies fail without interpreting a truncated prefix, printing raw
configuration or saving it to disk. Inspect the configuration privately rather
than retrying with guest-agent probes.
**A successful preflight does not verify disk freshness, guest-agent
responsiveness or thaw.**

Keep only the relevant time, Pulse and Proxmox versions, observation/error and
backup task sequence for a report. Redact private VM names, paths and addresses;
never include credentials, full VM configuration, service environments or a
whole host journal. See [Getting help](TROUBLESHOOTING.md#-getting-help).

## 📝 Notes

- **Network mounts:** NFS/SMB mounts are excluded from filesystem usage.
- **Databases:** Filesystem usage can differ from database-internal metrics.
- **Containers:** LXC collection does not use the VM's QEMU Guest Agent.
