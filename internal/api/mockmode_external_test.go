package api_test

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
)

func setMockModeForTest(t *testing.T, enabled bool) {
	t.Helper()
	testutil.SetMockMode(t, enabled)
}
