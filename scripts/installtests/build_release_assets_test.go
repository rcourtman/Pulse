package installtests

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestBuildReleaseUsesV6InstallScripts(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "build-release.sh"))
	if err != nil {
		t.Fatalf("read build-release.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`,
		`PULSE_REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"`,
		`cd "${PULSE_REPO_ROOT}"`,
		`source "${SCRIPT_DIR}/release_asset_common.sh"`,
		`RENDERED_INSTALLERS_DIR="${BUILD_DIR}/rendered-installers"`,
		`go run ./scripts/render_installers.go \`,
		// The published install.sh asset is the server installer (root install.sh).
		// The rendered AGENT installer is shipped inside tarballs and Docker images
		// at ./scripts/install.sh and served at the running server's /install.sh
		// endpoint, but is intentionally not a top-level GitHub Releases asset:
		// adapter_installsh, pulse-auto-update.sh, the root install.sh's own --rc/
		// --version flows, and the README quickstart all expect releases/<tag>/install.sh
		// to be the server installer that accepts --version vX.Y.Z.
		`cp install.sh "$RELEASE_DIR/install.sh"`,
		`[ -f "${RENDERED_INSTALLERS_DIR}/install.ps1" ] && cp "${RENDERED_INSTALLERS_DIR}/install.ps1" "$RELEASE_DIR/install.ps1"`,
		`cp "$BUILD_DIR/pulse-agent-linux-amd64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-arm64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-armv7" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-armv6" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-386" "$RELEASE_DIR/"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing required release asset copy: %s", needle)
		}
	}

	// Sanity-check the opposite drift: the rendered AGENT installer must NOT be
	// the published install.sh asset. Publishing it there shipped a broken LXC
	// install + auto-update path across every v6 RC (rc.1 → rc.5).
	if strings.Contains(script, `cp "${RENDERED_INSTALLERS_DIR}/install.sh" "$RELEASE_DIR/install.sh"`) {
		t.Fatal("build-release.sh must not publish the rendered agent install.sh as the top-level release asset")
	}

	requiredScriptWiring := []string{
		`agent_ldflags="$(./scripts/release_ldflags.sh agent --version "v${VERSION}" "${update_ldflags_args[@]}")"`,
		`server_ldflags="$(./scripts/release_ldflags.sh server --version "v${VERSION}" --build-time "${build_time}" --git-commit "${git_commit}" "${license_ldflags_args[@]}" "${update_ldflags_args[@]}")"`,
		`release_go_build_args=(-buildvcs=false -trimpath)`,
		`"${release_go_build_args[@]}"`,
		`RELEASE_PACKET_SBOM="pulse-v${VERSION}-release.sbom.spdx.json"`,
		`pulse_release_prepare_signing_state "pulse-installer" "pulse-install"`,
		`trap 'pulse_release_cleanup_signing_state' EXIT`,
		`--installer-ssh-public-key "${PULSE_RELEASE_UPDATE_SSH_PUBLIC_KEY}"`,
		`pulse_release_generate_packet_sbom "${RELEASE_DIR}" "${RELEASE_PACKET_SBOM}"`,
		`mapfile -t checksum_files < <(pulse_release_collect_checksum_files "${RELEASE_DIR}")`,
		`pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"`,
	}
	for _, needle := range requiredScriptWiring {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing canonical ldflags wiring: %s", needle)
		}
	}
	if builds, cleanBuilds := strings.Count(script, `env $build_env go build \`), strings.Count(script, `"${release_go_build_args[@]}"`); builds != cleanBuilds {
		t.Fatalf("build-release.sh must disable automatic VCS stamping on every release go build: builds=%d clean_builds=%d", builds, cleanBuilds)
	}

	helperBytes, err := os.ReadFile(repoFile("scripts", "release_asset_common.sh"))
	if err != nil {
		t.Fatalf("read release_asset_common.sh: %v", err)
	}
	helper := string(helperBytes)
	helperRequired := []string{
		`: "${PULSE_SCRIPTS_DIR:=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"`,
		`: "${PULSE_REPO_ROOT:=$(cd "${PULSE_SCRIPTS_DIR}/.." && pwd)}"`,
		`go -C "${PULSE_REPO_ROOT}" run ./scripts/release_update_key.go "$@"`,
		`pulse_release_go_run_update_key public-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"`,
		`pulse_release_go_run_update_key fingerprint --public-key "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}"`,
		`pulse_release_go_run_update_key public-key-ssh --private-key "${PULSE_UPDATE_SIGNING_KEY}"`,
		`pulse_release_go_run_update_key openssh-private-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"`,
		`pulse_release_go_run_update_key sign --private-key "${PULSE_UPDATE_SIGNING_KEY}" --file "${absolute_file}"`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY_FINGERPRINT`,
		`Verified update signing public key fingerprint: ${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT}`,
		`ssh-keygen -q -Y sign`,
		`"${resolved_tool}" "dir:${release_dir}" -o "spdx-json=${tmp_sbom}"`,
		`if compgen -G "pulse-*.sbom.spdx.json" > /dev/null; then`,
		`find . -maxdepth 1 -type f \( -name '*.sig' -o -name '*.sshsig' \) -delete`,
	}
	for _, needle := range helperRequired {
		if !strings.Contains(helper, needle) {
			t.Fatalf("release_asset_common.sh missing canonical release asset wiring: %s", needle)
		}
	}
}

func TestCreateReleaseUploadsPowerShellInstaller(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	validationContent, err := os.ReadFile(repoFile(".github", "workflows", "validate-release-assets.yml"))
	if err != nil {
		t.Fatalf("read validate-release-assets.yml: %v", err)
	}

	workflow := string(content)
	validationWorkflow := string(validationContent)
	required := []string{
		`historical_asset_backfill_only:`,
		`description: 'Repair an already-published release packet in place without rebuilding binaries'`,
		`SYFT_VERSION="1.42.4"`,
		`SYFT_ARCHIVE="syft_${SYFT_VERSION}_linux_amd64.tar.gz"`,
		`SYFT_SHA256="590650c2743b83f327d1bf9bec64f6f83b7fec504187bb84f500c862bf8f2a0f"`,
		`install -m 0755 "${TMP_DIR}/syft" /usr/local/bin/syft`,
		`release_upload_with_retry "${TAG}" release/*.sbom.spdx.json --clobber`,
		`release/pulse-agent-linux-amd64`,
		`release/pulse-agent-linux-arm64`,
		`release/pulse-agent-linux-armv7`,
		`release/pulse-agent-linux-armv6`,
		`release/pulse-agent-linux-386`,
		`release/pulse-agent-freebsd-amd64`,
		`release/pulse-agent-freebsd-arm64`,
		`release/pulse-agent-windows-amd64.exe`,
		`release/pulse-agent-windows-arm64.exe`,
		`release/pulse-agent-windows-386.exe`,
		`release_upload_with_retry "${TAG}" release/install.sh --clobber`,
		`if [ -f release/install.ps1 ]; then`,
		`release_upload_with_retry "${TAG}" release/install.ps1 --clobber`,
		`release_upload_with_retry "${TAG}" release/*.sig --clobber`,
		`release_upload_with_retry "${TAG}" release/*.sshsig --clobber`,
		`gh release upload "$@"`,
		`gh release upload failed on attempt ${attempt}/${max_attempts}; retrying in ${wait_seconds}s`,
		`gh release upload failed after ${max_attempts} attempts`,
		`uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4`,
		`subject-path: release/*`,
		`gh api "repos/${{ github.repository }}/releases?per_page=100" --paginate`,
		`git push origin "refs/tags/${TAG}" --force`,
		`-F target_commitish="${HEAD_SHA}"`,
		`historical_asset_backfill_only=${HISTORICAL_ASSET_BACKFILL_ONLY}`,
		`if: ${{ always() && needs.prepare.result == 'success' && needs.create_release.result == 'success' && needs.prepare.outputs.historical_asset_backfill_only != 'true' }}`,
		`if: ${{ needs.prepare.outputs.historical_asset_backfill_only == 'true' }}`,
		`permissions:`,
		`issues: write`,
		`statuses: write`,
		`ACTUAL_RELEASE_TAG=$(echo "$RELEASE_JSON" | jq -r '.tag_name // empty')`,
		`ACTUAL_TARGET_COMMITISH=$(echo "$RELEASE_JSON" | jq -r '.target_commitish // empty')`,
		`Draft release ${RELEASE_ID} is bound to tag ${ACTUAL_RELEASE_TAG}, expected ${TAG}.`,
		`Draft release ${RELEASE_ID} target_commitish is ${ACTUAL_TARGET_COMMITISH}, expected ${HEAD_SHA}.`,
		`./scripts/backfill-release-assets.sh --tag "${{ needs.prepare.outputs.tag }}" --repo "${{ github.repository }}"`,
		`./scripts/validate-published-release.sh "${{ needs.prepare.outputs.tag }}" "${{ github.repository }}"`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("create-release.yml missing required installer upload step: %s", needle)
		}
	}

	if !strings.Contains(workflow, `draft: ${{ github.event.inputs.draft_only == 'true' }}`) {
		t.Fatal("create-release.yml must pass the actual draft_only state into validate-release-assets")
	}
	if strings.Contains(workflow, `provenance: false`) {
		t.Fatal("create-release.yml must not disable release-image provenance")
	}

	validationRequired := []string{
		`statuses: write`,
		`curl --fail-with-body --silent --show-error -X POST`,
		`"context": "Release Asset Validation"`,
		`--arg tag "${{ steps.context.outputs.tag }}"`,
		`--arg target_commitish "${{ steps.context.outputs.target_commitish }}"`,
		`{body: $body, tag_name: $tag, target_commitish: $target_commitish}`,
		`{draft: true, tag_name: $tag, target_commitish: $target_commitish}`,
		`Validation release body update detached release tag`,
		`Validation release body update changed target_commitish`,
	}
	for _, needle := range validationRequired {
		if !strings.Contains(validationWorkflow, needle) {
			t.Fatalf("validate-release-assets.yml missing required status publication contract: %s", needle)
		}
	}
}

func TestBackfillReleaseWorkflowRepairsPublishedAssetsWithoutRebuilds(t *testing.T) {
	scriptBytes, err := os.ReadFile(repoFile("scripts", "backfill-release-assets.sh"))
	if err != nil {
		t.Fatalf("read backfill-release-assets.sh: %v", err)
	}
	script := string(scriptBytes)
	scriptRequired := []string{
		`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`,
		`PULSE_REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"`,
		`cd "${PULSE_REPO_ROOT}"`,
		`source "${SCRIPT_DIR}/release_asset_common.sh"`,
		`gh release view "${TAG}" -R "${REPO}" --json isDraft,tagName`,
		`Error: ${TAG} is still a draft release; use the normal release pipeline instead of historical backfill.`,
		`gh release download "${TAG}" -R "${REPO}" --dir "${RELEASE_DIR}" --clobber`,
		`pulse_release_prepare_signing_state "pulse-installer" "pulse-install"`,
		`pulse_release_generate_packet_sbom "${PAYLOAD_DIR}" "${RELEASE_PACKET_SBOM}"`,
		`pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"`,
		`gh release upload "${TAG}" "${RELEASE_DIR}/checksums.txt" --clobber`,
		`gh release upload "${TAG}" "${RELEASE_DIR}"/*.sha256 --clobber`,
		`gh release upload "${TAG}" "${RELEASE_DIR}"/*.sig --clobber`,
		`gh release upload "${TAG}" "${RELEASE_DIR}"/*.sshsig --clobber`,
		`gh release upload "${TAG}" "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}" --clobber`,
	}
	for _, needle := range scriptRequired {
		if !strings.Contains(script, needle) {
			t.Fatalf("backfill-release-assets.sh missing required historical backfill step: %s", needle)
		}
	}

	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "backfill-release-assets.yml"))
	if err != nil {
		t.Fatalf("read backfill-release-assets.yml: %v", err)
	}
	workflow := string(workflowBytes)
	workflowRequired := []string{
		`name: Backfill Release Assets`,
		`workflow_dispatch:`,
		`contents: write`,
		`runs-on: ubuntu-24.04`,
		`uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4`,
		`uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5`,
		`SYFT_VERSION="1.42.4"`,
		`SYFT_ARCHIVE="syft_${SYFT_VERSION}_linux_amd64.tar.gz"`,
		`SYFT_SHA256="590650c2743b83f327d1bf9bec64f6f83b7fec504187bb84f500c862bf8f2a0f"`,
		`./scripts/backfill-release-assets.sh --tag "${{ inputs.tag }}" --repo "${{ github.repository }}"`,
		`PULSE_UPDATE_SIGNING_KEY: ${{ secrets.PULSE_UPDATE_SIGNING_KEY }}`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}`,
		`./scripts/validate-published-release.sh "${{ inputs.tag }}" "${{ github.repository }}"`,
	}
	for _, needle := range workflowRequired {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("backfill-release-assets.yml missing required release-repair step: %s", needle)
		}
	}
}

func TestReleaseValidationRequiresSignedSidecars(t *testing.T) {
	localValidatorBytes, err := os.ReadFile(repoFile("scripts", "validate-release.sh"))
	if err != nil {
		t.Fatalf("read validate-release.sh: %v", err)
	}
	localValidator := string(localValidatorBytes)
	localRequired := []string{
		`"pulse-v${PULSE_VERSION}-release.sbom.spdx.json"`,
		`release_sbom="pulse-${PULSE_TAG}-release.sbom.spdx.json"`,
		`error "checksums.txt is missing ${release_sbom}"`,
		`success "Release SBOM is listed in checksums.txt"`,
		`info "Validating SSH signature sidecars..."`,
		`if [ ! -s "checksums.txt.sshsig" ]; then`,
		`error "Missing or empty checksums.txt.sshsig"`,
		`if [ ! -s "${filename}.sshsig" ]; then`,
		`error "Missing or empty ${filename}.sshsig"`,
		`success "SSH signature sidecars validated"`,
		`validate_download_binary_headers() {`,
		`http_header_value "X-Checksum-Sha256"`,
		`http_header_value "X-Signature-Ed25519"`,
		`http_header_value "X-Signature-SSHSIG"`,
		`Install script endpoints returned required signature headers`,
		`Download endpoints returned binaries with checksum and signature headers for all platforms/architectures`,
		`Offline self-heal: download endpoint works with checksum and signature headers without outbound network`,
		// Server installer identity guard — see the rc.1 → rc.5 regression where
		// the rendered agent installer shipped as the top-level install.sh asset
		// for 30 days before anyone noticed. Removing any of these unpins the asset.
		`Validating install.sh is the Pulse server installer`,
		`grep -qE '^# Pulse Installer Script'`,
		`grep -qE '^[[:space:]]*--version\)'`,
		`Pulse Unified Agent Installer`,
		`bash "$install_sh_path" --help`,
		`Install specific version (e.g.`,
		// README key drift guard — across v6 rc.2 → rc.5 the README pinned a
		// stale ed25519 key that did not verify install.sh.sshsig, so anyone
		// following the secure-install path saw "Could not verify signature".
		// validate-release.sh must extract the README's pinned key and actually
		// run ssh-keygen -Y verify against the signed installer.
		`Validating README pinned signature key matches install.sh.sshsig`,
		`grep -oE "ssh-ed25519 [A-Za-z0-9+/=]+ pulse-installer" "$readme_path"`,
		`ssh-keygen -Y verify \`,
		`README's pinned signature key does not verify install.sh.sshsig`,
	}

	readmeBytes, err := os.ReadFile(repoFile("README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(readmeBytes)
	// Lock in the actual signing key documented to customers. This is the public
	// counterpart of PULSE_UPDATE_SIGNING_KEY and matches what install.sh and
	// scripts/pulse-auto-update.sh have embedded. A future edit cannot silently
	// regress to the stale Ds21c5 key without tripping this assertion.
	const correctReadmeKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
	if !strings.Contains(readme, correctReadmeKey) {
		t.Fatalf("README.md must pin the correct pulse-installer ed25519 key for install.sh signature verification")
	}
	const staleReadmeKey = "Ds21c5oPk2khrdHlsw1aZ9EJKoTsyalGzhb0hdwJrkV"
	if strings.Contains(readme, staleReadmeKey) {
		t.Fatalf("README.md still references the stale pulse-installer key Ds21c5...; rc.2 → rc.5 shipped this drift")
	}

	installDocsBytes, err := os.ReadFile(repoFile("docs", "INSTALL.md"))
	if err != nil {
		t.Fatalf("read docs/INSTALL.md: %v", err)
	}
	installDocs := string(installDocsBytes)
	if !strings.Contains(installDocs, correctReadmeKey) {
		t.Fatalf("docs/INSTALL.md must pin the correct pulse-installer ed25519 key")
	}
	if strings.Contains(installDocs, staleReadmeKey) {
		t.Fatalf("docs/INSTALL.md still references the stale pulse-installer key Ds21c5...")
	}
	for _, needle := range localRequired {
		if !strings.Contains(localValidator, needle) {
			t.Fatalf("validate-release.sh missing signed sidecar validation: %s", needle)
		}
	}

	publishedValidatorBytes, err := os.ReadFile(repoFile("scripts", "validate-published-release.sh"))
	if err != nil {
		t.Fatalf("read validate-published-release.sh: %v", err)
	}
	publishedValidator := string(publishedValidatorBytes)
	publishedRequired := []string{
		`RELEASE_SBOM="pulse-${TAG}-release.sbom.spdx.json"`,
		`echo "Failed to download ${RELEASE_SBOM} for ${TAG}" >&2`,
		`echo "${RELEASE_SBOM} is empty for ${TAG}" >&2`,
		`CHECKSUMS_SIG_PATH="${TMP_DIR}/checksums.txt.sshsig"`,
		`"${BASE_URL}/checksums.txt.sshsig"`,
		`echo "Failed to download checksums.txt.sshsig for ${TAG}" >&2`,
		`sshsig_path="${TMP_DIR}/${filename}.sshsig"`,
		`"${artifact_url}.sshsig"`,
		`echo "Failed to download ${filename}.sshsig" >&2`,
		`Published release assets for ${TAG} match checksums.txt, *.sha256 files, and required *.sshsig sidecars.`,
	}
	for _, needle := range publishedRequired {
		if !strings.Contains(publishedValidator, needle) {
			t.Fatalf("validate-published-release.sh missing signed sidecar validation: %s", needle)
		}
	}

	contractBytes, err := os.ReadFile(repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"))
	if err != nil {
		t.Fatalf("read deployment-installability contract: %v", err)
	}
	contract := string(contractBytes)
	contractRequired := []string{
		"`scripts/validate-release.sh`",
		"`scripts/validate-published-release.sh`",
		"`scripts/backfill-release-assets.sh`",
		"`.github/workflows/backfill-release-assets.yml`",
		"`scripts/validate-release.sh`, and",
		"`scripts/release_asset_common.sh`",
		"must derive the embedded update trust root",
		"standalone SPDX JSON SBOM",
		"already-published packet",
		"derived integrity assets",
		"and fail validation if",
		"published artifact or",
		"`checksums.txt` is missing its `.sshsig` sidecar",
		"release-packet SBOM is absent",
		"download endpoints must return checksum and signature headers",
		"must disable Go's automatic VCS stamping",
		"`-buildvcs=false`",
	}
	for _, needle := range contractRequired {
		if !strings.Contains(contract, needle) {
			t.Fatalf("deployment-installability contract missing signed sidecar validation requirement: %s", needle)
		}
	}
}

func TestDockerAndDemoBuildsUseCanonicalReleaseLdflags(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(dockerfileBytes)
	dockerRequired := []string{
		`FROM --platform=linux/amd64 node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS frontend-builder`,
		`FROM --platform=linux/amd64 golang:1.25.9-alpine@sha256:5caaf1cca9dc351e13deafbc3879fd4754801acba8653fa9540cea125d01a71f AS backend-builder`,
		`FROM backend-builder AS release-assets-builder`,
		`FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS agent_runtime`,
		`FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS pulse-runtime-base`,
		`FROM pulse-runtime-base AS hosted_runtime`,
		`FROM pulse-runtime-base AS runtime`,
		`COPY scripts/release_ldflags.sh ./scripts/release_ldflags.sh`,
		`COPY scripts/release_update_key.go ./scripts/release_update_key.go`,
		`COPY scripts/render_installers.go ./scripts/render_installers.go`,
		`ARG PULSE_LICENSE_PUBLIC_KEY_SHA256`,
		`--mount=type=secret,id=pulse_license_public_key,required=false`,
		`--mount=type=secret,id=pulse_update_signing_key,required=false`,
		`ARG PULSE_UPDATE_SIGNING_PUBLIC_KEY`,
		`LICENSE_PUBLIC_KEY="$(tr -d '\r\n' < /run/secrets/pulse_license_public_key)"`,
		`EXPECTED_LICENSE_PUBLIC_KEY_SHA256="${PULSE_LICENSE_PUBLIC_KEY_SHA256#SHA256:}"`,
		`mounted license public key does not match PULSE_LICENSE_PUBLIC_KEY_SHA256.`,
		`UPDATE_PUBLIC_KEYS="$(go run ./scripts/release_update_key.go public-key --private-key "${UPDATE_SIGNING_KEY}")"`,
		`mounted update signing key does not match PULSE_UPDATE_SIGNING_PUBLIC_KEY.`,
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}"`,
		`./scripts/release_ldflags.sh agent --version "${VERSION}"`,
		`-buildvcs=false`,
		`go run ./scripts/render_installers.go --source-dir ./scripts --output-dir /app/rendered-installers`,
		`--allow-empty-installer-ssh-public-key`,
		`ssh-keygen -q -Y sign -f "${OPENSSH_SIGNING_KEY}" -n pulse-install`,
		`COPY --from=release-assets-builder /app/rendered-installers/install.sh /opt/pulse/scripts/install.sh`,
		`COPY --from=release-assets-builder /app/pulse-agent-* /opt/pulse/bin/`,
	}
	for _, needle := range dockerRequired {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing canonical release ldflags usage: %s", needle)
		}
	}
	hostedStart := strings.Index(dockerfile, `FROM pulse-runtime-base AS hosted_runtime`)
	runtimeStart := strings.Index(dockerfile, `FROM pulse-runtime-base AS runtime`)
	if hostedStart == -1 || runtimeStart == -1 || hostedStart > runtimeStart {
		t.Fatal("Dockerfile must define hosted_runtime from pulse-runtime-base before the full runtime stage")
	}
	hostedStage := dockerfile[hostedStart:runtimeStart]
	if strings.Contains(hostedStage, "rendered-installers") || strings.Contains(hostedStage, "/opt/pulse/bin") {
		t.Fatalf("hosted_runtime target must not depend on installer rendering or embedded agent artifacts:\n%s", hostedStage)
	}
	if strings.Contains(dockerfile, `FROM --platform=linux/amd64 node:20-alpine AS frontend-builder`) ||
		strings.Contains(dockerfile, `FROM --platform=linux/amd64 golang:1.25.9-alpine AS backend-builder`) ||
		strings.Contains(dockerfile, `FROM alpine:3.20 AS agent_runtime`) ||
		strings.Contains(dockerfile, `FROM alpine:3.20 AS pulse-runtime-base`) {
		t.Fatal("Dockerfile base images must be pinned by immutable @sha256 digests")
	}
	if builds, cleanBuilds := strings.Count(dockerfile, " go build \\"), strings.Count(dockerfile, "-buildvcs=false"); builds != cleanBuilds {
		t.Fatalf("Dockerfile release go builds must all disable automatic VCS stamping: builds=%d clean_builds=%d", builds, cleanBuilds)
	}

	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "deploy-demo-server.yml"))
	if err != nil {
		t.Fatalf("read deploy-demo-server workflow: %v", err)
	}
	workflow := string(workflowBytes)
	workflowRequired := []string{
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}"`,
		`-buildvcs=false`,
		`demo-preview-v6`,
		`demo-stable`,
		`workflow_dispatch:`,
		`target:`,
	}
	for _, needle := range workflowRequired {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("deploy-demo-server workflow missing canonical release ldflags usage: %s", needle)
		}
	}
}

func TestReleaseWorkflowsUseSecretSafeAttestedImageBuilds(t *testing.T) {
	createReleaseBytes, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	createRelease := string(createReleaseBytes)
	createReleaseRequired := []string{
		`provenance: mode=max`,
		`sbom: true`,
		`secrets: |`,
		`id: license_key_cache`,
		`PULSE_LICENSE_PUBLIC_KEY_SHA256=${{ steps.license_key_cache.outputs.sha256 }}`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY=${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}`,
		`pulse_license_public_key=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}`,
		`pulse_update_signing_key=${{ secrets.PULSE_UPDATE_SIGNING_KEY }}`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}`,
		`Validate installer signing key pins`,
		`go run ./scripts/release_update_key.go public-key-ssh`,
		`install.sh scripts/pulse-auto-update.sh release/pulse-auto-update.sh`,
		`does not trust the configured release signing key.`,
		`DOCKER_BUILDKIT: 1`,
		`--secret id=pulse_license_public_key,env=PULSE_LICENSE_PUBLIC_KEY`,
		`--secret id=pulse_update_signing_key,env=PULSE_UPDATE_SIGNING_KEY`,
		`--build-arg PULSE_LICENSE_PUBLIC_KEY_SHA256="${PULSE_LICENSE_PUBLIC_KEY_SHA256}"`,
		`--build-arg PULSE_UPDATE_SIGNING_PUBLIC_KEY="${PULSE_UPDATE_SIGNING_PUBLIC_KEY}"`,
		`id-token: write`,
		`attestations: write`,
		`uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4`,
	}
	for _, needle := range createReleaseRequired {
		if !strings.Contains(createRelease, needle) {
			t.Fatalf("create-release.yml missing attested secret-safe release build contract: %s", needle)
		}
	}
	if strings.Contains(createRelease, `PULSE_LICENSE_PUBLIC_KEY=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}`) {
		t.Fatal("create-release.yml must not pass the license public key through docker build args")
	}

	publishBytes, err := os.ReadFile(repoFile(".github", "workflows", "publish-docker.yml"))
	if err != nil {
		t.Fatalf("read publish-docker.yml: %v", err)
	}
	publish := string(publishBytes)
	publishRequired := []string{
		`provenance: mode=max`,
		`sbom: true`,
		`secrets: |`,
		`id: license_key_cache`,
		`PULSE_LICENSE_PUBLIC_KEY_SHA256=${{ steps.license_key_cache.outputs.sha256 }}`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY=${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}`,
		`pulse_license_public_key=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}`,
		`pulse_update_signing_key=${{ secrets.PULSE_UPDATE_SIGNING_KEY }}`,
		`subject-name: docker.io/rcourtman/pulse`,
		`subject-name: ghcr.io/${{ github.repository_owner }}/pulse`,
		// pulse-agent ships as release-asset binaries, not as a Docker
		// image (see commit dropping the agent image publish steps).
		// The agent attestation subject-names intentionally do not
		// appear in publish-docker.yml.
		`push-to-registry: true`,
		`create-storage-record: false`,
		`id-token: write`,
		`attestations: write`,
	}
	for _, needle := range publishRequired {
		if !strings.Contains(publish, needle) {
			t.Fatalf("publish-docker.yml missing attested secret-safe publish contract: %s", needle)
		}
	}
	if strings.Contains(publish, `PULSE_LICENSE_PUBLIC_KEY=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}`) {
		t.Fatal("publish-docker.yml must not pass the license public key through docker build args")
	}
}

func TestDeploymentDefaultsPinVersionedImagesAndHelmDocsChecksum(t *testing.T) {
	versionBytes, err := os.ReadFile(repoFile("VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if version == "" {
		t.Fatal("VERSION is empty")
	}

	composeBytes, err := os.ReadFile(repoFile("docker-compose.yml"))
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	compose := string(composeBytes)
	if !strings.Contains(compose, "image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}") {
		t.Fatalf("docker-compose.yml must pin the governed release version:\n%s", compose)
	}
	if strings.Contains(compose, ":latest") {
		t.Fatalf("docker-compose.yml must not default to a floating latest tag:\n%s", compose)
	}

	installDockerBytes, err := os.ReadFile(repoFile("scripts", "install-docker.sh"))
	if err != nil {
		t.Fatalf("read install-docker.sh: %v", err)
	}
	installDocker := string(installDockerBytes)
	if !strings.Contains(installDocker, `CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`) {
		t.Fatalf("install-docker.sh must pin the governed release version:\n%s", installDocker)
	}
	if strings.Contains(installDocker, ":latest") {
		t.Fatalf("install-docker.sh must not default to a floating latest tag:\n%s", installDocker)
	}

	helmPagesBytes, err := os.ReadFile(repoFile(".github", "workflows", "helm-pages.yml"))
	if err != nil {
		t.Fatalf("read helm-pages.yml: %v", err)
	}
	helmPages := string(helmPagesBytes)
	required := []string{
		`HELM_DOCS_VERSION="1.14.2"`,
		`HELM_DOCS_ARCHIVE="helm-docs_${HELM_DOCS_VERSION}_Linux_x86_64.tar.gz"`,
		`HELM_DOCS_SHA256="a8cf72ada34fad93285ba2a452b38bdc5bd52cc9a571236244ec31022928d6cc"`,
		`sha256sum --check --`,
	}
	for _, needle := range required {
		if !strings.Contains(helmPages, needle) {
			t.Fatalf("helm-pages.yml missing checksum-verified helm-docs install step: %s", needle)
		}
	}
}

func TestDeployDemoWorkflowFailsClosedForPreviewAndVerifiesFrontendParity(t *testing.T) {
	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "deploy-demo-server.yml"))
	if err != nil {
		t.Fatalf("read deploy-demo-server workflow: %v", err)
	}

	workflow := string(workflowBytes)
	required := []string{
		`DEMO_EXPECTED_HOSTNAME: ${{ vars.DEMO_EXPECTED_HOSTNAME }}`,
		`DEMO_LOCAL_BASE_URL: ${{ vars.DEMO_LOCAL_BASE_URL }}`,
		`[ -n "$DEMO_EXPECTED_HOSTNAME" ] || { echo "::error::DEMO_EXPECTED_HOSTNAME is required in the selected demo environment."; exit 1; }`,
		`[ -n "$DEMO_LOCAL_BASE_URL" ] || { echo "::error::DEMO_LOCAL_BASE_URL is required in the selected demo environment."; exit 1; }`,
		`Capture expected frontend entry asset`,
		`Verify target host identity`,
		`SERVICE_NAME="pulse-v6-preview"`,
		`Preview demo deployments must not target the stable pulse service.`,
		`Demo environment points at host $REMOTE_HOSTNAME but expected $DEMO_EXPECTED_HOSTNAME.`,
		`Verify frontend parity`,
		`Verify public browser smoke`,
		`./scripts/run_demo_public_browser_smoke.sh`,
		`extract_entry_asset()`,
		`<script\b[^>]*\bsrc=\"(/assets/index-[^\"]*\.js)\"`,
		`Remote service is serving $REMOTE_ASSET but the build expected $EXPECTED_ASSET.`,
		`Public demo is serving $PUBLIC_ASSET but the build expected $EXPECTED_ASSET.`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("deploy-demo-server workflow missing preview isolation or frontend parity proof: %s", needle)
		}
	}
}

func TestUpdateDemoWorkflowUsesGovernedNetworkPath(t *testing.T) {
	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "update-demo-server.yml"))
	if err != nil {
		t.Fatalf("read update-demo-server workflow: %v", err)
	}

	workflow := string(workflowBytes)
	required := []string{
		`- name: Tailscale`,
		`uses: tailscale/github-action@4e4c49acaa9818630ce0bd7a564372c17e33fb4d # v2`,
		`authkey: ${{ secrets.TS_AUTHKEY }}`,
		`uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5`,
		`go run ./scripts/release_update_key.go public-key-ssh`,
		`sed -i "s|^PINNED_RELEASE_SSH_PUBLIC_KEY=.*|PINNED_RELEASE_SSH_PUBLIC_KEY=\"${TRUSTED_SSH_PUBLIC_KEY}\"|" /tmp/pulse-install.sh`,
		`Verify target host identity`,
		`Demo environment points at host $REMOTE_HOSTNAME but expected $DEMO_EXPECTED_HOSTNAME.`,
		`Verify public browser smoke`,
		`./scripts/run_demo_public_browser_smoke.sh`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("update-demo-server workflow missing governed network path: %s", needle)
		}
	}
}

func TestDemoPublicBrowserSmokeWaitsForVisibleLoginUI(t *testing.T) {
	scriptBytes, err := os.ReadFile(repoFile("scripts", "demo_public_browser_smoke.cjs"))
	if err != nil {
		t.Fatalf("read demo public browser smoke script: %v", err)
	}

	script := string(scriptBytes)
	required := []string{
		`waitUntil: 'domcontentloaded'`,
		`getByLabel('Username').waitFor({ state: 'visible', timeout: 120000 })`,
		`getByLabel('Password').waitFor({ state: 'visible', timeout: 120000 })`,
		`getByRole('button', { name: 'Sign in to Pulse' }).waitFor({ state: 'visible', timeout: 120000 })`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("demo public browser smoke missing visible-login readiness proof: %s", needle)
		}
	}

	if strings.Contains(script, `waitUntil: 'networkidle'`) {
		t.Fatal("demo public browser smoke still depends on networkidle instead of visible login readiness")
	}
}

func TestDockerfileStagesShippedDocsForEmbeddedFrontendBuild(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	dockerfile := string(dockerfileBytes)
	required := []string{
		`COPY docs/ /app/docs/`,
		`COPY SECURITY.md TERMS.md /app/`,
	}
	for _, needle := range required {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing shipped-doc build input: %s", needle)
		}
	}

	dockerignoreBytes, err := os.ReadFile(repoFile(".dockerignore"))
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}

	dockerignore := string(dockerignoreBytes)
	requiredAllowlist := []string{
		`!docs/`,
		`!docs/**`,
		`!SECURITY.md`,
		`!TERMS.md`,
	}
	for _, needle := range requiredAllowlist {
		if !strings.Contains(dockerignore, needle) {
			t.Fatalf(".dockerignore missing shipped-doc allowlist entry: %s", needle)
		}
	}
}

func TestReleaseUpdateKeyFingerprintUsesCanonicalRawPublicKeyHash(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}

	cmd := exec.Command("go", "run", "./scripts/release_update_key.go", "fingerprint", "--private-key", base64.StdEncoding.EncodeToString(privateKey))
	cmd.Dir = repoFile()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release_update_key.go fingerprint failed: %v\n%s", err, output)
	}

	sum := sha256.Sum256(publicKey)
	expected := "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
	if got := strings.TrimSpace(string(output)); got != expected {
		t.Fatalf("fingerprint mismatch: got %q want %q", got, expected)
	}
}

func TestReleaseUpdateKeyPublicKeySSHAcceptsPublicKey(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}

	cmd := exec.Command("go", "run", "./scripts/release_update_key.go", "public-key-ssh", "--public-key", base64.StdEncoding.EncodeToString(publicKey), "--comment", "pulse-installer")
	cmd.Dir = repoFile()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release_update_key.go public-key-ssh failed: %v\n%s", err, output)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("derive SSH public key: %v", err)
	}
	expected := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublicKey))) + " pulse-installer"
	if got := strings.TrimSpace(string(output)); got != expected {
		t.Fatalf("SSH public key mismatch: got %q want %q", got, expected)
	}
}

func TestReleaseAssetCommonRunsUpdateKeyThroughModulePath(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}

	cmd := exec.Command("bash", "-lc", "source ./scripts/release_asset_common.sh; pulse_release_go_run_update_key")
	cmd.Dir = repoFile()
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected release_update_key.go usage failure, got success:\n%s", output)
	}
	text := string(output)
	if !strings.Contains(text, "release_update_key.go public-key") {
		t.Fatalf("expected release_update_key.go usage output, got:\n%s", output)
	}
	if strings.Contains(text, "use of internal package") {
		t.Fatalf("release helper invoked update key outside module import boundary:\n%s", output)
	}
}

func TestReleaseAssetCommonRejectsUnexpectedUpdateSigningPublicKey(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	unexpectedPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate unexpected public key: %v", err)
	}

	cmd := exec.Command("bash", "-lc", "source ./scripts/release_asset_common.sh; pulse_release_prepare_signing_state pulse-installer pulse-install")
	cmd.Dir = repoFile()
	cmd.Env = append(os.Environ(),
		"PULSE_UPDATE_SIGNING_KEY="+base64.StdEncoding.EncodeToString(privateKey),
		"PULSE_UPDATE_SIGNING_PUBLIC_KEY="+base64.StdEncoding.EncodeToString(unexpectedPublicKey),
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected release_asset_common.sh to reject a mismatched signing public key:\n%s", output)
	}
	if !strings.Contains(string(output), "does not match PULSE_UPDATE_SIGNING_PUBLIC_KEY") {
		t.Fatalf("expected mismatched signing public key error, got:\n%s", output)
	}
}

// TestBuildReleasePackagesPulseMcpForAllPlatforms pins the
// distribution path for pulse-mcp: each Pulse release must build
// the MCP adapter for the same multi-OS matrix as the unified
// agent and emit per-platform tarballs/zips, bare binaries (for
// /releases/latest/download/ redirect compatibility), and the
// install-mcp.sh script into RELEASE_DIR. Drift in any of those
// strings means an integrator following the published install
// path hits a 404 on the release endpoint instead of a working
// binary.
func TestBuildReleasePackagesPulseMcpForAllPlatforms(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "build-release.sh"))
	if err != nil {
		t.Fatalf("read build-release.sh: %v", err)
	}
	script := string(content)

	required := []string{
		// Build loop wires through ./cmd/pulse-mcp.
		`-o "$output_path" \
        ./cmd/pulse-mcp`,
		// Per-platform packaging follows the pulse-agent shape
		// exactly so the upload step's glob does not need
		// special cases.
		`tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-linux-amd64.tar.gz" -C "$BUILD_DIR" pulse-mcp-linux-amd64`,
		`tar -czf "$RELEASE_DIR/pulse-mcp-v${VERSION}-darwin-arm64.tar.gz" -C "$BUILD_DIR" pulse-mcp-darwin-arm64`,
		`zip -j "$RELEASE_DIR/pulse-mcp-v${VERSION}-windows-amd64.zip" "$BUILD_DIR/pulse-mcp-windows-amd64.exe"`,
		// Bare-binary copies for the /releases/latest/download/
		// redirect that install-mcp.sh fetches by default.
		`cp "$BUILD_DIR/pulse-mcp-linux-amd64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-mcp-darwin-amd64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-mcp-darwin-arm64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-mcp-windows-amd64.exe" "$RELEASE_DIR/"`,
		// The installer scripts themselves must reach
		// RELEASE_DIR so the GitHub Releases asset upload can
		// publish them as the canonical curl-pipe-bash entry
		// point.
		`cp scripts/install-mcp.sh "$RELEASE_DIR/install-mcp.sh"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing pulse-mcp distribution wiring: %s", needle)
		}
	}

	// install-mcp.sh and install-mcp.ps1 must both exist as
	// shipped scripts; the build pipeline references them, so
	// missing-file drift breaks release builds rather than
	// quietly ships an installer that 404s.
	if _, err := os.Stat(repoFile("scripts", "install-mcp.sh")); err != nil {
		t.Fatalf("scripts/install-mcp.sh missing: %v", err)
	}
	if _, err := os.Stat(repoFile("scripts", "install-mcp.ps1")); err != nil {
		t.Fatalf("scripts/install-mcp.ps1 missing: %v", err)
	}

	// install-mcp.sh's install-dir resolution and SHA256
	// verification are load-bearing: dropping either silently
	// turns the installer into "curl | bash with no integrity
	// check," which is the failure mode the hook is here to
	// prevent. Pin the touchstones.
	mcpScript, err := os.ReadFile(repoFile("scripts", "install-mcp.sh"))
	if err != nil {
		t.Fatalf("read install-mcp.sh: %v", err)
	}
	for _, needle := range []string{
		`detect_platform()`,
		`choose_install_dir()`,
		`PULSE_MCP_NO_VERIFY`,
		`checksums.txt`,
		`sha256 mismatch`,
	} {
		if !strings.Contains(string(mcpScript), needle) {
			t.Fatalf("install-mcp.sh missing required helper or guard: %s", needle)
		}
	}
}

func repoFile(parts ...string) string {
	root := filepath.Join("..", "..")
	segments := append([]string{root}, parts...)
	return filepath.Join(segments...)
}
