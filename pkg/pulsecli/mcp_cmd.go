package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
)

const (
	PulseMCPServerName    = "pulse-mcp"
	PulseMCPServerVersion = "0.1.0"
)

// MCPDeps injects optional transport, store, env, and stdio handles for `pulse mcp`.
type MCPDeps struct {
	HTTPClient HTTPDoer
	Store      *authstore.Store
	Getenv     func(string) string
	In         io.Reader
	Out        io.Writer
	ErrOut     io.Writer
}

// RunMCPStdioOptions configures the shared MCP stdio adapter runner used by
// both `pulse mcp` and the compatibility `cmd/pulse-mcp` binary.
type RunMCPStdioOptions struct {
	BaseURL           string
	BaseURLSet        bool
	Profile           string
	TokenEnv          string
	EmitNotifications bool
	Store             *authstore.Store
	Getenv            func(string) string
	HTTPClient        agentcapabilities.HTTPDoer
	ManifestClient    agentcapabilities.HTTPDoer
	In                io.Reader
	Out               io.Writer
	ErrOut            io.Writer
}

// NewMCPCommand creates the built-in `pulse mcp` stdio MCP server command.
// Credentials resolve from environment variables or a saved authstore profile
// (OS keychain or 0600 credentials fallback), so MCP client configs never need
// plaintext tokens.
func NewMCPCommand(deps *MCPDeps) *cobra.Command {
	var baseURLFlag string
	var tokenEnvFlag string
	var profileFlag string
	var emitNotifications bool

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the Pulse MCP stdio server for external agents",
		Long:  "Run a line-delimited JSON-RPC 2.0 Model Context Protocol (MCP) server over stdio backed by the target Pulse instance's /api/agent/capabilities manifest. Credentials are read from PULSE_API_TOKEN/PULSE_API_URL or your saved `pulse auth login` profile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			effectiveURL := strings.TrimSpace(baseURLFlag)
			urlSet := cmd.Flags().Changed("base-url")
			if !urlSet {
				if inherited := flagStringValue(cmd, "api-url"); inherited != "" {
					effectiveURL = inherited
					urlSet = true
				}
			}
			effectiveProfile := strings.TrimSpace(profileFlag)
			if effectiveProfile == "" {
				effectiveProfile = flagStringValue(cmd, "profile")
			}

			opts := RunMCPStdioOptions{
				BaseURL:           effectiveURL,
				BaseURLSet:        urlSet,
				Profile:           effectiveProfile,
				TokenEnv:          tokenEnvFlag,
				EmitNotifications: emitNotifications,
				In:                cmd.InOrStdin(),
				Out:               cmd.OutOrStdout(),
				ErrOut:            cmd.ErrOrStderr(),
			}
			if deps != nil {
				opts.Store = deps.Store
				opts.Getenv = deps.Getenv
				if deps.HTTPClient != nil {
					opts.HTTPClient = deps.HTTPClient
					opts.ManifestClient = deps.HTTPClient
				}
				if deps.In != nil {
					opts.In = deps.In
				}
				if deps.Out != nil {
					opts.Out = deps.Out
				}
				if deps.ErrOut != nil {
					opts.ErrOut = deps.ErrOut
				}
			}
			return RunMCPStdio(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&baseURLFlag, "base-url", agentcapabilities.DefaultMCPAdapterDefaultBaseURL, "Pulse base URL (overrides saved profile, PULSE_API_URL takes precedence)")
	cmd.Flags().StringVar(&tokenEnvFlag, "token-env", agentcapabilities.DefaultMCPAdapterTokenEnv, "Env var holding the Pulse API token (optional when a saved profile exists)")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "Saved auth profile name (defaults to active default profile)")
	cmd.Flags().BoolVar(&emitNotifications, "emit-notifications", false, "Translate Pulse SSE events into JSON-RPC notifications on stdout")
	return cmd
}

// RunMCPStdio resolves target credentials, fetches the canonical capabilities
// manifest, and serves line-delimited JSON-RPC 2.0 MCP traffic on stdio.
func RunMCPStdio(ctx context.Context, opts RunMCPStdioOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := opts.ErrOut
	if errOut == nil {
		errOut = os.Stderr
	}
	logger := log.New(errOut, "pulse-mcp ", log.LstdFlags)

	tokenEnv := strings.TrimSpace(opts.TokenEnv)
	if tokenEnv == "" {
		tokenEnv = agentcapabilities.DefaultMCPAdapterTokenEnv
	}

	baseURL, token := resolveMCPTarget(opts, tokenEnv)

	manifestClient := opts.ManifestClient
	if manifestClient == nil {
		manifestClient = &http.Client{Timeout: 10 * time.Second}
	}
	manifest, err := agentcapabilities.FetchManifest(ctx, PulseMCPAgentSurfaceHTTPDoer{Next: manifestClient}, baseURL)
	if err != nil {
		return fmt.Errorf("could not fetch capabilities manifest from %s: %w", baseURL, err)
	}
	logger.Printf("fetched manifest %s with %d capabilities from %s", manifest.Version, len(manifest.Capabilities), baseURL)

	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("%s", MissingMCPAPITokenMessage(tokenEnv, manifest))
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	server := &MCPStdioServer{
		BaseURL:           baseURL,
		Token:             token,
		Manifest:          manifest,
		HTTP:              httpClient,
		EmitNotifications: opts.EmitNotifications,
		Logger:            logger,
	}
	server.Serve(ctx, in, out)
	return nil
}

func resolveMCPTarget(opts RunMCPStdioOptions, tokenEnv string) (string, string) {
	urlFlag := ""
	if opts.BaseURLSet {
		urlFlag = strings.TrimSpace(opts.BaseURL)
	}
	customToken := ""
	if tokenEnv != "" && tokenEnv != agentcapabilities.DefaultMCPAdapterTokenEnv {
		customToken = strings.TrimSpace(cliGetenv(opts.Getenv, tokenEnv))
	}

	var baseURL, token string
	if target, err := ResolveTargetDetailed(TargetResolverDeps{
		Store:  opts.Store,
		Getenv: opts.Getenv,
	}, urlFlag, strings.TrimSpace(opts.Profile)); err == nil {
		baseURL = strings.TrimSpace(target.BaseURL)
		token = strings.TrimSpace(target.Token)
	}

	if customToken != "" {
		token = customToken
	}
	if token == "" {
		token = strings.TrimSpace(cliGetenv(opts.Getenv, tokenEnv))
	}
	if baseURL == "" {
		if envURL := strings.TrimSpace(cliGetenv(opts.Getenv, "PULSE_API_URL")); envURL != "" {
			baseURL = envURL
		} else if trimmed := strings.TrimSpace(opts.BaseURL); trimmed != "" {
			baseURL = trimmed
		} else {
			baseURL = agentcapabilities.DefaultMCPAdapterDefaultBaseURL
		}
	}
	return baseURL, token
}

// MissingMCPAPITokenMessage builds the startup error message when neither an
// env token nor a saved auth profile provides a Pulse API token.
func MissingMCPAPITokenMessage(tokenEnv string, manifest *agentcapabilities.Manifest) string {
	scopeList := agentcapabilities.ManifestRequiredScopeList(manifest)
	if scopeList == "" {
		return fmt.Sprintf("env var %s is empty and no saved profile was found. Run `pulse auth login <url>` or set %s for the manifest tools you enable", tokenEnv, tokenEnv)
	}
	return fmt.Sprintf("env var %s is empty and no saved profile was found. Run `pulse auth login <url>` or mint an API token with the manifest scopes for the tools you enable (current manifest scopes: %s)", tokenEnv, scopeList)
}

// PulseMCPAgentSurfaceHTTPDoer tags outgoing MCP adapter HTTP requests with
// X-Pulse-Agent-Surface: pulse_mcp and applies the shared redirect boundary.
type PulseMCPAgentSurfaceHTTPDoer struct {
	Next agentcapabilities.HTTPDoer
}

func (d PulseMCPAgentSurfaceHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	if req != nil {
		req = req.Clone(req.Context())
		req.Header = req.Header.Clone()
		req.Header.Set(agentcapabilities.AgentSurfaceHeader, agentcapabilities.AgentSurfacePulseMCP)
	}
	return agentcapabilities.DoAgentHTTPRequest(d.Next, req)
}

// MCPStdioServer holds per-session state for the Pulse MCP stdio adapter.
// All MCP method semantics, tool projection, and SSE notification projection
// live in internal/agentcapabilities.
type MCPStdioServer struct {
	BaseURL           string
	Token             string
	Manifest          *agentcapabilities.Manifest
	HTTP              agentcapabilities.HTTPDoer
	EmitNotifications bool
	Logger            *log.Logger
	Out               io.Writer

	mu                sync.Mutex
	notificationsOnce sync.Once
}

func (s *MCPStdioServer) logf(format string, args ...any) {
	if s != nil && s.Logger != nil {
		s.Logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Serve runs the stdio loop: read line-delimited JSON-RPC requests from in,
// dispatch through agentcapabilities, and write responses to out.
func (s *MCPStdioServer) Serve(ctx context.Context, in io.Reader, out io.Writer) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.Out = out
	if err := agentcapabilities.ServeJSONRPCLines(
		ctx,
		in,
		func(reqCtx context.Context, req agentcapabilities.JSONRPCRequest) agentcapabilities.JSONRPCResponse {
			return s.Dispatch(reqCtx, &req)
		},
		func(resp agentcapabilities.JSONRPCResponse) error {
			return s.WriteJSON(out, resp)
		},
	); err != nil {
		s.logf("stdio JSON-RPC: %v", err)
	}
}

// Dispatch delegates MCP method semantics to the shared Pulse Intelligence
// boundary. This adapter owns session policy and stdout serialization only.
func (s *MCPStdioServer) Dispatch(ctx context.Context, req *agentcapabilities.JSONRPCRequest) agentcapabilities.JSONRPCResponse {
	return agentcapabilities.DispatchMCPToolServerRequest(ctx, *req, s.ManifestToolServer().Handlers(func() {
		// Start the SSE-to-notifications bridge once per process,
		// only if the operator opted in. Spawned after the
		// initialize response is queued so the client sees the
		// handshake reply before any notification arrives.
		if s.EmitNotifications {
			s.notificationsOnce.Do(func() {
				go s.ConsumeSSEEvents(ctx)
			})
		}
	}))
}

// ManifestToolServer builds the shared agentcapabilities manifest tool server.
func (s *MCPStdioServer) ManifestToolServer() agentcapabilities.MCPManifestToolServer {
	var manifest agentcapabilities.Manifest
	if s.Manifest != nil {
		manifest = *s.Manifest
	}
	return agentcapabilities.MCPManifestToolServer{
		ServerName:                   PulseMCPServerName,
		ServerVersion:                PulseMCPServerVersion,
		SurfaceID:                    agentcapabilities.SurfaceIDPulseMCP,
		EmitPulseNotifications:       s.EmitNotifications,
		Client:                       s.pulseMCPHTTPClient(),
		BaseURL:                      s.BaseURL,
		Token:                        s.Token,
		Manifest:                     manifest,
		RecordWorkflowPromptActivity: s.RecordWorkflowPromptActivity,
	}
}

func (s *MCPStdioServer) pulseMCPHTTPClient() agentcapabilities.HTTPDoer {
	if s == nil {
		return PulseMCPAgentSurfaceHTTPDoer{}
	}
	return PulseMCPAgentSurfaceHTTPDoer{Next: s.HTTP}
}

// RecordWorkflowPromptActivity posts content-free prompt-starter usage to Pulse.
func (s *MCPStdioServer) RecordWorkflowPromptActivity(ctx context.Context, promptName string) {
	promptName = strings.TrimSpace(promptName)
	if s == nil || promptName == "" || strings.TrimSpace(s.BaseURL) == "" || strings.TrimSpace(s.Token) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	body, err := json.Marshal(map[string]string{"name": promptName})
	if err != nil {
		s.logf("workflow prompt activity: marshal: %v", err)
		return
	}
	req, err := agentcapabilities.NewAgentHTTPRequest(
		ctx,
		http.MethodPost,
		s.BaseURL,
		agentcapabilities.AgentWorkflowPromptActivityPath,
		s.Token,
		bytes.NewReader(body),
	)
	if err != nil {
		s.logf("workflow prompt activity: %v", err)
		return
	}
	resp, err := s.pulseMCPHTTPClient().Do(req)
	if err != nil {
		s.logf("workflow prompt activity: %v", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logf("workflow prompt activity: status %d", resp.StatusCode)
	}
}

// WriteJSON serializes one JSON-RPC message safely under the stdout mutex.
func (s *MCPStdioServer) WriteJSON(out io.Writer, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := agentcapabilities.WriteJSONRPCMessage(out, v); err != nil {
		s.logf("encode response: %v", err)
		return err
	}
	return nil
}

// ConsumeSSEEvents opens a long-lived SSE connection to Pulse's
// /api/agent/events stream and translates each non-keepalive event into a
// JSON-RPC notification on stdout with exponential backoff on reconnect.
func (s *MCPStdioServer) ConsumeSSEEvents(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		if err := s.StreamSSEOnce(ctx, agentcapabilities.AgentEventsPath); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.logf("SSE bridge: %v (reconnecting in %s)", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
	}
}

// StreamSSEOnce opens one SSE connection and streams projected MCP
// notifications to stdout until the stream closes or ctx is cancelled.
func (s *MCPStdioServer) StreamSSEOnce(ctx context.Context, path string) error {
	// Bypass finite s.HTTP timeouts: the SSE stream is long-lived and ctx is
	// the right cancellation signal. The wrapper still marks the pulse_mcp surface.
	return agentcapabilities.StreamMCPEventNotifications(ctx, PulseMCPAgentSurfaceHTTPDoer{}, s.BaseURL, path, s.Token, func(notification agentcapabilities.JSONRPCRequest) error {
		return s.WriteJSON(s.Out, notification)
	})
}
