package qualification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

type ClientConfig struct {
	BaseURL  string
	Username string
	Password string
	Timeout  time.Duration
}

type PulseClient struct {
	config ClientConfig
	client *http.Client
}

func NewPulseClient(config ClientConfig) (*PulseClient, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" {
		return nil, errors.New("Pulse base URL is required")
	}
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Pulse base URL %q", config.BaseURL)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Pulse base URL must not contain credentials, query parameters, or a fragment")
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Minute
	}
	return &PulseClient{config: config, client: &http.Client{Timeout: config.Timeout}}, nil
}

type AISettings struct {
	Enabled       bool   `json:"enabled"`
	Model         string `json:"model"`
	PatrolModel   string `json:"patrol_model"`
	PatrolEnabled bool   `json:"patrol_enabled"`
}

type PulseVersion struct {
	Version       string `json:"version"`
	Build         string `json:"build"`
	Runtime       string `json:"runtime"`
	IsDevelopment bool   `json:"isDevelopment"`
}

func (s AISettings) EffectivePatrolModel() string {
	if value := strings.TrimSpace(s.PatrolModel); value != "" {
		return value
	}
	return strings.TrimSpace(s.Model)
}

type PatrolReadiness struct {
	Status   string `json:"status"`
	Ready    bool   `json:"ready"`
	Summary  string `json:"summary"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type PatrolStatus struct {
	RuntimeState string          `json:"runtime_state"`
	Running      bool            `json:"running"`
	Enabled      bool            `json:"enabled"`
	Healthy      bool            `json:"healthy"`
	Readiness    PatrolReadiness `json:"readiness"`
}

type PatrolAutonomy struct {
	AutonomyLevel          string `json:"autonomy_level"`
	EffectiveAutonomyLevel string `json:"effective_autonomy_level"`
}

func (a PatrolAutonomy) Effective() string {
	if value := strings.TrimSpace(a.EffectiveAutonomyLevel); value != "" {
		return value
	}
	return strings.TrimSpace(a.AutonomyLevel)
}

type Resource struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Technology string          `json:"technology"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	LastSeen   any             `json:"lastSeen,omitempty"`
	Labels     map[string]any  `json:"labels,omitempty"`
	Docker     *DockerResource `json:"docker,omitempty"`
}

type DockerResource struct {
	ContainerState string `json:"containerState,omitempty"`
	Health         string `json:"health,omitempty"`
	RestartCount   int    `json:"restartCount,omitempty"`
}

type Finding struct {
	ID             string     `json:"id"`
	Key            string     `json:"key"`
	Severity       string     `json:"severity"`
	Category       string     `json:"category"`
	ResourceID     string     `json:"resource_id"`
	ResourceName   string     `json:"resource_name"`
	ResourceType   string     `json:"resource_type"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Impact         string     `json:"impact,omitempty"`
	Recommendation string     `json:"recommendation"`
	Evidence       string     `json:"evidence"`
	Source         string     `json:"source,omitempty"`
	DetectedAt     time.Time  `json:"detected_at"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	ToolName  string `json:"tool_name"`
	Input     string `json:"input"`
	Output    string `json:"output"`
	Success   bool   `json:"success"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Duration  int64  `json:"duration_ms"`
}

type PatrolFindingAssessment struct {
	FindingID  string    `json:"finding_id"`
	Verdict    string    `json:"verdict"`
	Evidence   string    `json:"evidence"`
	Reason     string    `json:"reason"`
	AssessedAt time.Time `json:"assessed_at"`
}

type PatrolRun struct {
	ID                        string                    `json:"id"`
	StartedAt                 time.Time                 `json:"started_at"`
	CompletedAt               time.Time                 `json:"completed_at"`
	DurationMs                int64                     `json:"duration_ms"`
	Type                      string                    `json:"type"`
	TriggerReason             string                    `json:"trigger_reason"`
	ScopeResourceIDs          []string                  `json:"scope_resource_ids,omitempty"`
	EffectiveScopeResourceIDs []string                  `json:"effective_scope_resource_ids,omitempty"`
	ResourcesChecked          int                       `json:"resources_checked"`
	NewFindings               int                       `json:"new_findings"`
	ExistingFindings          int                       `json:"existing_findings"`
	RejectedFindings          int                       `json:"rejected_findings"`
	ResolvedFindings          int                       `json:"resolved_findings"`
	FindingsSummary           string                    `json:"findings_summary"`
	FindingIDs                []string                  `json:"finding_ids"`
	FindingAssessments        []PatrolFindingAssessment `json:"finding_assessments,omitempty"`
	ErrorCount                int                       `json:"error_count"`
	Status                    string                    `json:"status"`
	ErrorSummary              string                    `json:"error_summary,omitempty"`
	AIAnalysis                string                    `json:"ai_analysis,omitempty"`
	InputTokens               int                       `json:"input_tokens"`
	OutputTokens              int                       `json:"output_tokens"`
	ToolCalls                 []ToolCall                `json:"tool_calls,omitempty"`
	ToolCallCount             int                       `json:"tool_call_count"`
}

func (c *PulseClient) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.config.BaseURL+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		return &HTTPError{StatusCode: resp.StatusCode, Path: path, Body: sanitizeArtifactText(string(payload))}
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

type HTTPError struct {
	StatusCode int
	Path       string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Pulse API %s returned %d: %s", e.Path, e.StatusCode, e.Body)
}

func (c *PulseClient) Settings(ctx context.Context) (AISettings, error) {
	var settings AISettings
	err := c.request(ctx, http.MethodGet, "/api/settings/ai", nil, &settings)
	return settings, err
}

func (c *PulseClient) Version(ctx context.Context) (PulseVersion, error) {
	var version PulseVersion
	err := c.request(ctx, http.MethodGet, "/api/version", nil, &version)
	return version, err
}

func (c *PulseClient) Status(ctx context.Context) (PatrolStatus, error) {
	var status PatrolStatus
	err := c.request(ctx, http.MethodGet, "/api/ai/patrol/status", nil, &status)
	return status, err
}

func (c *PulseClient) Autonomy(ctx context.Context) (PatrolAutonomy, error) {
	var autonomy PatrolAutonomy
	err := c.request(ctx, http.MethodGet, "/api/ai/patrol/autonomy", nil, &autonomy)
	return autonomy, err
}

func (c *PulseClient) SetPatrolModel(ctx context.Context, model string) error {
	return c.request(ctx, http.MethodPut, "/api/settings/ai/update", map[string]any{"patrol_model": strings.TrimSpace(model)}, nil)
}

func (c *PulseClient) OverridePatrolModel(ctx context.Context, model string) (func(context.Context) error, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return func(context.Context) error { return nil }, nil
	}
	settings, err := c.Settings(ctx)
	if err != nil {
		return nil, err
	}
	previous := settings.PatrolModel
	if previous == model {
		return func(context.Context) error { return nil }, nil
	}
	if err := c.SetPatrolModel(ctx, model); err != nil {
		return nil, err
	}
	return func(restoreCtx context.Context) error { return c.SetPatrolModel(restoreCtx, previous) }, nil
}

func (c *PulseClient) Resources(ctx context.Context) ([]Resource, error) {
	var response struct {
		Data []Resource `json:"data"`
	}
	err := c.request(ctx, http.MethodGet, "/api/resources?limit=1000", nil, &response)
	return response.Data, err
}

func (c *PulseClient) WaitForResources(ctx context.Context, names map[string]string, timeout, poll time.Duration) (map[string]Resource, error) {
	return c.WaitForResourcesMatching(ctx, names, timeout, poll, nil)
}

func (c *PulseClient) WaitForResourcesMatching(ctx context.Context, names map[string]string, timeout, poll time.Duration, validate func(map[string]Resource) error) (map[string]Resource, error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if poll <= 0 {
		poll = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastValidationErr error
	for {
		resources, err := c.Resources(ctx)
		if err == nil {
			matched := make(map[string]Resource, len(names))
			for alias, name := range names {
				for _, resource := range resources {
					if resource.Name == name {
						matched[alias] = resource
						break
					}
				}
			}
			if len(matched) == len(names) {
				if validate == nil {
					return matched, nil
				}
				if validationErr := validate(matched); validationErr == nil {
					return matched, nil
				} else {
					lastValidationErr = validationErr
				}
			}
		}
		if time.Now().After(deadline) {
			if lastValidationErr != nil {
				return nil, fmt.Errorf("collection exposed exact resources but did not converge to scenario-owned fault state before %s: %w", deadline.UTC().Format(time.RFC3339), lastValidationErr)
			}
			return nil, fmt.Errorf("collection did not expose all %d exact resource names before %s", len(names), deadline.UTC().Format(time.RFC3339))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (c *PulseClient) Findings(ctx context.Context) ([]Finding, error) {
	var findings []Finding
	err := c.request(ctx, http.MethodGet, "/api/ai/patrol/findings", nil, &findings)
	return findings, err
}

func (c *PulseClient) Runs(ctx context.Context) ([]PatrolRun, error) {
	var runs []PatrolRun
	err := c.request(ctx, http.MethodGet, "/api/ai/patrol/runs?limit=100&include=tool_calls", nil, &runs)
	return runs, err
}

func (c *PulseClient) Run(ctx context.Context, runID string) (PatrolRun, error) {
	var run PatrolRun
	err := c.request(ctx, http.MethodGet, "/api/ai/patrol/runs/"+url.PathEscape(runID)+"?include=tool_calls", nil, &run)
	return run, err
}

func (c *PulseClient) Trigger(ctx context.Context, resourceIDs []string, _ string) error {
	var body any
	if len(resourceIDs) > 0 {
		body = map[string]any{"resource_ids": resourceIDs}
	}
	return c.request(ctx, http.MethodPost, "/api/ai/patrol/run", body, nil)
}

func (c *PulseClient) TriggerAndWait(ctx context.Context, resourceIDs []string, contextText string, timeout time.Duration) (PatrolRun, error) {
	before, err := c.Runs(ctx)
	if err != nil {
		return PatrolRun{}, err
	}
	known := make(map[string]struct{}, len(before))
	for _, run := range before {
		known[run.ID] = struct{}{}
	}
	triggeredAt := time.Now().UTC()
	if err := c.Trigger(ctx, resourceIDs, contextText); err != nil {
		return PatrolRun{}, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		runs, runErr := c.Runs(ctx)
		if runErr == nil {
			sort.Slice(runs, func(i, j int) bool { return runs[i].StartedAt.After(runs[j].StartedAt) })
			for _, candidate := range runs {
				if _, exists := known[candidate.ID]; exists || candidate.CompletedAt.IsZero() {
					continue
				}
				if candidate.StartedAt.Before(triggeredAt.Add(-2 * time.Second)) {
					continue
				}
				if len(resourceIDs) > 0 && !intersects(candidate.EffectiveScopeResourceIDs, resourceIDs) && !intersects(candidate.ScopeResourceIDs, resourceIDs) {
					continue
				}
				return c.Run(ctx, candidate.ID)
			}
		}
		if time.Now().After(deadline) {
			return PatrolRun{}, fmt.Errorf("no completed Patrol run associated with trigger at %s", triggeredAt.Format(time.RFC3339Nano))
		}
		select {
		case <-ctx.Done():
			return PatrolRun{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *PulseClient) Investigation(ctx context.Context, findingID string) (aicontracts.InvestigationSession, error) {
	var result aicontracts.InvestigationSession
	err := c.request(ctx, http.MethodGet, "/api/ai/findings/"+url.PathEscape(findingID)+"/investigation", nil, &result)
	return result, err
}

func (c *PulseClient) WaitForInvestigation(ctx context.Context, findingID string, timeout time.Duration) (aicontracts.InvestigationSession, error) {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		investigation, err := c.Investigation(ctx, findingID)
		if err == nil {
			status := strings.ToLower(strings.TrimSpace(string(investigation.Status)))
			if status != "" && status != "pending" && status != "running" {
				return investigation, nil
			}
		} else {
			var apiErr *HTTPError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
				return aicontracts.InvestigationSession{}, err
			}
		}
		if time.Now().After(deadline) {
			return aicontracts.InvestigationSession{}, fmt.Errorf("investigation for finding %s did not complete", findingID)
		}
		select {
		case <-ctx.Done():
			return aicontracts.InvestigationSession{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

type ActionProjection struct {
	unifiedresources.ActionAuditRecord
	Resource *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"resource,omitempty"`
}

type ActionDetail struct {
	Audit    ActionProjection                        `json:"audit"`
	Events   []unifiedresources.ActionLifecycleEvent `json:"events"`
	ReadOnly bool                                    `json:"readOnly"`
}

func (c *PulseClient) Actions(ctx context.Context, view string) ([]ActionProjection, error) {
	if view == "" {
		view = "pending"
	}
	var response struct {
		Actions []ActionProjection `json:"actions"`
	}
	err := c.request(ctx, http.MethodGet, "/api/actions?view="+url.QueryEscape(view)+"&limit=500", nil, &response)
	return response.Actions, err
}

func (c *PulseClient) Action(ctx context.Context, actionID string) (ActionDetail, error) {
	var detail ActionDetail
	err := c.request(ctx, http.MethodGet, "/api/actions/"+url.PathEscape(actionID), nil, &detail)
	return detail, err
}

func (c *PulseClient) DecideAction(ctx context.Context, actionID, outcome, reason, planHash string) (ActionProjection, error) {
	var response struct {
		Audit ActionProjection `json:"audit"`
	}
	err := c.request(ctx, http.MethodPost, "/api/actions/"+url.PathEscape(actionID)+"/decision", map[string]any{
		"outcome":  outcome,
		"reason":   reason,
		"planHash": planHash,
	}, &response)
	return response.Audit, err
}

func (c *PulseClient) ExecuteAction(ctx context.Context, actionID, reason, planHash string) (ActionProjection, error) {
	var response struct {
		Audit ActionProjection `json:"audit"`
	}
	err := c.request(ctx, http.MethodPost, "/api/actions/"+url.PathEscape(actionID)+"/execute", map[string]any{
		"reason":   reason,
		"planHash": planHash,
	}, &response)
	return response.Audit, err
}

func (c *PulseClient) WaitForAction(ctx context.Context, actionID string, timeout time.Duration) (ActionDetail, error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		detail, err := c.Action(ctx, actionID)
		if err == nil {
			switch detail.Audit.State {
			case unifiedresources.ActionStateCompleted, unifiedresources.ActionStateFailed,
				unifiedresources.ActionStateRejected, unifiedresources.ActionStateExpired:
				return detail, nil
			}
		} else {
			return ActionDetail{}, err
		}
		if time.Now().After(deadline) {
			return ActionDetail{}, fmt.Errorf("action %s did not reach a terminal state", actionID)
		}
		select {
		case <-ctx.Done():
			return ActionDetail{}, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func filterRunFindings(before, after []Finding, run PatrolRun, resourceIDs map[string]Resource, triggeredAt time.Time) []Finding {
	wanted := make(map[string]struct{}, len(run.FindingIDs))
	for _, id := range run.FindingIDs {
		wanted[id] = struct{}{}
	}
	beforeSeen := make(map[string]time.Time, len(before))
	for _, finding := range before {
		beforeSeen[finding.ID] = finding.LastSeenAt
	}
	resources := make(map[string]struct{}, len(resourceIDs))
	for _, resource := range resourceIDs {
		resources[resource.ID] = struct{}{}
	}
	var result []Finding
	for _, finding := range after {
		_, runOwned := wanted[finding.ID]
		_, resourceOwned := resources[finding.ResourceID]
		previousSeen, existedBefore := beforeSeen[finding.ID]
		updated := (existedBefore && finding.LastSeenAt.After(previousSeen)) ||
			(!existedBefore && (finding.LastSeenAt.After(triggeredAt.Add(-2*time.Second)) || finding.DetectedAt.After(triggeredAt.Add(-2*time.Second))))
		if runOwned || (resourceOwned && updated) {
			result = append(result, finding)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func intersects(left, right []string) bool {
	wanted := make(map[string]struct{}, len(right))
	for _, value := range right {
		wanted[value] = struct{}{}
	}
	for _, value := range left {
		if _, ok := wanted[value]; ok {
			return true
		}
	}
	return false
}
