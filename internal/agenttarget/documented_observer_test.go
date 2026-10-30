package agenttarget

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the shipped recipe through the actual parser. No provider request,
// service installation or Proxmox credential operation is needed to choose it.
func TestDocumentedObserverTelemetryChoice(t *testing.T) {
	doc, err := os.ReadFile("../../docs/UNIFIED_AGENT.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(doc), "## Observer destinations\n")
	if !ok {
		t.Fatal("observer guide missing")
	}
	section, _, _ = strings.Cut(section, "### Advanced Flags\n")
	_, fence, ok := strings.Cut(section, "```json\n")
	if !ok {
		t.Fatal("documented observer JSON missing")
	}
	fence, _, _ = strings.Cut(fence, "```")
	var config observerFile
	if err := json.Unmarshal([]byte(fence), &config); err != nil {
		t.Fatal(err)
	}
	if config.Version != ConfigVersion || len(config.Observers) != 1 {
		t.Fatal("documented observer shape changed")
	}
	target := &config.Observers[0]
	if target.ProvisionProxmox == nil || *target.ProvisionProxmox || target.InsecureSkipVerify || target.AllowPlaintextHTTP || !strings.HasPrefix(target.URL, "https://") {
		t.Fatal("telemetry recipe must explicitly avoid extra API provisioning and retain verified HTTPS")
	}
	if !filepath.IsAbs(target.TokenFile) || target.ServerFingerprint == "" {
		t.Fatal("recipe must use a private token file and explicit trust")
	}

	dir := t.TempDir()
	target.TokenFile = filepath.Join(dir, "observer.token")
	writePrivateFile(t, target.TokenFile, "synthetic-docs-observer-token\n")
	configPath := filepath.Join(dir, "observers.json")
	load := func() ([]Observer, error) {
		t.Helper()
		data, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		writePrivateFile(t, configPath, string(data))
		return LoadObservers(configPath, "https://primary.example.test")
	}
	t.Run("documented-telemetry-only", func(t *testing.T) {
		got, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ProvisionProxmox || got[0].Token != "synthetic-docs-observer-token" {
			t.Fatal("documented choice did not survive actual loading")
		}
	})
	t.Run("omission-is-not-telemetry-only", func(t *testing.T) {
		saved := target.ProvisionProxmox
		target.ProvisionProxmox = nil
		defer func() { target.ProvisionProxmox = saved }()
		got, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if !got[0].ProvisionProxmox {
			t.Fatal("guide's omitted-field warning must match the runtime default")
		}
	})
	t.Run("primary-local-http-policy-is-not-observer-consent", func(t *testing.T) {
		saved := target.URL
		target.URL = "http://192.168.50.10:7655"
		defer func() { target.URL = saved }()
		if _, err := load(); err == nil {
			t.Fatal("observer recipe must not silently accept private-LAN plaintext")
		}
	})
}
