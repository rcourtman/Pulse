package main

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

func TestGetPassphrase_FromEnv(t *testing.T) {
	t.Setenv("PULSE_PASSPHRASE", "from-env")
	passphrase = ""
	t.Cleanup(func() { passphrase = "" })

	got := pulsecli.GetPassphrase(currentConfigDeps(), "ignored", false)
	if got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestGetPassphrase_FromFlag(t *testing.T) {
	t.Setenv("PULSE_PASSPHRASE", "")
	passphrase = "from-flag"
	t.Cleanup(func() { passphrase = "" })

	got := pulsecli.GetPassphrase(currentConfigDeps(), "ignored", false)
	if got != "from-flag" {
		t.Fatalf("got %q", got)
	}
}
