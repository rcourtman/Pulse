package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"testing"
	"time"
)

func TestSQLiteV2AndLegacySignaturesSurviveRestartQueryAndExport(t *testing.T) {
	dataDir := t.TempDir()
	signingKey := []byte("0123456789abcdef0123456789abcdef")
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{DataDir: dataDir, SigningKey: signingKey})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}

	v2Event := Event{
		ID:        "v2-event",
		Timestamp: time.Unix(1_725_555_555, 987_654_321).UTC(),
		EventType: "security|alert",
		User:      "監査\noperator",
		IP:        "2001:db8::1",
		Path:      "/api/audit|verify",
		Success:   true,
		Details:   "line one\nline two|done",
	}
	if err := logger.Record(v2Event); err != nil {
		t.Fatalf("Record v2 event: %v", err)
	}

	legacyEvent := Event{
		ID:        "legacy-event",
		Timestamp: time.Unix(1_725_555_556, 0).UTC(),
		EventType: "startup",
		User:      "admin",
		IP:        "127.0.0.1",
		Path:      "/api/audit",
		Success:   true,
		Details:   "historical",
	}
	legacyEvent.Signature = logger.signer.signCanonical(logger.signer.legacyTimeCanonicalForm(legacyEvent))
	if _, err := logger.db.Exec(`
		INSERT INTO audit_events (id, timestamp, event_type, user, ip, path, success, details, signature)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		legacyEvent.ID,
		legacyEvent.Timestamp.Unix(),
		legacyEvent.EventType,
		legacyEvent.User,
		legacyEvent.IP,
		legacyEvent.Path,
		1,
		legacyEvent.Details,
		legacyEvent.Signature,
	); err != nil {
		t.Fatalf("insert historical fixture: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	restarted, err := NewSQLiteLogger(SQLiteLoggerConfig{DataDir: dataDir, SigningKey: signingKey})
	if err != nil {
		t.Fatalf("restart SQLite logger: %v", err)
	}
	defer restarted.Close()

	events, total, err := restarted.QueryPage(QueryFilter{Limit: 10})
	if err != nil {
		t.Fatalf("QueryPage: %v", err)
	}
	if len(events) != 2 || total != 2 {
		t.Fatalf("QueryPage = %d events, total %d; want 2/2", len(events), total)
	}
	versions := make(map[string]SignatureVersion, len(events))
	for _, event := range events {
		versions[event.ID] = DetectSignatureVersion(event.Signature)
		if !restarted.VerifySignature(event) {
			t.Fatalf("signature for %s did not verify after restart", event.ID)
		}
	}
	if versions[v2Event.ID] != SignatureVersionV2 {
		t.Fatalf("new SQLite row version = %q, want v2", versions[v2Event.ID])
	}
	if versions[legacyEvent.ID] != SignatureVersionLegacy {
		t.Fatalf("historical SQLite row version = %q, want legacy", versions[legacyEvent.ID])
	}

	var storedLegacySignature string
	if err := restarted.db.QueryRow(`SELECT signature FROM audit_events WHERE id = ?`, legacyEvent.ID).Scan(&storedLegacySignature); err != nil {
		t.Fatalf("read historical signature: %v", err)
	}
	if storedLegacySignature != legacyEvent.Signature {
		t.Fatal("historical signature was rewritten")
	}

	exporter := NewExporter(restarted)
	jsonResult, err := exporter.Export(QueryFilter{}, ExportFormatJSON, true)
	if err != nil {
		t.Fatalf("JSON export: %v", err)
	}
	var jsonExport struct {
		Events []ExportEvent `json:"events"`
	}
	if err := json.Unmarshal(jsonResult.Data, &jsonExport); err != nil {
		t.Fatalf("decode JSON export: %v", err)
	}
	if len(jsonExport.Events) != 2 {
		t.Fatalf("JSON export events = %d, want 2", len(jsonExport.Events))
	}
	for _, event := range jsonExport.Events {
		if event.SignatureValid == nil || !*event.SignatureValid {
			t.Fatalf("JSON export signature verdict for %s = %v", event.ID, event.SignatureValid)
		}
	}

	csvResult, err := exporter.Export(QueryFilter{}, ExportFormatCSV, true)
	if err != nil {
		t.Fatalf("CSV export: %v", err)
	}
	records, err := csv.NewReader(bytes.NewReader(csvResult.Data)).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV export: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("CSV records = %d, want header plus 2 rows", len(records))
	}
	for _, row := range records[1:] {
		if row[len(row)-1] != "true" {
			t.Fatalf("CSV signature verdict for %s = %q", row[0], row[len(row)-1])
		}
		if DetectSignatureVersion(row[len(row)-2]) == SignatureVersionUnknown {
			t.Fatalf("CSV signature for %s lost its identifiable envelope", row[0])
		}
	}
}

func TestTenantSQLiteFactoryWritesV2Signatures(t *testing.T) {
	manager := NewTenantLoggerManager(t.TempDir(), &SQLiteLoggerFactory{
		CryptoMgr: newMockCryptoManager(),
	})
	defer manager.Close()

	if err := manager.Log("tenant-a", "security|alert", "user|admin", "127.0.0.1", "/api/audit", true, "tenant event"); err != nil {
		t.Fatalf("tenant Log: %v", err)
	}
	events, err := manager.Query("tenant-a", QueryFilter{})
	if err != nil {
		t.Fatalf("tenant Query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("tenant events = %d, want 1", len(events))
	}
	if DetectSignatureVersion(events[0].Signature) != SignatureVersionV2 {
		t.Fatalf("tenant signature version = %q, want v2", DetectSignatureVersion(events[0].Signature))
	}
	logger, ok := manager.GetLogger("tenant-a").(*SQLiteLogger)
	if !ok {
		t.Fatalf("tenant logger type = %T, want *SQLiteLogger", manager.GetLogger("tenant-a"))
	}
	if !logger.VerifySignature(events[0]) {
		t.Fatal("tenant v2 signature did not verify")
	}
}
