package secretredact

import (
	"strings"
	"testing"
)

// The rules are the ones the server applied to text bound for a model before
// they moved here. The server's own tests still run them through its wrapper;
// these hold the package's contract for the agent, which masks log lines on
// the host with nothing else in front of it.
func TestTextMasksCredentialsAndLeavesTheRest(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		secrets  []string
		keeps    []string
	}{
		{"key and value", `level=error msg="login failed" DB_PASSWORD=hunter2 retry=3`, []string{"hunter2"}, []string{"level=error"}},
		{"credentials in a URL", "dial postgres://vault:s3cr3t@10.0.0.5:5432/vault: connection refused", []string{"s3cr3t"}, []string{"10.0.0.5:5432", "connection refused"}},
		{"bearer header", "GET /api Authorization: Bearer abcDEF123456.token-value", []string{"abcDEF123456"}, []string{"GET /api"}},
		{"query string", "fetch https://example.test/hook?token=tok-abc123&x=1 failed", []string{"tok-abc123"}, []string{"example.test/hook"}},
		{"provider key", "using key sk-live0123456789abcdef for upstream", []string{"sk-live0123456789abcdef"}, []string{"for upstream"}},
		{"private key block", "loading key\n-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASC\n-----END PRIVATE KEY-----\nstarted", []string{"MIIEvQIBADANBgkqhkiG9w0BAQEFAASC"}, []string{"loading key", "started"}},
		{"nothing secret", `[vaultwarden::db][ERROR] invalid DATABASE_URL "10.0.0.5:5432/vault": relative URL without a base`, nil, []string{"invalid DATABASE_URL", "relative URL without a base"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, count := Text(tc.in)
			for _, secret := range tc.secrets {
				if strings.Contains(out, secret) {
					t.Fatalf("%q survived: %q", secret, out)
				}
			}
			for _, keep := range tc.keeps {
				if !strings.Contains(out, keep) {
					t.Fatalf("%q was lost: %q", keep, out)
				}
			}
			if (count > 0) != (len(tc.secrets) > 0) {
				t.Fatalf("redaction count = %d for %d secrets: %q", count, len(tc.secrets), out)
			}
			// A key and value use Placeholder; a provider key and a PEM block
			// say what they were.
			if len(tc.secrets) > 0 && !strings.Contains(out, "[REDACTED") {
				t.Fatalf("no placeholder in %q", out)
			}
		})
	}
	if out, count := Text(""); out != "" || count != 0 {
		t.Fatalf("empty input gave %q, %d", out, count)
	}
}
