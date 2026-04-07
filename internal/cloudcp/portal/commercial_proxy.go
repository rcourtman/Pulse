package portal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultCommercialProxyBaseURL = "https://license.pulserelay.pro"

var allowedCommercialProxyEndpoints = map[string]map[string]struct{}{
	"/v1/public/pricing-model": {
		http.MethodGet: {},
	},
	"/v1/checkout/session": {
		http.MethodGet:  {},
		http.MethodPost: {},
	},
	"/v1/checkout/intent": {
		http.MethodGet:  {},
		http.MethodPost: {},
	},
	"/v1/manage/request": {
		http.MethodPost: {},
	},
	"/v1/manage": {
		http.MethodPost: {},
	},
	"/v1/retrieve-license/request": {
		http.MethodPost: {},
	},
	"/v1/retrieve-license": {
		http.MethodPost: {},
	},
	"/v1/gdpr/request-export": {
		http.MethodPost: {},
	},
	"/v1/gdpr/export": {
		http.MethodPost: {},
	},
	"/v1/gdpr/request-delete": {
		http.MethodPost: {},
	},
	"/v1/gdpr/confirm-delete": {
		http.MethodPost: {},
	},
	"/v1/self-refund": {
		http.MethodPost: {},
	},
}

type CommercialProxyConfig struct {
	BaseURL string
}

func HandleCommercialProxy(cfg CommercialProxyConfig) http.HandlerFunc {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultCommercialProxyBaseURL
	}

	client := &http.Client{Timeout: 15 * time.Second}

	return func(w http.ResponseWriter, r *http.Request) {
		targetPath := normalizeCommercialProxyPath(r.PathValue("commercial_path"))
		allowedMethods, ok := allowedCommercialProxyEndpoints[targetPath]
		if !ok {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if _, allowed := allowedMethods[r.Method]; !allowed {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		upstreamURL, err := url.Parse(baseURL + targetPath)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		upstreamURL.RawQuery = strings.TrimSpace(r.URL.RawQuery)

		req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), bytes.NewReader(body))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if contentType := strings.TrimSpace(r.Header.Get("Content-Type")); contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, "commercial_proxy_unavailable"), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
	}
}

func normalizeCommercialProxyPath(raw string) string {
	raw = "/" + strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\", "/")
	for strings.Contains(raw, "//") {
		raw = strings.ReplaceAll(raw, "//", "/")
	}
	if raw == "/" {
		return ""
	}
	return raw
}
