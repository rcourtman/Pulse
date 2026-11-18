package agenttarget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

func observerJSONValue(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLoadObserversRejectsCompetingJSONFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "observers.json")
	tokenPath := filepath.Join(dir, "observer.token")
	otherTokenPath := filepath.Join(dir, "other.token")
	writePrivateFile(t, tokenPath, "synthetic-observer-token")
	writePrivateFile(t, otherTokenPath, "synthetic-other-token")
	base := []string{`"name":"dev"`, `"url":"https://observer.example.test"`, `"tokenFile":` + observerJSONValue(t, tokenPath)}
	row := "{" + strings.Join(base, ",") + "}"
	fields := []struct{ name, first, last string }{
		{"version", "0", "1"},
		{"observers", "[]", "[" + row + "]"},
		{"name", `"other"`, `"dev"`},
		{"url", `"https://other.example.test"`, `"https://observer.example.test"`},
		{"tokenFile", observerJSONValue(t, otherTokenPath), observerJSONValue(t, tokenPath)},
		{"insecureSkipVerify", "false", "true"},
		{"allowPlaintextHTTP", "false", "true"},
		{"caCertPath", `"/synthetic/first.pem"`, `"/synthetic/last.pem"`},
		{"serverFingerprint", `"first-synthetic-pin"`, `"last-synthetic-pin"`},
		{"provisionProxmox", "false", "true"},
	}
	for _, field := range fields {
		for _, alias := range []string{"exact", "folded", "escaped"} {
			for _, equal := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/equal=%t", field.name, alias, equal), func(t *testing.T) {
					key := field.name
					switch alias {
					case "folded":
						key = strings.ToUpper(key)
					case "escaped":
						key = fmt.Sprintf(`\u%04x%s`, key[0], key[1:])
					}
					first := field.first
					if equal {
						first = field.last
					}
					pair := `"` + field.name + `":` + first + `,"` + key + `":` + field.last
					var body string
					if field.name == "version" {
						body = "{" + pair + `,"observers":[` + row + "]}"
					} else if field.name == "observers" {
						body = `{"version":1,` + pair + "}"
					} else {
						members := []string{pair}
						for _, member := range base {
							if !strings.HasPrefix(member, `"`+field.name+`":`) {
								members = append(members, member)
							}
						}
						body = `{"version":1,"observers":[{` + strings.Join(members, ",") + "}]}"
					}
					writePrivateFile(t, configPath, body)
					got, err := LoadObservers(configPath, "https://primary.example.test")
					if got != nil || err == nil || err.Error() != "observer config contains repeated JSON fields; use each field once" {
						t.Fatalf("competing observer fields were not refused without exposing their content: %v", err)
					}
				})
			}
		}
	}
}

func TestLoadObserversChecksWholeDocumentBeforeTokenResolution(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "observers.json")
	missing := observerJSONValue(t, filepath.Join(dir, "missing-private-token"))
	// Even an earlier destination must not resolve its token before a later
	// duplicate is refused. The error is fixed, not a missing private path.
	writePrivateFile(t, configPath, `{"version":1,"observers":[{"name":"first","url":"https://first.example.test","tokenFile":`+missing+`},{"name":"second","url":"https://second.example.test","tokenFile":`+missing+`,"provisionProxmox":false,"PROVISIONPROXMOX":true}]}`)
	got, err := LoadObservers(configPath, "https://primary.example.test")
	if got != nil || err == nil || err.Error() != "observer config contains repeated JSON fields; use each field once" {
		t.Fatalf("token resolution preceded complete duplicate validation: %v", err)
	}
}

func TestLoadObserversPreservesSingletonJSONAliases(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "observers.json")
	tokenPath := filepath.Join(dir, "observer.token")
	writePrivateFile(t, tokenPath, "synthetic-observer-token")
	fields := map[string]any{"name": "dev", "url": "http://192.168.50.10:7655", "tokenFile": tokenPath, "insecureSkipVerify": true, "allowPlaintextHTTP": true, "caCertPath": "/synthetic/ca.pem", "serverFingerprint": "synthetic-pin", "provisionProxmox": false}
	writePrivateFile(t, configPath, observerJSONValue(t, map[string]any{"version": 1, "observers": []any{fields}}))
	want, err := LoadObservers(configPath, "https://primary.example.test")
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"upper", "unicode-fold", "escaped"} {
		t.Run(alias, func(t *testing.T) {
			body := observerJSONValue(t, map[string]any{"version": 1, "observers": []any{fields}})
			for _, name := range []string{"version", "observers", "name", "url", "tokenFile", "insecureSkipVerify", "allowPlaintextHTTP", "caCertPath", "serverFingerprint", "provisionProxmox"} {
				key := strings.ToUpper(name)
				if alias == "unicode-fold" {
					key = strings.ReplaceAll(key, "S", "ſ")
				} else if alias == "escaped" {
					key = fmt.Sprintf(`\u%04x%s`, name[0], name[1:])
				}
				body = strings.ReplaceAll(body, `"`+name+`":`, `"`+key+`":`)
			}
			writePrivateFile(t, configPath, body)
			got, err := LoadObservers(configPath, "https://primary.example.test")
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("singleton alias changed destination choices: %v", err)
			}
		})
	}
}

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
