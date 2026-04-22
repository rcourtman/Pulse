package installtests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReleaseUsesV6InstallScripts(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "build-release.sh"))
	if err != nil {
		t.Fatalf("read build-release.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`RENDERED_INSTALLERS_DIR="${BUILD_DIR}/rendered-installers"`,
		`go run ./scripts/render_installers.go \`,
		`cp "${RENDERED_INSTALLERS_DIR}/install.sh" "$RELEASE_DIR/install.sh"`,
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

	if strings.Contains(script, `cp install.sh "$RELEASE_DIR/install.sh"`) {
		t.Fatal("build-release.sh still copies the legacy root install.sh into release assets")
	}

	requiredScriptWiring := []string{
		`agent_ldflags="$(./scripts/release_ldflags.sh agent --version "v${VERSION}" "${update_ldflags_args[@]}")"`,
		`server_ldflags="$(./scripts/release_ldflags.sh server --version "v${VERSION}" --build-time "${build_time}" --git-commit "${git_commit}" "${license_ldflags_args[@]}" "${update_ldflags_args[@]}")"`,
		`PULSE_UPDATE_SIGNING_KEY`,
		`go run ./scripts/release_update_key.go public-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"`,
		`go run ./scripts/release_update_key.go public-key-ssh --private-key "${PULSE_UPDATE_SIGNING_KEY}"`,
		`go run ./scripts/release_update_key.go openssh-private-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"`,
		`ssh-keygen -q -Y sign`,
		`sign_release_file "${artifact}"`,
	}
	for _, needle := range requiredScriptWiring {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing canonical ldflags wiring: %s", needle)
		}
	}
}

func TestCreateReleaseUploadsPowerShellInstaller(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}

	workflow := string(content)
	required := []string{
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
		`gh release upload "${TAG}" release/install.sh --clobber`,
		`if [ -f release/install.ps1 ]; then`,
		`gh release upload "${TAG}" release/install.ps1 --clobber`,
		`gh release upload "${TAG}" release/*.sig --clobber`,
		`gh release upload "${TAG}" release/*.sshsig --clobber`,
		`uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4`,
		`subject-path: release/*`,
		`gh api "repos/${{ github.repository }}/releases?per_page=100" --paginate`,
		`git push origin "refs/tags/${TAG}" --force`,
		`-F target_commitish="${HEAD_SHA}"`,
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
		`FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS agent_runtime`,
		`FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS runtime`,
		`COPY scripts/release_ldflags.sh ./scripts/release_ldflags.sh`,
		`COPY scripts/release_update_key.go ./scripts/release_update_key.go`,
		`COPY scripts/render_installers.go ./scripts/render_installers.go`,
		`--mount=type=secret,id=pulse_license_public_key,required=false`,
		`--mount=type=secret,id=pulse_update_signing_key,required=false`,
		`LICENSE_PUBLIC_KEY="$(tr -d '\r\n' < /run/secrets/pulse_license_public_key)"`,
		`UPDATE_PUBLIC_KEYS="$(go run ./scripts/release_update_key.go public-key --private-key "${UPDATE_SIGNING_KEY}")"`,
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}"`,
		`./scripts/release_ldflags.sh agent --version "${VERSION}"`,
		`go run ./scripts/render_installers.go --source-dir ./scripts --output-dir /app/rendered-installers`,
		`ssh-keygen -q -Y sign -f "${OPENSSH_SIGNING_KEY}" -n pulse-install`,
		`COPY --from=backend-builder /app/rendered-installers/install.sh /opt/pulse/scripts/install.sh`,
		`COPY --from=backend-builder /app/pulse-agent-* /opt/pulse/bin/`,
	}
	for _, needle := range dockerRequired {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing canonical release ldflags usage: %s", needle)
		}
	}
	if strings.Contains(dockerfile, `FROM --platform=linux/amd64 node:20-alpine AS frontend-builder`) ||
		strings.Contains(dockerfile, `FROM --platform=linux/amd64 golang:1.25.9-alpine AS backend-builder`) ||
		strings.Contains(dockerfile, `FROM alpine:3.20 AS agent_runtime`) ||
		strings.Contains(dockerfile, `FROM alpine:3.20 AS runtime`) {
		t.Fatal("Dockerfile base images must be pinned by immutable @sha256 digests")
	}

	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "deploy-demo-server.yml"))
	if err != nil {
		t.Fatalf("read deploy-demo-server workflow: %v", err)
	}
	workflow := string(workflowBytes)
	workflowRequired := []string{
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}"`,
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
		`pulse_license_public_key=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}`,
		`pulse_update_signing_key=${{ secrets.PULSE_UPDATE_SIGNING_KEY }}`,
		`DOCKER_BUILDKIT: 1`,
		`--secret id=pulse_license_public_key,env=PULSE_LICENSE_PUBLIC_KEY`,
		`--secret id=pulse_update_signing_key,env=PULSE_UPDATE_SIGNING_KEY`,
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
		`pulse_license_public_key=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}`,
		`pulse_update_signing_key=${{ secrets.PULSE_UPDATE_SIGNING_KEY }}`,
		`subject-name: docker.io/rcourtman/pulse`,
		`subject-name: ghcr.io/${{ github.repository_owner }}/pulse`,
		`subject-name: docker.io/rcourtman/pulse-agent`,
		`subject-name: ghcr.io/${{ github.repository_owner }}/pulse-agent`,
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

func repoFile(parts ...string) string {
	root := filepath.Join("..", "..")
	segments := append([]string{root}, parts...)
	return filepath.Join(segments...)
}
