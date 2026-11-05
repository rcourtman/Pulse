package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentBuildRecipePreservesEveryPublishedTarget(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("requires Bash")
	}
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	tools := filepath.Join(root, "tools")
	for _, dir := range []string{scripts, tools} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"build-agents.sh", "release_build_targets.sh", "release_ldflags.sh"} {
		body, err := os.ReadFile(repoFile("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(scripts, name), body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	grammarDir := filepath.Join(root, "internal", "agentidentity")
	if err := os.MkdirAll(grammarDir, 0700); err != nil {
		t.Fatal(err)
	}
	grammar, err := os.ReadFile(repoFile("internal", "agentidentity", "version-pattern.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grammarDir, "version-pattern.txt"), grammar, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("6.2.0-rc.8\n"), 0600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(root, "invocations")
	compiler := []byte("#!/bin/sh\nprintf '%s|%s|%s|%s|%s\\n' \"$GOOS\" \"$GOARCH\" \"${GOARM-}\" \"$CGO_ENABLED\" \"$*\" >> \"$PULSE_TEST_COMPILER_LOG\"\n")
	if err := os.WriteFile(filepath.Join(tools, "go"), compiler, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(scripts, "build-agents.sh"))
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "PULSE_TEST_COMPILER_LOG="+log, "GOARM=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("agent producer: %v %s", err, out)
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 12 {
		t.Fatalf("published matrix changed: %v", lines)
	}
	found := map[string]bool{}
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			t.Fatalf("bad invocation %q", line)
		}
		found[parts[0]+"-"+parts[1]+"-"+parts[2]] = true
		for _, required := range []string{"[[pulse-agent-release:v1:v6.2.0-rc.8]]", "-trimpath", "-buildvcs=false", "./cmd/pulse-agent"} {
			if !strings.Contains(parts[4], required) {
				t.Fatalf("missing %s: %s", required, line)
			}
		}
		if parts[3] != "0" {
			t.Fatal("CGO was enabled")
		}
	}
	if !found["linux-arm-6"] || !found["linux-arm-7"] || !found["windows-amd64-"] || !found["darwin-arm64-"] {
		t.Fatalf("lost published targets: %v", found)
	}
}

func TestUniversalAgentWrapperDispatchesPublishedLinuxTargets(t *testing.T) {
	for _, name := range []string{"pulse", "pulse-agent"} {
		t.Run(name, func(t *testing.T) { checkUniversalWrapper(t, name) })
	}
}

func checkUniversalWrapper(t *testing.T, name string) {
	source, err := os.ReadFile(repoFile("scripts", "build-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	marker := `cat > "$universal_dir/bin/` + name + `" << 'EOF'` + "\n"
	start := strings.Index(string(source), marker)
	if start < 0 {
		t.Fatal("universal agent wrapper missing")
	}
	start += len(marker)
	end := strings.Index(string(source)[start:], "\nEOF")
	if end < 0 {
		t.Fatal("universal agent wrapper terminator missing")
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, name)
	if err := os.WriteFile(wrapper, source[start:start+end], 0700); err != nil {
		t.Fatal(err)
	}
	uname := filepath.Join(dir, "uname")
	if err := os.WriteFile(uname, []byte("#!/bin/sh\nprintf '%s\\n' \"$PULSE_TEST_ARCH\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"amd64", "arm64", "armv6", "armv7", "386"} {
		body := []byte("#!/bin/sh\nprintf '%s:%s\\n' " + target + " \"$1\"\n")
		if err := os.WriteFile(filepath.Join(dir, name+"-linux-"+target), body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for arch, target := range map[string]string{"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64", "armv6l": "armv6", "armv7l": "armv7", "armhf": "armv7", "i386": "386", "i686": "386", "unknown": ""} {
		cmd := exec.Command(wrapper, "probe")
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "PULSE_TEST_ARCH="+arch)
		out, err := cmd.CombinedOutput()
		if target == "" {
			if err == nil {
				t.Fatal("unsupported architecture accepted")
			}
			continue
		}
		if err != nil || string(out) != target+":probe\n" {
			t.Fatalf("%s selected %q: %v", arch, out, err)
		}
	}
}

func TestAgentBuildRecipeDevDeploymentUsesCanonicalIdentity(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"scripts/dev-deploy-agent.sh", "scripts/release_build_targets.sh", "scripts/release_ldflags.sh", "internal/agentidentity/version-pattern.txt"} {
		body, err := os.ReadFile(repoFile(strings.Split(name, "/")...))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("6.2.1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(root, "tools")
	if err := os.Mkdir(tools, 0700); err != nil {
		t.Fatal(err)
	}
	compiler := `#!/bin/sh
printf '%s|%s|%s|%s\n' "$GOOS" "$GOARCH" "$GOARM" "$*" >> "$PULSE_TEST_COMPILER_LOG"
while [ "$#" -gt 0 ]; do
 if [ "$1" = -o ]; then shift; : > "$1"; fi
 shift
done
`
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte(compiler), 0700); err != nil {
		t.Fatal(err)
	}
	// Refuse the explicit test host before any service or deployment command.
	if err := os.WriteFile(filepath.Join(tools, "ssh"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(root, "compiler.log")
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "dev-deploy-agent.sh"), "unreachable-fixture")
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "PULSE_TEST_COMPILER_LOG="+log, "GOOS=linux", "GOARCH=arm", "GOARM=6")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Cannot connect") {
		t.Fatalf("host refusal: %v %s", err, output)
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"linux|arm|6|", "pulse-agent-linux-armv6", "-trimpath", "-buildvcs=false", "[[pulse-agent-release:v1:v6.2.1]]"} {
		if !strings.Contains(string(body), required) {
			t.Fatalf("missing %q: %s", required, body)
		}
	}
}

func TestAgentBuildRecipeDevDeploymentContinuesAfterFailedHost(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"scripts/dev-deploy-agent.sh", "scripts/release_build_targets.sh", "scripts/release_ldflags.sh", "internal/agentidentity/version-pattern.txt"} {
		body, err := os.ReadFile(repoFile(strings.Split(name, "/")...))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("6.2.1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(root, "tools")
	if err := os.Mkdir(tools, 0700); err != nil {
		t.Fatal(err)
	}
	doubles := map[string]string{
		"go":  "#!/bin/sh\nwhile [ \"$#\" -gt 0 ];do if [ \"$1\" = -o ];then shift; : > \"$1\";fi;shift;done\n",
		"scp": "#!/bin/sh\nexit 0\n", "sleep": "#!/bin/sh\nexit 0\n",
		"ssh": `#!/bin/sh
failed=false
for arg do
 case "$arg" in failed-host) failed=true;; recovered-host) failed=false;; esac
 last="$arg"
done
case "$last" in
 "uname -m") echo x86_64;;
 *"pgrep -x"*) if "$failed";then exit 1;fi;;
 *"tail -n 5"*) echo "fixture diagnostic retained";exit 1;;
esac
`,
	}
	for name, body := range doubles {
		if err := os.WriteFile(filepath.Join(tools, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "dev-deploy-agent.sh"), "failed-host", "recovered-host")
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "GOOS=linux", "GOARCH=amd64")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("failed-host result lost")
	}
	for _, want := range []string{"fixture diagnostic retained", "Successfully deployed to: recovered-host", "Failed to deploy to: failed-host"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing %q: %v %s", want, err, output)
		}
	}
}
