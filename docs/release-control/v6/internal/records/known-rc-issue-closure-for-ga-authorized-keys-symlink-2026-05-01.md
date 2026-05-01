# Known RC Issue Closure For GA Authorized Keys Symlink Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The v5 maintenance delta audit found that `#1297` had only been partially
carried into v6. The v6 Proxmox setup script already escaped the temperature
monitoring forced-command key entry, but its install and uninstall paths still
rewrote `/root/.ssh/authorized_keys` directly.

That preserved the old regression class on Proxmox hosts where
`/root/.ssh/authorized_keys` is a symlink into `/etc/pve/priv/`: filtering old
Pulse keys through a local temp file and moving it back to the symlink path can
replace the symlink instead of updating the canonical target.

## Disposition

The v6 setup script renderer now owns the root fix in the generated shell
contract:

- The PVE setup script resolves `/root/.ssh/authorized_keys` with `readlink -f`
  before install or uninstall edits.
- Both install and uninstall paths use the resolved path, not the symlink path.
- Both paths write through a `/tmp/.pulse-authorized-keys.*` temp file and use a
  shared install helper with a copy fallback when a direct move cannot update the
  target filesystem.
- The uninstall path removes all Pulse-managed SSH key lines by matching
  `# pulse-`, so the current `# pulse-sensors` key is also removed.
- The forced-command key quoting fix remains covered by the existing generated
  script assertion.

## Proof

- `go test ./internal/api -run 'PVESetupScript(Quotes|Preserves)|Contract_SetupScriptEmbedsFailFastGuidance' -count=1`
- `go test ./internal/api -count=1 -json > /tmp/pulse-internal-api-test.json`

## Outcome

The v6 candidate no longer knowingly regresses the v5 `#1297` setup-script
fix. Temperature-monitoring setup and complete removal now preserve the
Proxmox-managed `authorized_keys` symlink while still cleaning up Pulse-managed
SSH key entries.
