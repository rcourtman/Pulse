package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file raises branch/function coverage for the three ConfigPersistence
// methods on report_schedules.go that were previously 0.0% covered:
//   - (*ConfigPersistence).ReportSchedulesPath()
//   - (*ConfigPersistence).LoadReportScheduleStore()
//   - (*ConfigPersistence).SaveReportScheduleStore()
//
// Conventions (table-driven subtests, NewConfigPersistence(t.TempDir())
// constructor, testify assert/require, package-level mockFSError for
// FileSystem error injection) are copied from availability_test.go and
// persistence_coverage_test.go in this directory.
//
// Test functions use the TestBranchcov0722PM prefix so the scoped run
// `go test ./internal/config/ -run '^TestBranchcov0722PM'` selects only them.

// newPlainReportSchedulePersistence builds a ConfigPersistence whose crypto is
// nil and whose reportSchedulesFile lives under dir. With crypto disabled we
// can (a) read the persisted bytes back as plaintext JSON to assert exactly
// what Save wrote, and (b) reach the parse-only error branch of Load that is
// shadowed by decryption when crypto is enabled. It writes only under dir.
func newPlainReportSchedulePersistence(dir string) *ConfigPersistence {
	return &ConfigPersistence{
		configDir:           dir,
		reportSchedulesFile: filepath.Join(dir, "report_schedules.json"),
		fs:                  defaultFileSystem{},
	}
}

// TestBranchcov0722PM_ReportSchedulesPath covers both arms of the nil-receiver
// guard in (*ConfigPersistence).ReportSchedulesPath and asserts the exact
// joined path against the persistence's configured directory.
func TestBranchcov0722PM_ReportSchedulesPath(t *testing.T) {
	t.Run("nil receiver returns empty string", func(t *testing.T) {
		var cp *ConfigPersistence
		assert.Equal(t, "", cp.ReportSchedulesPath())
	})

	t.Run("populated receiver returns joined path equal to internal field", func(t *testing.T) {
		dir := t.TempDir()
		cp := NewConfigPersistence(dir)

		got := cp.ReportSchedulesPath()
		want := filepath.Join(dir, "report_schedules.json")
		assert.Equal(t, want, got, "ReportSchedulesPath must be dir/report_schedules.json")
		assert.Equal(t, cp.reportSchedulesFile, got, "ReportSchedulesPath must mirror the configured field")
		assert.True(t, filepath.IsAbs(got), "path should be absolute under a temp dir")
		assert.True(t, strings.HasSuffix(got, "report_schedules.json"))
	})
}

// TestBranchcov0722PM_LoadReportScheduleStore covers every return path of
// (*ConfigPersistence).LoadReportScheduleStore: nil-receiver short-circuit,
// missing-file (os.ErrNotExist) default, plain-parse error (crypto nil),
// decrypt error (crypto non-nil, corrupt ciphertext), generic read error, and
// the success path round-tripped through Save.
func TestBranchcov0722PM_LoadReportScheduleStore(t *testing.T) {
	t.Run("nil receiver returns empty store and nil error", func(t *testing.T) {
		var cp *ConfigPersistence
		store, err := cp.LoadReportScheduleStore()
		require.NoError(t, err)
		require.NotNil(t, store, "nil receiver must still yield a usable empty store")
		require.NotNil(t, store.Schedules, "Schedules must be non-nil so it serializes as [] not null")
		assert.Empty(t, store.Schedules)
	})

	t.Run("missing file yields default empty store with no error", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		require.NoFileExists(t, cp.ReportSchedulesPath())

		store, err := cp.LoadReportScheduleStore()
		require.NoError(t, err)
		require.NotNil(t, store)
		require.NotNil(t, store.Schedules, "missing file must normalize to non-nil empty slice")
		assert.Empty(t, store.Schedules)
	})

	t.Run("corrupt file with crypto disabled surfaces parse error", func(t *testing.T) {
		dir := t.TempDir()
		cp := newPlainReportSchedulePersistence(dir)
		require.NoError(t, os.WriteFile(cp.ReportSchedulesPath(), []byte("{not valid json"), 0600))

		store, err := cp.LoadReportScheduleStore()
		require.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "parse report schedules",
			"with crypto nil the parse-error path must be taken (not the decrypt path)")
	})

	t.Run("corrupt file with crypto enabled surfaces decrypt error", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		// Write plaintext garbage; with crypto enabled, parsing fails first,
		// then Decrypt also fails on the non-ciphertext bytes.
		require.NoError(t, os.WriteFile(cp.ReportSchedulesPath(), []byte("{not valid json"), 0600))

		store, err := cp.LoadReportScheduleStore()
		require.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "decrypt report schedules",
			"with crypto enabled the decrypt-error path must be taken")
	})

	t.Run("non-ErrNotExist read error is wrapped as read report schedules", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		// Reuse the package-level mockFSError helper (see persistence_coverage_test.go)
		// to inject a read failure that is NOT os.ErrNotExist, exercising the
		// non-not-found error arm of Load.
		cp.SetFileSystem(&mockFSError{
			FileSystem: defaultFileSystem{},
			readError:  errors.New("disk read failed"),
		})

		store, err := cp.LoadReportScheduleStore()
		require.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "read report schedules")
		assert.Contains(t, err.Error(), "disk read failed")
	})

	t.Run("save then load round-trips every field through encryption", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())

		created := time.Date(2024, time.March, 15, 8, 30, 0, 0, time.UTC)
		lastRun := created.Add(-24 * time.Hour)
		nextRun := created.Add(7 * 24 * time.Hour)
		want := ReportScheduleStore{Schedules: []ReportSchedule{
			{
				ID:      "sched-1",
				Name:    "Monthly Uptime Report",
				Enabled: true,
				Cadence: ReportScheduleCadence{
					Type:       ReportScheduleCadenceMonthly,
					DayOfMonth: 15,
					Time:       "08:00",
					Timezone:   "UTC",
				},
				Window: "last_month",
				// Deliberately uppercase to prove Save normalizes before persisting.
				Format:         "PDF",
				RetentionCount: 7,
				Scope: ReportScheduleScope{
					Resources: []ReportScheduleResource{
						{ResourceType: "HOST", ResourceID: "h-1", Name: "Host One"},
					},
					Tags: []string{"prod"},
				},
				Delivery: ReportScheduleDelivery{
					Method:     ReportScheduleDeliveryEmail,
					To:         []string{"ops@example.com"},
					Attach:     true,
					SaveToDisk: true,
				},
				LastRunAt:         &lastRun,
				LastRunStatus:     ReportScheduleLastRunOK,
				NextRunAt:         &nextRun,
				LastOccurrenceKey: "2024-03",
				CreatedAt:         created,
				UpdatedAt:         created,
			},
		}}

		require.NoError(t, cp.SaveReportScheduleStore(want))

		// Sanity: with crypto enabled, the persisted bytes must not leak the
		// plaintext schedule identifier (i.e. the Encrypt branch ran).
		raw, err := os.ReadFile(cp.ReportSchedulesPath())
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "Monthly Uptime Report",
			"encrypted store must not contain plaintext schedule name")

		got, err := cp.LoadReportScheduleStore()
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Len(t, got.Schedules, 1)

		s := got.Schedules[0]
		assert.Equal(t, "sched-1", s.ID)
		assert.Equal(t, "Monthly Uptime Report", s.Name)
		assert.True(t, s.Enabled)
		assert.Equal(t, ReportScheduleCadenceMonthly, s.Cadence.Type)
		assert.Equal(t, 15, s.Cadence.DayOfMonth)
		assert.Equal(t, "08:00", s.Cadence.Time)
		assert.Equal(t, "UTC", s.Cadence.Timezone)
		assert.Equal(t, "last_month", s.Window)
		// Normalized (lowercased) by Save.
		assert.Equal(t, "pdf", s.Format)
		assert.Equal(t, 7, s.RetentionCount, "non-zero retention is preserved verbatim")
		require.Len(t, s.Scope.Resources, 1)
		assert.Equal(t, "host", s.Scope.Resources[0].ResourceType, "resource type lowercased by Save normalize")
		assert.Equal(t, "h-1", s.Scope.Resources[0].ResourceID)
		assert.Equal(t, "Host One", s.Scope.Resources[0].Name)
		assert.Equal(t, []string{"prod"}, s.Scope.Tags)
		assert.Equal(t, ReportScheduleDeliveryEmail, s.Delivery.Method)
		assert.Equal(t, []string{"ops@example.com"}, s.Delivery.To)
		assert.True(t, s.Delivery.Attach)
		assert.True(t, s.Delivery.SaveToDisk)
		// Pointer fields round-trip through JSON.
		require.NotNil(t, s.LastRunAt)
		assert.True(t, s.LastRunAt.Equal(lastRun), "LastRunAt round-tripped: got %v want %v", *s.LastRunAt, lastRun)
		require.NotNil(t, s.NextRunAt)
		assert.True(t, s.NextRunAt.Equal(nextRun), "NextRunAt round-tripped: got %v want %v", *s.NextRunAt, nextRun)
		assert.Equal(t, ReportScheduleLastRunOK, s.LastRunStatus)
		assert.Equal(t, "2024-03", s.LastOccurrenceKey)
		assert.True(t, s.CreatedAt.Equal(created), "CreatedAt round-tripped")
		assert.True(t, s.UpdatedAt.Equal(created), "UpdatedAt round-tripped")
	})
}

// TestBranchcov0722PM_SaveReportScheduleStore covers every return path of
// (*ConfigPersistence).SaveReportScheduleStore: nil-receiver guard, successful
// write (with normalization observable on disk), the generic write-error wrap,
// and the missing-parent-directory error arm.
func TestBranchcov0722PM_SaveReportScheduleStore(t *testing.T) {
	t.Run("nil receiver returns not-configured error", func(t *testing.T) {
		var cp *ConfigPersistence
		err := cp.SaveReportScheduleStore(EmptyReportScheduleStore())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config persistence is not configured")
	})

	t.Run("successful save writes normalized plaintext JSON when crypto is nil", func(t *testing.T) {
		dir := t.TempDir()
		cp := newPlainReportSchedulePersistence(dir)

		store := ReportScheduleStore{Schedules: []ReportSchedule{
			{
				ID:             "  s1  ",
				Name:           "  Weekly CSV  ",
				Format:         "CSV",
				RetentionCount: 0, // expect default applied by normalize
				Scope:          ReportScheduleScope{Tags: []string{"  Prod  ", "prod", "staging"}},
			},
		}}
		require.NoError(t, cp.SaveReportScheduleStore(store))

		raw, err := os.ReadFile(cp.ReportSchedulesPath())
		require.NoError(t, err)
		body := string(raw)

		// Save normalizes before marshaling: trimmed IDs, lowercased format,
		// default retention, and de-duplicated/lowercased tag slice.
		assert.Contains(t, body, `"id": "s1"`)
		assert.Contains(t, body, `"name": "Weekly CSV"`)
		assert.Contains(t, body, `"format": "csv"`)
		assert.Contains(t, body, `"retention_count": 12`)
		assert.Contains(t, body, `"tags": [`)

		// File is created with the restrictive perm passed by Save (0600).
		info, err := os.Stat(cp.ReportSchedulesPath())
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})

	t.Run("write error is wrapped as write report schedules", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		cp.SetFileSystem(&mockFSError{
			FileSystem: defaultFileSystem{},
			writeError: errors.New("disk full"),
		})

		err := cp.SaveReportScheduleStore(EmptyReportScheduleStore())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write report schedules")
		assert.Contains(t, err.Error(), "disk full")
	})

	t.Run("missing parent directory yields a write error", func(t *testing.T) {
		// writeConfigFileLocked does not create parent dirs (callers are
		// expected to EnsureConfigDir first); saving into a path whose parent
		// does not exist therefore surfaces the wrapped write error.
		dir := t.TempDir()
		cp := &ConfigPersistence{
			configDir:           dir,
			reportSchedulesFile: filepath.Join(dir, "does_not_exist", "report_schedules.json"),
			fs:                  defaultFileSystem{},
		}

		err := cp.SaveReportScheduleStore(EmptyReportScheduleStore())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write report schedules")
		assert.NoFileExists(t, cp.ReportSchedulesPath())
	})
}
