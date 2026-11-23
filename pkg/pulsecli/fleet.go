package pulsecli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
)

type FleetDeps struct {
	HTTPClient HTTPDoer
	Store      *authstore.Store
	Getenv     func(string) string
	IsTerminal func(io.Writer) bool
}

type fleetSummaryOptions struct {
	HasFindings  bool
	Severity     string
	Technology   string
	ResourceType string
}

type fleetConnectionsOptions struct {
	APIURL string
	Token  string
}

type fleetConnectionsResponse struct {
	Connections []json.RawMessage `json:"connections"`
	Systems     []json.RawMessage `json:"systems,omitempty"`
}

type fleetSummaryTablePayload struct {
	Resources []struct {
		CanonicalID             string `json:"canonicalId"`
		ResourceType            string `json:"resourceType"`
		ResourceName            string `json:"resourceName"`
		Technology              string `json:"technology"`
		IntentionallyOffline    bool   `json:"intentionallyOffline"`
		MonitoringMode          string `json:"monitoringMode"`
		LifecycleState          string `json:"lifecycleState"`
		NeverAutoRemediate      bool   `json:"neverAutoRemediate"`
		MaintenanceWindowActive bool   `json:"maintenanceWindowActive"`
		Findings                struct {
			Total    int `json:"total"`
			Critical int `json:"critical"`
			Warning  int `json:"warning"`
			Info     int `json:"info"`
		} `json:"findings"`
		PendingApprovalCount int `json:"pendingApprovalCount"`
	} `json:"resources"`
	GeneratedAt string `json:"generatedAt"`
}

func newFleetCmd(deps *FleetDeps) *cobra.Command {
	var summaryOpts fleetSummaryOptions

	fleetCmd := &cobra.Command{
		Use:   "fleet",
		Short: "Inspect canonical Pulse fleet state",
		Long:  "Show a concise fleet triage summary from get_fleet_context, or run `pulse fleet connections` to list canonical fleet connection rows.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFleetSummary(cmd, deps, summaryOpts)
		},
	}

	fleetCmd.Flags().BoolVar(&summaryOpts.HasFindings, "has-findings", false, "Return only resources with at least one active finding")
	fleetCmd.Flags().StringVar(&summaryOpts.Severity, "severity", "", "Filter by finding severity (critical, warning, info)")
	fleetCmd.Flags().StringVar(&summaryOpts.Technology, "technology", "", "Filter by technology family (e.g. proxmox, docker, lxc, qemu)")
	fleetCmd.Flags().StringVar(&summaryOpts.ResourceType, "resource-type", "", "Filter by canonical resource type (e.g. vm, agent, storage)")

	fleetCmd.AddCommand(newFleetConnectionsCmd(deps))
	return fleetCmd
}

func runFleetSummary(cmd *cobra.Command, deps *FleetDeps, opts fleetSummaryOptions) error {
	var remoteDeps *RemoteDeps
	if deps != nil {
		remoteDeps = &RemoteDeps{
			HTTPClient: deps.HTTPClient,
			Store:      deps.Store,
			Getenv:     deps.Getenv,
			IsTerminal: deps.IsTerminal,
		}
	}
	target, err := resolveRemoteTarget(cmd, remoteDeps)
	if err != nil {
		return err
	}

	args := map[string]any{}
	if opts.HasFindings {
		args["hasFindings"] = true
	}
	if severity := strings.TrimSpace(opts.Severity); severity != "" {
		args["severity"] = severity
	}
	if tech := strings.TrimSpace(opts.Technology); tech != "" {
		args["technology"] = tech
	}
	if resourceType := strings.TrimSpace(opts.ResourceType); resourceType != "" {
		args["resourceType"] = resourceType
	}

	resp, err := agentcapabilities.CallRequestResponseCapabilityHTTPByName(
		cmd.Context(),
		fleetHTTPClient(deps),
		target.BaseURL,
		target.Token,
		agentcapabilities.CanonicalManifest().Capabilities,
		agentcapabilities.FleetContextCapabilityName,
		args,
	)
	if err != nil {
		return fmt.Errorf("fleet summary request failed: %w", err)
	}
	if err := formatCapabilityHTTPError("fleet summary", resp); err != nil {
		return err
	}

	var isTerm func(io.Writer) bool
	if deps != nil {
		isTerm = deps.IsTerminal
	}
	if shouldOutputJSON(cmd, isTerm) {
		return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
	}

	var payload fleetSummaryTablePayload
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
	}
	return renderFleetSummaryTable(cmd.OutOrStdout(), payload)
}

func renderFleetSummaryTable(w io.Writer, payload fleetSummaryTablePayload) error {
	if len(payload.Resources) == 0 {
		_, err := fmt.Fprintln(w, "No resources found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "RESOURCE\tNAME\tTYPE\tTECH\tFINDINGS\tPENDING\tFLAGS")
	for _, r := range payload.Resources {
		findings := "-"
		if r.Findings.Total > 0 {
			findings = fmt.Sprintf("%d (c:%d w:%d i:%d)", r.Findings.Total, r.Findings.Critical, r.Findings.Warning, r.Findings.Info)
		}
		var flags []string
		if r.MaintenanceWindowActive {
			flags = append(flags, "maintenance")
		}
		if r.IntentionallyOffline || r.MonitoringMode == "expected_offline" {
			flags = append(flags, "expected-offline")
		} else if r.MonitoringMode == "muted" {
			flags = append(flags, "muted")
		}
		if r.LifecycleState == "retired" {
			flags = append(flags, "retired")
		}
		if r.NeverAutoRemediate {
			flags = append(flags, "no-remediate")
		}
		flagStr := "-"
		if len(flags) > 0 {
			flagStr = strings.Join(flags, ",")
		}
		tech := r.Technology
		if tech == "" {
			tech = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			r.CanonicalID, r.ResourceName, r.ResourceType, tech, findings, r.PendingApprovalCount, flagStr)
	}
	return tw.Flush()
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
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	return cmd
}

func runFleetConnections(cmd *cobra.Command, deps *FleetDeps, opts fleetConnectionsOptions) error {
	var store *authstore.Store
	var getenv func(string) string
	if deps != nil {
		store = deps.Store
		getenv = deps.Getenv
	}
	apiURL, token, err := resolveLegacyTarget(
		cmd,
		opts.APIURL,
		opts.Token,
		cmd.Flags().Changed("api-url"),
		cmd.Flags().Changed("token"),
		store,
		getenv,
	)
	if err != nil {
		return err
	}

	endpoint, err := pulseAPIEndpoint(apiURL, "/connections")
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
		return apiStatusErrorWithCode("fleet connections request", resp.StatusCode, resp.Status, respBody)
	}

	var connections fleetConnectionsResponse
	if err := decodeJSONBytes(respBody, &connections); err != nil {
		return fmt.Errorf("failed to decode fleet connections response: %w", err)
	}
	if connections.Connections == nil {
		// Missing data is not an empty fleet. The server emits an explicit
		// array, including [] when there are no connections in the scope.
		return fmt.Errorf("invalid fleet connections response: expected a non-null connections array (use [] for an empty fleet)")
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
