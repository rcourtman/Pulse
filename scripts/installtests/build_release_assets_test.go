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
		`cp scripts/install.sh "$RELEASE_DIR/install.sh"`,
		`[ -f "scripts/install.ps1" ] && cp "scripts/install.ps1" "$RELEASE_DIR/install.ps1"`,
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
		`agent_ldflags="$(./scripts/release_ldflags.sh agent --version "v${VERSION}")"`,
		`server_ldflags="$(./scripts/release_ldflags.sh server --version "v${VERSION}" --build-time "${build_time}" --git-commit "${git_commit}" "${license_ldflags_args[@]}")"`,
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
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("create-release.yml missing required installer upload step: %s", needle)
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
		`COPY scripts/release_ldflags.sh ./scripts/release_ldflags.sh`,
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}"`,
		`./scripts/release_ldflags.sh agent --version "${VERSION}"`,
	}
	for _, needle := range dockerRequired {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing canonical release ldflags usage: %s", needle)
		}
	}

	workflowBytes, err := os.ReadFile(repoFile(".github", "workflows", "deploy-demo-server.yml"))
	if err != nil {
		t.Fatalf("read deploy-demo-server workflow: %v", err)
	}
	workflow := string(workflowBytes)
	workflowRequired := []string{
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}"`,
	}
	for _, needle := range workflowRequired {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("deploy-demo-server workflow missing canonical release ldflags usage: %s", needle)
		}
	}
}

func repoFile(parts ...string) string {
	root := filepath.Join("..", "..")
	segments := append([]string{root}, parts...)
	return filepath.Join(segments...)
}
