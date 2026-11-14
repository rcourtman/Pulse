package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

// Exercise both actual response producers without an appliance or a changed
// access grant. Only a typed, wrapped destination refusal gets policy copy;
// the actionable certificate/authentication/network diagnostic stays intact.
func TestTrueNASPolicyRefusalFeedbackPreservesSavedConfiguration(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTest(t, false)
	policyErr := &url.Error{Op: "Post", URL: "https://synthetic-user:synthetic-password@private.example.test/private?token=synthetic", Err: fmt.Errorf("%w: nested dial detail", securityutil.ErrTenantOutboundDestination)}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"wrapped policy", policyErr, "This TrueNAS destination requires an instance administrator."},
		{"TLS", errors.New("x509: certificate signed by unknown authority"), "x509: certificate signed by unknown authority"},
		{"authentication", errors.New("TrueNAS authentication failed"), "TrueNAS authentication failed"},
		{"network", errors.New("connection refused"), "connection refused"},
		{"text is not authority", errors.New(securityutil.ErrTenantOutboundDestination.Error()), securityutil.ErrTenantOutboundDestination.Error()},
	} {
		for _, saved := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/saved=%t", tc.name, saved), func(t *testing.T) {
				handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
				instance := config.NewTrueNASInstance()
				instance.ID = "feedback-fixture"
				instance.Name = "Saved NAS"
				instance.Host = "https://nas.example.test"
				instance.Username = "fixture"
				instance.APIKey = "synthetic-key"
				instance.UseHTTPS = true
				instance.InsecureSkipVerify = false
				instance.Fingerprint = "saved-pin"
				instance.OutboundPolicy = securityutil.ConnectionOutboundTenantLAN
				if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{instance}); err != nil {
					t.Fatal(err)
				}
				before, err := persistence.LoadTrueNASConfig()
				if err != nil {
					t.Fatal(err)
				}
				clientCalls, probeCalls := 0, 0
				handler.newClient = func(truenas.ClientConfig) (trueNASClient, error) {
					clientCalls++
					return &fakeTrueNASClient{testConnection: func(context.Context) error { probeCalls++; return tc.err }}, nil
				}
				rec := httptest.NewRecorder()
				if saved {
					handler.HandleTestSavedConnection(rec, httptest.NewRequest(http.MethodPost, "/api/truenas/connections/feedback-fixture/test", nil))
				} else {
					body := marshalTrueNASRequest(t, map[string]any{"host": instance.Host, "apiKey": instance.APIKey, "username": instance.Username})
					handler.HandleTestConnection(rec, httptest.NewRequest(http.MethodPost, "/api/truenas/connections/test", bytes.NewReader(body)))
				}
				assertTrueNASGuidanceError(t, rec, http.StatusBadRequest, "truenas_connection_failed", "Failed to connect to TrueNAS")
				var response struct {
					Details map[string]string `json:"details"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Details["error"] != tc.want {
					t.Fatalf("details=%q want=%q", response.Details["error"], tc.want)
				}
				after, err := persistence.LoadTrueNASConfig()
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("failed test changed saved configuration: %v", err)
				}
				if clientCalls != 1 || probeCalls != 1 {
					t.Fatalf("clients=%d probes=%d", clientCalls, probeCalls)
				}
			})
		}
	}
}
