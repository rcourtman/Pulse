package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func tokenJSONFixture(t *testing.T) *orgTokenFixture {
	t.Helper()
	f := newOrgTokenFixture(t)
	for i := range f.cfg.APITokens {
		if f.cfg.APITokens[i].ID == f.orgB.ID {
			expiry := time.Now().UTC().Add(24 * time.Hour)
			f.cfg.APITokens[i].ExpiresAt = &expiry
			f.cfg.APITokens[i].OrgIDs = []string{"org-b"}
			f.cfg.APITokens[i].Metadata = map[string]string{"fixture": "preserve"}
		}
	}
	if err := f.router.persistence.SaveAPITokens(f.cfg.APITokens); err != nil {
		t.Fatal(err)
	}
	return f
}

func tokenJSONBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func tokenJSONFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[strings.TrimPrefix(path, dir)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func tokenJSONCall(f *orgTokenFixture, operation string, body io.Reader) *httptest.ResponseRecorder {
	method, path, handler := http.MethodPatch, "/api/security/tokens/"+f.orgB.ID, f.router.handleUpdateAPIToken
	if operation == "create" {
		method, path, handler = http.MethodPost, "/api/security/tokens", f.router.handleCreateAPIToken
	} else if operation == "rename" {
		handler = f.router.handleRenameAPIToken
	}
	req := httptest.NewRequest(method, path, body)
	req = req.WithContext(context.WithValue(auth.WithUser(req.Context(), "json-fixture-admin"), OrgIDContextKey, "org-b"))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func assertTokenJSONRefused(t *testing.T, operation string, body io.Reader) {
	t.Helper()
	capture := installTokenMutationAuditProbe(t, func() {})
	f := tokenJSONFixture(t)
	before := tokenJSONBytes(t, f.cfg.APITokens)
	primary := f.cfg.APIToken
	files := tokenJSONFiles(t, f.cfg.DataPath)
	rec := tokenJSONCall(f, operation, body)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "Invalid request body\n" {
		t.Fatalf("malformed mutation returned status %d instead of static 400", rec.Code)
	}
	if !bytes.Equal(before, tokenJSONBytes(t, f.cfg.APITokens)) || f.cfg.APIToken != primary {
		t.Fatal("rejected JSON changed live credentials or legacy primary")
	}
	if !reflect.DeepEqual(files, tokenJSONFiles(t, f.cfg.DataPath)) {
		t.Fatal("rejected JSON changed persistent files")
	}
	reopened, err := config.NewConfigPersistence(f.cfg.DataPath).LoadAPITokens()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, tokenJSONBytes(t, reopened)) {
		t.Fatal("rejected JSON changed the reopened inventory")
	}
	events, err := capture.Query(audit.QueryFilter{})
	if err != nil || len(events) != 1 || events[0].Success {
		t.Fatal("rejected JSON must audit failure, never a successful mutation")
	}
	if strings.Contains(events[0].Details, "private-body-marker") {
		t.Fatal("audit disclosed rejected request material")
	}
}

func TestAPITokenJSONRejectsAmbiguousMutations(t *testing.T) {
	for _, operation := range []string{"create", "update", "rename"} {
		for _, tc := range []struct{ name, body string }{
			{"duplicate scopes", `{"scopes":["monitoring:read"],"scopes":["*"]}`},
			{"identical scopes", `{"scopes":["monitoring:read"],"scopes":["monitoring:read"]}`},
			{"reverse scopes", `{"scopes":["*"],"scopes":["monitoring:read"]}`},
			{"null scope duplicate", `{"name":"private-body-marker","scopes":["monitoring:read"],"scopes":null}`},
			{"escaped scopes", `{"scopes":["monitoring:read"],"scope\u0073":["*"]}`},
			{"folded scopes", `{"scopes":["monitoring:read"],"SCOPES":["*"]}`},
			{"unicode folded scopes", `{"scopes":["monitoring:read"],"scopeſ":["*"]}`},
			{"duplicate name", `{"name":"private-body-marker","name":"other"}`},
			{"escaped name", `{"name":"private-body-marker","nam\u0065":"other"}`},
			{"folded name", `{"name":"private-body-marker","NAME":"other"}`},
			{"null name duplicate", `{"name":"private-body-marker","name":null,"scopes":["monitoring:read"]}`},
			{"second object", `{"name":"private-body-marker","scopes":["*"]}{"scopes":["monitoring:read"]}`},
			{"spaced second object", `{"name":"private-body-marker"} {}`},
			{"trailing junk", `{"name":"private-body-marker"}x`},
			{"trailing whitespace overflow", `{"name":"private-body-marker"}` + strings.Repeat(" ", maxAPITokenRequestBodySize)},
			{"oversized field", `{"name":"` + strings.Repeat("x", maxAPITokenRequestBodySize) + `"}`},
			{"wrong scope type", `{"name":"private-body-marker","scopes":"*"}`},
			{"wrong name type", `{"name":["private-body-marker"]}`},
			{"null root", `null`},
			{"non-JSON whitespace", "\u00a0"},
			{"non-JSON leading whitespace", "\u00a0{}"},
			{"non-JSON trailing whitespace", "{}\u00a0"},
			{"array root", `[]`},
			{"string root", `"private-body-marker"`},
			{"boolean root", `true`},
			{"broken object", `{"name":"private-body-marker"`},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				assertTokenJSONRefused(t, operation, strings.NewReader(tc.body))
			})
		}
	}
	for _, body := range []string{
		`{"expiresIn":"1h","expiresIn":""}`,
		`{"expiresIn":"1h","EXPIRESIN":""}`,
		`{"expiresIn":"1h","expire\u0073In":null}`,
		`{"expiresIn":"1h","expireſIn":""}`,
	} {
		t.Run("create/competing expiry/"+body, func(t *testing.T) {
			assertTokenJSONRefused(t, "create", strings.NewReader(body))
		})
	}
}

type tokenJSONReadError struct {
	*strings.Reader
}

func (r tokenJSONReadError) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF || r.Reader.Len() == 0 {
		return n, errors.New("private-body-marker: failed read")
	}
	return n, err
}

func TestAPITokenJSONRejectsFailedReadsWithoutDisclosure(t *testing.T) {
	// Earlier tests leave monitor goroutines logging, so capture through the
	// package's synchronized sink instead of replacing the global logger.
	logs := captureTestLogs(t)
	for _, operation := range []string{"create", "update", "rename"} {
		for _, body := range []string{`{"name":"private-body-marker","scopes":["*"]}`, "", " \n\t"} {
			t.Run(operation+"/"+body, func(t *testing.T) {
				assertTokenJSONRefused(t, operation, tokenJSONReadError{strings.NewReader(body)})
			})
		}
	}
	captured := logs.String()
	if !strings.Contains(captured, "Failed to decode API token") {
		t.Fatal("decode failures were not captured")
	}
	if strings.Contains(captured, "private-body-marker") {
		t.Fatal("decode log disclosed a request-reader error or body value")
	}
}

func TestAPITokenJSONKeepsCreateDefaultsAndAliases(t *testing.T) {
	installTokenMutationAuditProbe(t, func() {})
	for _, tc := range []struct {
		name, body, wantName string
		wantScopes           []string
		expires              bool
	}{
		{"bodyless", "", "API token", []string{"*"}, false},
		{"whitespace", " \n\t", "API token", []string{"*"}, false},
		{"empty object", `{}`, "API token", []string{"*"}, false},
		{"null fields", `{"name":null,"scopes":null,"expiresIn":null}`, "API token", []string{"*"}, false},
		{"explicit full access", `{"name":"full","scopes":["*"]}`, "full", []string{"*"}, false},
		{"ordinary client", `{"name":"automation","scopes":["monitoring:read"]}`, "automation", []string{"monitoring:read"}, false},
		{"deduplicated scopes", `{"name":"  automation  ","scopes":["settings:read","monitoring:read","settings:read"]}`, "automation", []string{"monitoring:read", "settings:read"}, false},
		{"singleton aliases", `{"NAME":"automation","scope\u0073":["monitoring:read"],"EXPIRESIN":"1h"}`, "automation", []string{"monitoring:read"}, true},
		{"unicode aliases", `{"name":"automation","scopeſ":["monitoring:read"],"expireſIn":"1h"}`, "automation", []string{"monitoring:read"}, true},
		{"unknown fields", `{"name":"automation","scopes":["monitoring:read"],"future":{"scopes":[],"scopes":["*"]},"Future":true}`, "automation", []string{"monitoring:read"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tokenJSONFixture(t)
			before := tokenJSONBytes(t, f.cfg.APITokens)
			rec := tokenJSONCall(f, "create", strings.NewReader(tc.body))
			var response struct {
				Token  string      `json:"token"`
				Record apiTokenDTO `json:"record"`
			}
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &response) != nil || response.Token == "" {
				t.Fatalf("valid create failed: status %d", rec.Code)
			}
			if response.Record.Name != tc.wantName || !reflect.DeepEqual(response.Record.Scopes, tc.wantScopes) || response.Record.OwnerUserID != "json-fixture-admin" {
				t.Fatal("valid create changed names, scope defaults or ownership")
			}
			if (response.Record.ExpiresAt != nil) != tc.expires {
				t.Fatal("valid create changed expiry presence")
			}
			if tc.expires && (time.Until(*response.Record.ExpiresAt) < 59*time.Minute || time.Until(*response.Record.ExpiresAt) > time.Hour) {
				t.Fatal("valid create changed expiry duration")
			}
			var retained []config.APITokenRecord
			for _, record := range f.cfg.APITokens {
				if record.ID == response.Record.ID {
					if record.OrgID != "org-b" {
						t.Fatal("create lost the selected organization")
					}
				} else {
					retained = append(retained, record)
				}
			}
			if !bytes.Equal(before, tokenJSONBytes(t, retained)) || len(f.cfg.APITokens) != 4 {
				t.Fatal("valid create changed neighboring credentials")
			}
			reopened, err := config.NewConfigPersistence(f.cfg.DataPath).LoadAPITokens()
			if err != nil || !bytes.Equal(tokenJSONBytes(t, f.cfg.APITokens), tokenJSONBytes(t, reopened)) {
				t.Fatal("valid create was not durably preserved")
			}
		})
	}
}

func TestAPITokenJSONKeepsEditsAndRenameIdentity(t *testing.T) {
	installTokenMutationAuditProbe(t, func() {})
	for _, tc := range []struct {
		name, body, wantName string
		wantScopes           []string
	}{
		{"rename only", `{"name":"renamed"}`, "renamed", []string{"monitoring:read"}},
		{"scopes only", `{"scopes":["settings:read","monitoring:read"]}`, "org-b", []string{"monitoring:read", "settings:read"}},
		{"full access", `{"scopes":["*"]}`, "org-b", []string{"*"}},
		{"combined", `{"name":"renamed","scopes":["settings:read"]}`, "renamed", []string{"settings:read"}},
		{"null scopes retain", `{"name":"renamed","scopes":null}`, "renamed", []string{"monitoring:read"}},
		{"null name retains", `{"name":null,"scopes":["settings:read"]}`, "org-b", []string{"settings:read"}},
		{"singleton aliases", `{"NAME":"renamed","scope\u0073":["settings:read"]}`, "renamed", []string{"settings:read"}},
		{"unicode alias", `{"name":"renamed","scopeſ":["settings:read"]}`, "renamed", []string{"settings:read"}},
		{"unknown fields", `{"name":"renamed","future":{"name":"ignored","NAME":"ignored"}}`, "renamed", []string{"monitoring:read"}},
	} {
		for _, operation := range []string{"update", "rename"} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				f := tokenJSONFixture(t)
				before := make([]config.APITokenRecord, len(f.cfg.APITokens))
				for i := range f.cfg.APITokens {
					before[i] = f.cfg.APITokens[i].Clone()
					if before[i].ID == f.orgB.ID {
						before[i].Name, before[i].Scopes = tc.wantName, tc.wantScopes
					}
				}
				rec := tokenJSONCall(f, operation, strings.NewReader(tc.body))
				if rec.Code != http.StatusOK || !bytes.Equal(tokenJSONBytes(t, before), tokenJSONBytes(t, f.cfg.APITokens)) {
					t.Fatalf("edit changed more than the selected name/scopes: status %d", rec.Code)
				}
				reopened, err := config.NewConfigPersistence(f.cfg.DataPath).LoadAPITokens()
				if err != nil || !bytes.Equal(tokenJSONBytes(t, before), tokenJSONBytes(t, reopened)) {
					t.Fatal("edit lost saved identity, expiry, metadata or organization bindings")
				}
			})
		}
	}
}

func TestAPITokenJSONBoundsIncludeCompleteBody(t *testing.T) {
	installTokenMutationAuditProbe(t, func() {})
	for _, operation := range []string{"create", "update", "rename"} {
		body := `{"name":"boundary","scopes":["monitoring:read"]}`
		body += strings.Repeat(" ", maxAPITokenRequestBodySize-len(body))
		t.Run(operation+"/exact bound", func(t *testing.T) {
			f := tokenJSONFixture(t)
			if rec := tokenJSONCall(f, operation, strings.NewReader(body)); rec.Code != http.StatusOK {
				t.Fatalf("exact-bound object rejected: status %d", rec.Code)
			}
		})
		t.Run(operation+"/one byte over", func(t *testing.T) {
			assertTokenJSONRefused(t, operation, strings.NewReader(body+" "))
		})
	}
	for _, operation := range []string{"update", "rename"} {
		t.Run(operation+"/empty", func(t *testing.T) { assertTokenJSONRefused(t, operation, nil) })
	}
}
