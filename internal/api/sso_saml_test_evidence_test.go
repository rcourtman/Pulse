package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func samlDiagnosticMetadata(entity, descriptor string) string {
	return `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="` + entity + `">` + descriptor + `</EntityDescriptor>`
}

func samlDiagnosticDescriptor(binding, endpoint, keys string) string {
	return `<IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">` + keys +
		`<SingleSignOnService Binding="` + binding + `" Location="` + endpoint + `"/></IDPSSODescriptor>`
}

func TestSAMLConnectionRequiresMetadataEvidence(t *testing.T) {
	for _, tt := range []struct{ name, entity, descriptor string }{
		{"empty metadata", "", ""},
		{"no IdP descriptor", "urn:synthetic:idp", ""},
		{"SP only", "urn:synthetic:idp", `<SPSSODescriptor/>`},
		{"missing identity", "", samlDiagnosticDescriptor(saml.HTTPRedirectBinding, "https://idp.example.test/sso", "")},
		{"blank identity", " &#x9;&#x2003;", samlDiagnosticDescriptor(saml.HTTPRedirectBinding, "https://idp.example.test/sso", "")},
		{"missing endpoint", "urn:synthetic:idp", samlDiagnosticDescriptor(saml.HTTPRedirectBinding, "", "")},
		{"no endpoints", "urn:synthetic:idp", `<IDPSSODescriptor/>`},
		{"unsupported binding", "urn:synthetic:idp", samlDiagnosticDescriptor("urn:unsupported:binding", "https://idp.example.test/sso", "")},
		{"relative endpoint", "urn:synthetic:idp", samlDiagnosticDescriptor(saml.HTTPRedirectBinding, "/sso", "")},
		{"unsafe endpoint", "urn:synthetic:idp", samlDiagnosticDescriptor(saml.HTTPPostBinding, "javascript:SYNTHETIC_PRIVATE", "")},
		{"credentialled endpoint", "urn:synthetic:idp", samlDiagnosticDescriptor(saml.HTTPPostBinding, "https://synthetic:SYNTHETIC_PRIVATE@idp.example.test/sso", "")},
		{"blank endpoint", "urn:synthetic:idp", samlDiagnosticDescriptor(saml.HTTPPostBinding, " &#x9;", "")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := (&Router{}).testSAMLConnection(t.Context(), &SAMLTestConfig{
				IDPMetadataXML: samlDiagnosticMetadata(tt.entity, tt.descriptor),
				// A manual fallback must not conceal unusable selected metadata.
				IDPSSOURL: "https://manual.example.test/sso",
			})
			require.False(t, result.Success)
			require.Equal(t, "metadata_incomplete", result.Error)
			require.Nil(t, result.Details, "incomplete evidence must not be presented as validated details")
			require.Contains(t, result.Message, "test again")
			require.Contains(t, result.Message, "Sign-in has not been tested.")
			require.NotContains(t, result.Message, "SYNTHETIC_PRIVATE")
		})
	}
}

// Certificates remain diagnostic information, not a new trust or sign-in gate.
// An actual sign-in still verifies the assertion through the existing service.
func TestSAMLConnectionLabelsPartialEvidence(t *testing.T) {
	const endpoint = "https://idp.example.test/Case/Sso?tenant=synthetic"
	for _, binding := range []string{saml.HTTPRedirectBinding, saml.HTTPPostBinding} {
		for _, wrapper := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wrapped=%t", binding, wrapper), func(t *testing.T) {
				xml := samlDiagnosticMetadata("urn:Synthetic:IdP", samlDiagnosticDescriptor(binding, endpoint, ""))
				if wrapper {
					xml = `<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">` + xml + `</EntitiesDescriptor>`
				}
				result := (&Router{}).testSAMLConnection(t.Context(), &SAMLTestConfig{IDPMetadataXML: xml})
				require.True(t, result.Success)
				require.Empty(t, result.Error)
				require.Equal(t, "SAML metadata includes an IdP identity and an SSO URL. Sign-in has not been tested.", result.Message)
				require.Equal(t, "urn:Synthetic:IdP", result.Details.EntityID)
				require.Equal(t, endpoint, result.Details.SSOURL, "do not rewrite opaque identity or endpoint path/query")
				require.Empty(t, result.Details.Certificates)
			})
		}
	}
	for _, cert := range []string{"", "synthetic-not-a-certificate"} {
		t.Run("manual/"+cert, func(t *testing.T) {
			result := (&Router{}).testSAMLConnection(t.Context(), &SAMLTestConfig{IDPSSOURL: endpoint, IDPCertificate: cert})
			require.True(t, result.Success)
			require.Equal(t, endpoint, result.Details.SSOURL)
			require.Equal(t, "Only the SAML SSO URL format was checked. Certificates and sign-in have not been tested.", result.Message)
			require.Empty(t, result.Details.Certificates, "manual certificate remains untested, not invented evidence")
		})
	}
}

func TestSAMLConnectionPreservesCertificateInformation(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	before, after := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic IdP"}, NotBefore: before, NotAfter: after, KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	keys := `<KeyDescriptor use="signing"><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>` + base64.StdEncoding.EncodeToString(der) + `</X509Certificate></X509Data></KeyInfo></KeyDescriptor>`
	result := (&Router{}).testSAMLConnection(t.Context(), &SAMLTestConfig{IDPMetadataXML: samlDiagnosticMetadata("urn:synthetic:idp", samlDiagnosticDescriptor(saml.HTTPRedirectBinding, "https://idp.example.test/sso", keys))})
	require.True(t, result.Success, "certificate expiry is reported, not a new diagnostic gate")
	require.Contains(t, result.Message, "Sign-in has not been tested.")
	require.Len(t, result.Details.Certificates, 1)
	require.Equal(t, "CN=Synthetic IdP", result.Details.Certificates[0].Subject)
	require.Equal(t, before, result.Details.Certificates[0].NotBefore)
	require.Equal(t, after, result.Details.Certificates[0].NotAfter)
	require.True(t, result.Details.Certificates[0].IsExpired)

	// Manual malformed certificates still fail in actual service setup; the
	// format-only diagnostic must neither claim nor attempt that validation.
	bad := &SAMLTestConfig{IDPSSOURL: "https://idp.example.test/sso", IDPCertificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")}))}
	require.True(t, (&Router{}).testSAMLConnection(t.Context(), bad).Success)
	_, err = NewSAMLService(t.Context(), "synthetic", &config.SAMLProviderConfig{IDPSSOURL: bad.IDPSSOURL, IDPCertificate: bad.IDPCertificate}, "https://pulse.example.test")
	require.Error(t, err, "the actual service must still refuse malformed manual certificate data")
}

func TestSAMLConnectionKeepsFailureCategories(t *testing.T) {
	for _, tt := range []struct {
		name string
		cfg  *SAMLTestConfig
		code string
	}{
		{"missing config", nil, "missing_config"},
		{"missing metadata", &SAMLTestConfig{}, "missing_metadata"},
		{"invalid metadata URL", &SAMLTestConfig{IDPMetadataURL: "javascript:synthetic"}, "invalid_url"},
		{"invalid XML", &SAMLTestConfig{IDPMetadataXML: "not XML"}, "parse_failed"},
		{"invalid manual URL", &SAMLTestConfig{IDPSSOURL: "javascript:synthetic"}, "invalid_url"},
		{"invalid SLO URL", &SAMLTestConfig{IDPSSOURL: "https://idp.example.test/sso", IDPSLOURL: "javascript:synthetic"}, "invalid_url"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := (&Router{}).testSAMLConnection(t.Context(), tt.cfg)
			require.False(t, response.Success)
			require.Equal(t, tt.code, response.Error)
			require.Nil(t, response.Details)
		})
	}
}

func TestSAMLConnectionKeepsMetadataTLSVerification(t *testing.T) {
	var requests atomic.Int32
	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(testSAMLMetadata))
	}))
	defer server.Close()
	result := (&Router{}).testSAMLConnection(t.Context(), &SAMLTestConfig{IDPMetadataURL: server.URL})
	require.False(t, result.Success)
	require.Equal(t, "fetch_failed", result.Error)
	require.Nil(t, result.Details)
	require.Zero(t, requests.Load(), "an untrusted TLS certificate must be refused before the metadata handler")
}
