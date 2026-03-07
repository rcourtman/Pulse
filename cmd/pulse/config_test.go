package main

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

func TestGetPassphrase_FromEnv(t *testing.T) {
	env := newTestCLIEnv()
	t.Setenv("PULSE_PASSPHRASE", "from-env")
	env.Passphrase = ""

	got := pulsecli.GetPassphrase(env.ConfigDeps(), "ignored", false)
	if got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestGetPassphrase_FromFlag(t *testing.T) {
	env := newTestCLIEnv()
	t.Setenv("PULSE_PASSPHRASE", "")
	env.Passphrase = "from-flag"

	got := pulsecli.GetPassphrase(env.ConfigDeps(), "ignored", false)
	if got != "from-flag" {
		t.Fatalf("got %q", got)
	}
}
