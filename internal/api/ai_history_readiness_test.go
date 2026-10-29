package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// The private store is supplied by a factory. This fixture reads real JSON
// files through that public seam; it does not model the private file format.
type historyReadDiskStore struct {
	*testInvestigationStore
	path string
	org  string
}

func (s *historyReadDiskStore) LoadFromDisk() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var record aicontracts.InvestigationSession
	if err := json.Unmarshal(raw, &record); err != nil {
		return err
	}
	s.sessions[record.ID] = &record
	s.byFinding[record.FindingID] = []string{record.ID}
	return nil
}

func (s *historyReadDiskStore) Create(string, string) *aicontracts.InvestigationSession {
	panic("reading retained history must not create a replacement investigation")
}

type historyReadOrchestrator struct {
	stubInvestigationOrchestrator
	store aicontracts.InvestigationStore
}

func (o *historyReadOrchestrator) GetInvestigationByFinding(id string) *aicontracts.InvestigationSession {
	return o.store.GetLatestByFinding(id)
}

func (o *historyReadOrchestrator) InvestigateFinding(context.Context, *aicontracts.Finding, string) error {
	panic("history read started an investigation")
}

func (o *historyReadOrchestrator) ReinvestigateFinding(context.Context, string, string) error {
	panic("history read started a re-investigation")
}

type historyReadMessages struct {
	stubChatService
	store *chat.SessionStore
}

func (s *historyReadMessages) GetMessages(_ context.Context, id string) ([]ai.ChatMessage, error) {
	messages, err := s.store.GetMessages(id)
	if err != nil {
		return nil, err
	}
	result := make([]ai.ChatMessage, len(messages))
	for i, message := range messages {
		result[i] = adaptChatMessage(message.ClientSafe())
	}
	return result, nil
}

type historyReadFixture struct {
	root         string
	handler      *AISettingsHandler
	services     map[string]*ai.Service
	records      map[string]aicontracts.InvestigationSession
	historyPaths map[string]string
	messagePaths map[string]string
	storeCalls   atomic.Int32
	factoryCalls atomic.Int32
}

func newHistoryReadFixture(t *testing.T) *historyReadFixture {
	t.Helper()
	f := &historyReadFixture{
		root: t.TempDir(), services: make(map[string]*ai.Service), records: make(map[string]aicontracts.InvestigationSession),
		historyPaths: make(map[string]string), messagePaths: make(map[string]string),
	}
	mtp := config.NewMultiTenantPersistence(f.root)
	f.handler = &AISettingsHandler{
		mtPersistence: mtp, aiServices: make(map[string]*ai.Service),
		defaultConfig: &config.Config{AuthUser: "retained-admin", AuthPass: "retained-hash"},
		chatHandler:   &AIHandler{services: make(map[string]AIService)},
	}
	messagesByOrg := make(map[string]*historyReadMessages)
	orgForDir := make(map[string]string)
	for _, org := range []string{"default", "tenant-a", "tenant-b"} {
		persistence, err := mtp.GetPersistence(org)
		if err != nil {
			t.Fatal(err)
		}
		orgForDir[persistence.DataDir()] = org
		// Retain an actual encrypted config alongside the history. Recovery
		// must not rewrite credentials or touch the administrator's access.
		if err := persistence.SaveSSOConfig(&config.SSOConfig{Providers: []config.SSOProvider{{
			ID: "retained-oidc", Type: config.SSOProviderTypeOIDC,
			OIDC: &config.OIDCProviderConfig{ClientID: org, ClientSecret: "synthetic-retained-secret"},
		}}}); err != nil {
			t.Fatal(err)
		}
		messages, err := chat.NewSessionStore(persistence.DataDir())
		if err != nil {
			t.Fatal(err)
		}
		original, err := messages.Create()
		if err != nil {
			t.Fatal(err)
		}
		if err := messages.AddMessage(original.ID, chat.Message{ID: "original-message", Role: "assistant", Content: "Retained evidence for " + org, Timestamp: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatal(err)
		}
		entries, err := filepath.Glob(filepath.Join(persistence.DataDir(), "ai_sessions", "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			raw, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "original-message") {
				f.messagePaths[org] = entry
			}
		}
		if f.messagePaths[org] == "" {
			t.Fatal("original persisted chat file missing")
		}
		completed := time.Date(2026, 10, 20, 10, 1, 0, 0, time.UTC)
		record := aicontracts.InvestigationSession{
			ID: "original-investigation-" + org, FindingID: "shared-history", SessionID: original.ID,
			Status: aicontracts.InvestigationStatusCompleted, Outcome: aicontracts.OutcomeUnclassified,
			StartedAt: completed.Add(-time.Minute), CompletedAt: &completed, TurnCount: 3,
			Summary: "Retained review for " + org, ToolsUsed: []string{"pulse_query"}, EvidenceIDs: []string{"original-evidence"},
		}
		f.records[org] = record
		path := filepath.Join(persistence.DataDir(), "retained-history-fixture.json")
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		f.historyPaths[org] = path
		svc := ai.NewService(nil, nil)
		svc.SetOrgID(org)
		svc.SetStateProvider(&MockStateProvider{})
		finding := investigatedFinding(record.FindingID)
		finding.InvestigationSessionID = record.SessionID
		finding.InvestigationStatus = string(record.Status)
		finding.InvestigationRecord = &aicontracts.InvestigationRecord{ID: record.ID, FindingID: record.FindingID, SessionID: record.SessionID,
			Status: record.Status, Outcome: record.Outcome, Conclusion: record.Summary, StartedAt: record.StartedAt}
		svc.GetPatrolService().GetFindings().Add(finding)
		f.services[org] = svc
		chatSvc := chat.NewService(chat.Config{AIConfig: config.NewDefaultAIConfig(), DataDir: persistence.DataDir(), OrgID: org})
		messagesByOrg[org] = &historyReadMessages{store: messages}
		if org == "default" {
			f.handler.defaultPersistence, f.handler.defaultAIService = persistence, svc
			f.handler.chatHandler.defaultService = chatSvc
		} else {
			f.handler.aiServices[org] = svc
			f.handler.chatHandler.services[org] = chatSvc
		}
	}
	prevStore, prevOrchestrator := getCreateInvestigationStore(), getCreateInvestigationOrchestrator()
	aiInvestigationEnabledMu.RLock()
	prevEnabled := aiInvestigationEnabledFunc
	aiInvestigationEnabledMu.RUnlock()
	t.Cleanup(func() {
		SetCreateInvestigationStore(prevStore)
		SetCreateInvestigationOrchestrator(prevOrchestrator)
		SetAIInvestigationEnabled(prevEnabled)
	})
	SetAIInvestigationEnabled(func() bool { return true })
	SetCreateInvestigationStore(func(dir string) aicontracts.InvestigationStore {
		f.storeCalls.Add(1)
		org, ok := orgForDir[dir]
		if !ok {
			panic("history factory received another organization's directory")
		}
		return &historyReadDiskStore{testInvestigationStore: newTestInvestigationStore(), path: f.historyPaths[org], org: org}
	})
	SetCreateInvestigationOrchestrator(func(deps aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator {
		f.factoryCalls.Add(1)
		store := deps.Store.(*historyReadDiskStore)
		f.services[store.org].SetChatService(messagesByOrg[store.org])
		return &historyReadOrchestrator{store: store}
	})
	return f
}

func (f *historyReadFixture) read(org, suffix string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/shared-history/investigation"+suffix, nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
	rec := httptest.NewRecorder()
	if suffix == "/messages" {
		f.handler.HandleGetInvestigationMessages(rec, req)
	} else {
		f.handler.HandleGetInvestigation(rec, req)
	}
	return rec
}

// Make the read fail deterministically even under root, without changing a
// byte of the original file. Restoring it is recovery, not a replacement run.
func blockHistoryFixtureFile(t *testing.T, path string) func() {
	t.Helper()
	if err := os.Rename(path, path+".retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".retained", path); err != nil {
			t.Fatal(err)
		}
	}
}

func historyFixtureFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err == nil {
			relative, _ := filepath.Rel(root, path)
			result[relative] = fmt.Sprintf("%x", sha256.Sum256(raw))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInvestigationReadRecovery(t *testing.T) {
	f := newHistoryReadFixture(t)
	before := historyFixtureFiles(t, f.root)
	restore := blockHistoryFixtureFile(t, f.historyPaths["tenant-a"])
	f.handler.setupInvestigationOrchestrator("tenant-a", f.services["tenant-a"])
	if f.factoryCalls.Load() != 0 || len(f.handler.investigationStores) != 0 || f.services["tenant-a"].GetPatrolService().GetInvestigationOrchestrator() != nil {
		t.Fatal("failed load was installed for investigation or exposed to its writer")
	}
	for _, suffix := range []string{"", "/messages"} {
		rec := f.read("tenant-a", suffix)
		if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), f.root) {
			t.Fatalf("unread history should be a redacted 503, got %d: %s", rec.Code, rec.Body.String())
		}
	}
	if rec := f.read("tenant-b", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Retained review for tenant-b") {
		t.Fatalf("other org lost its independent history: %d %s", rec.Code, rec.Body.String())
	}
	restore()
	// Race a restored startup against both consuming reads. Only one factory
	// may publish the recovered store/orchestrator for this organization.
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			suffix := ""
			if i%2 == 1 {
				suffix = "/messages"
			}
			rec := f.read("tenant-a", suffix)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), f.records["tenant-a"].SessionID) || strings.Contains(rec.Body.String(), "tenant-b") {
				t.Errorf("original org/session did not recover: %d %s", rec.Code, rec.Body.String())
			}
		}()
	}
	wg.Wait()
	if got := f.factoryCalls.Load(); got != 2 {
		t.Fatalf("orchestrator factory calls = %d, want one per recovered org", got)
	}
	if after := historyFixtureFiles(t, f.root); !reflect.DeepEqual(before, after) {
		t.Fatal("history, chat files or encrypted credentials changed during recovery")
	}
	if f.handler.defaultConfig.AuthUser != "retained-admin" || f.handler.defaultConfig.AuthPass != "retained-hash" {
		t.Fatal("history recovery changed local administrator access")
	}
}

func TestInvestigationReadMessageRecovery(t *testing.T) {
	f := newHistoryReadFixture(t)
	before := historyFixtureFiles(t, f.root)
	if rec := f.read("default", "/messages"); rec.Code != http.StatusOK {
		t.Fatalf("initial original thread: %d %s", rec.Code, rec.Body.String())
	}
	restore := blockHistoryFixtureFile(t, f.messagePaths["default"])
	rec := f.read("default", "/messages")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), f.root) || strings.Contains(rec.Body.String(), "original-message") {
		t.Fatalf("message failure should not be empty success or expose private data: %d %s", rec.Code, rec.Body.String())
	}
	restore()
	rec = f.read("default", "/messages")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "original-message") || !strings.Contains(rec.Body.String(), f.records["default"].SessionID) {
		t.Fatalf("original thread did not recover: %d %s", rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(before, historyFixtureFiles(t, f.root)) || f.factoryCalls.Load() != 1 {
		t.Fatal("message retry rewrote retained files or replaced the live orchestrator")
	}
}

func TestInvestigationReadReferencedRunIsNotAbsence(t *testing.T) {
	for _, mode := range []string{"missing", "different-session", "cached-record", "absent", "previous-occurrence"} {
		t.Run(mode, func(t *testing.T) {
			f := newHistoryReadFixture(t)
			svc := f.services["default"]
			finding := svc.GetPatrolService().GetFindings().Get("shared-history")
			session := f.records["default"]
			want := http.StatusServiceUnavailable
			switch mode {
			case "missing":
				session.FindingID = "other-finding"
			case "different-session":
				session.SessionID = "another-session"
			case "cached-record":
				finding.InvestigationSessionID = ""
				session.FindingID = "other-finding"
			case "absent", "previous-occurrence":
				finding.InvestigationSessionID, finding.InvestigationRecord = "", nil
				want = http.StatusNotFound
				if mode == "absent" {
					session.FindingID = "other-finding"
				} else {
					regressed := session.StartedAt.Add(time.Hour)
					finding.LastRegressionAt = &regressed
				}
			}
			svc.GetPatrolService().GetFindings().ClearAll()
			svc.GetPatrolService().GetFindings().Add(finding)
			svc.GetPatrolService().SetInvestigationOrchestrator(&stubInvestigationOrchestrator{session: &session})
			for _, suffix := range []string{"", "/messages"} {
				rec := f.read("default", suffix)
				if rec.Code != want {
					t.Fatalf("%s: %d %s, want %d", suffix, rec.Code, rec.Body.String(), want)
				}
			}
		})
	}
}

func TestInvestigationReadRequiresScopedPersistenceAndEnabledRuntime(t *testing.T) {
	f := newHistoryReadFixture(t)
	for _, org := range []string{"unavailable", "../tenant-a"} {
		svc := ai.NewService(nil, nil)
		svc.SetStateProvider(&MockStateProvider{})
		f.handler.chatHandler.services[org] = chat.NewService(chat.Config{AIConfig: config.NewDefaultAIConfig()})
		if org == "unavailable" {
			f.handler.mtPersistence = nil
		}
		f.handler.setupInvestigationOrchestrator(org, svc)
		if svc.GetPatrolService().GetInvestigationOrchestrator() != nil || f.storeCalls.Load() != 0 {
			t.Fatal("missing/invalid org persistence fell back to default or a memory-only store")
		}
		f.handler.mtPersistence = config.NewMultiTenantPersistence(f.root)
	}
	SetAIInvestigationEnabled(func() bool { return false })
	if rec := f.read("default", ""); rec.Code != http.StatusServiceUnavailable || f.storeCalls.Load() != 0 {
		t.Fatalf("disabled runtime was created by a read: %d %s", rec.Code, rec.Body.String())
	}
}
