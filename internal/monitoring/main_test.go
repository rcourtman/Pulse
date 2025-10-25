package monitoring

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dataDir, err := os.MkdirTemp("", "monitoring-test-data-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dataDir)

	if err := os.Setenv("PULSE_DATA_DIR", dataDir); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
