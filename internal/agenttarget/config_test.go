package agenttarget

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

func writePrivateFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadObservers(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret\n")
	writePrivateFile(t, configPath, `{
  "version": 1,
  "observers": [{
    "name": "dev",
    "url": "http://192.168.0.10:7655/",
    "tokenFile": "`+tokenPath+`",
    "allowPlaintextHTTP": true,
    "provisionProxmox": true
  }]
}`)

	observers, err := LoadObservers(configPath, "https://prod.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(observers) != 1 {
		t.Fatalf("len(observers) = %d, want 1", len(observers))
	}
	got := observers[0]
	if got.Name != "dev" || got.URL != "http://192.168.0.10:7655" || got.Token != "observer-secret" || !got.ProvisionProxmox || got.ID == "" {
		t.Fatalf("observer = %+v", got)
	}
}

func TestLoadObserversRejectsPrimaryDuplicate(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret")
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"https://pulse.example.com/","tokenFile":"`+tokenPath+`"}]}`)

	if _, err := LoadObservers(configPath, "https://pulse.example.com"); err == nil {
		t.Fatal("expected duplicate primary URL rejection")
	}
}

func TestLoadObserversRejectsInlineOrLooseSecrets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission check")
	}
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret")
	if err := os.Chmod(tokenPath, 0o644); err != nil {
		t.Fatal(err)
	}
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"https://dev.example.com","tokenFile":"`+tokenPath+`"}]}`)

	if _, err := LoadObservers(configPath, "https://prod.example.com"); err == nil {
		t.Fatal("expected loose token permission rejection")
	}
}

func TestLoadObserversRejectsInlineTokenField(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"https://dev.example.com","token":"inline-secret"}]}`)
	if _, err := LoadObservers(configPath, "https://prod.example.com"); err == nil {
		t.Fatal("expected inline token field rejection")
	}
}

func TestLoadObserversRejectsSymlinkedTokenFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges")
	}
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	linkPath := filepath.Join(dir, "dev-link.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret")
	if err := os.Symlink(tokenPath, linkPath); err != nil {
		t.Fatal(err)
	}
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"https://dev.example.com","tokenFile":"`+linkPath+`"}]}`)
	if _, err := LoadObservers(configPath, "https://prod.example.com"); err == nil {
		t.Fatal("expected symlinked token file rejection")
	}
}

func TestLoadObserversRequiresExplicitPublicPlaintextOptIn(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret")
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"http://203.0.113.10:7655","tokenFile":"`+tokenPath+`"}]}`)
	if _, err := LoadObservers(configPath, "https://prod.example.com"); err == nil {
		t.Fatal("expected public plaintext URL rejection")
	}
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"http://203.0.113.10:7655","tokenFile":"`+tokenPath+`","allowPlaintextHTTP":true}]}`)
	if _, err := LoadObservers(configPath, "https://prod.example.com"); err != nil {
		t.Fatalf("explicit plaintext opt-in: %v", err)
	}
}

func TestLoadObserversRequiresExplicitPrivatePlaintextOptIn(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret")
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"http://192.168.50.10:7655","tokenFile":"`+tokenPath+`"}]}`)
	if _, err := LoadObservers(configPath, "https://prod.example.com"); err == nil {
		t.Fatal("expected private-network plaintext URL rejection without observer opt-in")
	}
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"http://192.168.50.10:7655","tokenFile":"`+tokenPath+`","allowPlaintextHTTP":true}]}`)
	if _, err := LoadObservers(configPath, "https://prod.example.com"); err != nil {
		t.Fatalf("explicit private-network plaintext opt-in: %v", err)
	}
}

func TestObserverPlaintextPolicyDoesNotInheritPrimaryProcessConsent(t *testing.T) {
	securityutil.SetOperatorPlaintextHTTPConsent(true)
	defer securityutil.SetOperatorPlaintextHTTPConsent(false)

	if _, err := NormalizePulseURL("http://203.0.113.10:7655", false, false); err == nil {
		t.Fatal("observer without explicit opt-in inherited process-wide primary plaintext consent")
	}
	if got, err := NormalizePulseURL("http://203.0.113.10:7655", false, true); err != nil {
		t.Fatalf("explicit observer plaintext opt-in: %v", err)
	} else if got != "http://203.0.113.10:7655" {
		t.Fatalf("normalized URL = %q", got)
	}
}

func TestLoadObserversDefaultsProxmoxProvisioningOn(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	configPath := filepath.Join(dir, "observers.json")
	writePrivateFile(t, tokenPath, "observer-secret")
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"dev","url":"https://dev.example.com","tokenFile":"`+tokenPath+`"}]}`)

	observers, err := LoadObservers(configPath, "https://prod.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !observers[0].ProvisionProxmox {
		t.Fatal("expected Proxmox provisioning to default on")
	}
}
