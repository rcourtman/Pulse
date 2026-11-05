#!/usr/bin/env bash
# Full source proof only. Requires a Linux amd64 build worker, Go, Python,
# Git, OpenSSL, ssh-keygen and QEMU's ARM user emulator. It never installs or publishes artifacts.
set -euo pipefail
umask 077
# Cross-repository Git operations must not inherit another checkout's index or
# object/ref namespace (for example when invoked from a hook).
unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_PREFIX GIT_GRAFT_FILE GIT_NO_REPLACE_OBJECTS GIT_REPLACE_REF_BASE GIT_SHALLOW_FILE
qemu=${PULSE_TEST_QEMU_ARM:-qemu-arm}
output=
while (($#)); do
  case "$1" in
    --qemu-arm) qemu=$2; shift 2 ;;
    --output-dir) output=$2; shift 2 ;;
    --help) echo "Usage: $0 [--qemu-arm PATH] [--output-dir NEW_PRIVATE_DIRECTORY]"; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done
case "$(uname -s):$(uname -m)" in Linux:x86_64|Linux:amd64) ;; *) echo 'This proof requires a Linux amd64 build worker.' >&2; exit 2 ;; esac
qemu=$(command -v "$qemu") || { echo 'QEMU ARM is required; supply --qemu-arm or PULSE_TEST_QEMU_ARM.' >&2; exit 2; }
qemu="$(cd "$(dirname "$qemu")" && pwd)/$(basename "$qemu")"
test -x "$qemu"
for dependency in go python3 git openssl ssh-keygen; do
 command -v "$dependency" >/dev/null || { echo "Missing required proof tool: $dependency" >&2; exit 2; }
done
root=$(git -C "$(dirname "$0")/.." rev-parse --show-toplevel)
cd "$root"
git diff --quiet || { echo 'Stage source changes before proving the exact Git tree.' >&2; exit 2; }
source_head=$(git rev-parse HEAD)
source_tree=$(git write-tree)
if [[ -z "$output" ]]; then output=$(mktemp -d "${TMPDIR:-/tmp}/pulse-agent-identity.XXXXXX"); else mkdir -m 0700 "$output"; fi
output=$(cd "$output" && pwd)
exec > >(tee "$output/proof.log") 2>&1
printf 'Source HEAD: %s\nSource tree: %s\nPrivate proof directory: %s\n' "$source_head" "$source_tree" "$output"
printf '%s\n' "$source_head" > "$output/source-head"
printf '%s\n' "$source_tree" > "$output/source-tree"
mkdir "$output/repos" "$output/canonical" "$output/arm"
# An independent index preserves all staged source, including files omitted by
# export-ignore rules. Its source objects remain bound to the captured tree.
git clone --shared --no-checkout --quiet "$root" "$output/repos/pulse"
git -C "$output/repos/pulse" read-tree "$source_tree"
git -C "$output/repos/pulse" checkout-index --all
cd "$output/repos/pulse"
export GOWORK=off GOMAXPROCS=${GOMAXPROCS:-2}
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-p=1"
# API protocol tests need a Go embed input. This private fixture is never a
# frontend build or release payload, and is created only in the source copy.
mkdir -p internal/api/frontend-modern/dist
if [[ ! -f internal/api/frontend-modern/dist/index.html ]]; then
  printf '<html>Agent identity protocol test fixture</html>\n' > internal/api/frontend-modern/dist/index.html
fi
source scripts/release_build_targets.sh
build_agent() {
  local label=$1 version=$2 target=$3 directory=$4 flags
  local -a target_env
  read -r -a target_env <<< "$(pulse_release_target_env "$target")"
  flags=$(./scripts/release_ldflags.sh agent --version "$version")
  env CGO_ENABLED=0 "${target_env[@]}" go build -trimpath -buildvcs=false -ldflags="$flags" -o "$directory/$label" ./cmd/pulse-agent
}
for target in linux-amd64 windows-amd64 darwin-arm64; do build_agent "$target" 6.2.1 "$target" "$output/canonical"; done
build_agent longer-stable 6.2.10 linux-amd64 "$output/canonical"
build_agent old-armv6 6.1.0 linux-armv6 "$output/arm"
build_agent new-armv6 6.2.1 linux-armv6 "$output/arm"
build_agent new-armv7 6.2.1 linux-armv7 "$output/arm"
sha256sum "$output/canonical/"* "$output/arm/"* > "$output/artifacts.sha256"
cat "$output/artifacts.sha256"
go run ./cmd/verify-agent-release --version 6.2.1 \
 linux-amd64 "$output/canonical/linux-amd64" windows-amd64 "$output/canonical/windows-amd64" \
 darwin-arm64 "$output/canonical/darwin-arm64" linux-armv6 "$output/arm/new-armv6" linux-armv7 "$output/arm/new-armv7"
if go run ./cmd/verify-agent-release --version 6.2.1 linux-armv6 "$output/arm/new-armv7"; then
 echo 'Packaging verifier accepted a wrong-target copied artifact.' >&2
 exit 1
fi
export PULSE_AGENT_RELEASE_TEST_DIR="$output/canonical"
go test -p 1 -parallel 1 ./cmd/verify-agent-release ./internal/agentidentity/... ./internal/agentupdate ./internal/updates ./cmd/pulse-agent -count=1 -timeout=15m
go test -p 1 -parallel 1 ./internal/api -run 'Test(ProxyRejectsInvalidReleaseVersion|UnifiedAgent|DownloadUnifiedAgent|ValidateUnifiedAgent|ExpectedAgentBinaryVersion|CanonicalAgentDownload|MockModeToggleEndpoint)' -count=1 -timeout=10m
go test -p 1 -parallel 1 ./scripts/installtests -run 'TestSecureRuntimeSourceManifestCoversTransitiveProviders|TestAgentBuildRecipe|TestReleaseLdflags|TestUniversalAgentWrapper|TestNativeLifecycleCollectors|TestAgentInstallerARMv6SavedStateRepairPreflight|TestAgentInstallerSavedStateRepairVerifiedReplacement|TestPulseAgentStateDirLifecycleIntegration' -v -count=1 -timeout=10m | tee "$output/installer-controls.log"
grep -q -- '--- PASS: TestSecureRuntimeSourceManifestCoversTransitiveProviders' "$output/installer-controls.log"
grep -q -- '--- PASS: TestReleaseLdflagsVersionGrammarIsSealed' "$output/installer-controls.log"
# These focused controls open no Docker socket or TLS listener.
go test ./internal/dockeragent -run '^TestDetermineSelfUpdateArch' -count=1 -v | tee "$output/dockeragent-native-target.log"
grep -q -- '--- PASS: TestDetermineSelfUpdateArch ' "$output/dockeragent-native-target.log"
(cd scripts/tests && python3 -m unittest test_agent_docs.AgentDocsTest.test_changed_unix_recipes_parse)
# A passing top-level event is required; opt-in test skips cannot satisfy this
# driver even if their Go process exits successfully.
go test ./internal/agentidentity -run '^TestCanonicalReleaseExecutables$' -v -count=1 | tee "$output/canonical-executables.log"
grep -q -- '--- PASS: TestCanonicalReleaseExecutables' "$output/canonical-executables.log"
go test ./internal/api -run '^TestCanonicalAgentDownloadExecutables$' -v -count=1 | tee "$output/canonical-download.log"
grep -q -- '--- PASS: TestCanonicalAgentDownloadExecutables' "$output/canonical-download.log"
go test ./scripts/installtests -run '^TestAgentInstallerSavedStateRepairVerifiedReplacement$' -v -count=1 | tee "$output/installer-replacement.log"
grep -q -- '--- PASS: TestAgentInstallerSavedStateRepairVerifiedReplacement' "$output/installer-replacement.log"
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go test -c -o "$output/arm/update-tests" ./internal/agentupdate
for cpu in arm1176 cortex-a7; do
  PULSE_TEST_QEMU_ARM="$qemu" PULSE_TEST_ARM_CPU="$cpu" PULSE_TEST_ARM_ASSETS="$output/arm" \
    "$qemu" -cpu "$cpu" "$output/arm/update-tests" -test.run '^TestARMv6ExecutableUpdateUnderEmulation$' -test.v -test.timeout=3m | tee "$output/arm-$cpu.log"
  grep -q -- '--- PASS: TestARMv6ExecutableUpdateUnderEmulation' "$output/arm-$cpu.log"
done
# Exercise the native assertion's GOARM branch, not only amd64 host facts.
for arm_level in 6 7; do
 CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM="$arm_level" go test -c -o "$output/arm/dockeragent-tests-armv$arm_level" ./internal/dockeragent
 cpus=(cortex-a7)
 if [[ "$arm_level" == 6 ]]; then cpus=(arm1176 cortex-a7); fi
 for cpu in "${cpus[@]}"; do
  "$qemu" -cpu "$cpu" "$output/arm/dockeragent-tests-armv$arm_level" -test.run '^TestDetermineSelfUpdateArch$' -test.v -test.timeout=2m | tee "$output/dockeragent-armv$arm_level-$cpu.log"
  grep -q -- '--- PASS: TestDetermineSelfUpdateArch ' "$output/dockeragent-armv$arm_level-$cpu.log"
 done
done
python3 scripts/release_control/secure_runtime_attestation_v6_test.py
python3 scripts/release_control/secure_runtime_attestation_v7_test.py
python3 scripts/release_control/secure_runtime_rootful_attestation_v1_test.py
python3 scripts/release_control/secure_runtime_rootless_attestation_v1_test.py
git diff --quiet
test "$(git write-tree)" = "$source_tree"
cd "$root"
git diff --quiet
test "$(git write-tree)" = "$source_tree"
printf 'Full executable identity proof passed for tree %s\n' "$source_tree"
