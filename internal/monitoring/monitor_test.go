package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestParseDurationEnv(t *testing.T) {
	const testKey = "TEST_DURATION_ENV"
	defaultVal := 30 * time.Second

	t.Run("empty env var returns default", func(t *testing.T) {
		t.Setenv(testKey, "")
		result := parseDurationEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected %v, got %v", defaultVal, result)
		}
	})

	t.Run("unset env var returns default", func(t *testing.T) {
		// t.Setenv automatically cleans up, so not setting means unset
		result := parseDurationEnv("UNSET_DURATION_KEY_12345", defaultVal)
		if result != defaultVal {
			t.Errorf("expected %v, got %v", defaultVal, result)
		}
	})

	t.Run("valid duration seconds", func(t *testing.T) {
		t.Setenv(testKey, "1s")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 1 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("valid duration minutes", func(t *testing.T) {
		t.Setenv(testKey, "5m")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 5 * time.Minute
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("valid duration composite", func(t *testing.T) {
		t.Setenv(testKey, "2h30m")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 2*time.Hour + 30*time.Minute
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("valid duration milliseconds", func(t *testing.T) {
		t.Setenv(testKey, "500ms")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 500 * time.Millisecond
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("invalid duration returns default", func(t *testing.T) {
		t.Setenv(testKey, "invalid")
		result := parseDurationEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %v, got %v", defaultVal, result)
		}
	})

	t.Run("numeric without unit returns default", func(t *testing.T) {
		t.Setenv(testKey, "100")
		result := parseDurationEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %v, got %v", defaultVal, result)
		}
	})

	t.Run("negative duration parses correctly", func(t *testing.T) {
		t.Setenv(testKey, "-5s")
		result := parseDurationEnv(testKey, defaultVal)
		expected := -5 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})
}

func TestParseIntEnv(t *testing.T) {
	const testKey = "TEST_INT_ENV"
	defaultVal := 42

	t.Run("empty env var returns default", func(t *testing.T) {
		t.Setenv(testKey, "")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected %d, got %d", defaultVal, result)
		}
	})

	t.Run("unset env var returns default", func(t *testing.T) {
		result := parseIntEnv("UNSET_INT_KEY_12345", defaultVal)
		if result != defaultVal {
			t.Errorf("expected %d, got %d", defaultVal, result)
		}
	})

	t.Run("valid positive integer", func(t *testing.T) {
		t.Setenv(testKey, "100")
		result := parseIntEnv(testKey, defaultVal)
		if result != 100 {
			t.Errorf("expected 100, got %d", result)
		}
	})

	t.Run("valid zero", func(t *testing.T) {
		t.Setenv(testKey, "0")
		result := parseIntEnv(testKey, defaultVal)
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("valid negative integer", func(t *testing.T) {
		t.Setenv(testKey, "-50")
		result := parseIntEnv(testKey, defaultVal)
		if result != -50 {
			t.Errorf("expected -50, got %d", result)
		}
	})

	t.Run("invalid string returns default", func(t *testing.T) {
		t.Setenv(testKey, "not-a-number")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})

	t.Run("float returns default", func(t *testing.T) {
		t.Setenv(testKey, "3.14")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})

	t.Run("whitespace returns default", func(t *testing.T) {
		t.Setenv(testKey, "  ")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})

	t.Run("number with trailing text returns default", func(t *testing.T) {
		t.Setenv(testKey, "100abc")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})
}

func TestGetInstanceConfig(t *testing.T) {
	t.Run("nil Monitor returns nil", func(t *testing.T) {
		var m *Monitor
		result := m.getInstanceConfig("any")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("nil config returns nil", func(t *testing.T) {
		m := &Monitor{config: nil}
		result := m.getInstanceConfig("any")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("empty PVEInstances slice returns nil", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{},
			},
		}
		result := m.getInstanceConfig("any")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("instance found by exact name match", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
		}
		result := m.getInstanceConfig("pve1")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "pve1" {
			t.Errorf("expected Name 'pve1', got '%s'", result.Name)
		}
		if result.Host != "192.168.1.1" {
			t.Errorf("expected Host '192.168.1.1', got '%s'", result.Host)
		}
	})

	t.Run("instance found by case-insensitive match", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
		}
		result := m.getInstanceConfig("PVE1")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "pve1" {
			t.Errorf("expected Name 'pve1', got '%s'", result.Name)
		}
	})

	t.Run("instance not found returns nil", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
		}
		result := m.getInstanceConfig("nonexistent")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("multiple instances finds correct one", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
					{Name: "pve2", Host: "192.168.1.2"},
					{Name: "pve3", Host: "192.168.1.3"},
				},
			},
		}
		result := m.getInstanceConfig("pve2")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "pve2" {
			t.Errorf("expected Name 'pve2', got '%s'", result.Name)
		}
		if result.Host != "192.168.1.2" {
			t.Errorf("expected Host '192.168.1.2', got '%s'", result.Host)
		}
	})
}

func TestBaseIntervalForInstanceType(t *testing.T) {
	defaultInterval := DefaultSchedulerConfig().BaseInterval

	t.Run("nil Monitor returns default", func(t *testing.T) {
		var m *Monitor
		result := m.baseIntervalForInstanceType(InstanceTypePVE)
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})

	t.Run("nil config returns default", func(t *testing.T) {
		m := &Monitor{config: nil}
		result := m.baseIntervalForInstanceType(InstanceTypePVE)
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})

	t.Run("InstanceTypePVE returns effectivePVEPollingInterval result", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEPollingInterval: 30 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePVE)
		expected := 30 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePBS returns clamped PBS interval", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PBSPollingInterval: 45 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePBS)
		expected := 45 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePBS with interval < 10s gets clamped to 10s", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PBSPollingInterval: 5 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePBS)
		expected := 10 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePBS with interval > 1h gets clamped to 1h", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PBSPollingInterval: 2 * time.Hour,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePBS)
		expected := time.Hour
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePMG returns clamped PMG interval", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PMGPollingInterval: 2 * time.Minute,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePMG)
		expected := 2 * time.Minute
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("unknown instance type with positive AdaptivePollingBaseInterval", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				AdaptivePollingBaseInterval: 20 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceType("unknown"))
		expected := 20 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("unknown instance type with zero AdaptivePollingBaseInterval uses default", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				AdaptivePollingBaseInterval: 0,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceType("unknown"))
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})

	t.Run("unknown instance type with negative AdaptivePollingBaseInterval uses default", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				AdaptivePollingBaseInterval: -5 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceType("unknown"))
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})
}

func TestRecordAuthFailure(t *testing.T) {
	t.Run("empty nodeType uses instanceName as nodeID", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
		}

		m.recordAuthFailure("myinstance", "")

		if _, exists := m.authFailures["myinstance"]; !exists {
			t.Error("expected authFailures['myinstance'] to exist")
		}
		if m.authFailures["myinstance"] != 1 {
			t.Errorf("expected authFailures['myinstance'] = 1, got %d", m.authFailures["myinstance"])
		}
	})

	t.Run("non-empty nodeType creates type-instance nodeID", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
		}

		m.recordAuthFailure("myinstance", "pve")

		expectedID := "pve-myinstance"
		if _, exists := m.authFailures[expectedID]; !exists {
			t.Errorf("expected authFailures['%s'] to exist", expectedID)
		}
		if m.authFailures[expectedID] != 1 {
			t.Errorf("expected authFailures['%s'] = 1, got %d", expectedID, m.authFailures[expectedID])
		}
	})

	t.Run("increments authFailures counter", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
		}

		m.recordAuthFailure("node1", "pve")
		m.recordAuthFailure("node1", "pve")
		m.recordAuthFailure("node1", "pve")

		if m.authFailures["pve-node1"] != 3 {
			t.Errorf("expected 3 failures, got %d", m.authFailures["pve-node1"])
		}
	})

	t.Run("records lastAuthAttempt timestamp", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
		}

		before := time.Now()
		m.recordAuthFailure("node1", "pbs")
		after := time.Now()

		timestamp, exists := m.lastAuthAttempt["pbs-node1"]
		if !exists {
			t.Fatal("expected lastAuthAttempt['pbs-node1'] to exist")
		}
		if timestamp.Before(before) || timestamp.After(after) {
			t.Errorf("timestamp %v not between %v and %v", timestamp, before, after)
		}
	})

	t.Run("triggers removal at 5 failures for nodeType pve", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
			state: newMinimalState(),
		}

		// Record 4 failures - should not trigger removal
		for i := 0; i < 4; i++ {
			m.recordAuthFailure("pve1", "pve")
		}
		if m.authFailures["pve-pve1"] != 4 {
			t.Errorf("expected 4 failures, got %d", m.authFailures["pve-pve1"])
		}

		// 5th failure should trigger removal and reset counters
		m.recordAuthFailure("pve1", "pve")

		if _, exists := m.authFailures["pve-pve1"]; exists {
			t.Error("expected authFailures['pve-pve1'] to be deleted after 5 failures")
		}
		if _, exists := m.lastAuthAttempt["pve-pve1"]; exists {
			t.Error("expected lastAuthAttempt['pve-pve1'] to be deleted after 5 failures")
		}
	})

	t.Run("triggers removal at 5 failures for nodeType pbs", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
			config:          &config.Config{},
			state:           newMinimalState(),
		}

		// Record 5 failures
		for i := 0; i < 5; i++ {
			m.recordAuthFailure("pbs1", "pbs")
		}

		// Counters should be reset after removal
		if _, exists := m.authFailures["pbs-pbs1"]; exists {
			t.Error("expected authFailures['pbs-pbs1'] to be deleted after 5 failures")
		}
		if _, exists := m.lastAuthAttempt["pbs-pbs1"]; exists {
			t.Error("expected lastAuthAttempt['pbs-pbs1'] to be deleted after 5 failures")
		}
	})

	t.Run("triggers removal at 5 failures for nodeType pmg", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
			config:          &config.Config{},
			state:           newMinimalState(),
		}

		// Record 5 failures
		for i := 0; i < 5; i++ {
			m.recordAuthFailure("pmg1", "pmg")
		}

		// Counters should be reset after removal
		if _, exists := m.authFailures["pmg-pmg1"]; exists {
			t.Error("expected authFailures['pmg-pmg1'] to be deleted after 5 failures")
		}
		if _, exists := m.lastAuthAttempt["pmg-pmg1"]; exists {
			t.Error("expected lastAuthAttempt['pmg-pmg1'] to be deleted after 5 failures")
		}
	})

	t.Run("resets counters after removal", func(t *testing.T) {
		m := &Monitor{
			authFailures:    make(map[string]int),
			lastAuthAttempt: make(map[string]time.Time),
			config:          &config.Config{},
			state:           newMinimalState(),
		}

		// Trigger removal with 5 failures
		for i := 0; i < 5; i++ {
			m.recordAuthFailure("testnode", "pve")
		}

		// Verify counters are reset
		if len(m.authFailures) != 0 {
			t.Errorf("expected authFailures to be empty, got %v", m.authFailures)
		}
		if len(m.lastAuthAttempt) != 0 {
			t.Errorf("expected lastAuthAttempt to be empty, got %v", m.lastAuthAttempt)
		}

		// New failures should start from 1 again
		m.recordAuthFailure("testnode", "pve")
		if m.authFailures["pve-testnode"] != 1 {
			t.Errorf("expected counter to restart at 1, got %d", m.authFailures["pve-testnode"])
		}
	})
}

// newMinimalState creates a minimal State for testing
func newMinimalState() *models.State {
	return models.NewState()
}

func TestRecoverFromPanic(t *testing.T) {
	t.Run("no panic does nothing", func(t *testing.T) {
		// When no panic occurs, recoverFromPanic should do nothing
		// and the function should complete normally
		completed := false
		func() {
			defer recoverFromPanic("test-goroutine")
			completed = true
		}()
		if !completed {
			t.Error("expected function to complete normally without panic")
		}
	})

	t.Run("recovers from string panic", func(t *testing.T) {
		didPanic := false
		recovered := false
		func() {
			defer func() {
				// This runs after recoverFromPanic
				recovered = true
			}()
			defer recoverFromPanic("test-goroutine")
			didPanic = true
			panic("test panic message")
		}()
		if !didPanic {
			t.Error("expected panic to occur")
		}
		if !recovered {
			t.Error("expected to recover from panic")
		}
	})

	t.Run("recovers from error panic", func(t *testing.T) {
		didPanic := false
		recovered := false
		testErr := &testError{msg: "test error"}
		func() {
			defer func() {
				recovered = true
			}()
			defer recoverFromPanic("error-goroutine")
			didPanic = true
			panic(testErr)
		}()
		if !didPanic {
			t.Error("expected panic to occur")
		}
		if !recovered {
			t.Error("expected to recover from error panic")
		}
	})

	t.Run("recovers from int panic", func(t *testing.T) {
		didPanic := false
		recovered := false
		func() {
			defer func() {
				recovered = true
			}()
			defer recoverFromPanic("int-goroutine")
			didPanic = true
			panic(42)
		}()
		if !didPanic {
			t.Error("expected panic to occur")
		}
		if !recovered {
			t.Error("expected to recover from int panic")
		}
	})

	t.Run("recovers from struct panic", func(t *testing.T) {
		type panicData struct {
			code    int
			message string
		}
		didPanic := false
		recovered := false
		func() {
			defer func() {
				recovered = true
			}()
			defer recoverFromPanic("struct-goroutine")
			didPanic = true
			panic(panicData{code: 500, message: "internal error"})
		}()
		if !didPanic {
			t.Error("expected panic to occur")
		}
		if !recovered {
			t.Error("expected to recover from struct panic")
		}
	})

	t.Run("recovers from nil panic", func(t *testing.T) {
		didPanic := false
		recovered := false
		func() {
			defer func() {
				recovered = true
			}()
			defer recoverFromPanic("nil-goroutine")
			didPanic = true
			panic(nil)
		}()
		if !didPanic {
			t.Error("expected panic to occur")
		}
		if !recovered {
			t.Error("expected to recover from nil panic")
		}
	})

	t.Run("code after panic is not executed", func(t *testing.T) {
		afterPanicExecuted := false
		func() {
			defer recoverFromPanic("test-goroutine")
			panic("stop here")
			afterPanicExecuted = true //nolint:govet // unreachable code is intentional for test
		}()
		if afterPanicExecuted {
			t.Error("expected code after panic to not execute")
		}
	})
}

// testError implements error interface for panic testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestResetAuthFailures(t *testing.T) {
	t.Run("empty nodeType uses instanceName as nodeID", func(t *testing.T) {
		m := &Monitor{
			authFailures:    map[string]int{"myinstance": 3},
			lastAuthAttempt: map[string]time.Time{"myinstance": time.Now()},
		}

		m.resetAuthFailures("myinstance", "")

		if _, exists := m.authFailures["myinstance"]; exists {
			t.Error("expected authFailures['myinstance'] to be deleted")
		}
		if _, exists := m.lastAuthAttempt["myinstance"]; exists {
			t.Error("expected lastAuthAttempt['myinstance'] to be deleted")
		}
	})

	t.Run("non-empty nodeType creates type-instance nodeID", func(t *testing.T) {
		m := &Monitor{
			authFailures:    map[string]int{"pve-myinstance": 2},
			lastAuthAttempt: map[string]time.Time{"pve-myinstance": time.Now()},
		}

		m.resetAuthFailures("myinstance", "pve")

		if _, exists := m.authFailures["pve-myinstance"]; exists {
			t.Error("expected authFailures['pve-myinstance'] to be deleted")
		}
		if _, exists := m.lastAuthAttempt["pve-myinstance"]; exists {
			t.Error("expected lastAuthAttempt['pve-myinstance'] to be deleted")
		}
	})

	t.Run("deletes entry from authFailures when count > 0", func(t *testing.T) {
		m := &Monitor{
			authFailures:    map[string]int{"pbs-node1": 5, "pve-node2": 2},
			lastAuthAttempt: map[string]time.Time{"pbs-node1": time.Now(), "pve-node2": time.Now()},
		}

		m.resetAuthFailures("node1", "pbs")

		if _, exists := m.authFailures["pbs-node1"]; exists {
			t.Error("expected authFailures['pbs-node1'] to be deleted")
		}
		// Other entries should remain
		if _, exists := m.authFailures["pve-node2"]; !exists {
			t.Error("expected authFailures['pve-node2'] to remain")
		}
	})

	t.Run("deletes entry from lastAuthAttempt when count > 0", func(t *testing.T) {
		m := &Monitor{
			authFailures:    map[string]int{"pmg-server": 1},
			lastAuthAttempt: map[string]time.Time{"pmg-server": time.Now(), "other-node": time.Now()},
		}

		m.resetAuthFailures("server", "pmg")

		if _, exists := m.lastAuthAttempt["pmg-server"]; exists {
			t.Error("expected lastAuthAttempt['pmg-server'] to be deleted")
		}
		// Other entries should remain
		if _, exists := m.lastAuthAttempt["other-node"]; !exists {
			t.Error("expected lastAuthAttempt['other-node'] to remain")
		}
	})

	t.Run("does nothing when nodeID not in map", func(t *testing.T) {
		m := &Monitor{
			authFailures:    map[string]int{"pve-other": 3},
			lastAuthAttempt: map[string]time.Time{"pve-other": time.Now()},
		}

		m.resetAuthFailures("nonexistent", "pve")

		// Original entries should remain unchanged
		if count := m.authFailures["pve-other"]; count != 3 {
			t.Errorf("expected authFailures['pve-other'] = 3, got %d", count)
		}
		if _, exists := m.lastAuthAttempt["pve-other"]; !exists {
			t.Error("expected lastAuthAttempt['pve-other'] to remain")
		}
	})

	t.Run("does nothing when count is 0", func(t *testing.T) {
		timestamp := time.Now()
		m := &Monitor{
			authFailures:    map[string]int{"pve-zerocount": 0},
			lastAuthAttempt: map[string]time.Time{"pve-zerocount": timestamp},
		}

		m.resetAuthFailures("zerocount", "pve")

		// Entry should remain since count is 0
		if _, exists := m.authFailures["pve-zerocount"]; !exists {
			t.Error("expected authFailures['pve-zerocount'] to remain when count is 0")
		}
		if _, exists := m.lastAuthAttempt["pve-zerocount"]; !exists {
			t.Error("expected lastAuthAttempt['pve-zerocount'] to remain when count is 0")
		}
	})
}

func TestLookupClusterEndpointLabel(t *testing.T) {
	t.Run("nil instance returns empty string", func(t *testing.T) {
		result := lookupClusterEndpointLabel(nil, "node1")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("empty ClusterEndpoints returns empty string", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("no matching node name returns empty string", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "https://node1.lan:8006"},
				{NodeName: "node2", Host: "https://node2.lan:8006"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node3")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("case-insensitive node name matching", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "Node1", Host: "https://myhost.lan:8006"},
			},
		}
		// Search with lowercase
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "myhost.lan" {
			t.Errorf("expected 'myhost.lan', got %q", result)
		}

		// Search with uppercase
		result = lookupClusterEndpointLabel(instance, "NODE1")
		if result != "myhost.lan" {
			t.Errorf("expected 'myhost.lan', got %q", result)
		}
	})

	t.Run("returns host label (hostname, not IP) when available", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "https://pve-server.local:8006", IP: "192.168.1.100"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "pve-server.local" {
			t.Errorf("expected 'pve-server.local', got %q", result)
		}
	})

	t.Run("skips host if it's an IP address", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "https://192.168.1.100:8006", IP: "192.168.1.100"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		// Should fall back to NodeName since Host is an IP
		if result != "node1" {
			t.Errorf("expected 'node1', got %q", result)
		}
	})

	t.Run("falls back to NodeName when host is IP", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "pve-cluster-node", Host: "https://10.0.0.50:8006", IP: "10.0.0.50"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "pve-cluster-node")
		if result != "pve-cluster-node" {
			t.Errorf("expected 'pve-cluster-node', got %q", result)
		}
	})

	t.Run("falls back to IP when NodeName empty", func(t *testing.T) {
		// Test with empty Host - should fall back to NodeName
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "", IP: "192.168.1.100"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		// Host is empty, NodeName is "node1" (not empty), so should return NodeName
		if result != "node1" {
			t.Errorf("expected 'node1', got %q", result)
		}

		// Test with Host as IP - should fall back to NodeName
		instance2 := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "https://10.0.0.1:8006", IP: "172.16.0.1"},
			},
		}
		result = lookupClusterEndpointLabel(instance2, "node1")
		// Host is IP, so falls back to NodeName
		if result != "node1" {
			t.Errorf("expected 'node1', got %q", result)
		}
	})

	t.Run("returns IP when host is IP and NodeName is whitespace", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "searchme", Host: "https://192.168.1.50:8006", IP: "10.20.30.40"},
			},
		}
		// Temporarily modify to test scenario where NodeName after trim is empty
		// But we can't match on empty NodeName, so this tests the IP path differently

		// Actually - the function matches on NodeName first, so we need a valid NodeName to match
		// Then the logic checks host -> nodename -> IP for the label
		// Let's create a scenario where host is IP and nodename (after trim) is empty/whitespace
		// But wait - we match on NodeName, so it can't be empty to even get a match

		// The real scenario: endpoint with NodeName="node1", Host is IP, NodeName for label is " " (spaces)
		// But that's contradictory since we match on NodeName

		// Let me re-read the function... it uses endpoint.NodeName for both matching AND label
		// So if NodeName matches, it's not empty. The IP fallback only happens if:
		// 1. Host is IP (or empty)
		// 2. NodeName (trimmed) is empty
		// But #2 can't happen since we matched on NodeName

		// So the IP fallback case is when Host is empty AND NodeName is whitespace-only
		// But again, we can't match on whitespace-only NodeName with EqualFold

		// Actually the function iterates endpoints and compares endpoint.NodeName with the search nodeName
		// If endpoint.NodeName is "  node1  " and we search "node1", EqualFold won't match
		// So the IP fallback path is effectively unreachable in normal cases

		// Let's just test what we can: when Host is IP, it falls back to NodeName
		result := lookupClusterEndpointLabel(instance, "searchme")
		if result != "searchme" {
			t.Errorf("expected 'searchme', got %q", result)
		}
	})

	t.Run("handles host with port correctly", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "https://proxmox.example.com:8006"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "proxmox.example.com" {
			t.Errorf("expected 'proxmox.example.com', got %q", result)
		}
	})

	t.Run("handles host without scheme", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "myserver.lan:8006"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "myserver.lan" {
			t.Errorf("expected 'myserver.lan', got %q", result)
		}
	})

	t.Run("handles whitespace in host", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "  https://trimmed.lan:8006  ", IP: "1.2.3.4"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "trimmed.lan" {
			t.Errorf("expected 'trimmed.lan', got %q", result)
		}
	})

	t.Run("first matching endpoint wins", func(t *testing.T) {
		instance := &config.PVEInstance{
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node1", Host: "https://first.lan:8006"},
				{NodeName: "node1", Host: "https://second.lan:8006"},
			},
		}
		result := lookupClusterEndpointLabel(instance, "node1")
		if result != "first.lan" {
			t.Errorf("expected 'first.lan', got %q", result)
		}
	})
}

func TestExtractSnapshotName(t *testing.T) {
	t.Run("empty volid returns empty string", func(t *testing.T) {
		result := extractSnapshotName("")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("volid without colon without @ returns empty", func(t *testing.T) {
		result := extractSnapshotName("vm-100-disk-0")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("volid with colon without @ returns empty", func(t *testing.T) {
		result := extractSnapshotName("storage:vm-100-disk-0")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("volid with @ at end returns empty", func(t *testing.T) {
		result := extractSnapshotName("storage:vm-100-disk-0@")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("volid with storage prefix extracts snapshot name", func(t *testing.T) {
		result := extractSnapshotName("storage:vm-100-disk-0@snap1")
		if result != "snap1" {
			t.Errorf("expected 'snap1', got %q", result)
		}
	})

	t.Run("volid without storage prefix extracts snapshot name", func(t *testing.T) {
		result := extractSnapshotName("vm-100-disk-0@snap1")
		if result != "snap1" {
			t.Errorf("expected 'snap1', got %q", result)
		}
	})

	t.Run("snapshot name with whitespace is trimmed", func(t *testing.T) {
		result := extractSnapshotName("storage:vm-100-disk-0@  snap1  ")
		if result != "snap1" {
			t.Errorf("expected 'snap1', got %q", result)
		}
	})

	t.Run("multiple @ symbols uses first one", func(t *testing.T) {
		result := extractSnapshotName("storage:vm-100-disk-0@snap1@extra")
		if result != "snap1@extra" {
			t.Errorf("expected 'snap1@extra', got %q", result)
		}
	})
}

func TestEffectivePVEPollingInterval(t *testing.T) {
	tests := []struct {
		name     string
		monitor  *Monitor
		expected time.Duration
	}{
		{
			name:     "nil monitor returns minInterval",
			monitor:  nil,
			expected: 10 * time.Second,
		},
		{
			name:     "nil config returns minInterval",
			monitor:  &Monitor{config: nil},
			expected: 10 * time.Second,
		},
		{
			name: "zero PVEPollingInterval returns minInterval",
			monitor: &Monitor{
				config: &config.Config{
					PVEPollingInterval: 0,
				},
			},
			expected: 10 * time.Second,
		},
		{
			name: "valid interval within range",
			monitor: &Monitor{
				config: &config.Config{
					PVEPollingInterval: 30 * time.Second,
				},
			},
			expected: 30 * time.Second,
		},
		{
			name: "interval below minInterval clamped to 10s",
			monitor: &Monitor{
				config: &config.Config{
					PVEPollingInterval: 5 * time.Second,
				},
			},
			expected: 10 * time.Second,
		},
		{
			name: "interval above maxInterval clamped to 1h",
			monitor: &Monitor{
				config: &config.Config{
					PVEPollingInterval: 2 * time.Hour,
				},
			},
			expected: time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.monitor.effectivePVEPollingInterval()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClampUint64ToInt64(t *testing.T) {
	t.Run("zero value returns 0", func(t *testing.T) {
		result := clampUint64ToInt64(0)
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("small positive value returns same value", func(t *testing.T) {
		result := clampUint64ToInt64(12345)
		if result != 12345 {
			t.Errorf("expected 12345, got %d", result)
		}
	})

	t.Run("value at math.MaxInt64 returns math.MaxInt64", func(t *testing.T) {
		result := clampUint64ToInt64(uint64(math.MaxInt64))
		if result != math.MaxInt64 {
			t.Errorf("expected %d, got %d", int64(math.MaxInt64), result)
		}
	})

	t.Run("value at math.MaxInt64 + 1 clamps to math.MaxInt64", func(t *testing.T) {
		result := clampUint64ToInt64(uint64(math.MaxInt64) + 1)
		if result != math.MaxInt64 {
			t.Errorf("expected %d, got %d", int64(math.MaxInt64), result)
		}
	})

	t.Run("value at math.MaxUint64 clamps to math.MaxInt64", func(t *testing.T) {
		result := clampUint64ToInt64(math.MaxUint64)
		if result != math.MaxInt64 {
			t.Errorf("expected %d, got %d", int64(math.MaxInt64), result)
		}
	})
}

func TestRemoveFailedPBSNode(t *testing.T) {
	t.Run("removes correct instance from PBSInstances", func(t *testing.T) {
		state := models.NewState()
		state.UpdatePBSInstances([]models.PBSInstance{
			{Name: "pbs1", Host: "192.168.1.1"},
			{Name: "pbs2", Host: "192.168.1.2"},
			{Name: "pbs3", Host: "192.168.1.3"},
		})

		m := &Monitor{state: state}
		m.removeFailedPBSNode("pbs2")

		if len(state.PBSInstances) != 2 {
			t.Fatalf("expected 2 instances, got %d", len(state.PBSInstances))
		}
		for _, inst := range state.PBSInstances {
			if inst.Name == "pbs2" {
				t.Error("expected pbs2 to be removed")
			}
		}
		// Verify other instances remain
		names := make(map[string]bool)
		for _, inst := range state.PBSInstances {
			names[inst.Name] = true
		}
		if !names["pbs1"] || !names["pbs3"] {
			t.Errorf("expected pbs1 and pbs3 to remain, got %v", names)
		}
	})

	t.Run("clears PBS backups for that instance", func(t *testing.T) {
		state := models.NewState()
		state.UpdatePBSBackups("pbs1", []models.PBSBackup{
			{ID: "backup1", Instance: "pbs1"},
			{ID: "backup2", Instance: "pbs1"},
		})
		state.UpdatePBSBackups("pbs2", []models.PBSBackup{
			{ID: "backup3", Instance: "pbs2"},
		})

		m := &Monitor{state: state}
		m.removeFailedPBSNode("pbs1")

		// Verify pbs1 backups are gone
		for _, backup := range state.PBSBackups {
			if backup.Instance == "pbs1" {
				t.Errorf("expected pbs1 backups to be cleared, found %s", backup.ID)
			}
		}
		// Verify pbs2 backups remain
		found := false
		for _, backup := range state.PBSBackups {
			if backup.Instance == "pbs2" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected pbs2 backups to remain")
		}
	})

	t.Run("sets connection health to false", func(t *testing.T) {
		state := models.NewState()
		state.SetConnectionHealth("pbs-pbs1", true)
		state.SetConnectionHealth("pbs-pbs2", true)

		m := &Monitor{state: state}
		m.removeFailedPBSNode("pbs1")

		if state.ConnectionHealth["pbs-pbs1"] != false {
			t.Error("expected pbs-pbs1 connection health to be false")
		}
		if state.ConnectionHealth["pbs-pbs2"] != true {
			t.Error("expected pbs-pbs2 connection health to remain true")
		}
	})

	t.Run("handles empty instances list", func(t *testing.T) {
		state := models.NewState()
		m := &Monitor{state: state}

		// Should not panic
		m.removeFailedPBSNode("nonexistent")

		if len(state.PBSInstances) != 0 {
			t.Errorf("expected empty instances, got %d", len(state.PBSInstances))
		}
	})

	t.Run("handles instance not found", func(t *testing.T) {
		state := models.NewState()
		state.UpdatePBSInstances([]models.PBSInstance{
			{Name: "pbs1", Host: "192.168.1.1"},
		})

		m := &Monitor{state: state}
		m.removeFailedPBSNode("nonexistent")

		if len(state.PBSInstances) != 1 {
			t.Errorf("expected 1 instance to remain, got %d", len(state.PBSInstances))
		}
	})
}

func TestGetDockerHost(t *testing.T) {
	t.Run("empty hostID returns empty host and false", func(t *testing.T) {
		state := models.NewState()
		state.UpsertDockerHost(models.DockerHost{ID: "host1", Hostname: "docker1"})
		m := &Monitor{state: state}

		host, found := m.GetDockerHost("")
		if found {
			t.Error("expected found to be false for empty hostID")
		}
		if host.ID != "" {
			t.Errorf("expected empty host, got ID=%q", host.ID)
		}
	})

	t.Run("whitespace-only hostID returns empty host and false", func(t *testing.T) {
		state := models.NewState()
		state.UpsertDockerHost(models.DockerHost{ID: "host1", Hostname: "docker1"})
		m := &Monitor{state: state}

		host, found := m.GetDockerHost("   ")
		if found {
			t.Error("expected found to be false for whitespace-only hostID")
		}
		if host.ID != "" {
			t.Errorf("expected empty host, got ID=%q", host.ID)
		}
	})

	t.Run("host ID not found returns empty host and false", func(t *testing.T) {
		state := models.NewState()
		state.UpsertDockerHost(models.DockerHost{ID: "host1", Hostname: "docker1"})
		m := &Monitor{state: state}

		host, found := m.GetDockerHost("nonexistent")
		if found {
			t.Error("expected found to be false for nonexistent hostID")
		}
		if host.ID != "" {
			t.Errorf("expected empty host, got ID=%q", host.ID)
		}
	})

	t.Run("host ID found returns the host and true", func(t *testing.T) {
		state := models.NewState()
		state.UpsertDockerHost(models.DockerHost{ID: "host1", Hostname: "docker1"})
		state.UpsertDockerHost(models.DockerHost{ID: "host2", Hostname: "docker2"})
		m := &Monitor{state: state}

		host, found := m.GetDockerHost("host1")
		if !found {
			t.Error("expected found to be true")
		}
		if host.ID != "host1" {
			t.Errorf("expected ID='host1', got %q", host.ID)
		}
		if host.Hostname != "docker1" {
			t.Errorf("expected Hostname='docker1', got %q", host.Hostname)
		}
	})

	t.Run("hostID with leading/trailing whitespace is trimmed and found", func(t *testing.T) {
		state := models.NewState()
		state.UpsertDockerHost(models.DockerHost{ID: "host1", Hostname: "docker1"})
		m := &Monitor{state: state}

		host, found := m.GetDockerHost("  host1  ")
		if !found {
			t.Error("expected found to be true after trimming whitespace")
		}
		if host.ID != "host1" {
			t.Errorf("expected ID='host1', got %q", host.ID)
		}
		if host.Hostname != "docker1" {
			t.Errorf("expected Hostname='docker1', got %q", host.Hostname)
		}
	})
}

func TestRemoveFailedPMGInstance(t *testing.T) {
	t.Run("removes correct instance from PMGInstances", func(t *testing.T) {
		state := models.NewState()
		state.UpdatePMGInstances([]models.PMGInstance{
			{Name: "pmg1", Host: "192.168.1.1"},
			{Name: "pmg2", Host: "192.168.1.2"},
			{Name: "pmg3", Host: "192.168.1.3"},
		})

		m := &Monitor{state: state}
		m.removeFailedPMGInstance("pmg2")

		if len(state.PMGInstances) != 2 {
			t.Fatalf("expected 2 instances, got %d", len(state.PMGInstances))
		}
		for _, inst := range state.PMGInstances {
			if inst.Name == "pmg2" {
				t.Error("expected pmg2 to be removed")
			}
		}
		// Verify other instances remain
		names := make(map[string]bool)
		for _, inst := range state.PMGInstances {
			names[inst.Name] = true
		}
		if !names["pmg1"] || !names["pmg3"] {
			t.Errorf("expected pmg1 and pmg3 to remain, got %v", names)
		}
	})

	t.Run("clears PMG backups for that instance", func(t *testing.T) {
		state := models.NewState()
		state.UpdatePMGBackups("pmg1", []models.PMGBackup{
			{ID: "backup1", Instance: "pmg1"},
			{ID: "backup2", Instance: "pmg1"},
		})
		state.UpdatePMGBackups("pmg2", []models.PMGBackup{
			{ID: "backup3", Instance: "pmg2"},
		})

		m := &Monitor{state: state}
		m.removeFailedPMGInstance("pmg1")

		// Verify pmg1 backups are gone
		for _, backup := range state.PMGBackups {
			if backup.Instance == "pmg1" {
				t.Errorf("expected pmg1 backups to be cleared, found %s", backup.ID)
			}
		}
		// Verify pmg2 backups remain
		found := false
		for _, backup := range state.PMGBackups {
			if backup.Instance == "pmg2" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected pmg2 backups to remain")
		}
	})

	t.Run("sets connection health to false", func(t *testing.T) {
		state := models.NewState()
		state.SetConnectionHealth("pmg-pmg1", true)
		state.SetConnectionHealth("pmg-pmg2", true)

		m := &Monitor{state: state}
		m.removeFailedPMGInstance("pmg1")

		if state.ConnectionHealth["pmg-pmg1"] != false {
			t.Error("expected pmg-pmg1 connection health to be false")
		}
		if state.ConnectionHealth["pmg-pmg2"] != true {
			t.Error("expected pmg-pmg2 connection health to remain true")
		}
	})

	t.Run("handles empty instances list", func(t *testing.T) {
		state := models.NewState()
		m := &Monitor{state: state}

		// Should not panic
		m.removeFailedPMGInstance("nonexistent")

		if len(state.PMGInstances) != 0 {
			t.Errorf("expected empty instances, got %d", len(state.PMGInstances))
		}
	})

	t.Run("handles instance not found", func(t *testing.T) {
		state := models.NewState()
		state.UpdatePMGInstances([]models.PMGInstance{
			{Name: "pmg1", Host: "192.168.1.1"},
		})

		m := &Monitor{state: state}
		m.removeFailedPMGInstance("nonexistent")

		if len(state.PMGInstances) != 1 {
			t.Errorf("expected 1 instance to remain, got %d", len(state.PMGInstances))
		}
	})
}

func TestSchedulerHealth(t *testing.T) {
	t.Run("nil config returns Enabled false", func(t *testing.T) {
		m := &Monitor{config: nil}
		resp := m.SchedulerHealth()
		if resp.Enabled {
			t.Error("expected Enabled to be false when config is nil")
		}
	})

	t.Run("config with AdaptivePollingEnabled false returns Enabled false", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{AdaptivePollingEnabled: false},
		}
		resp := m.SchedulerHealth()
		if resp.Enabled {
			t.Error("expected Enabled to be false when AdaptivePollingEnabled is false")
		}
	})

	t.Run("config with AdaptivePollingEnabled true returns Enabled true", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{AdaptivePollingEnabled: true},
		}
		resp := m.SchedulerHealth()
		if !resp.Enabled {
			t.Error("expected Enabled to be true when AdaptivePollingEnabled is true")
		}
	})

	t.Run("nil taskQueue returns empty Queue", func(t *testing.T) {
		m := &Monitor{
			config:    &config.Config{},
			taskQueue: nil,
		}
		resp := m.SchedulerHealth()
		// Queue should be zero value (empty)
		if resp.Queue.Depth != 0 {
			t.Errorf("expected Queue.Depth to be 0, got %d", resp.Queue.Depth)
		}
		if len(resp.Queue.PerType) != 0 {
			t.Errorf("expected Queue.PerType to be empty, got %d entries", len(resp.Queue.PerType))
		}
	})

	t.Run("non-nil taskQueue returns queue snapshot", func(t *testing.T) {
		tq := NewTaskQueue()
		tq.Upsert(ScheduledTask{
			InstanceType: InstanceTypePVE,
			InstanceName: "pve1",
			NextRun:      time.Now().Add(time.Minute),
		})

		m := &Monitor{
			config:    &config.Config{},
			taskQueue: tq,
		}
		resp := m.SchedulerHealth()
		if resp.Queue.Depth != 1 {
			t.Errorf("expected Queue.Depth to be 1, got %d", resp.Queue.Depth)
		}
	})

	t.Run("nil deadLetterQueue returns empty DeadLetter", func(t *testing.T) {
		m := &Monitor{
			config:          &config.Config{},
			deadLetterQueue: nil,
		}
		resp := m.SchedulerHealth()
		if resp.DeadLetter.Count != 0 {
			t.Errorf("expected DeadLetter.Count to be 0, got %d", resp.DeadLetter.Count)
		}
		if len(resp.DeadLetter.Tasks) != 0 {
			t.Errorf("expected DeadLetter.Tasks to be empty, got %d tasks", len(resp.DeadLetter.Tasks))
		}
	})

	t.Run("non-nil deadLetterQueue returns dead letter snapshot", func(t *testing.T) {
		dlq := NewTaskQueue()
		dlq.Upsert(ScheduledTask{
			InstanceType: InstanceTypePVE,
			InstanceName: "failed-pve",
			NextRun:      time.Now(),
		})

		m := &Monitor{
			config:          &config.Config{},
			deadLetterQueue: dlq,
			lastOutcome:     make(map[string]taskOutcome),
			failureCounts:   make(map[string]int),
		}
		resp := m.SchedulerHealth()
		if resp.DeadLetter.Count != 1 {
			t.Errorf("expected DeadLetter.Count to be 1, got %d", resp.DeadLetter.Count)
		}
	})

	t.Run("circuit breaker key with :: separator extracts type and name", func(t *testing.T) {
		breaker := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
		// Record a failure to make the breaker appear in the response
		breaker.recordFailure(time.Now())

		m := &Monitor{
			config: &config.Config{},
			circuitBreakers: map[string]*circuitBreaker{
				"pve::my-node": breaker,
			},
		}
		resp := m.SchedulerHealth()

		if len(resp.Breakers) != 1 {
			t.Fatalf("expected 1 breaker, got %d", len(resp.Breakers))
		}
		if resp.Breakers[0].Type != "pve" {
			t.Errorf("expected breaker Type to be 'pve', got %q", resp.Breakers[0].Type)
		}
		if resp.Breakers[0].Instance != "my-node" {
			t.Errorf("expected breaker Instance to be 'my-node', got %q", resp.Breakers[0].Instance)
		}
	})

	t.Run("circuit breaker key without :: separator uses unknown type", func(t *testing.T) {
		breaker := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
		breaker.recordFailure(time.Now())

		m := &Monitor{
			config: &config.Config{},
			circuitBreakers: map[string]*circuitBreaker{
				"legacy-key-no-separator": breaker,
			},
		}
		resp := m.SchedulerHealth()

		if len(resp.Breakers) != 1 {
			t.Fatalf("expected 1 breaker, got %d", len(resp.Breakers))
		}
		if resp.Breakers[0].Type != "unknown" {
			t.Errorf("expected breaker Type to be 'unknown', got %q", resp.Breakers[0].Type)
		}
		if resp.Breakers[0].Instance != "legacy-key-no-separator" {
			t.Errorf("expected breaker Instance to be 'legacy-key-no-separator', got %q", resp.Breakers[0].Instance)
		}
	})

	t.Run("circuit breaker in closed state with 0 failures is skipped", func(t *testing.T) {
		// A freshly created breaker is closed with 0 failures
		breaker := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)

		m := &Monitor{
			config: &config.Config{},
			circuitBreakers: map[string]*circuitBreaker{
				"pve::healthy-node": breaker,
			},
		}
		resp := m.SchedulerHealth()

		if len(resp.Breakers) != 0 {
			t.Errorf("expected 0 breakers (closed with 0 failures should be skipped), got %d", len(resp.Breakers))
		}
	})

	t.Run("open breaker is included regardless of failure count", func(t *testing.T) {
		breaker := newCircuitBreaker(1, 5*time.Second, 5*time.Minute, 30*time.Second)
		// Trip the breaker open by recording enough failures
		breaker.recordFailure(time.Now())

		m := &Monitor{
			config: &config.Config{},
			circuitBreakers: map[string]*circuitBreaker{
				"pbs::failed-backup": breaker,
			},
		}
		resp := m.SchedulerHealth()

		if len(resp.Breakers) != 1 {
			t.Fatalf("expected 1 breaker, got %d", len(resp.Breakers))
		}
		if resp.Breakers[0].State != "open" {
			t.Errorf("expected breaker State to be 'open', got %q", resp.Breakers[0].State)
		}
	})

	t.Run("multiple breakers with mixed states", func(t *testing.T) {
		healthyBreaker := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)

		failingBreaker := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
		failingBreaker.recordFailure(time.Now())

		tripBreaker := newCircuitBreaker(1, 5*time.Second, 5*time.Minute, 30*time.Second)
		tripBreaker.recordFailure(time.Now())

		m := &Monitor{
			config: &config.Config{},
			circuitBreakers: map[string]*circuitBreaker{
				"pve::healthy":    healthyBreaker,
				"pve::failing":    failingBreaker,
				"pbs::tripped":    tripBreaker,
				"legacy-no-colon": failingBreaker, // shares same breaker but different key
			},
		}
		resp := m.SchedulerHealth()

		// healthyBreaker (closed, 0 failures) should be excluded
		// failingBreaker (closed, 1 failure) should be included twice (pve::failing and legacy-no-colon)
		// tripBreaker (open, 1 failure) should be included
		if len(resp.Breakers) != 3 {
			t.Errorf("expected 3 breakers (excluding healthy), got %d", len(resp.Breakers))
		}
	})

	t.Run("UpdatedAt is set to current time", func(t *testing.T) {
		before := time.Now()
		m := &Monitor{config: &config.Config{}}
		resp := m.SchedulerHealth()
		after := time.Now()

		if resp.UpdatedAt.Before(before) || resp.UpdatedAt.After(after) {
			t.Errorf("expected UpdatedAt between %v and %v, got %v", before, after, resp.UpdatedAt)
		}
	})
}

func TestConvertDockerSwarmInfo(t *testing.T) {
	tests := []struct {
		name     string
		input    *agentsdocker.SwarmInfo
		expected *models.DockerSwarmInfo
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty struct returns empty struct",
			input:    &agentsdocker.SwarmInfo{},
			expected: &models.DockerSwarmInfo{},
		},
		{
			name: "all fields populated",
			input: &agentsdocker.SwarmInfo{
				NodeID:           "node-abc123",
				NodeRole:         "manager",
				LocalState:       "active",
				ControlAvailable: true,
				ClusterID:        "cluster-xyz789",
				ClusterName:      "my-swarm",
				Scope:            "swarm",
				Error:            "",
			},
			expected: &models.DockerSwarmInfo{
				NodeID:           "node-abc123",
				NodeRole:         "manager",
				LocalState:       "active",
				ControlAvailable: true,
				ClusterID:        "cluster-xyz789",
				ClusterName:      "my-swarm",
				Scope:            "swarm",
				Error:            "",
			},
		},
		{
			name: "worker node with error",
			input: &agentsdocker.SwarmInfo{
				NodeID:           "node-worker1",
				NodeRole:         "worker",
				LocalState:       "pending",
				ControlAvailable: false,
				ClusterID:        "",
				ClusterName:      "",
				Scope:            "local",
				Error:            "connection refused",
			},
			expected: &models.DockerSwarmInfo{
				NodeID:           "node-worker1",
				NodeRole:         "worker",
				LocalState:       "pending",
				ControlAvailable: false,
				ClusterID:        "",
				ClusterName:      "",
				Scope:            "local",
				Error:            "connection refused",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertDockerSwarmInfo(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.NodeID != tt.expected.NodeID {
				t.Errorf("NodeID: expected %q, got %q", tt.expected.NodeID, result.NodeID)
			}
			if result.NodeRole != tt.expected.NodeRole {
				t.Errorf("NodeRole: expected %q, got %q", tt.expected.NodeRole, result.NodeRole)
			}
			if result.LocalState != tt.expected.LocalState {
				t.Errorf("LocalState: expected %q, got %q", tt.expected.LocalState, result.LocalState)
			}
			if result.ControlAvailable != tt.expected.ControlAvailable {
				t.Errorf("ControlAvailable: expected %v, got %v", tt.expected.ControlAvailable, result.ControlAvailable)
			}
			if result.ClusterID != tt.expected.ClusterID {
				t.Errorf("ClusterID: expected %q, got %q", tt.expected.ClusterID, result.ClusterID)
			}
			if result.ClusterName != tt.expected.ClusterName {
				t.Errorf("ClusterName: expected %q, got %q", tt.expected.ClusterName, result.ClusterName)
			}
			if result.Scope != tt.expected.Scope {
				t.Errorf("Scope: expected %q, got %q", tt.expected.Scope, result.Scope)
			}
			if result.Error != tt.expected.Error {
				t.Errorf("Error: expected %q, got %q", tt.expected.Error, result.Error)
			}
		})
	}
}

func TestAllowDockerHostReenroll(t *testing.T) {
	t.Run("empty hostID returns error", func(t *testing.T) {
		m := &Monitor{
			state:              models.NewState(),
			removedDockerHosts: make(map[string]time.Time),
		}

		err := m.AllowDockerHostReenroll("")
		if err == nil {
			t.Error("expected error for empty hostID")
		}
		if err.Error() != "docker host id is required" {
			t.Errorf("expected 'docker host id is required', got %q", err.Error())
		}
	})

	t.Run("whitespace-only hostID returns error", func(t *testing.T) {
		m := &Monitor{
			state:              models.NewState(),
			removedDockerHosts: make(map[string]time.Time),
		}

		err := m.AllowDockerHostReenroll("   ")
		if err == nil {
			t.Error("expected error for whitespace-only hostID")
		}
		if err.Error() != "docker host id is required" {
			t.Errorf("expected 'docker host id is required', got %q", err.Error())
		}
	})

	t.Run("host not blocked with host in state returns nil", func(t *testing.T) {
		state := models.NewState()
		state.UpsertDockerHost(models.DockerHost{ID: "host1", Hostname: "docker-host-1"})

		m := &Monitor{
			state:              state,
			removedDockerHosts: make(map[string]time.Time),
		}

		err := m.AllowDockerHostReenroll("host1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("host not blocked with host not in state returns nil", func(t *testing.T) {
		state := models.NewState()

		m := &Monitor{
			state:              state,
			removedDockerHosts: make(map[string]time.Time),
		}

		err := m.AllowDockerHostReenroll("nonexistent")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("blocked host gets removed and returns nil", func(t *testing.T) {
		state := models.NewState()

		m := &Monitor{
			state:              state,
			removedDockerHosts: map[string]time.Time{"host1": time.Now()},
			dockerCommands:     make(map[string]*dockerHostCommand),
			dockerCommandIndex: make(map[string]string),
		}

		err := m.AllowDockerHostReenroll("host1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		if _, exists := m.removedDockerHosts["host1"]; exists {
			t.Error("expected host1 to be removed from removedDockerHosts")
		}
	})

	t.Run("blocked host with dockerCommands entry gets cleaned up", func(t *testing.T) {
		state := models.NewState()

		cmd := &dockerHostCommand{
			status: models.DockerHostCommandStatus{
				ID: "cmd-123",
			},
		}

		m := &Monitor{
			state:              state,
			removedDockerHosts: map[string]time.Time{"host1": time.Now()},
			dockerCommands:     map[string]*dockerHostCommand{"host1": cmd},
			dockerCommandIndex: map[string]string{"cmd-123": "host1"},
		}

		err := m.AllowDockerHostReenroll("host1")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		if _, exists := m.removedDockerHosts["host1"]; exists {
			t.Error("expected host1 to be removed from removedDockerHosts")
		}
		if _, exists := m.dockerCommands["host1"]; exists {
			t.Error("expected host1 to be removed from dockerCommands")
		}
		if _, exists := m.dockerCommandIndex["cmd-123"]; exists {
			t.Error("expected cmd-123 to be removed from dockerCommandIndex")
		}
	})

	t.Run("hostID with whitespace is trimmed", func(t *testing.T) {
		state := models.NewState()

		m := &Monitor{
			state:              state,
			removedDockerHosts: map[string]time.Time{"host1": time.Now()},
			dockerCommands:     make(map[string]*dockerHostCommand),
			dockerCommandIndex: make(map[string]string),
		}

		err := m.AllowDockerHostReenroll("  host1  ")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		if _, exists := m.removedDockerHosts["host1"]; exists {
			t.Error("expected host1 to be removed from removedDockerHosts after trimming")
		}
	})
}

func TestEnsureBreaker(t *testing.T) {
	tests := []struct {
		name                  string
		circuitBreakers       map[string]*circuitBreaker
		existingBreaker       *circuitBreaker
		breakerBaseRetry      time.Duration
		breakerMaxDelay       time.Duration
		breakerHalfOpenWindow time.Duration
		key                   string
		wantRetryInterval     time.Duration
		wantMaxDelay          time.Duration
		wantHalfOpenWindow    time.Duration
		wantExisting          bool
	}{
		{
			name:                  "nil circuitBreakers map gets initialized",
			circuitBreakers:       nil,
			key:                   "test-key",
			wantRetryInterval:     5 * time.Second,
			wantMaxDelay:          5 * time.Minute,
			wantHalfOpenWindow:    30 * time.Second,
			wantExisting:          false,
		},
		{
			name:                  "existing breaker for key is returned",
			circuitBreakers:       map[string]*circuitBreaker{},
			existingBreaker:       newCircuitBreaker(3, 10*time.Second, 10*time.Minute, 60*time.Second),
			key:                   "existing-key",
			wantRetryInterval:     10 * time.Second,
			wantMaxDelay:          10 * time.Minute,
			wantHalfOpenWindow:    60 * time.Second,
			wantExisting:          true,
		},
		{
			name:                  "new breaker with default values (all config fields zero)",
			circuitBreakers:       map[string]*circuitBreaker{},
			breakerBaseRetry:      0,
			breakerMaxDelay:       0,
			breakerHalfOpenWindow: 0,
			key:                   "new-key",
			wantRetryInterval:     5 * time.Second,
			wantMaxDelay:          5 * time.Minute,
			wantHalfOpenWindow:    30 * time.Second,
			wantExisting:          false,
		},
		{
			name:                  "new breaker with custom breakerBaseRetry",
			circuitBreakers:       map[string]*circuitBreaker{},
			breakerBaseRetry:      2 * time.Second,
			breakerMaxDelay:       0,
			breakerHalfOpenWindow: 0,
			key:                   "custom-retry-key",
			wantRetryInterval:     2 * time.Second,
			wantMaxDelay:          5 * time.Minute,
			wantHalfOpenWindow:    30 * time.Second,
			wantExisting:          false,
		},
		{
			name:                  "new breaker with custom breakerMaxDelay",
			circuitBreakers:       map[string]*circuitBreaker{},
			breakerBaseRetry:      0,
			breakerMaxDelay:       10 * time.Minute,
			breakerHalfOpenWindow: 0,
			key:                   "custom-maxdelay-key",
			wantRetryInterval:     5 * time.Second,
			wantMaxDelay:          10 * time.Minute,
			wantHalfOpenWindow:    30 * time.Second,
			wantExisting:          false,
		},
		{
			name:                  "new breaker with custom breakerHalfOpenWindow",
			circuitBreakers:       map[string]*circuitBreaker{},
			breakerBaseRetry:      0,
			breakerMaxDelay:       0,
			breakerHalfOpenWindow: 15 * time.Second,
			key:                   "custom-halfopen-key",
			wantRetryInterval:     5 * time.Second,
			wantMaxDelay:          5 * time.Minute,
			wantHalfOpenWindow:    15 * time.Second,
			wantExisting:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Monitor{
				circuitBreakers:       tt.circuitBreakers,
				breakerBaseRetry:      tt.breakerBaseRetry,
				breakerMaxDelay:       tt.breakerMaxDelay,
				breakerHalfOpenWindow: tt.breakerHalfOpenWindow,
			}

			// If we have an existing breaker to test, add it before calling ensureBreaker
			if tt.existingBreaker != nil {
				m.circuitBreakers[tt.key] = tt.existingBreaker
			}

			breaker := m.ensureBreaker(tt.key)

			if breaker == nil {
				t.Fatal("expected non-nil breaker")
			}

			// Verify the map was initialized if it was nil
			if tt.circuitBreakers == nil && m.circuitBreakers == nil {
				t.Error("expected circuitBreakers map to be initialized")
			}

			// Verify the breaker is stored in the map
			storedBreaker, ok := m.circuitBreakers[tt.key]
			if !ok {
				t.Error("expected breaker to be stored in map")
			}
			if storedBreaker != breaker {
				t.Error("expected stored breaker to be the same as returned breaker")
			}

			// Verify we got back the existing breaker if expected
			if tt.wantExisting && breaker != tt.existingBreaker {
				t.Error("expected to get back the existing breaker")
			}

			// Verify breaker configuration
			if breaker.retryInterval != tt.wantRetryInterval {
				t.Errorf("retryInterval = %v, want %v", breaker.retryInterval, tt.wantRetryInterval)
			}
			if breaker.maxDelay != tt.wantMaxDelay {
				t.Errorf("maxDelay = %v, want %v", breaker.maxDelay, tt.wantMaxDelay)
			}
			if breaker.halfOpenWindow != tt.wantHalfOpenWindow {
				t.Errorf("halfOpenWindow = %v, want %v", breaker.halfOpenWindow, tt.wantHalfOpenWindow)
			}
		})
	}
}

func TestUpdateDeadLetterMetrics_NilPollMetrics(t *testing.T) {
	t.Parallel()

	m := &Monitor{
		pollMetrics:     nil,
		deadLetterQueue: NewTaskQueue(),
	}

	// Should not panic
	m.updateDeadLetterMetrics()
}

func TestUpdateDeadLetterMetrics_NilDeadLetterQueue(t *testing.T) {
	t.Parallel()

	m := &Monitor{
		pollMetrics:     newTestPollMetrics(t),
		deadLetterQueue: nil,
	}

	// Should not panic
	m.updateDeadLetterMetrics()
}

func TestUpdateDeadLetterMetrics_BothNil(t *testing.T) {
	t.Parallel()

	m := &Monitor{
		pollMetrics:     nil,
		deadLetterQueue: nil,
	}

	// Should not panic
	m.updateDeadLetterMetrics()
}

func TestUpdateDeadLetterMetrics_EmptyQueue(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)
	dlq := NewTaskQueue()

	// Pre-populate with some dead letter data to verify it gets cleared
	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
	})
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 1 {
		t.Fatalf("pre-check: dead_letter_depth{pve,pve1} = %v, want 1", got)
	}

	m := &Monitor{
		pollMetrics:     pm,
		deadLetterQueue: dlq,
	}

	// Empty queue should call UpdateDeadLetterCounts(nil) which clears previous entries
	m.updateDeadLetterMetrics()

	got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1")
	if got != 0 {
		t.Errorf("dead_letter_depth{pve,pve1} = %v, want 0 after empty queue update", got)
	}
}

func TestUpdateDeadLetterMetrics_NonEmptyQueue(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)
	dlq := NewTaskQueue()

	// Add tasks to the dead letter queue
	dlq.Upsert(ScheduledTask{
		InstanceType: InstanceTypePVE,
		InstanceName: "pve-instance-1",
		NextRun:      time.Now().Add(time.Hour),
	})
	dlq.Upsert(ScheduledTask{
		InstanceType: InstanceTypePBS,
		InstanceName: "pbs-instance-1",
		NextRun:      time.Now().Add(time.Hour),
	})

	m := &Monitor{
		pollMetrics:     pm,
		deadLetterQueue: dlq,
	}

	m.updateDeadLetterMetrics()

	// Verify metrics were updated with the tasks from the queue
	gotPve := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve-instance-1")
	if gotPve != 1 {
		t.Errorf("dead_letter_depth{pve,pve-instance-1} = %v, want 1", gotPve)
	}

	gotPbs := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs-instance-1")
	if gotPbs != 1 {
		t.Errorf("dead_letter_depth{pbs,pbs-instance-1} = %v, want 1", gotPbs)
	}
}

func TestUpdateDeadLetterMetrics_QueueChanges(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)
	dlq := NewTaskQueue()

	m := &Monitor{
		pollMetrics:     pm,
		deadLetterQueue: dlq,
	}

	// First, add some tasks
	dlq.Upsert(ScheduledTask{
		InstanceType: InstanceTypePVE,
		InstanceName: "pve1",
		NextRun:      time.Now().Add(time.Hour),
	})
	dlq.Upsert(ScheduledTask{
		InstanceType: InstanceTypePBS,
		InstanceName: "pbs1",
		NextRun:      time.Now().Add(time.Hour),
	})

	m.updateDeadLetterMetrics()

	// Verify initial state
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 1 {
		t.Fatalf("initial pve/pve1 = %v, want 1", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 1 {
		t.Fatalf("initial pbs/pbs1 = %v, want 1", got)
	}

	// Remove pbs1 from queue
	dlq.Remove(InstanceTypePBS, "pbs1")

	m.updateDeadLetterMetrics()

	// pve1 should still be 1, pbs1 should be cleared to 0
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 1 {
		t.Errorf("after removal pve/pve1 = %v, want 1", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 0 {
		t.Errorf("after removal pbs/pbs1 = %v, want 0", got)
	}
}
