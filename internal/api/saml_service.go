package api

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog/log"
	dsig "github.com/russellhaering/goxmldsig"
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

	// authnRequests holds the SP-initiated AuthnRequests still waiting for
	// the IdP's Response, each bound to the browser that started the login.
	authnRequests samlRequestStore
	// logoutRequests holds the SP-initiated LogoutRequests still waiting for
	// the IdP's LogoutResponse.
	logoutRequests samlRequestStore
}

const (
	// samlRequestTTL bounds how long the IdP round trip of an SP-initiated
	// login or logout may take, matching the OIDC login state lifetime.
	samlRequestTTL        = 10 * time.Minute
	maxSAMLRequestEntries = 1024
	// samlLogoutResponseMaxBytes matches crewjam/saml's inflate limit for the
	// HTTP-Redirect binding.
	samlLogoutResponseMaxBytes = 10 << 20
)

// errSAMLLogoutResponseUnbound marks a LogoutResponse that verified but does
// not answer an outstanding LogoutRequest this SP issued for the presenting
// session.
var errSAMLLogoutResponseUnbound = errors.New("logout response does not answer an outstanding logout request for this session")

// errSAMLResponseUnbound marks an ACS Response that does not answer an
// outstanding AuthnRequest this SP issued to the presenting browser.
var errSAMLResponseUnbound = errors.New("response does not answer an outstanding authentication request from this browser")

// samlRequestStore records the requests this SP issued and is still waiting
// for the IdP to answer, keyed by request ID. Each record carries the key of
// what the request is bound to: the browser that started an SP-initiated
// login, or the session an SP-initiated logout ended.
//
// crewjam/saml checks a LogoutResponse's signature, destination, issuer,
// status and age but never its InResponseTo, so without these records any
// recent signed response from the IdP, including one answering another user's
// logout, would be accepted. For logins crewjam does check InResponseTo, but
// only against the request IDs its caller supplies, and without records there
// were none to supply: every SP-initiated Response was refused.
type samlRequestStore struct {
	mu      sync.Mutex
	entries map[string]samlRequestRecord
}

type samlRequestRecord struct {
	bindingKey string
	// returnTo is the local path an SP-initiated login returns to.
	returnTo  string
	expiresAt time.Time
}

func (s *samlRequestStore) put(requestID string, record samlRequestRecord, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.entries == nil {
		s.entries = make(map[string]samlRequestRecord)
	}
	for id, entry := range s.entries {
		if !now.Before(entry.expiresAt) {
			delete(s.entries, id)
		}
	}
	for len(s.entries) >= maxSAMLRequestEntries {
		oldestID := ""
		var oldestExpiry time.Time
		for id, entry := range s.entries {
			if oldestID == "" || entry.expiresAt.Before(oldestExpiry) || (entry.expiresAt.Equal(oldestExpiry) && id < oldestID) {
				oldestID = id
				oldestExpiry = entry.expiresAt
			}
		}
		delete(s.entries, oldestID)
	}
	record.expiresAt = now.Add(samlRequestTTL)
	s.entries[requestID] = record
}

// consume spends the outstanding request a verified LogoutResponse answers.
// sessionKey identifies the session the presenting browser carries, or is
// empty when it carries none. The response may complete the logout only when
// it answers an unexpired request and the browser carries either no session
// or the session that request logged out. The request is spent on every
// verified presentation, so a response is honored at most once.
func (s *samlRequestStore) consume(requestID, sessionKey string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[requestID]
	if requestID == "" || !ok {
		return fmt.Errorf("%w: no outstanding request matches InResponseTo %q", errSAMLLogoutResponseUnbound, requestID)
	}
	delete(s.entries, requestID)
	if !now.Before(entry.expiresAt) {
		return fmt.Errorf("%w: request %q expired", errSAMLLogoutResponseUnbound, requestID)
	}
	if sessionKey != "" && sessionKey != entry.bindingKey {
		return fmt.Errorf("%w: request %q logged out a different session", errSAMLLogoutResponseUnbound, requestID)
	}
	return nil
}

// outstanding returns every unexpired request, sorted, and whether any of
// them is bound to bindingKey.
func (s *samlRequestStore) outstanding(bindingKey string, now time.Time) (requestIDs []string, bound bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.entries {
		if now.Before(entry.expiresAt) {
			requestIDs = append(requestIDs, id)
			bound = bound || (bindingKey != "" && entry.bindingKey == bindingKey)
		}
	}
	sort.Strings(requestIDs)
	return requestIDs, bound
}

// spend removes and returns the unexpired request a verified ACS Response
// answers, provided it is bound to bindingKey, the presenting browser. A
// request bound to another browser is left for that browser, so presenting a
// stolen Response elsewhere does not cost its owner the login.
func (s *samlRequestStore) spend(requestID, bindingKey string, now time.Time) (samlRequestRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[requestID]
	if requestID == "" || bindingKey == "" || !ok || entry.bindingKey != bindingKey {
		return samlRequestRecord{}, fmt.Errorf("%w: no outstanding request from this browser matches InResponseTo %q", errSAMLResponseUnbound, requestID)
	}
	delete(s.entries, requestID)
	if !now.Before(entry.expiresAt) {
		return samlRequestRecord{}, fmt.Errorf("%w: request %q expired", errSAMLResponseUnbound, requestID)
	}
	return entry, nil
}

func normalizeSAMLBaseURL(baseURL string) (string, error) {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalized == "" {
		return "", nil
	}
	parsed, err := securityutil.NormalizeAbsoluteHTTPURL(normalized)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(parsed.String(), "/"), nil
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

	normalizedBaseURL, err := normalizeSAMLBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid saml base url: %w", err)
	}

	service := &SAMLService{
		providerID: providerID,
		config:     cfg,
		baseURL:    normalizedBaseURL,
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
	return newSSOHTTPClient(30*time.Second, nil)
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
	targetURL, err := validateSSOFetchURL(ctx, metadataURL)
	if err != nil {
		return nil, err
	}

	req, err := securityutil.NewValidatedRequestWithContext(ctx, http.MethodGet, targetURL, nil)
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

	ssoURL, err := securityutil.NormalizeAbsoluteHTTPURL(s.config.IDPSSOURL)
	if err != nil {
		return nil, fmt.Errorf("invalid idp sso url: %w", err)
	}

	entityID := s.config.IDPEntityID
	if entityID == "" {
		entityID = s.config.IDPIssuer
	}
	if entityID == "" {
		entityID = ssoURL.String()
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
		sloURL, err := securityutil.NormalizeAbsoluteHTTPURL(s.config.IDPSLOURL)
		if err != nil {
			return nil, fmt.Errorf("invalid idp slo url: %w", err)
		}
		metadata.IDPSSODescriptors[0].SingleLogoutServices = []saml.Endpoint{
			{
				Binding:  saml.HTTPRedirectBinding,
				Location: sloURL.String(),
			},
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
		certData, err = readSSORegularFile(s.config.IDPCertFile)
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
						// Base64 DER, as <X509Certificate> carries it in IdP
						// metadata: crewjam/saml base64-decodes this field to
						// verify signatures and cannot read a PEM block.
						X509Certificates: []saml.X509Certificate{
							{Data: base64.StdEncoding.EncodeToString(cert.Raw)},
						},
					},
				},
			},
		}
	}

	return nil
}

func (s *SAMLService) initServiceProvider() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initServiceProviderLocked()
}

func (s *SAMLService) initServiceProviderLocked() error {
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

func (s *SAMLService) SetBaseURL(baseURL string) error {
	normalizedBaseURL, err := normalizeSAMLBaseURL(baseURL)
	if err != nil {
		return fmt.Errorf("invalid saml base url: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if normalizedBaseURL == s.baseURL {
		return nil
	}

	previousBaseURL := s.baseURL
	previousSP := s.sp

	s.baseURL = normalizedBaseURL
	if err := s.initServiceProviderLocked(); err != nil {
		s.baseURL = previousBaseURL
		s.sp = previousSP
		return err
	}

	return nil
}

func (s *SAMLService) loadSPCredentials() (*x509.Certificate, *rsa.PrivateKey, error) {
	var certData, keyData []byte
	var err error

	// Load certificate
	if s.config.SPCertFile != "" {
		certData, err = readSSORegularFile(s.config.SPCertFile)
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
		keyData, err = readSSORegularFile(s.config.SPKeyFile)
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

// MakeAuthRequest creates a SAML AuthnRequest and returns the HTTP-Redirect
// binding URL that sends it to the IdP. It records the request as outstanding
// for browserKey, the login binding key of the browser starting the login,
// together with returnTo, so that ProcessResponse accepts the IdP's answer to
// this request only once, only from that browser, and returns it to returnTo.
func (s *SAMLService) MakeAuthRequest(returnTo, browserKey string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return "", errors.New("service provider not initialized")
	}
	if browserKey == "" {
		return "", errors.New("authentication request has no browser to bind")
	}

	if returnTo == "" {
		returnTo = "/"
	}
	if len(s.idpMetadata.IDPSSODescriptors) == 0 ||
		len(s.idpMetadata.IDPSSODescriptors[0].SingleSignOnServices) == 0 {
		return "", errors.New("idp does not support single sign-on")
	}

	// This is crewjam's MakeRedirectAuthenticationRequest, unrolled to learn
	// the request's ID.
	authnRequest, err := s.sp.MakeAuthenticationRequest(s.sp.GetSSOBindingLocation(saml.HTTPRedirectBinding), saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}
	// Redirect appends RelayState to the query as given, so escape it: a
	// returnTo with its own query string would otherwise split into stray
	// parameters and come back from the IdP truncated.
	redirectURL, err := authnRequest.Redirect(url.QueryEscape(returnTo), s.sp)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	log.Debug().
		Str("provider_id", s.providerID).
		Str("redirect_url", redirectURL.String()).
		Msg("Created SAML AuthnRequest")

	validatedURL, err := validateSAMLRedirectTarget(redirectURL.String(), s.idpMetadata.IDPSSODescriptors[0].SingleSignOnServices)
	if err != nil {
		return "", fmt.Errorf("failed to validate auth redirect: %w", err)
	}
	s.authnRequests.put(authnRequest.ID, samlRequestRecord{bindingKey: browserKey, returnTo: returnTo}, time.Now())
	return validatedURL, nil
}

// AllowsIDPInitiated reports whether the provider accepts IdP-initiated
// (unsolicited) Responses, which answer no AuthnRequest and so cannot be bound
// to the browser that receives them.
func (s *SAMLService) AllowsIDPInitiated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sp != nil && s.sp.AllowIDPInitiated
}

// ProcessResponse verifies the IdP's Response posted to the ACS and extracts
// the user it authenticates, along with the local path the login returns to.
//
// Unless the provider allows IdP-initiated logins, the Response must answer
// an unexpired AuthnRequest from MakeAuthRequest that is bound to browserKey,
// the login binding key of the presenting browser ("" when it carries none),
// and it spends that request, so each login is completed at most once and only
// in the browser that started it. The request ID is read from what the IdP's
// signatures cover, never from crewjam/saml's parsed fields (see
// samlSignedContent). The returned path is the one recorded with the request,
// not the posted RelayState. Binding failures wrap errSAMLResponseUnbound.
//
// With IdP-initiated logins allowed, crewjam skips InResponseTo entirely, so a
// Response answering any request, or none, is accepted in any browser and
// returns to its posted RelayState, as before request binding existed.
func (s *SAMLService) ProcessResponse(r *http.Request, browserKey string) (*SAMLAuthResult, string, error) {
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

	// crewjam checks that the Response answers one of possibleRequestIDs,
	// every outstanding AuthnRequest here; spend then decides whether it is
	// this browser's.
	now := time.Now()
	possibleRequestIDs := []string{""}
	if !s.sp.AllowIDPInitiated {
		var bound bool
		possibleRequestIDs, bound = s.authnRequests.outstanding(browserKey, now)
		if !bound {
			return nil, relayState, fmt.Errorf("%w: this browser has no login in progress", errSAMLResponseUnbound)
		}
	}

	// Parse and validate the SAML assertion on a copy of the service provider
	// whose signature verifier keeps what each verified signature covers.
	signed := &samlSignedContent{}
	sp := *s.sp
	sp.SignatureVerifier = signed
	assertion, err := sp.ParseResponse(r, possibleRequestIDs)
	if err != nil {
		return nil, relayState, fmt.Errorf("failed to validate saml response: %w", err)
	}

	if !s.sp.AllowIDPInitiated {
		requestID, err := signed.inResponseTo(assertion.ID)
		if err != nil {
			return nil, relayState, fmt.Errorf("%w: %v", errSAMLResponseUnbound, err)
		}
		// Artifact resolution and signature checks take time, so judge
		// the request's expiry now rather than when parsing began.
		request, err := s.authnRequests.spend(requestID, browserKey, time.Now())
		if err != nil {
			return nil, relayState, err
		}
		relayState = request.returnTo
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

// MakeLogoutRequest creates a SAML LogoutRequest for SLO and records its ID
// as outstanding for sessionKey, the session being logged out, so that
// ValidateLogoutResponse accepts only the IdP's answer to this request.
func (s *SAMLService) MakeLogoutRequest(nameID, sessionIdx, sessionKey string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return "", errors.New("service provider not initialized")
	}
	if sessionKey == "" {
		return "", errors.New("logout request has no session to bind")
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

	validatedURL, err := validateSAMLRedirectTarget(redirectURL.String(), s.idpMetadata.IDPSSODescriptors[0].SingleLogoutServices)
	if err != nil {
		return "", err
	}
	s.logoutRequests.put(req.ID, samlRequestRecord{bindingKey: sessionKey}, time.Now())
	return validatedURL, nil
}

// ValidateLogoutResponse verifies an incoming IdP-signed SAML LogoutResponse,
// checking the XML-DSig signature against the IdP's published certificate as
// well as the standard LogoutResponse temporal / target invariants, and then
// binds it to the request it answers: its InResponseTo must name an
// unexpired LogoutRequest from MakeLogoutRequest, which it spends, and
// sessionKey (the presenting browser's session, or "" for none) must be empty
// or the session that request logged out. Returns nil only when all of that
// holds — anything else (no payload, bad signature, unknown issuer, expired,
// unsolicited, replayed, or another session's logout) returns an error and
// the caller MUST NOT mutate any session state on the strength of the
// request. Binding failures wrap errSAMLLogoutResponseUnbound.
//
// Without signature validation the SLO callback endpoint was an
// unauthenticated force-logout DoS: a cross-origin POST with no payload, or
// any forged payload, cleared the user's session. Before the binding, any
// user could replay the IdP's answer to their own logout through a cross-site
// GET to clear someone else's session.
func (s *SAMLService) ValidateLogoutResponse(req *http.Request, sessionKey string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sp == nil {
		return errors.New("service provider not initialized")
	}

	inResponseTo, err := s.verifiedLogoutResponseInResponseTo(req)
	if err != nil {
		return err
	}
	return s.logoutRequests.consume(inResponseTo, sessionKey, time.Now())
}

// verifiedLogoutResponseInResponseTo selects the SAMLResponse payload the way
// crewjam/saml's ValidateLogoutResponseRequest does (the HTTP-Redirect query
// parameter first, then the HTTP-POST form field), has crewjam verify exactly
// that payload, and reads InResponseTo from the same bytes.
func (s *SAMLService) verifiedLogoutResponseInResponseTo(req *http.Request) (string, error) {
	var raw []byte
	if data := req.URL.Query().Get("SAMLResponse"); data != "" {
		if err := s.sp.ValidateLogoutResponseRedirect(data); err != nil {
			return "", err
		}
		compressed, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", fmt.Errorf("decode logout response: %w", err)
		}
		raw, err = io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(compressed)), samlLogoutResponseMaxBytes))
		if err != nil {
			return "", fmt.Errorf("inflate logout response: %w", err)
		}
	} else {
		if err := req.ParseForm(); err != nil {
			return "", fmt.Errorf("parse logout response form: %w", err)
		}
		data := req.PostForm.Get("SAMLResponse")
		if err := s.sp.ValidateLogoutResponseForm(data); err != nil {
			return "", err
		}
		var err error
		raw, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", fmt.Errorf("decode logout response: %w", err)
		}
	}
	return logoutResponseInResponseTo(raw)
}

// logoutResponseInResponseTo returns the root LogoutResponse's InResponseTo
// attribute as written without a prefix, the one the IdP's enveloped signature
// covers, or "" when it has none. The attributes are read by hand, lexically,
// because encoding/xml would also fill an un-namespaced attribute field from a
// namespace declaration such as xmlns:InResponseTo="...", which exclusive
// canonicalization can leave out of the signed form when nothing uses it. crewjam
// has already checked that the root is a protocol LogoutResponse.
func logoutResponseInResponseTo(raw []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := decoder.RawToken()
		if err != nil {
			return "", fmt.Errorf("parse logout response: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "LogoutResponse" {
			return "", fmt.Errorf("parse logout response: unexpected root element %q", start.Name.Local)
		}
		inResponseTo, found := "", false
		for _, attr := range start.Attr {
			if attr.Name.Space != "" || attr.Name.Local != "InResponseTo" {
				continue
			}
			if found {
				return "", errors.New("parse logout response: duplicate InResponseTo")
			}
			inResponseTo, found = attr.Value, true
		}
		return inResponseTo, nil
	}
}

// samlSignedContent is the signature verifier ProcessResponse gives crewjam/saml
// for one ACS Response. It verifies each signature exactly as crewjam's default
// verifier does and keeps the content that signature covers: the canonical form
// goxmldsig digested, which holds only what the IdP signed.
//
// crewjam reads a Response's fields by unmarshalling the posted elements with
// encoding/xml, which fills an un-namespaced attribute field from any attribute
// with that local name, whatever its prefix, the last one winning. That
// includes a namespace declaration such as xmlns:IssueInstant, and exclusive
// canonicalization leaves a declaration nothing uses out of the signed form,
// so one can be appended to a signed element without breaking its signature
// and override what crewjam checks: InResponseTo, IssueInstant, NotOnOrAfter,
// Recipient, Destination. VerifySignature therefore refuses signed content
// carrying any prefixed attribute or declaration named like an un-namespaced
// attribute crewjam reads, so crewjam's fields match what was signed, and the
// binding decision still reads InResponseTo from the kept canonical content
// rather than from crewjam's fields.
type samlSignedContent struct {
	elements []*etree.Element
}

func (c *samlSignedContent) VerifySignature(validationContext *dsig.ValidationContext, el *etree.Element) error {
	if shadow := samlShadowingAttribute(el); shadow != "" {
		return fmt.Errorf("cannot validate signature on %s: %s shadows a SAML attribute", el.Tag, shadow)
	}
	signed, err := validationContext.Validate(el)
	if err != nil {
		return fmt.Errorf("cannot validate signature on %s: %v", el.Tag, err)
	}
	c.elements = append(c.elements, signed)
	return nil
}

// inResponseTo returns the AuthnRequest ID that the signature covering the
// assertion crewjam returned, identified by its ID, says it answers.
//
// When the Response, or an ArtifactResponse carrying it, is signed, the IdP
// signed the whole message, the returned assertion included, and the signed
// Response's InResponseTo names the request. Otherwise crewjam verified the
// returned assertion's own signature, and the request is named by that
// assertion's SubjectConfirmationData: every SubjectConfirmation of its
// Subject must carry the same one, and at least one must exist, because
// crewjam accepts a Subject without any. Other signed assertions in the same
// Response, and assertions in Advice, do not take part.
func (c *samlSignedContent) inResponseTo(assertionID string) (string, error) {
	for _, el := range c.elements {
		response := el
		if el.Tag == "ArtifactResponse" {
			response = samlChildElement(el, "Response")
		}
		if response == nil || response.Tag != "Response" {
			continue
		}
		if requestID := samlUnprefixedAttr(response, "InResponseTo"); requestID != "" {
			return requestID, nil
		}
		return "", errors.New("the signed Response answers no request")
	}
	var assertion *etree.Element
	for _, el := range c.elements {
		if el.Tag != "Assertion" || samlUnprefixedAttr(el, "ID") != assertionID {
			continue
		}
		if assertion != nil {
			return "", fmt.Errorf("several signed assertions carry ID %q", assertionID)
		}
		assertion = el
	}
	if assertion == nil {
		return "", errors.New("no signature covers the returned assertion")
	}
	requestID := ""
	if subject := samlChildElement(assertion, "Subject"); subject != nil {
		for _, confirmation := range subject.ChildElements() {
			if confirmation.Tag != "SubjectConfirmation" {
				continue
			}
			// crewjam keeps the last SubjectConfirmationData of each
			// confirmation, so every one must agree.
			named := false
			for _, data := range confirmation.ChildElements() {
				if data.Tag != "SubjectConfirmationData" {
					continue
				}
				value := samlUnprefixedAttr(data, "InResponseTo")
				if value == "" || (requestID != "" && value != requestID) {
					return "", errors.New("the signed assertion's subject confirmations do not name one request")
				}
				requestID, named = value, true
			}
			if !named {
				return "", errors.New("a signed subject confirmation names no request")
			}
		}
	}
	if requestID == "" {
		return "", errors.New("the signed assertion has no subject confirmation naming a request")
	}
	return requestID, nil
}

// samlChildElement returns el's first child element with local name tag.
func samlChildElement(el *etree.Element, tag string) *etree.Element {
	for _, child := range el.ChildElements() {
		if child.Tag == tag {
			return child
		}
	}
	return nil
}

// samlUnprefixedAttr returns el's attribute key written without a prefix, the
// one a signature over el covers, or "" when it has none.
func samlUnprefixedAttr(el *etree.Element, key string) string {
	for _, attr := range el.Attr {
		if attr.Space == "" && attr.Key == key {
			return attr.Value
		}
	}
	return ""
}

// samlBoundAttributeNames holds the name of every un-namespaced XML attribute
// crewjam/saml unmarshals from a Response or an Assertion. Namespaced ones such
// as xsi:type only match their own namespace.
var samlBoundAttributeNames = func() map[string]bool {
	names := make(map[string]bool)
	seen := make(map[reflect.Type]bool)
	var collect func(t reflect.Type)
	collect = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			name, options, _ := strings.Cut(field.Tag.Get("xml"), ",")
			if strings.Contains(","+options+",", ",attr,") {
				if name == "" {
					name = field.Name
				}
				if !strings.Contains(name, " ") {
					names[name] = true
				}
			}
			collect(field.Type)
		}
	}
	collect(reflect.TypeOf(saml.Response{}))
	collect(reflect.TypeOf(saml.Assertion{}))
	return names
}()

// samlShadowingAttribute returns the first prefixed attribute or namespace
// declaration in el or its descendants whose local name is also an
// un-namespaced attribute crewjam/saml reads, as written, or "" when there is
// none.
func samlShadowingAttribute(el *etree.Element) string {
	for _, attr := range el.Attr {
		if attr.Space != "" && samlBoundAttributeNames[attr.Key] {
			return attr.Space + ":" + attr.Key
		}
	}
	for _, child := range el.ChildElements() {
		if shadow := samlShadowingAttribute(child); shadow != "" {
			return shadow
		}
	}
	return ""
}

func validateSAMLRedirectTarget(rawURL string, allowedEndpoints []saml.Endpoint) (string, error) {
	validatedURL, err := securityutil.NormalizeAbsoluteHTTPURL(rawURL)
	if err != nil {
		return "", err
	}
	for _, endpoint := range allowedEndpoints {
		endpointURL, err := securityutil.NormalizeAbsoluteHTTPURL(endpoint.Location)
		if err != nil {
			continue
		}
		if strings.EqualFold(validatedURL.Scheme, endpointURL.Scheme) &&
			strings.EqualFold(validatedURL.Host, endpointURL.Host) &&
			validatedURL.Path == endpointURL.Path {
			return validatedURL.String(), nil
		}
	}
	return "", fmt.Errorf("redirect target does not match configured SAML endpoint")
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
