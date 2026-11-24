# Pulse Homebrew Tap (`rcourtman/homebrew-tap`)

This directory holds the formula template ([pulse.rb.tmpl](pulse.rb.tmpl)) for installing the signed `pulse` workstation CLI and MCP client via Homebrew:

```sh
brew install rcourtman/tap/pulse
```

## Automated publication on stable releases

`.github/workflows/publish-homebrew-tap.yml` runs automatically when a stable GitHub release is published (ignoring drafts and prereleases) and can also be run manually via `workflow_dispatch` with a published tag (for example, `v6.9.0`).

The workflow:

1. Downloads the four `pulse-cli-v<VERSION>-{darwin,linux}-{arm64,amd64}.tar.gz` archives and `checksums.txt` from the published GitHub release.
2. Verifies all four archive SHA-256 digests against `checksums.txt`.
3. Renders `Formula/pulse.rb` using `scripts/render-homebrew-formula.sh`.
4. Uploads the rendered formula as a workflow artifact (`homebrew-formula-<tag>`).
5. Pushes `Formula/pulse.rb` to `rcourtman/homebrew-tap` when the `HOMEBREW_TAP_TOKEN` repository secret is configured.

## One-time setup

1. Create the public tap repository:

```sh
gh repo create rcourtman/homebrew-tap \
  --public \
  --description "Homebrew tap for Pulse CLI (rcourtman/Pulse)" \
  --license MIT
```

2. Create a fine-grained GitHub personal access token with **Contents: Read and write** scoped solely to `rcourtman/homebrew-tap`, then store it on `rcourtman/Pulse`:

```sh
gh secret set HOMEBREW_TAP_TOKEN --repo rcourtman/Pulse
```

3. For an already-published stable release, trigger the workflow once to populate `Formula/pulse.rb`:

```sh
gh workflow run publish-homebrew-tap.yml --repo rcourtman/Pulse -f tag=v6.9.0
```

## Manual local rendering

To render a formula locally from a downloaded release directory:

```sh
bash scripts/render-homebrew-formula.sh \
  --version v6.9.0 \
  --release-dir ./release-cli \
  --output ./Formula/pulse.rb
```
