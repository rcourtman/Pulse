package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// auditLogger emits append-only, hash-chained audit events.
type auditLogger struct {
	mu       sync.Mutex
	file     *os.File
	logger   zerolog.Logger
	prevHash []byte
	sequence uint64
}

// AuditEvent captures a single security-relevant action.
type AuditEvent struct {
	Sequence      uint64    `json:"seq"`
	Timestamp     time.Time `json:"ts"`
	EventType     string    `json:"event_type"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	PeerUID       *uint32   `json:"peer_uid,omitempty"`
	PeerGID       *uint32   `json:"peer_gid,omitempty"`
	PeerPID       *uint32   `json:"peer_pid,omitempty"`
	RemoteAddr    string    `json:"remote_addr,omitempty"`
	Command       string    `json:"command,omitempty"`
	Args          []string  `json:"args,omitempty"`
	Target        string    `json:"target,omitempty"`
	Decision      string    `json:"decision,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	Limiter       string    `json:"limiter,omitempty"`
	ExitCode      *int      `json:"exit_code,omitempty"`
	DurationMs    *int64    `json:"duration_ms,omitempty"`
	StdoutHash    string    `json:"stdout_sha256,omitempty"`
	StderrHash    string    `json:"stderr_sha256,omitempty"`
	Error         string    `json:"error,omitempty"`
	PrevHash      string    `json:"prev_hash"`
	EventHash     string    `json:"event_hash"`
}

// newAuditLogger opens the audit log file and prepares hash chaining.
func newAuditLogger(path string) (*auditLogger, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}

	writer := zerolog.New(file).With().Timestamp().Logger()

	return &auditLogger{
		file:   file,
		logger: writer,
	}, nil
}

// Close flushes and closes the audit log file.
func (a *auditLogger) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return nil
	}
	err := a.file.Close()
	a.file = nil
	return err
}

// LogConnectionAccepted records an authorized connection.
func (a *auditLogger) LogConnectionAccepted(correlationID string, cred *peerCredentials, remote string) {
	event := AuditEvent{
		EventType:     "connection.accepted",
		CorrelationID: correlationID,
		RemoteAddr:    remote,
		Decision:      "allowed",
	}
	event.applyPeer(cred)
	a.log(&event)
}

// LogConnectionDenied records a rejected connection attempt.
func (a *auditLogger) LogConnectionDenied(correlationID string, cred *peerCredentials, remote, reason string) {
	event := AuditEvent{
		EventType:     "connection.denied",
		CorrelationID: correlationID,
		RemoteAddr:    remote,
		Decision:      "denied",
		Reason:        reason,
	}
	event.applyPeer(cred)
	a.log(&event)
}

// LogRateLimitHit records limiter rejections.
func (a *auditLogger) LogRateLimitHit(correlationID string, cred *peerCredentials, remote, limiter string) {
	event := AuditEvent{
		EventType:     "limiter.rejection",
		CorrelationID: correlationID,
		RemoteAddr:    remote,
		Decision:      "denied",
		Limiter:       limiter,
	}
	event.applyPeer(cred)
	a.log(&event)
}

// LogCommandStart records command execution approval.
func (a *auditLogger) LogCommandStart(correlationID string, cred *peerCredentials, remote, target, command string, args []string) {
	event := AuditEvent{
		EventType:     "command.start",
		CorrelationID: correlationID,
		RemoteAddr:    remote,
		Decision:      "allowed",
		Command:       command,
		Args:          args,
		Target:        target,
	}
	event.applyPeer(cred)
	a.log(&event)
}

// LogCommandResult records command completion.
func (a *auditLogger) LogCommandResult(correlationID string, cred *peerCredentials, remote, target, command string, args []string, exitCode int, duration time.Duration, stdoutHash, stderrHash string, execErr error) {
	event := AuditEvent{
		EventType:     "command.finish",
		CorrelationID: correlationID,
		RemoteAddr:    remote,
		Command:       command,
		Args:          args,
		Target:        target,
		ExitCode:      intPtr(exitCode),
		StdoutHash:    stdoutHash,
		StderrHash:    stderrHash,
	}
	event.applyPeer(cred)
	if duration > 0 {
		ms := duration.Milliseconds()
		event.DurationMs = int64Ptr(ms)
	}
	if execErr != nil {
		event.Error = execErr.Error()
		event.Decision = "failed"
	} else {
		event.Decision = "completed"
	}
	a.log(&event)
}

// LogValidationFailure records validator rejections.
func (a *auditLogger) LogValidationFailure(correlationID string, cred *peerCredentials, remote, command string, args []string, reason string) {
	event := AuditEvent{
		EventType:     "command.validation_failed",
		CorrelationID: correlationID,
		RemoteAddr:    remote,
		Command:       command,
		Args:          args,
		Decision:      "denied",
		Reason:        reason,
	}
	event.applyPeer(cred)
	a.log(&event)
}

func (e *AuditEvent) applyPeer(cred *peerCredentials) {
	if cred == nil {
		return
	}
	e.PeerUID = uint32Ptr(cred.uid)
	e.PeerGID = uint32Ptr(cred.gid)
	e.PeerPID = uint32Ptr(cred.pid)
}

// log persists the event with hash chaining.
func (a *auditLogger) log(event *AuditEvent) {
	if event == nil {
		log.Error().Msg("audit log called with nil event")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.sequence++
	event.Sequence = a.sequence

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}

	event.PrevHash = hex.EncodeToString(a.prevHash)

	payload, err := eventMarshalForHash(event)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal audit event")
		return
	}

	sum := sha256.Sum256(append(a.prevHash, payload...))
	a.prevHash = sum[:]
	event.EventHash = hex.EncodeToString(sum[:])

	a.logger.Info().Fields(eventToMap(event)).Send()
}

func eventMarshalForHash(event *AuditEvent) ([]byte, error) {
	clone := *event
	clone.EventHash = ""
	return json.Marshal(clone)
}

func eventToMap(event *AuditEvent) map[string]interface{} {
	m := map[string]interface{}{
		"ts":             event.Timestamp.Format(time.RFC3339Nano),
		"event_type":     event.EventType,
		"seq":            event.Sequence,
		"prev_hash":      event.PrevHash,
		"event_hash":     event.EventHash,
		"decision":       event.Decision,
		"correlation_id": event.CorrelationID,
	}

	if event.PeerUID != nil {
		m["peer_uid"] = *event.PeerUID
	}
	if event.PeerGID != nil {
		m["peer_gid"] = *event.PeerGID
	}
	if event.PeerPID != nil {
		m["peer_pid"] = *event.PeerPID
	}
	if event.RemoteAddr != "" {
		m["remote_addr"] = event.RemoteAddr
	}
	if event.Command != "" {
		m["command"] = event.Command
	}
	if len(event.Args) > 0 {
		m["args"] = event.Args
	}
	if event.Target != "" {
		m["target"] = event.Target
	}
	if event.Reason != "" {
		m["reason"] = event.Reason
	}
	if event.Limiter != "" {
		m["limiter"] = event.Limiter
	}
	if event.ExitCode != nil {
		m["exit_code"] = *event.ExitCode
	}
	if event.DurationMs != nil {
		m["duration_ms"] = *event.DurationMs
	}
	if event.StdoutHash != "" {
		m["stdout_sha256"] = event.StdoutHash
	}
	if event.StderrHash != "" {
		m["stderr_sha256"] = event.StderrHash
	}
	if event.Error != "" {
		m["error"] = event.Error
	}

	return m
}

func uint32Ptr(v uint32) *uint32 {
	value := v
	return &value
}

func intPtr(v int) *int {
	value := v
	return &value
}

func int64Ptr(v int64) *int64 {
	value := v
	return &value
}
