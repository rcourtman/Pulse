package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

type auditRecord map[string]interface{}

func TestAuditLogValidationFailure(t *testing.T) {
	tmp, err := os.CreateTemp("", "audit-test-*.log")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	logger := newAuditLogger(path)

	cred := &peerCredentials{uid: 1000, gid: 1000, pid: 4242}
	logger.LogValidationFailure("corr-123", cred, "remote", "get_temperature", []string{"node"}, "invalid_node")
	logger.Close()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatalf("expected at least one audit entry (file may be empty)")
	}

	line := scanner.Bytes()
	if len(line) == 0 {
		t.Fatalf("empty line in audit log")
	}

	t.Logf("Audit log line: %s", string(line))

	var record auditRecord
	if err := json.Unmarshal(line, &record); err != nil {
		t.Fatalf("unmarshal (line=%s): %v", string(line), err)
	}

	t.Logf("Parsed record: %+v", record)

	if record["event_type"] != "command.validation_failed" {
		t.Fatalf("unexpected event_type: %v (full record: %+v)", record["event_type"], record)
	}
	if record["correlation_id"] != "corr-123" {
		t.Fatalf("unexpected correlation id: %v", record["correlation_id"])
	}
	if record["command"] != "get_temperature" {
		t.Fatalf("unexpected command: %v", record["command"])
	}
	if record["reason"] != "invalid_node" {
		t.Fatalf("unexpected reason: %v", record["reason"])
	}
	if record["decision"] != "denied" {
		t.Fatalf("unexpected decision: %v", record["decision"])
	}
	if record["event_hash"] == "" {
		t.Fatalf("expected event_hash to be set")
	}
}

func TestAuditLoggerFallback(t *testing.T) {
	// Try to open a file in a non-existent directory to trigger fallback
	logger := newAuditLogger("/nonexistent/directory/audit.log")
	if logger.file != nil {
		t.Error("expected file to be nil for fallback")
	}

	// Should not panic when logging to fallback
	logger.LogConnectionAccepted("corr-456", &peerCredentials{uid: 0}, "local")
	logger.Close()
}

func TestAuditLoggerAllEvents(t *testing.T) {
	tmp, err := os.CreateTemp("", "audit-test-all-*.log")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	logger := newAuditLogger(path)
	cred := &peerCredentials{uid: 1000, gid: 1000, pid: 4242}

	logger.LogConnectionAccepted("c1", cred, "r1")
	logger.LogConnectionDenied("c2", cred, "r2", "bad token")
	logger.LogRateLimitHit("c3", cred, "r3", "global")
	logger.LogCommandStart("c4", cred, "r4", "t4", "cmd4", []string{"arg4"})
	logger.LogCommandResult("c5", cred, "r5", "t5", "cmd5", []string{"arg5"}, 0, time.Second, "h1", "h2", nil)
	logger.LogCommandResult("c6", cred, "r6", "t6", "cmd6", []string{"arg6"}, 1, time.Second, "", "", errors.New("exec error"))
	logger.LogHTTPRequest("r7", "GET", "/path", 200, "ok")

	// Log with nil creds
	logger.LogConnectionAccepted("c8", nil, "r8")

	// Log with nil logger (should handle gracefully if possible, but it's a pointer receiver)
	// logger.log(nil) // Already tested indirectly via internal calls if event is nil

	logger.Close()
	// Double close should be fine
	logger.Close()

	// Basic verification that all lines are present
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if count != 8 {
		t.Errorf("expected 8 audit entries, got %d", count)
	}
}

func TestAuditEvent_ApplyPeer_Nil(t *testing.T) {
	e := &AuditEvent{}
	e.applyPeer(nil)
	if e.PeerUID != nil {
		t.Error("expected PeerUID to be nil")
	}
}

func TestAuditLogNilEvent(t *testing.T) {
	logger := newAuditLogger("") // This might fail or use fallback
	// Calling a.log(nil) directly to test the nil check
	logger.log(nil)
}
