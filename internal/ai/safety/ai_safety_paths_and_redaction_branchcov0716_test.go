package safety

import (
	"strings"
	"testing"
)

// This file adds branch-coverage tests for the path- and redaction-related
// helpers in package safety. It targets branches that the existing test files
// leave uncovered: empty/zero inputs, every switch arm (including defaults),
// error/short-circuit returns, normalization edge cases, and the multi-arm
// PEM/kv redaction state machine in RedactSensitiveText / RedactSensitiveValue.

func TestBranchCovIsSensitivePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantSense   bool
		wantReasons []string // any of these substrings is acceptable
	}{
		// Empty / zero input -> early return.
		{"empty path", "", false, nil},

		// System credential databases: switch-case arms.
		{"etc shadow", "/etc/shadow", true, []string{"system credential file"}},
		{"etc gshadow", "/etc/gshadow", true, []string{"system credential file"}},
		{"etc sudoers", "/etc/sudoers", true, []string{"system credential file"}},

		// filepath.Clean + ToLower normalization paths.
		{"cleaned dotdot shadow", "/etc/../etc/shadow", true, []string{"system credential file"}},
		{"trailing slash shadow", "/etc/shadow/", true, []string{"system credential file"}},
		{"uppercase shadow lowercased", "/ETC/SHADOW", true, []string{"system credential file"}},
		{"mixed-case Sudoers", "/EtC/SuDoErS", true, []string{"system credential file"}},

		// SSH directory contains-arm.
		{"ssh dir config", "/home/u/.ssh/config", true, []string{"ssh key/config directory"}},
		{"ssh dir anywhere", "/root/.ssh/random", true, []string{"ssh key/config directory"}},

		// SSH key suffix loop, every name. NOTE: these paths deliberately do NOT
		// contain "/.ssh/" so they fall through the contains-arm and reach the
		// HasSuffix("/<name>") arm (otherwise the contains-arm wins and returns
		// "ssh key/config directory" first).
		{"ssh id_rsa suffix", "/tmp/id_rsa", true, []string{"ssh key material"}},
		{"ssh id_ed25519 suffix", "/tmp/id_ed25519", true, []string{"ssh key material"}},
		{"ssh authorized_keys suffix", "/root/authorized_keys", true, []string{"ssh key material"}},
		{"ssh known_hosts suffix", "/root/known_hosts", true, []string{"ssh key material"}},

		// Bare relative ssh filename (no slash) does NOT match the suffix arm.
		{"bare id_rsa no slash", "id_rsa", false, nil},

		// Secrets directory prefix loop, every prefix.
		{"run secrets prefix", "/run/secrets/db", true, []string{"secrets directory"}},
		{"var run secrets prefix", "/var/run/secrets/db", true, []string{"secrets directory"}},
		{"etc secrets prefix", "/etc/secrets/db", true, []string{"secrets directory"}},
		{"secrets root prefix", "/secrets/db", true, []string{"secrets directory"}},

		// /proc/<pid>/environ combined predicate.
		{"proc environ", "/proc/1/environ", true, []string{"process environment file"}},
		// Only one half of the /proc/ + /environ predicate -> not sensitive.
		{"proc without environ", "/proc/1/status", false, nil},
		{"environ without proc", "/tmp/environ", false, nil},

		// Private key / cert extension loop, every ext.
		{"pem ext", "/srv/tls/server.pem", true, []string{"private key or certificate file"}},
		{"key ext", "/srv/tls/server.key", true, []string{"private key or certificate file"}},
		{"p12 ext", "/srv/cert.p12", true, []string{"private key or certificate file"}},
		{"pfx ext", "/srv/cert.pfx", true, []string{"private key or certificate file"}},

		// ai.enc store: HasSuffix OR Contains arms.
		{"ai enc suffix", "/var/lib/pulse/ai.enc", true, []string{"pulse encrypted AI provider config store"}},
		{"ai enc contains embedded", "/tmp/ai.enc.bak", true, []string{"pulse encrypted AI provider config store"}},

		// Credentials dotfile suffix loop, every base name.
		{"env dotfile", "/app/.env", true, []string{"credentials dotfile"}},
		{"npmrc dotfile", "/home/u/.npmrc", true, []string{"credentials dotfile"}},
		{"pypirc dotfile", "/home/u/.pypirc", true, []string{"credentials dotfile"}},
		{"netrc dotfile", "/home/u/.netrc", true, []string{"credentials dotfile"}},
		{"aws credentials", "/home/u/.aws/credentials", true, []string{"credentials dotfile"}},
		// Bare relative ".env" (no slash) -> suffix arm does NOT match.
		{"bare env no slash", ".env", false, nil},

		// Wholly benign path -> final return (false, "").
		{"benign readme", "/srv/app/README.md", false, nil},
		{"benign source", "/srv/app/main.go", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := IsSensitivePath(tt.path)
			if got != tt.wantSense {
				t.Fatalf("IsSensitivePath(%q) sense = %v, want %v (reason=%q)", tt.path, got, tt.wantSense, reason)
			}
			if !tt.wantSense {
				if reason != "" {
					t.Errorf("IsSensitivePath(%q) benign returned reason %q, want empty", tt.path, reason)
				}
				return
			}
			// For sensitive hits, reason must be non-empty and contain an expected substring.
			if reason == "" {
				t.Fatalf("IsSensitivePath(%q) returned true with empty reason", tt.path)
			}
			matched := false
			for _, want := range tt.wantReasons {
				if strings.Contains(reason, want) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("IsSensitivePath(%q) reason = %q, want one of %v", tt.path, reason, tt.wantReasons)
			}
		})
	}
}

func TestBranchCovCommandTouchesSensitivePath(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		wantSense   bool
		wantReasons []string
	}{
		// Empty command -> early return. (Lowercasing "" still yields "".)
		{"empty command", "", false, nil},

		// Each high-confidence substring arm of the loop.
		{"etc shadow substring", "cat /etc/shadow", true, []string{"references sensitive path"}},
		{"etc gshadow substring", "cat /etc/gshadow", true, []string{"references sensitive path"}},
		{"etc sudoers substring", "grep root /etc/sudoers", true, []string{"references sensitive path"}},
		{"run secrets substring", "ls /run/secrets/db", true, []string{"references sensitive path"}},
		{"var run secrets substring", "ls /var/run/secrets/db", true, []string{"references sensitive path"}},
		{"ssh substring", "cat /home/u/.ssh/id_rsa", true, []string{"references sensitive path"}},
		{"ai enc substring", "cp /var/lib/pulse/ai.enc /tmp", true, []string{"references sensitive path"}},

		// Case-insensitivity: input is lowercased before matching.
		{"uppercase etc shadow", "CAT /ETC/SHADOW", true, []string{"references sensitive path"}},

		// Combined /proc/ + environ predicate: both substrings required.
		{"proc environ combined", "cat /proc/42/environ", true, []string{"references process environment file"}},
		{"proc only not environ", "cat /proc/42/status", false, nil},
		{"environ only not proc", "cat /tmp/environ", false, nil},

		// Benign command -> final return (false, "").
		{"benign ls", "ls -la /tmp", false, nil},
		{"benign ps", "ps aux", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := CommandTouchesSensitivePath(tt.cmd)
			if got != tt.wantSense {
				t.Fatalf("CommandTouchesSensitivePath(%q) sense = %v, want %v (reason=%q)", tt.cmd, got, tt.wantSense, reason)
			}
			if !tt.wantSense {
				if reason != "" {
					t.Errorf("CommandTouchesSensitivePath(%q) benign returned reason %q, want empty", tt.cmd, reason)
				}
				return
			}
			if reason == "" {
				t.Fatalf("CommandTouchesSensitivePath(%q) returned true with empty reason", tt.cmd)
			}
			matched := false
			for _, want := range tt.wantReasons {
				if strings.Contains(reason, want) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("CommandTouchesSensitivePath(%q) reason = %q, want one of %v", tt.cmd, reason, tt.wantReasons)
			}
		})
	}
}

func TestBranchCovIsSensitiveValueCarrierFieldName(t *testing.T) {
	positive := []string{
		"value", "values", "default", "defaults", "example", "examples", "enum", "const",
	}
	negative := []string{
		// Empty / whitespace-only -> normalized "" -> switch default.
		"",
		"   ",
		"\t\n",
		// Unrelated names -> default arm.
		"description",
		"username",
		"type",
		// Looks related but normalizes to something not in the switch.
		"defaultvalue", // default + value concatenated
		"exampleurl",
	}

	for _, n := range positive {
		if !IsSensitiveValueCarrierFieldName(n) {
			t.Errorf("IsSensitiveValueCarrierFieldName(%q) = false, want true", n)
		}
	}
	for _, n := range negative {
		if IsSensitiveValueCarrierFieldName(n) {
			t.Errorf("IsSensitiveValueCarrierFieldName(%q) = true, want false", n)
		}
	}

	// Normalization: case-insensitive and punctuation stripping.
	normalizations := map[string]string{
		"VALUE":    "value",
		"Defaults": "defaults",
		"Example":  "example",
		"  enum  ": "enum",
		"ex-ample": "example", // hyphen stripped -> "example" matches
	}
	for raw, want := range normalizations {
		// Both the raw and the equivalent canonical form must classify identically.
		if IsSensitiveValueCarrierFieldName(raw) != IsSensitiveValueCarrierFieldName(want) {
			t.Errorf("normalization mismatch: raw=%q canonical=%q classify differently", raw, want)
		}
		if !IsSensitiveValueCarrierFieldName(raw) {
			t.Errorf("IsSensitiveValueCarrierFieldName(%q) = false, want true (canonical %q)", raw, want)
		}
	}
}

func TestBranchCovIsSensitiveFieldName(t *testing.T) {
	positive := []string{
		"password", "passwd", "passphrase", "secret", "token", "apikey",
		"clientsecret", "privatekey", "accesstoken", "refreshtoken",
		"authorization", "xapikey", "credential", "credentials",
	}
	negative := []string{
		// Empty -> early return (normalized == "").
		"",
		"   ",
		// Unrelated names -> switch default.
		"username",
		"displayname",
		"email",
		// Sensitive-looking but normalizes away from the list.
		"secretpassword", // concatenated, not in list
	}

	for _, n := range positive {
		if !IsSensitiveFieldName(n) {
			t.Errorf("IsSensitiveFieldName(%q) = false, want true", n)
		}
	}
	for _, n := range negative {
		if IsSensitiveFieldName(n) {
			t.Errorf("IsSensitiveFieldName(%q) = true, want false", n)
		}
	}

	// Normalization: punctuation/case-insensitivity maps to canonical arms.
	cases := map[string]string{
		"API-Key":       "apikey",
		"api_key":       "apikey",
		"ClientSecret":  "clientsecret",
		"client-secret": "clientsecret",
		"X-API-Key":     "xapikey",
		"Password":      "password",
		"password!!!":   "password", // non-alnum stripped
		"  token  ":     "token",
	}
	for raw, want := range cases {
		if IsSensitiveFieldName(raw) != IsSensitiveFieldName(want) {
			t.Errorf("normalization mismatch: raw=%q canonical=%q classify differently", raw, want)
		}
		if !IsSensitiveFieldName(raw) {
			t.Errorf("IsSensitiveFieldName(%q) = false, want true (canonical %q)", raw, want)
		}
	}
}

func TestBranchCovRedactSensitiveText(t *testing.T) {
	// Empty input -> early return ("", 0).
	t.Run("empty input returns empty", func(t *testing.T) {
		out, n := RedactSensitiveText("")
		if out != "" || n != 0 {
			t.Fatalf("RedactSensitiveText(\"\") = (%q, %d), want (\"\", 0)", out, n)
		}
	})

	// Newline/whitespace-only input: no redactions. The "drop empty lines" loop
	// only drops *truly-empty* lines; whitespace-only lines (e.g. "  ") survive.
	// For input "\n\n  \n" the surviving line is "  ", so output is "  " (count 0).
	t.Run("whitespace lines are not secret and pass through", func(t *testing.T) {
		out, n := RedactSensitiveText("\n\n  \n")
		if n != 0 {
			t.Fatalf("RedactSensitiveText(%q) count = %d, want 0", "\n\n  \n", n)
		}
		if out != "  " {
			t.Errorf("RedactSensitiveText(%q) = %q, want %q (only truly-empty lines are dropped)", "\n\n  \n", out, "  ")
		}
	})

	// kvSecretRE arm: only the value portion is replaced, key context preserved.
	t.Run("kv secret value redacted key preserved", func(t *testing.T) {
		out, n := RedactSensitiveText("password: hunter2")
		if n != 1 {
			t.Fatalf("expected 1 redaction, got %d (%q)", n, out)
		}
		if strings.Contains(out, "hunter2") {
			t.Errorf("value leaked: %q", out)
		}
		if !strings.HasPrefix(out, "password:") || !strings.Contains(out, "[REDACTED]") {
			t.Errorf("key context lost: %q", out)
		}
	})

	// PEM state machine: begin marker -> "[REDACTED PEM BLOCK]", body dropped,
	// end marker reached -> inPEM=false and the marker line is also dropped.
	t.Run("pem block begin body and end", func(t *testing.T) {
		input := strings.Join([]string{
			"prelude",
			"-----BEGIN PRIVATE KEY-----",
			"ZmFrZS1iYXNlNjQtcGVtLmJvZHk=",
			"-----END PRIVATE KEY-----",
			"epilogue",
		}, "\n")
		out, n := RedactSensitiveText(input)
		if n != 1 {
			t.Fatalf("expected exactly 1 redaction (begin marker), got %d (%q)", n, out)
		}
		if !strings.Contains(out, "[REDACTED PEM BLOCK]") {
			t.Errorf("missing PEM block marker: %q", out)
		}
		// Both base64 body and END marker must be stripped.
		if strings.Contains(out, "ZmFrZS1iYXNlNjQtcGVtLmJvZHk=") {
			t.Errorf("PEM body leaked: %q", out)
		}
		if strings.Contains(out, "END PRIVATE KEY") {
			t.Errorf("PEM END marker should be dropped, present in %q", out)
		}
		// Lines outside the block are retained.
		if !strings.Contains(out, "prelude") || !strings.Contains(out, "epilogue") {
			t.Errorf("non-PEM lines dropped: %q", out)
		}
	})

	// PEM state machine: unterminated block -> inPEM stays true to EOF,
	// every subsequent line (including a would-be secret kv line) is blanked.
	t.Run("unterminated pem swallows rest", func(t *testing.T) {
		input := strings.Join([]string{
			"-----BEGIN CERTIFICATE-----",
			"line1",
			"password: should-not-be-processed-by-kv-path",
		}, "\n")
		out, n := RedactSensitiveText(input)
		if n != 1 {
			t.Fatalf("expected exactly 1 redaction, got %d (%q)", n, out)
		}
		if !strings.Contains(out, "[REDACTED PEM BLOCK]") {
			t.Errorf("missing PEM block marker: %q", out)
		}
		// The kv line inside the unterminated PEM is blanked, NOT kv-redacted.
		if strings.Contains(out, "should-not-be-processed-by-kv-path") {
			t.Errorf("PEM body line leaked: %q", out)
		}
		if strings.Contains(out, "[REDACTED]") {
			t.Errorf("kv redactor should not run inside PEM block: %q", out)
		}
	})

	// redactLineSecretPatterns: each provider-token regex produces its own marker.
	// Covers AWS, JWT, OpenAI-style, Google API key, GitHub token arms.
	t.Run("provider token patterns", func(t *testing.T) {
		cases := []struct {
			name string
			line string
		}{
			{"aws access key", "id=AKIAIOSFODNN7EXAMPLE"},
			{"jwt", "tok=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"},
			{"openai key", "k=sk-abcdef123456"},
			{"google api key", "k=AIzaSyABCdefghIJKlmnoPQRstuVWxyz1234567"},
			{"github token", "t=ghp_abcdefghijklmnopqrstuvwxyz"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				out, n := RedactSensitiveText(c.line)
				if n == 0 {
					t.Fatalf("expected a redaction, got 0 for %q", c.line)
				}
				if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") && c.name == "aws access key" {
					t.Errorf("aws key leaked: %q", out)
				}
				// All markers are documented as bracketed; ensure no raw secret remains
				// and that the result contains a "[REDACTED" token.
				if !strings.Contains(out, "[REDACTED") {
					t.Errorf("missing redaction marker in %q", out)
				}
			})
		}
	})

	// Header / URL / quoted-JSON arms (distinct from kvSecretRE).
	t.Run("header and url and json forms", func(t *testing.T) {
		input := strings.Join([]string{
			`Authorization: Bearer abc123def456`,
			`x-api-key: somekeyvalue`,
			`https://user:pass@example.test/`,
			`{"secret":"shh"}`,
			`GET https://example.test/v1?key=AIzaSyAbcDefGhiJklMnoPqrStuVwxYz1234567`,
		}, "\n")
		out, n := RedactSensitiveText(input)
		if n == 0 {
			t.Fatalf("expected redactions, got 0")
		}
		for _, leak := range []string{"abc123def456", "somekeyvalue", "user:pass@", "shh", "AIzaSyAbcDefGhiJklMnoPqrStuVwxYz1234567"} {
			if strings.Contains(out, leak) {
				t.Errorf("leaked %q in %q", leak, out)
			}
		}
	})

	// No-match text passes through unchanged with count 0.
	t.Run("no secrets returns unchanged", func(t *testing.T) {
		in := "just a regular log line with no secrets"
		out, n := RedactSensitiveText(in)
		if out != in || n != 0 {
			t.Fatalf("RedactSensitiveText(%q) = (%q, %d), want unchanged/0", in, out, n)
		}
	})
}

func TestBranchCovRedactSensitiveValue(t *testing.T) {
	// Branch: input "" -> RedactSensitiveText returns ("", 0); TrimSpace == "" -> return as-is.
	t.Run("empty input", func(t *testing.T) {
		out, n := RedactSensitiveValue("")
		if out != "" || n != 0 {
			t.Fatalf("RedactSensitiveValue(\"\") = (%q, %d), want (\"\", 0)", out, n)
		}
	})

	// Branch: redacted text is whitespace-only (non-empty but trims to "").
	// RedactSensitiveText leaves the surviving whitespace line intact ("  "),
	// and the TrimSpace(redacted)=="" arm returns that untrimmed value with the
	// inner count (0) WITHOUT forcing the [REDACTED] marker.
	t.Run("whitespace only trims empty short circuits", func(t *testing.T) {
		out, n := RedactSensitiveValue("\n\n  \n")
		if n != 0 {
			t.Fatalf("RedactSensitiveValue(%q) count = %d, want 0", "\n\n  \n", n)
		}
		if out != "  " {
			t.Errorf("RedactSensitiveValue(%q) = %q, want %q (TrimSpace arm preserves untrimmed redacted text)", "\n\n  \n", out, "  ")
		}
	})

	// Branch: redacted already equals the canonical marker -> returned unchanged,
	// count NOT incremented. Achievable by feeding the literal marker string.
	t.Run("already redacted marker", func(t *testing.T) {
		out, n := RedactSensitiveValue("[REDACTED]")
		if out != "[REDACTED]" {
			t.Fatalf("RedactSensitiveValue(\"[REDACTED]\") = %q, want \"[REDACTED]\"", out)
		}
		if n != 0 {
			t.Errorf("expected count 0 for already-redacted marker, got %d", n)
		}
	})

	// Branch: non-empty, non-marker plain value -> final arm forces the marker
	// and increments count by 1 (key-context collapses any non-secret value).
	t.Run("plain value collapsed to marker", func(t *testing.T) {
		out, n := RedactSensitiveValue("just a plain string, nothing secret-shaped")
		if out != "[REDACTED]" {
			t.Fatalf("RedactSensitiveValue(plain) = %q, want \"[REDACTED]\"", out)
		}
		if n != 1 {
			t.Errorf("expected count 1 for forced collapse, got %d", n)
		}
	})

	// A value that itself contains a redactable token: count reflects the inner
	// redaction (>=1) PLUS the final collapse increment.
	t.Run("value with embedded token then collapsed", func(t *testing.T) {
		out, n := RedactSensitiveValue("token=ghp_abcdefghijklmnopqrstuvwxyz")
		if out != "[REDACTED]" {
			t.Fatalf("RedactSensitiveValue(embedded) = %q, want \"[REDACTED]\"", out)
		}
		if n < 2 {
			t.Errorf("expected count >= 2 (inner redaction + collapse), got %d", n)
		}
	})
}
