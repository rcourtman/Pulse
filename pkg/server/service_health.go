package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	serviceHealthRetryDelay   = time.Second
	serviceHealthBodyLimit    = 2 << 20
	serviceHealthAssetLimit   = 32
)

var frontendAssetReferencePattern = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["'](/assets/[^"'#?]+(?:\?[^"'#]*)?)["']`)

func newServiceHealthProbe(listener net.Listener, tlsEnabled bool) func() telemetry.ServiceHealthObservation {
	baseURLs := localServiceHealthBaseURLs(listener, tlsEnabled)
	if len(baseURLs) == 0 {
		return func() telemetry.ServiceHealthObservation {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureAPIConnectivity)
		}
	}

	transport := &http.Transport{}
	if tlsEnabled {
		// The probe connects only to the address already bound by listener
		// (or loopback for a wildcard). Certificate trust is a client-facing
		// concern; this verifies Pulse can serve its own HTTPS handler.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   serviceHealthProbeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return serviceHealthProbe(baseURLs, client, serviceHealthProbeTimeout, serviceHealthRetryDelay)
}

// Each observation gets at most two five-second attempts with one second for
// startup to settle between them. Telemetry invokes this in its background
// runner, never on the monitoring or HTTP serving path.
func serviceHealthProbe(baseURLs []string, client *http.Client, timeout, retryDelay time.Duration) func() telemetry.ServiceHealthObservation {
	return func() telemetry.ServiceHealthObservation {
		defer client.CloseIdleConnections()
		var observation telemetry.ServiceHealthObservation
		for attempt := 0; attempt < 2; attempt++ {
			if attempt > 0 {
				timer := time.NewTimer(retryDelay)
				<-timer.C
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			observation = observeServiceHealth(ctx, client, baseURLs)
			cancel()
			if observation.Healthy {
				return observation
			}
		}
		return observation
	}
}

func observeServiceHealth(ctx context.Context, client *http.Client, baseURLs []string) telemetry.ServiceHealthObservation {
	baseURL, apiBody, status, err := serviceHealthAPI(ctx, client, baseURLs)
	if err != nil {
		category := telemetry.ServiceHealthFailureAPIConnectivity
		if status != 0 {
			category = telemetry.ServiceHealthFailureAPIStatus
		}
		return unhealthyServiceObservation(serviceHealthErrorCategory(err, category))
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
	if err != nil {
		return unhealthyServiceObservation(serviceHealthErrorCategory(err, telemetry.ServiceHealthFailureUIStatus))
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices ||
		!strings.Contains(strings.ToLower(string(indexBody)), "<html") {
		return unhealthyServiceObservation(telemetry.ServiceHealthFailureUIStatus)
	}

	assetPaths := frontendAssetPaths(indexBody)
	if len(assetPaths) == 0 {
		return unhealthyServiceObservation(telemetry.ServiceHealthFailureFrontendAssets)
	}
	for _, assetPath := range assetPaths {
		body, assetStatus, assetErr := serviceHealthGET(ctx, client, baseURL+assetPath)
		if assetErr != nil {
			return unhealthyServiceObservation(serviceHealthErrorCategory(assetErr, telemetry.ServiceHealthFailureFrontendAssets))
		}
		if assetStatus < http.StatusOK || assetStatus >= http.StatusMultipleChoices || len(body) == 0 {
			return unhealthyServiceObservation(telemetry.ServiceHealthFailureFrontendAssets)
		}
	}

	return telemetry.ServiceHealthObservation{Observed: true, Healthy: true}
}

// Only a transport failure tries the other loopback family. A response pins
// the entire API/UI/asset observation to that address, even when it is unhealthy.
// Divide the remaining API budget so a stalled family cannot starve the other.
func serviceHealthAPI(ctx context.Context, client *http.Client, baseURLs []string) (string, []byte, int, error) {
	var lastErr, timeoutErr error
	for i, baseURL := range baseURLs {
		deadline, _ := ctx.Deadline()
		candidateCtx, cancel := context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(baseURLs)-i))
		body, status, err := serviceHealthGET(candidateCtx, client, baseURL+"/api/health")
		cancel()
		if err == nil {
			return baseURL, body, status, nil
		}
		if status != 0 {
			// A response whose body fails or exceeds the limit is still an
			// observation of this server, not an unavailable address family.
			return baseURL, nil, status, err
		}
		lastErr = err
		if serviceHealthErrorCategory(err, "") == telemetry.ServiceHealthFailureTimeout {
			timeoutErr = err
		}
	}
	if timeoutErr != nil {
		return "", nil, 0, timeoutErr
	}
	return "", nil, 0, lastErr
}

func serviceHealthErrorCategory(err error, fallback string) string {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return telemetry.ServiceHealthFailureTimeout
	}
	return fallback
}

func localServiceHealthBaseURLs(listener net.Listener, tlsEnabled bool) []string {
	if listener == nil {
		return nil
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || tcpAddr.Port <= 0 || tcpAddr.Port > 65535 {
		return nil
	}
	hosts := []string{tcpAddr.IP.String()}
	if tcpAddr.IP == nil || tcpAddr.IP.IsUnspecified() {
		hosts = []string{"127.0.0.1"}
		// An IPv6 wildcard can be dual-stack or IPv6-only. IPv4-first also
		// works when IPv6 loopback is disabled. An IPv4-only wildcard must
		// not inspect a different IPv6 listener that happens to share its port.
		if tcpAddr.IP == nil || tcpAddr.IP.To4() == nil {
			hosts = append(hosts, "::1")
		}
	} else if tcpAddr.Zone != "" && tcpAddr.IP.To4() == nil {
		hosts[0] += "%" + tcpAddr.Zone
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	baseURLs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		baseURL := url.URL{Scheme: scheme, Host: net.JoinHostPort(host, fmt.Sprintf("%d", tcpAddr.Port))}
		baseURLs = append(baseURLs, baseURL.String())
	}
	return baseURLs
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
