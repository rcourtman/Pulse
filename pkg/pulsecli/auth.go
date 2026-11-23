package pulsecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	pulseCLIAgentSurface   = "pulse_cli"
	defaultCLITimeout      = 15 * time.Second
	maxStdinTokenBytes     = 8 * 1024
	maxFleetContextBytes   = 16 << 20 // matches agentcapabilities capability body limit
	authScopeReadOnly      = "read-only"
	authScopePropose       = "propose"
	externalAgentsNavLabel = "Settings -> Pulse Intelligence -> External agents"
)

// AuthDeps allows deterministic testing of `pulse auth` subcommands without
// hitting the host keychain, terminal, or network. All fields are optional.
type AuthDeps struct {
	HTTPClient   HTTPDoer
	Store        *authstore.Store
	Getenv       func(string) string
	ReadPassword func(int) ([]byte, error)
	IsTerminal   func(int) bool
	Now          func() time.Time
}

// TargetResolverDeps allows deterministic testing of ResolveTargetDetailed.
type TargetResolverDeps struct {
	Store  *authstore.Store
	Getenv func(string) string
}

// ResolvedTarget describes the resolved Pulse base URL, token, and where each
// value originated.
type ResolvedTarget struct {
	BaseURL      string
	Token        string
	URLSource    string
	TokenSource  string
	ProfileName  string
	TokenBackend string
}

// ResolveTarget resolves the Pulse base URL and API token using canonical
// precedence:
//   - PULSE_API_URL / PULSE_API_TOKEN environment variables
//   - --api-url flag (URL only; tokens are never accepted on argv)
//   - saved authstore profile (--profile or default)
func ResolveTarget(apiURLFlag, profileName string) (baseURL, token string, err error) {
	resolved, err := ResolveTargetDetailed(TargetResolverDeps{}, apiURLFlag, profileName)
	if err != nil {
		return "", "", err
	}
	return resolved.BaseURL, resolved.Token, nil
}

// ResolveTargetDetailed resolves the Pulse base URL and API token along with
// provenance metadata for status and doctor diagnostics.
func ResolveTargetDetailed(deps TargetResolverDeps, apiURLFlag, profileName string) (ResolvedTarget, error) {
	var res ResolvedTarget

	envURL := strings.TrimSpace(cliGetenv(deps.Getenv, "PULSE_API_URL"))
	envToken := strings.TrimSpace(cliGetenv(deps.Getenv, "PULSE_API_TOKEN"))
	flagURL := strings.TrimSpace(apiURLFlag)
	requestedProfile := strings.TrimSpace(profileName)

	if envURL != "" {
		clean, err := authstore.NormalizeURL(envURL)
		if err != nil {
			return res, fmt.Errorf("invalid PULSE_API_URL: %w", err)
		}
		res.BaseURL = clean
		res.URLSource = "env"
	} else if flagURL != "" {
		clean, err := authstore.NormalizeURL(flagURL)
		if err != nil {
			return res, fmt.Errorf("invalid --api-url: %w", err)
		}
		res.BaseURL = clean
		res.URLSource = "flag"
	}

	if envToken != "" {
		res.Token = envToken
		res.TokenSource = "env"
	}

	needProfile := res.BaseURL == "" || res.Token == "" || requestedProfile != ""
	if needProfile {
		store, err := resolveAuthStore(deps.Store)
		if err != nil {
			return res, err
		}

		if res.Token == "" {
			resolvedName, profile, tok, tokErr := store.GetToken(requestedProfile)
			if tokErr != nil {
				if requestedProfile != "" || !errors.Is(tokErr, authstore.ErrProfileNotFound) {
					if errors.Is(tokErr, authstore.ErrProfileNotFound) {
						return res, fmt.Errorf("profile %q not found. Run \"pulse auth login <url> --profile %s\"", requestedProfile, requestedProfile)
					}
					return res, tokErr
				}
			} else {
				res.ProfileName = resolvedName
				res.TokenBackend = profile.TokenBackend
				res.Token = tok
				res.TokenSource = "profile"
				if res.BaseURL == "" {
					res.BaseURL = profile.URL
					res.URLSource = "profile"
				}
			}
		} else {
			// Token came from PULSE_API_TOKEN; only read profile metadata if URL
			// is missing or a specific profile was requested.
			resolvedName, profile, profErr := store.GetProfile(requestedProfile)
			if profErr != nil {
				if requestedProfile != "" {
					if errors.Is(profErr, authstore.ErrProfileNotFound) {
						return res, fmt.Errorf("profile %q not found. Run \"pulse auth login <url> --profile %s\"", requestedProfile, requestedProfile)
					}
					return res, profErr
				}
			} else {
				res.ProfileName = resolvedName
				res.TokenBackend = profile.TokenBackend
				if res.BaseURL == "" {
					res.BaseURL = profile.URL
					res.URLSource = "profile"
				}
			}
		}
	}

	if res.BaseURL == "" {
		return res, fmt.Errorf("Pulse URL is required (run \"pulse auth login <url>\", pass --api-url, or set PULSE_API_URL)")
	}
	if res.Token == "" {
		return res, fmt.Errorf("Pulse API token is required (run \"pulse auth login <url>\" or set PULSE_API_TOKEN)")
	}
	return res, nil
}

// NewAuthCommand constructs the `pulse auth` command tree (`login`, `logout`, `status`).
func NewAuthCommand(deps *AuthDeps) *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Pulse CLI authentication profiles and credentials",
	}
	authCmd.AddCommand(newAuthLoginCmd(deps))
	authCmd.AddCommand(newAuthLogoutCmd(deps))
	authCmd.AddCommand(newAuthStatusCmd(deps))
	return authCmd
}

type authLoginOptions struct {
	Profile    string
	TokenStdin bool
	Scope      string
	JSON       bool
}

type authLoginSummary struct {
	Profile         string `json:"profile"`
	URL             string `json:"url"`
	Scope           string `json:"scope"`
	TokenBackend    string `json:"tokenBackend"`
	CredentialsPath string `json:"credentialsPath,omitempty"`
	ManifestVersion string `json:"manifestVersion"`
	CapabilityCount int    `json:"capabilityCount"`
	ResourceCount   int    `json:"resourceCount"`
}

func newAuthLoginCmd(deps *AuthDeps) *cobra.Command {
	opts := authLoginOptions{
		Scope: authScopeReadOnly,
	}

	cmd := &cobra.Command{
		Use:   "login <url>",
		Short: "Authenticate against a Pulse instance and save credentials securely",
		Long: "Validate reachability and a Pulse API token against /api/agent/capabilities and " +
			"/api/agent/fleet-context, then save the profile metadata and secret in your OS keychain " +
			"(with an automatic 0600 credentials file fallback on headless hosts). Tokens are read " +
			"from a hidden prompt or --token-stdin and are never accepted on argv.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = resolveFlagString(cmd, "profile", opts.Profile)
			opts.JSON = resolveFlagBool(cmd, "json", opts.JSON)
			return runAuthLogin(cmd, deps, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Profile name to save (defaults to \"default\")")
	cmd.Flags().BoolVar(&opts.TokenStdin, "token-stdin", false, "Read API token from stdin instead of an interactive prompt")
	cmd.Flags().StringVar(&opts.Scope, "scope", authScopeReadOnly, "Intended token scope preset: read-only (default) or propose")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output login summary as JSON")
	return cmd
}

func runAuthLogin(cmd *cobra.Command, deps *AuthDeps, rawURL string, opts authLoginOptions) error {
	cleanURL, err := authstore.NormalizeURL(rawURL)
	if err != nil {
		return err
	}
	profileName, err := authstore.NormalizeProfileName(opts.Profile)
	if err != nil {
		return err
	}
	scope, err := normalizeAuthScope(opts.Scope)
	if err != nil {
		return err
	}

	if strings.HasPrefix(cleanURL, "http://") && !isLoopbackURL(cleanURL) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s uses unencrypted HTTP. HTTPS is recommended for non-local Pulse instances.\n", cleanURL)
	}

	token, err := readLoginToken(cmd, deps, cleanURL, scope, opts.TokenStdin)
	if err != nil {
		return err
	}

	client := pulseCLISurfaceClient(authHTTPClient(deps))
	manifest, err := agentcapabilities.FetchManifest(cmd.Context(), client, cleanURL)
	if err != nil {
		return fmt.Errorf("failed to reach Pulse capabilities at %s%s: %w", cleanURL, agentcapabilities.AgentCapabilitiesPath, err)
	}

	probe, err := probeFleetContext(cmd.Context(), client, cleanURL, token)
	if err != nil {
		return err
	}

	store, err := resolveAuthStore(authDepsStore(deps))
	if err != nil {
		return err
	}
	saved, err := store.SaveProfile(profileName, cleanURL, token, scope)
	if err != nil {
		return fmt.Errorf("save profile %q: %w", profileName, err)
	}

	summary := authLoginSummary{
		Profile:         profileName,
		URL:             saved.URL,
		Scope:           scope,
		TokenBackend:    saved.TokenBackend,
		ManifestVersion: manifest.Version,
		CapabilityCount: len(manifest.Capabilities),
		ResourceCount:   probe.ResourceCount,
	}
	if saved.TokenBackend == authstore.BackendFile {
		summary.CredentialsPath = store.CredentialsPath()
	}

	if opts.JSON {
		return writeJSONIndent(cmd.OutOrStdout(), summary)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Logged in to %s (profile %q)\n", summary.URL, summary.Profile)
	fmt.Fprintf(out, "  Scope preset: %s (%s)\n", summary.Scope, scopePresetDescription(summary.Scope))
	if summary.TokenBackend == authstore.BackendFile {
		fmt.Fprintf(out, "  Credentials:  %s (%s, 0600)\n", summary.TokenBackend, summary.CredentialsPath)
	} else {
		fmt.Fprintf(out, "  Credentials:  %s (OS keychain)\n", summary.TokenBackend)
	}
	fmt.Fprintf(out, "  Verified:     manifest %s (%d capabilities), %d visible resources\n",
		summary.ManifestVersion, summary.CapabilityCount, summary.ResourceCount)
	if profileName == authstore.DefaultProfileName {
		fmt.Fprintf(out, "\nNext: pulse connect claude-code   (or codex, opencode, claude-desktop)\n")
	} else {
		fmt.Fprintf(out, "\nNext: pulse connect claude-code --profile %s\n", profileName)
	}
	return nil
}

type authLogoutOptions struct {
	Profile string
	JSON    bool
}

type authLogoutSummary struct {
	Profile string `json:"profile"`
	Removed bool   `json:"removed"`
}

func newAuthLogoutCmd(deps *AuthDeps) *cobra.Command {
	var opts authLogoutOptions

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove a saved Pulse profile and delete its stored API token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = resolveFlagString(cmd, "profile", opts.Profile)
			opts.JSON = resolveFlagBool(cmd, "json", opts.JSON)
			return runAuthLogout(cmd, deps, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Profile name to remove (defaults to active default profile)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output result as JSON")
	return cmd
}

func runAuthLogout(cmd *cobra.Command, deps *AuthDeps, opts authLogoutOptions) error {
	store, err := resolveAuthStore(authDepsStore(deps))
	if err != nil {
		return err
	}

	targetName, _, err := store.GetProfile(opts.Profile)
	if err != nil {
		if errors.Is(err, authstore.ErrProfileNotFound) {
			if strings.TrimSpace(opts.Profile) == "" {
				return fmt.Errorf("no saved Pulse profile found")
			}
			return fmt.Errorf("profile %q not found", opts.Profile)
		}
		return err
	}

	if err := store.DeleteProfile(targetName); err != nil {
		return err
	}

	summary := authLogoutSummary{
		Profile: targetName,
		Removed: true,
	}
	if opts.JSON {
		return writeJSONIndent(cmd.OutOrStdout(), summary)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Logged out profile %q and removed stored credentials.\n", targetName)
	return nil
}

type authStatusOptions struct {
	Profile string
	JSON    bool
}

type authStatusProfileView struct {
	Name         string    `json:"name"`
	Default      bool      `json:"default"`
	URL          string    `json:"url"`
	Scope        string    `json:"scope,omitempty"`
	TokenBackend string    `json:"tokenBackend"`
	TokenStored  bool      `json:"tokenStored"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type authStatusOutput struct {
	ProfilesPath   string                  `json:"profilesPath"`
	DefaultProfile string                  `json:"defaultProfile,omitempty"`
	EnvURLSet      bool                    `json:"envUrlSet"`
	EnvTokenSet    bool                    `json:"envTokenSet"`
	ActiveURL      string                  `json:"activeUrl,omitempty"`
	URLSource      string                  `json:"urlSource,omitempty"`
	TokenSource    string                  `json:"tokenSource,omitempty"`
	ActiveProfile  string                  `json:"activeProfile,omitempty"`
	Profiles       []authStatusProfileView `json:"profiles"`
}

func newAuthStatusCmd(deps *AuthDeps) *cobra.Command {
	var opts authStatusOptions

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show saved Pulse profiles and active credential resolution",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = resolveFlagString(cmd, "profile", opts.Profile)
			opts.JSON = resolveFlagBool(cmd, "json", opts.JSON)
			return runAuthStatus(cmd, deps, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Profile name to inspect")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output status as JSON")
	return cmd
}

func runAuthStatus(cmd *cobra.Command, deps *AuthDeps, opts authStatusOptions) error {
	store, err := resolveAuthStore(authDepsStore(deps))
	if err != nil {
		return err
	}
	getenv := authDepsGetenv(deps)

	defaultName, profiles, err := store.ListProfiles()
	if err != nil {
		return err
	}

	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	views := make([]authStatusProfileView, 0, len(names))
	for _, name := range names {
		p := profiles[name]
		_, _, _, tokErr := store.GetToken(name)
		views = append(views, authStatusProfileView{
			Name:         name,
			Default:      name == defaultName,
			URL:          p.URL,
			Scope:        p.Scope,
			TokenBackend: p.TokenBackend,
			TokenStored:  tokErr == nil,
			UpdatedAt:    p.UpdatedAt,
		})
	}

	status := authStatusOutput{
		ProfilesPath:   store.ProfilesPath(),
		DefaultProfile: defaultName,
		EnvURLSet:      strings.TrimSpace(cliGetenv(getenv, "PULSE_API_URL")) != "",
		EnvTokenSet:    strings.TrimSpace(cliGetenv(getenv, "PULSE_API_TOKEN")) != "",
		Profiles:       views,
	}

	if resolved, resErr := ResolveTargetDetailed(TargetResolverDeps{
		Store:  store,
		Getenv: getenv,
	}, "", opts.Profile); resErr == nil {
		status.ActiveURL = resolved.BaseURL
		status.URLSource = resolved.URLSource
		status.TokenSource = resolved.TokenSource
		status.ActiveProfile = resolved.ProfileName
	}

	if opts.JSON {
		return writeJSONIndent(cmd.OutOrStdout(), status)
	}

	out := cmd.OutOrStdout()
	if status.ActiveURL != "" {
		fmt.Fprintf(out, "Active target: %s (url: %s, token: %s)\n", status.ActiveURL, status.URLSource, status.TokenSource)
	} else {
		fmt.Fprintf(out, "Active target: not configured (run \"pulse auth login <url>\")\n")
	}
	if status.EnvURLSet || status.EnvTokenSet {
		fmt.Fprintf(out, "Environment overrides: PULSE_API_URL=%v, PULSE_API_TOKEN=%v\n", status.EnvURLSet, status.EnvTokenSet)
	}
	fmt.Fprintf(out, "Profiles file: %s\n", status.ProfilesPath)
	if len(status.Profiles) == 0 {
		fmt.Fprintf(out, "Saved profiles: none\n")
		return nil
	}
	fmt.Fprintf(out, "Saved profiles:\n")
	for _, view := range status.Profiles {
		marker := " "
		if view.Default {
			marker = "*"
		}
		tokenState := "stored"
		if !view.TokenStored {
			tokenState = "missing"
		}
		scopeSuffix := ""
		if view.Scope != "" {
			scopeSuffix = ", scope=" + view.Scope
		}
		fmt.Fprintf(out, "  %s %s  %s  (backend=%s, token=%s%s)\n",
			marker, view.Name, view.URL, view.TokenBackend, tokenState, scopeSuffix)
	}
	return nil
}

func normalizeAuthScope(raw string) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(raw))
	if scope == "" {
		return authScopeReadOnly, nil
	}
	switch scope {
	case authScopeReadOnly, authScopePropose:
		return scope, nil
	default:
		return "", fmt.Errorf("invalid --scope %q: must be %q or %q", raw, authScopeReadOnly, authScopePropose)
	}
}

func scopePresetDescription(scope string) string {
	switch scope {
	case authScopePropose:
		return "monitoring:read, actions:plan"
	default:
		return "monitoring:read"
	}
}

func readLoginToken(cmd *cobra.Command, deps *AuthDeps, baseURL, scope string, fromStdin bool) (string, error) {
	if fromStdin {
		raw, err := ReadBoundedHTTPBody(cmd.InOrStdin(), -1, maxStdinTokenBytes, "API token stdin")
		if err != nil {
			return "", err
		}
		token := strings.TrimSpace(string(raw))
		if token == "" {
			return "", fmt.Errorf("no API token received on stdin")
		}
		if strings.ContainsAny(token, "\r\n") {
			return "", fmt.Errorf("API token on stdin must be a single line")
		}
		return token, nil
	}

	isTerm := term.IsTerminal
	if deps != nil && deps.IsTerminal != nil {
		isTerm = deps.IsTerminal
	}
	fd := int(os.Stdin.Fd())
	if !isTerm(fd) {
		return "", fmt.Errorf("stdin is not a terminal. Run interactively or pass --token-stdin")
	}

	presetLabel := "External agent (read-only)"
	if scope == authScopePropose {
		presetLabel = "External agent (propose fixes)"
	}
	errOut := cmd.ErrOrStderr()
	fmt.Fprintf(errOut, "Create a token in %s (%s) under %s.\n", baseURL, presetLabel, externalAgentsNavLabel)
	fmt.Fprintf(errOut, "Enter Pulse API token: ")

	readPass := term.ReadPassword
	if deps != nil && deps.ReadPassword != nil {
		readPass = deps.ReadPassword
	}
	raw, err := readPass(fd)
	fmt.Fprintln(errOut)
	if err != nil {
		return "", fmt.Errorf("read API token: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("API token must not be empty")
	}
	return token, nil
}

type fleetContextProbeResult struct {
	StatusCode    int
	ResourceCount int
}

type fleetContextSummaryPayload struct {
	Resources []json.RawMessage `json:"resources"`
}

func probeFleetContext(ctx context.Context, client HTTPDoer, baseURL, token string) (fleetContextProbeResult, error) {
	req, err := agentcapabilities.NewAgentHTTPRequest(ctx, http.MethodGet, baseURL, agentcapabilities.FleetContextCapabilityPath, token, nil)
	if err != nil {
		return fleetContextProbeResult{}, fmt.Errorf("build fleet-context request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(agentcapabilities.AgentSurfaceHeader, pulseCLIAgentSurface)

	resp, err := client.Do(req)
	if err != nil {
		return fleetContextProbeResult{}, fmt.Errorf("GET %s failed: %w", agentcapabilities.FleetContextCapabilityPath, err)
	}
	defer resp.Body.Close()

	body, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxFleetContextBytes, "fleet-context response")
	if err != nil {
		return fleetContextProbeResult{StatusCode: resp.StatusCode}, fmt.Errorf("read %s response: %w", agentcapabilities.FleetContextCapabilityPath, err)
	}

	result := fleetContextProbeResult{StatusCode: resp.StatusCode}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return result, fmt.Errorf("authentication failed (HTTP 401): token was rejected by Pulse. Verify the token in %s", externalAgentsNavLabel)
	case resp.StatusCode == http.StatusForbidden:
		return result, fmt.Errorf("authorization failed (HTTP 403): token lacks required monitoring:read scope. Create a read-only token in %s", externalAgentsNavLabel)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return result, apiStatusError("GET "+agentcapabilities.FleetContextCapabilityPath, resp.Status, body)
	}

	var payload fleetContextSummaryPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return result, fmt.Errorf("decode %s response: %w", agentcapabilities.FleetContextCapabilityPath, err)
	}
	result.ResourceCount = len(payload.Resources)
	return result, nil
}

type pulseCLISurfaceDoer struct {
	next HTTPDoer
}

func (d pulseCLISurfaceDoer) Do(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header.Get(agentcapabilities.AgentSurfaceHeader) == "" {
		cloned.Header.Set(agentcapabilities.AgentSurfaceHeader, pulseCLIAgentSurface)
	}
	return d.next.Do(cloned)
}

func pulseCLISurfaceClient(client HTTPDoer) HTTPDoer {
	if client == nil {
		client = &http.Client{Timeout: defaultCLITimeout}
	}
	return pulseCLISurfaceDoer{next: cliHTTPClient(client)}
}

func authHTTPClient(deps *AuthDeps) HTTPDoer {
	if deps != nil && deps.HTTPClient != nil {
		return deps.HTTPClient
	}
	return nil
}

func authDepsStore(deps *AuthDeps) *authstore.Store {
	if deps != nil {
		return deps.Store
	}
	return nil
}

func authDepsGetenv(deps *AuthDeps) func(string) string {
	if deps != nil {
		return deps.Getenv
	}
	return nil
}

func resolveAuthStore(store *authstore.Store) (*authstore.Store, error) {
	if store != nil {
		return store, nil
	}
	return authstore.NewDefaultStore()
}

func resolveFlagString(cmd *cobra.Command, name, localVal string) string {
	if cmd == nil {
		return localVal
	}
	if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
		return localVal
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil && f.Changed {
		return f.Value.String()
	}
	if root := cmd.Root(); root != nil && root != cmd {
		if f := root.PersistentFlags().Lookup(name); f != nil && f.Changed {
			return f.Value.String()
		}
	}
	return localVal
}

func resolveFlagBool(cmd *cobra.Command, name string, localVal bool) bool {
	if cmd == nil {
		return localVal
	}
	if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
		return localVal
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil && f.Changed {
		return f.Value.String() == "true"
	}
	if root := cmd.Root(); root != nil && root != cmd {
		if f := root.PersistentFlags().Lookup(name); f != nil && f.Changed {
			return f.Value.String() == "true"
		}
	}
	return localVal
}

func isLoopbackURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func writeJSONIndent(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
