package collectorlifecycle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalsecurity "github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func TestPrivateValueFileRejectsNULWithoutReturningCredential(t *testing.T) {
	for name, encoded := range map[string]string{
		"only":         "\x00",
		"leading":      "\x00synthetic-secret",
		"interior":     "synthetic\x00-secret",
		"trailing":     "synthetic-secret\x00",
		"before LF":    "synthetic-secret\x00\n",
		"after LF":     "synthetic-secret\n\x00",
		"within limit": strings.Repeat("s", maximumBearerBytes-1) + "\x00",
	} {
		t.Run(name, func(t *testing.T) {
			path := writePrivateValue(t, encoded)
			value, err := ReadPrivateValueFile(path, testTokenOwnerUID())
			if err == nil || value != "" {
				t.Fatal("NUL-containing file returned a value instead of failing closed")
			}
			if err.Error() != "collector lifecycle token file is empty or invalid" {
				t.Fatal("invalid credential did not produce the fixed non-disclosing error")
			}
			client, err := New(Config{PulseURL: "http://127.0.0.1:7655", TokenFile: path, TokenOwnerUID: testTokenOwnerUID()})
			if client != nil || err == nil {
				if client != nil {
					client.Close()
				}
				t.Fatal("NUL-containing file initialized a lifecycle transport")
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, []byte(encoded)) {
				t.Fatal("credential validation changed the caller's file")
			}
		})
	}
}

func TestPrivateValueFileRetainsValidNormalization(t *testing.T) {
	for name, encoded := range map[string]string{
		"bare":       "synthetic-secret",
		"LF":         "synthetic-secret\n",
		"CRLF":       "synthetic-secret\r\n",
		"whitespace": " \tsynthetic-secret\r\n\n",
		"literal":    "PULSE_TOKEN='synthetic-secret'",
		"UTF8":       "synthetic-caf\u00e9",
		"limit":      strings.Repeat("s", maximumBearerBytes),
	} {
		t.Run(name, func(t *testing.T) {
			path := writePrivateValue(t, encoded)
			value, err := ReadPrivateValueFile(path, testTokenOwnerUID())
			if err != nil || value != strings.TrimSpace(encoded) {
				t.Fatal("valid credential bytes or existing whitespace normalization changed")
			}
		})
	}
}

func writePrivateValue(t *testing.T, encoded string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "collector.token")
	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		t.Fatal(err)
	}
	if err := internalsecurity.HardenPrivatePath(path, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
