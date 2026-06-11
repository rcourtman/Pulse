package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type titleStubProvider struct {
	title string
	calls int
}

func (p *titleStubProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	p.calls++
	return &providers.ChatResponse{Content: p.title, Model: req.Model, InputTokens: 10, OutputTokens: 5}, nil
}

func (p *titleStubProvider) TestConnection(ctx context.Context) error { return nil }
func (p *titleStubProvider) Name() string                             { return "stub" }
func (p *titleStubProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}
func (p *titleStubProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	return nil
}
func (p *titleStubProvider) SupportsThinking(model string) bool { return false }

func newTitleTestService(t *testing.T, provider providers.StreamingProvider) (*Service, *SessionStore) {
	t.Helper()
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)
	return &Service{sessions: store, provider: provider, started: true}, store
}

func seedFirstExchange(t *testing.T, store *SessionStore, prompt, answer string) *Session {
	t.Helper()
	session, err := store.Create()
	require.NoError(t, err)
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "user", Content: prompt}))
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "assistant", Content: answer}))
	return session
}

func TestGenerateSessionTitleUpgradesPlaceholderAfterFirstExchange(t *testing.T) {
	provider := &titleStubProvider{title: "\"Frigate memory check on delly.\"\nExtra line ignored"}
	svc, store := newTitleTestService(t, provider)
	session := seedFirstExchange(t, store, "how much memory is frigate using on delly right now?", "Frigate uses 1.2 GiB.")

	require.NoError(t, svc.generateSessionTitle(context.Background(), session.ID))

	updated, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, "Frigate memory check on delly", updated.Title)
	assert.Equal(t, 1, provider.calls)
}

func TestGenerateSessionTitleSkipsUserRenamedSessions(t *testing.T) {
	provider := &titleStubProvider{title: "Generated title"}
	svc, store := newTitleTestService(t, provider)
	session := seedFirstExchange(t, store, "check pbs backups", "All PBS jobs succeeded.")
	_, err := store.Rename(session.ID, "My backup investigation")
	require.NoError(t, err)

	err = svc.generateSessionTitle(context.Background(), session.ID)
	require.Error(t, err)
	assert.Equal(t, 0, provider.calls)

	updated, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, "My backup investigation", updated.Title)
}

func TestGenerateSessionTitleSkipsLaterTurns(t *testing.T) {
	provider := &titleStubProvider{title: "Generated title"}
	svc, store := newTitleTestService(t, provider)
	session := seedFirstExchange(t, store, "first prompt", "first answer")
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "user", Content: "second prompt"}))
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "assistant", Content: "second answer"}))

	err := svc.generateSessionTitle(context.Background(), session.ID)
	require.Error(t, err)
	assert.Equal(t, 0, provider.calls)
}

func TestNormalizeGeneratedSessionTitle(t *testing.T) {
	assert.Equal(t, "Disk pressure on minipc", normalizeGeneratedSessionTitle("  \"Disk pressure on minipc.\"  "))
	assert.Equal(t, "", normalizeGeneratedSessionTitle("   \n"))
	long := normalizeGeneratedSessionTitle("An extremely verbose session title that keeps going well past the fifty character budget")
	assert.LessOrEqual(t, len([]rune(long)), 53, "should truncate near the 50-char budget (plus ellipsis)")
	assert.Contains(t, long, "...")
}
