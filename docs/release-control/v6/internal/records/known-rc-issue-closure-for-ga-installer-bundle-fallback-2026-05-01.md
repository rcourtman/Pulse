# Known RC Issue Closure For GA Installer Bundle Fallback Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Lane: `L1`
- Result: `passed`

## Context

The v5 maintenance audit found that `release/5.1` had removed an eager
universal-bundle fallback from the root installer. The old fallback attempted
to download `pulse-${version}.tar.gz` during install when every cross-arch
agent binary was not already present locally.

The v6 root installer still carried the same fallback for unified-agent
binaries. That made a normal install perform an unnecessary second network
lookup for a non-arch bundle and could produce a confusing warning even after
the actual platform archive had installed successfully.

## Disposition

The v6 installer now matches the maintenance-line behavior:

- bundled unified-agent binaries are still copied from the extracted release
  archive when present;
- the installer no longer tries to download a universal cross-architecture
  agent bundle during the main install path;
- missing unified-agent binaries are left for the on-demand agent install path
  instead of making the server installer depend on a fallback release asset.

This is the canonical root fix for RC3 because the install-time dependency was
owned by the release installer, not by downstream agent setup commands.

## Proof

- `go test ./scripts/installtests -run 'TestInstallAdditionalAgentBinaries|TestResolveInstallScriptDownloadURL|TestInstallSHRequiresPinnedSignatureVerificationForReleaseDownloads' -count=1`
- `bash -n install.sh`
- `git diff --check`

## Outcome

The v5 installer bundle-fallback fix is ported to the current v6 candidate. The
`known-rc-issue-closure-for-ga` gate remains satisfied for this slice.
