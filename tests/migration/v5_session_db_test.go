// Package migration contains integration tests verifying v5→v6 migration safety
// for session/CSRF token continuity and SQLite database schema auto-migration.
package migration

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestV5DataDir_SessionTokenContinuity verifies that sessions.json written by a
// v5 binary in the hashed format are correctly loaded by the v6 SessionStore,
// preserving all session data across the binary upgrade.
func TestV5DataDir_SessionTokenContinuity(t *testing.T) {
	dataDir := t.TempDir()

	// Simulate v5-written sessions.json in hashed (current) format.
	// v5 writes sessions as a JSON array of objects with SHA256-hashed keys.
	// The raw tokens "token-admin-v5", "token-viewer-v5", and "token-expired-v5"
	// map to their respective SHA256 hashes.
	adminToken := "token-admin-v5"
	viewerToken := "token-viewer-v5"
	expiredToken := "token-expired-v5"
	adminHash := "100e59de5770d3d39661074377e2a1917a888664833ade104d8d9c53a0fbca7c"  // sha256("token-admin-v5")
	viewerHash := "9a3f360e528a5da0be0d0125fffe8c0184dda6a12a2fa44dc6c5b1c0f561cc1d" // sha256("token-viewer-v5")
	futureExpiry := time.Now().Add(24 * time.Hour)

	v5Sessions := []map[string]interface{}{
		{
			"key":               adminHash,
			"username":          "admin",
			"expires_at":        futureExpiry.Format(time.RFC3339Nano),
			"created_at":        time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano),
			"user_agent":        "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0",
			"ip":                "192.168.1.100",
			"original_duration": float64(86400000000000), // 24h in nanoseconds
		},
		{
			"key":               viewerHash,
			"username":          "viewer",
			"expires_at":        futureExpiry.Format(time.RFC3339Nano),
			"created_at":        time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
			"user_agent":        "curl/7.88.1",
			"ip":                "10.0.0.5",
			"original_duration": float64(3600000000000), // 1h in nanoseconds
		},
		{
			// Expired session — should be filtered out during load.
			// Uses sha256("token-expired-v5") so we can verify by raw token lookup.
			"key":        "fec6dfe256d34be31619a833ea19b2866838af691d19b9086c63570b80324ab0",
			"username":   "old-user",
			"expires_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano),
			"created_at": time.Now().Add(-25 * time.Hour).Format(time.RFC3339Nano),
		},
	}

	data, err := json.Marshal(v5Sessions)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "sessions.json"), data, 0o600))

	// v6 SessionStore loads sessions.json — sessions should survive
	store := api.NewSessionStore(dataDir)
	require.NotNil(t, store)

	// Verify the v5 "admin" session was loaded and is accessible via the raw token.
	// SessionStore hashes the raw token to look up the key in the map.
	assert.True(t, store.ValidateSession(adminToken), "v5 admin session must be valid after v6 load")
	adminSD := store.GetSession(adminToken)
	require.NotNil(t, adminSD, "v5 admin session data must be retrievable by raw token")
	assert.Equal(t, "admin", adminSD.Username)
	assert.Equal(t, "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0", adminSD.UserAgent)
	assert.Equal(t, "192.168.1.100", adminSD.IP)

	// Verify the v5 "viewer" session was loaded
	assert.True(t, store.ValidateSession(viewerToken), "v5 viewer session must be valid after v6 load")
	viewerSD := store.GetSession(viewerToken)
	require.NotNil(t, viewerSD, "v5 viewer session data must be retrievable")
	assert.Equal(t, "viewer", viewerSD.Username)
	assert.Equal(t, "curl/7.88.1", viewerSD.UserAgent)

	// Verify the expired session was filtered out during load — use the exact
	// raw token whose hash is in the file, confirming load-time expiry filtering.
	assert.False(t, store.ValidateSession(expiredToken), "expired session must be filtered out during load")
	assert.Nil(t, store.GetSession(expiredToken), "expired session data must not be present")

	// Verify sliding expiration works (v6 feature) on a v5-loaded session.
	// The fixture sets OriginalDuration = 24h and ExpiresAt = now+24h (set at file
	// write time), so ValidateAndExtendSession recalculates ExpiresAt = time.Now() + 24h
	// which must be strictly after the original value (time has advanced since write).
	beforeExtend := store.GetSession(adminToken).ExpiresAt
	assert.True(t, store.ValidateAndExtendSession(adminToken), "sliding expiration must work on v5 session")
	afterExtend := store.GetSession(adminToken).ExpiresAt
	assert.True(t, afterExtend.After(beforeExtend),
		"ExpiresAt must increase after ValidateAndExtendSession (was %v, now %v)", beforeExtend, afterExtend)
}

// TestV5DataDir_SessionLegacyMapFormat verifies that sessions.json in the legacy
// v5 map format (raw tokens as keys) is correctly loaded and migrated to the
// hashed format by v6.
func TestV5DataDir_SessionLegacyMapFormat(t *testing.T) {
	dataDir := t.TempDir()

	rawToken := "legacy-v5-session-token-plaintext"
	futureExpiry := time.Now().Add(12 * time.Hour)

	// Legacy v5 format: map[rawToken] -> SessionData
	legacySessions := map[string]map[string]interface{}{
		rawToken: {
			"username":   "legacyuser",
			"expires_at": futureExpiry.Format(time.RFC3339Nano),
			"created_at": time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano),
			"user_agent": "Mozilla/5.0 legacy browser",
			"ip":         "192.168.0.50",
		},
	}

	data, err := json.Marshal(legacySessions)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "sessions.json"), data, 0o600))

	// v6 SessionStore should load legacy format and migrate to hashed keys
	store := api.NewSessionStore(dataDir)
	require.NotNil(t, store)

	// The legacy token should be valid — the store hashes it on load
	assert.True(t, store.ValidateSession(rawToken), "legacy raw-token session must validate in v6")

	sd := store.GetSession(rawToken)
	require.NotNil(t, sd, "legacy session data must be retrievable")
	assert.Equal(t, "legacyuser", sd.Username)

	// Re-read the file and verify it's now immediately in the hashed (array) format,
	// not legacy map format.
	savedData, err := os.ReadFile(filepath.Join(dataDir, "sessions.json"))
	require.NoError(t, err)
	var hashedFormat []json.RawMessage
	require.NoError(t, json.Unmarshal(savedData, &hashedFormat), "after save, sessions.json must be in hashed array format")
	assert.Len(t, hashedFormat, 1, "saved file must contain the migrated legacy session only")
}

// TestV5DataDir_CSRFTokenFileContinuity verifies that csrf_tokens.json written
// by v5 in the hashed format is loadable by the v6 binary.
//
// NOTE: The CSRFTokenStore uses sync.Once for initialization, making it
// impossible to create isolated instances in tests. We verify format
// compatibility by confirming the v5 file deserializes into the v6
// CSRFTokenData type — the same struct used by CSRFTokenStore.load().
func TestV5DataDir_CSRFTokenFileContinuity(t *testing.T) {
	dataDir := t.TempDir()

	futureExpiry := time.Now().Add(4 * time.Hour)

	// Write v5-format csrf_tokens.json (hashed format)
	v5CSRFTokens := []map[string]interface{}{
		{
			"token_hash":  "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			"session_key": "session-key-hash-1",
			"expires_at":  futureExpiry.Format(time.RFC3339Nano),
		},
		{
			"token_hash":  "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5",
			"session_key": "session-key-hash-2",
			"expires_at":  futureExpiry.Format(time.RFC3339Nano),
		},
	}

	data, err := json.Marshal(v5CSRFTokens)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "csrf_tokens.json"), data, 0o600))

	// Verify the file is valid JSON that deserializes into the v6 CSRFTokenData format
	fileData, err := os.ReadFile(filepath.Join(dataDir, "csrf_tokens.json"))
	require.NoError(t, err)

	var tokens []api.CSRFTokenData
	require.NoError(t, json.Unmarshal(fileData, &tokens), "v5 csrf_tokens.json must unmarshal into v6 CSRFTokenData")
	require.Len(t, tokens, 2)

	assert.Equal(t, "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", tokens[0].TokenHash)
	assert.Equal(t, "session-key-hash-1", tokens[0].SessionKey)
	assert.False(t, tokens[0].ExpiresAt.IsZero())
	assert.True(t, tokens[0].ExpiresAt.After(time.Now()), "token should not be expired")
}

// TestV5DataDir_CSRFLegacyMapFormat verifies that csrf_tokens.json in the legacy
// v5 map format (raw tokens) deserializes correctly for the v6 migration path.
// See TestV5DataDir_CSRFTokenFileContinuity for the sync.Once limitation note.
func TestV5DataDir_CSRFLegacyMapFormat(t *testing.T) {
	dataDir := t.TempDir()

	futureExpiry := time.Now().Add(4 * time.Hour)

	// Legacy format: map[sessionID] -> {token, session_id, expires_at}
	legacyCSRF := map[string]map[string]interface{}{
		"raw-session-id-1": {
			"token":      "raw-csrf-token-plaintext",
			"session_id": "raw-session-id-1",
			"expires_at": futureExpiry.Format(time.RFC3339Nano),
		},
	}

	data, err := json.Marshal(legacyCSRF)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "csrf_tokens.json"), data, 0o600))

	// Verify the legacy format can be read (the current format unmarshal will fail,
	// but the fallback legacy unmarshal should succeed)
	fileData, err := os.ReadFile(filepath.Join(dataDir, "csrf_tokens.json"))
	require.NoError(t, err)

	// v6 load() tries hashed format first, then falls back to legacy map format.
	// Verify the legacy format deserializes into the expected legacy struct shape.
	type legacyCSRFEntry struct {
		Token     string    `json:"token"`
		SessionID string    `json:"session_id"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	var legacyMap map[string]*legacyCSRFEntry
	require.NoError(t, json.Unmarshal(fileData, &legacyMap), "legacy csrf_tokens.json must unmarshal as map[string]*legacyCSRFEntry")
	require.Contains(t, legacyMap, "raw-session-id-1")

	entry := legacyMap["raw-session-id-1"]
	require.NotNil(t, entry)
	assert.Equal(t, "raw-csrf-token-plaintext", entry.Token)
	assert.Equal(t, "raw-session-id-1", entry.SessionID)
	assert.False(t, entry.ExpiresAt.IsZero(), "expires_at must parse correctly")
}

// TestV5DataDir_MissingSessionFiles verifies that v6 starts cleanly when no
// sessions.json or csrf_tokens.json exist (fresh install or v5 install that
// never had active sessions persisted).
func TestV5DataDir_MissingSessionFiles(t *testing.T) {
	dataDir := t.TempDir()

	// No sessions.json or csrf_tokens.json — v6 should start cleanly
	store := api.NewSessionStore(dataDir)
	require.NotNil(t, store)

	// Store should be empty but functional
	assert.False(t, store.ValidateSession("nonexistent-token"))

	// Creating sessions should work on a fresh store
	store.CreateSession("fresh-token", time.Hour, "TestAgent", "127.0.0.1", "admin")
	assert.True(t, store.ValidateSession("fresh-token"))
}

// TestV5DataDir_MetricsDBSchemaAutoMigration verifies that the v6 metrics store
// can open a v5-era metrics.db (with a minimal schema) and auto-migrate it by
// adding missing indexes and tables without losing existing data.
func TestV5DataDir_MetricsDBSchemaAutoMigration(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "metrics.db")

	// Create a minimal v5-era metrics database with just the basic table
	// (no unique index, no metrics_meta table — these are v6 additions).
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(5000)",
			"journal_mode(WAL)",
		},
	}.Encode()
	rawDB, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)

	// Create v5 schema (basic metrics table only, no unique index)
	_, err = rawDB.Exec(`
		CREATE TABLE IF NOT EXISTS metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			metric_type TEXT NOT NULL,
			value REAL NOT NULL,
			min_value REAL,
			max_value REAL,
			timestamp INTEGER NOT NULL,
			tier TEXT NOT NULL DEFAULT 'raw'
		);
		CREATE INDEX IF NOT EXISTS idx_metrics_lookup
		ON metrics(resource_type, resource_id, metric_type, tier, timestamp);
	`)
	require.NoError(t, err)

	// Insert v5-era test data
	now := time.Now()
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute).Unix()
		_, err = rawDB.Exec(`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
			VALUES (?, ?, ?, ?, ?, ?)`,
			"node", "pve-node-1", "cpu", 45.0+float64(i)*5.0, ts, "raw")
		require.NoError(t, err)
	}

	// Verify v5 data was written
	var count int
	require.NoError(t, rawDB.QueryRow("SELECT COUNT(*) FROM metrics").Scan(&count))
	assert.Equal(t, 5, count, "v5 metrics data should be present")
	rawDB.Close()

	// Now open with v6 metrics store — should auto-migrate schema
	cfg := metrics.StoreConfig{
		DBPath:          dbPath,
		WriteBufferSize: 10,
		FlushInterval:   1 * time.Second,
		RetentionRaw:    2 * time.Hour,
		RetentionMinute: 24 * time.Hour,
		RetentionHourly: 7 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	}
	store, err := metrics.NewStore(cfg)
	require.NoError(t, err, "v6 metrics store must open v5 database without error")
	defer store.Close()

	// Verify v5 data survived the schema migration
	start := now.Add(-10 * time.Minute)
	end := now.Add(time.Minute)
	points, err := store.Query("node", "pve-node-1", "cpu", start, end, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(points), 5, "all v5 metric points must survive schema migration")

	// Verify v6 can write new data to the migrated database synchronously.
	// Use WriteBatchSync to bypass the async buffer and confirm the insert directly.
	v6WriteTime := now.Add(10 * time.Minute)
	store.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "node",
		ResourceID:   "pve-node-1",
		MetricType:   "cpu",
		Value:        99.0,
		Timestamp:    v6WriteTime,
		Tier:         metrics.TierRaw,
	}})

	// Query again to verify new data was written
	points2, err := store.Query("node", "pve-node-1", "cpu", start, v6WriteTime.Add(time.Minute), 0)
	require.NoError(t, err)
	assert.Equal(t, 6, len(points2), "v5 data (5) + v6 write (1) must all be present")
}

// TestV5DataDir_MetricsDBDuplicateDedup verifies that the v6 metrics store
// handles a v5 database containing duplicate metrics (which older versions
// could produce) by deduplicating and creating a unique index.
func TestV5DataDir_MetricsDBDuplicateDedup(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "metrics.db")

	// Create v5-era database with intentional duplicates
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(5000)",
			"journal_mode(WAL)",
		},
	}.Encode()
	rawDB, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)

	_, err = rawDB.Exec(`
		CREATE TABLE IF NOT EXISTS metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			metric_type TEXT NOT NULL,
			value REAL NOT NULL,
			min_value REAL,
			max_value REAL,
			timestamp INTEGER NOT NULL,
			tier TEXT NOT NULL DEFAULT 'raw'
		);
	`)
	require.NoError(t, err)

	// Insert duplicate entries (same resource/metric/timestamp/tier)
	ts := time.Now().Unix()
	for i := 0; i < 3; i++ {
		_, err = rawDB.Exec(`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
			VALUES (?, ?, ?, ?, ?, ?)`,
			"vm", "vm-100", "memory", 72.5, ts, "raw")
		require.NoError(t, err)
	}

	var count int
	require.NoError(t, rawDB.QueryRow("SELECT COUNT(*) FROM metrics").Scan(&count))
	assert.Equal(t, 3, count, "should have 3 duplicate rows before migration")
	rawDB.Close()

	// v6 store should detect duplicates, deduplicate, and create unique index
	cfg := metrics.StoreConfig{
		DBPath:          dbPath,
		WriteBufferSize: 10,
		FlushInterval:   1 * time.Second,
		RetentionRaw:    2 * time.Hour,
		RetentionMinute: 24 * time.Hour,
		RetentionHourly: 7 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	}
	store, err := metrics.NewStore(cfg)
	require.NoError(t, err, "v6 must open v5 database with duplicates and auto-deduplicate")
	defer store.Close()

	// After migration, only 1 copy should remain
	start := time.Unix(ts-60, 0)
	end := time.Unix(ts+60, 0)
	points, err := store.Query("vm", "vm-100", "memory", start, end, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, len(points), "duplicates must be deduplicated to 1 row")
}

// TestV5DataDir_AuditDBSchemaAutoMigration verifies that the v6 audit logger
// can create a new audit.db in a v5 data directory and that the schema includes
// all required tables and indexes for v6 operation.
func TestV5DataDir_AuditDBSchemaAutoMigration(t *testing.T) {
	dataDir := t.TempDir()

	// Create audit logger (creates audit.db with v6 schema)
	logger, err := audit.NewSQLiteLogger(audit.SQLiteLoggerConfig{
		DataDir:       dataDir,
		RetentionDays: 90,
	})
	require.NoError(t, err, "v6 audit logger must initialize on v5 data directory")
	defer logger.Close()

	// Log a test event to verify the schema works
	event := audit.Event{
		ID:        "test-migration-event-1",
		Timestamp: time.Now(),
		EventType: "config_change",
		User:      "admin",
		IP:        "192.168.1.1",
		Path:      "/api/settings",
		Success:   true,
		Details:   "test migration verification",
	}
	require.NoError(t, logger.Log(event))

	// Query the event back
	events, err := logger.Query(audit.QueryFilter{ID: "test-migration-event-1"})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "config_change", events[0].EventType)
	assert.Equal(t, "admin", events[0].User)
	assert.True(t, events[0].Success)

	// Verify the schema_version table exists and has version 1
	dbPath := filepath.Join(dataDir, "audit", "audit.db")
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{"busy_timeout(5000)"},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()

	var version int
	require.NoError(t, db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version))
	assert.Equal(t, 1, version, "schema_version should be 1")
}

// TestV5DataDir_AuditDBPreExistingData verifies that the v6 audit logger
// preserves events already in an audit.db created by v5.
func TestV5DataDir_AuditDBPreExistingData(t *testing.T) {
	dataDir := t.TempDir()
	auditDir := filepath.Join(dataDir, "audit")
	require.NoError(t, os.MkdirAll(auditDir, 0o700))
	dbPath := filepath.Join(auditDir, "audit.db")

	// Create a v5-era audit database with some events
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(5000)",
			"journal_mode(WAL)",
		},
	}.Encode()
	rawDB, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)

	_, err = rawDB.Exec(`
		CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			timestamp INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			user TEXT,
			ip TEXT,
			path TEXT,
			success INTEGER NOT NULL,
			details TEXT,
			signature TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS audit_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		);
	`)
	require.NoError(t, err)

	// Insert v5 audit events
	ts := time.Now().Unix()
	_, err = rawDB.Exec(`INSERT INTO audit_events (id, timestamp, event_type, user, ip, path, success, details, signature)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"v5-event-001", ts, "login", "admin", "192.168.1.1", "/api/auth/login", 1, "successful login", "v5-sig-placeholder")
	require.NoError(t, err)
	_, err = rawDB.Exec(`INSERT INTO audit_events (id, timestamp, event_type, user, ip, path, success, details, signature)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"v5-event-002", ts-3600, "config_change", "admin", "10.0.0.1", "/api/settings", 1, "changed polling interval", "v5-sig-placeholder-2")
	require.NoError(t, err)
	rawDB.Close()

	// Open with v6 audit logger — should auto-migrate (add schema_version table, indexes)
	logger, err := audit.NewSQLiteLogger(audit.SQLiteLoggerConfig{
		DataDir:       dataDir,
		RetentionDays: 90,
	})
	require.NoError(t, err, "v6 audit logger must open v5 audit.db without error")
	defer logger.Close()

	// Verify v5 events survived the migration
	events, err := logger.Query(audit.QueryFilter{ID: "v5-event-001"})
	require.NoError(t, err)
	require.Len(t, events, 1, "v5 audit event must survive v6 schema migration")
	assert.Equal(t, "login", events[0].EventType)
	assert.Equal(t, "admin", events[0].User)
	assert.True(t, events[0].Success)

	events2, err := logger.Query(audit.QueryFilter{ID: "v5-event-002"})
	require.NoError(t, err)
	require.Len(t, events2, 1)
	assert.Equal(t, "config_change", events2[0].EventType)

	// Verify v6 can write new events alongside v5 data
	newEvent := audit.Event{
		ID:        "v6-event-001",
		Timestamp: time.Now(),
		EventType: "logout",
		User:      "admin",
		IP:        "192.168.1.1",
		Success:   true,
		Details:   "v6 logout after migration",
	}
	require.NoError(t, logger.Log(newEvent))

	// Verify both v5 and v6 events coexist
	allEvents, err := logger.Query(audit.QueryFilter{Limit: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allEvents), 3, "v5 + v6 events must coexist")
}
