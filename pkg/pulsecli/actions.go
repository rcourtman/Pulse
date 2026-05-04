package pulsecli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/spf13/cobra"
)

const (
	maxActionRequestBytes = 1 << 20
)

type ActionsDeps struct {
	HTTPClient HTTPDoer
	Getenv     func(string) string
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
}

type actionExecutionOptions struct {
	APIURL   string
	Token    string
	ActionID string
	Reason   string
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

func newActionsCmd(deps *ActionsDeps) *cobra.Command {
	actionsCmd := &cobra.Command{
		Use:   "actions",
		Short: "Inspect and plan governed Pulse actions",
	}

	opts := actionPlanOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Request a deterministic pre-execution action plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionPlan(cmd, deps, opts)
		},
	}

	planCmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	planCmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	planCmd.Flags().StringVar(&opts.RequestFile, "request-file", "", "JSON ActionRequest file; use - for stdin")
	planCmd.Flags().StringVar(&opts.RequestID, "request-id", "", "caller idempotency key")
	planCmd.Flags().StringVar(&opts.ResourceID, "resource-id", "", "canonical unified resource id")
	planCmd.Flags().StringVar(&opts.CapabilityName, "capability", "", "resource capability name to plan")
	planCmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for the requested action")
	planCmd.Flags().StringVar(&opts.RequestedBy, "requested-by", "", "requester identity, for example agent:oncall-helper")
	planCmd.Flags().StringVar(&opts.ParamsJSON, "params-json", "", "JSON object merged into request params")
	planCmd.Flags().StringArrayVar(&opts.Params, "param", nil, "request param as key=value; repeatable, values parse as JSON when possible")

	actionsCmd.AddCommand(planCmd)
	actionsCmd.AddCommand(newActionCapabilitiesCmd(deps))
	actionsCmd.AddCommand(newActionDecisionCmd(deps))
	actionsCmd.AddCommand(newActionExecutionCmd(deps))
	actionsCmd.AddCommand(newActionAuditCmd(deps))
	actionsCmd.AddCommand(newActionEventsCmd(deps))
	return actionsCmd
}

func newActionCapabilitiesCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionCapabilitiesOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "List capabilities advertised by a unified resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionCapabilities(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	cmd.Flags().StringVar(&opts.ResourceID, "resource-id", "", "canonical unified resource id")
	return cmd
}

func newActionDecisionCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionDecisionOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	cmd := &cobra.Command{
		Use:   "decide",
		Short: "Approve or reject a governed action plan without execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionDecision(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id")
	cmd.Flags().StringVar(&opts.Outcome, "outcome", "", "decision outcome: approved or rejected")
	cmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for the decision")
	return cmd
}

func newActionExecutionCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionExecutionOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute a governed action through the API action contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionExecution(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id")
	cmd.Flags().StringVar(&opts.Reason, "reason", "", "audit reason for starting execution")
	return cmd
}

func newActionAuditCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionAuditOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
		Limit:  100,
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List governed action audit records",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionAudit(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	cmd.Flags().StringVar(&opts.ResourceID, "resource-id", "", "canonical unified resource id")
	cmd.Flags().StringVar(&opts.Since, "since", "", "RFC3339 lower bound for audit records")
	cmd.Flags().IntVar(&opts.Limit, "limit", opts.Limit, "maximum audit records to return")
	return cmd
}

func newActionEventsCmd(deps *ActionsDeps) *cobra.Command {
	opts := actionEventsOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
		Limit:  100,
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultPulseAPIURL
	}

	cmd := &cobra.Command{
		Use:   "events",
		Short: "List lifecycle events for a governed action",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionEvents(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.APIURL, "api-url", opts.APIURL, "Pulse server URL or /api base URL")
	cmd.Flags().StringVar(&opts.Token, "token", opts.Token, "Pulse API token; defaults to PULSE_API_TOKEN")
	cmd.Flags().StringVar(&opts.ActionID, "action-id", "", "governed action id")
	cmd.Flags().StringVar(&opts.Since, "since", "", "RFC3339 lower bound for lifecycle events")
	cmd.Flags().IntVar(&opts.Limit, "limit", opts.Limit, "maximum lifecycle events to return")
	return cmd
}

func runActionPlan(cmd *cobra.Command, deps *ActionsDeps, opts actionPlanOptions) error {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	endpoint, err := actionPlanEndpoint(opts.APIURL)
	if err != nil {
		return err
	}

	actionReq, err := buildActionRequest(cmd, opts)
	if err != nil {
		return err
	}

	body, err := json.Marshal(actionReq)
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
		return apiStatusError("action plan request", resp.Status, respBody)
	}

	var plan unified.ActionPlan
	if err := decodeJSONBytes(respBody, &plan); err != nil {
		return fmt.Errorf("failed to decode action plan response: %w", err)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		return fmt.Errorf("failed to write action plan response: %w", err)
	}
	return nil
}

func runActionDecision(cmd *cobra.Command, deps *ActionsDeps, opts actionDecisionOptions) error {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (use --action-id)")
	}
	outcome := unified.ApprovalOutcome(strings.TrimSpace(opts.Outcome))
	if outcome != unified.OutcomeApproved && outcome != unified.OutcomeRejected {
		return fmt.Errorf("outcome must be approved or rejected (use --outcome)")
	}

	endpoint, err := pulseAPIEndpoint(opts.APIURL, "/actions/"+url.PathEscape(actionID)+"/decision")
	if err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		Outcome unified.ApprovalOutcome `json:"outcome"`
		Reason  string                  `json:"reason,omitempty"`
	}{
		Outcome: outcome,
		Reason:  strings.TrimSpace(opts.Reason),
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
		return apiStatusError("action decision request", resp.Status, respBody)
	}

	var decision actionDecisionResponse
	if err := decodeJSONBytes(respBody, &decision); err != nil {
		return fmt.Errorf("failed to decode action decision response: %w", err)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(decision); err != nil {
		return fmt.Errorf("failed to write action decision response: %w", err)
	}
	return nil
}

func runActionExecution(cmd *cobra.Command, deps *ActionsDeps, opts actionExecutionOptions) error {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (use --action-id)")
	}

	endpoint, err := pulseAPIEndpoint(opts.APIURL, "/actions/"+url.PathEscape(actionID)+"/execute")
	if err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		Reason string `json:"reason,omitempty"`
	}{
		Reason: strings.TrimSpace(opts.Reason),
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
		return apiStatusError("action execution request", resp.Status, respBody)
	}

	var execution actionExecutionResponse
	if err := decodeJSONBytes(respBody, &execution); err != nil {
		return fmt.Errorf("failed to decode action execution response: %w", err)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(execution); err != nil {
		return fmt.Errorf("failed to write action execution response: %w", err)
	}
	return nil
}

func runActionCapabilities(cmd *cobra.Command, deps *ActionsDeps, opts actionCapabilitiesOptions) error {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	resourceID := unified.CanonicalResourceID(opts.ResourceID)
	if resourceID == "" {
		return fmt.Errorf("resourceId is required (use --resource-id)")
	}

	endpoint, err := actionResourceFacetsEndpoint(opts.APIURL, resourceID)
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
		return apiStatusError("action capabilities request", resp.Status, respBody)
	}

	var facets actionResourceFacetsResponse
	if err := decodeJSONBytes(respBody, &facets); err != nil {
		return fmt.Errorf("failed to decode action capabilities response: %w", err)
	}
	output := actionCapabilitiesResponse{
		ResourceID:   resourceID,
		Capabilities: facets.Capabilities,
		Count:        len(facets.Capabilities),
	}
	if strings.TrimSpace(facets.ResourceID) != "" {
		if canonical := unified.CanonicalResourceID(facets.ResourceID); canonical != "" {
			output.ResourceID = canonical
		}
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
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	query, err := actionAuditQuery(opts.ResourceID, opts.Since, opts.Limit)
	if err != nil {
		return err
	}
	endpoint, err := pulseAPIEndpoint(opts.APIURL, "/audit/actions")
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
		return apiStatusError("action audit request", resp.Status, respBody)
	}

	var audits actionAuditListResponse
	if err := decodeJSONBytes(respBody, &audits); err != nil {
		return fmt.Errorf("failed to decode action audit response: %w", err)
	}
	if audits.Audits == nil {
		audits.Audits = []unified.ActionAuditRecord{}
	}
	audits.Count = len(audits.Audits)

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(audits); err != nil {
		return fmt.Errorf("failed to write action audit response: %w", err)
	}
	return nil
}

func runActionEvents(cmd *cobra.Command, deps *ActionsDeps, opts actionEventsOptions) error {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return fmt.Errorf("api token is required (use --token or PULSE_API_TOKEN)")
	}

	actionID := strings.TrimSpace(opts.ActionID)
	if actionID == "" {
		return fmt.Errorf("actionId is required (use --action-id)")
	}

	query, err := actionAuditQuery("", opts.Since, opts.Limit)
	if err != nil {
		return err
	}
	endpoint, err := pulseAPIEndpoint(opts.APIURL, "/audit/actions/"+url.PathEscape(actionID)+"/events")
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
		return apiStatusError("action lifecycle request", resp.Status, respBody)
	}

	var events actionLifecycleEventsResponse
	if err := decodeJSONBytes(respBody, &events); err != nil {
		return fmt.Errorf("failed to decode action lifecycle response: %w", err)
	}
	if strings.TrimSpace(events.ActionID) == "" {
		events.ActionID = actionID
	}
	if events.Events == nil {
		events.Events = []unified.ActionLifecycleEvent{}
	}
	events.Count = len(events.Events)

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(events); err != nil {
		return fmt.Errorf("failed to write action lifecycle response: %w", err)
	}
	return nil
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
		{field: "requestedBy", value: req.RequestedBy, flag: "--requested-by"},
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
