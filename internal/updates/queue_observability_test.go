package updates

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestUpdateQueueEnqueueRejectLogsStructuredContext(t *testing.T) {
	var buf bytes.Buffer
	restore := setUpdateQueueTestLogger(&buf)
	defer restore()

	queue := NewUpdateQueue()
	job, accepted := queue.Enqueue("https://example.com/update1.tar.gz")
	if !accepted || job == nil {
		t.Fatal("expected first enqueue to succeed")
	}
	if !queue.MarkRunning(job.ID) {
		t.Fatal("expected mark running to succeed")
	}

	buf.Reset()

	if _, accepted := queue.Enqueue("https://example.com/update2.tar.gz"); accepted {
		t.Fatal("expected second enqueue to be rejected")
	}

	entry := singleLogEntry(t, buf.String())
	assertLogStringField(t, entry, "component", updatesQueueComponent)
	assertLogStringField(t, entry, "action", "enqueue_rejected")
	assertLogStringField(t, entry, "current_job_id", job.ID)
	assertLogStringField(t, entry, "current_state", string(JobStateRunning))
	assertLogStringField(t, entry, "requested_download_url", "https://example.com/update2.tar.gz")
}

func TestUpdateQueueMarkCompletedFailureLogsStructuredContext(t *testing.T) {
	var buf bytes.Buffer
	restore := setUpdateQueueTestLogger(&buf)
	defer restore()

	queue := NewUpdateQueue()
	job, accepted := queue.Enqueue("https://example.com/update.tar.gz")
	if !accepted || job == nil {
		t.Fatal("expected enqueue to succeed")
	}
	if !queue.MarkRunning(job.ID) {
		t.Fatal("expected mark running to succeed")
	}

	buf.Reset()

	queue.MarkCompleted(job.ID, errors.New("boom"))

	entry := singleLogEntry(t, buf.String())
	assertLogStringField(t, entry, "component", updatesQueueComponent)
	assertLogStringField(t, entry, "action", "complete_failed")
	assertLogStringField(t, entry, "job_id", job.ID)
	assertLogStringField(t, entry, "job_state", string(JobStateFailed))
	assertLogStringField(t, entry, "error", "boom")
	if _, ok := entry["duration"]; !ok {
		t.Fatal("expected duration field in failure log")
	}
}

func TestUpdateQueueCancelLogsStructuredContext(t *testing.T) {
	var buf bytes.Buffer
	restore := setUpdateQueueTestLogger(&buf)
	defer restore()

	queue := NewUpdateQueue()
	job, accepted := queue.Enqueue("https://example.com/update.tar.gz")
	if !accepted || job == nil {
		t.Fatal("expected enqueue to succeed")
	}
	if !queue.MarkRunning(job.ID) {
		t.Fatal("expected mark running to succeed")
	}

	buf.Reset()

	if !queue.Cancel(job.ID) {
		t.Fatal("expected cancel to succeed")
	}

	entry := singleLogEntry(t, buf.String())
	assertLogStringField(t, entry, "component", updatesQueueComponent)
	assertLogStringField(t, entry, "action", "cancel")
	assertLogStringField(t, entry, "job_id", job.ID)
	assertLogStringField(t, entry, "previous_state", string(JobStateRunning))
	assertLogStringField(t, entry, "job_state", string(JobStateCancelled))
}

func setUpdateQueueTestLogger(buf *bytes.Buffer) func() {
	original := log.Logger
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	return func() {
		log.Logger = original
	}
}

func singleLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one log line, got %d: %q", len(lines), raw)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	return parsed
}

func assertLogStringField(t *testing.T, entry map[string]any, field, want string) {
	t.Helper()

	raw, ok := entry[field]
	if !ok {
		t.Fatalf("missing field %q in log entry: %+v", field, entry)
	}

	got, ok := raw.(string)
	if !ok {
		t.Fatalf("field %q has non-string value %#v", field, raw)
	}

	if got != want {
		t.Fatalf("field %q = %q, want %q", field, got, want)
	}
}
