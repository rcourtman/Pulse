package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/mock"
)

// Assistant sessions belong to the caller who started them, because their
// transcripts hold what the Assistant read for that caller. These cases pin
// the identity each caller owns sessions under, and that only the callers who
// get host-command authority are given the sessions of others.
func TestAssistantSessionAccessNamesTheCaller(t *testing.T) {
	InitSessionStore(t.TempDir())
	cfg := &config.Config{AuthUser: "admin"}
	owners := map[string]string{
		"viewer session":                    "user:viewer",
		"administrator session":             "user:admin",
		"explicit action grant":             "user:operator",
		"RBAC denies execute":               "user:admin",
		"ai:chat token":                     "api-token:assistant-token-ai:chat",
		"ai:chat and ai:execute token":      "api-token:assistant-token-ai:chat-ai:execute",
		"ai:chat and actions:execute token": "api-token:assistant-token-ai:chat-actions:execute",
		"token denied by RBAC":              "api-token:assistant-token-ai:chat-ai:execute",
	}
	for _, caller := range assistantHostCommandCallers() {
		t.Run(caller.name, func(t *testing.T) {
			h := newTestAIHandler(cfg, nil, nil)
			svc := new(MockAIService)
			h.defaultService = svc
			svc.On("IsRunning").Return(true)
			var got chat.SessionAccess
			var bound bool
			svc.On("ListSessions", mock.Anything).Return([]chat.Session{}, nil).Run(func(args mock.Arguments) {
				got, bound = chat.SessionAccessFromContext(args.Get(0).(context.Context))
			})

			req := caller.prepare(t, httptest.NewRequest(http.MethodGet, "/api/ai/sessions", nil))
			withAssistantSessionAccess(cfg, caller.authorizer, h.HandleSessions)(httptest.NewRecorder(), req)

			want := chat.SessionAccess{Owner: owners[caller.name], All: caller.want}
			if !bound || got != want {
				t.Fatalf("session access = %+v (bound=%v), want %+v", got, bound, want)
			}
		})
	}
}

func TestAssistantSessionHandlersWithoutBindingOwnNothing(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	svc := new(MockAIService)
	h.defaultService = svc
	svc.On("IsRunning").Return(true)
	var seen []string
	record := func(operation string) func(mock.Arguments) {
		return func(args mock.Arguments) {
			access, bound := chat.SessionAccessFromContext(args.Get(0).(context.Context))
			if !bound || access != (chat.SessionAccess{}) {
				t.Errorf("%s reached the chat service with session access %+v (bound=%v); an unbound request must fail closed", operation, access, bound)
			}
			seen = append(seen, operation)
		}
	}
	svc.On("ListSessions", mock.Anything).Return([]chat.Session{}, nil).Run(record("list"))
	svc.On("CreateSession", mock.Anything).Return(&chat.Session{ID: "s1"}, nil).Run(record("create"))
	svc.On("GetMessages", mock.Anything, "s1").Return([]chat.Message{}, nil).Run(record("messages"))
	svc.On("RenameSession", mock.Anything, "s1", "Renamed").Return(&chat.Session{ID: "s1"}, nil).Run(record("rename"))
	svc.On("DeleteSession", mock.Anything, "s1").Return(nil).Run(record("delete"))
	svc.On("AbortSession", mock.Anything, "s1").Return(nil).Run(record("abort"))
	svc.On("GetModelHandoffFindingID", mock.Anything, "s1").Return("", nil).Run(record("handoff finding"))
	svc.On("GetModelHandoffMetadata", mock.Anything, "s1").Return(chat.HandoffMetadata{}, nil).Run(record("handoff metadata"))
	svc.On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(record("chat"))

	h.HandleSessions(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ai/sessions", nil))
	h.HandleCreateSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/ai/sessions", nil))
	h.HandleMessages(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ai/sessions/s1/messages", nil), "s1")
	h.HandleRenameSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/api/ai/sessions/s1", strings.NewReader(`{"title":"Renamed"}`)), "s1")
	h.HandleDeleteSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/ai/sessions/s1", nil), "s1")
	h.HandleAbort(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/ai/sessions/s1/abort", nil), "s1")
	h.HandleChat(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{"prompt":"hi","session_id":"s1"}`)))

	want := []string{"list", "create", "messages", "rename", "delete", "abort", "handoff finding", "handoff metadata", "chat"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Fatalf("session operations that reached the chat service = %v, want %v", seen, want)
	}
}

// A session that belongs to another caller and one that does not exist are
// the same 404; anything else stays a sanitized 500.
func TestAssistantSessionRoutesAnswerNotFoundForUnusableSessions(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	svc := new(MockAIService)
	h.defaultService = svc
	svc.On("IsRunning").Return(true)
	notFound := fmt.Errorf("%w: theirs", chat.ErrSessionNotFound)
	svc.On("GetMessages", mock.Anything, "theirs").Return([]chat.Message(nil), notFound)
	svc.On("RenameSession", mock.Anything, "theirs", "Renamed").Return(nil, notFound)
	svc.On("DeleteSession", mock.Anything, "theirs").Return(notFound)
	svc.On("AbortSession", mock.Anything, "theirs").Return(notFound)
	svc.On("GetMessages", mock.Anything, "broken").Return([]chat.Message(nil), errors.New("failed to parse session: /data/ai_sessions/x.json"))

	for name, serve := range map[string]func(w http.ResponseWriter){
		"messages": func(w http.ResponseWriter) {
			h.HandleMessages(w, httptest.NewRequest(http.MethodGet, "/api/ai/sessions/theirs/messages", nil), "theirs")
		},
		"rename": func(w http.ResponseWriter) {
			h.HandleRenameSession(w, httptest.NewRequest(http.MethodPatch, "/api/ai/sessions/theirs", strings.NewReader(`{"title":"Renamed"}`)), "theirs")
		},
		"delete": func(w http.ResponseWriter) {
			h.HandleDeleteSession(w, httptest.NewRequest(http.MethodDelete, "/api/ai/sessions/theirs", nil), "theirs")
		},
		"abort": func(w http.ResponseWriter) {
			h.HandleAbort(w, httptest.NewRequest(http.MethodPost, "/api/ai/sessions/theirs/abort", nil), "theirs")
		},
	} {
		rec := httptest.NewRecorder()
		serve(rec)
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "theirs") {
			t.Fatalf("%s of an unusable session = %d %q, want a bare 404", name, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	h.HandleMessages(rec, httptest.NewRequest(http.MethodGet, "/api/ai/sessions/broken/messages", nil), "broken")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "ai_sessions") {
		t.Fatalf("storage failure = %d %q, want a sanitized 500", rec.Code, rec.Body.String())
	}
}

// The registered routes, through the whole router chain and the real chat
// service: a token that may only chat keeps its own sessions and cannot list,
// read, rename, abort, delete or continue a session an administrator's
// credential started, while that credential still sees both.
func TestAssistantSessionRoutesConfineCallersToTheirOwnSessions(t *testing.T) {
	f := newAssistantStreamFixture(t, "chat", []string{config.ScopeAIChat})
	call := func(token, method, path, body string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Pulse-Org-ID", "default")
		req.Header.Set("X-API-Token", token)
		resp, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(data)
	}
	create := func(token string) string {
		t.Helper()
		status, body := call(token, http.MethodPost, "/api/ai/sessions", "")
		var session chat.Session
		if err := json.Unmarshal([]byte(body), &session); status != http.StatusOK || err != nil || session.ID == "" {
			t.Fatalf("create session = %d %q (%v)", status, body, err)
		}
		if strings.Contains(body, "owner") {
			t.Fatalf("session payload discloses its owner: %s", body)
		}
		return session.ID
	}
	listed := func(token string) []string {
		t.Helper()
		status, body := call(token, http.MethodGet, "/api/ai/sessions", "")
		var sessions []chat.Session
		if err := json.Unmarshal([]byte(body), &sessions); status != http.StatusOK || err != nil {
			t.Fatalf("list sessions = %d %q (%v)", status, body, err)
		}
		ids := make([]string, 0, len(sessions))
		for _, session := range sessions {
			ids = append(ids, session.ID)
		}
		return ids
	}
	sameIDs := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		seen := make(map[string]bool, len(got))
		for _, id := range got {
			seen[id] = true
		}
		for _, id := range want {
			if !seen[id] {
				return false
			}
		}
		return true
	}

	adminSession := create(assistantControlToken)
	ownSession := create(f.token)

	if got := listed(f.token); !sameIDs(got, ownSession) {
		t.Fatalf("ai:chat token lists %v, want only its own session %s", got, ownSession)
	}
	if got := listed(assistantControlToken); !sameIDs(got, adminSession, ownSession) {
		t.Fatalf("administrator credential lists %v, want both sessions", got)
	}

	for _, refused := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/ai/sessions/" + adminSession + "/messages", ""},
		{http.MethodPatch, "/api/ai/sessions/" + adminSession, `{"title":"Taken over"}`},
		{http.MethodPost, "/api/ai/sessions/" + adminSession + "/abort", ""},
		{http.MethodDelete, "/api/ai/sessions/" + adminSession, ""},
		{http.MethodGet, "/api/ai/sessions/patrol-main/messages", ""},
	} {
		if status, body := call(f.token, refused.method, refused.path, refused.body); status != http.StatusNotFound {
			t.Fatalf("ai:chat token %s %s = %d %q, want 404", refused.method, refused.path, status, body)
		}
	}
	status, stream := call(f.token, http.MethodPost, "/api/ai/chat", fmt.Sprintf(`{"prompt":"continue","session_id":%q}`, adminSession))
	if status != http.StatusOK || !strings.Contains(stream, `"type":"error"`) || strings.Contains(stream, `"type":"content"`) {
		t.Fatalf("ai:chat token continuing the administrator's session = %d %q, want a refused turn", status, stream)
	}

	// Control: nothing above changed the administrator's session, and both
	// callers keep every operation on what they may use.
	if status, body := call(assistantControlToken, http.MethodGet, "/api/ai/sessions/"+adminSession+"/messages", ""); status != http.StatusOK || strings.TrimSpace(body) != "[]" {
		t.Fatalf("administrator's transcript after the refused requests = %d %q, want it empty and readable", status, body)
	}
	if status, body := call(assistantControlToken, http.MethodGet, "/api/ai/sessions/"+ownSession+"/messages", ""); status != http.StatusOK {
		t.Fatalf("administrator credential reading the token's session = %d %q", status, body)
	}
	if status, body := call(f.token, http.MethodGet, "/api/ai/sessions/"+ownSession+"/messages", ""); status != http.StatusOK {
		t.Fatalf("ai:chat token reading its own session = %d %q", status, body)
	}
	if status, body := call(f.token, http.MethodPatch, "/api/ai/sessions/"+ownSession, `{"title":"Mine"}`); status != http.StatusOK {
		t.Fatalf("ai:chat token renaming its own session = %d %q", status, body)
	}
	if status, body := call(f.token, http.MethodDelete, "/api/ai/sessions/"+ownSession, ""); status != http.StatusNoContent {
		t.Fatalf("ai:chat token deleting its own session = %d %q", status, body)
	}
}
