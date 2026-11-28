package metrics

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestMain(m *testing.M) {
	// Silence the package-global logger once before tests start so parallel
	// tests do not mutate log.Logger concurrently. Serial tests that capture
	// log output (and restore log.Logger via t.Cleanup) run before any
	// t.Parallel() test begins.
	log.Logger = zerolog.Nop()
	os.Exit(m.Run())
}
