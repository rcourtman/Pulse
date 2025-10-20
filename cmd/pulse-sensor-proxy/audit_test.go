package main

import (
    "bufio"
    "encoding/json"
    "os"
    "testing"
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

    logger, err := newAuditLogger(path)
    if err != nil {
        t.Fatalf("newAuditLogger: %v", err)
    }

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
        t.Fatalf("expected at least one audit entry")
    }

    var record auditRecord
    if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    if record["event_type"] != "command.validation_failed" {
        t.Fatalf("unexpected event_type: %v", record["event_type"])
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
