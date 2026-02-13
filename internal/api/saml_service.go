package api

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// SAMLService manages SAML Service Provider functionality for a single provider
type SAMLService struct {
	mu          sync.RWMutex
	providerID  string
	config      *config.SAMLProviderConfig
	sp          *saml.ServiceProvider
	idpMetadata *saml.EntityDescriptor
	httpClient  *http.Client
	baseURL     string
	lastRefresh time.Time
}

// SAMLAuthResult contains the result of a successful SAML authentication
type SAMLAuthResult struct {
	Username   string
	Email      string
	Groups     []string
	FirstName  string
	LastName   string
	NameID     string
	SessionIdx string
	Attributes map[string][]string
}

// NewSAMLService creates a new SAML service for a provider
func NewSAMLService(ctx context.Context, providerID string, cfg *config.SAMLProviderConfig, baseURL string) (*SAMLService, error) {
	if cfg == nil {
		return nil, errors.New("saml configuration is nil")
	}

	service := &SAMLService{
		providerID: providerID,
		config:     cfg,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: newSAMLHTTPClient(),
	}

	// Load IdP metadata
	if err := service.loadIDPMetadata(ctx); err != nil {
		return nil, fmt.Errorf("failed to load idp metadata: %w", err)
	}

	// Initialize Service Provider
	if err := service.initServiceProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize service provider: %w", err)
	}

	return service, nil
}

func newSAMLHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// loadIDPMetadata loads Identity Provider metadata from URL or XML
func (s *SAMLService) loadIDPMetadata(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metadata *saml.EntityDescriptor
	var err error

	if s.config.IDPMetadataURL != "" {
		metadata, err = s.fetchIDPMetadataFromURL(ctx, s.config.IDPMetadataURL)
		if err != nil {
			return fmt.Errorf("failed to fetch idp metadata from url: %w", err)
		}
	} else if s.config.IDPMetadataXML != "" {
		metadata, err = parseIDPMetadataXML([]byte(s.config.IDPMetadataXML))
		if err != nil {
			return fmt.Errorf("failed to parse idp metadata xml: %w", err)
		}
	} else {
		// Build metadata from manual configuration
		metadata, err = s.buildManualMetadata()
		if err != nil {
			return fmt.Errorf("failed to build manual metadata: %w", err)
		}
	}

	s.idpMetadata = metadata
	s.lastRefresh = time.Now()

	log.Info().
		Str("provider_id", s.providerID).
		Str("entity_id", metadata.EntityID).
		Msg("Loaded SAML IdP metadata")

	return nil
}

func (s *SAMLService) fetchIDPMetadataFromURL(ctx context.Context, metadataURL string) (*saml.EntityDescriptor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, err
	}

	return parseIDPMetadataXML(body)
}

func parseIDPMetadataXML(data []byte) (*saml.EntityDescriptor, error) {
	var metadata saml.EntityDescriptor
	if err := xml.Unmarshal(data, &metadata); err != nil {
		// Try parsing as EntityDescriptor wrapped in EntitiesDescriptor
		var entities saml.EntitiesDescriptor
		if err2 := xml.Unmarshal(data, &entities); err2 != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
		if len(entities.EntityDescriptors) == 0 {
			return nil, errors.New("no entity descriptors found in metadata")
		}
		metadata = entities.EntityDescriptors[0]
	}
	return &metadata, nil
}

func (s *SAMLService) buildManualMetadata() (*saml.EntityDescriptor, error) {
	if s.config.IDPSSOURL == "" {
		return nil, errors.New("idp sso url is required for manual configuration")
	}

	ssoURL, err := url.Parse(s.config.IDPSSOURL)
	if err != nil {
		return nil, fmt.Errorf("invalid idp sso url: %w", err)
	}

	entityID := s.config.IDPEntityID
	if entityID == "" {
		entityID = s.config.IDPIssuer
	}
	if entityID == "" {
		entityID = s.config.IDPSSOURL
	}

	metadata := &saml.EntityDescriptor{
		EntityID: entityID,
		IDPSSODescriptors: []saml.IDPSSODescriptor{
			{
				SSODescriptor: saml.SSODescriptor{
					RoleDescriptor: saml.RoleDescriptor{
						ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
					},
				},
				SingleSignOnServices: []saml.Endpoint{
					{
						Binding:  saml.HTTPRedirectBinding,
						Location: ssoURL.String(),
					},
					{
						Binding:  saml.HTTPPostBinding,
						Location: ssoURL.String(),
					},
				},
			},
		},
	}

	// Add SLO endpoint if configured
	if s.config.IDPSLOURL != "" {
		sloURL, err := url.Parse(s.config.IDPSLOURL)
		if err == nil {
			metadata.IDPSSODescriptors[0].SingleLogoutServices = []saml.Endpoint{
				{
					Binding:  saml.HTTPRedirectBinding,
					Location: sloURL.String(),
				},
			}
		}
	}

	// Add IdP certificate if provided
	if err := s.addIDPCertificate(metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

func (s *SAMLService) addIDPCertificate(metadata *saml.EntityDescriptor) error {
	var certData []byte
	var err error

	if s.config.IDPCertFile != "" {
		certData, err = os.ReadFile(s.config.IDPCertFile)
		if err != nil {
			return fmt.Errorf("failed to read idp certificate file: %w", err)
		}
	} else if s.config.IDPCertificate != "" {
		certData = []byte(s.config.IDPCertificate)
	} else {
		return nil // No certificate provided
	}

	// Parse PEM certificate
	block, _ := pem.Decode(certData)
	if block == nil {
		return errors.New("failed to decode idp certificate pem")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse idp certificate: %w", err)
	}

	if len(metadata.IDPSSODescriptors) > 0 {
		metadata.IDPSSODescriptors[0].KeyDescriptors = []saml.KeyDescriptor{
			{
				Use: "signing",
				KeyInfo: saml.KeyInfo{
					X509Data: saml.X509Data{
						X509Certificates: []saml.X509Certificate{
							{Data: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))},
						},
					},
				},
			},
		}
	}

	return nil
}

func (s *SAMLService) initServiceProvider() error {
	// Build SP Entity ID
	spEntityID := s.config.SPEntityID
	if spEntityID == "" {
		spEntityID = fmt.Sprintf("%s/saml/%s", s.baseURL, s.providerID)
	}

	// Build ACS URL
	acsPath := s.config.SPACSPath
	if acsPath == "" {
		acsPath = fmt.Sprintf("/api/saml/%s/acs", s.providerID)
	}
	acsURL, err := url.Parse(s.baseURL + acsPath)
	if err != nil {
		return fmt.Errorf("failed to parse acs url: %w", err)
	}

	// Build Metadata URL
	metadataPath := s.config.SPMetadataPath
	if metadataPath == "" {
		metadataPath = fmt.Sprintf("/api/saml/%s/metadata", s.providerID)
	}
	metadataURL, err := url.Parse(s.baseURL + metadataPath)
	if err != nil {
		return fmt.Errorf("failed to parse metadata url: %w", err)
	}

	forceAuthn := s.config.ForceAuthn

	sp := saml.ServiceProvider{
		EntityID:          spEntityID,
		AcsURL:            *acsURL,
		MetadataURL:       *metadataURL,
		IDPMetadata:       s.idpMetadata,
		AllowIDPInitiated: s.config.AllowIDPInitiated,
		ForceAuthn:        &forceAuthn,
	}

	// Set SLO URL if the IdP supports it
	if len(s.idpMetadata.IDPSSODescriptors) > 0 &&
		len(s.idpMetadata.IDPSSODescriptors[0].SingleLogoutServices) > 0 {
		sloURL, err := url.Parse(s.baseURL + fmt.Sprintf("/api/saml/%s/slo", s.providerID))
		if err == nil {
			sp.SloURL = *sloURL
		}
	}

	// Load SP certificate and key if signing is enabled
	if s.config.SignRequests {
		cert, key, err := s.loadSPCredentials()
		if err != nil {
			return fmt.Errorf("failed to load sp credentials: %w", err)
		}
		sp.Key = key
		sp.Certificate = cert
	}

	s.sp = &sp

	log.Info().
		Str("provider_id", s.providerID).
		Str("entity_id", spEntityID).
		Str("acs_url", acsURL.String()).
		Bool("sign_requests", s.config.SignRequests).
		Msg("Initialized SAML Service Provider")

	return nil
}

func (s *SAMLService) loadSPCredentials() (*x509.Certificate, *rsa.PrivateKey, error) {
	var certData, keyData []byte
	var err error

	// Load certificate
	if s.config.SPCertFile != "" {
		certData, err = os.ReadFile(s.config.SPCertFile)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read sp certificate file: %w", err)
		}
	} else if s.config.SPCertificate != "" {
		certData = []byte(s.config.SPCertificate)
	} else {
		return nil, nil, errors.New("sp certificate is required for signing")
	}

	// Load private key
	if s.config.SPKeyFile != "" {
		keyData, err = os.ReadFile(s.config.SPKeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read sp private key file: %w", err)
		}
	} else if s.config.SPPrivateKey != "" {
		keyData = []byte(s.config.SPPrivateKey)
	} else {
		return nil, nil, errors.New("sp private key is required for signing")
	}

	// Parse certificate
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return nil, nil, errors.New("failed to decode sp certificate pem")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse sp certificate: %w", err)
	}

	// Parse private key
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return nil, nil, errors.New("failed to decode sp private key pem")
	}

	var key *rsa.PrivateKey
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		parsedKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse pkcs8 private key: %w", err)
		}
		var ok bool
		key, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, errors.New("sp private key is not rsa")
		}
	default:
		return nil, nil, fmt.Errorf("unsupported private key type: %s", keyBlock.Type)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse sp private key: %w", err)
	}

	return cert, key, nil
}

// MakeAuthRequest creates a SAML AuthnRequest and returns the redirect URL
func (s *SAMLService) MakeAuthRequest(relayState string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return "", errors.New("service provider not initialized")
	}

	if relayState == "" {
		relayState = "/"
	}

	// Use the simple redirect method
	redirectURL, err := s.sp.MakeRedirectAuthenticationRequest(relayState)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	log.Debug().
		Str("provider_id", s.providerID).
		Str("redirect_url", redirectURL.String()).
		Msg("Created SAML AuthnRequest")

	return redirectURL.String(), nil
}

// ProcessResponse processes a SAML response and extracts user information
func (s *SAMLService) ProcessResponse(r *http.Request) (*SAMLAuthResult, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return nil, "", errors.New("service provider not initialized")
	}

	// Parse the form to get SAMLResponse and RelayState
	if err := r.ParseForm(); err != nil {
		return nil, "", fmt.Errorf("failed to parse form: %w", err)
	}

	relayState := r.FormValue("RelayState")

	// Allow IdP-initiated flow
	possibleRequestIDs := []string{}
	if s.sp.AllowIDPInitiated {
		possibleRequestIDs = append(possibleRequestIDs, "")
	}

	// Parse and validate the SAML assertion
	assertion, err := s.sp.ParseResponse(r, possibleRequestIDs)
	if err != nil {
		return nil, relayState, fmt.Errorf("failed to validate saml response: %w", err)
	}

	// Extract user information from assertion
	result := &SAMLAuthResult{
		Attributes: make(map[string][]string),
	}

	// Get NameID
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		result.NameID = assertion.Subject.NameID.Value
	}

	// Get session index from AuthnStatement
	for _, authnStatement := range assertion.AuthnStatements {
		if authnStatement.SessionIndex != "" {
			result.SessionIdx = authnStatement.SessionIndex
			break
		}
	}

	// Extract attributes
	for _, statement := range assertion.AttributeStatements {
		for _, attr := range statement.Attributes {
			values := make([]string, 0, len(attr.Values))
			for _, v := range attr.Values {
				values = append(values, v.Value)
			}
			result.Attributes[attr.Name] = values

			// Also try FriendlyName
			if attr.FriendlyName != "" {
				result.Attributes[attr.FriendlyName] = values
			}
		}
	}

	// Extract specific attributes based on configuration
	result.Username = s.extractAttribute(result.Attributes, s.config.UsernameAttr, result.NameID)
	result.Email = s.extractAttribute(result.Attributes, s.config.EmailAttr, "")
	result.FirstName = s.extractAttribute(result.Attributes, s.config.FirstNameAttr, "")
	result.LastName = s.extractAttribute(result.Attributes, s.config.LastNameAttr, "")

	// Extract groups
	if s.config.GroupsAttr != "" {
		if groups, ok := result.Attributes[s.config.GroupsAttr]; ok {
			result.Groups = groups
		}
	}

	log.Info().
		Str("provider_id", s.providerID).
		Str("username", result.Username).
		Str("email", result.Email).
		Int("groups", len(result.Groups)).
		Msg("Processed SAML assertion")

	return result, relayState, nil
}

func (s *SAMLService) extractAttribute(attrs map[string][]string, attrName, defaultValue string) string {
	if attrName == "" {
		return defaultValue
	}
	if vals, ok := attrs[attrName]; ok && len(vals) > 0 {
		return vals[0]
	}
	return defaultValue
}

// GetMetadata returns the SP metadata XML
func (s *SAMLService) GetMetadata() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return nil, errors.New("service provider not initialized")
	}

	metadata := s.sp.Metadata()
	return xml.MarshalIndent(metadata, "", "  ")
}

// MakeLogoutRequest creates a SAML LogoutRequest for SLO
func (s *SAMLService) MakeLogoutRequest(nameID, sessionIdx string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return "", errors.New("service provider not initialized")
	}

	// Check if IdP supports SLO
	if len(s.idpMetadata.IDPSSODescriptors) == 0 ||
		len(s.idpMetadata.IDPSSODescriptors[0].SingleLogoutServices) == 0 {
		return "", errors.New("idp does not support single logout")
	}

	sloService := s.idpMetadata.IDPSSODescriptors[0].SingleLogoutServices[0]

	req, err := s.sp.MakeLogoutRequest(sloService.Location, nameID)
	if err != nil {
		return "", fmt.Errorf("failed to create logout request: %w", err)
	}

	// Build redirect URL
	redirectURL := req.Redirect("")

	return redirectURL.String(), nil
}

// RefreshMetadata reloads IdP metadata (useful for key rotation)
func (s *SAMLService) RefreshMetadata(ctx context.Context) error {
	if s.config.IDPMetadataURL == "" {
		return errors.New("cannot refresh metadata without url")
	}

	if err := s.loadIDPMetadata(ctx); err != nil {
		return fmt.Errorf("load idp metadata: %w", err)
	}

	// Reinitialize SP with new metadata
	if err := s.initServiceProvider(); err != nil {
		return fmt.Errorf("initialize service provider: %w", err)
	}
	return nil
}

// ProviderID returns the provider identifier
func (s *SAMLService) ProviderID() string {
	return s.providerID
}

// GetSPEntityID returns the Service Provider Entity ID
func (s *SAMLService) GetSPEntityID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return ""
	}
	return s.sp.EntityID
}

// GetIDPEntityID returns the Identity Provider Entity ID
func (s *SAMLService) GetIDPEntityID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.idpMetadata == nil {
		return ""
	}
	return s.idpMetadata.EntityID
}
