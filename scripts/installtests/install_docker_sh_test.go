package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDockerScriptUsesConfiguredImageRepoDefault(t *testing.T) {
	workDir := t.TempDir()
	runInstallDockerScript(t, workDir, "DOCKER_IMAGE_REPO=example/pulse-enterprise")

	composePath := filepath.Join(workDir, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	composeText := string(composeContent)
	if !strings.Contains(composeText, "image: ${PULSE_IMAGE:-example/pulse-enterprise:latest}") {
		t.Fatalf("docker-compose.yml missing configured image default:\n%s", composeText)
	}
	if strings.Contains(composeText, "rcourtman/pulse:latest") {
		t.Fatalf("docker-compose.yml still hardcodes upstream image:\n%s", composeText)
	}

	envPath := filepath.Join(workDir, ".env")
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	envText := string(envContent)
	if !strings.Contains(envText, "PULSE_IMAGE=example/pulse-enterprise:latest") {
		t.Fatalf(".env missing configured image default:\n%s", envText)
	}
}

func TestInstallDockerScriptPrefersExplicitPulseImage(t *testing.T) {
	workDir := t.TempDir()
	runInstallDockerScript(
		t,
		workDir,
		"DOCKER_IMAGE_REPO=example/pulse-enterprise",
		"PULSE_IMAGE=ghcr.io/example/pulse-enterprise:v9.9.9",
	)

	composePath := filepath.Join(workDir, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	composeText := string(composeContent)
	if !strings.Contains(composeText, "image: ${PULSE_IMAGE:-example/pulse-enterprise:latest}") {
		t.Fatalf("docker-compose.yml lost configured default image:\n%s", composeText)
	}

	envPath := filepath.Join(workDir, ".env")
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	envText := string(envContent)
	if !strings.Contains(envText, "PULSE_IMAGE=ghcr.io/example/pulse-enterprise:v9.9.9") {
		t.Fatalf(".env did not preserve explicit image override:\n%s", envText)
	}
}

func runInstallDockerScript(t *testing.T, workDir string, envVars ...string) {
	t.Helper()

	scriptPath := repoFile("scripts", "install-docker.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install-docker.sh: %v", err)
	}

	script := string(content)
	script = strings.Replace(script, rootCheckBlock, ":", 1)
	script = strings.Replace(script, containerCheckBlock, ":", 1)

	tmpScript := filepath.Join(workDir, "install-docker.sh")
	if err := os.WriteFile(tmpScript, []byte(script), 0o755); err != nil {
		t.Fatalf("write temp install-docker.sh: %v", err)
	}

	binDir := filepath.Join(workDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeTestStub(t, filepath.Join(binDir, "docker"), "#!/bin/sh\nif [ \"$1\" = \"compose\" ] && [ \"$2\" = \"version\" ]; then exit 0; fi\nexit 0\n")
	writeTestStub(t, filepath.Join(binDir, "timedatectl"), "#!/bin/sh\necho Europe/London\n")
	writeTestStub(t, filepath.Join(binDir, "hostname"), "#!/bin/sh\nif [ \"$1\" = \"-I\" ]; then echo 192.0.2.10; else echo pulse-host; fi\n")

	cmd := exec.Command("bash", tmpScript)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), append([]string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}, envVars...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run install-docker.sh: %v\n%s", err, out)
	}
}

func writeTestStub(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}

const rootCheckBlock = `# Check if running as root (early check for better error messages)
if [ "$EUID" -ne 0 ]; then
    echo "❌ ERROR: This script must be run as root"
    echo ""
    echo "Please run: sudo $0"
    exit 1
fi
`

const containerCheckBlock = `# Detect if running in a container
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    echo "❌ ERROR: This script must run on the Docker host, not inside a container"
    echo ""
    echo "Please run this script on your Docker host machine."
    exit 1
fi
`
