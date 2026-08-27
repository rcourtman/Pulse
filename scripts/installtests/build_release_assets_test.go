package installtests

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	compileContent, err := os.ReadFile(repoFile("scripts", "build-release-binaries.sh"))
	if err != nil {
		t.Fatalf("read build-release-binaries.sh: %v", err)
	}
	compileScript := string(compileContent)
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
		// pulse-auto-update.sh, the root install.sh's own --rc/--version flows, and
		// the README quickstart all expect releases/<tag>/install.sh
		// to be the server installer that accepts --version vX.Y.Z.
		`cp install.sh "$RELEASE_DIR/install.sh"`,
		`[ -f "${RENDERED_INSTALLERS_DIR}/install.ps1" ] && cp "${RENDERED_INSTALLERS_DIR}/install.ps1" "$RELEASE_DIR/install.ps1"`,
		`cp "$BUILD_DIR/pulse-agent-linux-amd64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-arm64" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-armv7" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-armv6" "$RELEASE_DIR/"`,
		`cp "$BUILD_DIR/pulse-agent-linux-386" "$RELEASE_DIR/"`,
		`provider_msp_bundle_root="pulse-provider-msp-v${VERSION}"`,
		`cp -a deploy/provider-msp/. "${provider_msp_bundle_dir}/"`,
		`CONTROL_PLANE_IMAGE=ghcr.io/rcourtman/pulse-control-plane:v${VERSION}`,
		`CP_PULSE_IMAGE=ghcr.io/rcourtman/pulse:v${VERSION}`,
		`tar -czf "${provider_msp_bundle_asset}" -C "${BUILD_DIR}" "${provider_msp_bundle_root}"`,
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
	if builds, cleanBuilds := strings.Count(script, "go build \\\n"), strings.Count(script, `"${release_go_build_args[@]}"`); builds != cleanBuilds {
		t.Fatalf("build-release.sh must disable automatic VCS stamping on every release go build: builds=%d clean_builds=%d", builds, cleanBuilds)
	}
	for _, needle := range []string{
		`release_go_build_args=(-buildvcs=false -trimpath)`,
		`command=(go build "${release_go_build_args[@]}")`,
		`package=./cmd/pulse-control-plane`,
		`task_components+=(control-plane)`,
	} {
		if !strings.Contains(compileScript, needle) {
			t.Fatalf("build-release-binaries.sh missing clean compilation contract: %s", needle)
		}
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
		`pulse_release_stage_server_archive()`,
		`for target in "${PULSE_RELEASE_AGENT_TARGETS[@]}"; do`,
		`install -m 0755 "${server_binary}" "${staging_dir}/bin/pulse"`,
	}
	for _, needle := range helperRequired {
		if !strings.Contains(helper, needle) {
			t.Fatalf("release_asset_common.sh missing canonical release asset wiring: %s", needle)
		}
	}
	for _, needle := range []string{
		`package_workers="${PULSE_RELEASE_PACKAGE_WORKERS:-4}"`,
		`package_server_target "${build_name}" &`,
		`pulse_release_stage_server_archive \`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing bounded parallel archive assembly: %s", needle)
		}
	}
}

func TestProPackagingBuildsFrontendEmbedWithoutTransferringBundle(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "build-release-binaries.sh"))
	if err != nil {
		t.Fatalf("read build-release-binaries.sh: %v", err)
	}
	script := string(content)

	for _, needle := range []string{
		`build_frontend >"${frontend_log}" 2>&1 &`,
		`npm --prefix frontend-modern ci`,
		`npm --prefix frontend-modern run build`,
		`if [[ "${PROFILE}" == "full" ]]; then`,
		`cp -a frontend-modern/dist/. "${FRONTEND_DIR}/"`,
		`if [[ "${component}" == server || "${component}" == control-plane ]]; then`,
		`finish_frontend`,
		`wait -n -p completed_pid "${active_pids[@]}"`,
		`completed release compilation child is not in the active task set`,
		`transfer public Unified Agent binaries only`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release-binaries.sh missing Pro frontend embed contract: %s", needle)
		}
	}
	if strings.Contains(script, `if [[ "${PROFILE}" == "full" ]]; then
	    mkdir -p "${FRONTEND_DIR}"
	    echo "Building exact-SHA frontend bundle..."
	    npm --prefix frontend-modern ci`) {
		t.Fatal("Pro packaging must build the frontend embed prerequisite")
	}
	if strings.Contains(script, `wait -n -p completed_pid;`) {
		t.Fatal("release compilation must not let wait -n consume the independent frontend child")
	}
	serverGate := strings.Index(script, `if [[ "${component}" == server || "${component}" == control-plane ]]; then`)
	serverLaunch := strings.Index(script, `build_one "${component}" "${target}"`)
	if serverGate < 0 || serverLaunch < 0 || serverGate > serverLaunch ||
		!strings.Contains(script[serverGate:serverLaunch], "finish_frontend") {
		t.Fatal("server and control-plane tasks must join the frontend build before launch")
	}
}

func TestReleaseContainerTargetsConsumeImmutableCandidate(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	prepareBytes, err := os.ReadFile(repoFile("scripts", "prepare-release-container-context.sh"))
	if err != nil {
		t.Fatalf("read prepare-release-container-context.sh: %v", err)
	}
	dockerfile := string(dockerfileBytes)
	prepareScript := string(prepareBytes)

	for _, needle := range []string{
		"FROM pulse-runtime-foundation AS prebuilt-runtime-base",
		"COPY --from=release_payload /${TARGETARCH:-amd64}/bin/pulse ./pulse",
		"FROM prebuilt-runtime-base AS runtime_prebuilt",
		"COPY --from=release_payload /amd64/bin/pulse /opt/pulse/bin/pulse-linux-amd64",
		"COPY --from=release_payload /arm64/bin/pulse /opt/pulse/bin/pulse-linux-arm64",
		"FROM alpine:3.20@sha256:",
		"AS agent_runtime_prebuilt",
	} {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing immutable-candidate container target: %s", needle)
		}
	}
	for _, needle := range []string{
		`pulse-v${version}-linux-${arch}.tar.gz`,
		`validate_archive_entries "${archive}"`,
		`tar --no-same-owner --no-same-permissions -xzf`,
		`bin/pulse.sig`,
		`bin/pulse.sshsig`,
		`--exclude=pulse.sig`,
		`--exclude=pulse.sshsig`,
		`find "${output_dir}/arm64" -depth -mindepth 1`,
	} {
		if !strings.Contains(prepareScript, needle) {
			t.Fatalf("prepare-release-container-context.sh missing candidate guard: %s", needle)
		}
	}
}

func TestReleaseContainerContextTreatsServerSignaturesAsArchitectureBound(t *testing.T) {
	version := "6.3.0-rc.test"
	releaseDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "container-context")

	writeArchive := func(arch string, driftUniversalPayload bool) {
		t.Helper()
		archivePath := filepath.Join(releaseDir, "pulse-v"+version+"-linux-"+arch+".tar.gz")
		archiveFile, err := os.Create(archivePath)
		if err != nil {
			t.Fatalf("create %s archive: %v", arch, err)
		}
		gzipWriter := gzip.NewWriter(archiveFile)
		tarWriter := tar.NewWriter(gzipWriter)
		files := map[string]string{
			"bin/pulse":                          "server-" + arch,
			"bin/pulse.sig":                      "minisign-" + arch,
			"bin/pulse.sshsig":                   "sshsig-" + arch,
			"bin/pulse-agent-linux-amd64":        "shared-agent",
			"bin/pulse-agent-linux-amd64.sig":    "shared-agent-minisign",
			"bin/pulse-agent-linux-amd64.sshsig": "shared-agent-sshsig",
			"bin/pulse-agent-windows-amd64.exe":  "shared-windows-amd64-agent",
			"bin/pulse-agent-windows-arm64.exe":  "shared-windows-arm64-agent",
			"bin/pulse-agent-windows-386.exe":    "shared-windows-386-agent",
			"scripts/install-container-agent.sh": "shared-container-installer",
			"scripts/install-docker.sh":          "shared-docker-installer",
			"scripts/install.sh":                 "shared-agent-installer",
			"scripts/install.sh.sig":             "shared-installer-minisign",
			"scripts/install.sh.sshsig":          "shared-installer-sshsig",
			"VERSION":                            version,
		}
		if driftUniversalPayload {
			files["scripts/install.sh"] = "drifted-agent-installer"
		}
		for name, content := range files {
			header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
			if strings.HasPrefix(name, "bin/") || strings.HasSuffix(name, ".sh") {
				header.Mode = 0o755
			}
			if err := tarWriter.WriteHeader(header); err != nil {
				t.Fatalf("write %s header to %s archive: %v", name, arch, err)
			}
			if _, err := tarWriter.Write([]byte(content)); err != nil {
				t.Fatalf("write %s to %s archive: %v", name, arch, err)
			}
		}
		for _, windowsArch := range []string{"amd64", "arm64", "386"} {
			name := "bin/pulse-agent-windows-" + windowsArch
			target := "pulse-agent-windows-" + windowsArch + ".exe"
			header := &tar.Header{Name: name, Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: target}
			if err := tarWriter.WriteHeader(header); err != nil {
				t.Fatalf("write %s alias to %s archive: %v", name, arch, err)
			}
		}
		if err := tarWriter.Close(); err != nil {
			t.Fatalf("close %s tar stream: %v", arch, err)
		}
		if err := gzipWriter.Close(); err != nil {
			t.Fatalf("close %s gzip stream: %v", arch, err)
		}
		if err := archiveFile.Close(); err != nil {
			t.Fatalf("close %s archive: %v", arch, err)
		}
	}

	writeArchive("amd64", false)
	writeArchive("arm64", false)
	cmd := exec.Command(repoFile("scripts", "prepare-release-container-context.sh"), releaseDir, version, outputDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare context with architecture-bound server signatures: %v\n%s", err, output)
	}
	for _, relativePath := range []string{
		"amd64/bin/pulse",
		"amd64/bin/pulse.sig",
		"amd64/bin/pulse.sshsig",
		"arm64/bin/pulse",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(relativePath))); err != nil {
			t.Fatalf("prepared context missing %s: %v", relativePath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "arm64", "scripts", "install.sh")); !os.IsNotExist(err) {
		t.Fatalf("prepared context retained duplicate universal payload: %v", err)
	}
	for _, windowsArch := range []string{"amd64", "arm64", "386"} {
		aliasPath := filepath.Join(outputDir, "amd64", "bin", "pulse-agent-windows-"+windowsArch)
		if _, err := os.Lstat(aliasPath); !os.IsNotExist(err) {
			t.Fatalf("prepared context retained recreated Windows alias %s: %v", aliasPath, err)
		}
	}

	writeArchive("arm64", true)
	cmd = exec.Command(repoFile("scripts", "prepare-release-container-context.sh"), releaseDir, version, outputDir)
	if output, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(output), "differ outside the target server binary and its signatures") {
		t.Fatalf("prepare context accepted drifted universal payload: err=%v\n%s", err, output)
	}
}

func TestValidateReleaseScansArchivesOnceInParallel(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "validate-release.sh"))
	if err != nil {
		t.Fatalf("read validate-release.sh: %v", err)
	}
	script := string(content)
	for _, needle := range []string{
		`platform_tar_entries=(`,
		`check_tar_entries_nonempty "$tarball" "${platform_tar_entries[@]}"`,
		`validate_platform_tarball "${arch}" >"${log_path}" 2>&1 &`,
		`validate_universal_tarball >"${archive_validation_logs}/universal.log" 2>&1 &`,
		`wait "${archive_validation_pids[$index]}"`,
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("validate-release.sh missing single-pass parallel archive validation: %s", needle)
		}
	}
	if strings.Contains(script, `tar -tzf "$tarball"`) {
		t.Fatal("validate-release.sh must not rescan every platform archive with tar -t")
	}
}

func TestProviderMSPReleaseBundleIsRequiredAndValidated(t *testing.T) {
	validateBytes, err := os.ReadFile(repoFile("scripts", "validate-release.sh"))
	if err != nil {
		t.Fatalf("read validate-release.sh: %v", err)
	}
	validate := string(validateBytes)
	for _, needle := range []string{
		`"pulse-provider-msp-v${PULSE_VERSION}.tar.gz"`,
		`section "Validating provider MSP bundle"`,
		`provider_msp_root="pulse-provider-msp-v${PULSE_VERSION}"`,
		`CONTROL_PLANE_IMAGE=ghcr.io/rcourtman/pulse-control-plane:${PULSE_TAG}`,
		`CP_PULSE_IMAGE=ghcr.io/rcourtman/pulse:${PULSE_TAG}`,
	} {
		if !strings.Contains(validate, needle) {
			t.Fatalf("validate-release.sh missing provider MSP bundle guard: %s", needle)
		}
	}

	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	workflow := string(workflowBytes)
	if !strings.Contains(workflow, `"pulse-provider-msp-${TAG}.tar.gz"`) {
		t.Fatal("create-release.yml must read the exact provider MSP asset before customer activation")
	}
}

func TestHelmChartShipsOpenShiftProfile(t *testing.T) {
	read := func(parts ...string) string {
		t.Helper()
		content, err := os.ReadFile(repoFile(parts...))
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Join(parts...), err)
		}
		return string(content)
	}

	values := read("deploy", "helm", "pulse", "values.yaml")
	agent := read("deploy", "helm", "pulse", "templates", "agent.yaml")
	deployment := read("deploy", "helm", "pulse", "templates", "deployment.yaml")
	rbac := read("deploy", "helm", "pulse", "templates", "agent-rbac.yaml")
	helmCI := read(".github", "workflows", "helm-ci.yml")
	docs := read("docs", "KUBERNETES.md")

	for _, required := range []string{
		"openShift:",
		"kubernetesAgent:",
		"clusterID:",
		"rbac:",
	} {
		if !strings.Contains(values, required) {
			t.Fatalf("values.yaml missing OpenShift chart value %q", required)
		}
	}
	for _, required := range []string{
		`$openShiftAgent := and .Values.openShift.enabled .Values.openShift.kubernetesAgent.enabled`,
		`- --enable-kubernetes`,
		`- --enable-host=false`,
		`"name" "PULSE_AGENT_ID"`,
		`$dockerSocketEnabled := and .Values.agent.dockerSocket.enabled (not $openShiftAgent)`,
		`"runAsNonRoot" true`,
		`omit $openShiftSecurityContext "runAsUser" "runAsGroup"`,
	} {
		if !strings.Contains(agent, required) {
			t.Fatalf("agent template missing OpenShift contract %q", required)
		}
	}
	for _, required := range []string{
		`.Values.openShift.enabled`,
		`omit $openShiftContainerSecurityContext "runAsUser" "runAsGroup"`,
		`"allowPrivilegeEscalation" false`,
	} {
		if !strings.Contains(deployment, required) {
			t.Fatalf("server deployment missing OpenShift SCC contract %q", required)
		}
	}
	for _, required := range []string{
		"kind: ClusterRole",
		"kind: ClusterRoleBinding",
		`apiGroups: ["metrics.k8s.io"]`,
		`resources: ["nodes", "pods"]`,
		`apiGroups: ["discovery.k8s.io"]`,
		`resources: ["endpointslices"]`,
		`apiGroups: ["rbac.authorization.k8s.io"]`,
	} {
		if !strings.Contains(rbac, required) {
			t.Fatalf("agent RBAC template missing read-only collector rule %q", required)
		}
	}
	if strings.Contains(rbac, "- secrets") || strings.Contains(rbac, "- nodes/proxy") {
		t.Fatal("OpenShift default role must not grant Secrets or direct kubelet proxy access")
	}
	for _, required := range []string{
		"Render and verify the OpenShift profile",
		"--show-only templates/agent-rbac.yaml",
		`grep -Eq "runAs(User|Group):|fsGroup:"`,
		`grep -q "/var/run/docker.sock"`,
	} {
		if !strings.Contains(helmCI, required) {
			t.Fatalf("Helm CI missing OpenShift render assertion %q", required)
		}
	}
	if !strings.Contains(docs, "--set openShift.enabled=true") ||
		!strings.Contains(docs, "--set openShift.kubernetesAgent.enabled=true") ||
		!strings.Contains(docs, "create secret generic pulse-server-env") ||
		!strings.Contains(docs, "create secret generic pulse-agent-env") {
		t.Fatal("Kubernetes guide must document the shipped OpenShift profile")
	}
	if strings.Contains(docs, "--set-string agent.secretEnv.data.PULSE_TOKEN") {
		t.Fatal("OpenShift guide must not persist the agent token in Helm release values")
	}
}

func shieldsBadgeMessage(value string) string {
	return strings.ReplaceAll(value, "-", "--")
}

// TestAgentBuildCacheDoesNotResurrectPulseAgentPackage guards the repository's
// GHCR package list, which is a user-facing surface. A registry cache ref
// creates the package it points at, so pointing the agent_runtime build cache
// at ghcr.io/<owner>/pulse-agent recreated an empty package on every release.
// It then sat in the repo's Packages sidebar beside pulse, pulse-control-plane
// and pulse-chart/pulse, reading like a pullable agent image even though no
// workflow publishes it. Only images a release workflow actually pushes may
// own a package; the unified agent ships inside the main pulse image.
func TestAgentBuildCacheDoesNotResurrectPulseAgentPackage(t *testing.T) {
	workflowDir := repoFile(".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("read workflow dir: %v", err)
	}

	// Registry-qualified refs only: the release binaries are legitimately named
	// pulse-agent-<os>-<arch> and must keep matching nothing here. Workflow
	// expressions are collapsed first so an owner interpolated as
	// ${{ github.repository_owner }} cannot hide the ref behind its spaces.
	workflowExpr := regexp.MustCompile(`\$\{\{[^}]*\}\}`)
	packageRef := regexp.MustCompile(`(?:ghcr\.io|docker\.io)/[^\s"']*/pulse-agent\b|(?:^|\s)rcourtman/pulse-agent\b`)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(workflowDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			if match := packageRef.FindString(workflowExpr.ReplaceAllString(line, "EXPR")); match != "" {
				t.Fatalf("%s:%d targets the unpublished pulse-agent package (%q); no workflow may reference it, buildcache refs included", name, i+1, strings.TrimSpace(match))
			}
		}
	}

	release, err := os.ReadFile(repoFile(".github", "workflows", "qualify-release-containers.yml"))
	if err != nil {
		t.Fatalf("read qualify-release-containers.yml: %v", err)
	}
	releaseText := string(release)
	if !strings.Contains(releaseText, `--target agent_runtime_prebuilt`) {
		t.Fatal("qualify-release-containers.yml must assemble the candidate agent image without targeting an unpublished package")
	}
	if strings.Contains(releaseText, "agent-buildcache") {
		t.Fatal("exact-candidate release qualification must not create a remote agent image cache")
	}

	values, err := os.ReadFile(repoFile("deploy", "helm", "pulse", "values.yaml"))
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}
	agentBlock := topLevelYAMLBlock(t, string(values), "agent")
	if !strings.Contains(agentBlock, "repository: rcourtman/pulse\n") {
		t.Fatal("chart agent.image.repository must default to the published rcourtman/pulse image")
	}
}

// topLevelYAMLBlock returns the lines of a top-level mapping key, from the key
// itself up to the next unindented key.
func topLevelYAMLBlock(t *testing.T, doc string, key string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start := -1
	for i, line := range lines {
		if line == key+":" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("values.yaml missing top-level %q key", key)
	}
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.Join(lines[start:i], "\n")
	}
	return strings.Join(lines[start:], "\n")
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
	convergenceContent, err := os.ReadFile(repoFile(".github", "workflows", "release-convergence.yml"))
	if err != nil {
		t.Fatalf("read release-convergence.yml: %v", err)
	}

	workflow := string(content)
	validationWorkflow := string(validationContent)
	convergenceWorkflow := string(convergenceContent)
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
		`--rawfile body "$NOTES_FILE"`,
		`--input "$RELEASE_PAYLOAD"`,
		`--expected-body-file "$NOTES_FILE"`,
		`historical_asset_backfill_only=${HISTORICAL_ASSET_BACKFILL_ONLY}`,
		`if: ${{ always() && needs.prepare.result == 'success' && needs.build_release_candidate.result == 'success' && needs.create_release.result == 'success' && needs.prepare.outputs.historical_asset_backfill_only != 'true' }}`,
		`candidate_manifest_artifact: ${{ needs.build_release_candidate.outputs.manifest_artifact_name }}`,
		`if: ${{ needs.prepare.outputs.historical_asset_backfill_only == 'true' }}`,
		`permissions:`,
		`issues: write`,
		`statuses: write`,
		`ACTUAL_RELEASE_TAG=$(jq -r '.tag_name // empty' "$RELEASE_JSON_FILE")`,
		`ACTUAL_TARGET_COMMITISH=$(jq -r '.target_commitish // empty' "$RELEASE_JSON_FILE")`,
		`Draft release ${RELEASE_ID} is bound to tag ${ACTUAL_RELEASE_TAG}, expected ${TAG}.`,
		`Draft release ${RELEASE_ID} target_commitish is ${ACTUAL_TARGET_COMMITISH}, expected ${HEAD_SHA}.`,
		`./scripts/backfill-release-assets.sh --tag "${{ needs.prepare.outputs.tag }}" --repo "${{ github.repository }}"`,
		`./scripts/validate-published-release.sh "${{ needs.prepare.outputs.tag }}" "${{ github.repository }}"`,
		// End-to-end install.sh smoke must run downstream of
		// validate_release_assets on every release that is not a
		// historical asset backfill. Without this wiring the smoke
		// workflow exists but never actually protects a release —
		// exactly the regression class that let rc.1 → rc.5 ship with
		// broken install.sh.
		`uses: ./.github/workflows/install-sh-smoke.yml`,
		`install_sh_smoke:`,
		`needs.validate_release_assets.result == 'success'`,
		`needs.prepare.outputs.historical_asset_backfill_only != 'true'`,
		`repository: ${{ github.repository }}`,
		`asset_source: staged`,
		`release_id: ${{ needs.create_release.outputs.release_id }}`,
		// Helm chart publish must be called explicitly from create-release
		// because the draft→PATCH(draft=false) publish path does NOT fire
		// the `release: published` webhook (GitHub-documented quirk). v6
		// rc.1 → rc.5 published successfully but never produced a Helm
		// chart on the GitHub Pages index, breaking
		// `helm install pulse pulse/pulse --version 6.0.0-rc.X`.
		`uses: ./.github/workflows/publish-helm-chart.yml`,
		`publish_helm_chart:`,
		`chart_version: ${{ needs.prepare.outputs.version }}`,
		`app_version: ${{ needs.prepare.outputs.version }}`,
		// Draft-only mode stops after staged validation and skips the
		// customer activation sequence.
		`needs.prepare.outputs.historical_asset_backfill_only != 'true' && github.event.inputs.draft_only != 'true'`,
		`dispatch_release_convergence:`,
		`release-convergence.yml/dispatches`,
		`return_run_details: true`,
		`activate_release:`,
		`continue-on-error: true`,
		`Publish the fully staged release`,
		`'{draft: false, make_latest: $make_latest}'`,
		`returning ${TAG} to draft quarantine`,
		`release-activation.json`,
		`release_commit_verdict:`,
		`Release Activation Commit Verdict`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("create-release.yml missing required installer upload step: %s", needle)
		}
	}

	publishedReleaseGuard := `needs.prepare.outputs.historical_asset_backfill_only != 'true' && github.event.inputs.draft_only != 'true'`
	for _, job := range []string{"install_sh_smoke", "publish_helm_chart"} {
		block := workflowJobBlock(t, workflow, job)
		if !strings.Contains(block, publishedReleaseGuard) {
			t.Fatalf("create-release.yml job %s must skip historical backfill and draft-only runs before invoking downstream workflow_call", job)
		}
	}
	installSmokeJob := workflowJobBlock(t, workflow, "install_sh_smoke")
	if !strings.Contains(installSmokeJob, "contents: write") {
		t.Fatal("create-release.yml install_sh_smoke must grant contents: write so the called workflow can read unpublished draft assets")
	}
	readinessJob := workflowJobBlock(t, workflow, "release_readiness")
	if !strings.Contains(readinessJob, publishedReleaseGuard) {
		t.Fatal("release_readiness must skip historical backfill and draft-only runs")
	}
	for _, job := range []string{"promote_floating_tags", "publish_helm_pages", "promote_private_pro_runtime", "update_stable_demo"} {
		if strings.Contains(workflow, "\n  "+job+":\n") {
			t.Fatalf("create-release.yml must not mutate customer surface %s inline", job)
		}
	}
	for _, needle := range []string{
		`await_activation_commit:`,
		`release-activation.json`,
		`acquire_customer_promotion_lease:`,
		`uses: ./.github/workflows/promote-floating-tags.yml`,
		`uses: ./.github/workflows/helm-pages.yml`,
		`uses: ./.github/workflows/promote-private-pro-runtime.yml`,
		`uses: ./.github/workflows/update-demo-server.yml`,
	} {
		if !strings.Contains(convergenceWorkflow, needle) {
			t.Fatalf("release-convergence.yml missing durable customer-promotion contract: %s", needle)
		}
	}

	if !strings.Contains(workflow, `draft: true`) {
		t.Fatal("create-release.yml must validate the release while it remains staged as a draft")
	}
	createJob := workflowJobBlock(t, workflow, "create_release")
	if strings.Contains(createJob, `draft=false`) || strings.Contains(createJob, `Publish release`) {
		t.Fatal("create_release must stage assets without crossing the customer publication boundary")
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
		`Validate release body integrity`,
		`--validate-body-file "$RELEASE_BODY_FILE"`,
		`--expected-body-file "$CLEAN_BODY_FILE"`,
		`Quarantine malformed release body`,
		`The release was quarantined as a draft without deleting its assets.`,
	}
	for _, needle := range validationRequired {
		if !strings.Contains(validationWorkflow, needle) {
			t.Fatalf("validate-release-assets.yml missing required status publication contract: %s", needle)
		}
	}
}

func TestCurrentStablePatchReleasePacketTracksInstallMetadata(t *testing.T) {
	version := currentReleaseVersion(t)
	if isPrereleaseVersion(version) {
		t.Skip("current release is a prerelease")
	}
	previous, ok := previousStablePatchVersion(version)
	if !ok {
		t.Skip("current release is not a stable patch release")
	}
	releaseBranch := requiredReleaseBranchForVersion(t, version)

	releaseNotesPath := repoFile("docs", "releases", "RELEASE_NOTES_v"+version+".md")
	changelogPath := repoFile("docs", "releases", "V6_CHANGELOG_v"+version+".md")

	assertFileContainsAllNormalized(t, releaseNotesPath,
		"`v"+version+"` is a stable patch release",
		"`v"+previous+"`",
		"Use the normal v6 install or update flow",
		"integrated exact-SHA candidate checks",
		"`v6.3.1`-only exception",
		"not Authenticode-signed",
		"Unknown Publisher warning",
		"exact-SHA checksums, detached signatures, immutable-manifest verification",
		"`no-mobile-impact`",
		"rollback target is `v"+previous+"`",
	)
	assertFileContainsAllNormalized(t, changelogPath,
		"Version: `v"+version+"`",
		"Rollback target: `v"+previous+"`",
		"Promotion path: emergency stable patch from `"+releaseBranch+"`",
		"Windows signing decision: version-bound `v6.3.1` owner exception",
		"not Authenticode-signed",
		"Unknown Publisher warning",
		"Mobile decision: `no-mobile-impact`",
	)
	assertFileContainsAll(t, repoFile("docs", "RELEASE_NOTES.md"),
		"docs/releases/RELEASE_NOTES_v"+version+".md",
		"docs/releases/V6_CHANGELOG_v"+version+".md",
	)
	assertFileContainsAll(t, repoFile("docs", "UPGRADE_v6.md"),
		"docs/releases/RELEASE_NOTES_v"+version+".md",
		"docs/releases/V6_CHANGELOG_v"+version+".md",
	)
	assertFileContainsAll(t, repoFile("deploy", "helm", "pulse", "Chart.yaml"),
		"version: "+version,
		`appVersion: "`+version+`"`,
		"raw.githubusercontent.com/rcourtman/Pulse/v"+version+"/docs/images/pulse-logo.svg",
		"blob/v"+version+"/docs/KUBERNETES.md",
	)
	assertFileContainsAll(t, repoFile("deploy", "helm", "pulse", "README.md"),
		"Version-"+version+"-informational",
		"AppVersion-"+version+"-informational",
		"Autogenerated from chart metadata using [helm-docs v1.14.2]",
	)
	assertFileContainsAll(t, repoFile("docker-compose.yml"),
		"image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}",
	)
	assertFileContainsAll(t, repoFile("scripts", "install-docker.sh"),
		`CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`,
	)
	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active stable `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		"This patch release uses the stable hotfix path with `rollback_version=v"+previous+"`, `hotfix_exception=true`, a release-owner reason, and no fabricated same-version RC tag.",
		"For the active stable `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"`",
	)
}

func TestCurrentStableMinorReleasePacketTracksInstallMetadata(t *testing.T) {
	version := currentReleaseVersion(t)
	if isPrereleaseVersion(version) {
		t.Skip("current release is a prerelease")
	}
	parts, valid := parseStableVersion(version)
	if !valid || parts[1] == 0 || parts[2] != 0 {
		t.Skip("current release is not a stable minor release")
	}
	previous, ok := previousStableForPrereleaseVersion(version + "-rc.1")
	if !ok {
		t.Fatal("stable minor release has no earlier stable rollback packet")
	}

	releaseNotesPath := repoFile("docs", "releases", "RELEASE_NOTES_v"+version+".md")
	changelogPath := repoFile("docs", "releases", "V6_CHANGELOG_v"+version+".md")

	assertFileContainsAllNormalized(t, releaseNotesPath,
		"`v"+version+"` is a stable minor release",
		"stable `v"+previous+"`",
		"## Highlights",
		"Operational Trust",
		"Actions provides a dedicated inbox",
		"existing Pulse Mobile candidate",
		"not Authenticode-signed",
		"Unknown Publisher warning",
		"The rollback target is `v"+previous+"`",
	)
	assertFileContainsAllNormalized(t, changelogPath,
		"Version: `v"+version+"`",
		"Previous stable: `v"+previous+"`",
		"Rollback target: `v"+previous+"`",
		"Promotion path: owner-approved exact-SHA stable cutoff from `main`",
		"Mobile decision: `existing-mobile-build-compatible`",
	)
	assertFileContainsAll(t, repoFile("docs", "RELEASE_NOTES.md"),
		"docs/releases/RELEASE_NOTES_v"+version+".md",
		"docs/releases/V6_CHANGELOG_v"+version+".md",
	)
	assertFileContainsAll(t, repoFile("docs", "UPGRADE_v6.md"),
		"docs/releases/RELEASE_NOTES_v"+version+".md",
		"docs/releases/V6_CHANGELOG_v"+version+".md",
	)
	assertFileContainsAll(t, repoFile("deploy", "helm", "pulse", "Chart.yaml"),
		"version: "+version,
		`appVersion: "`+version+`"`,
		"raw.githubusercontent.com/rcourtman/Pulse/v"+version+"/docs/images/pulse-logo.svg",
		"blob/v"+version+"/docs/KUBERNETES.md",
	)
	assertFileContainsAll(t, repoFile("deploy", "helm", "pulse", "README.md"),
		"Version-"+version+"-informational",
		"AppVersion-"+version+"-informational",
		"Autogenerated from chart metadata using [helm-docs v1.14.2]",
	)
	assertFileContainsAll(t, repoFile("docker-compose.yml"),
		"image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}",
	)
	assertFileContainsAll(t, repoFile("scripts", "install-docker.sh"),
		`CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`,
	)
	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active stable `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		"`rollback_version=v"+previous+"`",
		"The exact stable `main` SHA must pass the no-publication dry run before the same SHA is dispatched through the single-build publish workflow.",
		"The stable server cut is classified `existing-mobile-build-compatible`.",
		"explicit version-bound decision",
		"For the active stable `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"`",
	)
}

func TestCurrentPrereleasePacketTracksInstallMetadata(t *testing.T) {
	version := currentReleaseVersion(t)
	if !isPrereleaseVersion(version) {
		t.Skip("current release is stable")
	}
	previous, ok := previousStableForPrereleaseVersion(version)
	if !ok {
		t.Skip("current prerelease does not have a previous stable patch")
	}
	stableTarget, _, ok := strings.Cut(version, "-")
	if !ok {
		t.Fatalf("current prerelease %q has no stable target", version)
	}
	comparisonVersion, ok := previousPrereleaseVersion(version)
	if !ok {
		comparisonVersion = previous
	}

	releaseNotesPath := repoFile("docs", "releases", "RELEASE_NOTES_v"+version+".md")
	changelogPath := repoFile("docs", "releases", "V6_CHANGELOG_v"+version+".md")

	assertFileContainsAllNormalized(t, releaseNotesPath,
		"# Pulse v"+version+" Release Notes",
		"## What's improved",
		"## Fixes",
		"## Before you upgrade",
		"## Known issues",
		"Complete standalone PBS details",
		"Complete LXC filesystem coverage",
		"Earlier disk-cabling warnings",
		"Alert policy is resolved consistently",
		"Alert-event queries now allocate",
		"Pulse Mobile does not consume the changed PBS browser detail or alert evaluation internals",
		"not Authenticode-signed",
		"Unknown Publisher warning",
	)
	assertFileContainsAllNormalized(t, changelogPath,
		"Version: `v"+version+"`",
		"Previous stable: `v"+previous+"`",
		"Rollback target: `v"+previous+"`",
		"Promotion path: exact-SHA single-build release candidate from `main`",
		"This changelog describes the changes since `v"+comparisonVersion+"`",
		"SMART UDMA CRC counter growth now raises a disk-health warning",
		"Alert configuration resolves through one declarative policy fold",
		"Legacy transition-tracking maps have been removed",
		"Standalone PBS rows open the canonical resource drawer",
		"Proxmox LXC filesystem collection uses host-namespace `statfs`",
		"Alert-event queries allocate from the bounded effective result limit",
		"Release builds use Go 1.26.7",
		"Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode",
		"Mobile decision: `no-mobile-impact`",
		"no companion build or public store rollout is required",
	)
	if version == "6.3.0-rc.6" {
		assertFileContainsAllNormalized(t, releaseNotesPath,
			"Chart and resource-query services now qualify independently from the residual API router, shrinking the root test critical path.",
			"Public server and provider control-plane images publish and attest in parallel from one verified exact-candidate payload.",
			"PVE compilation remains credential-free. GitHub-hosted jobs retain signing, release mutation, and publication credentials.",
		)
		assertFileContainsAllNormalized(t, changelogPath,
			"Chart handling and resource queries are production packages with independent test scheduling",
			"Exact-version public Docker staging overlaps qualification, and server and provider control-plane products publish as parallel matrix legs.",
			"Publication still requires exact-source identity, immutable manifests, signatures, public/private artifact integrity, installer smoke, and final convergence verification.",
		)
	}
	assertFileContainsAll(t, repoFile("docs", "RELEASE_NOTES.md"),
		"docs/releases/RELEASE_NOTES_v"+version+".md",
		"docs/releases/V6_CHANGELOG_v"+version+".md",
		"current v6 release candidate packet",
	)
	assertFileContainsAll(t, repoFile("docs", "UPGRADE_v6.md"),
		"docs/releases/RELEASE_NOTES_v"+version+".md",
		"docs/releases/V6_CHANGELOG_v"+version+".md",
		"current v6 release candidate packet",
	)
	assertFileContainsAll(t, repoFile("deploy", "helm", "pulse", "Chart.yaml"),
		"version: "+version,
		`appVersion: "`+version+`"`,
		"raw.githubusercontent.com/rcourtman/Pulse/v"+version+"/docs/images/pulse-logo.svg",
		"blob/v"+version+"/docs/KUBERNETES.md",
	)
	assertFileContainsAll(t, repoFile("deploy", "helm", "pulse", "README.md"),
		"Version-"+shieldsBadgeMessage(version)+"-informational",
		"AppVersion-"+shieldsBadgeMessage(version)+"-informational",
		"Autogenerated from chart metadata using [helm-docs v1.14.2]",
	)
	assertFileContainsAll(t, repoFile("docker-compose.yml"),
		"image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}",
	)
	assertFileContainsAll(t, repoFile("scripts", "install-docker.sh"),
		`CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`,
	)
	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active prerelease `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		"This prerelease keeps `rollback_version=v"+previous+"`, publishes a versioned public GitHub prerelease plus versioned Docker and Helm artifacts, and does not move stable/latest install pointers or stable semver aliases.",
		"For the active prerelease `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"` until the next governed stable cut moves them forward.",
		"The changes since `v"+comparisonVersion+"` do not require a Pulse Mobile client change and preserve the existing mobile, Relay, onboarding, and mobile-facing API contracts, so the server cut is classified `no-mobile-impact`; no companion upload or public mobile-store rollout is part of this candidate.",
		"The prerelease Windows path retains exact-SHA, checksum, and detached-signature verification without Authenticode. Stable `v"+stableTarget+"` also skips SignPath under the standing unavailable policy",
	)
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
		`uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3`,
		`uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0`,
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
		`url="http://127.0.0.1:${HOST_PORT}/${script_name}"`,
		`^# Pulse Unified Agent Installer`,
		`--token-file`,
		`TokenFile`,
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
	// Format drift guard — ssh-keygen -Y verify -f expects an allowed_signers
	// file whose FIRST field is the principal. The docs shipped the key in
	// authorized_keys order (principal last, parsed as a comment), so the
	// documented verification failed against a perfectly good signature.
	// Reported by a customer against v6.0.5 on 2026-07-13.
	const allowedSignersLine = `pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer`
	if !strings.Contains(readme, allowedSignersLine) {
		t.Fatalf("README.md verification snippet must publish the key as an allowed_signers line (principal first), not authorized_keys order")
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
	if !strings.Contains(installDocs, allowedSignersLine) {
		t.Fatalf("docs/INSTALL.md verification snippet must publish the key as an allowed_signers line (principal first), not authorized_keys order")
	}
	for _, needle := range localRequired {
		if !strings.Contains(localValidator, needle) {
			t.Fatalf("validate-release.sh missing signed sidecar validation: %s", needle)
		}
	}
	if strings.Contains(localValidator, `url="http://127.0.0.1:${HOST_PORT}/download/${script_name}"`) {
		t.Fatal("validate-release.sh must smoke-test /install.sh and /install.ps1, not non-existent /download/install.* routes")
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

func TestDockerBuildUsesCanonicalReleaseLdflags(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(dockerfileBytes)
	dockerRequired := []string{
		`FROM --platform=linux/amd64 node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS frontend-builder`,
		`FROM --platform=linux/amd64 golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS backend-builder`,
		`FROM backend-builder AS release-assets-builder`,
		`FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS agent_runtime`,
		`FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS pulse-runtime-foundation`,
		`FROM pulse-runtime-foundation AS pulse-runtime-base`,
		`FROM pulse-runtime-foundation AS prebuilt-runtime-base`,
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
		strings.Contains(dockerfile, `FROM --platform=linux/amd64 golang:1.26.7-alpine AS backend-builder`) ||
		strings.Contains(dockerfile, `FROM alpine:3.20 AS agent_runtime`) ||
		strings.Contains(dockerfile, `FROM alpine:3.20 AS pulse-runtime-base`) {
		t.Fatal("Dockerfile base images must be pinned by immutable @sha256 digests")
	}
	if builds, cleanBuilds := strings.Count(dockerfile, " go build \\"), strings.Count(dockerfile, "-buildvcs=false"); builds != cleanBuilds {
		t.Fatalf("Dockerfile release go builds must all disable automatic VCS stamping: builds=%d clean_builds=%d", builds, cleanBuilds)
	}

}

func TestDockerRuntimeShipsPinnedAppriseCLI(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(dockerfileBytes)
	required := []string{
		`ARG APPRISE_VERSION=1.12.0`,
		`AS apprise-builder`,
		`python3 -m venv /opt/apprise`,
		`/opt/apprise/bin/pip install --no-cache-dir "apprise==${APPRISE_VERSION}"`,
		`COPY --from=apprise-builder /opt/apprise /opt/apprise`,
		`ln -s /opt/apprise/bin/apprise /usr/local/bin/apprise`,
		`apprise --version | grep -F "Apprise v${APPRISE_VERSION}"`,
	}
	for _, needle := range required {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing pinned Apprise runtime contract: %s", needle)
		}
	}
	if strings.Count(dockerfile, `apprise --version | grep -F "Apprise v${APPRISE_VERSION}"`) < 2 {
		t.Fatal("Dockerfile must verify the pinned Apprise CLI in both its build and runtime stages")
	}
}

func TestAgentRuntimeImageDefaultsToUnifiedHostAndDockerMonitoring(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(dockerfileBytes)

	required := []string{
		`mkdir -p /var/lib/pulse-agent`,
		`PULSE_DISABLE_AUTO_UPDATE=true`,
		`PULSE_ENABLE_HOST=true`,
		`PULSE_ENABLE_DOCKER=true`,
		`PULSE_AGENT_ID_FILE=/var/lib/pulse-agent/agent-id`,
		`PULSE_STATE_DIR=/var/lib/pulse-agent`,
		`VOLUME ["/var/lib/pulse-agent"]`,
		`ENTRYPOINT ["/usr/local/bin/pulse-agent"]`,
	}
	for _, needle := range required {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile agent_runtime missing unified host and Docker contract: %s", needle)
		}
	}
	if strings.Contains(dockerfile, `PULSE_ENABLE_HOST=false`) {
		t.Fatal("agent_runtime must not silently force every deployment into workload-only mode")
	}
	if strings.Contains(dockerfile, `ENTRYPOINT ["/usr/local/bin/pulse-agent", "--enable-docker", "--enable-host=false"]`) {
		t.Fatal("agent_runtime must not hard-code module flags in ENTRYPOINT; env defaults keep user args overridable")
	}
}

func TestReleaseCandidateRequiresPlatformNativeAgentSigning(t *testing.T) {
	candidateWorkflowPath := repoFile(".github", "workflows", "build-release-candidate.yml")
	assertFileContainsAll(t, candidateWorkflowPath,
		`require_macos_signing:`,
		`require_windows_signing:`,
		`sign-macos-agent:`,
		`codesign --force --timestamp --options runtime`,
		`xcrun notarytool submit`,
		`--output-format json > notarization-result.json`,
		`result.get('status') != 'Accepted'`,
		`codesign --verify --deep --strict --verbose=2`,
		`sign-windows-agent:`,
		`collect-windows-signing:`,
		`windows_signing_backend:`,
		`signpath/github-action-submit-signing-request@b9d91eadd323de506c0c81cf0c7fe7438f3360fd # v2`,
		`github-artifact-id: ${{ steps.upload-unsigned-windows.outputs.artifact-id }}`,
		`wait-for-completion: false`,
		`windows-signing-request.json`,
		`Re-run failed jobs`,
		`SIGNPATH_API_TOKEN`,
		`SIGNPATH_ORGANIZATION_ID`,
		`SIGNPATH_PROJECT_SLUG`,
		`SIGNPATH_SIGNING_POLICY_SLUG`,
		`SIGNPATH_ARTIFACT_CONFIGURATION_SLUG`,
		`SIGNPATH_EXPECTED_CERTIFICATE_SUBJECT`,
		`signtool sign`,
		`signtool verify /pa /v`,
		`windows-signing-evidence.json`,
		`signerThumbprint`,
		`PULSE_AGENT_NATIVE_BINARIES_DIR:`,
	)
	candidateWorkflow, err := os.ReadFile(candidateWorkflowPath)
	if err != nil {
		t.Fatalf("read build-release-candidate.yml: %v", err)
	}
	if strings.Contains(string(candidateWorkflow), `spctl --assess --type execute`) {
		t.Fatal("bare command-line Mach-O binaries must not use Gatekeeper app assessment after notarization")
	}
	if strings.Contains(string(candidateWorkflow), `wait-for-completion: true`) {
		t.Fatal("SignPath submission must not block the Windows build job on manual approval; collection is a separate resumable job")
	}
	assertFileContainsAll(t, repoFile(".github", "workflows", "create-release.yml"),
		`require_macos_signing: true`,
		`require_windows_signing: ${{ needs.prepare.outputs.require_windows_signing == 'true' }}`,
		`!contains(inputs.version, '-') && 'ubuntu-24.04'`,
		`unsigned_windows_exception:`,
		`unsigned_windows_reason:`,
		`windows_signing_backend: signpath`,
	)
	assertFileContainsAll(t, repoFile(".github", "workflows", "release-dry-run.yml"),
		`Definitive Dry-Run Verdict`,
		`require_windows_signing: false`,
		`WINDOWS_AUTHENTICODE_AVAILABLE`,
		`require_result "exact-SHA release candidate" "$CANDIDATE_RESULT" success`,
		`require_result "stable demo no-mutation verification" "$DEMO_RESULT" success`,
	)
	assertFileContainsAll(t, repoFile("scripts", "release_control", "resolve_release_promotion.py"),
		`WINDOWS_AUTHENTICODE_AVAILABLE = False`,
		`WINDOWS_AUTHENTICODE_STANDING_UNSIGNED_MIN_VERSION = (6, 3, 2)`,
		`version not in {"6.1.0", "6.1.1", "6.1.2", "6.2.0", "6.2.1", "6.3.0", "6.3.1", "6.3.2"}`,
		`unsigned_windows_reason is required`,
		`not Authenticode-signed`,
		`require_windows_signing = not is_prerelease and not effective_unsigned_windows_exception`,
	)
	assertFileContainsAll(t, repoFile("scripts", "build-release.sh"),
		`PULSE_AGENT_NATIVE_BINARIES_DIR`,
		`native_targets=()`,
		`PULSE_REQUIRE_MACOS_SIGNING:-false`,
		`native_targets+=(darwin-amd64 darwin-arm64)`,
		`PULSE_REQUIRE_WINDOWS_SIGNING:-false`,
		`native_targets+=(windows-amd64 windows-arm64 windows-386)`,
		`Applied required platform-native signed Unified Agent binaries.`,
		`required native signing is enabled but PULSE_AGENT_NATIVE_BINARIES_DIR is empty.`,
	)
}

func TestReleaseWorkflowsUseSecretSafeAttestedImageBuilds(t *testing.T) {
	createReleaseBytes, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	candidateWorkflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "build-release-candidate.yml"))
	if err != nil {
		t.Fatalf("read build-release-candidate.yml: %v", err)
	}
	qualifierWorkflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "qualify-release-containers.yml"))
	if err != nil {
		t.Fatalf("read qualify-release-containers.yml: %v", err)
	}
	createRelease := string(createReleaseBytes) + "\n" + string(candidateWorkflowBytes) + "\n" + string(qualifierWorkflowBytes)
	createReleaseRequired := []string{
		`Exact-Candidate Container and Helm Smoke`,
		`prepare_cluster_tool`,
		`https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64`,
		`513a7213d6d3332dd9ef27c24dab35e5ef10a04fa27274fe1c14d8a246493ded`,
		`https://dl.k8s.io/release/v1.27.3/bin/linux/amd64/kubectl`,
		`fba6c062e754a120bc8105cde1344de200452fe014a8759e06e4eec7ed258a09`,
		`printf '%s  %s\n' "$expected_sha" "$destination" | sha256sum --check`,
		`test -x "$destination"`,
		`test "$(command -v kind)" = "$RUNNER_TEMP/kind"`,
		`test "$(command -v kubectl)" = "$RUNNER_TEMP/kubectl"`,
		`kind version`,
		`kubectl version --client=true`,
		`./scripts/prepare-release-container-context.sh`,
		`container_artifact_name`,
		`container_artifact: ${{ needs.build_release_candidate.outputs.container_artifact_name }}`,
		`source_sha: ${{ github.sha }}`,
		`release-container-payload.json`,
		`--target runtime_prebuilt`,
		`--target agent_runtime_prebuilt`,
		`--target control_plane_prebuilt`,
		`Verify container binaries match immutable candidate`,
		`PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}`,
		`Validate installer signing key pins`,
		`go run ./scripts/release_update_key.go public-key-ssh`,
		`install.sh scripts/pulse-auto-update.sh release/pulse-auto-update.sh`,
		`does not trust the configured release signing key.`,
		`id-token: write`,
		`attestations: write`,
		`uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4`,
	}
	containerJob := workflowJobBlock(t, string(qualifierWorkflowBytes), "qualify")
	for _, needle := range []string{
		`!contains(inputs.version, '-') && 'ubuntu-24.04'`,
		"pulse-pve-build",
	} {
		if !strings.Contains(containerJob, needle) {
			t.Fatalf("exact-candidate container qualification missing stable-hosted runner contract: %s", needle)
		}
	}
	if !strings.Contains(string(candidateWorkflowBytes), "always() && inputs.qualify_containers && needs.build.result == 'success'") {
		t.Fatal("standalone exact-candidate qualification must not inherit skipped native-signing dependencies")
	}
	for _, forbidden := range []string{
		"PULSE_UPDATE_SIGNING_KEY",
		"PULSE_LICENSE_PUBLIC_KEY",
		"packages: write",
		"docker/login-action",
	} {
		if strings.Contains(containerJob, forbidden) {
			t.Fatalf("exact-candidate container qualification must not receive release authority: %s", forbidden)
		}
	}
	controlPlaneDockerfileBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "Dockerfile.control-plane"))
	if err != nil {
		t.Fatalf("read Dockerfile.control-plane: %v", err)
	}
	controlPlaneDockerfile := string(controlPlaneDockerfileBytes)
	for _, needle := range []string{
		"FROM control-plane-runtime-foundation AS control_plane_prebuilt",
		"COPY --from=compiled_payload /binaries/pulse-control-plane-linux-${TARGETARCH:-amd64}",
		"FROM control-plane-runtime-foundation AS runtime",
	} {
		if !strings.Contains(controlPlaneDockerfile, needle) {
			t.Fatalf("Dockerfile.control-plane missing exact-candidate target: %s", needle)
		}
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
		`name: Publish ${{ matrix.image }} image`,
		`fail-fast: false`,
		`- server`,
		`- control-plane`,
		`provenance: mode=max`,
		`sbom: true`,
		`container_artifact:`,
		`source_sha:`,
		`Download exact-candidate container payload`,
		`Verify exact-candidate container payload`,
		`target: runtime_prebuilt`,
		`release_payload=${{ runner.temp }}/release-container-payload/payload/release`,
		`id: build_control_plane_image`,
		`if: matrix.image == 'server'`,
		`if: matrix.image == 'control-plane'`,
		`file: deploy/provider-msp/Dockerfile.control-plane`,
		`target: control_plane_prebuilt`,
		`compiled_payload=${{ runner.temp }}/release-container-payload/payload/compiled`,
		`subject-name: docker.io/rcourtman/pulse`,
		`subject-name: ghcr.io/${{ github.repository_owner }}/pulse`,
		`subject-name: docker.io/rcourtman/pulse-control-plane`,
		`subject-name: ghcr.io/${{ github.repository_owner }}/pulse-control-plane`,
		`rcourtman/pulse-control-plane:${{ steps.version.outputs.tag }}`,
		`ghcr.io/${{ github.repository_owner }}/pulse-control-plane:${{ steps.version.outputs.tag }}`,
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
	for _, forbidden := range []string{
		"PULSE_LICENSE_PUBLIC_KEY",
		"PULSE_UPDATE_SIGNING_KEY",
		"pulse_license_public_key",
		"pulse_update_signing_key",
	} {
		if strings.Contains(publish, forbidden) {
			t.Fatalf("publish-docker.yml must assemble the verified candidate without release build secrets: %s", forbidden)
		}
	}
}

func TestReleaseContainerQualificationBindsCheckoutToCallerCommit(t *testing.T) {
	qualifierBytes, err := os.ReadFile(repoFile(".github", "workflows", "qualify-release-containers.yml"))
	if err != nil {
		t.Fatalf("read qualify-release-containers.yml: %v", err)
	}
	candidateBytes, err := os.ReadFile(repoFile(".github", "workflows", "build-release-candidate.yml"))
	if err != nil {
		t.Fatalf("read build-release-candidate.yml: %v", err)
	}
	releaseBytes, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	contractBytes, err := os.ReadFile(repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"))
	if err != nil {
		t.Fatalf("read deployment-installability contract: %v", err)
	}
	contract := strings.Join(strings.Fields(string(contractBytes)), " ")

	qualifier := string(qualifierBytes)
	qualifyJob := workflowJobBlock(t, qualifier, "qualify")
	for _, required := range []string{
		`ref: ${{ github.sha }}`,
		`persist-credentials: false`,
		`EXPECTED_SOURCE_SHA: ${{ github.sha }}`,
		`test "$(git rev-parse HEAD)" = "${EXPECTED_SOURCE_SHA}"`,
		`--source-sha "${{ github.sha }}"`,
	} {
		if !strings.Contains(qualifyJob, required) {
			t.Fatalf("release container qualification does not fail closed on the exact caller commit: %s", required)
		}
	}
	for _, forbidden := range []string{
		"inputs.source_sha",
		"source_sha:",
	} {
		if strings.Contains(qualifier, forbidden) {
			t.Fatalf("release container qualification must not accept an arbitrary source ref: %s", forbidden)
		}
	}
	for _, required := range []string{
		"must not accept a caller-selected source revision",
		"checks out that commit without persisting credentials",
		"binds candidate-manifest verification to the same SHA",
	} {
		if !strings.Contains(contract, required) {
			t.Fatalf("deployment installability contract is missing the release source-trust boundary: %s", required)
		}
	}

	callerJobs := map[string]string{
		"build-release-candidate.yml": workflowJobBlock(t, string(candidateBytes), "qualify-release-containers"),
		"create-release.yml":          workflowJobBlock(t, string(releaseBytes), "qualify_release_containers"),
	}
	for caller, job := range callerJobs {
		if strings.Contains(job, "source_sha:") {
			t.Fatalf("%s must not forward a caller-selectable source ref to container qualification", caller)
		}
		if !strings.Contains(job, "uses: ./.github/workflows/qualify-release-containers.yml") {
			t.Fatalf("%s no longer calls the trusted container qualifier", caller)
		}
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

	chartBytes, err := os.ReadFile(repoFile("deploy", "helm", "pulse", "Chart.yaml"))
	if err != nil {
		t.Fatalf("read Helm Chart.yaml: %v", err)
	}
	chart := string(chartBytes)
	chartRequired := []string{
		"version: " + version,
		`appVersion: "` + version + `"`,
		"https://raw.githubusercontent.com/rcourtman/Pulse/v" + version + "/docs/images/pulse-logo.svg",
		"https://github.com/rcourtman/Pulse/blob/v" + version + "/docs/KUBERNETES.md",
	}
	for _, needle := range chartRequired {
		if !strings.Contains(chart, needle) {
			t.Fatalf("Helm Chart.yaml must pin the governed release version, missing %s:\n%s", needle, chart)
		}
	}
	if previous, ok := previousStablePatchVersion(version); ok && strings.Contains(chart, "v"+previous) {
		t.Fatalf("Helm Chart.yaml must not retain the previous stable patch tag v%s:\n%s", previous, chart)
	}
	if previous, ok := previousPrereleaseVersion(version); ok && strings.Contains(chart, "v"+previous) {
		t.Fatalf("Helm Chart.yaml must not retain the previous prerelease tag v%s:\n%s", previous, chart)
	}

	chartReadmeBytes, err := os.ReadFile(repoFile("deploy", "helm", "pulse", "README.md"))
	if err != nil {
		t.Fatalf("read Helm README.md: %v", err)
	}
	chartReadme := string(chartReadmeBytes)
	badgeVersion := shieldsBadgeMessage(version)
	chartReadmeRequired := []string{
		"![Version: " + version + "](https://img.shields.io/badge/Version-" + badgeVersion + "-informational?style=flat-square)",
		"![AppVersion: " + version + "](https://img.shields.io/badge/AppVersion-" + badgeVersion + "-informational?style=flat-square)",
	}
	for _, needle := range chartReadmeRequired {
		if !strings.Contains(chartReadme, needle) {
			t.Fatalf("Helm README.md must reflect the governed release version, missing %s:\n%s", needle, chartReadme)
		}
	}
	if previous, ok := previousStablePatchVersion(version); ok && strings.Contains(chartReadme, previous) {
		t.Fatalf("Helm README.md must not retain the previous stable patch version %s:\n%s", previous, chartReadme)
	}
	if previous, ok := previousPrereleaseVersion(version); ok && strings.Contains(chartReadme, previous) {
		t.Fatalf("Helm README.md must not retain the previous prerelease version %s:\n%s", previous, chartReadme)
	}

	helmPagesBytes, err := os.ReadFile(repoFile(".github", "workflows", "helm-pages.yml"))
	if err != nil {
		t.Fatalf("read helm-pages.yml: %v", err)
	}
	helmPages := string(helmPagesBytes)
	required := []string{
		`workflow_call:`,
		`chart_version:`,
		`source_release_run_id:`,
		`target_commitish:`,
		`Require activated GitHub release and source run`,
		`release-activation.json`,
		`.github/workflows/create-release.yml`,
		`gh run download "${SOURCE_RELEASE_RUN_ID}"`,
		`--name "pulse-chart-${VERSION}"`,
		`qualified chart metadata does not match the activated release`,
		`name: Publish chart release and merge Pages index`,
		`gh release create "${chart_release}" "${chart_path}"`,
		`--repo "${GITHUB_REPOSITORY}"`,
		`helm repo index "${index_work}"`,
		`git -C gh-pages push origin HEAD:gh-pages`,
		`grep -q "version: ${VERSION}"`,
		`helm show chart pulse-public/pulse --version "${VERSION}"`,
	}
	for _, needle := range required {
		if !strings.Contains(helmPages, needle) {
			t.Fatalf("helm-pages.yml missing immutable chart promotion step: %s", needle)
		}
	}
	for _, forbidden := range []string{
		"workflow_run:",
		`Smoke test with kind`,
		`Install helm-docs`,
		`helm package deploy/helm/pulse`,
		`git checkout -B "$REQUIRED_BRANCH"`,
		`git push origin HEAD:"$REQUIRED_BRANCH"`,
	} {
		if strings.Contains(helmPages, forbidden) {
			t.Fatalf("helm-pages.yml must be an awaited exact-tag staging job; found forbidden %q", forbidden)
		}
	}
}

func TestHelmChartDoesNotPublishRetiredExplorePrepassMonitoring(t *testing.T) {
	chartDir := repoFile("deploy", "helm", "pulse")
	err := filepath.WalkDir(chartDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".yaml", ".json", ".md":
		default:
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(content)
		for _, forbidden := range []string{
			"prometheusRule",
			"pulse_ai_explore",
			"Explore pre-pass",
			"explore_runs_total",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("helm chart file %s must not publish retired Assistant explore-prepass monitoring %q", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk helm chart: %v", err)
	}
}

func TestDeployDemoWorkflowIsRetiredToNonMutatingVerification(t *testing.T) {
	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "deploy-demo-server.yml"))
	if err != nil {
		t.Fatalf("read deploy-demo-server workflow: %v", err)
	}

	workflow := string(workflowBytes)
	required := []string{
		`name: Verify Demo Server`,
		`workflow_dispatch:`,
		`Verify Current Committed Stable Demo (No Mutation)`,
		`uses: ./.github/workflows/update-demo-server.yml`,
		`tag: latest`,
		`target: stable`,
		`verify_only: true`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("retired deploy-demo-server workflow missing non-mutating verification contract: %s", needle)
		}
	}
	for _, forbidden := range []string{`go build`, `scp `, `docker compose`, `verify_only: false`} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("retired deploy-demo-server workflow must not mutate the stable demo: %s", forbidden)
		}
	}
}

func TestUpdateDemoWorkflowUsesGovernedNetworkPath(t *testing.T) {
	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "update-demo-server.yml"))
	if err != nil {
		t.Fatalf("read update-demo-server workflow: %v", err)
	}

	profileBytes, err := os.ReadFile(repoFile(".github", "scripts", "resolve-demo-runtime-profile.sh"))
	if err != nil {
		t.Fatalf("read demo runtime profile resolver: %v", err)
	}
	workflow := string(workflowBytes) + "\n" + string(profileBytes)
	required := []string{
		`- name: Tailscale`,
		`uses: tailscale/github-action@306e68a486fd2350f2bfc3b19fcd143891a4a2d8 # v4`,
		`oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}`,
		`oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}`,
		`tags: tag:infra`,
		`version: '1.94.2'`,
		`ping: ${{ secrets.DEMO_SERVER_HOST }}`,
		`bash .github/scripts/check-demo-reachability.sh`,
		`workflow_call:`,
		`verify_only:`,
		`Waiting for activated release assets to be available`,
		`bash /tmp/pulse-install.sh --version "$TAG"`,
		`Refuse mutation during verification-only checks`,
		`uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0`,
		`go run ./scripts/release_update_key.go public-key-ssh`,
		`sed -i "s|^PINNED_RELEASE_SSH_PUBLIC_KEY=.*|PINNED_RELEASE_SSH_PUBLIC_KEY=\"${TRUSTED_SSH_PUBLIC_KEY}\"|" /tmp/pulse-install.sh`,
		`Verify target host identity`,
		`bash .github/scripts/setup-demo-ssh.sh`,
		`Demo environment points at host $REMOTE_HOSTNAME but expected $DEMO_EXPECTED_HOSTNAME.`,
		`Prepare demo host storage`,
		`KEEP_BACKUPS=2`,
		`Removing demo backup to restore install headroom: %s`,
		`Pruning demo volatile runtime stores to restore install headroom.`,
		`sudo find "$CONFIG_DIR" -xdev -type f`,
		`-name "metrics.db"`,
		`Removing demo volatile store: %s`,
		`Demo host does not have enough free space to back up $CONFIG_DIR before install.`,
		`Restore demo runtime configuration`,
		`Resolve target-compatible demo runtime profile`,
		`git grep -q 'mockEagerHistoryPVEGuestLimit'`,
		`git grep -q 'UpdateMetricCohort'`,
		`PROFILE="large-estate"`,
		`MOCK_NODES=50`,
		`MOCK_VMS_PER_NODE=10`,
		`MOCK_K8S_PODS=40`,
		`MOCK_SEED_DURATION=48h`,
		`MOCK_UPDATE_INTERVAL=2s`,
		`PROFILE="legacy-bounded"`,
		`MOCK_NODES=8`,
		`MOCK_VMS_PER_NODE=6`,
		`MOCK_LXCS_PER_NODE=4`,
		`MOCK_DOCKER_HOSTS=2`,
		`MOCK_DOCKER_CONTAINERS=8`,
		`MOCK_GENERIC_HOSTS=2`,
		`MOCK_K8S_CLUSTERS=1`,
		`MOCK_K8S_NODES=3`,
		`MOCK_K8S_PODS=12`,
		`MOCK_K8S_DEPLOYMENTS=4`,
		`MOCK_SEED_DURATION=2h`,
		`MOCK_UPDATE_INTERVAL=15s`,
		`resolve_config_dir`,
		`set_env_value DEMO_MODE true`,
		`set_env_value PULSE_MOCK_MODE true`,
		`set_env_value PULSE_MOCK_NODES "$MOCK_NODES"`,
		`set_env_value PULSE_MOCK_VMS_PER_NODE "$MOCK_VMS_PER_NODE"`,
		`set_env_value PULSE_MOCK_LXCS_PER_NODE "$MOCK_LXCS_PER_NODE"`,
		`set_env_value PULSE_MOCK_DOCKER_HOSTS "$MOCK_DOCKER_HOSTS"`,
		`set_env_value PULSE_MOCK_K8S_PODS "$MOCK_K8S_PODS"`,
		`set_env_value PULSE_MOCK_TRENDS_SEED_DURATION "$MOCK_SEED_DURATION"`,
		`set_env_value PULSE_MOCK_UPDATE_INTERVAL "$MOCK_UPDATE_INTERVAL"`,
		`ensure_demo_fixture_entitlement`,
		`"demo_fixtures"`,
		`del(.integrity)`,
		`Demo fixture entitlement ensured in governed demo billing state.`,
		`/api/license/runtime-capabilities`,
		`Mock mode enabled`,
		`Demo server mock mode did not enable after entitlement sync`,
		`Require exact committed activation marker for mutation`,
		`activation_convergence_run_id:`,
		`release-activation.json`,
		`.convergence_run_id == $convergence_run_id`,
		`gh release view "${TAG}" --repo "${GITHUB_REPOSITORY}"`,
		`Stable demo mutation refuses inactive or prerelease tag`,
		`Verify public browser smoke`,
		`PULSE_DEMO_AUTH_USER`,
		`PULSE_DEMO_AUTH_PASS`,
		`./scripts/run_demo_public_browser_smoke.sh`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("update-demo-server workflow missing governed network path: %s", needle)
		}
	}
	for _, forbidden := range []string{
		`release_id:`,
		`RELEASE_ID: ${{ inputs.release_id }}`,
		`--archive "/tmp/${tarball}"`,
		`unpublished draft`,
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("update-demo-server.yml must not deploy draft release assets; found %q", forbidden)
		}
	}
}

func TestDemoSshSetupHelperHandlesIpLiteralTargets(t *testing.T) {
	helperBytes, err := os.ReadFile(repoFile(".github", "scripts", "setup-demo-ssh.sh"))
	if err != nil {
		t.Fatalf("read demo ssh setup helper: %v", err)
	}
	helper := string(helperBytes)
	required := []string{
		`is_ip_literal()`,
		`ipaddress.ip_address(sys.argv[1])`,
		`host_needs_dns=false`,
		`Demo SSH host is an IP literal; skipping DNS resolution wait.`,
		`[ "$host_needs_dns" = "true" ] && ! getent hosts "$DEMO_SERVER_HOST"`,
		`ssh-keyscan -T 10 -H "$DEMO_SERVER_HOST"`,
		`MAX_SSH_SETUP_ATTEMPTS="${DEMO_SSH_SETUP_ATTEMPTS:-3}"`,
		`Demo network preflight passed, but ssh-keyscan did not return host keys.`,
	}
	for _, needle := range required {
		if !strings.Contains(helper, needle) {
			t.Fatalf("demo ssh setup helper missing guarded IP/hostname behavior: %s", needle)
		}
	}

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	getentMarker := filepath.Join(tmpDir, "getent-called")
	if err := os.WriteFile(filepath.Join(fakeBin, "getent"), []byte("#!/bin/sh\n: > \"$GETENT_MARKER\"\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake getent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "ssh-keyscan"), []byte("#!/bin/sh\nprintf '100.109.163.95 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDemo\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake ssh-keyscan: %v", err)
	}

	homeDir := filepath.Join(tmpDir, "home")
	cmd := exec.Command("bash", repoFile(".github", "scripts", "setup-demo-ssh.sh"))
	cmd.Env = append(os.Environ(),
		"DEMO_SERVER_HOST=100.109.163.95",
		"DEMO_SERVER_SSH_KEY=fake-private-key",
		"GETENT_MARKER="+getentMarker,
		"HOME="+homeDir,
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demo ssh setup helper failed for IP literal: %v\n%s", err, output)
	}
	if _, err := os.Stat(getentMarker); !os.IsNotExist(err) {
		t.Fatalf("demo ssh setup helper must not require getent hosts for IP literals; stat err=%v", err)
	}
	knownHosts, err := os.ReadFile(filepath.Join(homeDir, ".ssh", "known_hosts"))
	if err != nil {
		t.Fatalf("read generated known_hosts: %v", err)
	}
	if !strings.Contains(string(knownHosts), "ssh-ed25519") {
		t.Fatalf("known_hosts missing captured key: %s", knownHosts)
	}
	if !strings.Contains(string(output), "Demo SSH host is an IP literal; skipping DNS resolution wait.") {
		t.Fatalf("helper output did not report IP literal path: %s", output)
	}
}

func TestDemoReachabilityHelperSeparatesTailnetAndSshTransportProof(t *testing.T) {
	helperBytes, err := os.ReadFile(repoFile(".github", "scripts", "check-demo-reachability.sh"))
	if err != nil {
		t.Fatalf("read demo reachability helper: %v", err)
	}
	helper := string(helperBytes)
	for _, needle := range []string{
		`tailscale status --json`,
		`tailscale ping --c 3 --timeout 10s "$DEMO_SERVER_HOST"`,
		`nc -z -w 5 "$DEMO_SERVER_HOST" "$TCP_PORT"`,
		`Runner Tailscale DNS:`,
		`Runner Tailscale tags:`,
		`Demo peer is not present in the runner peer map yet.`,
		`Verify sshd and the host firewall on tailscale0.`,
	} {
		if !strings.Contains(helper, needle) {
			t.Fatalf("demo reachability helper missing diagnostic contract: %s", needle)
		}
	}

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	tailscaleScript := `#!/bin/sh
if [ "$1" = "status" ]; then
  printf '%s\n' '{"BackendState":"Running","Self":{"TailscaleIPs":["100.100.100.1"]},"Peer":{"demo":{"TailscaleIPs":["100.109.163.95"],"Online":true,"Active":true,"Relay":"lhr"}}}'
  exit 0
fi
if [ "$1" = "ping" ]; then
  echo 'pong from demo'
  exit 0
fi
exit 1
`
	if err := os.WriteFile(filepath.Join(fakeBin, "tailscale"), []byte(tailscaleScript), 0o755); err != nil {
		t.Fatalf("write fake tailscale: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "nc"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake nc: %v", err)
	}

	cmd := exec.Command("bash", repoFile(".github", "scripts", "check-demo-reachability.sh"))
	cmd.Env = append(os.Environ(),
		"DEMO_SERVER_HOST=100.109.163.95",
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demo reachability helper failed: %v\n%s", err, output)
	}
	for _, needle := range []string{"Tailscale backend: Running", "Demo peer state: online=True active=True relay=lhr", "Demo SSH transport is reachable over Tailscale."} {
		if !strings.Contains(string(output), needle) {
			t.Fatalf("demo reachability output missing %q: %s", needle, output)
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

func TestDockerfileStampsTelemetryDeploymentMethod(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	if !strings.Contains(string(dockerfileBytes), `ENV PULSE_DEPLOYMENT_METHOD=container_other`) {
		t.Fatal("Dockerfile must stamp the closed fallback deployment method for container images")
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
	compileContent, err := os.ReadFile(repoFile("scripts", "build-release-binaries.sh"))
	if err != nil {
		t.Fatalf("read build-release-binaries.sh: %v", err)
	}
	compileScript := string(compileContent)

	required := []string{
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
		`[ -f scripts/install-mcp.ps1 ] && cp scripts/install-mcp.ps1 "$RELEASE_DIR/install-mcp.ps1"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing pulse-mcp distribution wiring: %s", needle)
		}
	}
	for _, needle := range []string{
		`package=./cmd/pulse-mcp`,
		`pulse_release_binary_filename "${component}" "${target}"`,
	} {
		if !strings.Contains(compileScript, needle) {
			t.Fatalf("build-release-binaries.sh missing pulse-mcp compilation wiring: %s", needle)
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

	mcpPowerShell, err := os.ReadFile(repoFile("scripts", "install-mcp.ps1"))
	if err != nil {
		t.Fatalf("read install-mcp.ps1: %v", err)
	}
	for _, needle := range []string{
		`function Resolve-Architecture`,
		`PULSE_MCP_NO_VERIFY`,
		`checksums.txt`,
		`Get-FileHash -Path $tmp -Algorithm SHA256`,
		`sha256 mismatch`,
	} {
		if !strings.Contains(string(mcpPowerShell), needle) {
			t.Fatalf("install-mcp.ps1 missing required helper or guard: %s", needle)
		}
	}
}

// The release-pipeline downstream workflows and private Pro publication path
// share one customer boundary. Exact-version artifacts are staged behind a
// draft, verified, and only then activated; GitHub publication is the final
// notification rather than the trigger for a long tail of publication work.
// The tests below pin that barrier so the staggered-release regression class
// cannot return.

func TestInstallShSmokeWorkflowPresent(t *testing.T) {
	workflowPath := repoFile(".github", "workflows", "install-sh-smoke.yml")
	assertFileContainsAll(t, workflowPath,
		// Inputs and triggers.
		`name: install.sh Smoke (Release Assets)`,
		`workflow_call:`,
		`workflow_dispatch:`,
		`asset_source:`,
		`release_id:`,
		// Staged cuts use authenticated draft assets; manual verification can
		// still pull from the public release URL.
		`repos/${REPO}/releases/${RELEASE_ID}/assets?per_page=100`,
		`repos/${REPO}/releases/assets/${asset_id}`,
		`Accept: application/octet-stream`,
		`releases/download/${TAG}`,
		`install.sh.sshsig`,
		`pulse-${TAG}-linux-amd64.tar.gz`,
		// README key extraction + ssh-keygen verify against the asset.
		`grep -oE 'ssh-ed25519 [A-Za-z0-9+/=]+ pulse-installer' README.md`,
		`ssh-keygen -Y verify \`,
		`-I pulse-installer \`,
		`-n pulse-install \`,
		`-s install.sh.sshsig < install.sh`,
		// Server-installer identity assertions, mirroring validate-release.sh.
		`grep -qE '^# Pulse Installer Script' install.sh`,
		`grep -q 'Pulse Unified Agent Installer' install.sh`,
		`grep -qE '^[[:space:]]*--version\)' install.sh`,
		// End-to-end install in a privileged systemd container.
		`jrei/systemd-debian:12`,
		`bash install.sh --archive /smoke/${tarball} --disable-auto-updates`,
		`systemctl is-active pulse`,
		// curl --retry handles its own poll loop instead of a bash for-loop.
		`--retry 30 --retry-delay 2 --retry-connrefused --retry-all-errors http://127.0.0.1:7655/api/health`,
		// Authoritative version check via /api/version (not /api/health).
		`curl -fsS http://127.0.0.1:7655/api/version`,
		`Installed version mismatch. Expected`,
	)

	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read install-sh-smoke workflow: %v", err)
	}
	smokeJob := workflowJobBlock(t, string(workflowBytes), "smoke")
	if !strings.Contains(smokeJob, "contents: write") {
		t.Fatal("install-sh-smoke.yml smoke job must grant contents: write to read unpublished draft release assets")
	}
}

func TestPromoteFloatingTagsReachableViaWorkflowCall(t *testing.T) {
	workflowPath := repoFile(".github", "workflows", "promote-floating-tags.yml")
	assertFileContainsAll(t, workflowPath,
		`workflow_call:`,
		`tag:`,
		`description: "Release tag (e.g., v6.0.0). Required for workflow_call."`,
		`prerelease:`,
		`type: boolean`,
		`TAG="${INPUT_TAG}"`,
		`Require activated GitHub release`,
		`gh release view "${TAG}" --json isDraft,publishedAt,tagName`,
		`Floating-tag promotion refuses inactive release ${TAG}.`,
		`for image in pulse pulse-control-plane; do`,
		`"rcourtman/${image}:rc"`,
		`"ghcr.io/${OWNER}/${image}:latest"`,
	)
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read promote-floating-tags.yml: %v", err)
	}
	if strings.Contains(string(content), "workflow_run:") {
		t.Fatal("floating aliases must have one explicit activation owner, not an implicit workflow_run trigger")
	}
	publishBytes, err := os.ReadFile(repoFile(".github", "workflows", "publish-docker.yml"))
	if err != nil {
		t.Fatalf("read publish-docker.yml: %v", err)
	}
	publishWorkflow := string(publishBytes)
	for _, mutableTag := range []string{
		`rcourtman/pulse:latest`,
		`ghcr.io/{0}/pulse:latest`,
		`rcourtman/pulse-control-plane:latest`,
		`ghcr.io/{0}/pulse-control-plane:latest`,
	} {
		if strings.Contains(publishWorkflow, mutableTag) {
			t.Fatalf("publish-docker.yml must stage exact-version images without moving mutable alias %q", mutableTag)
		}
	}
}

func TestPublishHelmChartReachableViaWorkflowCall(t *testing.T) {
	workflowPath := repoFile(".github", "workflows", "publish-helm-chart.yml")
	assertFileContainsAll(t, workflowPath,
		`workflow_call:`,
		`chart_version:`,
		`description: "Chart version (e.g., 6.0.0-rc.5). Required for workflow_call."`,
		`required: true`,
		`type: string`,
		`app_version:`,
		// Chart-version resolver prefers inputs over release-event tag.
		`if [ -n "${INPUT_CHART_VERSION}" ]; then`,
		`RELEASE_TAG="${RELEASE_TAG_NAME}"`,
		`name: Verify public GHCR chart read`,
		`helm registry logout ghcr.io || true`,
		`helm show chart`,
		`oci://ghcr.io/${{ github.repository_owner }}/pulse-chart/pulse`,
		`--version "${{ steps.versions.outputs.chart_version }}"`,
	)

	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read publish-helm-chart.yml: %v", err)
	}
	workflow := string(content)
	for _, forbidden := range []string{
		`versions/latest/restore`,
		`-f visibility=public`,
		`Package visibility configuration attempted`,
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("publish-helm-chart.yml must verify chart readability instead of masking GHCR visibility API failures; found %q", forbidden)
		}
	}
}

func TestReleasePipelinePromotesOneImmutableCandidate(t *testing.T) {
	createBytes, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	candidateBytes, err := os.ReadFile(repoFile(".github", "workflows", "build-release-candidate.yml"))
	if err != nil {
		t.Fatalf("read build-release-candidate.yml: %v", err)
	}
	compilerBytes, err := os.ReadFile(repoFile(".github", "workflows", "compile-release-payload.yml"))
	if err != nil {
		t.Fatalf("read compile-release-payload.yml: %v", err)
	}
	validationBytes, err := os.ReadFile(repoFile(".github", "workflows", "validate-release-assets.yml"))
	if err != nil {
		t.Fatalf("read validate-release-assets.yml: %v", err)
	}
	convergenceBytes, err := os.ReadFile(repoFile(".github", "workflows", "release-convergence.yml"))
	if err != nil {
		t.Fatalf("read release-convergence.yml: %v", err)
	}
	recoveryBytes, err := os.ReadFile(repoFile(".github", "workflows", "recover-release-activation.yml"))
	if err != nil {
		t.Fatalf("read recover-release-activation.yml: %v", err)
	}
	leaseScriptBytes, err := os.ReadFile(repoFile("scripts", "release_control", "customer_promotion_lease.sh"))
	if err != nil {
		t.Fatalf("read customer_promotion_lease.sh: %v", err)
	}

	createWorkflow := string(createBytes)
	candidateWorkflow := string(candidateBytes)
	compilerWorkflow := string(compilerBytes)
	compileScriptBytes, err := os.ReadFile(repoFile("scripts", "build-release-binaries.sh"))
	if err != nil {
		t.Fatalf("read build-release-binaries.sh: %v", err)
	}
	compileScript := string(compileScriptBytes)
	validationWorkflow := string(validationBytes)
	convergenceWorkflow := string(convergenceBytes)
	recoveryWorkflow := string(recoveryBytes)
	leaseScript := string(leaseScriptBytes)
	createJob := workflowJobBlock(t, createWorkflow, "create_release")
	prepareJob := workflowJobBlock(t, createWorkflow, "prepare")
	frontendBundleJob := workflowJobBlock(t, createWorkflow, "frontend_bundle")
	backendJob := workflowJobBlock(t, createWorkflow, "backend_tests")
	integrationJob := workflowJobBlock(t, createWorkflow, "integration_tests")
	validationJob := workflowJobBlock(t, createWorkflow, "validate_release_assets")
	privateStageJob := workflowJobBlock(t, createWorkflow, "stage_private_pro_runtime")
	readinessJob := workflowJobBlock(t, createWorkflow, "release_readiness")
	dispatchJob := workflowJobBlock(t, createWorkflow, "dispatch_release_convergence")
	activationJob := workflowJobBlock(t, createWorkflow, "activate_release")
	leaseJob := workflowJobBlock(t, convergenceWorkflow, "acquire_customer_promotion_lease")
	privatePromotionJob := workflowJobBlock(t, convergenceWorkflow, "promote_private_pro_runtime")
	floatingJob := workflowJobBlock(t, convergenceWorkflow, "promote_floating_tags")
	helmPagesJob := workflowJobBlock(t, convergenceWorkflow, "publish_helm_pages")
	demoJob := workflowJobBlock(t, convergenceWorkflow, "update_stable_demo")
	compileJob := workflowJobBlock(t, compilerWorkflow, "compile-release-payload")
	obtainPayloadJob := workflowJobBlock(t, candidateWorkflow, "obtain-release-payload")
	candidateBuildJob := workflowJobBlock(t, candidateWorkflow, "build")

	for _, needle := range []string{
		`!contains(inputs.version, '-') && 'ubuntu-24.04'`,
		`'ubuntu-24.04'`,
		`pulse-pve-compile`,
	} {
		if !strings.Contains(prepareJob, needle) {
			t.Fatalf("release preparation missing stable-hosted runner selection: %s", needle)
		}
	}
	if strings.Contains(prepareJob, "sparse-checkout") {
		t.Fatal("release preparation must leave a complete worktree for the following compile job")
	}

	for _, needle := range []string{
		`fromJSON('["self-hosted","Linux","X64","pulse-pve-compile"]')`,
		`GITHUB_WORKFLOW_SHA`,
		`ref: ${{ inputs.source_sha }}`,
		`PULSE_RELEASE_BUILD_JOBS: "2"`,
		`./scripts/build-release-binaries.sh "${{ inputs.version }}"`,
		`release-compiled-${{ inputs.source_sha }}-${{ inputs.version }}-${{ inputs.request_id }}`,
	} {
		if !strings.Contains(compileJob, needle) {
			t.Fatalf("compiled release payload job missing exact-SHA contract: %s", needle)
		}
	}
	if strings.Contains(compileJob, "PULSE_UPDATE_SIGNING_KEY") {
		t.Fatal("release compilation job must not receive private update-signing material")
	}
	if strings.Contains(candidateWorkflow, `runs-on: ${{ fromJSON('["self-hosted"`) {
		t.Fatal("SignPath release workflow must not contain a self-hosted runner job")
	}
	for _, needle := range []string{
		`runs-on: ubuntu-24.04`,
		`actions: write`,
		`actions/workflows/compile-release-payload.yml/dispatches`,
		`X-GitHub-Api-Version: 2026-03-10`,
		`compiler_run_id: ${{ steps.dispatch.outputs.compiler_run_id }}`,
		`.path == ".github/workflows/compile-release-payload.yml"`,
	} {
		if !strings.Contains(obtainPayloadJob, needle) {
			t.Fatalf("hosted compiler handoff job missing isolated-workflow contract: %s", needle)
		}
	}
	for label, job := range map[string]string{
		"frontend bundle": frontendBundleJob,
		"backend tests":   backendJob,
	} {
		if !strings.Contains(job, `!contains(inputs.version, '-') && 'ubuntu-24.04'`) {
			t.Fatalf("%s must stay GitHub-hosted for every stable release", label)
		}
		if !strings.Contains(job, "pulse-pve-") {
			t.Fatalf("%s must retain PVE acceleration for prereleases", label)
		}
		if strings.Contains(job, "require_windows_signing") || strings.Contains(job, "unsigned_windows_exception") {
			t.Fatalf("%s runner selection must not depend on the Windows-signing decision", label)
		}
	}
	if !strings.Contains(compileJob, "cache: false") || strings.Contains(compileJob, "cache: 'npm'") {
		t.Fatal("release compilation must avoid Actions cache archival on both hosted and PVE runners")
	}
	if strings.Contains(frontendBundleJob, "cache: 'npm'") {
		t.Fatal("PVE frontend bundle must use its persistent runner-local npm cache")
	}
	if !strings.Contains(backendJob, "cache: false") {
		t.Fatal("PVE backend qualification must not archive its persistent Go caches through Actions")
	}
	for _, needle := range []string{
		`scripts/release_candidate_manifest.py create`,
		`--source-sha "${SOURCE_SHA}"`,
		`--release-dir "${PAYLOAD_DIR}"`,
	} {
		if !strings.Contains(compileScript, needle) {
			t.Fatalf("compiled release payload script missing manifest binding: %s", needle)
		}
	}
	for _, needle := range []string{
		`needs.obtain-release-payload.result == 'success'`,
		`actions: read`,
		`EXPECTED_ARTIFACT_ID: ${{ needs.obtain-release-payload.outputs.artifact_id }}`,
		`EXPECTED_ARTIFACT_DIGEST: ${{ needs.obtain-release-payload.outputs.artifact_digest }}`,
		`EXPECTED_COMPILER_RUN_ID: ${{ needs.obtain-release-payload.outputs.compiler_run_id }}`,
		`actions/artifacts/${EXPECTED_ARTIFACT_ID}`,
		`.workflow_run.head_sha == $source_sha`,
		`sha256sum --check --`,
		`scripts/release_candidate_manifest.py verify-local`,
		`compiled-payload-verification.json`,
		`separate-trusted-self-hosted-compiler-workflow`,
		`PULSE_RELEASE_COMPILED_PAYLOAD_DIR`,
	} {
		if !strings.Contains(candidateBuildJob, needle) {
			t.Fatalf("hosted candidate build missing compiled-payload verification: %s", needle)
		}
	}

	for _, needle := range []string{
		`./scripts/build-release.sh "${{ inputs.version }}"`,
		`scripts/validate-release.sh "${{ inputs.version }}" --skip-docker`,
		`scripts/release_candidate_manifest.py create`,
		`compression-level: 0`,
		`retention-days: 1`,
	} {
		if !strings.Contains(candidateWorkflow, needle) {
			t.Fatalf("build-release-candidate.yml missing single-build contract: %s", needle)
		}
	}
	publishDockerJob := workflowJobBlock(t, createWorkflow, "publish_docker")
	for _, needle := range []string{
		"- build_release_candidate",
		`source_sha: ${{ github.sha }}`,
	} {
		if !strings.Contains(publishDockerJob, needle) {
			t.Fatalf("inert exact-version Docker staging missing acceleration contract: %s", needle)
		}
	}
	for _, forbiddenDependency := range []string{"- qualify_release_containers", "- create_release"} {
		if strings.Contains(publishDockerJob, forbiddenDependency) {
			t.Fatalf("inert exact-version Docker staging must overlap independent qualification: %s", forbiddenDependency)
		}
	}

	for _, needle := range []string{
		`Download immutable release candidate`,
		`scripts/release_candidate_manifest.py verify-local`,
		`needs.build_release_candidate.outputs.artifact_name`,
	} {
		if !strings.Contains(createJob, needle) {
			t.Fatalf("create_release missing candidate promotion contract: %s", needle)
		}
	}
	if strings.Contains(createJob, "scripts/build-release.sh") {
		t.Fatal("create_release must promote the verified candidate instead of rebuilding release assets")
	}

	if !strings.Contains(backendJob, "- frontend_bundle") || !strings.Contains(integrationJob, "- frontend_bundle") {
		t.Fatal("backend and integration jobs must consume the shared verified frontend bundle")
	}
	if strings.Contains(integrationJob, "- backend_tests") {
		t.Fatal("integration tests must run in parallel with backend tests")
	}
	if !strings.Contains(integrationJob, `tests/66-organization-sharing-approval-ui.spec.ts`) {
		t.Fatal("integration release gate missing current organization-sharing coverage")
	}
	if strings.Contains(integrationJob, `tests/03-multi-tenant.spec.ts`) {
		t.Fatal("integration release gate must not target the quarantined multi-tenant spec")
	}
	if strings.Contains(validationJob, "- publish_docker") {
		t.Fatal("release asset digest validation must run in parallel with Docker publication")
	}
	if !strings.Contains(privateStageJob, "- prepare") ||
		strings.Contains(privateStageJob, "- create_release") ||
		strings.Contains(privateStageJob, "- validate_release_assets") {
		t.Fatal("inert private Pro staging must start after preparation without waiting for public qualification or draft creation")
	}
	for _, needle := range []string{
		`--arg pulse_checkout_ref "${GITHUB_SHA}"`,
		`pulse_checkout_ref: $pulse_checkout_ref`,
		`allow_pre_activation_staging: "true"`,
	} {
		if !strings.Contains(privateStageJob, needle) {
			t.Fatalf("private Pro pre-activation staging missing exact-SHA contract: %s", needle)
		}
	}
	for _, dependency := range []string{
		"- create_release",
		"- publish_docker",
		"- validate_release_assets",
		"- install_sh_smoke",
		"- publish_helm_chart",
		"- stage_private_pro_runtime",
	} {
		if !strings.Contains(readinessJob, dependency) {
			t.Fatalf("immutable release readiness missing dependency: %s", dependency)
		}
	}
	for _, forbiddenDependency := range []string{
		"- publish_helm_pages",
		"- promote_floating_tags",
		"- promote_private_pro_runtime",
		"- update_stable_demo",
	} {
		if strings.Contains(readinessJob, forbiddenDependency) {
			t.Fatalf("immutable readiness must exclude mutable customer state: %s", forbiddenDependency)
		}
	}
	for _, dependency := range []string{"- create_release", "- stage_private_pro_runtime"} {
		if !strings.Contains(dispatchJob, dependency) {
			t.Fatalf("durable convergence dispatch missing staged dependency: %s", dependency)
		}
	}
	if strings.Contains(dispatchJob, "- release_readiness") {
		t.Fatal("durable convergence dispatch must prewarm before the readiness join")
	}
	if !strings.Contains(dispatchJob, "github.event.inputs.draft_only != 'true'") ||
		!strings.Contains(dispatchJob, "historical_asset_backfill_only != 'true'") {
		t.Fatal("early convergence dispatch must remain disabled for inert release modes")
	}
	if !strings.Contains(convergenceWorkflow, `gh run view "${EXPECTED_SOURCE_RUN_ID}"`) ||
		!strings.Contains(convergenceWorkflow, "completed without the exact activation marker") ||
		!strings.Contains(convergenceWorkflow, `select(.name == "release-activation.json") | .state`) ||
		!strings.Contains(convergenceWorkflow, "uploaded but not publicly readable yet") {
		t.Fatal("prewarmed convergence must terminate when its source run ends without activation")
	}
	if !strings.Contains(activationJob, "- dispatch_release_convergence") {
		t.Fatal("release activation must depend on the exact durable convergence dispatch")
	}
	for _, forbiddenDependency := range []string{
		"- update_stable_demo",
		"- promote_floating_tags",
		"- promote_private_pro_runtime",
	} {
		if strings.Contains(activationJob, forbiddenDependency) {
			t.Fatalf("release activation must precede mutable customer state: %s", forbiddenDependency)
		}
	}
	if !strings.Contains(activationJob, `'{draft: false, make_latest: $make_latest}'`) {
		t.Fatal("release activation must be the job that crosses the draft publication boundary")
	}
	for _, needle := range []string{
		`release-activation.json`,
		`require_viable_convergence_owner`,
		`continue-on-error: true`,
		`Resuming quarantined activation for ${TAG}`,
		`[ "$activation_committed" = "true" ] ||`,
		`--repo "${GITHUB_REPOSITORY}"`,
	} {
		if !strings.Contains(activationJob, needle) {
			t.Fatalf("release activation missing irreversible handoff contract: %s", needle)
		}
	}
	if strings.Contains(activationJob, `[ -n "$published_at" ] ||`) {
		t.Fatal("release activation must allow retrying a current draft when GitHub retains historical published_at metadata")
	}
	if !strings.Contains(createJob, `Resuming quarantined draft for ${TAG}`) ||
		!strings.Contains(createJob, `Resuming quarantined draft release for ${TAG}`) {
		t.Fatal("release creation must explicitly support resuming a quarantined draft")
	}
	if !strings.Contains(createJob, `release_activation_committed=${RELEASE_ACTIVATION_COMMITTED}`) ||
		!strings.Contains(createJob, `[ "$EXISTING_RELEASE_ACTIVATION_COMMITTED" != "true" ]`) ||
		!strings.Contains(createJob, `[ "$ACTIVATION_COMMITTED" != "true" ]`) {
		t.Fatal("release creation must refuse to retarget a draft whose irreversible activation marker exists")
	}
	if strings.Contains(createJob, `[ -z "$EXISTING_RELEASE_PUBLISHED_AT" ]`) ||
		strings.Contains(createJob, `[ -z "$PUBLISHED_AT" ]`) {
		t.Fatal("release creation must not mistake historical published_at metadata for current public visibility")
	}
	for _, needle := range []string{
		`.path == ".github/workflows/create-release.yml"`,
		`release-candidate-manifest-${source_sha}-${version}`,
		`scripts/release_candidate_manifest.py verify-release`,
		`failure outside the recoverable activation boundary`,
		`release-convergence.yml/dispatches`,
		`activation_recovery_run_id`,
		`for attempt in $(seq 1 12)`,
		`waiting for GitHub indexing`,
		`--repo "${GITHUB_REPOSITORY}"`,
	} {
		if !strings.Contains(recoveryWorkflow, needle) {
			t.Fatalf("activation-only recovery missing qualified reuse contract: %s", needle)
		}
	}
	if strings.Contains(recoveryWorkflow, `scripts/build-release.sh`) ||
		strings.Contains(recoveryWorkflow, `build-release-candidate.yml`) {
		t.Fatal("activation-only recovery must never rebuild the qualified candidate")
	}
	for _, needle := range []string{`sort -Vr`, `superseded=true`} {
		if !strings.Contains(leaseJob, needle) {
			t.Fatalf("global customer-promotion lease missing monotonicity contract: %s", needle)
		}
	}
	for _, needle := range []string{`refs/heads/release-customer-promotion-lock`, `--force-with-lease="${LOCK_REF}:${observed_sha}"`} {
		if !strings.Contains(leaseScript, needle) {
			t.Fatalf("shared global customer-promotion lease helper missing contract: %s", needle)
		}
	}
	for jobName, jobBlock := range map[string]string{
		"floating-tag promotion":     floatingJob,
		"private Pro live promotion": privatePromotionJob,
		"stable demo deployment":     demoJob,
	} {
		if !strings.Contains(jobBlock, `needs: acquire_customer_promotion_lease`) ||
			!strings.Contains(jobBlock, `needs.acquire_customer_promotion_lease.outputs.superseded != 'true'`) {
			t.Fatalf("%s must run under the monotonic global promotion lease", jobName)
		}
	}
	if !strings.Contains(helmPagesJob, `needs: acquire_customer_promotion_lease`) ||
		strings.Contains(helmPagesJob, `superseded != 'true'`) {
		t.Fatal("additive Helm Pages indexing must run under the lease for every committed release")
	}
	if strings.Contains(demoJob, "release_id:") {
		t.Fatal("stable demo must use activated public assets, not an unpublished release id")
	}
	for _, needle := range []string{
		`inputs.candidate_manifest_artifact != ''`,
		`scripts/release_candidate_manifest.py verify-release`,
		`inputs.candidate_manifest_artifact == ''`,
	} {
		if !strings.Contains(validationWorkflow, needle) {
			t.Fatalf("validate-release-assets.yml missing fast digest contract: %s", needle)
		}
	}
}

func TestFrontendDependencySecurityAuditsAreRequired(t *testing.T) {
	workflowPath := repoFile(".github", "workflows", "build-and-test.yml")
	assertFileContainsAll(t, workflowPath,
		`- name: Audit complete frontend dependency graph`,
		`run: npm audit`,
		`- name: Audit production frontend dependencies`,
		`run: npm audit --omit=dev`,
	)
}

func TestReleaseCutGatesCriticalFrontendAndWindowsRuntimeProof(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}

	workflow := string(content)
	frontendJob := workflowJobBlock(t, workflow, "frontend_checks")
	windowsJob := workflowJobBlock(t, workflow, "windows_install_command_smoke")
	smokeJob := workflowJobBlock(t, workflow, "release_smoke")
	createJob := workflowJobBlock(t, workflow, "create_release")
	readinessJob := workflowJobBlock(t, workflow, "release_readiness")
	verdictJob := workflowJobBlock(t, workflow, "release_commit_verdict")

	for _, needle := range []string{
		`npm --prefix frontend-modern run type-check`,
		`npm --prefix frontend-modern test`,
	} {
		if !strings.Contains(frontendJob, needle) {
			t.Fatalf("frontend release gate missing %s", needle)
		}
	}
	for _, needle := range []string{
		`runs-on: windows-2025`,
		`agentInstallCommand.windows.test.ts`,
	} {
		if !strings.Contains(windowsJob, needle) {
			t.Fatalf("Windows install-command release gate missing %s", needle)
		}
	}
	for _, needle := range []string{
		`tests/95-release-smoke.spec.ts`,
		`release-smoke-failures-${{ github.sha }}`,
		`tests/integration/test-results/`,
	} {
		if !strings.Contains(smokeJob, needle) {
			t.Fatalf("release render smoke missing %s", needle)
		}
	}
	if !strings.Contains(readinessJob, `needs.windows_install_command_smoke.result == 'success'`) {
		t.Fatal("release readiness must fail closed on the Windows install-command smoke")
	}
	if strings.Contains(createJob, `needs.windows_install_command_smoke.result`) {
		t.Fatal("inert draft staging must overlap independent Windows qualification")
	}
	for _, result := range []string{
		"needs.qualify_release_containers.result == 'success'",
		"needs.frontend_checks.result == 'success'",
		"needs.backend_tests.result == 'success'",
		"needs.release_smoke.result == 'success'",
	} {
		if !strings.Contains(readinessJob, result) {
			t.Fatalf("release readiness missing deferred qualification join: %s", result)
		}
		if strings.Contains(createJob, result) {
			t.Fatalf("inert draft staging must not serialize on deferred qualification: %s", result)
		}
	}
	if !strings.Contains(workflow, "qualify_containers: false") ||
		!strings.Contains(workflow, "uses: ./.github/workflows/qualify-release-containers.yml") {
		t.Fatal("publishing release must qualify the candidate beside inert draft staging")
	}
	if !strings.Contains(verdictJob, `require_result "Windows install command smoke" "$WINDOWS_INSTALL_COMMAND_RESULT" success`) {
		t.Fatal("release activation commit verdict must report the Windows install-command smoke")
	}
}

func TestCreateReleasePublishesPrivateProRuntime(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	workflow := string(content)
	stageJob := workflowJobBlock(t, workflow, "stage_private_pro_runtime")
	promotionContent, err := os.ReadFile(repoFile(".github", "workflows", "promote-private-pro-runtime.yml"))
	if err != nil {
		t.Fatalf("read promote-private-pro-runtime.yml: %v", err)
	}
	convergenceContent, err := os.ReadFile(repoFile(".github", "workflows", "release-convergence.yml"))
	if err != nil {
		t.Fatalf("read release-convergence.yml: %v", err)
	}
	promotionJob := workflowJobBlock(t, string(promotionContent), "promote")
	convergencePromotionJob := workflowJobBlock(t, string(convergenceContent), "promote_private_pro_runtime")

	for _, needle := range []string{
		`needs.prepare.result == 'success'`,
		`github.event.inputs.draft_only != 'true'`,
		`startsWith(needs.prepare.outputs.version, '6.')`,
		`GH_TOKEN: ${{ secrets.WORKFLOW_PAT }}`,
		`--json createdAt`,
		`r2_prefix="${TAG}-pro-${run_created_date}-${GITHUB_RUN_ID}"`,
		`return_run_details: true`,
		`pulse_ref: $pulse_ref`,
		`pulse_checkout_ref: $pulse_checkout_ref`,
		`version: $version`,
		`upload_actions_artifact: "false"`,
		`upload_to_r2: "true"`,
		`publish_docker_image: "true"`,
		`docker_image: "license.pulserelay.pro/pulse-pro"`,
		`r2_prefix: $r2_prefix`,
		`reuse_existing_packet: "true"`,
		`allow_pre_activation_staging: "true"`,
		`allow_stable_ga_publish: $allow_stable_ga_publish`,
		`repos/rcourtman/pulse-enterprise/actions/workflows/build-pro-release.yml/dispatches`,
		`build_run_id="$(jq -r '.workflow_run_id // empty' <<<"${build_dispatch}")"`,
		`wait_for_workflow rcourtman/pulse-enterprise "${build_run_id}" "private Pro build"`,
		`echo "r2_prefix=${r2_prefix}" >> "$GITHUB_OUTPUT"`,
	} {
		if !strings.Contains(stageJob, needle) {
			t.Fatalf("stage_private_pro_runtime missing required contract: %s", needle)
		}
	}
	for _, needle := range []string{
		`R2_PREFIX: ${{ inputs.r2_prefix }}`,
		`PULSE_LEASE_SHA: ${{ inputs.pulse_lease_sha }}`,
		`PULSE_CONVERGENCE_RUN_ID: ${{ inputs.pulse_convergence_run_id }}`,
		`return_run_details: true`,
		`r2_prefix: $r2_prefix`,
		`allow_ga_prefix: $allow_ga_prefix`,
		`pulse_lease_sha: $pulse_lease_sha`,
		`pulse_convergence_run_id: $pulse_convergence_run_id`,
		`repos/rcourtman/pulse-pro/actions/workflows/promote-paid-runtime-release.yml/dispatches`,
		`promote_run_id="$(jq -r '.workflow_run_id // empty' <<<"${promote_dispatch}")"`,
		`wait_for_workflow rcourtman/pulse-pro "${promote_run_id}" "private Pro live promotion"`,
		`echo "::error::${label} failed with conclusion=${conclusion}: ${url}"`,
	} {
		if !strings.Contains(promotionJob, needle) {
			t.Fatalf("promote_private_pro_runtime missing required contract: %s", needle)
		}
	}
	for _, needle := range []string{
		`uses: ./.github/workflows/promote-private-pro-runtime.yml`,
		`r2_prefix: ${{ inputs.r2_prefix }}`,
		`pulse_lease_sha: ${{ needs.acquire_customer_promotion_lease.outputs.lock_sha }}`,
		`pulse_convergence_run_id: ${{ github.run_id }}`,
		`needs: acquire_customer_promotion_lease`,
	} {
		if !strings.Contains(convergencePromotionJob, needle) {
			t.Fatalf("release convergence private Pro promotion missing required contract: %s", needle)
		}
	}
	if strings.Contains(stageJob, "continue-on-error: true") || strings.Contains(promotionJob, "continue-on-error: true") {
		t.Fatal("private Pro staging and promotion must fail the release pipeline on error")
	}
	for label, job := range map[string]string{
		"private Pro staging":   stageJob,
		"private Pro promotion": promotionJob,
	} {
		if strings.Contains(job, "gh run list") || strings.Contains(job, "started_at") {
			t.Fatalf("%s must not infer a downstream run from time-ordered workflow listings", label)
		}
		for _, needle := range []string{
			`-H "X-GitHub-Api-Version: 2026-03-10"`,
			`local run_id="$2"`,
			`gh run view "${run_id}"`,
			`did not return an exact workflow run ID`,
		} {
			if !strings.Contains(job, needle) {
				t.Fatalf("%s missing exact downstream-run correlation contract: %s", label, needle)
			}
		}
	}
}

func TestReleaseBackendRaceGateUsesCompletePVEPartition(t *testing.T) {
	makefileBytes, err := os.ReadFile(repoFile("Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	if !strings.Contains(string(makefileBytes), "GO_TEST_TIMEOUT ?= 30m") {
		t.Fatal("backend race suite must keep a 30-minute per-package timeout for hosted-runner variance")
	}
	if !strings.Contains(string(makefileBytes), "go test -race -timeout $(GO_TEST_TIMEOUT)") {
		t.Fatal("make test must apply the governed backend race-suite timeout")
	}

	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}
	backendJob := workflowJobBlock(t, string(workflowBytes), "backend_tests")
	if !strings.Contains(backendJob, "timeout-minutes: 20") {
		t.Fatal("release backend job must retain the measured PVE execution ceiling")
	}
	if !strings.Contains(backendJob, "pulse-pve-tests") || !strings.Contains(backendJob, "run-release-backend-tests.sh") {
		t.Fatal("release backend job must use the dedicated PVE partition runner")
	}

	backendScriptBytes, err := os.ReadFile(repoFile("scripts", "run-release-backend-tests.sh"))
	if err != nil {
		t.Fatalf("read run-release-backend-tests.sh: %v", err)
	}
	backendScript := string(backendScriptBytes)
	for _, needle := range []string{
		"go test -c -race",
		"python3 scripts/shard_go_tests.py",
		`--max-regex-bytes "$MAX_REGEX_BYTES"`,
		`MEMORY_WAIT_SECONDS="${PULSE_BACKEND_TEST_MEMORY_WAIT_SECONDS:-120}"`,
		// Admission thresholds are grounded in the 2026-08-21 direct probe on
		// the PVE worker (~7.5 GiB measured gate footprint); auto mode must
		// degrade the shard count when headroom is missing, never fail the
		// release at admission.
		`2) echo $((8 * 1024 * 1024))`,
		`*) echo $((10 * 1024 * 1024))`,
		"Degrading to $cpu_shards API shard(s)",
		// Shard CPU is weighted by planned test volume: the serial prefix
		// shard measured 569s at 2 procs vs 484s at 4 on 2026-08-21, while
		// the wait-bound tails cannot use extra width.
		"SHARD_GOMAXPROCS",
		`GOMAXPROCS="$shard_procs"`,
		"--shard-boundaries",
		"TestWebSocketOriginAllowsTrustedForwardedHostedOriginIPv6Loopback",
		"TestServerInfoEndpointMethodNotAllowed",
		"go test -race -timeout 30m",
		`-test.timeout 30m`,
	} {
		if !strings.Contains(backendScript, needle) {
			t.Fatalf("PVE backend partition missing coverage contract: %s", needle)
		}
	}
}

func TestHelmAgentRuntimePointsAtRealImage(t *testing.T) {
	// The helm chart's agent.enabled=true workload used to default to
	// ghcr.io/rcourtman/pulse-agent — an image that was never published.
	// The chart now points at the main rcourtman/pulse image and uses an
	// arch-resolved /usr/local/bin/pulse-agent symlink baked into the
	// runtime stage. This test pins:
	//   1. values.yaml uses the main image
	//   2. values.yaml has the command override
	//   3. the agent template renders the command
	//   4. the Dockerfile creates the symlink for every supported arch
	//   5. validate-release.sh asserts the symlink exists in the published image
	// Reverting any one of these unwires the chart back to ImagePullBackOff.

	valuesBytes, err := os.ReadFile(repoFile("deploy", "helm", "pulse", "values.yaml"))
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}
	values := string(valuesBytes)
	if !strings.Contains(values, "repository: rcourtman/pulse\n") {
		t.Fatal("agent.image.repository must default to rcourtman/pulse (single-image agent + server)")
	}
	// Match the actual config value, not casual mentions in surrounding
	// comments that explain why the default changed.
	if strings.Contains(values, "repository: ghcr.io/rcourtman/pulse-agent") {
		t.Fatal("agent.image.repository must not reference the never-published ghcr.io/rcourtman/pulse-agent image")
	}
	if !strings.Contains(values, "- /usr/local/bin/pulse-agent") {
		t.Fatal("agent.command must default to /usr/local/bin/pulse-agent so the main image's server ENTRYPOINT is overridden")
	}

	agentTemplate, err := os.ReadFile(repoFile("deploy", "helm", "pulse", "templates", "agent.yaml"))
	if err != nil {
		t.Fatalf("read agent.yaml: %v", err)
	}
	tmpl := string(agentTemplate)
	if !strings.Contains(tmpl, "{{- if .Values.agent.command }}") {
		t.Fatal("agent.yaml template must conditionally render command from .Values.agent.command")
	}
	if !strings.Contains(tmpl, "command:\n            {{- toYaml .Values.agent.command | nindent 12 }}") {
		t.Fatal("agent.yaml template must render command via toYaml so list values pass through correctly")
	}

	assertFileContainsAll(t, repoFile("Dockerfile"),
		`ln -s /opt/pulse/bin/pulse-agent-linux-arm64 /usr/local/bin/pulse-agent`,
		`ln -s /opt/pulse/bin/pulse-agent-linux-armv7 /usr/local/bin/pulse-agent`,
		`ln -s /opt/pulse/bin/pulse-agent-linux-amd64 /usr/local/bin/pulse-agent`,
	)

	assertFileContainsAll(t, repoFile("scripts", "validate-release.sh"),
		`Validating /usr/local/bin/pulse-agent arch-resolved symlink`,
		`[ -L /usr/local/bin/pulse-agent ]`,
		`/usr/local/bin/pulse-agent target is not executable`,
	)
}

func repoFile(parts ...string) string {
	root := filepath.Join("..", "..")
	segments := append([]string{root}, parts...)
	return filepath.Join(segments...)
}

// assertFileContainsAll reads the file at path and fails the test if any of
// the required substrings is missing. The standard pinning-test shape in
// this package.
func assertFileContainsAll(t *testing.T, path string, required ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(content)
	for _, needle := range required {
		if !strings.Contains(s, needle) {
			t.Fatalf("%s missing required substring: %s", path, needle)
		}
	}
}

func assertFileContainsAllNormalized(t *testing.T, path string, required ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := normalizedInstallTestWhitespace(string(content))
	for _, needle := range required {
		if !strings.Contains(s, normalizedInstallTestWhitespace(needle)) {
			t.Fatalf("%s missing required normalized substring: %s", path, needle)
		}
	}
}

func normalizedInstallTestWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func workflowJobBlock(t *testing.T, workflow, job string) string {
	t.Helper()

	startMarker := "\n  " + job + ":\n"
	start := strings.Index(workflow, startMarker)
	if start == -1 {
		t.Fatalf("workflow missing job %s", job)
	}
	start += 1
	rest := workflow[start+len("  "+job+":\n"):]
	end := len(rest)
	for _, line := range strings.Split(rest, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			candidate := strings.Index(rest, "\n"+line)
			if candidate >= 0 {
				end = candidate
				break
			}
		}
	}
	return workflow[start : start+len("  "+job+":\n")+end]
}
