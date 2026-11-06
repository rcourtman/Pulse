package api

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDecodeAdminJSONObjectEmptyBodyIsEOF(t *testing.T) {
	for _, body := range []string{"", " \n\t"} {
		provider := config.SSOProvider{Name: "retained"}
		err := decodeAdminJSONObject(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)), &provider, 1024)
		require.ErrorIs(t, err, io.EOF)
		require.Equal(t, "retained", provider.Name)
	}
	request := httptest.NewRequest("POST", "/", nil)
	request.Body = nil
	var provider config.SSOProvider
	require.ErrorIs(t, decodeAdminJSONObject(httptest.NewRecorder(), request, &provider, 1024), io.EOF)
}

func TestDecodeAdminJSONObject(t *testing.T) {
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{"unique", `{"name":"SSO","allowedGroups":["ops"]}`, true},
		{"singleton aliases", `{"NAME":"SSO","allowedGroup\u0073":[],"OIDC":{"CLIENTID":"pulse"}}`, true},
		{"unicode singleton alias", `{"allowedGroupſ":["ops"]}`, true},
		{"explicit clears", `{"allowedGroups":[],"allowedDomains":[],"allowedEmails":[],"groupRoleMappings":{}}`, true},
		{"case sensitive group names", `{"groupRoleMappings":{"Ops":"admin","ops":"viewer"}}`, true},
		{"unknown fields", `{"future":{"allowedGroups":[],"AllowedGroups":["ignored"]},"runtimeManaged":true}`, true},
		{"exact duplicate", `{"allowedGroups":["ops"],"allowedGroups":[]}`, false},
		{"escaped duplicate", `{"allowedGroups":["ops"],"allowedGroup\u0073":[]}`, false},
		{"case folded duplicate", `{"allowedGroups":["ops"],"ALLOWEDGROUPS":[]}`, false},
		{"unicode folded duplicate", `{"allowedGroups":["ops"],"allowedGroupſ":[]}`, false},
		{"nested duplicate", `{"oidc":{"clientSecret":"one","CLIENTSECRET":"two"}}`, false},
		{"merged nested objects", `{"oidc":{"clientId":"one"},"OIDC":{"clientSecret":"two"}}`, false},
		{"group duplicate", `{"groupRoleMappings":{"ops":"viewer","ops":"admin"}}`, false},
		{"escaped group duplicate", `{"groupRoleMappings":{"ops":"viewer","op\u0073":"admin"}}`, false},
		{"wrong field type", `{"allowedGroups":"ops"}`, false},
		{"null root", `null`, false},
		{"array root", `[]`, false},
		{"empty", ``, false},
		{"non-JSON whitespace", "\u00a0", false},
		{"non-JSON leading whitespace", "\u00a0{}", false},
		{"non-JSON trailing whitespace", "{}\u00a0", false},
		{"incomplete", `{"name":"SSO"`, false},
		{"second object", `{"name":"SSO"}{}`, false},
		{"trailing junk", `{"name":"SSO"}x`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider config.SSOProvider
			err := decodeAdminJSONObject(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(tt.body)), &provider, 1024)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestDecodeAdminJSONObjectCompleteBound(t *testing.T) {
	body := `{"name":"SSO"}`
	for _, tt := range []struct {
		name string
		body string
		ok   bool
	}{
		{"exact limit", body, true},
		{"whitespace within limit", body + " ", true},
		{"whitespace overflow", body + "  ", false},
		{"unread second object", body + " {}", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var provider config.SSOProvider
			err := decodeAdminJSONObject(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(tt.body)), &provider, int64(len(body)+1))
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
