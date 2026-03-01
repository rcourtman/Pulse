package agentexec

import (
	"encoding/json"
	"testing"
)

func TestDeployPreflightPayloadRoundTrip(t *testing.T) {
	payload := DeployPreflightPayload{
		RequestID:   "req-1",
		JobID:       "job-1",
		PulseURL:    "http://10.0.0.1:7655",
		MaxParallel: 2,
		Timeout:     120,
		Targets: []DeployPreflightTarget{
			{TargetID: "t1", NodeName: "pve-b", NodeIP: "10.0.0.2"},
			{TargetID: "t2", NodeName: "pve-c", NodeIP: "10.0.0.3"},
		},
	}

	msg, err := NewMessage(MsgTypeDeployPreflight, payload.RequestID, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.Type != MsgTypeDeployPreflight {
		t.Errorf("expected type %s, got %s", MsgTypeDeployPreflight, msg.Type)
	}
	if msg.ID != "req-1" {
		t.Errorf("expected ID req-1, got %s", msg.ID)
	}

	var decoded DeployPreflightPayload
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if decoded.JobID != "job-1" {
		t.Errorf("expected job_id job-1, got %s", decoded.JobID)
	}
	if len(decoded.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(decoded.Targets))
	}
	if decoded.Targets[0].NodeIP != "10.0.0.2" {
		t.Errorf("expected node_ip 10.0.0.2, got %s", decoded.Targets[0].NodeIP)
	}
}

func TestDeployInstallPayloadRoundTrip(t *testing.T) {
	payload := DeployInstallPayload{
		RequestID:   "req-2",
		JobID:       "job-1",
		PulseURL:    "http://10.0.0.1:7655",
		MaxParallel: 2,
		Timeout:     300,
		Targets: []DeployInstallTarget{
			{TargetID: "t1", NodeName: "pve-b", NodeIP: "10.0.0.2", Arch: "amd64", BootstrapToken: "tok-secret"},
		},
	}

	msg, err := NewMessage(MsgTypeDeployInstall, payload.RequestID, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}

	var decoded DeployInstallPayload
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if decoded.Targets[0].BootstrapToken != "tok-secret" {
		t.Errorf("expected bootstrap_token tok-secret, got %s", decoded.Targets[0].BootstrapToken)
	}
	if decoded.Targets[0].Arch != "amd64" {
		t.Errorf("expected arch amd64, got %s", decoded.Targets[0].Arch)
	}
}

func TestDeployProgressPayloadRoundTrip(t *testing.T) {
	payload := DeployProgressPayload{
		RequestID: "req-1",
		JobID:     "job-1",
		TargetID:  "t1",
		Phase:     DeployPhasePreflightSSH,
		Status:    DeployStepOK,
		Message:   "SSH reachable",
		Final:     false,
	}

	msg, err := NewMessage(MsgTypeDeployProgress, payload.RequestID, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}

	var decoded DeployProgressPayload
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if decoded.Phase != DeployPhasePreflightSSH {
		t.Errorf("expected phase %s, got %s", DeployPhasePreflightSSH, decoded.Phase)
	}
	if decoded.Status != DeployStepOK {
		t.Errorf("expected status %s, got %s", DeployStepOK, decoded.Status)
	}
	if decoded.Final {
		t.Error("expected final=false")
	}
}

func TestDeployProgressPayloadWithData(t *testing.T) {
	result := PreflightResultData{
		Arch:           "amd64",
		HasAgent:       false,
		PulseReachable: true,
		SSHReachable:   true,
	}
	resultJSON, _ := json.Marshal(result)

	payload := DeployProgressPayload{
		RequestID: "req-1",
		JobID:     "job-1",
		TargetID:  "t1",
		Phase:     DeployPhasePreflightComplete,
		Status:    DeployStepOK,
		Message:   "Ready for deployment",
		Data:      string(resultJSON),
		Final:     false,
	}

	msg, err := NewMessage(MsgTypeDeployProgress, payload.RequestID, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}

	var decoded DeployProgressPayload
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	var decodedResult PreflightResultData
	if err := json.Unmarshal([]byte(decoded.Data), &decodedResult); err != nil {
		t.Fatalf("Unmarshal preflight result: %v", err)
	}
	if decodedResult.Arch != "amd64" {
		t.Errorf("expected arch amd64, got %s", decodedResult.Arch)
	}
	if decodedResult.HasAgent {
		t.Error("expected has_agent=false")
	}
}

func TestDeployCancelPayloadRoundTrip(t *testing.T) {
	payload := DeployCancelPayload{
		RequestID: "req-3",
		JobID:     "job-1",
	}

	msg, err := NewMessage(MsgTypeDeployCancelJob, payload.RequestID, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}

	var decoded DeployCancelPayload
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if decoded.JobID != "job-1" {
		t.Errorf("expected job_id job-1, got %s", decoded.JobID)
	}
}

func TestSubscribeDeployProgress(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })

	ch := s.SubscribeDeployProgress("agent-1", "job-1", 10)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Verify capacity.
	if cap(ch) != 10 {
		t.Errorf("expected capacity 10, got %d", cap(ch))
	}

	// Can send to channel.
	ch <- DeployProgressPayload{JobID: "job-1", Phase: DeployPhasePreflightSSH, Status: DeployStepStarted}

	// Unsubscribe.
	s.UnsubscribeDeployProgress("agent-1", "job-1")

	// After unsubscribe, readLoop won't find the channel (tested implicitly).
	subKey := deploySubKey("agent-1", "job-1")
	s.mu.RLock()
	_, exists := s.deploySubs[subKey]
	s.mu.RUnlock()
	if exists {
		t.Error("expected subscription to be removed")
	}
}

func TestSubscribeDeployProgressDefaultBuffer(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })
	ch := s.SubscribeDeployProgress("agent-1", "job-2", 0)
	if cap(ch) != 64 {
		t.Errorf("expected default capacity 64, got %d", cap(ch))
	}
	s.UnsubscribeDeployProgress("agent-1", "job-2")
}

func TestSubscribeDeployProgressAgentIsolation(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })

	ch1 := s.SubscribeDeployProgress("agent-1", "job-1", 10)
	ch2 := s.SubscribeDeployProgress("agent-2", "job-1", 10)

	// Same jobID, different agents — should be separate channels.
	if ch1 == nil || ch2 == nil {
		t.Fatal("expected non-nil channels")
	}

	// Send to ch1 only.
	ch1 <- DeployProgressPayload{JobID: "job-1", TargetID: "t1"}
	if len(ch2) != 0 {
		t.Error("expected ch2 to be empty — agent isolation violated")
	}

	s.UnsubscribeDeployProgress("agent-1", "job-1")
	s.UnsubscribeDeployProgress("agent-2", "job-1")
}

func TestSendDeployPreflightAgentNotConnected(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })

	err := s.SendDeployPreflight(nil, "missing-agent", DeployPreflightPayload{
		RequestID: "req-1",
		JobID:     "job-1",
	})
	if err == nil {
		t.Fatal("expected error for disconnected agent")
	}
}

func TestSendDeployInstallAgentNotConnected(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })

	err := s.SendDeployInstall(nil, "missing-agent", DeployInstallPayload{
		RequestID: "req-1",
		JobID:     "job-1",
	})
	if err == nil {
		t.Fatal("expected error for disconnected agent")
	}
}

func TestSendDeployCancelAgentNotConnected(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })

	err := s.SendDeployCancel(nil, "missing-agent", DeployCancelPayload{
		RequestID: "req-1",
		JobID:     "job-1",
	})
	if err == nil {
		t.Fatal("expected error for disconnected agent")
	}
}

func TestSendDeployCommandEmptyAgentID(t *testing.T) {
	s := NewServer(func(string, string) bool { return true })

	err := s.SendDeployPreflight(nil, "", DeployPreflightPayload{RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected error for empty agent ID")
	}
}
