package pulsecli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	defaultPulseAPIURL        = "http://127.0.0.1:7655"
	maxPulseAPIResponseBytes  = 1 << 20
	maxPulseAPIErrorBodyChars = 4096
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// RemoteDeps injects transport, credential store, environment, TTY detection,
// and clock dependencies for manifest-backed remote CLI commands.
type RemoteDeps struct {
	HTTPClient HTTPDoer
	Store      *authstore.Store
	Getenv     func(string) string
	IsTerminal func(io.Writer) bool
	Now        func() time.Time
}

func cliHTTPClient(client HTTPDoer) HTTPDoer {
	return cliAPIClient{next: client}
}

type cliAPIClient struct{ next HTTPDoer }

func (c cliAPIClient) Do(req *http.Request) (*http.Response, error) {
	// Mark CLI-origin requests with X-Pulse-Agent-Surface: pulse_cli unless a
	// caller (such as pulse mcp) has already set a more specific surface.
	if req != nil && strings.TrimSpace(req.Header.Get(agentcapabilities.AgentSurfaceHeader)) == "" {
		req = req.Clone(req.Context())
		req.Header = req.Header.Clone()
		req.Header.Set(agentcapabilities.AgentSurfaceHeader, agentcapabilities.AgentSurfacePulseCLI)
	}
	// Fleet, read, and action commands have the same token/body authority
	// boundary as MCP. Reuse that guard rather than maintaining a second
	// redirect policy. Standard clients keep their TLS, timeout and stricter
	// policy; custom HTTPDoers continue to own their transport/redirect handling.
	return agentcapabilities.DoAgentHTTPRequest(c.next, req)
}

func cliGetenv(getenv func(string) string, key string) string {
	if getenv != nil {
		return getenv(key)
	}
	return os.Getenv(key)
}

func writerIsTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

func shouldOutputJSON(cmd *cobra.Command, isTerminal func(io.Writer) bool) bool {
	if flagBoolValue(cmd, "json") {
		return true
	}
	check := isTerminal
	if check == nil {
		check = writerIsTerminal
	}
	var out io.Writer
	if cmd != nil {
		out = cmd.OutOrStdout()
	}
	return !check(out)
}

func flagStringValue(cmd *cobra.Command, name string) string {
	if cmd == nil {
		return ""
	}
	if f := cmd.Flags().Lookup(name); f != nil {
		if val := strings.TrimSpace(f.Value.String()); val != "" || f.Changed {
			return val
		}
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil {
		return strings.TrimSpace(f.Value.String())
	}
	return ""
}

func flagBoolValue(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	if f := cmd.Flags().Lookup(name); f != nil && f.Value.String() == "true" {
		return true
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil && f.Value.String() == "true" {
		return true
	}
	return false
}

func resolveRemoteTarget(cmd *cobra.Command, deps *RemoteDeps) (ResolvedTarget, error) {
	var resolverDeps TargetResolverDeps
	if deps != nil {
		resolverDeps.Store = deps.Store
		resolverDeps.Getenv = deps.Getenv
	}
	apiURLFlag := flagStringValue(cmd, "api-url")
	profileFlag := flagStringValue(cmd, "profile")
	return ResolveTargetDetailed(resolverDeps, apiURLFlag, profileFlag)
}

func resolveLegacyTarget(
	cmd *cobra.Command,
	explicitAPIURL string,
	explicitToken string,
	apiURLChanged bool,
	tokenChanged bool,
	store *authstore.Store,
	getenv func(string) string,
) (string, string, error) {
	profileFlag := flagStringValue(cmd, "profile")
	inheritedAPIURL := ""
	if cmd != nil {
		if f := cmd.InheritedFlags().Lookup("api-url"); f != nil {
			inheritedAPIURL = strings.TrimSpace(f.Value.String())
		}
	}
	envURL := strings.TrimSpace(cliGetenv(getenv, "PULSE_API_URL"))
	envToken := strings.TrimSpace(cliGetenv(getenv, "PULSE_API_TOKEN"))

	var apiURLFlag string
	switch {
	case apiURLChanged:
		apiURLFlag = strings.TrimSpace(explicitAPIURL)
	case inheritedAPIURL != "":
		apiURLFlag = inheritedAPIURL
	}

	canUseProfileStore := store != nil || getenv == nil || profileFlag != ""

	// Explicit --token flag (deprecated compatibility path).
	if tokenChanged && strings.TrimSpace(explicitToken) != "" {
		baseURL := apiURLFlag
		if baseURL == "" {
			baseURL = envURL
		}
		if baseURL == "" && canUseProfileStore {
			if target, err := ResolveTargetDetailed(TargetResolverDeps{Store: store, Getenv: getenv}, "", profileFlag); err == nil && strings.TrimSpace(target.BaseURL) != "" {
				baseURL = target.BaseURL
			}
		}
		if baseURL == "" {
			baseURL = strings.TrimSpace(explicitAPIURL)
		}
		if baseURL == "" {
			baseURL = defaultPulseAPIURL
		}
		return baseURL, strings.TrimSpace(explicitToken), nil
	}

	// Environment token path.
	if envToken != "" {
		baseURL := apiURLFlag
		if baseURL == "" {
			baseURL = envURL
		}
		if baseURL == "" && canUseProfileStore {
			if target, err := ResolveTargetDetailed(TargetResolverDeps{Store: store, Getenv: getenv}, "", profileFlag); err == nil && strings.TrimSpace(target.BaseURL) != "" {
				baseURL = target.BaseURL
			}
		}
		if baseURL == "" {
			baseURL = strings.TrimSpace(explicitAPIURL)
		}
		if baseURL == "" {
			baseURL = defaultPulseAPIURL
		}
		return baseURL, envToken, nil
	}

	// Saved profile store fallback (default in production or when Store/profile is provided).
	if canUseProfileStore {
		target, err := ResolveTargetDetailed(TargetResolverDeps{Store: store, Getenv: getenv}, apiURLFlag, profileFlag)
		if err == nil && strings.TrimSpace(target.Token) != "" {
			baseURL := strings.TrimSpace(target.BaseURL)
			if baseURL == "" {
				baseURL = defaultPulseAPIURL
			}
			return baseURL, strings.TrimSpace(target.Token), nil
		}
		if profileFlag != "" && err != nil {
			return "", "", err
		}
	}

	return "", "", fmt.Errorf("api token is required (run `pulse auth login <url>` or set PULSE_API_TOKEN)")
}

func pulseAPIEndpoint(raw, apiPath string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("api url is required (use --api-url or PULSE_API_URL)")
	}
	if !strings.HasPrefix(apiPath, "/") {
		return nil, fmt.Errorf("api path must start with /")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid api url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid api url: scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid api url: host is required")
	}

	apiPath = strings.TrimRight(apiPath, "/")
	fullPath := "/api" + apiPath
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = fullPath
	case path == fullPath || strings.HasSuffix(path, fullPath):
		parsed.Path = path
	case path == "/api" || strings.HasSuffix(path, "/api"):
		parsed.Path = path + apiPath
	default:
		parsed.Path = path + fullPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

type licenseRequiredErrorPayload struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Feature    string `json:"feature"`
	UpgradeURL string `json:"upgrade_url"`
}

// formatLicenseRequiredError renders an HTTP 402 WriteLicenseRequired response
// cleanly for terminal operators without retrying.
func formatLicenseRequiredError(operation string, status string, body []byte) error {
	if strings.TrimSpace(operation) == "" {
		operation = "request"
	}
	var payload licenseRequiredErrorPayload
	if err := json.Unmarshal(body, &payload); err == nil && (strings.TrimSpace(payload.Message) != "" || strings.TrimSpace(payload.Feature) != "") {
		msg := strings.TrimSpace(payload.Message)
		feature := strings.TrimSpace(payload.Feature)
		upgrade := strings.TrimSpace(payload.UpgradeURL)

		var b strings.Builder
		b.WriteString(operation)
		b.WriteString(" requires a Pulse Pro license")
		if feature != "" {
			b.WriteString(" (")
			b.WriteString(feature)
			b.WriteString(")")
		}
		if msg != "" {
			b.WriteString(": ")
			b.WriteString(msg)
		}
		if upgrade != "" {
			b.WriteString(". Upgrade: ")
			b.WriteString(upgrade)
		}
		return fmt.Errorf("%s", b.String())
	}
	return apiStatusError(operation, status, body)
}

func apiStatusErrorWithCode(operation string, statusCode int, status string, body []byte) error {
	if statusCode == http.StatusPaymentRequired {
		return formatLicenseRequiredError(operation, status, body)
	}
	return apiStatusError(operation, status, body)
}

func formatCapabilityHTTPError(operation string, resp agentcapabilities.HTTPCallResponse) error {
	if resp.OK() {
		return nil
	}
	if resp.StatusCode == http.StatusPaymentRequired {
		return formatLicenseRequiredError(operation, fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)), resp.Body)
	}
	return resp.FailureError()
}

func apiStatusError(operation, status string, body []byte) error {
	if strings.TrimSpace(operation) == "" {
		operation = "request"
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s failed: %s", operation, status)
	}
	if len(message) > maxPulseAPIErrorBodyChars {
		message = message[:maxPulseAPIErrorBodyChars] + "..."
	}
	return fmt.Errorf("%s failed: %s: %s", operation, status, message)
}

func writePrettyJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func writePrettyJSONBytes(w io.Writer, raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, trimmed, "", "  "); err != nil {
		if _, writeErr := w.Write(trimmed); writeErr != nil {
			return writeErr
		}
		_, writeErr := io.WriteString(w, "\n")
		return writeErr
	}
	indented.WriteByte('\n')
	_, err := w.Write(indented.Bytes())
	return err
}

func decodeJSONBytes(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid trailing JSON content")
		}
		return err
	}
	return nil
}

func decodeJSONString(data string, out any) error {
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid trailing JSON content")
		}
		return err
	}
	return nil
}
