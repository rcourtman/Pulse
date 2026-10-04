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
and restart it only after independently confirming the guest has thawed, using
your existing guest console or workload checks. **Pulse monitoring and alerts
are unavailable while it is stopped.** An OK backup task or an absent VM lock
does not prove thaw succeeded. This is a temporary precaution, not a claim
that the monitoring defect is fixed.

Do not test freeze/thaw commands, clear backup locks, disable backup freezing
or force-reset a guest as a disk-monitoring diagnostic. Those operations can
affect workloads or backup consistency. Preserve the existing backup task and
redacted error, rather than repeat a potentially harmful backup for a report.

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

| Collection | Proxmox VE 9+ | Proxmox VE 8 |
| --- | --- | --- |
| Filesystem usage and guest information | `VM.GuestAgent.Audit` | `VM.Monitor` |
| Reading guest memory via `/proc/meminfo` | Also `VM.GuestAgent.FileRead` | `VM.Monitor` |

Do not grant Administrator or guest execution/write permissions to diagnose a
missing disk value. The built-in `PVEAuditor` role cannot be modified; use a
custom monitoring role if the required read privileges are absent.

## 🔧 Troubleshooting

| Observation | Useful next check |
| --- | --- |
| **Disk shows “-”** | Read its explanation and observation time. Check the owning host, current VM options, guest-local service and configured API token access. |
| **Permission denied** | Review the token and account's effective read privileges on that VM, including privilege-separated token ACLs. A successful root command would not disprove this error. |
| **Timeout or backup lock** | Check the existing backup/task timeline and guest workload locally. Defer active guest-agent probes; increasing a timeout is not a contention or thaw repair. |
| **Rocky Linux / RHEL memory missing** | Review `/etc/sysconfig/qemu-ga` inside the guest. File-read restrictions can explain memory collection failure; they do not by themselves prove why filesystem usage is absent. Change the guest's allowlist or restart its agent only through your normal maintenance procedure, outside backups. |
| **Windows service stopped** | Check the QEMU Guest Agent service inside Windows. Schedule any restart outside backups. |

### Passive host preflight

The current `scripts/test-vm-disk.sh` helper reads only local VM status and
current configuration, with bounded timeouts. It sends **no guest-agent
commands**, reads no guest files, and changes no service, lock or ACL. Use it
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
