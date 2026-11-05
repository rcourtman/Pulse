package main

import (
	"io"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity/identitytest"
)

func TestVerifyAgentRelease(t *testing.T) {
	for _, target := range []string{"linux-amd64", "windows-amd64", "darwin-arm64"} {
		path := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", Target: target})
		if err := verify([]string{"--version", "6.2.1", target, path}, io.Discard); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"--version", "6.2.10", target, path}, {"--version", "6.2.1", "linux-armv6", path}, {"--version", "6.2.1", target, path, target, path + ".missing"}, {"--version", "6.2.1", target}, {target, path}, {"--version", "6.2.1"}} {
			if err := verify(args, io.Discard); err == nil {
				t.Fatalf("accepted invalid artifact request %q", args)
			}
		}
	}
	path := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", Target: "windows-amd64", Stamp: "unstamped"})
	if err := verify([]string{"--version", "6.2.1", "windows-amd64", path}, io.Discard); err == nil {
		t.Fatal("accepted unstamped native replacement")
	}
}
