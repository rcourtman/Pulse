package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The rule-removal help uses existing session and CSRF protection, not a token
// pasted into a command. Exercise the actual handlers/store and Router middleware.
func TestPatrolSuppressionManagementBrowserSession(t *testing.T) {
	router, session := newRouterWithSession(t)
	handler := newTestAISettingsHandler(router.config, config.NewConfigPersistence(router.config.DataPath), nil)
	handler.defaultAIService.SetStateProvider(&stubStateProvider{})
	t.Cleanup(handler.defaultAIService.Stop)
	patrol := handler.defaultAIService.GetPatrolService()
	if patrol == nil {
		t.Fatal("expected Patrol service")
	}
	store := patrol.GetFindings()
	selected := store.AddSuppressionRule("vm-1", "Test VM", ai.FindingCategoryBackup, "Test reason")
	other := store.AddSuppressionRule("vm-2", "Other VM", ai.FindingCategoryCapacity, "Keep this rule")
	router.mux.HandleFunc("/api/ai/patrol/suppressions", RequireAuth(router.config, RequireScope(config.ScopeAIExecute, handler.HandleGetSuppressionRules)))
	router.mux.HandleFunc("/api/ai/patrol/suppressions/", RequireAuth(router.config, RequireScope(config.ScopeAIExecute, handler.HandleDeleteSuppressionRule)))

	request := func(method, path, csrf string, authenticated bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Pulse-Org-ID", "default")
		if authenticated {
			req.AddCookie(&http.Cookie{Name: "pulse_session", Value: session})
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	unauthenticated := request(http.MethodGet, "/api/ai/patrol/suppressions", "", false)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want 401", unauthenticated.Code)
	}
	list := request(http.MethodGet, "/api/ai/patrol/suppressions", "", true)
	if list.Code != http.StatusOK {
		t.Fatalf("authenticated list status = %d, want 200", list.Code)
	}
	var rules []ai.SuppressionRule
	if err := json.Unmarshal(list.Body.Bytes(), &rules); err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("listed %d rules, want 2", len(rules))
	}
	for _, rule := range rules {
		if rule.CreatedFrom != "manual" {
			t.Fatalf("created_from = %q, want manual", rule.CreatedFrom)
		}
	}
	csrf := ""
	for _, cookie := range list.Result().Cookies() {
		if cookie.Name == "pulse_csrf" {
			csrf = cookie.Value
		}
	}
	if csrf == "" {
		t.Fatal("authenticated GET must issue a browser CSRF cookie")
	}
	path := "/api/ai/patrol/suppressions/" + selected.ID
	for _, token := range []string{"", "invalid-csrf"} {
		rec := request(http.MethodDelete, path, token, true)
		if rec.Code != http.StatusForbidden || len(store.GetSuppressionRules()) != 2 {
			t.Fatalf("invalid-CSRF deletion status = %d; both rules must remain", rec.Code)
		}
	}
	removed := request(http.MethodDelete, path, csrf, true)
	if removed.Code != http.StatusOK {
		t.Fatalf("deletion status = %d, want 200", removed.Code)
	}
	if store.MatchesSuppressionRule("vm-1", ai.FindingCategoryBackup) {
		t.Fatal("selected rule still suppresses future matching findings")
	}
	if !store.MatchesSuppressionRule("vm-2", ai.FindingCategoryCapacity) {
		t.Fatal("unselected rule must remain effective")
	}
	readback := request(http.MethodGet, "/api/ai/patrol/suppressions", "", true)
	if readback.Code != http.StatusOK {
		t.Fatalf("readback status = %d, want 200", readback.Code)
	}
	if err := json.Unmarshal(readback.Body.Bytes(), &rules); err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].ID != other.ID {
		t.Fatal("readback must retain only the unselected rule")
	}
	notFound := request(http.MethodDelete, path, csrf, true)
	if notFound.Code != http.StatusNotFound || len(store.GetSuppressionRules()) != 1 {
		t.Fatalf("already-removed deletion status = %d; remaining rule must survive", notFound.Code)
	}
}
