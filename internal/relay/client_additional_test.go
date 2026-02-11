package relay

import (
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Enabled {
		t.Error("Enabled: got true, want false")
	}
	if cfg.ServerURL != DefaultServerURL {
		t.Errorf("ServerURL: got %q, want %q", cfg.ServerURL, DefaultServerURL)
	}
}

func TestClientClose(t *testing.T) {
	t.Run("returns when done is closed", func(t *testing.T) {
		c := &Client{done: make(chan struct{})}
		close(c.done)
		c.Close()
	})

	t.Run("invokes cancel when configured", func(t *testing.T) {
		cancelCalled := false
		c := &Client{
			done: make(chan struct{}),
			cancel: func() {
				cancelCalled = true
			},
		}
		close(c.done)

		c.Close()

		if !cancelCalled {
			t.Error("expected cancel to be called")
		}
	})
}

func TestLicenseErrorFormattingAndDetection(t *testing.T) {
	err := &licenseError{code: ErrCodeLicenseExpired, message: "expired"}
	if got, want := err.Error(), "license error (LICENSE_EXPIRED): expired"; got != want {
		t.Errorf("Error(): got %q, want %q", got, want)
	}
	if !isLicenseError(err) {
		t.Error("isLicenseError() = false, want true")
	}
	if isLicenseError(errors.New("other")) {
		t.Error("isLicenseError() = true for non-license error")
	}
}

func TestQueueFrame(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("queues frame when capacity is available", func(t *testing.T) {
		sendCh := make(chan []byte, 1)
		queueFrame(sendCh, NewPingFrame(), logger)
		if len(sendCh) != 1 {
			t.Errorf("send channel length: got %d, want 1", len(sendCh))
		}
	})

	t.Run("drops frame when channel is full", func(t *testing.T) {
		sendCh := make(chan []byte, 1)
		sendCh <- []byte("already full")

		queueFrame(sendCh, NewPongFrame(), logger)

		if len(sendCh) != 1 {
			t.Errorf("send channel length: got %d, want 1", len(sendCh))
		}
	})

	t.Run("drops frame when encoding fails", func(t *testing.T) {
		sendCh := make(chan []byte, 1)
		tooLargePayload := make([]byte, MaxPayloadSize+1)

		queueFrame(sendCh, NewFrame(FrameData, 1, tooLargePayload), logger)

		if len(sendCh) != 0 {
			t.Errorf("send channel length: got %d, want 0", len(sendCh))
		}
	})
}

func TestClientSendPushNotificationChannelFull(t *testing.T) {
	ch := make(chan []byte, 1)
	ch <- []byte("occupied")

	c := &Client{connected: true, sendCh: ch}
	notification := NewPatrolFindingNotification("finding-test", "warning", "capacity", "Queue Saturated")

	err := c.SendPushNotification(notification)
	if err == nil {
		t.Fatal("expected error when send channel is full")
	}
	if !strings.Contains(err.Error(), "send channel full") {
		t.Errorf("error: got %q, want to contain %q", err.Error(), "send channel full")
	}
}
