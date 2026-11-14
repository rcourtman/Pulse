package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type outboundNASProbe struct{}

func (outboundNASProbe) TestConnection(context.Context) error { return nil }
func (outboundNASProbe) Close()                               {}

type outboundVMwareProbe struct{}

func (outboundVMwareProbe) TestConnection(context.Context) (*vmware.InventorySummary, error) {
	return &vmware.InventorySummary{}, nil
}
func (outboundVMwareProbe) Close() {}

func TestApplianceTenantSavesStampPolicyAndIgnoreForgedMetadata(t *testing.T) {
	router := notificationAuthorityRouter(t)
	mtp := config.NewMultiTenantPersistence(router.config.DataPath)
	tenant, err := mtp.GetPersistence("org-a")
	if err != nil {
		t.Fatal(err)
	}
	// Instance authority is not organization membership. Prove that fence,
	// then explicitly arrange membership for this synthetic fixture only.
	denied := notificationAuthorityRequest(t, router, http.MethodGet, "/api/truenas/connections", "", "platform-admin", "org-a")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("nonmember instance operator = %d", denied.Code)
	}
	org, err := mtp.LoadOrganization("org-a")
	if err != nil {
		t.Fatal(err)
	}
	org.Members = append(org.Members, models.OrganizationMember{UserID: "platform-admin", Role: models.OrgRoleAdmin, AddedAt: time.Now()})
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"truenas", "vmware"} {
		t.Run(platform, func(t *testing.T) {
			body := `{"id":"policy-connection","name":"fixture","host":"93.184.216.34","username":"fixture","apiKey":"synthetic-key","password":"synthetic-password","outboundPolicy":""}`
			rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/"+platform+"/connections", body, "org-a-owner", "org-a")
			if rec.Code != http.StatusCreated {
				t.Fatalf("add=%d %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "outboundPolicy") {
				t.Fatal("policy metadata leaked to response")
			}
			readPolicy := func() securityutil.ConnectionOutboundPolicy {
				if platform == "truenas" {
					rows, err := tenant.LoadTrueNASConfig()
					if err != nil || len(rows) != 1 {
						t.Fatalf("NAS reload=%v %v", rows, err)
					}
					return rows[0].OutboundPolicy
				}
				rows, err := tenant.LoadVMwareConfig()
				if err != nil || len(rows) != 1 {
					t.Fatalf("VC reload=%v %v", rows, err)
				}
				return rows[0].OutboundPolicy
			}
			if got := readPolicy(); got != securityutil.ConnectionOutboundTenantLAN {
				t.Fatalf("saved policy=%q", got)
			}
			for _, forged := range []string{`null`, `""`, `"instance"`, `"tenant-public"`} {
				rec = notificationAuthorityRequest(t, router, http.MethodPut, "/api/"+platform+"/connections/policy-connection", `{"outboundPolicy":`+forged+`,"password":"********","apiKey":"********"}`, "org-a-owner", "org-a")
				if rec.Code != http.StatusOK || readPolicy() != securityutil.ConnectionOutboundTenantLAN {
					t.Fatalf("forged update=%d %s", rec.Code, rec.Body.String())
				}
			}
			// The authenticated instance operator can explicitly own this connection,
			// when also admitted to this organization. A body cannot impersonate either grant.
			rec = notificationAuthorityRequest(t, router, http.MethodPut, "/api/"+platform+"/connections/policy-connection", `{"outboundPolicy":"tenant-public"}`, "platform-admin", "org-a")
			if rec.Code != http.StatusOK || readPolicy() != "" {
				t.Fatalf("instance save=%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestApplianceProbePolicyUsesCurrentActorNotBodyOrSavedMarker(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want securityutil.ConnectionOutboundPolicy
	}{
		{"instance", context.Background(), ""},
		{"LAN tenant", securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{AllowPrivateNetworks: true}), securityutil.ConnectionOutboundTenantLAN},
		{"cloud tenant", securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{}), securityutil.ConnectionOutboundTenantPublic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A direct consuming-stage fixture, independently of the earlier
			// route validation. Both fresh and saved test/preview paths use it.
			req := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(tc.ctx)
			nas := &TrueNASHandlers{newClient: func(cfg truenas.ClientConfig) (trueNASClient, error) {
				if cfg.OutboundPolicy != tc.want {
					t.Errorf("NAS client policy=%q", cfg.OutboundPolicy)
				}
				return outboundNASProbe{}, nil
			}}
			if _, err := nas.testConnectionInstance(req, config.TrueNASInstance{OutboundPolicy: "forged", Host: "93.184.216.34"}); err != nil {
				t.Fatal(err)
			}
			vc := &VMwareHandlers{newClient: func(cfg vmware.ClientConfig) (vmwareClient, error) {
				if cfg.OutboundPolicy != tc.want {
					t.Errorf("VC client policy=%q", cfg.OutboundPolicy)
				}
				return outboundVMwareProbe{}, nil
			}}
			if _, _, err := vc.testConnectionInstance(req, config.VMwareVCenterInstance{OutboundPolicy: "forged", Host: "93.184.216.34"}); err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{"", `{"outboundPolicy":""}`, `{"outboundPolicy":"forged"}`} {
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).WithContext(tc.ctx)
				item, _, ok := decodeOptionalTrueNASInstanceRequest(httptest.NewRecorder(), r, config.TrueNASInstance{OutboundPolicy: "forged"})
				if !ok || item.OutboundPolicy != tc.want {
					t.Fatal("NAS optional decoder trusted metadata")
				}
				r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).WithContext(tc.ctx)
				vm, _, ok := decodeOptionalVMwareInstanceRequest(httptest.NewRecorder(), r, config.VMwareVCenterInstance{OutboundPolicy: "forged"})
				if !ok || vm.OutboundPolicy != tc.want {
					t.Fatal("VC optional decoder trusted metadata")
				}
			}
		})
	}
}

func TestApplianceCloudSaveRetainsPublicOnlyPolicy(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")
	t.Setenv("PULSE_PROVIDER_HOSTED", "")
	router := notificationAuthorityRouter(t)
	// Exercise the actual active-subscription route. The original 402 was
	// a valid billing fence, not a reason to weaken product admission.
	setupHostedLicenseProvider(t, pkglicensing.SubStateActive, nil)
	for _, platform := range []string{"truenas", "vmware"} {
		rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/"+platform+"/connections", `{"name":"fixture","host":"93.184.216.34","username":"fixture","apiKey":"synthetic-key","password":"synthetic-password","outboundPolicy":"tenant-lan"}`, "org-a-owner", "org-a")
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s cloud add=%d %s", platform, rec.Code, rec.Body.String())
		}
	}
	store, err := config.NewMultiTenantPersistence(router.config.DataPath).GetPersistence("org-a")
	if err != nil {
		t.Fatal(err)
	}
	nas, err := store.LoadTrueNASConfig()
	if err != nil {
		t.Fatal(err)
	}
	vc, err := store.LoadVMwareConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(nas) != 1 || len(vc) != 1 || nas[0].OutboundPolicy != securityutil.ConnectionOutboundTenantPublic || vc[0].OutboundPolicy != securityutil.ConnectionOutboundTenantPublic {
		t.Fatal("cloud save widened private access")
	}
	// Public API list remains unchanged, without a writable-looking policy field.
	for _, platform := range []string{"truenas", "vmware"} {
		rec := notificationAuthorityRequest(t, router, http.MethodGet, "/api/"+platform+"/connections", "", "org-a-owner", "org-a")
		var decoded any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &decoded) != nil || strings.Contains(rec.Body.String(), "outboundPolicy") {
			t.Fatalf("%s list=%d %s", platform, rec.Code, rec.Body.String())
		}
	}
}
