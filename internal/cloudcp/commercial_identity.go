package cloudcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const commercialIdentityLookupTimeout = 10 * time.Second

type commercialIdentity struct {
	Email                 string   `json:"email"`
	HasCommercialIdentity bool     `json:"has_commercial_identity"`
	Sources               []string `json:"sources,omitempty"`
	V5LicenseCount        int      `json:"v5_license_count"`
	V6LicenseCount        int      `json:"v6_license_count"`
	StripeCustomerID      string   `json:"stripe_customer_id,omitempty"`
}

type commercialIdentityLookupFunc func(ctx context.Context, email string) (*commercialIdentity, error)

func newCommercialIdentityLookup(cfg *CPConfig) commercialIdentityLookupFunc {
	if cfg == nil {
		return nil
	}
	baseURL := strings.TrimSpace(cfg.LicenseServerURL)
	adminToken := strings.TrimSpace(cfg.LicenseAdminToken)
	if baseURL == "" || adminToken == "" {
		return nil
	}

	client := &http.Client{Timeout: commercialIdentityLookupTimeout}
	return func(ctx context.Context, email string) (*commercialIdentity, error) {
		email = strings.TrimSpace(email)
		if email == "" {
			return nil, fmt.Errorf("email is required")
		}

		endpoint, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("parse license server url: %w", err)
		}
		endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/v1/admin/commercial/lookup"
		query := endpoint.Query()
		query.Set("email", email)
		endpoint.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("build commercial lookup request: %w", err)
		}
		req.Header.Set("X-Admin-Key", adminToken)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("perform commercial lookup request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("commercial lookup returned status %d", resp.StatusCode)
		}

		var result commercialIdentity
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode commercial lookup response: %w", err)
		}
		return &result, nil
	}
}
