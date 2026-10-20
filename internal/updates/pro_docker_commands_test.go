package updates

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestProDockerUpdateCommandsImageShapes(t *testing.T) {
	const image = "registry.pulserelay.pro/pulse/pulse-pro"
	digest := "sha256:" + strings.Repeat("ab", 32)
	other := "sha256:" + strings.Repeat("cd", 32)
	ref := image + "@" + digest
	for _, tc := range []struct {
		name   string
		change func(*proBrokerDocker)
		want   bool
	}{
		{"pinned broker image", func(d *proBrokerDocker) {}, true},
		{"legacy bare image", func(d *proBrokerDocker) { d.Image = image }, true},
		{"tag and digest identity", func(d *proBrokerDocker) {
			d.Image = image + ":v6.6.0@" + digest
			d.ComposePullCommand = strings.ReplaceAll(d.ComposePullCommand, ref, d.Image)
			d.ComposeUpCommand = strings.ReplaceAll(d.ComposeUpCommand, ref, d.Image)
		}, true},
		{"conflicting image pin", func(d *proBrokerDocker) { d.Image = image + "@" + other }, false},
		{"conflicting digest field", func(d *proBrokerDocker) { d.ImageDigest = other }, false},
		{"double pin", func(d *proBrokerDocker) { d.Image += "@" + digest }, false},
		{"empty image base", func(d *proBrokerDocker) { d.Image = "@" + digest }, false},
		{"empty image", func(d *proBrokerDocker) { d.Image = "" }, false},
		{"empty image pin", func(d *proBrokerDocker) { d.Image = image + "@" }, false},
		{"malformed digest", func(d *proBrokerDocker) { d.ImageDigest = "sha256:ab" }, false},
		{"missing digest", func(d *proBrokerDocker) { d.ImageDigest = "" }, false},
		{"missing pull", func(d *proBrokerDocker) { d.ComposePullCommand = "" }, false},
		{"missing up", func(d *proBrokerDocker) { d.ComposeUpCommand = "" }, false},
		{"wrong pull pin", func(d *proBrokerDocker) {
			d.ComposePullCommand = strings.ReplaceAll(d.ComposePullCommand, digest, other)
		}, false},
		{"wrong up pin", func(d *proBrokerDocker) {
			d.ComposeUpCommand = strings.ReplaceAll(d.ComposeUpCommand, digest, other)
		}, false},
		{"mutable pull tag", func(d *proBrokerDocker) {
			d.ComposePullCommand = "PULSE_IMAGE='" + image + ":v6.6.0' docker compose pull"
		}, false},
		{"mutable up tag", func(d *proBrokerDocker) {
			d.ComposeUpCommand = "PULSE_IMAGE='" + image + ":v6.6.0' docker compose up -d"
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := &proBrokerResponse{Docker: &proBrokerDocker{
				Image:              ref,
				ImageDigest:        digest,
				LoginCommand:       "docker login registry.pulserelay.pro --username 'fixture'",
				ComposePullCommand: "PULSE_IMAGE='" + ref + "' docker compose -f 'monitoring.yml' -p 'existing-project' pull pulse",
				ComposeUpCommand:   "PULSE_IMAGE='" + ref + "' docker compose -f 'monitoring.yml' -p 'existing-project' up -d pulse",
			}}
			manifest.Release.Version = "6.6.0"
			tc.change(manifest.Docker)
			got := proDockerUpdateCommands(manifest)
			if !tc.want {
				if got != nil {
					t.Fatalf("inconsistent broker block must withhold commands, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("consistent broker block must retain commands")
			}
			d := manifest.Docker
			if got.Image != d.Image || got.ImageDigest != digest || got.Version != "v6.6.0" ||
				got.ComposePullCommand != d.ComposePullCommand || got.ComposeUpCommand != d.ComposeUpCommand || got.LoginCommand != d.LoginCommand {
				t.Fatalf("reader must preserve the broker's exact image, Compose identity and login: %+v", got)
			}
		})
	}
	if proDockerUpdateCommands(nil) != nil || proDockerUpdateCommands(&proBrokerResponse{}) != nil {
		t.Fatal("missing manifest or Docker block must withhold commands")
	}
}

// Exercise the real authenticated HTTP broker reader and update-check output,
// not just a manually assembled frontend DockerUpdateCommands fixture. The
// default broker fixture mirrors the producer's already-pinned image field.
func TestCheckProUpdatesDockerBrokerImageShapes(t *testing.T) {
	setupProUpdateTest(t, "6.5.0")
	for _, tc := range []struct {
		name, shape, login string
	}{
		{name: "pinned", shape: "pinned"},
		{name: "legacy", shape: "legacy"},
		{name: "pinned-prompt", shape: "pinned", login: "docker login 'registry.pulserelay.pro' --username 'lic_test'"},
		{name: "legacy-prompt", shape: "legacy", login: "docker login 'registry.pulserelay.pro' --username 'lic_test'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newProBrokerFixture(t, "6.6.0", false)
			fixture.dockerBlock = defaultBrokerDockerBlock(fixture.version)
			if tc.shape == "legacy" {
				fixture.dockerBlock["image"] = strings.SplitN(fixture.dockerBlock["image"].(string), "@", 2)[0]
			}
			if tc.login != "" {
				fixture.dockerBlock["login_command"] = tc.login
			}
			manager := NewManager(&config.Config{DataPath: t.TempDir()})
			manager.SetProUpdateCredentialSource(fixture.credentialSource())
			current, err := ParseVersion("6.5.0")
			if err != nil {
				t.Fatal(err)
			}
			info, err := manager.checkProUpdates(context.Background(), "stable", &VersionInfo{
				Version: "6.5.0", Build: "release", IsDocker: true, DeploymentType: "docker",
			}, current)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Available || info.DockerUpdate == nil || fixture.brokerCalls != 1 || fixture.tarballCalls != 0 || fixture.sshsigCalls != 0 {
				t.Fatalf("expected one broker check with Docker guidance and no artifact/install work: %+v", info)
			}
			got := info.DockerUpdate
			if got.Image != fixture.dockerBlock["image"] || got.ComposePullCommand != fixture.dockerBlock["compose_pull_command"] ||
				got.ComposeUpCommand != fixture.dockerBlock["compose_up_command"] || got.LoginCommand != fixture.dockerBlock["login_command"] {
				t.Fatalf("broker identity/commands changed: %+v", got)
			}
			wire, err := json.Marshal(info)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("DOCKER_BROKER_READER_%s=%s", tc.name, wire)
		})
	}
}
