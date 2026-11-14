package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func TestApplianceOutboundPolicySurvivesEncryptedReloadButNotPublicProjection(t *testing.T) {
	dir := t.TempDir()
	store := NewConfigPersistence(dir)
	nas := NewTrueNASInstance()
	nas.Host = "nas.example.test"
	nas.Username = "fixture"
	nas.APIKey = "synthetic-key"
	nas.OutboundPolicy = securityutil.ConnectionOutboundTenantLAN
	vc := NewVMwareVCenterInstance()
	vc.Host = "vc.example.test"
	vc.Username = "fixture"
	vc.Password = "synthetic-password"
	vc.OutboundPolicy = securityutil.ConnectionOutboundTenantPublic
	if err := store.SaveTrueNASConfig([]TrueNASInstance{nas}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVMwareConfig([]VMwareVCenterInstance{vc}); err != nil {
		t.Fatal(err)
	}
	fresh := NewConfigPersistence(dir)
	nasRows, err := fresh.LoadTrueNASConfig()
	if err != nil {
		t.Fatal(err)
	}
	vcRows, err := fresh.LoadVMwareConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(nasRows) != 1 || nasRows[0] != nas || len(vcRows) != 1 || vcRows[0] != vc {
		t.Fatal("reload lost policy or other fields")
	}
	for _, projection := range []any{nasRows[0].Redacted(), vcRows[0].Redacted()} {
		data, err := json.Marshal(projection)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "outboundPolicy") || strings.Contains(string(data), "synthetic-") {
			t.Fatalf("internal metadata or credentials in projection: %s", data)
		}
	}
	var legacyNAS TrueNASInstance
	var legacyVC VMwareVCenterInstance
	if err := json.Unmarshal([]byte(`{"host":"127.0.0.1"}`), &legacyNAS); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"host":"127.0.0.1"}`), &legacyVC); err != nil {
		t.Fatal(err)
	}
	legacyNAS.ApplyDefaults()
	legacyVC.ApplyDefaults()
	if legacyNAS.OutboundPolicy != "" || legacyVC.OutboundPolicy != "" {
		t.Fatal("invented actor provenance for old records")
	}
}
