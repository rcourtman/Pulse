package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
)

const (
	maxActionRequestBytes = 1 << 20
)

type ActionsDeps struct {
	HTTPClient HTTPDoer
	Store      *authstore.Store
	Getenv     func(string) string
	IsTerminal func(io.Writer) bool
}

type actionPlanOptions struct {
	APIURL         string
	Token          string
	RequestFile    string
	RequestID      string
	ResourceID     string
	CapabilityName string
	Reason         string
	RequestedBy    string
	ParamsJSON     string
	Params         []string
}

type actionCapabilitiesOptions struct {
	APIURL     string
	Token      string
	ResourceID string
}

type actionDecisionOptions struct {
	APIURL   string
	Token    string
	ActionID string
	Outcome  string
	Reason   string
	PlanHash string
}

type actionExecutionOptions struct {
	APIURL   string
	Token    string
	ActionID string
	Reason   string
	PlanHash string
}

type actionAuditOptions struct {
	APIURL     string
	Token      string
	ResourceID string
	Since      string
	Limit      int
}

type actionEventsOptions struct {
	APIURL   string
	Token    string
	ActionID string
	Since    string
	Limit    int
}

type actionListOptions struct {
	View  string
	Limit int
}

type actionGetOptions struct {
	ActionID string
}

type actionRefreshOptions struct {
	ActionID string
	PlanHash string
}

type actionWatchOptions struct {
	ActionID string
	Interval time.Duration
	Timeout  time.Duration
	Until    string
}

type actionActorView struct {
	SubjectID       string `json:"subjectId,omitempty"`
	Kind            string `json:"kind,omitempty"`
	CredentialID    string `json:"credentialId,omitempty"`
	CredentialLabel string `json:"credentialLabel,omitempty"`
	OrgID           string `json:"orgId,omitempty"`
}

type actionRequestView struct {
	RequestID      string          `json:"requestId,omitempty"`
	ResourceID     string          `json:"resourceId,omitempty"`
	CapabilityName string          `json:"capabilityName,omitempty"`
	Params         map[string]any  `json:"params,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	RequestedBy    string          `json:"requestedBy,omitempty"`
	Actor          actionActorView `json:"actor,omitempty"`
}

type actionResourceView struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

type actionAuditView struct {
	ID                  string                            `json:"id"`
	CreatedAt           time.Time                         `json:"createdAt,omitempty"`
	UpdatedAt           time.Time                         `json:"updatedAt,omitempty"`
	State               unified.ActionState               `json:"state"`
	DecisionRevision    uint64                            `json:"decisionRevision,omitempty"`
	Request             actionRequestView                 `json:"request"`
	Plan                unified.ActionPlan                `json:"plan"`
	Origin              *unified.ActionOrigin             `json:"origin,omitempty"`
	Resource            *actionResourceView               `json:"resource,omitempty"`
	BlastRadius         []actionResourceView              `json:"blastRadius,omitempty"`
	Approvals           []unified.ActionApprovalRecord    `json:"approvals,omitempty"`
	Result              *unified.ExecutionResult          `json:"result,omitempty"`
	Verification        *unified.ActionVerificationResult `json:"verification,omitempty"`
	VerificationOutcome unified.VerificationOutcome       `json:"verificationOutcome,omitempty"`
}

type actionReadinessView struct {
	Ready       bool      `json:"ready"`
	Code        string    `json:"code,omitempty"`
	Message     string    `json:"message,omitempty"`
	Remediation string    `json:"remediation,omitempty"`
	Refreshable bool      `json:"refreshable,omitempty"`
	CheckedAt   time.Time `json:"checkedAt,omitempty"`
}

type actionListResponse struct {
	View     string            `json:"view"`
	Actions  []actionAuditView `json:"actions"`
	Count    int               `json:"count"`
	ReadOnly bool              `json:"readOnly,omitempty"`
}

type actionDetailViewResponse struct {
	Audit     actionAuditView                `json:"audit"`
	Events    []unified.ActionLifecycleEvent `json:"events"`
	Attempt   map[string]any                 `json:"attempt,omitempty"`
	Receipt   map[string]any                 `json:"receipt,omitempty"`
	Readiness *actionReadinessView           `json:"readiness,omitempty"`
	ReadOnly  bool                           `json:"readOnly,omitempty"`
}

type actionWatchEvent struct {
	ActionID            string                       `json:"actionId"`
	State               unified.ActionState          `json:"state"`
	CapabilityName      string                       `json:"capabilityName,omitempty"`
	ResourceID          string                       `json:"resourceId,omitempty"`
	PlanHash            string                       `json:"planHash,omitempty"`
	UpdatedAt           time.Time                    `json:"updatedAt,omitempty"`
	Readiness           *actionReadinessView         `json:"readiness,omitempty"`
	Result              *unified.ExecutionResult     `json:"result,omitempty"`
	VerificationOutcome *unified.VerificationOutcome `json:"verificationOutcome,omitempty"`
}

type actionCapabilitiesResponse struct {
	ResourceID   string                       `json:"resourceId"`
	Count        int                          `json:"count"`
	Capabilities []unified.ResourceCapability `json:"capabilities"`
}

type actionResourceFacetsResponse struct {
	ResourceID   string                       `json:"resourceId"`
	Capabilities []unified.ResourceCapability `json:"capabilities,omitempty"`
}

type actionAuditListResponse struct {
	Audits     []unified.ActionAuditRecord `json:"audits"`
	Count      int                         `json:"count"`
	ResourceID string                      `json:"resourceId,omitempty"`
}

type actionLifecycleEventsResponse struct {
	ActionID string                         `json:"actionId"`
	Events   []unified.ActionLifecycleEvent `json:"events"`
	Count    int                            `json:"count"`
}

type actionDecisionResponse struct {
	ActionID string                       `json:"actionId"`
	State    unified.ActionState          `json:"state"`
	Approval unified.ActionApprovalRecord `json:"approval"`
	Audit    unified.ActionAuditRecord    `json:"audit"`
}

type actionExecutionResponse struct {
	ActionID string                    `json:"actionId"`
	State    unified.ActionState       `json:"state"`
	Result   *unified.ExecutionResult  `json:"result,omitempty"`
	Audit    unified.ActionAuditRecord `json:"audit"`
}

// actionAPIDefaults seeds the shared Pulse API URL and token defaults for
// action subcommands from the environment.
func actionAPIDefaults(deps *ActionsDeps) (apiURL, token string) {
	apiURL = strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL"))
	if apiURL == "" {
		apiURL = defaultPulseAPIURL
	}
	return apiURL, strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN"))
}

func newActionsCmd(deps *ActionsDeps) *cobra.Command {
	actionsCmd := &cobra.Command{
		Use:   "actions",
		Short: "Inspect and plan governed Pulse actions",
	}

	apiURL, token := actionAPIDefaults(deps)
	opts := actionPlanOptions{
		APIURL: apiURL,
		Token:  token,
	}

	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Request a deterministic pre-execution action plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionPlan(cmd, deps, opts)
		},
	}

	planCmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	planCmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	planCmd.Flags().StringVar(&opts.RequestFile, "request-file", "", "JSON ActionRequest file, or - for stdin")
	planCmd.Flags().StringVar(&opts.RequestID, "request-id", "", "caller idempotency key")
	planCmd.Flags().StringVar(&opts.ResourceID, "resource-id", "", "canonical unified resource id")
	planCmd.Flags().StringVar(&opts.CapabilityName, "capability", "", "resource capability name to plan")
	planCmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for the requested action")
	planCmd.Flags().StringVar(&opts.RequestedBy, "requested-by", "", "deprecated compatibility label, server derives requester identity from authentication")
	planCmd.Flags().StringVar(&opts.ParamsJSON, "params-json", "", "JSON object merged into request params")
	planCmd.Flags().StringArrayVar(&opts.Params, "param", nil, "request param as key=value, repeatable, values parse as JSON when possible")

	actionsCmd.AddCommand(newActionListCmd(deps))
	actionsCmd.AddCommand(newActionGetCmd(deps))
	actionsCmd.AddCommand(planCmd)
	actionsCmd.AddCommand(newActionRefreshCmd(deps))
	actionsCmd.AddCommand(newActionWatchCmd(deps))
	actionsCmd.AddCommand(newActionCapabilitiesCmd(deps))
	actionsCmd.AddCommand(newActionDecisionCmd(deps))
	actionsCmd.AddCommand(newActionOutcomeShorthandCmd(deps, "approve", unified.OutcomeApproved, "Approve a pending governed action plan"))
	actionsCmd.AddCommand(newActionOutcomeShorthandCmd(deps, "reject", unified.OutcomeRejected, "Reject a pending governed action plan"))
	actionsCmd.AddCommand(newActionExecutionCmd(deps))
	actionsCmd.AddCommand(newActionAuditCmd(deps))
	actionsCmd.AddCommand(newActionEventsCmd(deps))
	return actionsCmd
}

func newActionListCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionListOptions{
		View: "pending",
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List governed actions in the queue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionList(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.View, "view", opts.View, "Queue view: pending, settled, or all")
	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "Maximum actions to return (1 to 500)")
	return cmd
}

func newActionGetCmd(deps *ActionsDeps) *cobra.Command {
	var opts actionGetOptions
	cmd := &cobra.Command{
		Use:   "get <action-id>",
		Short: "Inspect a governed action plan, readiness, events, and outcome",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			actionID, err := resolveActionIDArg(opts.ActionID, args)
			if err != nil {
				return err
			}
			return runActionGet(cmd, deps, actionID)
		},
	}
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id (optional when passed as positional argument)")
	return cmd
}

func newActionRefreshCmd(deps *ActionsDeps) *cobra.Command {
	var opts actionRefreshOptions
	cmd := &cobra.Command{
		Use:   "refresh <action-id>",
		Short: "Refresh an expired or drifted governed action plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			callOpts := opts
			actionID, err := resolveActionIDArg(opts.ActionID, args)
			if err != nil {
				return err
			}
			callOpts.ActionID = actionID
			return runActionRefresh(cmd, deps, callOpts)
		},
	}
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id (optional when passed as positional argument)")
	cmd.Flags().StringVar(&opts.PlanHash, "plan-hash", "", "reviewed planHash binding the refresh to the exact inspected plan")
	return cmd
}

func newActionWatchCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionWatchOptions{
		Interval: 2 * time.Second,
		Timeout:  30 * time.Minute,
		Until:    "terminal",
	}
	cmd := &cobra.Command{
		Use:   "watch <action-id>",
		Short: "Watch a governed action until it reaches a decided or terminal state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			callOpts := opts
			actionID, err := resolveActionIDArg(opts.ActionID, args)
			if err != nil {
				return err
			}
			callOpts.ActionID = actionID
			return runActionWatch(cmd, deps, callOpts)
		},
	}
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id (optional when passed as positional argument)")
	cmd.Flags().DurationVar(&opts.Interval, "interval", opts.Interval, "Polling interval between status checks")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", opts.Timeout, "Maximum duration to wait before timing out")
	cmd.Flags().StringVar(&opts.Until, "until", opts.Until, "Stop condition: terminal or decided")
	return cmd
}

func newActionOutcomeShorthandCmd(deps *ActionsDeps, use string, outcome unified.ApprovalOutcome, short string) *cobra.Command {
	opts := actionDecisionOptions{
		Outcome: string(outcome),
	}
	cmd := &cobra.Command{
		Use:   use + " <action-id>",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			callOpts := opts
			actionID, err := resolveActionIDArg(opts.ActionID, args)
			if err != nil {
				return err
			}
			callOpts.ActionID = actionID
			return runActionDecision(cmd, deps, callOpts)
		},
	}
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id (optional when passed as positional argument)")
	cmd.Flags().StringVar(&opts.PlanHash, "plan-hash", "", "reviewed planHash binding the decision to the exact inspected plan")
	cmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for the decision")
	return cmd
}

func newActionCapabilitiesCmd(deps *ActionsDeps) *cobra.Command {
	apiURL, token := actionAPIDefaults(deps)
	opts := actionCapabilitiesOptions{
		APIURL: apiURL,
		Token:  token,
	}

	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "List capabilities advertised by a unified resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionCapabilities(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	cmd.Flags().StringVar(&opts.ResourceID, "resource-id", "", "canonical unified resource id")
	return cmd
}

func newActionDecisionCmd(deps *ActionsDeps) *cobra.Command {
	apiURL, token := actionAPIDefaults(deps)
	opts := actionDecisionOptions{
		APIURL: apiURL,
		Token:  token,
	}

	cmd := &cobra.Command{
		Use:   "decide",
		Short: "Approve or reject a governed action plan without execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionDecision(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id")
	cmd.Flags().StringVar(&opts.Outcome, "outcome", "", "decision outcome: approved or rejected")
	cmd.Flags().StringVar(&opts.PlanHash, "plan-hash", "", "reviewed planHash binding the decision to the exact inspected plan")
	cmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for the decision")
	return cmd
}

func newActionExecutionCmd(deps *ActionsDeps) *cobra.Command {
	apiURL, token := actionAPIDefaults(deps)
	opts := actionExecutionOptions{
		APIURL: apiURL,
		Token:  token,
	}

	cmd := &cobra.Command{
		Use:   "execute [action-id]",
		Short: "Execute a governed action through the API action contract",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			callOpts := opts
			if len(args) > 0 {
				actionID, err := resolveActionIDArg(opts.ActionID, args)
				if err != nil {
					return err
				}
				callOpts.ActionID = actionID
			}
			return runActionExecution(cmd, deps, callOpts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id (optional when passed as positional argument)")
	cmd.Flags().StringVar(&opts.PlanHash, "plan-hash", "", "reviewed planHash binding execution to the exact approved plan")
	cmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for starting execution")
	return cmd
}

//nolint:dupl // parallel cobra subcommand registration; flags/help diverge per command
func newActionAuditCmd(deps *ActionsDeps) *cobra.Command {
	apiURL, token := actionAPIDefaults(deps)
	opts := actionAuditOptions{
		APIURL: apiURL,
		Token:  token,
		Limit:  100,
	}

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List governed action audit records",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionAudit(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	cmd.Flags().StringVar(&opts.ResourceID, "resource-id", "", "canonical unified resource id")
	cmd.Flags().StringVar(&opts.Since, "since", "", "RFC3339 lower bound for audit records")
	cmd.Flags().IntVar(&opts.Limit, "limit", opts.Limit, "maximum audit records to return")
	return cmd
}

//nolint:dupl // parallel cobra subcommand registration; flags/help diverge per command
func newActionEventsCmd(deps *ActionsDeps) *cobra.Command {
	apiURL, token := actionAPIDefaults(deps)
	opts := actionEventsOptions{
		APIURL: apiURL,
		Token:  token,
		Limit:  100,
	}

	cmd := &cobra.Command{
		Use:   "events",
		Short: "List lifecycle events for a governed action",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionEvents(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token (deprecated: prefer pulse auth login or PULSE_API_TOKEN)")
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id")
	cmd.Flags().StringVar(&opts.Since, "since", "", "RFC3339 lower bound for lifecycle events")
	cmd.Flags().IntVar(&opts.Limit, "limit", opts.Limit, "maximum lifecycle events to return")
	return cmd
}

func resolveActionIDArg(flagVal string, args []string) (string, error) {
	fromFlag := strings.TrimSpace(flagVal)
	fromPos := ""
	if len(args) > 0 {
		fromPos = strings.TrimSpace(args[0])
	}
	if fromFlag != "" && fromPos != "" && fromFlag != fromPos {
		return "", fmt.Errorf("conflicting action IDs provided via positional argument and --action-id")
	}
	if fromPos != "" {
		return fromPos, nil
	}
	if fromFlag != "" {
		return fromFlag, nil
	}
	return "", fmt.Errorf("actionId is required (pass <action-id> or --action-id)")
}

func resolveActionTarget(cmd *cobra.Command, deps *ActionsDeps, apiURL, token string) (string, string, error) {
	var store *authstore.Store
	var getenv func(string) string
	if deps != nil {
		store = deps.Store
		getenv = deps.Getenv
	}
	localAPIURLChanged := cmd != nil && cmd.LocalFlags().Changed("api-url")
	localTokenChanged := cmd != nil && cmd.LocalFlags().Changed("token")
	return resolveLegacyTarget(
		cmd,
		apiURL,
		token,
		localAPIURLChanged,
		localTokenChanged,
		store,
		getenv,
	)
}

func runActionPlan(cmd *cobra.Command, deps *ActionsDeps, opts actionPlanOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, opts.APIURL, opts.Token)
	if err != nil {
		return err
	}

	endpoint, err := actionPlanEndpoint(apiURL)
	if err != nil {
		return err
	}

	actionReq, err := buildActionRequest(cmd, opts)
	if err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		RequestID      string         `json:"requestId"`
		ResourceID     string         `json:"resourceId"`
		CapabilityName string         `json:"capabilityName"`
		Params         map[string]any `json:"params,omitempty"`
		Reason         string         `json:"reason"`
		RequestedBy    string         `json:"requestedBy,omitempty"`
	}{
		RequestID: actionReq.RequestID, ResourceID: actionReq.ResourceID, CapabilityName: actionReq.CapabilityName,
		Params: actionReq.Params, Reason: actionReq.Reason, RequestedBy: actionReq.RequestedBy,
	})
	if err != nil {
		return fmt.Errorf("failed to encode action request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build action plan request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action plan request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action plan response")
	if err != nil {
		return fmt.Errorf("failed to read action plan response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action plan request", resp.StatusCode, resp.Status, respBody)
	}

	var plan unified.ActionPlan
	if err := decodeActionConfirmationResponse(respBody, &plan, "actionId", "requestId", "allowed"); err != nil {
		return actionConfirmationResponseError("plan", err)
	}
	if strings.TrimSpace(plan.ActionID) == "" || plan.RequestID != actionReq.RequestID {
		return actionConfirmationResponseError("plan", fmt.Errorf("missing actionId or requestId does not match the request"))
	}

	if shouldOutputJSON(cmd, actionIsTerminal(deps)) {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			return fmt.Errorf("failed to write action plan response: %w", err)
		}
		return nil
	}
	return renderActionPlanHuman(cmd.OutOrStdout(), plan, extractOriginSurfaceFromJSON(respBody))
}

func runActionList(cmd *cobra.Command, deps *ActionsDeps, opts actionListOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, "", "")
	if err != nil {
		return err
	}

	view := strings.TrimSpace(opts.View)
	if view == "" {
		view = "pending"
	}
	switch view {
	case "pending", "settled", "all":
	default:
		return fmt.Errorf("view must be pending, settled, or all")
	}
	limitChanged := cmd != nil && cmd.Flags().Changed("limit")
	if (limitChanged || opts.Limit != 0) && (opts.Limit < 1 || opts.Limit > 500) {
		return fmt.Errorf("limit must be between 1 and 500")
	}

	endpoint, err := pulseAPIEndpoint(apiURL, "/actions")
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("view", view)
	if opts.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to build action list request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action list request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action list response")
	if err != nil {
		return fmt.Errorf("failed to read action list response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action list request", resp.StatusCode, resp.Status, respBody)
	}

	var listResp actionListResponse
	if err := decodeJSONBytes(respBody, &listResp); err != nil {
		return fmt.Errorf("failed to decode action list response: %w", err)
	}

	if shouldOutputJSON(cmd, actionIsTerminal(deps)) {
		return writePrettyJSONBytes(cmd.OutOrStdout(), respBody)
	}
	return renderActionListHuman(cmd.OutOrStdout(), listResp)
}

func runActionGet(cmd *cobra.Command, deps *ActionsDeps, actionID string) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, "", "")
	if err != nil {
		return err
	}

	detail, respBody, err := fetchActionDetail(cmd.Context(), deps, apiURL, token, actionID)
	if err != nil {
		return err
	}
	if shouldOutputJSON(cmd, actionIsTerminal(deps)) {
		return writePrettyJSONBytes(cmd.OutOrStdout(), respBody)
	}
	return renderActionDetailHuman(cmd.OutOrStdout(), detail)
}

func runActionRefresh(cmd *cobra.Command, deps *ActionsDeps, opts actionRefreshOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, "", "")
	if err != nil {
		return err
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (pass <action-id> or --action-id)")
	}
	planHash := strings.TrimSpace(opts.PlanHash)
	if planHash == "" {
		return fmt.Errorf("planHash is required (use --plan-hash)")
	}

	endpoint, err := pulseAPIEndpoint(apiURL, "/actions/"+url.PathEscape(actionID)+"/refresh")
	if err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		PlanHash string `json:"planHash"`
	}{
		PlanHash: planHash,
	})
	if err != nil {
		return fmt.Errorf("failed to encode action refresh request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build action refresh request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action refresh response")
	if err != nil {
		return fmt.Errorf("failed to read action refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action refresh request", resp.StatusCode, resp.Status, respBody)
	}

	var detail actionDetailViewResponse
	if err := decodeJSONBytes(respBody, &detail); err != nil {
		return fmt.Errorf("failed to decode action refresh response: %w", err)
	}
	refreshedID := strings.TrimSpace(detail.Audit.ID)
	if refreshedID == "" {
		refreshedID = strings.TrimSpace(detail.Audit.Plan.ActionID)
	}
	if refreshedID == "" {
		return fmt.Errorf("invalid action refresh response: missing refreshed actionId")
	}

	if shouldOutputJSON(cmd, actionIsTerminal(deps)) {
		return writePrettyJSONBytes(cmd.OutOrStdout(), respBody)
	}
	return renderActionRefreshHuman(cmd.OutOrStdout(), detail)
}

func runActionWatch(cmd *cobra.Command, deps *ActionsDeps, opts actionWatchOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, "", "")
	if err != nil {
		return err
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (pass <action-id> or --action-id)")
	}
	if opts.Interval <= 0 {
		return fmt.Errorf("interval must be greater than zero")
	}
	if opts.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	until := strings.TrimSpace(opts.Until)
	if until == "" {
		until = "terminal"
	}
	if until != "terminal" && until != "decided" {
		return fmt.Errorf("until must be terminal or decided")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
	defer cancel()

	jsonMode := shouldOutputJSON(cmd, actionIsTerminal(deps))
	out := cmd.OutOrStdout()
	lastState := ""

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for {
		detail, _, err := fetchActionDetail(ctx, deps, apiURL, token, actionID)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				if lastState != "" {
					return fmt.Errorf("timed out waiting for action %s after %s (last state: %s)", actionID, opts.Timeout, lastState)
				}
				return fmt.Errorf("timed out waiting for action %s after %s", actionID, opts.Timeout)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		state := strings.TrimSpace(string(detail.Audit.State))
		if state != lastState {
			if err := writeActionWatchUpdate(out, detail, jsonMode); err != nil {
				return err
			}
			lastState = state
		}
		if actionWatchDone(state, until) {
			return nil
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				if lastState != "" {
					return fmt.Errorf("timed out waiting for action %s after %s (last state: %s)", actionID, opts.Timeout, lastState)
				}
				return fmt.Errorf("timed out waiting for action %s after %s", actionID, opts.Timeout)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func fetchActionDetail(ctx context.Context, deps *ActionsDeps, apiURL, token, actionID string) (actionDetailViewResponse, []byte, error) {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return actionDetailViewResponse{}, nil, fmt.Errorf("actionId is required (pass <action-id> or --action-id)")
	}
	endpoint, err := pulseAPIEndpoint(apiURL, "/actions/"+url.PathEscape(actionID))
	if err != nil {
		return actionDetailViewResponse{}, nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return actionDetailViewResponse{}, nil, fmt.Errorf("failed to build action detail request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return actionDetailViewResponse{}, nil, fmt.Errorf("action detail request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action detail response")
	if err != nil {
		return actionDetailViewResponse{}, nil, fmt.Errorf("failed to read action detail response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return actionDetailViewResponse{}, nil, apiStatusErrorWithCode("action detail request", resp.StatusCode, resp.Status, respBody)
	}

	var detail actionDetailViewResponse
	if err := decodeJSONBytes(respBody, &detail); err != nil {
		return actionDetailViewResponse{}, nil, fmt.Errorf("failed to decode action detail response: %w", err)
	}
	if strings.TrimSpace(detail.Audit.ID) == "" && strings.TrimSpace(detail.Audit.Plan.ActionID) != "" {
		detail.Audit.ID = strings.TrimSpace(detail.Audit.Plan.ActionID)
	}
	if strings.TrimSpace(detail.Audit.ID) == "" {
		return actionDetailViewResponse{}, nil, fmt.Errorf("invalid action detail response: missing audit id")
	}
	return detail, respBody, nil
}

func actionWatchDone(state, until string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "completed", "failed", "rejected", "expired", "cancelled", "canceled":
		return true
	case "approved":
		return until == "decided"
	default:
		return false
	}
}

func writeActionWatchUpdate(w io.Writer, detail actionDetailViewResponse, jsonMode bool) error {
	if jsonMode {
		event := actionWatchEvent{
			ActionID:       detail.Audit.ID,
			State:          detail.Audit.State,
			CapabilityName: detail.Audit.Request.CapabilityName,
			ResourceID:     detail.Audit.Request.ResourceID,
			PlanHash:       detail.Audit.Plan.PlanHash,
			UpdatedAt:      detail.Audit.UpdatedAt,
			Readiness:      detail.Readiness,
			Result:         detail.Audit.Result,
		}
		if detail.Audit.VerificationOutcome.Status != "" {
			outcome := detail.Audit.VerificationOutcome
			event.VerificationOutcome = &outcome
		}
		return json.NewEncoder(w).Encode(event)
	}
	ts := detail.Audit.UpdatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	resource := formatActionResourceCompact(detail.Audit)
	capability := fallbackDash(detail.Audit.Request.CapabilityName)
	line := fmt.Sprintf("%s  %s  %s  %s  %s",
		ts.UTC().Format(time.RFC3339),
		detail.Audit.ID,
		fallbackDash(string(detail.Audit.State)),
		capability,
		resource,
	)
	if detail.Audit.Result != nil {
		if detail.Audit.Result.Success {
			line += "  outcome=success"
		} else if msg := strings.TrimSpace(detail.Audit.Result.ErrorMessage); msg != "" {
			line += "  error=" + msg
		} else {
			line += "  outcome=failed"
		}
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

func extractOriginSurfaceFromJSON(raw []byte) string {
	var envelope struct {
		Origin json.RawMessage `json:"origin"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(bytes.TrimSpace(envelope.Origin)) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(envelope.Origin, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	var asObj struct {
		Surface string `json:"surface"`
	}
	if err := json.Unmarshal(envelope.Origin, &asObj); err == nil {
		return strings.TrimSpace(asObj.Surface)
	}
	return ""
}

func renderActionPlanHuman(w io.Writer, plan unified.ActionPlan, originSurface string) error {
	if strings.TrimSpace(originSurface) == "" {
		originSurface = "pulse_cli"
	}
	approval := fallbackDash(string(plan.ApprovalPolicy))
	if plan.RequiresApproval {
		approval += " (approval required)"
	}
	blastRadius := "none"
	if len(plan.PredictedBlastRadius) > 0 {
		blastRadius = strings.Join(plan.PredictedBlastRadius, ", ")
	}
	expiresAt := "-"
	if !plan.ExpiresAt.IsZero() {
		expiresAt = plan.ExpiresAt.UTC().Format(time.RFC3339)
	}

	fmt.Fprintf(w, "Action ID:     %s\n", plan.ActionID)
	fmt.Fprintf(w, "Request ID:    %s\n", plan.RequestID)
	fmt.Fprintf(w, "Allowed:       %t\n", plan.Allowed)
	fmt.Fprintf(w, "Approval:      %s\n", approval)
	fmt.Fprintf(w, "Origin:        %s\n", originSurface)
	fmt.Fprintf(w, "Plan hash:     %s\n", fallbackDash(plan.PlanHash))
	fmt.Fprintf(w, "Expires at:    %s\n", expiresAt)
	fmt.Fprintf(w, "Blast radius:  %s\n", blastRadius)
	if msg := strings.TrimSpace(plan.Message); msg != "" {
		fmt.Fprintf(w, "Message:       %s\n", msg)
	}
	return nil
}

func renderActionListHuman(w io.Writer, list actionListResponse) error {
	if len(list.Actions) == 0 {
		_, err := fmt.Fprintln(w, "No actions found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION ID\tSTATE\tCAPABILITY\tRESOURCE\tORIGIN\tREQUESTER\tEXPIRES")
	for _, a := range list.Actions {
		expires := "-"
		if !a.Plan.ExpiresAt.IsZero() {
			expires = a.Plan.ExpiresAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			fallbackDash(a.ID),
			fallbackDash(string(a.State)),
			fallbackDash(a.Request.CapabilityName),
			formatActionResourceCompact(a),
			formatActionOrigin(a.Origin),
			formatActionRequester(a.Request),
			expires,
		)
	}
	return tw.Flush()
}

func renderActionDetailHuman(w io.Writer, detail actionDetailViewResponse) error {
	a := detail.Audit
	expires := "-"
	if !a.Plan.ExpiresAt.IsZero() {
		expires = a.Plan.ExpiresAt.UTC().Format(time.RFC3339)
	}
	blast := formatActionBlastRadius(a)

	fmt.Fprintf(w, "Action ID:     %s\n", fallbackDash(a.ID))
	fmt.Fprintf(w, "State:         %s\n", fallbackDash(string(a.State)))
	fmt.Fprintf(w, "Capability:    %s\n", fallbackDash(a.Request.CapabilityName))
	fmt.Fprintf(w, "Resource:      %s\n", formatActionResourceCompact(a))
	fmt.Fprintf(w, "Origin:        %s\n", formatActionOrigin(a.Origin))
	fmt.Fprintf(w, "Requester:     %s\n", formatActionRequester(a.Request))
	fmt.Fprintf(w, "Plan hash:     %s\n", fallbackDash(a.Plan.PlanHash))
	fmt.Fprintf(w, "Expires at:    %s\n", expires)
	fmt.Fprintf(w, "Blast radius:  %s\n", blast)
	if detail.Readiness != nil {
		status := "not ready"
		if detail.Readiness.Ready {
			status = "ready"
		}
		line := status
		if code := strings.TrimSpace(detail.Readiness.Code); code != "" {
			line += " (" + code + ")"
		}
		if msg := strings.TrimSpace(detail.Readiness.Message); msg != "" {
			line += ": " + msg
		}
		fmt.Fprintf(w, "Readiness:     %s\n", line)
		if rem := strings.TrimSpace(detail.Readiness.Remediation); rem != "" {
			fmt.Fprintf(w, "Remediation:   %s\n", rem)
		}
	}
	if a.Result != nil {
		outcome := "failed"
		if a.Result.Success {
			outcome = "success"
		}
		if outText := strings.TrimSpace(a.Result.Output); outText != "" {
			fmt.Fprintf(w, "Outcome:       %s (%s)\n", outcome, outText)
		} else if errText := strings.TrimSpace(a.Result.ErrorMessage); errText != "" {
			fmt.Fprintf(w, "Outcome:       %s (%s)\n", outcome, errText)
		} else {
			fmt.Fprintf(w, "Outcome:       %s\n", outcome)
		}
	}
	if vLine := formatActionVerification(a); vLine != "" {
		fmt.Fprintf(w, "Verification:  %s\n", vLine)
	}
	if len(detail.Events) > 0 {
		fmt.Fprintf(w, "Events:        %d\n", len(detail.Events))
	}
	if len(detail.Receipt) > 0 {
		receiptSummary := "present"
		if id, ok := detail.Receipt["id"].(string); ok && strings.TrimSpace(id) != "" {
			receiptSummary = strings.TrimSpace(id)
		} else if status, ok := detail.Receipt["status"].(string); ok && strings.TrimSpace(status) != "" {
			receiptSummary = strings.TrimSpace(status)
		}
		fmt.Fprintf(w, "Receipt:       %s\n", receiptSummary)
	}
	return nil
}

func renderActionRefreshHuman(w io.Writer, detail actionDetailViewResponse) error {
	refreshedID := strings.TrimSpace(detail.Audit.ID)
	if refreshedID == "" {
		refreshedID = strings.TrimSpace(detail.Audit.Plan.ActionID)
	}
	expires := "-"
	if !detail.Audit.Plan.ExpiresAt.IsZero() {
		expires = detail.Audit.Plan.ExpiresAt.UTC().Format(time.RFC3339)
	}
	fmt.Fprintf(w, "Action ID:   %s\n", fallbackDash(refreshedID))
	if state := strings.TrimSpace(string(detail.Audit.State)); state != "" {
		fmt.Fprintf(w, "State:       %s\n", state)
	}
	fmt.Fprintf(w, "Plan hash:   %s\n", fallbackDash(detail.Audit.Plan.PlanHash))
	fmt.Fprintf(w, "Expires at:  %s\n", expires)
	return nil
}

func formatActionOrigin(origin *unified.ActionOrigin) string {
	if origin == nil || strings.TrimSpace(origin.Surface) == "" {
		return "-"
	}
	return strings.TrimSpace(origin.Surface)
}

func formatActionRequester(req actionRequestView) string {
	label := strings.TrimSpace(req.Actor.CredentialLabel)
	by := strings.TrimSpace(req.RequestedBy)
	if by == "" {
		by = strings.TrimSpace(req.Actor.SubjectID)
	}
	switch {
	case label != "" && by != "" && label != by:
		return fmt.Sprintf("%s (%s)", by, label)
	case label != "":
		return label
	case by != "":
		return by
	default:
		return "-"
	}
}

func formatActionResourceCompact(a actionAuditView) string {
	id := strings.TrimSpace(a.Request.ResourceID)
	if a.Resource != nil && strings.TrimSpace(a.Resource.Name) != "" {
		name := strings.TrimSpace(a.Resource.Name)
		if id != "" && id != name {
			return fmt.Sprintf("%s (%s)", name, id)
		}
		return name
	}
	return fallbackDash(id)
}

func formatActionBlastRadius(a actionAuditView) string {
	if len(a.BlastRadius) > 0 {
		items := make([]string, 0, len(a.BlastRadius))
		for _, r := range a.BlastRadius {
			name := strings.TrimSpace(r.Name)
			id := strings.TrimSpace(r.ID)
			switch {
			case name != "" && id != "" && name != id:
				items = append(items, fmt.Sprintf("%s (%s)", name, id))
			case name != "":
				items = append(items, name)
			case id != "":
				items = append(items, id)
			}
		}
		if len(items) > 0 {
			return strings.Join(items, ", ")
		}
	}
	if len(a.Plan.PredictedBlastRadius) > 0 {
		return strings.Join(a.Plan.PredictedBlastRadius, ", ")
	}
	return "none"
}

func formatActionVerification(a actionAuditView) string {
	if status := strings.TrimSpace(string(a.VerificationOutcome.Status)); status != "" && a.VerificationOutcome.Status != unified.VerificationUnknown {
		if summary := strings.TrimSpace(a.VerificationOutcome.EvidenceSummary); summary != "" {
			return fmt.Sprintf("%s (%s)", status, summary)
		}
		return status
	}
	v := a.Verification
	if v == nil && a.Result != nil {
		v = a.Result.Verification
	}
	if v == nil {
		return ""
	}
	if !v.Ran {
		if note := strings.TrimSpace(v.Note); note != "" {
			return fmt.Sprintf("not run (%s)", note)
		}
		return "not run"
	}
	status := "failed"
	if v.Success {
		status = "verified"
	}
	if note := strings.TrimSpace(v.Note); note != "" {
		return fmt.Sprintf("%s (%s)", status, note)
	}
	return status
}

func fallbackDash(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "-"
}

func actionIsTerminal(deps *ActionsDeps) func(io.Writer) bool {
	if deps != nil {
		return deps.IsTerminal
	}
	return nil
}

func runActionDecision(cmd *cobra.Command, deps *ActionsDeps, opts actionDecisionOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, opts.APIURL, opts.Token)
	if err != nil {
		return err
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (use --action-id)")
	}
	outcome := unified.ApprovalOutcome(strings.TrimSpace(opts.Outcome))
	if outcome != unified.OutcomeApproved && outcome != unified.OutcomeRejected {
		return fmt.Errorf("outcome must be approved or rejected (use --outcome)")
	}

	endpoint, err := pulseAPIEndpoint(apiURL, "/actions/"+url.PathEscape(actionID)+"/decision")
	if err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		Outcome  unified.ApprovalOutcome `json:"outcome"`
		Reason   string                  `json:"reason,omitempty"`
		PlanHash string                  `json:"planHash,omitempty"`
	}{
		Outcome:  outcome,
		Reason:   strings.TrimSpace(opts.Reason),
		PlanHash: strings.TrimSpace(opts.PlanHash),
	})
	if err != nil {
		return fmt.Errorf("failed to encode action decision: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build action decision request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action decision request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action decision response")
	if err != nil {
		return fmt.Errorf("failed to read action decision response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action decision request", resp.StatusCode, resp.Status, respBody)
	}

	var decision actionDecisionResponse
	if err := decodeActionConfirmationResponse(respBody, &decision, "actionId", "state"); err != nil {
		return actionConfirmationResponseError("decision", err)
	}
	if decision.ActionID != actionID || strings.TrimSpace(string(decision.State)) == "" {
		return actionConfirmationResponseError("decision", fmt.Errorf("actionId does not match the request or state is blank"))
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(decision); err != nil {
		return fmt.Errorf("failed to write action decision response: %w", err)
	}
	return nil
}

func runActionExecution(cmd *cobra.Command, deps *ActionsDeps, opts actionExecutionOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, opts.APIURL, opts.Token)
	if err != nil {
		return err
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (use --action-id)")
	}

	endpoint, err := pulseAPIEndpoint(apiURL, "/actions/"+url.PathEscape(actionID)+"/execute")
	if err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		Reason   string `json:"reason,omitempty"`
		PlanHash string `json:"planHash,omitempty"`
	}{
		Reason:   strings.TrimSpace(opts.Reason),
		PlanHash: strings.TrimSpace(opts.PlanHash),
	})
	if err != nil {
		return fmt.Errorf("failed to encode action execution: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build action execution request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action execution request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action execution response")
	if err != nil {
		return fmt.Errorf("failed to read action execution response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action execution request", resp.StatusCode, resp.Status, respBody)
	}

	var execution actionExecutionResponse
	if err := decodeActionConfirmationResponse(respBody, &execution, "actionId", "state"); err != nil {
		return actionConfirmationResponseError("execution", err)
	}
	if execution.ActionID != actionID || strings.TrimSpace(string(execution.State)) == "" {
		return actionConfirmationResponseError("execution", fmt.Errorf("actionId does not match the request or state is blank"))
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(execution); err != nil {
		return fmt.Errorf("failed to write action execution response: %w", err)
	}
	return nil
}

// Confirm only the response's binding and explicit outcome fields, not a
// particular business outcome: a plan may be disallowed, a decision replay may
// show a later state or another approver, and execution may still be in flight.
// Optional results and additive server fields remain compatible.
func decodeActionConfirmationResponse(data []byte, out any, requiredFields ...string) error {
	var envelope map[string]json.RawMessage
	if err := decodeJSONBytes(data, &envelope); err != nil {
		return fmt.Errorf("expected one JSON object")
	}
	for _, field := range requiredFields {
		value, present := envelope[field]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("expected a non-null %s field", field)
		}
	}
	if err := decodeJSONBytes(data, out); err != nil {
		// Some field decoders (notably timestamps) include the bad value in
		// their errors. It may belong to an unrelated response, not this caller.
		return fmt.Errorf("invalid response field types")
	}
	return nil
}

func actionConfirmationResponseError(operation string, err error) error {
	// The POST may already have taken effect. Never imply that a bad response
	// made it safe to repeat the mutation, or echo an unrelated response body.
	return fmt.Errorf("invalid action %s response: %w; request outcome is unconfirmed; check action audit before retrying", operation, err)
}

func runActionCapabilities(cmd *cobra.Command, deps *ActionsDeps, opts actionCapabilitiesOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, opts.APIURL, opts.Token)
	if err != nil {
		return err
	}

	resourceID := unified.CanonicalResourceID(opts.ResourceID)
	if resourceID == "" {
		return fmt.Errorf("resourceId is required (use --resource-id)")
	}

	endpoint, err := actionResourceFacetsEndpoint(apiURL, resourceID)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to build action capabilities request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action capabilities request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action capabilities response")
	if err != nil {
		return fmt.Errorf("failed to read action capabilities response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action capabilities request", resp.StatusCode, resp.Status, respBody)
	}

	var facets actionResourceFacetsResponse
	if err := decodeJSONBytes(respBody, &facets); err != nil {
		return fmt.Errorf("failed to decode action capabilities response: %w", err)
	}
	if unified.CanonicalResourceID(facets.ResourceID) != resourceID {
		// Empty capabilities may be omitted by the facets API, but its
		// resource binding must be explicit even for an empty result.
		return fmt.Errorf("invalid action capabilities response: resourceId does not match the request")
	}
	output := actionCapabilitiesResponse{
		ResourceID:   resourceID,
		Capabilities: facets.Capabilities,
		Count:        len(facets.Capabilities),
	}
	if output.Capabilities == nil {
		output.Capabilities = []unified.ResourceCapability{}
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to write action capabilities response: %w", err)
	}
	return nil
}

func runActionAudit(cmd *cobra.Command, deps *ActionsDeps, opts actionAuditOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, opts.APIURL, opts.Token)
	if err != nil {
		return err
	}

	query, err := actionAuditQuery(opts.ResourceID, opts.Since, opts.Limit)
	if err != nil {
		return err
	}
	endpoint, err := pulseAPIEndpoint(apiURL, "/audit/actions")
	if err != nil {
		return err
	}
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to build action audit request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action audit request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action audit response")
	if err != nil {
		return fmt.Errorf("failed to read action audit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action audit request", resp.StatusCode, resp.Status, respBody)
	}

	var audits actionAuditListResponse
	if err := decodeActionHistoryResponse(respBody, &audits, "audits"); err != nil {
		return fmt.Errorf("failed to decode action audit response: %w", err)
	}
	if audits.Count != len(audits.Audits) {
		return fmt.Errorf("invalid action audit response: count does not match audits")
	}
	if unified.CanonicalResourceID(audits.ResourceID) != unified.CanonicalResourceID(opts.ResourceID) {
		return fmt.Errorf("invalid action audit response: resourceId does not match the request")
	}
	if audits.Audits == nil {
		audits.Audits = []unified.ActionAuditRecord{}
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(audits); err != nil {
		return fmt.Errorf("failed to write action audit response: %w", err)
	}
	return nil
}

func runActionEvents(cmd *cobra.Command, deps *ActionsDeps, opts actionEventsOptions) error {
	apiURL, token, err := resolveActionTarget(cmd, deps, opts.APIURL, opts.Token)
	if err != nil {
		return err
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (use --action-id)")
	}

	query, err := actionAuditQuery("", opts.Since, opts.Limit)
	if err != nil {
		return err
	}
	endpoint, err := pulseAPIEndpoint(apiURL, "/audit/actions/"+url.PathEscape(actionID)+"/events")
	if err != nil {
		return err
	}
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to build action lifecycle request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := actionHTTPClient(deps).Do(httpReq)
	if err != nil {
		return fmt.Errorf("action lifecycle request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxPulseAPIResponseBytes, "action lifecycle response")
	if err != nil {
		return fmt.Errorf("failed to read action lifecycle response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiStatusErrorWithCode("action lifecycle request", resp.StatusCode, resp.Status, respBody)
	}

	var events actionLifecycleEventsResponse
	if err := decodeActionHistoryResponse(respBody, &events, "events"); err != nil {
		return fmt.Errorf("failed to decode action lifecycle response: %w", err)
	}
	if events.Count != len(events.Events) {
		return fmt.Errorf("invalid action lifecycle response: count does not match events")
	}
	if events.ActionID != actionID {
		return fmt.Errorf("invalid action lifecycle response: actionId does not match the request")
	}
	if events.Events == nil {
		events.Events = []unified.ActionLifecycleEvent{}
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(events); err != nil {
		return fmt.Errorf("failed to write action lifecycle response: %w", err)
	}
	return nil
}

// Empty history must be explicit, not inferred from a missing response. Keep
// accepting a present null collection with count zero: existing servers may
// encode an empty store that way. Callers check the count and request binding
// before emitting any history JSON.
func decodeActionHistoryResponse(data []byte, out any, collection string) error {
	var envelope map[string]json.RawMessage
	if err := decodeJSONBytes(data, &envelope); err != nil {
		return err
	}
	_, hasCollection := envelope[collection]
	count, hasCount := envelope["count"]
	if !hasCollection || !hasCount || bytes.Equal(bytes.TrimSpace(count), []byte("null")) {
		return fmt.Errorf("expected %s and a non-null count field", collection)
	}
	return decodeJSONBytes(data, out)
}

func buildActionRequest(cmd *cobra.Command, opts actionPlanOptions) (unified.ActionRequest, error) {
	var req unified.ActionRequest
	if strings.TrimSpace(opts.RequestFile) != "" {
		data, err := readActionRequestFile(cmd, opts.RequestFile)
		if err != nil {
			return unified.ActionRequest{}, err
		}
		if err := decodeJSONBytes(data, &req); err != nil {
			return unified.ActionRequest{}, fmt.Errorf("failed to decode action request file: %w", err)
		}
	}

	if strings.TrimSpace(opts.RequestID) != "" {
		req.RequestID = opts.RequestID
	}
	if strings.TrimSpace(opts.ResourceID) != "" {
		req.ResourceID = opts.ResourceID
	}
	if strings.TrimSpace(opts.CapabilityName) != "" {
		req.CapabilityName = opts.CapabilityName
	}
	if strings.TrimSpace(opts.Reason) != "" {
		req.Reason = opts.Reason
	}
	if strings.TrimSpace(opts.RequestedBy) != "" {
		req.RequestedBy = opts.RequestedBy
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}

	if strings.TrimSpace(opts.ParamsJSON) != "" {
		params, err := decodeActionParamsObject([]byte(opts.ParamsJSON), "--params-json")
		if err != nil {
			return unified.ActionRequest{}, err
		}
		for key, value := range params {
			req.Params[key] = value
		}
	}

	for _, raw := range opts.Params {
		key, value, err := parseActionParam(raw)
		if err != nil {
			return unified.ActionRequest{}, err
		}
		req.Params[key] = value
	}

	req = normalizeCLIActionRequest(req)
	if err := validateCLIActionRequest(req); err != nil {
		return unified.ActionRequest{}, err
	}
	return req, nil
}

func normalizeCLIActionRequest(req unified.ActionRequest) unified.ActionRequest {
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	req.CapabilityName = strings.TrimSpace(req.CapabilityName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	return req
}

func validateCLIActionRequest(req unified.ActionRequest) error {
	required := []struct {
		field string
		value string
		flag  string
	}{
		{field: "requestId", value: req.RequestID, flag: "--request-id"},
		{field: "resourceId", value: req.ResourceID, flag: "--resource-id"},
		{field: "capabilityName", value: req.CapabilityName, flag: "--capability"},
		{field: "reason", value: req.Reason, flag: "--reason"},
	}
	for _, item := range required {
		if item.value == "" {
			return fmt.Errorf("%s is required (use %s or --request-file)", item.field, item.flag)
		}
	}
	return nil
}

func readActionRequestFile(cmd *cobra.Command, path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "-" {
		return ReadBoundedHTTPBody(cmd.InOrStdin(), -1, maxActionRequestBytes, "action request body")
	}
	data, err := ReadBoundedRegularFile(path, maxActionRequestBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read action request file: %w", err)
	}
	return data, nil
}

func parseActionParam(raw string) (string, any, error) {
	key, valueText, ok := strings.Cut(raw, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return "", nil, fmt.Errorf("--param must use key=value")
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

func decodeActionParamsObject(data []byte, source string) (map[string]any, error) {
	var params map[string]any
	if err := decodeJSONBytes(data, &params); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", source, err)
	}
	if params == nil {
		return nil, fmt.Errorf("%s must be a JSON object", source)
	}
	for key := range params {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s contains an empty param name", source)
		}
	}
	return params, nil
}

func actionPlanEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("api url is required (use --api-url or PULSE_API_URL)")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid api url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid api url: scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid api url: host is required")
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = "/api/actions/plan"
	case path == "/api/actions/plan" || strings.HasSuffix(path, "/api/actions/plan"):
		parsed.Path = path
	case path == "/api" || strings.HasSuffix(path, "/api"):
		parsed.Path = path + "/actions/plan"
	default:
		parsed.Path = path + "/api/actions/plan"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func actionResourceFacetsEndpoint(raw, resourceID string) (string, error) {
	resourceID = unified.CanonicalResourceID(resourceID)
	if resourceID == "" {
		return "", fmt.Errorf("resourceId is required (use --resource-id)")
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("api url is required (use --api-url or PULSE_API_URL)")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid api url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid api url: scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid api url: host is required")
	}

	path := strings.TrimRight(parsed.Path, "/")
	resourcePath := "/resources/" + url.PathEscape(resourceID) + "/facets"
	switch {
	case path == "":
		parsed.Path = "/api" + resourcePath
	case path == "/api" || strings.HasSuffix(path, "/api"):
		parsed.Path = path + resourcePath
	default:
		parsed.Path = path + "/api" + resourcePath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func actionAuditQuery(resourceID, since string, limit int) (url.Values, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	query := url.Values{}
	if resourceID = unified.CanonicalResourceID(resourceID); resourceID != "" {
		query.Set("resourceId", resourceID)
	}
	if since = strings.TrimSpace(since); since != "" {
		parsed, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return nil, fmt.Errorf("since must be RFC3339: %w", err)
		}
		query.Set("since", parsed.UTC().Format(time.RFC3339))
	}
	query.Set("limit", fmt.Sprintf("%d", limit))
	return query, nil
}

func actionHTTPClient(deps *ActionsDeps) HTTPDoer {
	if deps != nil {
		return cliHTTPClient(deps.HTTPClient)
	}
	return cliHTTPClient(nil)
}

func actionGetenv(deps *ActionsDeps, key string) string {
	if deps != nil {
		return cliGetenv(deps.Getenv, key)
	}
	return cliGetenv(nil, key)
}
