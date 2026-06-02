package reporting

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/require"
)

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII="

func testReportLogo(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	require.NoError(t, err)
	return data
}

func TestReportBrandingEffectiveBrand(t *testing.T) {
	branding := ReportBranding{
		Entitled:        true,
		ProviderDefault: ReportBrand{DisplayName: "Provider Default"},
	}
	if got := branding.EffectiveBrand(); got == nil || got.DisplayName != "Provider Default" {
		t.Fatalf("provider default brand = %#v, want Provider Default", got)
	}

	branding.WorkspaceOverride = ReportBrand{DisplayName: "Client Override"}
	if got := branding.EffectiveBrand(); got == nil || got.DisplayName != "Client Override" {
		t.Fatalf("workspace override brand = %#v, want Client Override", got)
	}

	branding.Entitled = false
	if got := branding.EffectiveBrand(); got != nil {
		t.Fatalf("unentitled brand = %#v, want nil", got)
	}
}

func TestReportBrandingRendersPerClientPDFsAndEntitlementGate(t *testing.T) {
	logo := testReportLogo(t)
	now := time.Now().UTC()
	start := now.Add(-time.Hour)
	end := now.Add(time.Minute)

	clientA := renderBrandedClientReport(t, "client-a-node", start, end, ReportBranding{
		Entitled:        true,
		ProviderDefault: ReportBrand{DisplayName: "Provider Default", LogoData: logo, LogoFormat: "png"},
		WorkspaceOverride: ReportBrand{
			DisplayName: "Client Alpha MSP",
			LogoData:    logo,
			LogoFormat:  "png",
		},
	})
	clientB := renderBrandedClientReport(t, "client-b-node", start, end, ReportBranding{
		Entitled:        true,
		ProviderDefault: ReportBrand{DisplayName: "Provider Default", LogoData: logo, LogoFormat: "png"},
		WorkspaceOverride: ReportBrand{
			DisplayName: "Client Beta MSP",
			LogoData:    logo,
			LogoFormat:  "png",
		},
	})
	unentitled := renderBrandedClientReport(t, "client-c-node", start, end, ReportBranding{
		Entitled:        false,
		ProviderDefault: ReportBrand{DisplayName: "Provider Default", LogoData: logo, LogoFormat: "png"},
		WorkspaceOverride: ReportBrand{
			DisplayName: "Client Gamma MSP",
			LogoData:    logo,
			LogoFormat:  "png",
		},
	})

	assertPDFContainsOnlyClient(t, clientA.text, "Client Alpha MSP", "client-a-node", []string{"Client Beta MSP", "client-b-node", "Client Gamma MSP", "client-c-node", "Provider Default"})
	assertPDFContainsOnlyClient(t, clientB.text, "Client Beta MSP", "client-b-node", []string{"Client Alpha MSP", "client-a-node", "Client Gamma MSP", "client-c-node", "Provider Default"})
	if !bytes.Contains(clientA.pdf, []byte("/Subtype /Image")) || !bytes.Contains(clientB.pdf, []byte("/Subtype /Image")) {
		t.Fatal("entitled branded PDFs should embed the configured logo image")
	}

	if strings.Contains(unentitled.text, "Client Gamma MSP") || strings.Contains(unentitled.text, "Provider Default") {
		t.Fatalf("unentitled report rendered custom branding:\n%s", unentitled.text)
	}
	if !strings.Contains(unentitled.text, "PULSE") {
		t.Fatalf("unentitled report should keep built-in Pulse identity, got:\n%s", unentitled.text)
	}
	if bytes.Contains(unentitled.pdf, []byte("/Subtype /Image")) {
		t.Fatal("unentitled report embedded a custom logo image")
	}
	if strings.Contains(unentitled.text, "client-a-node") || strings.Contains(unentitled.text, "client-b-node") {
		t.Fatalf("unentitled report included another client resource:\n%s", unentitled.text)
	}
}

type renderedClientReport struct {
	pdf  []byte
	text string
}

func renderBrandedClientReport(t *testing.T, resourceID string, start, end time.Time, branding ReportBranding) renderedClientReport {
	t.Helper()

	dir := t.TempDir()
	store, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir, "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   25 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	for i := 0; i < 4; i++ {
		store.Write("node", resourceID, "cpu", float64(10+i), start.Add(time.Duration(i)*10*time.Minute))
	}
	store.Flush()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})
	req := MetricReportRequest{
		ResourceType: "node",
		ResourceID:   resourceID,
		Start:        start,
		End:          end,
		Format:       FormatPDF,
		Branding:     branding,
	}

	var pdf []byte
	var contentType string
	require.Eventually(t, func() bool {
		var genErr error
		pdf, contentType, genErr = engine.Generate(req)
		return genErr == nil && contentType == "application/pdf" && len(pdf) > 1000
	}, 2*time.Second, 25*time.Millisecond)

	return renderedClientReport{
		pdf:  pdf,
		text: extractPDFText(t, pdf),
	}
}

func assertPDFContainsOnlyClient(t *testing.T, text, brand, resource string, forbidden []string) {
	t.Helper()
	if !strings.Contains(text, brand) {
		t.Fatalf("report missing brand %q:\n%s", brand, text)
	}
	if !strings.Contains(text, resource) {
		t.Fatalf("report missing resource %q:\n%s", resource, text)
	}
	for _, value := range forbidden {
		if strings.Contains(text, value) {
			t.Fatalf("report included forbidden value %q:\n%s", value, text)
		}
	}
}
