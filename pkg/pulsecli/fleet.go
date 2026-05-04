package pulsecli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

type FleetDeps struct {
	HTTPClient HTTPDoer
	Getenv     func(string) string
}

type fleetConnectionsOptions struct {
	APIURL string
	Token  string
}

type fleetConnectionsResponse struct {
	Connections []json.RawMessage `json:"connections"`
	Systems     []json.RawMessage `json:"systems,omitempty"`
}

func newFleetCmd(deps *FleetDeps) *cobra.Command {
	fleetCmd := &cobra.Command{
		Use:   "fleet",
		Short: "Inspect canonical Pulse fleet state",
	}
	fleetCmd.AddCommand(newFleetConnectionsCmd(deps))
	return fleetCmd
}

func newFleetConnectionsCmd(deps *FleetDeps) *cobra.Command {
	opts := fleetConnectionsOptions{
		APIURL: strings.TrimSpace(fleetGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(fleetGetenv(deps, "PULSE_API_TOKEN")),
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	cmd := &cobra.Command{
		Use:   "connections",
		Short: "List canonical fleet connection rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFleetConnections(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	return cmd
}

func runFleetConnections(cmd *cobra.Command, deps *FleetDeps, opts fleetConnectionsOptions) error {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	endpoint, err := pulseAPIEndpoint(opts.APIURL, "/connections")
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to build fleet connections request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := fleetHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("fleet connections request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "fleet connections response")
	if err != nil {
		return fmt.Errorf("failed to read fleet connections response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusError("fleet connections request", resp.Status, respBody)
	}

	var connections fleetConnectionsResponse
	if err := decodeJSONBytes(respBody, &connections); err != nil {
		return fmt.Errorf("failed to decode fleet connections response: %w", err)
	}
	if connections.Connections == nil {
		connections.Connections = []json.RawMessage{}
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(connections); err != nil {
		return fmt.Errorf("failed to write fleet connections response: %w", err)
	}
	return nil
}

func fleetHTTPClient(deps *FleetDeps) HTTPDoer {
	if deps != nil {
		return cliHTTPClient(deps.HTTPClient)
	}
	return cliHTTPClient(nil)
}

func fleetGetenv(deps *FleetDeps, key string) string {
	if deps != nil {
		return cliGetenv(deps.Getenv, key)
	}
	return cliGetenv(nil, key)
}
