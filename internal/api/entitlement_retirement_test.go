package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
)

func TestLicenseHTTPRetirementCopyPreservesEntitlements(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	for _, tier := range []pkglicensing.Tier{pkglicensing.TierFree, pkglicensing.TierRelay, pkglicensing.TierPro} {
		t.Run(string(tier), func(t *testing.T) {
			handler := createTestHandler(t)
			if tier != pkglicensing.TierFree {
				key, err := licensetestsupport.GenerateLicenseForTesting("fixture@example.com", tier, 24*time.Hour)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := handler.Service(context.Background()).Activate(key); err != nil {
					t.Fatal(err)
				}
			}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/license/entitlements", handler.HandleEntitlements)
			mux.HandleFunc("GET /api/license/commercial-posture", handler.HandleCommercialPosture)
			mux.HandleFunc("GET /api/license/runtime-capabilities", handler.HandleRuntimeCapabilities)
			server := httptest.NewServer(mux)
			defer server.Close()

			fixtures := make(map[string]json.RawMessage)
			for _, endpoint := range []string{"entitlements", "commercial-posture", "runtime-capabilities"} {
				response, err := server.Client().Get(server.URL + "/api/license/" + endpoint)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || response.StatusCode != http.StatusOK {
					t.Fatalf("%s: status %d, read %v", endpoint, response.StatusCode, err)
				}
				fixtures[endpoint] = body
			}
			var entitlement EntitlementPayload
			var posture CommercialPosturePayload
			var runtime RuntimeCapabilitiesPayload
			for _, decode := range []struct {
				name string
				out  any
			}{
				{"entitlements", &entitlement},
				{"commercial-posture", &posture},
				{"runtime-capabilities", &runtime},
			} {
				if err := json.Unmarshal(fixtures[decode.name], decode.out); err != nil {
					t.Fatal(err)
				}
			}
			if entitlement.Tier != string(tier) || posture.Tier != string(tier) {
				t.Fatal("marketing correction changed the active tier")
			}
			if entitlement.MaxHistoryDays != pkglicensing.TierHistoryDays[tier] || runtime.MaxHistoryDays != entitlement.MaxHistoryDays {
				t.Fatalf("history compatibility changed: entitlement %d, runtime %d", entitlement.MaxHistoryDays, runtime.MaxHistoryDays)
			}
			// Signed activation canonicalizes capability order; grants are a set.
			wantCapabilities := slices.Clone(pkglicensing.TierFeatures[tier])
			gotCapabilities := slices.Clone(entitlement.Capabilities)
			slices.Sort(wantCapabilities)
			slices.Sort(gotCapabilities)
			if !slices.Equal(gotCapabilities, wantCapabilities) {
				t.Fatalf("grants = %v, want %v", gotCapabilities, wantCapabilities)
			}
			// This is the public Community binary, not the private Pro runtime.
			// Its existing paid-runtime fence must survive the copy correction.
			if runtime.Runtime == nil || runtime.Runtime.Build != pkglicensing.RuntimeBuildCommunity {
				t.Fatalf("runtime identity = %+v, want Community", runtime.Runtime)
			}
			wantRuntimeCapabilities := slices.Clone(wantCapabilities)
			privateRuntimeCapabilities := []string{
				pkglicensing.FeatureAgentProfiles, pkglicensing.FeatureAIAlerts,
				pkglicensing.FeatureAIAutoFix, pkglicensing.FeatureAuditLogging,
				pkglicensing.FeatureKubernetesAI, pkglicensing.FeatureRBAC,
			}
			if tier == pkglicensing.TierPro {
				wantRuntimeCapabilities = slices.DeleteFunc(wantRuntimeCapabilities, func(feature string) bool {
					return slices.Contains(privateRuntimeCapabilities, feature)
				})
				blockedKeys := make([]string, 0, len(runtime.BlockedCapabilities))
				for _, block := range runtime.BlockedCapabilities {
					blockedKeys = append(blockedKeys, block.Key)
					if block.Reason != "paid_runtime_required" || block.ActionURL != pkglicensing.PulseProDownloadURL {
						t.Errorf("private-runtime recovery changed: %+v", block)
					}
				}
				slices.Sort(blockedKeys)
				slices.Sort(privateRuntimeCapabilities)
				if !slices.Equal(blockedKeys, privateRuntimeCapabilities) {
					t.Fatalf("private-runtime blocks = %v, want %v", blockedKeys, privateRuntimeCapabilities)
				}
			}
			gotRuntimeCapabilities := slices.Clone(runtime.Capabilities)
			slices.Sort(gotRuntimeCapabilities)
			if !slices.Equal(gotRuntimeCapabilities, wantRuntimeCapabilities) {
				t.Fatalf("runtime capabilities = %v, want %v", gotRuntimeCapabilities, wantRuntimeCapabilities)
			}
			if !reflect.DeepEqual(posture.UpgradeReasons, entitlement.UpgradeReasons) {
				t.Fatal("commercial projection disagrees with full entitlement reasons")
			}
			foundHistory := false
			for _, reason := range posture.UpgradeReasons {
				switch reason.Key {
				case pkglicensing.FeatureRelay, pkglicensing.FeatureMobileApp, pkglicensing.FeaturePushNotifications:
					t.Errorf("%s still markets retired %s", tier, reason.Key)
				case pkglicensing.FeatureLongTermMetrics:
					foundHistory = true
					if reason.Reason != "Upgrade to Pro for 90 days of metric history, so you can see what changed before and after an incident." {
						t.Errorf("obsolete history offer: %q", reason.Reason)
					}
				}
				if strings.Contains(reason.Reason, "Get Relay") {
					t.Errorf("retired offer: %q", reason.Reason)
				}
			}
			if foundHistory != (tier == pkglicensing.TierFree) {
				t.Error("history upgrade must only appear for a missing capability")
			}
			for _, forbidden := range []string{"licensed_email", "plan_version", "max_history_days", "capabilities"} {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(fixtures["commercial-posture"], &fields); err != nil {
					t.Fatal(err)
				}
				if _, ok := fields[forbidden]; ok {
					t.Errorf("commercial posture leaked %s", forbidden)
				}
			}
			encoded, err := json.Marshal(fixtures)
			if err != nil {
				t.Fatal(err)
			}
			// Synthetic handler output feeds the offline consuming-browser proof.
			// Never include activated license keys or real customer data here.
			t.Logf("RETIREMENT_FIXTURE:%s:%s", tier, base64.StdEncoding.EncodeToString(encoded))
		})
	}
}
