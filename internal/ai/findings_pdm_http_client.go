package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

const (
	pdmAPIURLEnv                = "PDM_API_URL"
	pdmAPITokenEnv              = "PDM_API_TOKEN"
	pdmAPITokenSecretEnv        = "PDM_API_TOKEN_SECRET"
	pdmInsecureSkipVerifyEnv    = "PDM_INSECURE_SKIP_VERIFY"
	pdmResourcesListPath        = "api2/extjs/resources/list"
	pdmAPITokenAuthScheme       = "PDMAPIToken"
	pdmHTTPClientRequestTimeout = 10 * time.Second
)

type pdmHTTPClient struct {
	baseURL     *url.URL
	httpClient  *http.Client
	tokenID     string
	tokenSecret string
}

func loadPDMAlertBridgeConfigFromEnv() pdmAlertBridgeConfig {
	return pdmAlertBridgeConfig{
		APIURL:             strings.TrimSpace(os.Getenv(pdmAPIURLEnv)),
		APIToken:           strings.TrimSpace(os.Getenv(pdmAPITokenEnv)),
		APITokenSecret:     strings.TrimSpace(os.Getenv(pdmAPITokenSecretEnv)),
		InsecureSkipVerify: parsePDMInsecureSkipVerify(os.Getenv(pdmInsecureSkipVerifyEnv)),
	}
}

func parsePDMInsecureSkipVerify(raw string) bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	return err == nil && enabled
}

func (cfg pdmAlertBridgeConfig) enabled() bool {
	return cfg.APIURL != "" && cfg.APIToken != "" && cfg.APITokenSecret != ""
}

func newPDMAlertSourceFromEnv() pdmAlertSource {
	client, err := newPDMHTTPClient(loadPDMAlertBridgeConfigFromEnv())
	if err != nil {
		return nil
	}
	if client == nil {
		return nil
	}
	return client
}

func newPDMHTTPClient(cfg pdmAlertBridgeConfig) (*pdmHTTPClient, error) {
	if !cfg.enabled() {
		return nil, nil
	}

	baseURL, err := securityutil.NormalizeHTTPBaseURL(cfg.APIURL, "https")
	if err != nil {
		return nil, fmt.Errorf("invalid PDM API URL: %w", err)
	}

	httpClient := tlsutil.CreateHTTPClientWithTimeout(!cfg.InsecureSkipVerify, "", pdmHTTPClientRequestTimeout)
	if httpClient == nil {
		return nil, fmt.Errorf("create PDM HTTP client")
	}

	return &pdmHTTPClient{
		baseURL:     baseURL,
		httpClient:  httpClient,
		tokenID:     cfg.APIToken,
		tokenSecret: cfg.APITokenSecret,
	}, nil
}

func (c *pdmHTTPClient) ResourceList(ctx context.Context) ([]pdmResource, error) {
	if c == nil {
		return nil, fmt.Errorf("PDM HTTP client is nil")
	}

	endpoint := securityutil.AppendURLPath(c.baseURL, pdmResourcesListPath)
	req, err := securityutil.NewValidatedRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create PDM resources request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", pdmAPITokenAuthScheme+" "+c.tokenID+":"+c.tokenSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch PDM resources: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch PDM resources: unexpected status %d", resp.StatusCode)
	}

	var envelope pdmResourcesResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode PDM resources response: %w", err)
	}

	return envelope.resources(), nil
}

type pdmResourcesResponse struct {
	Data []pdmRemoteResources `json:"data"`
}

type pdmRemoteResources struct {
	Remote    string           `json:"remote"`
	Error     string           `json:"error,omitempty"`
	Resources []pdmRawResource `json:"resources"`
}

type pdmRawResource struct {
	Type        string  `json:"type"`
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Node        string  `json:"node"`
	Storage     string  `json:"storage"`
	Status      string  `json:"status"`
	Uptime      uint64  `json:"uptime"`
	Maintenance *string `json:"maintenance"`
}

func (r pdmResourcesResponse) resources() []pdmResource {
	var out []pdmResource
	for _, remote := range r.Data {
		remoteID := strings.TrimSpace(remote.Remote)
		if remoteID == "" {
			continue
		}
		for _, raw := range remote.Resources {
			resource, ok := raw.resource(remoteID)
			if ok {
				out = append(out, resource)
			}
		}
	}
	return out
}

func (r pdmRawResource) resource(remoteID string) (pdmResource, bool) {
	resourceType, ok := pdmBridgeResourceType(r.Type)
	if !ok {
		return pdmResource{}, false
	}

	name := strings.TrimSpace(r.resourceName())
	if name == "" {
		return pdmResource{}, false
	}

	status := r.resourceStatus()
	id := strings.TrimSpace(r.ID)
	if id == "" {
		id = remoteID + "/" + resourceType + "/" + name
	}

	return pdmResource{
		ID:       id,
		RemoteID: remoteID,
		Name:     name,
		Type:     resourceType,
		Status:   status,
	}, true
}

func (r pdmRawResource) resourceName() string {
	switch r.Type {
	case "pve-node":
		return r.Node
	case "pve-storage":
		return r.Storage
	default:
		return r.Name
	}
}

func (r pdmRawResource) resourceStatus() string {
	switch r.Type {
	case "pbs-node":
		if r.Uptime > 0 {
			return "online"
		}
		return "offline"
	case "pbs-datastore":
		if r.Maintenance == nil {
			return "online"
		}
		return "unknown"
	default:
		status := strings.TrimSpace(strings.ToLower(r.Status))
		if status == "" {
			return "unknown"
		}
		return status
	}
}

func pdmBridgeResourceType(resourceType string) (string, bool) {
	switch resourceType {
	case "pve-qemu":
		return "qemu", true
	case "pve-lxc":
		return "lxc", true
	case "pve-node", "pbs-node":
		return "node", true
	case "pve-storage", "pbs-datastore":
		return "storage", true
	}
	return "", false
}
