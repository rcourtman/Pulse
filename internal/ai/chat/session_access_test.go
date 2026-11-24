package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An Assistant transcript holds what the Assistant read for its caller, host
// command output included. These cases pin that a caller who may not run host
// commands reaches only the sessions it started, on every session operation.

func newSessionAccessTestService(t *testing.T, dataDir string) *Service {
	t.Helper()
	store, err := NewSessionStore(dataDir)
	require.NoError(t, err)
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	return &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: NewAgenticLoop(provider, executor, "system"),
		provider:    provider,
		started:     true,
	}
}

func sessionIDs(sessions []Session) []string {
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	return ids
}

func TestServiceConfinesSessionsToTheirCaller(t *testing.T) {
	dataDir := t.TempDir()
	svc := newSessionAccessTestService(t, dataDir)

	system := context.Background()
	admin := WithSessionAccess(system, SessionAccess{Owner: "user:admin", All: true})
	viewer := WithSessionAccess(system, SessionAccess{Owner: "user:viewer"})
	chatToken := WithSessionAccess(system, SessionAccess{Owner: "api-token:chat-only"})
	unidentified := WithSessionAccess(system, SessionAccess{})

	require.NoError(t, svc.ExecuteStream(admin, ExecuteRequest{SessionID: "admin-chat", Prompt: "read the payments log on host1"}, nil))
	require.NoError(t, svc.ExecuteStream(viewer, ExecuteRequest{SessionID: "viewer-chat", Prompt: "hello"}, nil))
	created, err := svc.CreateSession(chatToken)
	require.NoError(t, err)
	// Stored before owners were recorded, and Pulse's own background run.
	_, err = svc.sessions.EnsureSession("legacy-chat")
	require.NoError(t, err)
	_, err = svc.sessions.EnsureSession("patrol-main")
	require.NoError(t, err)

	list := func(ctx context.Context) []string {
		t.Helper()
		sessions, err := svc.ListSessions(ctx)
		require.NoError(t, err)
		return sessionIDs(sessions)
	}
	everything := []string{"admin-chat", "viewer-chat", created.ID, "legacy-chat", "patrol-main"}
	assert.ElementsMatch(t, everything, list(admin))
	assert.ElementsMatch(t, everything, list(system), "Pulse's own work is not confined to a caller")
	assert.ElementsMatch(t, []string{"viewer-chat"}, list(viewer))
	assert.ElementsMatch(t, []string{created.ID}, list(chatToken))
	assert.Empty(t, list(unidentified))

	for _, denied := range []struct {
		name      string
		ctx       context.Context
		sessionID string
	}{
		{name: "another caller's session", ctx: viewer, sessionID: "admin-chat"},
		{name: "a token reading a person's session", ctx: chatToken, sessionID: "viewer-chat"},
		{name: "a session without an owner", ctx: viewer, sessionID: "legacy-chat"},
		{name: "a background session", ctx: viewer, sessionID: "patrol-main"},
		{name: "an unidentified caller", ctx: unidentified, sessionID: "legacy-chat"},
	} {
		t.Run(denied.name, func(t *testing.T) {
			before, err := svc.sessions.GetMessages(denied.sessionID)
			require.NoError(t, err)

			_, err = svc.GetMessages(denied.ctx, denied.sessionID)
			assert.ErrorIs(t, err, ErrSessionNotFound)
			_, err = svc.RenameSession(denied.ctx, denied.sessionID, "renamed")
			assert.ErrorIs(t, err, ErrSessionNotFound)
			_, err = svc.GetModelHandoffFindingID(denied.ctx, denied.sessionID)
			assert.ErrorIs(t, err, ErrSessionNotFound)
			_, err = svc.GetModelHandoffMetadata(denied.ctx, denied.sessionID)
			assert.ErrorIs(t, err, ErrSessionNotFound)
			assert.ErrorIs(t, svc.ClearModelHandoffContext(denied.ctx, denied.sessionID), ErrSessionNotFound)
			assert.ErrorIs(t, svc.AbortSession(denied.ctx, denied.sessionID), ErrSessionNotFound)
			assert.ErrorIs(t, svc.ExecuteStream(denied.ctx, ExecuteRequest{SessionID: denied.sessionID, Prompt: "continue"}, nil), ErrSessionNotFound)
			assert.ErrorIs(t, svc.DeleteSession(denied.ctx, denied.sessionID), ErrSessionNotFound)

			after, err := svc.sessions.GetMessages(denied.sessionID)
			require.NoError(t, err, "the session must survive a refused delete")
			assert.Len(t, after, len(before), "a refused turn must not reach the transcript")
			session, err := svc.sessions.Get(denied.sessionID)
			require.NoError(t, err)
			assert.NotEqual(t, "renamed", session.Title)
		})
	}

	// A refused session is answered exactly like one that does not exist.
	_, missing := svc.GetMessages(viewer, "no-such-session")
	_, refused := svc.GetMessages(viewer, "admin-chat")
	require.ErrorIs(t, missing, ErrSessionNotFound)
	assert.Equal(t, sessionNotFoundError("admin-chat").Error(), refused.Error())

	// The owner keeps every operation on its own session.
	_, err = svc.GetMessages(viewer, "viewer-chat")
	require.NoError(t, err)
	_, err = svc.RenameSession(viewer, "viewer-chat", "Mine")
	require.NoError(t, err)
	require.NoError(t, svc.AbortSession(viewer, "viewer-chat"))
	require.NoError(t, svc.ExecuteStream(viewer, ExecuteRequest{SessionID: "viewer-chat", Prompt: "and again"}, nil))

	// Owners outlive the process: the summary index must not drop them.
	reopened := newSessionAccessTestService(t, dataDir)
	sessions, err := reopened.ListSessions(viewer)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"viewer-chat"}, sessionIDs(sessions))

	require.NoError(t, svc.DeleteSession(viewer, "viewer-chat"))
	assert.Empty(t, list(viewer))
}

func TestServiceSessionAccessBackgroundIDsAreNeverACallersOwn(t *testing.T) {
	svc := newSessionAccessTestService(t, t.TempDir())
	viewer := WithSessionAccess(context.Background(), SessionAccess{Owner: "user:viewer"})

	// The ID is free, so nothing stored refuses it: claiming it would hand
	// the viewer every later Patrol run written under the same ID.
	for _, id := range []string{"patrol-main", "patrol-eval", "investigation-42"} {
		err := svc.ExecuteStream(viewer, ExecuteRequest{SessionID: id, Prompt: "hello"}, nil)
		assert.ErrorIs(t, err, ErrSessionNotFound, id)
		_, err = svc.sessions.Get(id)
		assert.ErrorIs(t, err, ErrSessionNotFound, id)
	}
}

func TestServiceSessionChangesHandsWhenAPrivilegedCallerContinuesIt(t *testing.T) {
	svc := newSessionAccessTestService(t, t.TempDir())
	viewer := WithSessionAccess(context.Background(), SessionAccess{Owner: "user:viewer"})
	admin := WithSessionAccess(context.Background(), SessionAccess{Owner: "user:admin", All: true})

	require.NoError(t, svc.ExecuteStream(viewer, ExecuteRequest{SessionID: "shared-chat", Prompt: "hello"}, nil))
	_, err := svc.GetMessages(viewer, "shared-chat")
	require.NoError(t, err)

	// The administrator's turn may run host commands, and their output is
	// stored in the session, so the viewer can no longer open it.
	require.NoError(t, svc.ExecuteStream(admin, ExecuteRequest{SessionID: "shared-chat", Prompt: "read the payments log on host1"}, nil))
	_, err = svc.GetMessages(viewer, "shared-chat")
	assert.ErrorIs(t, err, ErrSessionNotFound)
	messages, err := svc.GetMessages(admin, "shared-chat")
	require.NoError(t, err)
	assert.NotEmpty(t, messages)

	// Reading or renaming alone leaves the owner as it is.
	require.NoError(t, svc.ExecuteStream(viewer, ExecuteRequest{SessionID: "viewer-chat", Prompt: "hello"}, nil))
	_, err = svc.GetMessages(admin, "viewer-chat")
	require.NoError(t, err)
	_, err = svc.RenameSession(admin, "viewer-chat", "Reviewed")
	require.NoError(t, err)
	_, err = svc.GetMessages(viewer, "viewer-chat")
	require.NoError(t, err)
}

func TestStoredHandoffEnvelopeDoesNotReplayAcrossHigherAuthority(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)
	session, err := store.CreateFor("user:viewer")
	require.NoError(t, err)

	viewerAuth := HandoffAuthority{HostCommands: false, Execute: false}
	adminAuth := HandoffAuthority{HostCommands: true, Execute: true}
	require.NoError(t, store.SetModelHandoffEnvelopeFor(
		session.ID,
		"",
		"run pulse_read on host1",
		[]HandoffResource{{Name: "host1", Type: "agent"}},
		nil,
		HandoffMetadata{},
		viewerAuth,
	))

	// Same authority sees the stored handoff.
	gotCtx, gotRes, _, _, err := store.GetModelHandoffEnvelopeFor(session.ID, viewerAuth)
	require.NoError(t, err)
	assert.Equal(t, "run pulse_read on host1", gotCtx)
	assert.Len(t, gotRes, 1)

	// A higher-authority caller continuing the session must not replay lower-authority handoff context.
	gotCtx, gotRes, _, _, err = store.GetModelHandoffEnvelopeFor(session.ID, adminAuth)
	require.NoError(t, err)
	assert.Empty(t, gotCtx)
	assert.Empty(t, gotRes)

	// An envelope supplied by a higher-authority caller still replays for that authority.
	require.NoError(t, store.SetModelHandoffEnvelopeFor(
		session.ID,
		"",
		"admin finding context",
		nil,
		nil,
		HandoffMetadata{},
		adminAuth,
	))
	gotCtx, _, _, _, err = store.GetModelHandoffEnvelopeFor(session.ID, adminAuth)
	require.NoError(t, err)
	assert.Equal(t, "admin finding context", gotCtx)
}
