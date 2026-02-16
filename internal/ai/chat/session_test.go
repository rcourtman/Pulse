package chat

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-store-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store, err := NewSessionStore(tempDir)
	require.NoError(t, err)

	t.Run("Create and Get", func(t *testing.T) {
		session, err := store.Create()
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)
		assert.Empty(t, session.Title)

		retrieved, err := store.Get(session.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrieved.ID)
	})

	t.Run("List", func(t *testing.T) {
		// New store for isolation
		d := filepath.Join(tempDir, "list-test")
		s, _ := NewSessionStore(d)

		s1, _ := s.Create()
		time.Sleep(10 * time.Millisecond) // Ensure time difference
		s2, _ := s.Create()

		sessions, err := s.List()
		require.NoError(t, err)
		require.Len(t, sessions, 2)
		// Should be newest first
		assert.Equal(t, s2.ID, sessions[0].ID)
		assert.Equal(t, s1.ID, sessions[1].ID)
	})

	t.Run("Delete", func(t *testing.T) {
		session, _ := store.Create()
		err := store.Delete(session.ID)
		assert.NoError(t, err)

		_, err = store.Get(session.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("AddMessage and Title Generation", func(t *testing.T) {
		session, _ := store.Create()
		msg := Message{
			Role:    "user",
			Content: "What is the status of node-1?",
		}
		err := store.AddMessage(session.ID, msg)
		require.NoError(t, err)

		updated, _ := store.Get(session.ID)
		assert.Equal(t, "What is the status of node-1?", updated.Title)
		assert.Equal(t, 1, updated.MessageCount)

		messages, err := store.GetMessages(session.ID)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, "What is the status of node-1?", messages[0].Content)
	})

	t.Run("UpdateLastMessage", func(t *testing.T) {
		session, _ := store.Create()
		_ = store.AddMessage(session.ID, Message{Role: "assistant", Content: "Thinking..."})

		updatedMsg := Message{Role: "assistant", Content: "Resolved."}
		err := store.UpdateLastMessage(session.ID, updatedMsg)
		require.NoError(t, err)

		messages, _ := store.GetMessages(session.ID)
		assert.Equal(t, "Resolved.", messages[0].Content)
	})

	t.Run("EnsureSession", func(t *testing.T) {
		session, err := store.EnsureSession("")
		assert.NoError(t, err)
		assert.NotEmpty(t, session.ID)

		sessionFixed, err := store.EnsureSession("fixed-id")
		assert.NoError(t, err)
		assert.Equal(t, "fixed-id", sessionFixed.ID)

		retrieved, err := store.EnsureSession("fixed-id")
		assert.NoError(t, err)
		assert.Equal(t, sessionFixed.ID, retrieved.ID)
	})
}

func TestGenerateTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Short message", "Short message"},
		{"This is a very long message that should definitely be truncated because it exceeds the fifty character limit", "This is a very long message that should..."},
		{"Multiple    spaces    cleaned", "Multiple spaces cleaned"},
		{"New\nLines\nRemoved", "New Lines Removed"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, generateTitle(tt.input))
	}
}
