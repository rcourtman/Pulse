package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const savedAuthRecordToken = "synthetic-saved-incomplete-inventory"

func authRecordFixture(kind, raw string) (current, legacy map[string]any, identities []string) {
	current = map[string]any{"expires_at": "2099-01-01T00:00:00Z"}
	legacy = map[string]any{"expires_at": "2099-01-01T00:00:00Z"}
	switch kind {
	case "sessions":
		current["key"], current["username"] = sessionHash(raw), "fixture-user"
		legacy["username"] = "fixture-user"
		identities = []string{"key"}
	case "csrf_tokens":
		current["session_key"], current["token_hash"] = csrfSessionKey(raw), csrfTokenHash(raw)
		legacy["session_id"], legacy["token"] = raw, raw
		identities = []string{"session_key", "token_hash"}
	case "recovery_tokens":
		current["token_hash"], current["ip"] = recoveryTokenHash(raw), "loopback"
		legacy["token"], legacy["ip"] = raw, "loopback"
		identities = []string{"token_hash"}
	}
	return
}

func authRecordBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func authIncompleteRecordCases(kind string) map[string]any {
	current, legacy, identities := authRecordFixture(kind, savedAuthRecordToken)
	cases := map[string]any{}
	addCurrent := func(name string, record any) {
		cases["hashed/"+name+"/only"] = []any{record}
		cases["hashed/"+name+"/valid-first"] = []any{current, record}
		cases["hashed/"+name+"/valid-last"] = []any{record, current}
	}
	addCurrent("null-row", nil)
	addCurrent("empty-record", map[string]any{})
	for _, identity := range identities {
		for _, shape := range []string{"missing", "null", "empty", "blank"} {
			record, _, _ := authRecordFixture(kind, savedAuthRecordToken+"-incomplete")
			switch shape {
			case "missing":
				delete(record, identity)
			case "null":
				record[identity] = nil
			case "empty":
				record[identity] = ""
			case "blank":
				record[identity] = " \t\r\n"
			}
			addCurrent(identity+"-"+shape, record)
		}
	}
	// Legacy credentials can live in a map key or in the existing fallback
	// fields. Validate that effective identity, without changing precedence.
	addLegacy := func(name, key string, record any) {
		cases["legacy/"+name+"/only"] = map[string]any{key: record}
		cases["legacy/"+name+"/mixed"] = map[string]any{savedAuthRecordToken: legacy, key: record}
	}
	addLegacy("null-row", savedAuthRecordToken+"-incomplete", nil)
	legacyShapes := []string{"missing", "null", "empty", "blank"}
	if kind == "sessions" {
		legacyShapes = []string{"empty", "blank"}
	}
	for _, shape := range legacyShapes {
		_, record, _ := authRecordFixture(kind, savedAuthRecordToken+"-incomplete")
		key, field := "", "token"
		if kind == "csrf_tokens" {
			key = savedAuthRecordToken + "-incomplete"
		}
		if kind == "sessions" {
			// Sessions have no credential-field fallback for an empty map key.
			if shape == "blank" {
				key = " \t\r\n"
			}
		} else {
			switch shape {
			case "missing":
				delete(record, field)
			case "null":
				record[field] = nil
			case "empty":
				record[field] = ""
			case "blank":
				record[field] = " \t\r\n"
			}
		}
		addLegacy("credential-"+shape, key, record)
	}
	if kind == "csrf_tokens" {
		for _, value := range []string{"", " \t\r\n"} {
			_, record, _ := authRecordFixture(kind, savedAuthRecordToken+"-incomplete")
			record["session_id"] = value
			addLegacy("binding-"+map[bool]string{true: "blank", false: "empty"}[value != ""], "", record)
		}
	}
	return cases
}

func authRecordPreservationPattern(kind string) string {
	return map[string]string{"sessions": ".sessions-unread-*", "csrf_tokens": ".csrf-tokens-unread-*", "recovery_tokens": ".recovery-tokens-unread-*"}[kind]
}

func checkAuthRecordPreserved(t *testing.T, kind, dir string, original os.FileInfo, want []byte) {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(dir, authRecordPreservationPattern(kind)))
	if err != nil || len(dirs) != 1 {
		t.Fatalf("expected exactly one preserved inventory, got %d: %v", len(dirs), err)
	}
	info, err := os.Stat(dirs[0])
	if err != nil || !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm() != 0700) {
		t.Fatal("preservation directory is not owner-private")
	}
	entries, err := os.ReadDir(dirs[0])
	if err != nil || len(entries) != 1 || entries[0].Name() != kind+".json" {
		t.Fatal("preservation directory contains unexpected files")
	}
	path := filepath.Join(dirs[0], kind+".json")
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("incomplete inventory bytes were changed or lost")
	}
	info, err = os.Stat(path)
	if err != nil || !os.SameFile(original, info) || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatal("incomplete inventory inode was not retained owner-private")
	}
}

func issueAuthRecord(t *testing.T, s startupAuthInventory, binding string) string {
	t.Helper()
	switch {
	case s.session != nil:
		s.session.CreateSession(binding, time.Hour, "fixture", "127.0.0.1", "current-user")
		return binding
	case s.csrf != nil:
		return s.csrf.GenerateCSRFToken(binding)
	default:
		raw, err := s.recovery.GenerateRecoveryToken(time.Hour, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
}

func authRecordValid(s startupAuthInventory, binding, raw string) bool {
	switch {
	case s.session != nil:
		return s.session.ValidateSession(raw)
	case s.csrf != nil:
		return s.csrf.ValidateCSRFToken(binding, raw)
	default:
		return s.recovery.IsRecoveryTokenValidConstantTime(raw, "127.0.0.1")
	}
}

func TestAuthInventoryIncompleteRecordsPreserveOriginal(t *testing.T) {
	for _, kind := range []string{"sessions", "csrf_tokens", "recovery_tokens"} {
		for name, input := range authIncompleteRecordCases(kind) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, kind+".json")
				data := authRecordBytes(t, input)
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
				original, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				logs := captureTestLogs(t)
				s := startAuthInventory(kind, dir)
				t.Cleanup(s.shutdown)
				s.requireUnread(t)
				// All rows must be checked before loading any valid prefix or
				// publishing a lossy legacy migration.
				got, err := os.ReadFile(path)
				info, statErr := os.Stat(path)
				if err != nil || statErr != nil || !bytes.Equal(got, data) || !os.SameFile(original, info) {
					t.Fatal("startup migrated or changed an incomplete inventory")
				}
				for _, secret := range []string{savedAuthRecordToken, sessionHash(savedAuthRecordToken), csrfTokenHash(savedAuthRecordToken)} {
					if strings.Contains(logs.String(), secret) {
						t.Fatal("incomplete-inventory diagnostic disclosed credential identity")
					}
				}
				fresh := issueAuthRecord(t, s, "synthetic-fresh-record")
				control := issueAuthRecord(t, s, "synthetic-control-record")
				if fresh == "" || control == "" || !authRecordValid(s, "synthetic-fresh-record", fresh) || !authRecordValid(s, "synthetic-control-record", control) {
					t.Fatal("preservation blocked current authenticated authority")
				}
				checkAuthRecordPreserved(t, kind, dir, original, data)
				switch {
				case s.session != nil:
					s.session.DeleteSession(fresh)
				case s.csrf != nil:
					s.csrf.DeleteCSRFToken("synthetic-fresh-record")
				default:
					if !s.recovery.ValidateRecoveryTokenConstantTime(fresh, "127.0.0.1") {
						t.Fatal("current recovery token could not be consumed durably")
					}
				}
				s.shutdown()
				restarted := startAuthInventory(kind, dir)
				defer restarted.shutdown()
				if authRecordValid(restarted, savedAuthRecordToken, savedAuthRecordToken) || authRecordValid(restarted, "synthetic-fresh-record", fresh) || !authRecordValid(restarted, "synthetic-control-record", control) {
					t.Fatal("restart revived unknown/withdrawn authority or lost independent current authority")
				}
				checkAuthRecordPreserved(t, kind, dir, original, data)
			})
		}
	}
}

func TestAuthInventoryCompleteRecordsKeepCompatibility(t *testing.T) {
	for _, kind := range []string{"sessions", "csrf_tokens", "recovery_tokens"} {
		current, legacy, _ := authRecordFixture(kind, savedAuthRecordToken)
		expired, expiredLegacy, _ := authRecordFixture(kind, savedAuthRecordToken+"-expired")
		expired["expires_at"], expiredLegacy["expires_at"] = "2020-01-01T00:00:00Z", "2020-01-01T00:00:00Z"
		cases := map[string]any{
			"null": nil, "empty-array": []any{}, "empty-object": map[string]any{},
			"hashed": []any{current, expired},
			"legacy": map[string]any{savedAuthRecordToken: legacy, savedAuthRecordToken + "-expired": expiredLegacy},
		}
		if kind != "sessions" {
			cases["legacy-fallback"] = map[string]any{"": legacy}
			_, keyed, _ := authRecordFixture(kind, savedAuthRecordToken)
			fallbackField := "token"
			if kind == "csrf_tokens" {
				fallbackField = "session_id"
			}
			keyed[fallbackField] = "synthetic-unused-fallback"
			cases["legacy-key-precedence"] = map[string]any{savedAuthRecordToken: keyed}
			_, keyed, _ = authRecordFixture(kind, savedAuthRecordToken)
			delete(keyed, fallbackField)
			cases["legacy-no-redundant-fallback"] = map[string]any{savedAuthRecordToken: keyed}
		}
		for name, input := range cases {
			t.Run(kind+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, kind+".json")
				if err := os.WriteFile(path, authRecordBytes(t, input), 0600); err != nil {
					t.Fatal(err)
				}
				s := startAuthInventory(kind, dir)
				t.Cleanup(s.shutdown)
				wantSaved := strings.HasPrefix(name, "hashed") || strings.HasPrefix(name, "legacy")
				if authRecordValid(s, savedAuthRecordToken, savedAuthRecordToken) != wantSaved || authRecordValid(s, savedAuthRecordToken+"-expired", savedAuthRecordToken+"-expired") {
					t.Fatal("complete saved/expired credential admission changed")
				}
				fresh := issueAuthRecord(t, s, "synthetic-current-record")
				s.shutdown()
				restarted := startAuthInventory(kind, dir)
				defer restarted.shutdown()
				if !authRecordValid(restarted, "synthetic-current-record", fresh) || authRecordValid(restarted, savedAuthRecordToken, savedAuthRecordToken) != wantSaved {
					t.Fatal("complete inventories stopped persisting current/saved authority")
				}
				dirs, err := filepath.Glob(filepath.Join(dir, authRecordPreservationPattern(kind)))
				if err != nil || len(dirs) != 0 {
					t.Fatal("complete inventory was unnecessarily preserved")
				}
			})
		}
	}
}

func TestAuthInventoryIncompleteRecordsHoldFailedPreservation(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires ordinary-user POSIX directory permissions")
	}
	for _, kind := range []string{"sessions", "csrf_tokens", "recovery_tokens"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, kind+".json")
			data := authRecordBytes(t, authIncompleteRecordCases(kind)["hashed/null-row/valid-first"])
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			s := startAuthInventory(kind, dir)
			t.Cleanup(s.shutdown)
			// The session crypto initializer may harden directory permissions;
			// impose the storage failure only after actual initialization.
			t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			if err := os.Chmod(dir, 0500); err != nil {
				t.Fatal(err)
			}
			s.exerciseHeldWrites(t)
			got, err := os.ReadFile(path)
			info, statErr := os.Stat(path)
			if err != nil || statErr != nil || !bytes.Equal(got, data) || !os.SameFile(original, info) {
				t.Fatal("failed preservation replaced the incomplete inventory")
			}
			dirs, err := filepath.Glob(filepath.Join(dir, authRecordPreservationPattern(kind)))
			if err != nil || len(dirs) != 0 {
				t.Fatal("failed preservation left a partial retained inventory")
			}
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			fresh := issueAuthRecord(t, s, "synthetic-after-storage-repair")
			s.shutdown()
			restarted := startAuthInventory(kind, dir)
			defer restarted.shutdown()
			if !authRecordValid(restarted, "synthetic-after-storage-repair", fresh) || authRecordValid(restarted, savedAuthRecordToken, savedAuthRecordToken) {
				t.Fatal("storage recovery lost fresh authority or revived the rejected inventory")
			}
			checkAuthRecordPreserved(t, kind, dir, original, data)
		})
	}
}
