package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity"
)

func TestPublishedAgentReleaseIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("real release-style executable build")
	}
	root := filepath.Join("..", "..")
	flags := exec.Command("bash", filepath.ToSlash(filepath.Join(root, "scripts", "release_ldflags.sh")), "agent", "--version", "6.2.0-rc.8")
	output, err := flags.CombinedOutput()
	if err != nil {
		t.Fatalf("release flags: %v %s", err, output)
	}
	binary := filepath.Join(t.TempDir(), "pulse-agent")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	cmd := exec.Command("go", "build", "-p=1", "-trimpath", "-buildvcs=false", "-ldflags="+strings.TrimSpace(string(output)), "-o", binary, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("canonical build: %v %s", err, out)
	}
	target, err := agentidentity.CurrentTarget()
	if err != nil {
		t.Fatal(err)
	}
	if err := agentidentity.Validate(binary, "6.2.0-rc.8", target); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil || string(out) != "v6.2.0-rc.8\n" {
		t.Fatalf("--version: %q %v", out, err)
	}
	out, err = exec.Command(binary, "--release-identity").CombinedOutput()
	if err != nil || string(out) != "[[pulse-agent-release:v1:v6.2.0-rc.8]]\n" {
		t.Fatalf("runtime release identity: %q %v", out, err)
	}
	if err := agentidentity.Validate(binary, "6.2.0", target); err == nil {
		t.Fatal("accepted RC as stable")
	}
}

func TestUnstampedAgentReleaseIdentityCannotUseLinkerLiteral(t *testing.T) {
	if testing.Short() {
		t.Skip("real unstamped executable control")
	}
	binary := filepath.Join(t.TempDir(), "unstamped-agent")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	// Non-trimpath linker command metadata can contain a reserved literal even
	// when the actual shared ReleaseIdentity variable remains unset. This is a
	// static-contract control, not a canonical release or source authentication.
	flags := "-s -w -X main.Version=v6.2.1 -X main.UnusedReleaseLiteral=[[pulse-agent-release:v1:v6.2.1]]"
	cmd := exec.Command("go", "build", "-p=1", "-buildvcs=false", "-ldflags="+flags, "-o", binary, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unstamped main build: %v %s", err, out)
	}
	target, err := agentidentity.CurrentTarget()
	if err != nil {
		t.Fatal(err)
	}
	if err := agentidentity.Validate(binary, "v6.2.1", target); err == nil || !strings.Contains(err.Error(), "recorded linker flags") {
		t.Fatalf("wrong-symbol linker literal admitted: %v", err)
	}
	out, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil || string(out) != "v6.2.1\n" {
		t.Fatalf("development version output: %q %v", out, err)
	}
	out, err = exec.Command(binary, "--release-identity").CombinedOutput()
	if err == nil {
		t.Fatalf("unstamped executable admitted through linker metadata: %q", out)
	}
}

func TestPublishedAgentReleaseIdentityNonTrimpath(t *testing.T) {
	if testing.Short() {
		t.Skip("real non-trimpath release build")
	}
	recipe := exec.Command("bash", filepath.Join("..", "..", "scripts", "release_ldflags.sh"), "agent", "--version", "6.2.1")
	output, err := recipe.CombinedOutput()
	if err != nil {
		t.Fatalf("release flags: %v %s", err, output)
	}
	path := filepath.Join(t.TempDir(), "agent")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	cmd := exec.Command("go", "build", "-p=1", "-buildvcs=false", "-ldflags="+strings.TrimSpace(string(output)), "-o", path, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("non-trimpath release build: %v %s", err, out)
	}
	target, err := agentidentity.CurrentTarget()
	if err != nil {
		t.Fatal(err)
	}
	if err := agentidentity.Validate(path, "6.2.1", target); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(path, "--release-identity").CombinedOutput()
	if err != nil || string(out) != "[[pulse-agent-release:v1:v6.2.1]]\n" {
		t.Fatalf("actual stamp: %q %v", out, err)
	}
}
