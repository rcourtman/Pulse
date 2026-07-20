package config_test

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAlertIntentPolicyPersistenceMissingAndRoundTrip(t *testing.T) {
	cp := config.NewConfigPersistence(t.TempDir())
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	missing, err := cp.LoadAlertIntentPolicies()
	if err != nil {
		t.Fatalf("LoadAlertIntentPolicies missing: %v", err)
	}
	if missing.SchemaVersion != alerts.CurrentAlertIntentPolicySchemaVersion || missing.Revision != 0 {
		t.Fatalf("missing policy document = %+v", missing)
	}

	grace := 75
	document := alerts.NewAlertIntentPolicyDocument()
	document.Revision = 4
	document.Resources["vm:101"] = map[string]alerts.AlertIntentRule{
		string(alerts.AlertIntentSignalOffline): {
			GraceSeconds: &grace,
			BackupOffline: &alerts.BackupOfflineIntentPolicy{
				Enabled: true, PostGraceSeconds: 30, MaxDeferralSeconds: 900,
			},
		},
	}
	if err := cp.SaveAlertIntentPolicies(document); err != nil {
		t.Fatalf("SaveAlertIntentPolicies: %v", err)
	}
	loaded, err := cp.LoadAlertIntentPolicies()
	if err != nil {
		t.Fatalf("LoadAlertIntentPolicies: %v", err)
	}
	want := alerts.NormalizeAlertIntentPolicyDocument(document)
	if !reflect.DeepEqual(*loaded, want) {
		t.Fatalf("loaded policy = %#v, want %#v", *loaded, want)
	}
}

func TestExportImportIncludesAlertIntentPolicies(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	source := config.NewConfigPersistence(t.TempDir())
	if err := source.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir source: %v", err)
	}

	grace := 120
	document := alerts.NewAlertIntentPolicyDocument()
	document.Resources["service:dns"] = map[string]alerts.AlertIntentRule{
		string(alerts.AlertIntentSignalAvailability): {GraceSeconds: &grace},
	}
	if err := source.SaveAlertIntentPolicies(document); err != nil {
		t.Fatalf("SaveAlertIntentPolicies: %v", err)
	}

	const passphrase = "intent-policy-round-trip"
	bundle, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}
	decoded := mustDecodeExport(t, bundle, passphrase)
	if decoded.Version != "4.3" || decoded.AlertIntent == nil {
		t.Fatalf("export metadata = version %q intent %#v", decoded.Version, decoded.AlertIntent)
	}

	destination := config.NewConfigPersistence(t.TempDir())
	if err := destination.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir destination: %v", err)
	}
	if err := destination.ImportConfig(bundle, passphrase); err != nil {
		t.Fatalf("ImportConfig: %v", err)
	}
	loaded, err := destination.LoadAlertIntentPolicies()
	if err != nil {
		t.Fatalf("LoadAlertIntentPolicies: %v", err)
	}
	want := alerts.NormalizeAlertIntentPolicyDocument(document)
	if !reflect.DeepEqual(*loaded, want) {
		t.Fatalf("imported policy = %#v, want %#v", *loaded, want)
	}
}
