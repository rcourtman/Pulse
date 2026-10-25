package configapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const replayTestPassphrase = "synthetic-replay-control-passphrase"

func replayTestHandler(t *testing.T) (*ConfigHandlers, *atomic.Int32) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_DEV", "true")
	p := config.NewConfigPersistence(dir)
	if err := p.SaveSystemSettings(*config.DefaultSystemSettings()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := &atomic.Int32{}
	h := &ConfigHandlers{
		defaultConfig:      &config.Config{DataPath: dir, ConfigPath: dir},
		defaultPersistence: p,
		reloadFunc:         func() error { calls.Add(1); return nil },
	}
	h.SetRuntimeDependencies(RuntimeDependencies{})
	store := p.GetGuestMetadataStore()
	t.Cleanup(func() {
		if err := store.Close(3 * time.Second); err != nil {
			t.Error(err)
		}
	})
	return h, calls
}

func replayTestRequest(t *testing.T, name string) ImportConfigRequest {
	t.Helper()
	data := config.ExportData{
		Version:       "4.4",
		Nodes:         config.NodesConfig{PVEInstances: []config.PVEInstance{{Name: name, Host: "https://" + name + ".invalid:8006", Disabled: true, VerifySSL: true, TokenValue: "synthetic-" + name + "-secret"}}},
		System:        *config.DefaultSystemSettings(),
		GuestMetadata: map[string]*config.GuestMetadata{"synthetic:101": {Description: name}},
		APITokens:     []config.APITokenRecord{{ID: name, Hash: "synthetic-" + name + "-hash", OrgID: "default"}},
	}
	data.System.PublicURL = "https://" + name + ".invalid"
	plain, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return ImportConfigRequest{Data: encryptedImportValidationFixture(t, plain, replayTestPassphrase), Passphrase: replayTestPassphrase}
}

func replayTestBody(t *testing.T, request ImportConfigRequest) []byte {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func replayTestSubmit(h *ConfigHandlers, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.HandleImportConfig(rec, httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body)))
	return rec
}

func replayTestNewerEdit(t *testing.T, h *ConfigHandlers) {
	t.Helper()
	p := h.defaultPersistence
	if err := p.SaveNodesConfig([]config.PVEInstance{{Name: "newer-edit", Host: "https://newer.invalid:8006", TokenValue: "synthetic-newer-secret", Disabled: true, VerifySSL: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSystemSettings()
	settings.PublicURL = "https://newer.invalid"
	if err := p.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	if err := p.SaveAPITokens([]config.APITokenRecord{{ID: "newer-token", Hash: "synthetic-newer-hash", OrgID: "default"}}); err != nil {
		t.Fatal(err)
	}
	if err := p.GetGuestMetadataStore().Set("synthetic:101", &config.GuestMetadata{Description: "newer metadata"}); err != nil {
		t.Fatal(err)
	}
}

// Real loopback HTTP drops two response bodies after running the real handler.
// Three equal POSTs must apply once, even if a legitimate edit follows the
// unseen first response. No monitored guest or external endpoint is contacted.
func TestHandleImportConfigLostResponsePreservesNewerEdit(t *testing.T) {
	h, calls := replayTestHandler(t)
	request := replayTestRequest(t, "restored")
	body := replayTestBody(t, request)
	var wire atomic.Int32
	var filesAfterEdit map[string]string
	var events []*httptest.ResponseRecorder
	observed := make(chan *httptest.ResponseRecorder, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := wire.Add(1)
		rec := httptest.NewRecorder()
		h.HandleImportConfig(rec, r)
		if n <= 2 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
		} else {
			for name, values := range rec.Header() {
				w.Header()[name] = values
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
		}
		observed <- rec
	}))
	defer srv.Close()
	client := srv.Client()
	client.Timeout = 10 * time.Second
	for i := 0; i < 3; i++ {
		resp, err := client.Post(srv.URL+"/api/config/import", "application/json", bytes.NewReader(body))
		if i < 2 {
			if err == nil {
				resp.Body.Close()
				t.Fatal("response-loss control unexpectedly received a response")
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			responseBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if resp.StatusCode != http.StatusConflict {
				t.Errorf("third wire response = %d %s; want 409", resp.StatusCode, responseBody)
			}
		}
		select {
		case rec := <-observed:
			events = append(events, rec)
		case <-time.After(10 * time.Second):
			t.Fatal("handler did not complete")
		}
		if i == 0 {
			if events[0].Code != http.StatusOK {
				t.Fatalf("first import = %d %s", events[0].Code, events[0].Body.String())
			}
			replayTestNewerEdit(t, h)
			filesAfterEdit = metadataImportFiles(t, h.defaultPersistence.GetConfigDir())
		}
		if i > 0 && !reflect.DeepEqual(filesAfterEdit, metadataImportFiles(t, h.defaultPersistence.GetConfigDir())) {
			t.Error("response-loss replay changed persisted files or recovery backups")
		}
		if i > 0 && events[i].Code != http.StatusConflict {
			t.Errorf("replay %d = %d, want conflict", i, events[i].Code)
		}
	}
	if wire.Load() != 3 || calls.Load() != 1 {
		t.Errorf("three POSTs caused %d reloads, want exactly one", calls.Load())
	}
	settings, err := h.defaultPersistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := h.defaultPersistence.LoadAPITokens()
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := h.defaultPersistence.LoadNodesConfig()
	if err != nil {
		t.Fatal(err)
	}
	metadata := h.defaultPersistence.GetGuestMetadataStore().Get("synthetic:101")
	if settings.PublicURL != "https://newer.invalid" || len(tokens) != 1 || tokens[0].ID != "newer-token" || len(nodes.PVEInstances) != 1 || nodes.PVEInstances[0].Name != "newer-edit" || metadata == nil || metadata.Description != "newer metadata" {
		t.Error("response-loss replays overwrote a newer legitimate edit")
	}
	logReplayProducer(t, "completed", request, events[2])
}

func TestHandleImportConfigOverlappingRequestsDoNotWriteOrReload(t *testing.T) {
	for _, overlap := range []string{"same archive", "different archive"} {
		t.Run(overlap, func(t *testing.T) {
			h, calls := replayTestHandler(t)
			first := replayTestBody(t, replayTestRequest(t, "first"))
			request := replayTestRequest(t, "second")
			if overlap == "same archive" {
				if err := json.Unmarshal(first, &request); err != nil {
					t.Fatal(err)
				}
			}
			started, release, done := make(chan struct{}), make(chan struct{}), make(chan *httptest.ResponseRecorder, 1)
			h.reloadFunc = func() error {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return nil
			}
			go func() { done <- replayTestSubmit(h, first) }()
			released := false
			defer func() {
				if !released {
					close(release)
				}
				<-done
			}()
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("first import did not reach apply")
			}
			before := metadataImportFiles(t, h.defaultPersistence.GetConfigDir())
			rec := replayTestSubmit(h, replayTestBody(t, request))
			if rec.Code != http.StatusConflict {
				t.Errorf("overlap = %d %s, want 409", rec.Code, rec.Body.String())
			}
			if calls.Load() != 1 || !reflect.DeepEqual(before, metadataImportFiles(t, h.defaultPersistence.GetConfigDir())) {
				t.Error("overlap repeated disk writes/backups or reloaded monitoring")
			}
			logReplayProducer(t, "in-flight", request, rec)
			close(release)
			released = true
		})
	}
}

func TestHandleImportConfigAppliedFailureCannotReplay(t *testing.T) {
	h, calls := replayTestHandler(t)
	h.reloadFunc = func() error { calls.Add(1); return errors.New("synthetic apply failure") }
	request := replayTestRequest(t, "applied-before-error")
	body := replayTestBody(t, request)
	rec := replayTestSubmit(h, body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("first apply failure = %d", rec.Code)
	}
	replayTestNewerEdit(t, h)
	before := metadataImportFiles(t, h.defaultPersistence.GetConfigDir())
	rec = replayTestSubmit(h, body)
	if rec.Code != http.StatusConflict || calls.Load() != 1 || !reflect.DeepEqual(before, metadataImportFiles(t, h.defaultPersistence.GetConfigDir())) {
		t.Error("a committed import with a failed runtime apply was reapplied")
	}
	logReplayProducer(t, "applied-with-error", request, rec)
}

func TestHandleImportConfigUncommittedFailureCanRetry(t *testing.T) {
	h, calls := replayTestHandler(t)
	request := replayTestRequest(t, "retry-after-storage-repair")
	body := replayTestBody(t, request)
	path := filepath.Join(h.defaultPersistence.GetConfigDir(), "guest_metadata.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if rec := replayTestSubmit(h, body); rec.Code != http.StatusBadRequest {
		t.Fatalf("failed commit status = %d", rec.Code)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if rec := replayTestSubmit(h, body); rec.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("corrected retry = %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandleImportConfigWrongPassphraseCannotReuseSuccess(t *testing.T) {
	h, calls := replayTestHandler(t)
	request := replayTestRequest(t, "passphrase-bound")
	if rec := replayTestSubmit(h, replayTestBody(t, request)); rec.Code != http.StatusOK {
		t.Fatalf("first import = %d", rec.Code)
	}
	before := metadataImportFiles(t, h.defaultPersistence.GetConfigDir())
	request.Passphrase = "synthetic-wrong-passphrase"
	rec := replayTestSubmit(h, replayTestBody(t, request))
	if rec.Code != http.StatusBadRequest || calls.Load() != 1 || !reflect.DeepEqual(before, metadataImportFiles(t, h.defaultPersistence.GetConfigDir())) {
		t.Error("wrong passphrase reused successful admission or changed state")
	}
}

func logReplayProducer(t *testing.T, state string, request ImportConfigRequest, rec *httptest.ResponseRecorder) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"state": state, "request": request, "status": rec.Code, "contentType": rec.Header().Get("Content-Type"), "body": rec.Body.String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SYNTHETIC_IMPORT_REPLAY_READBACK %s", data)
}

func TestHandleImportConfigDifferentArchiveThenOldReplay(t *testing.T) {
	h, calls := replayTestHandler(t)
	first := replayTestBody(t, replayTestRequest(t, "first"))
	second := replayTestBody(t, replayTestRequest(t, "second"))
	for _, body := range [][]byte{first, second} {
		if rec := replayTestSubmit(h, body); rec.Code != http.StatusOK {
			t.Fatalf("distinct import = %d %s", rec.Code, rec.Body.String())
		}
	}
	before := metadataImportFiles(t, h.defaultPersistence.GetConfigDir())
	if rec := replayTestSubmit(h, first); rec.Code != http.StatusConflict {
		t.Fatalf("old replay = %d", rec.Code)
	}
	if calls.Load() != 2 || !reflect.DeepEqual(before, metadataImportFiles(t, h.defaultPersistence.GetConfigDir())) {
		t.Error("old replay replaced the newer legitimate import")
	}
}
