package logging

import (
	"bytes"
	"container/ring"
	"strings"
	"testing"
)

func TestLogBroadcasterWriteLogsBlockedSubscriberContext(t *testing.T) {
	b := &LogBroadcaster{
		buffer:      ring.New(DefaultBufferSize),
		subscribers: map[string]chan string{"slow-subscriber": make(chan string)},
	}

	var warnOutput bytes.Buffer
	origWarnWriter := broadcastWarnWriter
	broadcastWarnWriter = &warnOutput
	defer func() {
		broadcastWarnWriter = origWarnWriter
	}()

	n, err := b.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write() error = %v, want nil", err)
	}
	if n != len("hello world") {
		t.Fatalf("Write() bytes = %d, want %d", n, len("hello world"))
	}

	got := warnOutput.String()
	if !strings.Contains(got, "subscriber_blocked") {
		t.Fatalf("blocked subscriber warning missing reason: %q", got)
	}
	if !strings.Contains(got, "subscriber_id=slow-subscriber") {
		t.Fatalf("blocked subscriber warning missing id context: %q", got)
	}
	if !strings.Contains(got, "action=drop_message") {
		t.Fatalf("blocked subscriber warning missing action context: %q", got)
	}
}
