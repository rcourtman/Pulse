package cloudcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultLicenseServerURL = "https://license.pulserelay.pro"

type CommercialIdentityLookupResult struct {
	Email                 string   `json:"email"`
	HasCommercialIdentity bool     `json:"has_commercial_identity"`
	Sources               []string `json:"sources,omitempty"`
	V5LicenseCount        int      `json:"v5_license_count"`
	V6LicenseCount        int      `json:"v6_license_count"`
	StripeCustomerID      string   `json:"stripe_customer_id,omitempty"`
}

type commercialIdentityLookup interface {
	LookupCommercialIdentity(ctx context.Context, email string) (*CommercialIdentityLookupResult, error)
}

type commercialIdentityClient struct {
	baseURL    string
	adminToken string
	httpClient *http.Client
}

func newCommercialIdentityClient(cfg *CPConfig) commercialIdentityLookup {
	if cfg == nil {
		return nil
	}
	adminToken := strings.TrimSpace(cfg.LicenseAdminToken)
	if adminToken == "" {
		return nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.LicenseServerURL), "/")
	if baseURL == "" {
		baseURL = defaultLicenseServerURL
	}
	return &commercialIdentityClient{
		baseURL:    baseURL,
		adminToken: adminToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *commercialIdentityClient) LookupCommercialIdentity(ctx context.Context, email string) (*CommercialIdentityLookupResult, error) {
	if c == nil {
		return nil, nil
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	endpoint := c.baseURL + "/v1/admin/commercial/lookup?email=" + url.QueryEscape(email)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create commercial identity request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Token", c.adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("commercial identity request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("commercial identity lookup returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result CommercialIdentityLookupResult
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode commercial identity response: %w", err)
	}
	return &result, nil
}
