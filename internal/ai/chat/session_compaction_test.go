package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func seedCompactionSession(t *testing.T, store *SessionStore, sessionID string) {
	t.Helper()
	for turn := 1; turn <= 5; turn++ {
		require.NoError(t, store.AddMessage(sessionID, Message{
			ID:      "u-compaction-test",
			Role:    "user",
			Content: "Prompt turn " + string(rune('0'+turn)),
		}))
		require.NoError(t, store.AddMessage(sessionID, Message{
			ID:      "a-compaction-test",
			Role:    "assistant",
			Content: "Answer turn " + string(rune('0'+turn)),
		}))
	}
}

func TestSessionStoreCompactWithSummaryRewritesOldTurns(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)
	session, err := store.Create()
	require.NoError(t, err)
	seedCompactionSession(t, store, session.ID)

	result, err := store.CompactWithSummary(session.ID, "The earlier turns inspected storage health.", 4, "openrouter:test-model")
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, "compacted", result.Status)
	require.Equal(t, 10, result.OriginalMessageCount)
	require.Equal(t, 9, result.CompactedMessageCount)
	require.Equal(t, 2, result.CompactedMessages)
	require.Equal(t, 8, result.KeptRecentMessages)

	messages, err := store.GetMessages(session.ID)
	require.NoError(t, err)
	require.Len(t, messages, 9)
	require.Equal(t, "assistant", messages[0].Role)
	require.Contains(t, messages[0].Content, "Session compacted.")
	require.Contains(t, messages[0].Content, "The earlier turns inspected storage health.")
	require.Equal(t, "openrouter:test-model", messages[0].Model)
	require.Equal(t, "Prompt turn 2", messages[1].Content)
	require.NotContains(t, strings.Join(messageContents(messages), "\n"), "Prompt turn 1")
}

func TestServiceSummarizeSessionUsesProviderAndCompactsTranscript(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)
	session, err := store.Create()
	require.NoError(t, err)
	seedCompactionSession(t, store, session.ID)
	require.NoError(t, store.AddMessage(session.ID, Message{
		ID:      "secret-turn",
		Role:    "user",
		Content: "api_key: sk-live-secret-value",
	}))
	require.NoError(t, store.AddMessage(session.ID, Message{
		ID:      "secret-answer",
		Role:    "assistant",
		Content: "Acknowledged.",
	}))

	provider := &MockProvider{}
	provider.On("Chat", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		if !strings.Contains(req.System, "compact Pulse Assistant chat sessions") {
			return false
		}
		if len(req.Messages) != 1 {
			return false
		}
		prompt := req.Messages[0].Content
		return strings.Contains(prompt, "Prompt turn 1") &&
			strings.Contains(prompt, "[REDACTED]") &&
			!strings.Contains(prompt, "sk-live-secret-value")
	})).Return(&providers.ChatResponse{
		Content:      "Earlier turns established storage health and a final secret-bearing prompt was redacted.",
		Model:        "openrouter:test-model",
		InputTokens:  42,
		OutputTokens: 24,
	}, nil).Once()
	provider.On("Name").Return("mock-provider").Maybe()

	service := &Service{
		started:  true,
		sessions: store,
		provider: provider,
	}

	result, err := service.SummarizeSession(context.Background(), session.ID)
	require.NoError(t, err)
	require.Equal(t, true, result["success"])
	require.Equal(t, "compacted", result["status"])
	require.Equal(t, session.ID, result["session_id"])

	messages, err := store.GetMessages(session.ID)
	require.NoError(t, err)
	require.Contains(t, messages[0].Content, "Earlier turns established storage health")
	require.NotContains(t, messages[0].Content, "sk-live-secret-value")
	provider.AssertExpectations(t)
}

func TestServiceSummarizeSessionRedactsResourceIdentifiersForCloud(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)
	session, err := store.Create()
	require.NoError(t, err)
	seedCompactionSession(t, store, session.ID)
	// The persisted transcript carries the raw resource hostname the user typed,
	// regardless of how the live turn was redacted. Compaction must not ship it
	// to a cloud model.
	require.NoError(t, store.AddMessage(session.ID, Message{
		ID:      "vault-turn",
		Role:    "user",
		Content: "check vault.lan health please",
	}))
	require.NoError(t, store.AddMessage(session.ID, Message{
		ID:      "vault-answer",
		Role:    "assistant",
		Content: "Acknowledged.",
	}))

	unifiedProvider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeAgent: {{
			ID:       "agent/vault",
			Name:     "vault",
			Type:     unifiedresources.ResourceTypeAgent,
			Status:   unifiedresources.StatusOnline,
			Tags:     []string{"secret"}, // -> Restricted -> identifiers redacted
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"vault.lan"}},
		}},
	}}

	var capturedPrompt string
	provider := &MockProvider{}
	provider.On("Chat", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		if len(req.Messages) == 1 {
			capturedPrompt = req.Messages[0].Content
		}
		return true
	})).Return(&providers.ChatResponse{
		Content: "Compacted summary.",
		Model:   "openrouter:test-model",
	}, nil).Once()
	provider.On("Name").Return("mock-provider").Maybe()

	service := &Service{
		started:                 true,
		sessions:                store,
		provider:                provider,
		cfg:                     &config.AIConfig{ChatModel: "openrouter:test-model", CloudContextPrivacy: config.CloudContextPrivacyRedacted},
		unifiedResourceProvider: unifiedProvider,
	}

	_, err = service.SummarizeSession(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotEmpty(t, capturedPrompt)
	// The transcript reached the model (proves the turn ran) but the resource
	// hostname was redacted at the model boundary by the privacy dial.
	require.Contains(t, capturedPrompt, "Prompt turn 1")
	require.NotContains(t, capturedPrompt, "vault.lan")
	provider.AssertExpectations(t)
}

func messageContents(messages []Message) []string {
	contents := make([]string, 0, len(messages))
	for _, message := range messages {
		contents = append(contents, message.Content)
	}
	return contents
}
