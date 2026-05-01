# Self-Hosted Installer Disk Preflight Record

- Date: `2026-05-01`
- Lane: `L1`
- Subsystem: `deployment-installability`
- Result: `passed`

## Context

The v5 maintenance audit found `release/5.1` commit `4de1c3745`
(`Preflight disk space before Pulse updates`). That fix prevents small LXC or
single-disk installs from stopping the running Pulse service and then failing
part-way through update staging or extraction because `/tmp` or `/opt/pulse`
does not have enough free space.

The current v6 candidate already had update-manager extraction/backup
space checks, but the root server installer still lacked the earlier
operator-facing preflight.

## Disposition

The v6 root installer now performs a disk-space preflight before it detects and
stops the active Pulse service or downloads/applies the release archive:

- `/tmp` must have enough staging headroom;
- the install filesystem must have apply headroom;
- when `/tmp` and the install directory share the same filesystem, the
  required space is combined and checked as one capacity constraint;
- failure reports the available and required sizes before any service stop.

`internal/updates/adapter_installsh.go` now reflects the same operator-facing
requirement in the update plan prerequisites.

## Proof

- `bash -n install.sh`
- `go test ./internal/updates -run 'TestInstallShAdapter_PrepareUpdate|InstallSh' -count=1`
- `go test ./scripts/installtests -run 'RootInstallScript.*(ArchiveSupport|UpdateDiskHeadroom)|RootInstallScriptRequiresSignedReleaseDownloads' -count=1`

## Outcome

The v5 installer disk-space preflight is ported to the current v6 candidate.
The self-hosted release confidence lane remains at the RC floor for this
slice.
