package main

import (
	"context"
	"io"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

type agentCapability = agentcapabilities.Capability
type agentCapabilitiesManifest = agentcapabilities.Manifest
type jsonRPCRequest = agentcapabilities.JSONRPCRequest

const (
	pulseMCPServerName    = pulsecli.PulseMCPServerName
	pulseMCPServerVersion = pulsecli.PulseMCPServerVersion
)

func missingAPITokenMessage(tokenEnv string, manifest *agentCapabilitiesManifest) string {
	return pulsecli.MissingMCPAPITokenMessage(tokenEnv, manifest)
}

type pulseMCPAgentSurfaceHTTPDoer struct {
	next agentcapabilities.HTTPDoer
}

func (d pulseMCPAgentSurfaceHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return (pulsecli.PulseMCPAgentSurfaceHTTPDoer{Next: d.next}).Do(req)
}

// mcpServer adapts legacy package-main test fixtures onto the shared
// pulsecli.MCPStdioServer implementation so every cmd/pulse-mcp test exercises
// the single production runner.
type mcpServer struct {
	baseURL           string
	token             string
	manifest          *agentCapabilitiesManifest
	http              agentcapabilities.HTTPDoer
	emitNotifications bool
	out               io.Writer
	shared            *pulsecli.MCPStdioServer
}

func (s *mcpServer) delegate() *pulsecli.MCPStdioServer {
	if s.shared == nil {
		s.shared = &pulsecli.MCPStdioServer{}
	}
	s.shared.BaseURL = s.baseURL
	s.shared.Token = s.token
	s.shared.Manifest = s.manifest
	s.shared.HTTP = s.http
	s.shared.EmitNotifications = s.emitNotifications
	if s.out != nil {
		s.shared.Out = s.out
	}
	return s.shared
}

func (s *mcpServer) serve(in io.Reader, out io.Writer) {
	s.out = out
	s.delegate().Serve(context.Background(), in, out)
}

func (s *mcpServer) dispatch(ctx context.Context, req *jsonRPCRequest) agentcapabilities.JSONRPCResponse {
	return s.delegate().Dispatch(ctx, req)
}

func (s *mcpServer) manifestToolServer() agentcapabilities.MCPManifestToolServer {
	return s.delegate().ManifestToolServer()
}

func (s *mcpServer) recordWorkflowPromptActivity(ctx context.Context, promptName string) {
	s.delegate().RecordWorkflowPromptActivity(ctx, promptName)
}

func (s *mcpServer) streamSSEOnce(ctx context.Context, path string) error {
	return s.delegate().StreamSSEOnce(ctx, path)
}
