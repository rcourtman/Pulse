package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
