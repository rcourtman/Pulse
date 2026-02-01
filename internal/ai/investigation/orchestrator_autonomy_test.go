package investigation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingChatService records the AutonomousMode passed via ExecuteRequest
// and tracks any direct SetAutonomousMode calls.
type capturingChatService struct {
	stubChatService
	autonomousCalls  []bool           // direct SetAutonomousMode calls
	capturedRequests []ExecuteRequest // requests passed to ExecuteStream
}

func (c *capturingChatService) SetAutonomousMode(enabled bool) {
	c.autonomousCalls = append(c.autonomousCalls, enabled)
}

func (c *capturingChatService) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	c.capturedRequests = append(c.capturedRequests, req)
	if c.execute != nil {
		return c.execute(callback)
	}
	return nil
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

	// Autonomous mode should be passed via the request, not via SetAutonomousMode
	require.Len(t, chat.capturedRequests, 1)
	require.NotNil(t, chat.capturedRequests[0].AutonomousMode)
	assert.True(t, *chat.capturedRequests[0].AutonomousMode, "full autonomy should set autonomous mode to true in request")

	// SetAutonomousMode should NOT be called on the chat service
	assert.Empty(t, chat.autonomousCalls, "should not mutate shared chat service autonomous mode")
}

func TestInvestigateFinding_AutonomousMode_Approval(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "approval")
	require.NoError(t, err)

	require.Len(t, chat.capturedRequests, 1)
	require.NotNil(t, chat.capturedRequests[0].AutonomousMode)
	assert.False(t, *chat.capturedRequests[0].AutonomousMode, "approval mode should set autonomous mode to false in request")
	assert.Empty(t, chat.autonomousCalls)
}

func TestInvestigateFinding_AutonomousMode_Assisted(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "assisted")
	require.NoError(t, err)

	require.Len(t, chat.capturedRequests, 1)
	require.NotNil(t, chat.capturedRequests[0].AutonomousMode)
	assert.False(t, *chat.capturedRequests[0].AutonomousMode, "assisted mode should set autonomous mode to false in request")
	assert.Empty(t, chat.autonomousCalls)
}

func TestInvestigateFinding_AutonomousMode_Monitor(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "monitor")
	require.NoError(t, err)

	require.Len(t, chat.capturedRequests, 1)
	require.NotNil(t, chat.capturedRequests[0].AutonomousMode)
	assert.False(t, *chat.capturedRequests[0].AutonomousMode, "monitor mode should set autonomous mode to false in request")
	assert.Empty(t, chat.autonomousCalls)
}

func TestInvestigateFinding_AutonomousMode_Empty(t *testing.T) {
	chat := newCapturingChat()
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "f1", Severity: "warning"}}
	o := NewOrchestrator(chat, store, findings, nil, DefaultConfig())

	err := o.InvestigateFinding(context.Background(), findings.finding, "")
	require.NoError(t, err)

	require.Len(t, chat.capturedRequests, 1)
	require.NotNil(t, chat.capturedRequests[0].AutonomousMode)
	assert.False(t, *chat.capturedRequests[0].AutonomousMode, "empty autonomy level should set autonomous mode to false in request")
	assert.Empty(t, chat.autonomousCalls)
}
