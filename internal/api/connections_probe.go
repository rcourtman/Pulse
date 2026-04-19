package api

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Probe budget. Per-candidate and whole-request ceilings are kept tight to
// make the endpoint unusable as a slow-leak scanner. Each candidate gets
// 2s connect + 1s read, and the whole request is capped at the total budget.
const (
	probeDialTimeout     = 2 * time.Second
	probeTotalBudget     = 3 * time.Second
	probeMaxConcurrent   = 5
	probeMaxAddressBytes = 512
)

// ProbeRequest is the POST body for /api/connections/probe.
type ProbeRequest struct {
	Address string `json:"address"`
}

// ProbeCandidate is one detected product at a host:port.
type ProbeCandidate struct {
	Type  ConnectionType    `json:"type"`
	Host  string            `json:"host"`
	Port  int               `json:"port"`
	Hints map[string]string `json:"hints,omitempty"`
}

// ProbeResponse is the envelope returned to the frontend.
type ProbeResponse struct {
	Candidates []ProbeCandidate `json:"candidates"`
	ProbedMs   int64            `json:"probedMs"`
}

// probeTarget is a single {type, scheme, port, path} fingerprint we attempt.
type probeTarget struct {
	Type       ConnectionType
	Scheme     string
	Port       int
	Path       string
	identifyFn func(resp *http.Response, body []byte) (match bool, hints map[string]string)
}

// PVE, PBS, and PMG all delay auth-required 401 responses by ~3 seconds as a
// timing-attack mitigation. /api2/json/version requires auth, so probing it
// costs the full probe budget on every attempt. The web-UI root at / returns
// the login page in <100ms and carries the same identifying Server banner
// (PVE/PMG) or an unambiguous HTML title (PBS, which omits Server on /).
var defaultProbeTargets = []probeTarget{
	{
		Type:   ConnectionTypePVE,
		Scheme: "https",
		Port:   8006,
		Path:   "/",
		identifyFn: func(resp *http.Response, body []byte) (bool, map[string]string) {
			server := strings.ToLower(resp.Header.Get("Server"))
			if strings.Contains(server, "pve-api-daemon") {
				return true, map[string]string{"product": "Proxmox VE"}
			}
			if strings.Contains(string(body), "Proxmox Virtual Environment") {
				return true, map[string]string{"product": "Proxmox VE"}
			}
			return false, nil
		},
	},
	{
		Type:   ConnectionTypePBS,
		Scheme: "https",
		Port:   8007,
		Path:   "/",
		identifyFn: func(resp *http.Response, body []byte) (bool, map[string]string) {
			server := strings.ToLower(resp.Header.Get("Server"))
			if strings.Contains(server, "proxmox-backup") {
				return true, map[string]string{"product": "Proxmox Backup Server"}
			}
			if strings.Contains(string(body), "Proxmox Backup Server") {
				return true, map[string]string{"product": "Proxmox Backup Server"}
			}
			return false, nil
		},
	},
	{
		Type:   ConnectionTypePMG,
		Scheme: "https",
		Port:   8006,
		Path:   "/",
		identifyFn: func(resp *http.Response, body []byte) (bool, map[string]string) {
			server := strings.ToLower(resp.Header.Get("Server"))
			if strings.Contains(server, "pmg-api-daemon") {
				return true, map[string]string{"product": "Proxmox Mail Gateway"}
			}
			if strings.Contains(string(body), "Proxmox Mail Gateway") {
				return true, map[string]string{"product": "Proxmox Mail Gateway"}
			}
			return false, nil
		},
	},
	{
		Type:   ConnectionTypeVMware,
		Scheme: "https",
		Port:   443,
		Path:   "/sdk/vimServiceVersions.xml",
		identifyFn: func(_ *http.Response, body []byte) (bool, map[string]string) {
			if strings.Contains(string(body), "urn:vim25") {
				return true, map[string]string{"product": "VMware vCenter"}
			}
			return false, nil
		},
	},
	{
		Type:   ConnectionTypeTrueNAS,
		Scheme: "https",
		Port:   443,
		Path:   "/api/v2.0/system/product_name",
		identifyFn: func(_ *http.Response, body []byte) (bool, map[string]string) {
			upper := strings.ToUpper(string(body))
			if strings.Contains(upper, "TRUENAS") {
				return true, map[string]string{"product": "TrueNAS"}
			}
			return false, nil
		},
	},
}

// parseProbeAddress normalizes user input into (host, explicitPort).
// Accepted forms: "host", "host:port", "ip", "ip:port", "scheme://host[:port]".
// explicitPort == 0 means "probe all defaults"; otherwise only probe targets
// that match the given port.
func parseProbeAddress(raw string) (host string, explicitPort int, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, fmt.Errorf("address is required")
	}
	if len(raw) > probeMaxAddressBytes {
		return "", 0, fmt.Errorf("address is too long")
	}

	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return "", 0, fmt.Errorf("invalid URL: %s", raw)
		}
		host = u.Hostname()
		if portStr := u.Port(); portStr != "" {
			p, err := strconv.Atoi(portStr)
			if err != nil || p < 1 || p > 65535 {
				return "", 0, fmt.Errorf("invalid port in URL")
			}
			explicitPort = p
		}
		return host, explicitPort, nil
	}

	if h, p, splitErr := net.SplitHostPort(raw); splitErr == nil {
		portNum, convErr := strconv.Atoi(p)
		if convErr != nil || portNum < 1 || portNum > 65535 {
			return "", 0, fmt.Errorf("invalid port")
		}
		return h, portNum, nil
	}

	return raw, 0, nil
}

// targetsForPort narrows defaultProbeTargets to only those matching the
// user's explicit port. Same host can serve PVE on 8006 and PBS on 8007, so
// zero port means "try them all."
func targetsForPort(port int) []probeTarget {
	if port == 0 {
		return defaultProbeTargets
	}
	out := make([]probeTarget, 0, 2)
	for _, t := range defaultProbeTargets {
		if t.Port == port {
			out = append(out, t)
		}
	}
	return out
}

// probeHTTPClient builds one client per probe so that dial timeouts,
// TLS-skip, and cancellation are self-contained. InsecureSkipVerify is on
// because the whole point of probing is to talk to a server whose cert we
// haven't trusted yet.
func probeHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: probeDialTimeout,
			}).DialContext,
			TLSHandshakeTimeout: probeDialTimeout,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives:   true,
		},
		// Overall per-request ceiling is the dial + read budget.
		Timeout: probeDialTimeout + time.Second,
	}
}

// runProbe fans out probe requests against every candidate target. It
// returns (sorted-deduped candidates, total elapsed). The function never
// returns an error — individual probe failures are swallowed as "not that
// type" rather than bubbling up and confusing the caller.
func runProbe(ctx context.Context, host string, port int, client *http.Client) ([]ProbeCandidate, time.Duration) {
	start := time.Now()
	budget := probeTotalBudget
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	targets := targetsForPort(port)
	if len(targets) == 0 {
		return []ProbeCandidate{}, time.Since(start)
	}

	sem := make(chan struct{}, probeMaxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]ProbeCandidate, 0, len(targets))

	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			if cand, ok := probeOne(ctx, host, target, client); ok {
				mu.Lock()
				results = append(results, cand)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Type != results[j].Type {
			return results[i].Type < results[j].Type
		}
		return results[i].Port < results[j].Port
	})

	return results, time.Since(start)
}

// probeOne runs a single probe. It returns (candidate, true) only when the
// target's identifyFn confirms the product.
func probeOne(ctx context.Context, host string, target probeTarget, client *http.Client) (ProbeCandidate, bool) {
	endpoint := fmt.Sprintf("%s://%s:%d%s", target.Scheme, host, target.Port, target.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ProbeCandidate{}, false
	}
	req.Header.Set("User-Agent", "Pulse/connections-probe")
	req.Header.Set("Accept", "application/json,text/xml,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return ProbeCandidate{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return ProbeCandidate{}, false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return ProbeCandidate{}, false
	}

	match, hints := target.identifyFn(resp, body)
	if !match {
		return ProbeCandidate{}, false
	}

	if hints == nil {
		hints = map[string]string{}
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
		hints["fingerprint"] = "SHA256:" + hex.EncodeToString(sum[:])
	}

	return ProbeCandidate{
		Type:  target.Type,
		Host:  fmt.Sprintf("%s://%s:%d", target.Scheme, host, target.Port),
		Port:  target.Port,
		Hints: hints,
	}, true
}
