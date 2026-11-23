package pulsecli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/spf13/cobra"
)

func remoteHTTPClient(deps *RemoteDeps) HTTPDoer {
	if deps != nil {
		return cliHTTPClient(deps.HTTPClient)
	}
	return cliHTTPClient(nil)
}

func remoteIsTerminal(deps *RemoteDeps) func(io.Writer) bool {
	if deps != nil {
		return deps.IsTerminal
	}
	return nil
}

func remoteNow(deps *RemoteDeps) time.Time {
	if deps != nil && deps.Now != nil {
		return deps.Now().UTC()
	}
	return time.Now().UTC()
}

// invokeCanonicalCapability calls a request/response capability from the
// canonical manifest through the shared projection, auth, redirect, and
// pulse_cli telemetry surface path.
func invokeCanonicalCapability(cmd *cobra.Command, deps *RemoteDeps, capabilityName string, args map[string]any) (agentcapabilities.HTTPCallResponse, error) {
	target, err := resolveRemoteTarget(cmd, deps)
	if err != nil {
		return agentcapabilities.HTTPCallResponse{}, err
	}
	resp, err := agentcapabilities.CallRequestResponseCapabilityHTTPByName(
		cmd.Context(),
		remoteHTTPClient(deps),
		target.BaseURL,
		target.Token,
		agentcapabilities.CanonicalManifest().Capabilities,
		capabilityName,
		args,
	)
	if err != nil {
		return agentcapabilities.HTTPCallResponse{}, fmt.Errorf("%s failed: %w", capabilityName, err)
	}
	if err := formatCapabilityHTTPError(capabilityName, resp); err != nil {
		return agentcapabilities.HTTPCallResponse{}, err
	}
	return resp, nil
}

// --- pulse resource <id> ---

func newResourceCmd(deps *RemoteDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "resource <id>",
		Short: "Inspect situated context for a single resource",
		Long:  "Fetch identity, operator state, active findings, pending approvals, and recent actions for a canonical resource ID via get_resource_context.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceID := strings.TrimSpace(args[0])
			if resourceID == "" {
				return fmt.Errorf("resource id is required")
			}
			resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.ResourceContextCapabilityName, map[string]any{
				agentcapabilities.ResourceIDArgumentName: resourceID,
			})
			if err != nil {
				return err
			}
			if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
				return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
			}
			return renderResourceContextHuman(cmd.OutOrStdout(), resp.Body)
		},
	}
}

type resourceContextHumanPayload struct {
	CanonicalID   string `json:"canonicalId"`
	ResourceType  string `json:"resourceType"`
	ResourceName  string `json:"resourceName"`
	Technology    string `json:"technology"`
	OperatorState *struct {
		MonitoringMode          string `json:"monitoringMode"`
		LifecycleState          string `json:"lifecycleState"`
		IntentionallyOffline    bool   `json:"intentionallyOffline"`
		NeverAutoRemediate      bool   `json:"neverAutoRemediate"`
		MaintenanceWindowActive bool   `json:"maintenanceWindowActive"`
		Criticality             string `json:"criticality"`
		Note                    string `json:"note"`
	} `json:"operatorState"`
	ActiveFindings []struct {
		ID       string `json:"id"`
		Severity string `json:"severity"`
		Title    string `json:"title"`
		Summary  string `json:"summary"`
	} `json:"activeFindings"`
	PendingApprovals []struct {
		ID         string `json:"id"`
		ActionID   string `json:"actionId"`
		Capability string `json:"capability"`
		RiskLevel  string `json:"riskLevel"`
	} `json:"pendingApprovals"`
	RecentActions []struct {
		ID             string `json:"id"`
		ActionID       string `json:"actionId"`
		CapabilityName string `json:"capabilityName"`
		State          string `json:"state"`
	} `json:"recentActions"`
}

func renderResourceContextHuman(w io.Writer, body []byte) error {
	var p resourceContextHumanPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return writePrettyJSONBytes(w, body)
	}
	fmt.Fprintf(w, "Resource:   %s\n", p.CanonicalID)
	if p.ResourceName != "" {
		fmt.Fprintf(w, "Name:       %s\n", p.ResourceName)
	}
	if p.ResourceType != "" {
		fmt.Fprintf(w, "Type:       %s\n", p.ResourceType)
	}
	if p.Technology != "" {
		fmt.Fprintf(w, "Technology: %s\n", p.Technology)
	}
	if p.OperatorState != nil {
		mode := p.OperatorState.MonitoringMode
		if mode == "" {
			if p.OperatorState.IntentionallyOffline {
				mode = "expected_offline"
			} else {
				mode = "normal"
			}
		}
		fmt.Fprintf(w, "Posture:    mode=%s maintenance=%t neverAutoRemediate=%t",
			mode, p.OperatorState.MaintenanceWindowActive, p.OperatorState.NeverAutoRemediate)
		if p.OperatorState.Criticality != "" {
			fmt.Fprintf(w, " criticality=%s", p.OperatorState.Criticality)
		}
		fmt.Fprintln(w)
		if p.OperatorState.Note != "" {
			fmt.Fprintf(w, "Note:       %s\n", p.OperatorState.Note)
		}
	}
	fmt.Fprintf(w, "Findings:   %d active\n", len(p.ActiveFindings))
	for _, f := range p.ActiveFindings {
		title := f.Title
		if title == "" {
			title = f.Summary
		}
		fmt.Fprintf(w, "  - [%s] %s (%s)\n", f.Severity, title, f.ID)
	}
	fmt.Fprintf(w, "Approvals:  %d pending\n", len(p.PendingApprovals))
	for _, a := range p.PendingApprovals {
		id := a.ActionID
		if id == "" {
			id = a.ID
		}
		fmt.Fprintf(w, "  - %s (%s, risk=%s)\n", id, a.Capability, a.RiskLevel)
	}
	if len(p.RecentActions) > 0 {
		fmt.Fprintf(w, "Actions:    %d recent\n", len(p.RecentActions))
		for _, a := range p.RecentActions {
			id := a.ActionID
			if id == "" {
				id = a.ID
			}
			fmt.Fprintf(w, "  - %s %s [%s]\n", id, a.CapabilityName, a.State)
		}
	}
	return nil
}

// --- pulse changes [--since 24h] [--kind ...] [--resource ...] ---

func newChangesCmd(deps *RemoteDeps) *cobra.Command {
	var sinceFlag string
	var beforeFlag string
	var kindFlag string
	var resourceFlag string
	var sourceTypeFlag string
	var limitFlag int

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "List recent changes observed across the fleet",
		Long:  "Read observed state transitions, restarts, alerts, and actions via list_recent_changes. Accepts RFC3339 timestamps or relative durations such as 12h, 24h, or 7d for --since.",
		RunE: func(cmd *cobra.Command, args []string) error {
			callArgs := map[string]any{}
			now := remoteNow(deps)
			if strings.TrimSpace(sinceFlag) != "" {
				ts, err := parseSinceTimestamp(sinceFlag, now)
				if err != nil {
					return fmt.Errorf("invalid --since value: %w", err)
				}
				callArgs["since"] = ts
			}
			if strings.TrimSpace(beforeFlag) != "" {
				ts, err := parseSinceTimestamp(beforeFlag, now)
				if err != nil {
					return fmt.Errorf("invalid --before value: %w", err)
				}
				callArgs["before"] = ts
			}
			if kind := strings.TrimSpace(kindFlag); kind != "" {
				callArgs["kind"] = kind
			}
			if resource := strings.TrimSpace(resourceFlag); resource != "" {
				callArgs[agentcapabilities.ResourceIDArgumentName] = resource
			}
			if src := strings.TrimSpace(sourceTypeFlag); src != "" {
				callArgs["sourceType"] = src
			}
			if limitFlag > 0 {
				callArgs["limit"] = limitFlag
			}

			resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.ListRecentChangesCapabilityName, callArgs)
			if err != nil {
				return err
			}
			if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
				return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
			}
			return renderRecentChangesTable(cmd.OutOrStdout(), resp.Body)
		},
	}

	cmd.Flags().StringVar(&sinceFlag, "since", "24h", "Window start: relative duration (e.g. 12h, 24h, 7d) or RFC3339 timestamp")
	cmd.Flags().StringVar(&beforeFlag, "before", "", "Optional exclusive window end: relative duration or RFC3339 timestamp")
	cmd.Flags().StringVar(&kindFlag, "kind", "", "Comma-separated change kinds (e.g. restart,alert_fired,state_transition)")
	cmd.Flags().StringVar(&resourceFlag, "resource", "", "Filter to a canonical resource ID")
	cmd.Flags().StringVar(&sourceTypeFlag, "source-type", "", "Comma-separated origins (platform_event,pulse_diff,heuristic,user_action,agent_action)")
	cmd.Flags().IntVar(&limitFlag, "limit", 0, "Maximum number of changes to return (server default 50, max 200)")
	return cmd
}

// parseSinceTimestamp accepts RFC3339 timestamps or relative lookback durations
// such as "12h", "24h", "30m", "7d", or "2d12h".
func parseSinceTimestamp(raw string, now time.Time) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty timestamp")
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC().Format(time.RFC3339), nil
	}
	dur, err := parseRelativeLookback(raw)
	if err != nil {
		return "", fmt.Errorf("expected RFC3339 timestamp or duration like 12h, 24h, 7d: %w", err)
	}
	if dur <= 0 {
		return "", fmt.Errorf("duration must be positive")
	}
	return now.UTC().Add(-dur).Format(time.RFC3339), nil
}

func parseRelativeLookback(raw string) (time.Duration, error) {
	s := strings.TrimSpace(raw)
	if idx := strings.IndexByte(s, 'd'); idx > 0 {
		daysPart := s[:idx]
		rest := s[idx+1:]
		days, err := strconv.ParseFloat(daysPart, 64)
		if err != nil || math.IsNaN(days) || math.IsInf(days, 0) {
			return 0, fmt.Errorf("invalid day duration %q", raw)
		}
		total := time.Duration(days * float64(24*time.Hour))
		if rest != "" {
			rem, err := time.ParseDuration(rest)
			if err != nil {
				return 0, err
			}
			total += rem
		}
		return total, nil
	}
	return time.ParseDuration(s)
}

type recentChangesHumanPayload struct {
	Changes []struct {
		Timestamp    string `json:"timestamp"`
		OccurredAt   string `json:"occurredAt"`
		Kind         string `json:"kind"`
		ResourceID   string `json:"resourceId"`
		ResourceName string `json:"resourceName"`
		FromState    string `json:"fromState"`
		ToState      string `json:"toState"`
		Summary      string `json:"summary"`
		Reason       string `json:"reason"`
		SourceType   string `json:"sourceType"`
	} `json:"changes"`
	Count     int  `json:"count"`
	Truncated bool `json:"truncated"`
}

func renderRecentChangesTable(w io.Writer, body []byte) error {
	var p recentChangesHumanPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return writePrettyJSONBytes(w, body)
	}
	if len(p.Changes) == 0 {
		_, err := fmt.Fprintln(w, "No changes observed in window.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tKIND\tRESOURCE\tDETAIL\tSOURCE")
	for _, c := range p.Changes {
		ts := c.Timestamp
		if ts == "" {
			ts = c.OccurredAt
		}
		res := c.ResourceID
		if c.ResourceName != "" && c.ResourceName != c.ResourceID {
			res = fmt.Sprintf("%s (%s)", c.ResourceID, c.ResourceName)
		}
		detail := c.Summary
		if detail == "" && (c.FromState != "" || c.ToState != "") {
			detail = fmt.Sprintf("%s -> %s", c.FromState, c.ToState)
		}
		if detail == "" {
			detail = c.Reason
		}
		if detail == "" {
			detail = "-"
		}
		src := c.SourceType
		if src == "" {
			src = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", ts, c.Kind, res, detail, src)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if p.Truncated {
		fmt.Fprintf(w, "\nShowing %d of %d matching changes (narrow --since, --kind, or --resource to see more).\n", len(p.Changes), p.Count)
	}
	return nil
}

// --- pulse metrics <id> [--range 24h] ---

func newMetricsCmd(deps *RemoteDeps) *cobra.Command {
	var rangeFlag string
	var metricFlag string
	var maxPointsFlag int

	cmd := &cobra.Command{
		Use:   "metrics <id>",
		Short: "Read recorded metric history for a resource",
		Long:  "Fetch recorded metric history for a canonical resource ID via get_metric_history.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceID := strings.TrimSpace(args[0])
			if resourceID == "" {
				return fmt.Errorf("resource id is required")
			}
			callArgs := map[string]any{
				agentcapabilities.ResourceIDArgumentName: resourceID,
				"range":                                  strings.TrimSpace(rangeFlag),
			}
			if m := strings.TrimSpace(metricFlag); m != "" {
				callArgs["metric"] = m
			}
			if maxPointsFlag > 0 {
				callArgs["maxPoints"] = maxPointsFlag
			}
			resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.GetMetricHistoryCapabilityName, callArgs)
			if err != nil {
				return err
			}
			if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
				return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
			}
			return renderMetricHistoryTable(cmd.OutOrStdout(), resp.Body)
		},
	}

	cmd.Flags().StringVar(&rangeFlag, "range", agentcapabilities.MetricHistoryDefaultRange, "Window ending now (30m, 1h, 6h, 12h, 24h, 7d, 30d, 90d)")
	cmd.Flags().StringVar(&metricFlag, "metric", "", "Specific metric name (e.g. cpu, memory, disk). Omit for all metrics")
	cmd.Flags().IntVar(&maxPointsFlag, "max-points", 0, "Target number of bucketed points per metric (default 60, max 200)")
	return cmd
}

type metricPointHuman struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
}

type metricHistoryHumanPayload struct {
	ResourceID string                        `json:"resourceId"`
	Metric     string                        `json:"metric"`
	Range      string                        `json:"range"`
	Source     string                        `json:"source"`
	Points     []metricPointHuman            `json:"points"`
	Metrics    map[string][]metricPointHuman `json:"metrics"`
}

func renderMetricHistoryTable(w io.Writer, body []byte) error {
	var p metricHistoryHumanPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return writePrettyJSONBytes(w, body)
	}
	series := map[string][]metricPointHuman{}
	if len(p.Metrics) > 0 {
		series = p.Metrics
	} else if p.Metric != "" || len(p.Points) > 0 {
		name := p.Metric
		if name == "" {
			name = "value"
		}
		series[name] = p.Points
	}
	if len(series) == 0 {
		_, err := fmt.Fprintln(w, "No metric points recorded in range.")
		return err
	}
	names := make([]string, 0, len(series))
	for name := range series {
		names = append(names, name)
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "METRIC\tPOINTS\tLATEST\tMIN\tMAX\tSOURCE")
	for _, name := range names {
		pts := series[name]
		if len(pts) == 0 {
			fmt.Fprintf(tw, "%s\t0\t-\t-\t-\t%s\n", name, p.Source)
			continue
		}
		minVal := pts[0].Value
		maxVal := pts[0].Value
		for _, pt := range pts {
			if pt.Min < minVal && pt.Min != 0 {
				minVal = pt.Min
			} else if pt.Value < minVal {
				minVal = pt.Value
			}
			if pt.Max > maxVal {
				maxVal = pt.Max
			} else if pt.Value > maxVal {
				maxVal = pt.Value
			}
		}
		latest := pts[len(pts)-1].Value
		fmt.Fprintf(tw, "%s\t%d\t%.2f\t%.2f\t%.2f\t%s\n", name, len(pts), latest, minVal, maxVal, p.Source)
	}
	return tw.Flush()
}

// --- pulse events [--follow] ---

func newEventsCmd(deps *RemoteDeps) *cobra.Command {
	var follow bool
	var limit int

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Stream real-time Pulse Intelligence SSE events",
		Long:  "Subscribe to /api/agent/events (subscribe_events). Emits newline-delimited JSON in non-TTY / --json mode and formatted lines on a TTY. Pass --follow to automatically reconnect after transient stream drops.",
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveRemoteTarget(cmd, deps)
			if err != nil {
				return err
			}
			jsonMode := shouldOutputJSON(cmd, remoteIsTerminal(deps))
			return runEventsStream(cmd.Context(), remoteHTTPClient(deps), target.BaseURL, target.Token, cmd.OutOrStdout(), cmd.ErrOrStderr(), follow, limit, jsonMode)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Reconnect automatically on transient disconnects until interrupted")
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after receiving N events (0 = stream until closed or interrupted)")
	return cmd
}

func runEventsStream(
	ctx context.Context,
	client HTTPDoer,
	baseURL, token string,
	out, errOut io.Writer,
	follow bool,
	limit int,
	jsonMode bool,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	received := 0
	backoff := time.Second

	for {
		var writeErr error
		err := agentcapabilities.StreamAgentActionableSSERecords(ctx, client, baseURL, agentcapabilities.AgentEventsPath, token, func(record agentcapabilities.SSERecord) bool {
			if jsonMode {
				var rawData any
				if json.Valid([]byte(record.Data)) {
					rawData = json.RawMessage(record.Data)
				} else {
					rawData = record.Data
				}
				line, mErr := json.Marshal(map[string]any{
					"event": record.Event,
					"data":  rawData,
				})
				if mErr != nil {
					writeErr = mErr
					return false
				}
				if _, wErr := fmt.Fprintln(out, string(line)); wErr != nil {
					writeErr = wErr
					return false
				}
			} else {
				if _, wErr := fmt.Fprintf(out, "[%s] %s\n", record.Event, record.Data); wErr != nil {
					writeErr = wErr
					return false
				}
			}
			received++
			if limit > 0 && received >= limit {
				return false
			}
			return true
		})
		if writeErr != nil {
			return writeErr
		}
		if limit > 0 && received >= limit {
			return nil
		}
		if !follow {
			if err != nil && ctx.Err() == nil {
				return fmt.Errorf("event stream failed: %w", err)
			}
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && errOut != nil {
			fmt.Fprintf(errOut, "event stream disconnected: %v (reconnecting in %s)\n", err, backoff)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// --- pulse findings [list|ack|snooze|dismiss|resolve] ---

func newFindingsCmd(deps *RemoteDeps) *cobra.Command {
	findingsCmd := &cobra.Command{
		Use:   "findings",
		Short: "Inspect and manage Patrol findings",
		Long:  "List Patrol findings or acknowledge, snooze, dismiss, or resolve a finding via the Pulse agent capabilities API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindingsList(cmd, deps)
		},
	}

	findingsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Patrol findings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindingsList(cmd, deps)
		},
	})

	findingsCmd.AddCommand(&cobra.Command{
		Use:     "ack <finding-id>",
		Aliases: []string{"acknowledge"},
		Short:   "Acknowledge a Patrol finding",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindingMutation(cmd, deps, agentcapabilities.AcknowledgeFindingCapabilityName, map[string]any{
				agentcapabilities.FindingIDArgumentName: strings.TrimSpace(args[0]),
			})
		},
	})

	var snoozeHours int
	snoozeCmd := &cobra.Command{
		Use:   "snooze <finding-id>",
		Short: "Snooze a Patrol finding for a number of hours",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if snoozeHours < 1 || snoozeHours > 168 {
				return fmt.Errorf("--hours must be between 1 and 168")
			}
			return runFindingMutation(cmd, deps, agentcapabilities.SnoozeFindingCapabilityName, map[string]any{
				agentcapabilities.FindingIDArgumentName: strings.TrimSpace(args[0]),
				"duration_hours":                        snoozeHours,
			})
		},
	}
	snoozeCmd.Flags().IntVar(&snoozeHours, "hours", 24, "Duration in hours to snooze the finding (1-168)")
	findingsCmd.AddCommand(snoozeCmd)

	var dismissReason string
	var dismissNote string
	dismissCmd := &cobra.Command{
		Use:   "dismiss <finding-id>",
		Short: "Dismiss a Patrol finding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reason := strings.TrimSpace(dismissReason)
			if reason == "" {
				return fmt.Errorf("--reason is required (not_an_issue, expected_behavior, or will_fix_later)")
			}
			callArgs := map[string]any{
				agentcapabilities.FindingIDArgumentName: strings.TrimSpace(args[0]),
				agentcapabilities.ReasonArgumentName:    reason,
			}
			if note := strings.TrimSpace(dismissNote); note != "" {
				callArgs[agentcapabilities.NoteArgumentName] = note
			}
			return runFindingMutation(cmd, deps, agentcapabilities.DismissFindingCapabilityName, callArgs)
		},
	}
	dismissCmd.Flags().StringVar(&dismissReason, "reason", "", "Dismissal reason: not_an_issue, expected_behavior, or will_fix_later")
	dismissCmd.Flags().StringVar(&dismissNote, "note", "", "Optional operator note explaining the dismissal")
	findingsCmd.AddCommand(dismissCmd)

	var resolveNote string
	resolveCmd := &cobra.Command{
		Use:   "resolve <finding-id>",
		Short: "Mark a Patrol finding as resolved",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			callArgs := map[string]any{
				agentcapabilities.FindingIDArgumentName: strings.TrimSpace(args[0]),
			}
			if note := strings.TrimSpace(resolveNote); note != "" {
				callArgs[agentcapabilities.ResolutionNoteArgumentName] = note
			}
			return runFindingMutation(cmd, deps, agentcapabilities.ResolveFindingCapabilityName, callArgs)
		},
	}
	resolveCmd.Flags().StringVar(&resolveNote, "note", "", "Optional operator note describing how the finding was resolved")
	findingsCmd.AddCommand(resolveCmd)

	return findingsCmd
}

func runFindingsList(cmd *cobra.Command, deps *RemoteDeps) error {
	resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.ListFindingsCapabilityName, nil)
	if err != nil {
		return err
	}
	if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
		return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
	}
	return renderFindingsTable(cmd.OutOrStdout(), resp.Body)
}

func runFindingMutation(cmd *cobra.Command, deps *RemoteDeps, capabilityName string, callArgs map[string]any) error {
	resp, err := invokeCanonicalCapability(cmd, deps, capabilityName, callArgs)
	if err != nil {
		return err
	}
	if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
		return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Body, &result); err == nil && result.Message != "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), result.Message)
		return err
	}
	return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
}

type findingRowHuman struct {
	ID           string `json:"id"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Title        string `json:"title"`
}

func renderFindingsTable(w io.Writer, body []byte) error {
	var rows []findingRowHuman
	if err := json.Unmarshal(body, &rows); err != nil {
		var wrapped struct {
			Items []findingRowHuman `json:"items"`
		}
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			return writePrettyJSONBytes(w, body)
		}
		rows = wrapped.Items
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No findings.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSEVERITY\tSTATUS\tRESOURCE\tTITLE")
	for _, f := range rows {
		res := f.ResourceID
		if res == "" {
			res = f.ResourceName
		}
		if res == "" {
			res = "-"
		}
		status := f.Status
		if status == "" {
			status = "active"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.ID, f.Severity, status, res, f.Title)
	}
	return tw.Flush()
}

// --- pulse operator-state [get|set|clear] ---

func newOperatorStateCmd(deps *RemoteDeps) *cobra.Command {
	opCmd := &cobra.Command{
		Use:   "operator-state",
		Short: "Read or update per-resource operator intent state",
		Long:  "Manage per-resource monitoring posture, maintenance windows, remediation locks, and notes via get_operator_state, set_operator_state, and clear_operator_state.",
	}

	opCmd.AddCommand(&cobra.Command{
		Use:   "get <resource-id>",
		Short: "Read operator state for a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.GetOperatorStateCapabilityName, map[string]any{
				agentcapabilities.ResourceIDArgumentName: strings.TrimSpace(args[0]),
			})
			if err != nil {
				return err
			}
			return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
		},
	})

	var monitoringMode string
	var lifecycleState string
	var intentionallyOffline bool
	var neverAutoRemediate bool
	var maintenanceStart string
	var maintenanceEnd string
	var maintenanceScope string
	var maintenanceReason string
	var criticality string
	var note string

	setCmd := &cobra.Command{
		Use:   "set <resource-id>",
		Short: "Set operator state for a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			offline := intentionallyOffline
			if monitoringMode == "expected_offline" && !cmd.Flags().Changed("intentionally-offline") {
				offline = true
			}
			callArgs := map[string]any{
				agentcapabilities.ResourceIDArgumentName: strings.TrimSpace(args[0]),
				"intentionallyOffline":                   offline,
				"neverAutoRemediate":                     neverAutoRemediate,
			}
			if v := strings.TrimSpace(monitoringMode); v != "" {
				callArgs["monitoringMode"] = v
			}
			if v := strings.TrimSpace(lifecycleState); v != "" {
				callArgs["lifecycleState"] = v
			}
			if v := strings.TrimSpace(maintenanceStart); v != "" {
				callArgs["maintenanceStartAt"] = v
			}
			if v := strings.TrimSpace(maintenanceEnd); v != "" {
				callArgs["maintenanceEndAt"] = v
			}
			if v := strings.TrimSpace(maintenanceScope); v != "" {
				callArgs["maintenanceScope"] = v
			}
			if v := strings.TrimSpace(maintenanceReason); v != "" {
				callArgs["maintenanceReason"] = v
			}
			if cmd.Flags().Changed("criticality") {
				callArgs["criticality"] = strings.TrimSpace(criticality)
			}
			if cmd.Flags().Changed("note") {
				callArgs["note"] = note
			}
			resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.SetOperatorStateCapabilityName, callArgs)
			if err != nil {
				return err
			}
			return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
		},
	}
	setCmd.Flags().StringVar(&monitoringMode, "monitoring-mode", "", "Monitoring posture: normal, expected_offline, or muted")
	setCmd.Flags().StringVar(&lifecycleState, "lifecycle-state", "", "Lifecycle state: active or retired")
	setCmd.Flags().BoolVar(&intentionallyOffline, "intentionally-offline", false, "Mark resource as expected offline")
	setCmd.Flags().BoolVar(&neverAutoRemediate, "never-auto-remediate", false, "Refuse automated remediation for this resource")
	setCmd.Flags().StringVar(&maintenanceStart, "maintenance-start", "", "Maintenance window start (RFC3339)")
	setCmd.Flags().StringVar(&maintenanceEnd, "maintenance-end", "", "Maintenance window end (RFC3339)")
	setCmd.Flags().StringVar(&maintenanceScope, "maintenance-scope", "", "Maintenance scope: resource or resource_and_descendants")
	setCmd.Flags().StringVar(&maintenanceReason, "maintenance-reason", "", "Reason for maintenance window")
	setCmd.Flags().StringVar(&criticality, "criticality", "", "Finding sort hint: high, medium, low, or empty to clear")
	setCmd.Flags().StringVar(&note, "note", "", "Operator note for this resource")
	opCmd.AddCommand(setCmd)

	opCmd.AddCommand(&cobra.Command{
		Use:   "clear <resource-id>",
		Short: "Clear operator state for a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeCanonicalCapability(cmd, deps, agentcapabilities.ClearOperatorStateCapabilityName, map[string]any{
				agentcapabilities.ResourceIDArgumentName: strings.TrimSpace(args[0]),
			})
			if err != nil {
				return err
			}
			if len(strings.TrimSpace(string(resp.Body))) == 0 {
				if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
					return writePrettyJSON(cmd.OutOrStdout(), map[string]any{"cleared": true, "resourceId": strings.TrimSpace(args[0])})
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Cleared operator state for %s\n", strings.TrimSpace(args[0]))
				return err
			}
			return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
		},
	})

	return opCmd
}

// --- pulse call <capability> [--arg key=value ...] ---

func newCallCmd(deps *RemoteDeps) *cobra.Command {
	var argPairs []string
	var bodyJSON string
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "call <capability>",
		Short: "Invoke any capability published by /api/agent/capabilities",
		Long:  "Generic manifest-driven fallback that discovers capabilities live from the target server's /api/agent/capabilities endpoint and invokes the named capability with --arg key=value or --body-json.",
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveRemoteTarget(cmd, deps)
			if err != nil {
				return err
			}
			client := remoteHTTPClient(deps)
			manifest, err := agentcapabilities.FetchManifest(cmd.Context(), client, target.BaseURL)
			if err != nil {
				return fmt.Errorf("fetch capabilities manifest: %w", err)
			}

			if listOnly {
				return renderManifestCapabilities(cmd, deps, manifest)
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("capability name is required (or pass --list to view available capabilities)")
			}
			capName := strings.TrimSpace(args[0])

			callArgs := map[string]any{}
			if strings.TrimSpace(bodyJSON) != "" {
				parsed, err := decodeActionParamsObject([]byte(bodyJSON), "--body-json")
				if err != nil {
					return err
				}
				for k, v := range parsed {
					callArgs[k] = v
				}
			}
			for _, raw := range argPairs {
				k, v, err := parseCapabilityArg(raw)
				if err != nil {
					return err
				}
				callArgs[k] = v
			}

			resp, err := agentcapabilities.CallRequestResponseCapabilityHTTPByName(
				cmd.Context(),
				client,
				target.BaseURL,
				target.Token,
				manifest.Capabilities,
				capName,
				callArgs,
			)
			if err != nil {
				return fmt.Errorf("call %s failed: %w", capName, err)
			}
			if err := formatCapabilityHTTPError(capName, resp); err != nil {
				return err
			}
			return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
		},
	}

	cmd.Flags().StringArrayVar(&argPairs, "arg", nil, "Capability argument as key=value (repeatable, values parse as JSON when valid)")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "JSON object of capability arguments (merged before --arg)")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List all capabilities advertised by the server manifest")
	return cmd
}

func parseCapabilityArg(raw string) (string, any, error) {
	key, valueText, ok := strings.Cut(raw, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return "", nil, fmt.Errorf("--arg must use key=value")
	}
	valueText = strings.TrimSpace(valueText)
	if valueText == "" {
		return key, "", nil
	}
	var value any
	if err := decodeJSONString(valueText, &value); err != nil {
		return key, valueText, nil
	}
	return key, value, nil
}

func renderManifestCapabilities(cmd *cobra.Command, deps *RemoteDeps, manifest *agentcapabilities.Manifest) error {
	if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
		return writePrettyJSON(cmd.OutOrStdout(), manifest.Capabilities)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CAPABILITY\tMETHOD\tPATH\tSCOPE\tMODE")
	for _, c := range manifest.Capabilities {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", c.Name, c.Method, c.Path, c.Scope, c.ActionMode)
	}
	return tw.Flush()
}
