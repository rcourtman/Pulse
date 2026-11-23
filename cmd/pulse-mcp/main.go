// Command pulse-mcp is the compatibility entry point for the Model Context
// Protocol adapter for Pulse Intelligence. It delegates directly to the shared
// stdio MCP runner in pkg/pulsecli so `pulse mcp` and `pulse-mcp` share one
// implementation with zero duplicated server logic.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

func main() {
	baseURL := flag.String("base-url", agentcapabilities.DefaultMCPAdapterDefaultBaseURL, "Pulse base URL")
	tokenEnv := flag.String("token-env", agentcapabilities.DefaultMCPAdapterTokenEnv, "Env var holding the Pulse API token")
	profile := flag.String("profile", "", "Saved auth profile name (defaults to active default profile)")
	emitNotifications := flag.Bool("emit-notifications", false, "Translate Pulse SSE events into JSON-RPC notifications on stdout. Off by default because not every MCP client surfaces server-initiated notifications; enable when wiring an autonomous agent that processes the JSON-RPC stream.")
	flag.Parse()

	baseURLSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "base-url" {
			baseURLSet = true
		}
	})

	log.SetOutput(os.Stderr)
	log.SetPrefix("pulse-mcp ")
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if err := pulsecli.RunMCPStdio(context.Background(), pulsecli.RunMCPStdioOptions{
		BaseURL:           *baseURL,
		BaseURLSet:        baseURLSet,
		Profile:           *profile,
		TokenEnv:          *tokenEnv,
		EmitNotifications: *emitNotifications,
		In:                os.Stdin,
		Out:               os.Stdout,
		ErrOut:            os.Stderr,
	}); err != nil {
		log.Fatal(err)
	}
}
