package licensing

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConversionStoreInfrastructureOnboardingReport(t *testing.T) {
	store, err := NewConversionStore(filepath.Join(t.TempDir(), "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	windowStart := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(48 * time.Hour)

	for _, event := range []StoredConversionEvent{
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingOpened,
			Surface:        "settings_infrastructure_add",
			IdempotencyKey: "org-a:opened:1",
			CreatedAt:      windowStart.Add(2 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingPathSelected,
			Surface:        "settings_infrastructure_add",
			Capability:     "api",
			IdempotencyKey: "org-a:path:api",
			CreatedAt:      windowStart.Add(2*time.Hour + time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingProbeResult,
			Surface:        "settings_infrastructure_add",
			Capability:     "no-match",
			IdempotencyKey: "org-a:probe:no-match",
			CreatedAt:      windowStart.Add(2*time.Hour + 2*time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingPathSelected,
			Surface:        "settings_infrastructure_add",
			Capability:     "agent",
			IdempotencyKey: "org-a:path:agent",
			CreatedAt:      windowStart.Add(2*time.Hour + 3*time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingCredentialsOpened,
			Surface:        "settings_infrastructure_add",
			Capability:     "agent",
			IdempotencyKey: "org-a:credentials:agent",
			CreatedAt:      windowStart.Add(2*time.Hour + 4*time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingOpened,
			Surface:        "settings_infrastructure_add",
			IdempotencyKey: "org-a:opened:2",
			CreatedAt:      windowStart.Add(26 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingPathSelected,
			Surface:        "settings_infrastructure_add",
			Capability:     "api",
			IdempotencyKey: "org-a:path:api:2",
			CreatedAt:      windowStart.Add(26*time.Hour + time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingProbeResult,
			Surface:        "settings_infrastructure_add",
			Capability:     "detected",
			IdempotencyKey: "org-a:probe:detected",
			CreatedAt:      windowStart.Add(26*time.Hour + 2*time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingCatalogSelected,
			Surface:        "settings_infrastructure_add",
			Capability:     "truenas",
			IdempotencyKey: "org-a:catalog:truenas",
			CreatedAt:      windowStart.Add(26*time.Hour + 3*time.Minute),
		},
		{
			OrgID:          "org-a",
			EventType:      EventInfrastructureOnboardingCredentialsOpened,
			Surface:        "settings_infrastructure_add",
			Capability:     "truenas",
			IdempotencyKey: "org-a:credentials:truenas",
			CreatedAt:      windowStart.Add(26*time.Hour + 4*time.Minute),
		},
		{
			OrgID:          "org-b",
			EventType:      EventInfrastructureOnboardingOpened,
			Surface:        "settings_infrastructure_add",
			IdempotencyKey: "org-b:opened:1",
			CreatedAt:      windowStart.Add(2 * time.Hour),
		},
	} {
		if err := store.Record(event); err != nil {
			t.Fatalf("Record(%s) error = %v", event.IdempotencyKey, err)
		}
	}

	report, err := store.InfrastructureOnboardingReport("org-a", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("InfrastructureOnboardingReport() error = %v", err)
	}

	if report.Summary.Opened != 2 {
		t.Fatalf("Opened = %d, want 2", report.Summary.Opened)
	}
	if report.Summary.APIPathSelected != 2 {
		t.Fatalf("APIPathSelected = %d, want 2", report.Summary.APIPathSelected)
	}
	if report.Summary.AgentPathSelected != 1 {
		t.Fatalf("AgentPathSelected = %d, want 1", report.Summary.AgentPathSelected)
	}
	if report.Summary.ProbeNoMatch != 1 || report.Summary.ProbeDetected != 1 {
		t.Fatalf("unexpected probe summary: %+v", report.Summary.InfrastructureOnboardingStageCounts)
	}
	if report.Summary.CatalogSelected != 1 {
		t.Fatalf("CatalogSelected = %d, want 1", report.Summary.CatalogSelected)
	}
	if report.Summary.CredentialsOpened != 2 {
		t.Fatalf("CredentialsOpened = %d, want 2", report.Summary.CredentialsOpened)
	}

	if len(report.Daily) != 2 {
		t.Fatalf("len(Daily) = %d, want 2", len(report.Daily))
	}
	if report.Daily[0].ProbeNoMatch != 1 || report.Daily[1].ProbeDetected != 1 {
		t.Fatalf("unexpected daily breakdown: %+v", report.Daily)
	}

	if len(report.Paths) != 2 {
		t.Fatalf("len(Paths) = %d, want 2", len(report.Paths))
	}
	if report.Paths[0].Key != "api" || report.Paths[0].Count != 2 {
		t.Fatalf("unexpected path ranking: %+v", report.Paths)
	}

	if len(report.Platforms) != 1 {
		t.Fatalf("len(Platforms) = %d, want 1", len(report.Platforms))
	}
	if report.Platforms[0].Key != "truenas" ||
		report.Platforms[0].CatalogSelected != 1 ||
		report.Platforms[0].CredentialsOpened != 1 {
		t.Fatalf("unexpected platform breakdown: %+v", report.Platforms)
	}
}

func TestConversionStoreFunnelDimensionBreakdownAccumulatesAcrossMultipleKeys(t *testing.T) {
	store, err := NewConversionStore(filepath.Join(t.TempDir(), "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	windowStart := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)

	for _, event := range []StoredConversionEvent{
		{
			OrgID:          "org-a",
			EventType:      EventCheckoutClicked,
			Surface:        "settings_self_hosted_billing_compare_prompt",
			IdempotencyKey: "org-a:checkout:1",
			CreatedAt:      windowStart.Add(time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventPricingViewed,
			Surface:        "settings_self_hosted_billing_plan",
			IdempotencyKey: "org-a:pricing:1",
			CreatedAt:      windowStart.Add(2 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventCheckoutClicked,
			Surface:        "settings_self_hosted_billing_compare_prompt",
			IdempotencyKey: "org-a:checkout:2",
			CreatedAt:      windowStart.Add(3 * time.Hour),
		},
	} {
		if err := store.Record(event); err != nil {
			t.Fatalf("Record(%s) error = %v", event.IdempotencyKey, err)
		}
	}

	breakdown, err := store.FunnelDimensionBreakdown("org-a", windowStart, windowEnd, "surface")
	if err != nil {
		t.Fatalf("FunnelDimensionBreakdown() error = %v", err)
	}

	if len(breakdown) != 2 {
		t.Fatalf("len(breakdown) = %d, want 2", len(breakdown))
	}
	if breakdown[0].Key != "settings_self_hosted_billing_compare_prompt" || breakdown[0].CheckoutClicked != 2 {
		t.Fatalf("unexpected primary breakdown entry: %+v", breakdown)
	}
	if breakdown[1].Key != "settings_self_hosted_billing_plan" || breakdown[1].PricingViewed != 1 {
		t.Fatalf("unexpected secondary breakdown entry: %+v", breakdown)
	}
}
