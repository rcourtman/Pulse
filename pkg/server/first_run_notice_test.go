package server

import (
	"strings"
	"testing"
)

func TestFormatFirstRunStartupNotice(t *testing.T) {
	t.Run("docker with wildcard bind falls back to localhost and docker command", func(t *testing.T) {
		setupURL, command := formatFirstRunStartupNotice("http", "0.0.0.0", 7655, "", true)
		if setupURL != "http://localhost:7655" {
			t.Fatalf("setupURL = %q, want %q", setupURL, "http://localhost:7655")
		}
		if !strings.Contains(command, "docker exec") || !strings.Contains(command, "/app/pulse bootstrap-token") {
			t.Fatalf("command = %q, want docker exec and container console command", command)
		}
	})

	t.Run("linux with explicit public url uses public url and sudo pulse command", func(t *testing.T) {
		setupURL, command := formatFirstRunStartupNotice("https", "0.0.0.0", 7655, "https://pulse.lan:7655", false)
		if setupURL != "https://pulse.lan:7655" {
			t.Fatalf("setupURL = %q, want %q", setupURL, "https://pulse.lan:7655")
		}
		if command != "sudo pulse bootstrap-token" {
			t.Fatalf("command = %q, want %q", command, "sudo pulse bootstrap-token")
		}
	})

	t.Run("loopback bind preserves host and port", func(t *testing.T) {
		setupURL, _ := formatFirstRunStartupNotice("http", "127.0.0.1", 17655, "", false)
		if setupURL != "http://127.0.0.1:17655" {
			t.Fatalf("setupURL = %q, want %q", setupURL, "http://127.0.0.1:17655")
		}
	})

	t.Run("startup notice message reflects open setup window vs strict token mode", func(t *testing.T) {
		openMsg := firstRunStartupNoticeMessage(true)
		if !strings.Contains(openMsg, "within 10 minutes") {
			t.Fatalf("open-window message = %q, want 10-minute guidance", openMsg)
		}
		strictMsg := firstRunStartupNoticeMessage(false)
		if !strings.Contains(strictMsg, "run the token command to unlock Pulse") {
			t.Fatalf("strict message = %q, want token unlock guidance", strictMsg)
		}
	})
}
