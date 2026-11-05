package updates

import (
	"errors"
	"testing"
)

func TestProUpdateTargetUsesExecutableMetadata(t *testing.T) {
	original := proRuntimeTarget
	t.Cleanup(func() { proRuntimeTarget = original })
	for _, target := range []string{"linux-armv6", "linux-armv7", "linux-amd64"} {
		proRuntimeTarget = func() (string, error) { return target, nil }
		if got := proUpdateTarget(); got != target {
			t.Fatalf("got %s want %s", got, target)
		}
	}
	proRuntimeTarget = func() (string, error) { return "", errors.New("missing GOARM") }
	if proUpdateTarget() != "" {
		t.Fatal("guessed unsupported target")
	}
}
