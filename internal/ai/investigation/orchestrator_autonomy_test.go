package investigation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingChatService records the boolean passed to SetAutonomousMode.
type capturingChatService struct {
	stubChatService
	autonomousCalls []bool
}

func (c *capturingChatService) SetAutonomousMode(enabled bool) {
	c.autonomousCalls = append(c.autonomousCalls, enabled)
}

func newCapturingChat() *capturingChatService {
	cs := &capturingChatService{}
	cs.execute = func(cb StreamCallback) error {
		// Minimal response so the investigation completes quickly
		cb(StreamEvent{Type: "content", Data: []byte(`{"text": "CANNOT_FIX: nothing to do"}`)})
		return nil
	}
	return cs
}

func TestInvestigateFinding_AutonomousMode_Full(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "full")
	require.NoError(t, err)

	// First call should be SetAutonomousMode(true), then deferred false
	require.Len(t, chat.autonomousCalls, 2, "expected two SetAutonomousMode calls (set + deferred reset)")
	assert.True(t, chat.autonomousCalls[0], "full autonomy should set autonomous mode to true")
	assert.False(t, chat.autonomousCalls[1], "deferred reset should set autonomous mode to false")
}

func TestInvestigateFinding_AutonomousMode_Approval(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "approval")
	require.NoError(t, err)

	require.Len(t, chat.autonomousCalls, 2)
	assert.False(t, chat.autonomousCalls[0], "approval mode should set autonomous mode to false")
	assert.False(t, chat.autonomousCalls[1], "deferred reset should set autonomous mode to false")
}

func TestInvestigateFinding_AutonomousMode_Assisted(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "assisted")
	require.NoError(t, err)

	require.Len(t, chat.autonomousCalls, 2)
	assert.False(t, chat.autonomousCalls[0], "assisted mode should set autonomous mode to false")
	assert.False(t, chat.autonomousCalls[1])
}

func TestInvestigateFinding_AutonomousMode_Monitor(t *testing.T) {
	// Monitor mode normally wouldn't reach InvestigateFinding (blocked by ShouldInvestigate),
	// but if called directly, it should still set autonomous mode to false.
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "monitor")
	require.NoError(t, err)

	require.Len(t, chat.autonomousCalls, 2)
	assert.False(t, chat.autonomousCalls[0], "monitor mode should set autonomous mode to false")
	assert.False(t, chat.autonomousCalls[1])
}

func TestInvestigateFinding_AutonomousMode_Empty(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "")
	require.NoError(t, err)

	require.Len(t, chat.autonomousCalls, 2)
	assert.False(t, chat.autonomousCalls[0], "empty autonomy level should set autonomous mode to false")
	assert.False(t, chat.autonomousCalls[1])
}
