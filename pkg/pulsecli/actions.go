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

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/spf13/cobra"
)

const (
	defaultActionsAPIURL       = "http://127.0.0.1:7655"
	maxActionRequestBytes      = 1 << 20
	maxActionPlanResponseBytes = 1 << 20
	maxActionErrorBodyChars    = 4096
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

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

func newActionsCmd(deps *ActionsDeps) *cobra.Command {
	actionsCmd := &cobra.Command{
		Use:   "actions",
		Short: "Plan governed Pulse actions",
	}

	opts := actionPlanOptions{
		APIURL: strings.TrimSpace(actionGetenv(deps, "PULSE_API_URL")),
		Token:  strings.TrimSpace(actionGetenv(deps, "PULSE_API_TOKEN")),
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultActionsAPIURL
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
	return actionsCmd
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

	respBody, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxActionPlanResponseBytes, "action plan response")
	if err != nil {
		return fmt.Errorf("failed to read action plan response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return actionStatusError(resp.Status, respBody)
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

func actionHTTPClient(deps *ActionsDeps) HTTPDoer {
	if deps != nil && deps.HTTPClient != nil {
		return deps.HTTPClient
	}
	return http.DefaultClient
}

func actionGetenv(deps *ActionsDeps, key string) string {
	if deps != nil && deps.Getenv != nil {
		return deps.Getenv(key)
	}
	return os.Getenv(key)
}

func actionStatusError(status string, body []byte) error {
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("action plan request failed: %s", status)
	}
	if len(message) > maxActionErrorBodyChars {
		message = message[:maxActionErrorBodyChars] + "..."
	}
	return fmt.Errorf("action plan request failed: %s: %s", status, message)
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
