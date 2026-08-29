package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

const (
	serviceHealthProbeTimeout = 5 * time.Second
	serviceHealthBodyLimit    = 2 << 20
	serviceHealthAssetLimit   = 32
)

var frontendAssetReferencePattern = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["'](/assets/[^"'#?]+(?:\?[^"'#]*)?)["']`)

func newServiceHealthProbe(listener net.Listener, tlsEnabled bool) func() telemetry.ServiceHealthObservation {
	baseURL, ok := localServiceHealthBaseURL(listener, tlsEnabled)
	if !ok {
		return func() telemetry.ServiceHealthObservation {
			return telemetry.ServiceHealthObservation{
				Observed:        true,
				FailureCategory: telemetry.ServiceHealthFailureAPIConnectivity,
			}
		}
	}

	transport := &http.Transport{}
	if tlsEnabled {
		// The probe stays inside this process and connects only to the address
		// already bound by listener. Certificate trust is a client-facing concern,
		// while this probe verifies that Pulse can serve its own HTTPS handler.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   serviceHealthProbeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return func() telemetry.ServiceHealthObservation {
		ctx, cancel := context.WithTimeout(context.Background(), serviceHealthProbeTimeout)
		defer cancel()

		apiBody, status, err := serviceHealthGET(ctx, client, baseURL+"/api/health")
		if err != nil {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureAPIConnectivity)
		}
		if status < http.StatusOK || status >= http.StatusMultipleChoices {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureAPIStatus)
		}
		var health struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(apiBody, &health) != nil || health.Status != "healthy" {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureAPIStatus)
		}

		indexBody, status, err := serviceHealthGET(ctx, client, baseURL+"/")
		if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices ||
			!strings.Contains(strings.ToLower(string(indexBody)), "<html") {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureUIStatus)
		}

		assetPaths := frontendAssetPaths(indexBody)
		if len(assetPaths) == 0 {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureFrontendAssets)
		}
		for _, assetPath := range assetPaths {
			body, assetStatus, assetErr := serviceHealthGET(ctx, client, baseURL+assetPath)
			if assetErr != nil || assetStatus < http.StatusOK || assetStatus >= http.StatusMultipleChoices || len(body) == 0 {
				return unhealthyServiceObservation(telemetry.ServiceHealthFailureFrontendAssets)
			}
		}

		return telemetry.ServiceHealthObservation{Observed: true, Healthy: true}
	}
}

func localServiceHealthBaseURL(listener net.Listener, tlsEnabled bool) (string, bool) {
	if listener == nil {
		return "", false
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || tcpAddr.Port <= 0 {
		return "", false
	}
	ip := tcpAddr.IP
	if ip == nil || ip.IsUnspecified() {
		if ip != nil && ip.To4() == nil {
			ip = net.IPv6loopback
		} else {
			ip = net.IPv4(127, 0, 0, 1)
		}
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(ip.String(), fmt.Sprintf("%d", tcpAddr.Port))), true
}

func frontendAssetPaths(index []byte) []string {
	matches := frontendAssetReferencePattern.FindAllSubmatch(index, serviceHealthAssetLimit)
	paths := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		parsed, err := url.Parse(string(match[1]))
		if err != nil || !strings.HasPrefix(parsed.Path, "/assets/") {
			continue
		}
		path := parsed.EscapedPath()
		if parsed.RawQuery != "" {
			path += "?" + parsed.RawQuery
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func serviceHealthGET(ctx context.Context, client *http.Client, target string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, serviceHealthBodyLimit+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(body) > serviceHealthBodyLimit {
		return nil, resp.StatusCode, fmt.Errorf("service-health response exceeded bounded size")
	}
	return body, resp.StatusCode, nil
}

func unhealthyServiceObservation(category string) telemetry.ServiceHealthObservation {
	return telemetry.ServiceHealthObservation{Observed: true, FailureCategory: category}
}
