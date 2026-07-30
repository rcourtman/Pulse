package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestRuntimeBrandingResponseRequiresEntitlement(t *testing.T) {
	settings := &config.ReportBrandSettings{
		DisplayName: "Acme Operations",
		LogoBase64:  base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n")),
		LogoFormat:  "png",
	}

	got := runtimeBrandingResponse(settings, false)
	if got.Enabled || got.DisplayName != "" || got.LogoDataURL != "" {
		t.Fatalf("unentitled branding leaked into runtime response: %+v", got)
	}
}

func TestRuntimeBrandingResponseBuildsCanonicalPresentationPayload(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	settings := &config.ReportBrandSettings{
		DisplayName: "  Acme Operations  ",
		LogoBase64:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	}

	got := runtimeBrandingResponse(settings, true)
	if !got.Enabled {
		t.Fatal("expected configured entitled branding to be enabled")
	}
	if got.DisplayName != "Acme Operations" {
		t.Fatalf("displayName = %q, want trimmed brand", got.DisplayName)
	}
	if !strings.HasPrefix(got.LogoDataURL, "data:image/png;base64,") {
		t.Fatalf("logoDataUrl = %q, want canonical PNG data URL", got.LogoDataURL)
	}
}

func TestRuntimeBrandingResponseKeepsNameWhenLogoIsNotAnImage(t *testing.T) {
	settings := &config.ReportBrandSettings{
		DisplayName: "Acme",
		LogoBase64:  base64.StdEncoding.EncodeToString([]byte("not an image")),
	}

	got := runtimeBrandingResponse(settings, true)
	if !got.Enabled || got.DisplayName != "Acme" || got.LogoDataURL != "" {
		t.Fatalf("unexpected name-only runtime branding: %+v", got)
	}
}

func TestRuntimeBrandingResponseRejectsMismatchedImageFormat(t *testing.T) {
	settings := &config.ReportBrandSettings{
		DisplayName: "Acme",
		LogoBase64:  base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n")),
		LogoFormat:  "gif",
	}

	got := runtimeBrandingResponse(settings, true)
	if !got.Enabled || got.DisplayName != "Acme" || got.LogoDataURL != "" {
		t.Fatalf("mismatched image format must be omitted: %+v", got)
	}
}

func TestHandleGetRuntimeBrandingRejectsUnsupportedMethod(t *testing.T) {
	handler := &SystemSettingsHandler{}
	request := httptest.NewRequest(http.MethodPost, "/api/runtime/branding", nil)
	response := httptest.NewRecorder()

	handler.HandleGetRuntimeBranding(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleGetRuntimeBrandingReturnsPersistedBrandForEntitledRuntime(t *testing.T) {
	service := pkglicensing.NewService()
	service.SetCurrentForTesting(&pkglicensing.License{
		Claims: pkglicensing.Claims{
			LicenseID: "lic_runtime_branding",
			Email:     "brand@example.test",
			Tier:      pkglicensing.TierEnterprise,
		},
		ValidatedAt: time.Now(),
	})
	SetLicenseServiceProvider(reportBrandLicenseProvider{service: service})
	t.Cleanup(func() { SetLicenseServiceProvider(nil) })

	persistence := config.NewConfigPersistence(t.TempDir())
	settings := config.DefaultSystemSettings()
	settings.ReportBranding = &config.ReportBrandSettings{
		DisplayName: "Acme Operations",
		LogoBase64:  base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n")),
		LogoFormat:  "png",
	}
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatalf("save system settings: %v", err)
	}

	handler := &SystemSettingsHandler{persistence: persistence}
	request := httptest.NewRequest(http.MethodGet, "/api/runtime/branding", nil)
	response := httptest.NewRecorder()

	handler.HandleGetRuntimeBranding(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var got RuntimeBrandingResponse
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Enabled || got.DisplayName != "Acme Operations" ||
		!strings.HasPrefix(got.LogoDataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected runtime branding response: %+v", got)
	}
}
