package testutil

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestScopedMockEstateRestoresModeConfigAndEnvironment(t *testing.T) {
	original, enabled := mock.GetConfig(), mock.IsMockEnabled()
	flag, hadFlag := os.LookupEnv("PULSE_MOCK_MODE")
	t.Run("small estate", func(t *testing.T) {
		cfg := mock.MockConfig{NodeCount: 2, VMsPerNode: 1, UpdateInterval: time.Second}
		SetMockModeWithConfig(t, true, cfg)
		if got := mock.GetConfig(); !mock.IsMockEnabled() || got.NodeCount != 2 || got.VMsPerNode != 1 {
			t.Fatalf("unexpected scoped estate: enabled=%v config=%+v", mock.IsMockEnabled(), got)
		}
	})
	if got := mock.GetConfig(); !reflect.DeepEqual(got, original) || mock.IsMockEnabled() != enabled {
		t.Fatalf("mock runtime leaked from subtest: enabled=%v config=%+v", mock.IsMockEnabled(), got)
	}
	if got, ok := os.LookupEnv("PULSE_MOCK_MODE"); got != flag || ok != hadFlag {
		t.Fatalf("mock mode environment changed from (%q, %v) to (%q, %v)", flag, hadFlag, got, ok)
	}
}
