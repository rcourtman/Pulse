package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tenantIdentityTestAlert() *alerts.Alert {
	return &alerts.Alert{
		ID:           "alert-1",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceName: "web-01",
		Node:         "pve1",
		Message:      "CPU usage above threshold",
		Value:        95.0,
		Threshold:    90.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
	}
}

func TestPrepareWebhookDataNoTenantIdentityByDefault(t *testing.T) {
	t.Setenv("PULSE_TENANT_ID", "")
	t.Setenv("PULSE_TENANT_NAME", "")
	nm := NewNotificationManager("http://pulse.local")

	data := nm.prepareWebhookData(tenantIdentityTestAlert(), nil)
	assert.Empty(t, data.TenantID)
	assert.Empty(t, data.TenantName)
}

func TestPrepareWebhookDataTenantIdentityFromEnv(t *testing.T) {
	t.Setenv("PULSE_TENANT_ID", "client-acme")
	t.Setenv("PULSE_TENANT_NAME", "Acme Corp")
	nm := NewNotificationManager("http://pulse.local")

	data := nm.prepareWebhookData(tenantIdentityTestAlert(), nil)
	assert.Equal(t, "client-acme", data.TenantID)
	assert.Equal(t, "Acme Corp", data.TenantName)
}

func TestPrepareWebhookDataTenantNameFallsBackToID(t *testing.T) {
	t.Setenv("PULSE_TENANT_ID", "client-acme")
	t.Setenv("PULSE_TENANT_NAME", "")
	nm := NewNotificationManager("http://pulse.local")

	data := nm.prepareWebhookData(tenantIdentityTestAlert(), nil)
	assert.Equal(t, "client-acme", data.TenantID)
	assert.Equal(t, "client-acme", data.TenantName)
}

func TestTenantIdentityResolverOverridesEnv(t *testing.T) {
	t.Setenv("PULSE_TENANT_ID", "env-tenant")
	t.Setenv("PULSE_TENANT_NAME", "Env Tenant")
	nm := NewNotificationManager("http://pulse.local")
	nm.SetTenantIdentityResolver(func() (string, string) {
		return "org-beta", "Beta GmbH"
	})

	data := nm.prepareWebhookData(tenantIdentityTestAlert(), nil)
	assert.Equal(t, "org-beta", data.TenantID)
	assert.Equal(t, "Beta GmbH", data.TenantName)
}

func TestTenantIdentityResolverEmptyIDKeepsEnvDefaults(t *testing.T) {
	t.Setenv("PULSE_TENANT_ID", "env-tenant")
	t.Setenv("PULSE_TENANT_NAME", "Env Tenant")
	nm := NewNotificationManager("http://pulse.local")
	nm.SetTenantIdentityResolver(func() (string, string) {
		return "", "ignored"
	})

	data := nm.prepareWebhookData(tenantIdentityTestAlert(), nil)
	assert.Equal(t, "env-tenant", data.TenantID)
	assert.Equal(t, "Env Tenant", data.TenantName)
}

// The generic service template must emit a tenant block when tenant identity
// is configured and omit it entirely when it is not, staying valid JSON in
// both shapes.
func TestGenericTemplateTenantBlock(t *testing.T) {
	var genericTemplate string
	for _, tmpl := range GetWebhookTemplates() {
		if tmpl.Service == "generic" {
			genericTemplate = tmpl.PayloadTemplate
			break
		}
	}
	require.NotEmpty(t, genericTemplate)

	t.Setenv("PULSE_TENANT_ID", "client-acme")
	t.Setenv("PULSE_TENANT_NAME", "Acme Corp")
	nm := NewNotificationManager("http://pulse.local")

	data := nm.prepareWebhookData(tenantIdentityTestAlert(), nil)
	payload, err := nm.generatePayloadFromTemplateWithService(genericTemplate, data, "generic")
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	tenant, ok := decoded["tenant"].(map[string]interface{})
	require.True(t, ok, "tenant block missing from generic payload: %s", payload)
	assert.Equal(t, "client-acme", tenant["id"])
	assert.Equal(t, "Acme Corp", tenant["name"])

	// Without tenant identity the block is omitted.
	data.TenantID = ""
	data.TenantName = ""
	payload, err = nm.generatePayloadFromTemplateWithService(genericTemplate, data, "generic")
	require.NoError(t, err)
	decoded = map[string]interface{}{}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	_, hasTenant := decoded["tenant"]
	assert.False(t, hasTenant)
}
