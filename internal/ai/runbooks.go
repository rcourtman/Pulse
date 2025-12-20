package ai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rs/zerolog/log"
)

type RunbookRisk string

const (
	RunbookRiskLow    RunbookRisk = "low"
	RunbookRiskMedium RunbookRisk = "medium"
	RunbookRiskHigh   RunbookRisk = "high"
)

const runbookVerifierDiskUsage = "disk-usage"

var (
	ErrRunbookNotFound      = errors.New("runbook not found")
	ErrRunbookNotApplicable = errors.New("runbook does not apply to finding")
)

type RunbookInfo struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Risk        RunbookRisk `json:"risk"`
}

type RunbookStep struct {
	Name         string
	Command      string
	RunOnHost    bool
	AllowFailure bool
}

type RunbookVerification struct {
	Name         string
	Command      string
	RunOnHost    bool
	SuccessRegex string
	FailureRegex string
	Verifier     string
}

type Runbook struct {
	ID             string
	Title          string
	Description    string
	Risk           RunbookRisk
	FindingKeys    []string
	ResourceTypes  []string
	Steps          []RunbookStep
	Verification   *RunbookVerification
	ResolutionNote string
}

type RunbookStepResult struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

type RunbookExecutionResult struct {
	RunbookID  string              `json:"runbook_id"`
	Outcome    memory.Outcome      `json:"outcome"`
	Message    string              `json:"message"`
	Steps      []RunbookStepResult `json:"steps"`
	VerifyStep *RunbookStepResult  `json:"verification,omitempty"`
	Resolved   bool                `json:"resolved"`
	ExecutedAt time.Time           `json:"executed_at"`
	FindingID  string              `json:"finding_id"`
	FindingKey string              `json:"finding_key,omitempty"`
}

func (p *PatrolService) GetRunbooksForFinding(findingID string) ([]RunbookInfo, error) {
	finding := p.findings.Get(findingID)
	if finding == nil {
		return nil, fmt.Errorf("finding not found")
	}

	runbooks := matchRunbooksForFinding(finding)
	infos := make([]RunbookInfo, 0, len(runbooks))
	for _, rb := range runbooks {
		infos = append(infos, RunbookInfo{
			ID:          rb.ID,
			Title:       rb.Title,
			Description: rb.Description,
			Risk:        rb.Risk,
		})
	}
	return infos, nil
}

func (p *PatrolService) ExecuteRunbook(ctx context.Context, findingID, runbookID string) (*RunbookExecutionResult, error) {
	return p.executeRunbook(ctx, findingID, runbookID, false)
}

func (p *PatrolService) executeRunbook(ctx context.Context, findingID, runbookID string, automatic bool) (*RunbookExecutionResult, error) {
	finding := p.findings.Get(findingID)
	if finding == nil {
		return nil, fmt.Errorf("finding not found")
	}

	runbook, ok := getRunbookByID(runbookID)
	if !ok {
		return nil, ErrRunbookNotFound
	}
	if !runbookApplies(runbook, finding) {
		return nil, ErrRunbookNotApplicable
	}
	if p.aiService == nil {
		return nil, fmt.Errorf("AI service not available")
	}

	context := buildRunbookContext(finding)
	results := make([]RunbookStepResult, 0, len(runbook.Steps))
	executionTime := time.Now()

	for _, step := range runbook.Steps {
		if step.RunOnHost && context.Node == "" {
			return nil, fmt.Errorf("target host is required for runbook step %q", step.Name)
		}
		command, err := renderRunbookCommand(step.Command, context)
		if err != nil {
			return nil, err
		}

		resp, err := p.aiService.RunCommand(ctx, RunCommandRequest{
			Command:    command,
			TargetType: runbookTargetType(finding, step.RunOnHost),
			TargetID:   finding.ResourceID,
			RunOnHost:  step.RunOnHost,
			VMID:       context.VMID,
			TargetHost: context.Node,
		})
		if err != nil {
			return nil, err
		}

		stepResult := RunbookStepResult{
			Name:    step.Name,
			Command: command,
			Output:  strings.TrimSpace(resp.Output),
			Success: resp.Success,
		}
		results = append(results, stepResult)

		if !resp.Success && !step.AllowFailure {
			outcome := memory.OutcomeFailed
			message := fmt.Sprintf("Runbook step failed: %s", step.Name)
			p.logRunbookExecution(finding, runbook, results, nil, outcome, message, executionTime, automatic)
			return &RunbookExecutionResult{
				RunbookID:  runbook.ID,
				Outcome:    outcome,
				Message:    message,
				Steps:      results,
				Resolved:   false,
				ExecutedAt: executionTime,
				FindingID:  finding.ID,
				FindingKey: finding.Key,
			}, nil
		}
	}

	var verifyResult *RunbookStepResult
	outcome := memory.OutcomeUnknown
	message := "Runbook completed"

	if runbook.Verification != nil {
		if runbook.Verification.RunOnHost && context.Node == "" {
			return nil, fmt.Errorf("target host is required for runbook verification")
		}
		command, err := renderRunbookCommand(runbook.Verification.Command, context)
		if err != nil {
			return nil, err
		}

		resp, err := p.aiService.RunCommand(ctx, RunCommandRequest{
			Command:    command,
			TargetType: runbookTargetType(finding, runbook.Verification.RunOnHost),
			TargetID:   finding.ResourceID,
			RunOnHost:  runbook.Verification.RunOnHost,
			VMID:       context.VMID,
			TargetHost: context.Node,
		})
		if err != nil {
			return nil, err
		}

		verifyResult = &RunbookStepResult{
			Name:    runbook.Verification.Name,
			Command: command,
			Output:  strings.TrimSpace(resp.Output),
			Success: resp.Success,
		}

		outcome, message = evaluateRunbookVerification(runbook.Verification, verifyResult.Output, finding, p.thresholds)
		if !resp.Success && outcome == memory.OutcomeResolved {
			outcome = memory.OutcomePartial
			message = "Verification command failed"
		}
	} else {
		outcome = memory.OutcomeUnknown
		message = "Runbook completed without verification"
	}

	resolved := false
	if outcome == memory.OutcomeResolved {
		note := runbook.ResolutionNote
		if note == "" {
			note = fmt.Sprintf("Applied runbook: %s", runbook.Title)
		}
		if err := p.ResolveFinding(finding.ID, note); err == nil {
			resolved = true
		}
	}

	p.logRunbookExecution(finding, runbook, results, verifyResult, outcome, message, executionTime, automatic)

	return &RunbookExecutionResult{
		RunbookID:  runbook.ID,
		Outcome:    outcome,
		Message:    message,
		Steps:      results,
		VerifyStep: verifyResult,
		Resolved:   resolved,
		ExecutedAt: executionTime,
		FindingID:  finding.ID,
		FindingKey: finding.Key,
	}, nil
}

func (p *PatrolService) AutoFixWithRunbooks(ctx context.Context, findings []*Finding) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if p.aiService == nil || len(findings) == 0 {
		return 0
	}

	resolved := 0
	for _, finding := range findings {
		if finding == nil || finding.Key == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return resolved
		default:
		}

		runbook, ok := selectRunbookForAutoFix(finding)
		if !ok {
			continue
		}
		if !p.shouldAutoFixFinding(finding) {
			continue
		}

		result, err := p.executeRunbook(ctx, finding.ID, runbook.ID, true)
		if err != nil {
			log.Warn().Err(err).Str("runbook_id", runbook.ID).Str("finding_id", finding.ID).Msg("Runbook auto-fix failed")
			continue
		}
		if result != nil && result.Resolved {
			resolved++
		}
	}

	return resolved
}

func (p *PatrolService) logRunbookExecution(finding *Finding, runbook Runbook, steps []RunbookStepResult, verify *RunbookStepResult, outcome memory.Outcome, message string, executedAt time.Time, automatic bool) {
	if p.remediationLog == nil {
		return
	}

	noteParts := []string{}
	for _, step := range steps {
		status := "ok"
		if !step.Success {
			status = "failed"
		}
		noteParts = append(noteParts, fmt.Sprintf("%s (%s)", step.Name, status))
	}
	if verify != nil {
		noteParts = append(noteParts, fmt.Sprintf("verification: %s", verify.Name))
	}
	note := strings.Join(noteParts, "; ")
	if message != "" {
		note = message + ". " + note
	}

	output := ""
	if verify != nil {
		output = verify.Output
	}

	record := memory.RemediationRecord{
		Timestamp:    executedAt,
		ResourceID:   finding.ResourceID,
		ResourceType: finding.ResourceType,
		ResourceName: finding.ResourceName,
		FindingID:    finding.ID,
		Problem:      finding.Title,
		Action:       runbook.Title,
		Output:       truncateRunbookOutput(output, 1000),
		Outcome:      outcome,
		Note:         truncateRunbookOutput(note, 500),
		Automatic:    automatic,
	}

	if err := p.remediationLog.Log(record); err != nil {
		log.Warn().Err(err).Msg("Failed to log runbook execution")
	}
}

type runbookContext struct {
	ResourceID   string
	ResourceName string
	Node         string
	VMID         string
	PBSID        string
	Datastore    string
	JobID        string
}

func buildRunbookContext(finding *Finding) runbookContext {
	vmid := parseVMID(finding.ResourceID)
	pbsID, datastore, jobID := parsePBSResourceParts(finding.ResourceID)

	return runbookContext{
		ResourceID:   finding.ResourceID,
		ResourceName: finding.ResourceName,
		Node:         finding.Node,
		VMID:         vmid,
		PBSID:        pbsID,
		Datastore:    datastore,
		JobID:        jobID,
	}
}

func runbookTargetType(finding *Finding, runOnHost bool) string {
	if runOnHost {
		return "host"
	}
	switch finding.ResourceType {
	case "vm":
		return "vm"
	case "container":
		return "container"
	default:
		return "host"
	}
}

func renderRunbookCommand(command string, ctx runbookContext) (string, error) {
	placeholders := map[string]string{
		"resource_id":   ctx.ResourceID,
		"resource_name": ctx.ResourceName,
		"node":          ctx.Node,
		"vmid":          ctx.VMID,
		"pbs_id":        ctx.PBSID,
		"datastore":     ctx.Datastore,
		"job_id":        ctx.JobID,
	}

	result := command
	for key, value := range placeholders {
		placeholder := "{{" + key + "}}"
		if strings.Contains(result, placeholder) {
			if value == "" {
				return "", fmt.Errorf("missing value for %s", key)
			}
			result = strings.ReplaceAll(result, placeholder, escapeShellArg(value))
		}
	}

	return result, nil
}

func escapeShellArg(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"") {
		return value
	}
	escaped := strings.ReplaceAll(value, `'`, `'\''`)
	return "'" + escaped + "'"
}

func parseVMID(resourceID string) string {
	if resourceID == "" {
		return ""
	}
	parts := strings.Split(resourceID, "-")
	last := parts[len(parts)-1]
	if _, err := strconv.Atoi(last); err == nil {
		return last
	}
	return ""
}

func parsePBSResourceParts(resourceID string) (string, string, string) {
	if resourceID == "" {
		return "", "", ""
	}
	parts := strings.Split(resourceID, ":")
	if len(parts) < 2 {
		return resourceID, "", ""
	}
	pbsID := parts[0]
	if len(parts) >= 3 && (parts[1] == "job" || parts[1] == "verify") {
		return pbsID, "", parts[2]
	}
	return pbsID, parts[1], ""
}

func evaluateRunbookVerification(verification *RunbookVerification, output string, finding *Finding, thresholds PatrolThresholds) (memory.Outcome, string) {
	if verification == nil {
		return memory.OutcomeUnknown, "No verification configured"
	}

	if verification.Verifier == runbookVerifierDiskUsage {
		return verifyDiskUsage(output, finding, thresholds)
	}

	if verification.SuccessRegex != "" {
		matched, _ := regexp.MatchString(verification.SuccessRegex, output)
		if matched {
			return memory.OutcomeResolved, "Verification passed"
		}
	}

	if verification.FailureRegex != "" {
		matched, _ := regexp.MatchString(verification.FailureRegex, output)
		if matched {
			return memory.OutcomeFailed, "Verification failed"
		}
	}

	return memory.OutcomeUnknown, "Verification inconclusive"
}

func verifyDiskUsage(output string, finding *Finding, thresholds PatrolThresholds) (memory.Outcome, string) {
	usage, ok := parseDFUsagePercent(output)
	if !ok {
		return memory.OutcomeUnknown, "Unable to parse disk usage"
	}

	threshold := thresholds.GuestDiskWatch
	if finding.ResourceType == "storage" {
		threshold = thresholds.StorageWatch
	}

	note := fmt.Sprintf("Disk usage now %d%% (threshold %.0f%%)", usage, threshold)

	if float64(usage) < threshold {
		return memory.OutcomeResolved, note
	}

	baseline := parsePercentFromFinding(finding.Evidence)
	if baseline > 0 {
		if float64(usage) < baseline {
			return memory.OutcomePartial, note + fmt.Sprintf(", was %.0f%%", baseline)
		}
		return memory.OutcomeFailed, note + fmt.Sprintf(", was %.0f%%", baseline)
	}

	return memory.OutcomeUnknown, note
}

func parseDFUsagePercent(output string) (int, bool) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		mount := fields[len(fields)-1]
		if mount != "/" {
			continue
		}
		usageField := fields[4]
		usageField = strings.TrimSuffix(usageField, "%")
		usage, err := strconv.Atoi(usageField)
		if err != nil {
			return 0, false
		}
		return usage, true
	}
	return 0, false
}

func parsePercentFromFinding(value string) float64 {
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
	match := re.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	parsed, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	return parsed
}

func truncateRunbookOutput(output string, limit int) string {
	output = strings.TrimSpace(output)
	if len(output) <= limit {
		return output
	}
	return output[:limit] + "..."
}

func runbookApplies(runbook Runbook, finding *Finding) bool {
	if len(runbook.FindingKeys) > 0 {
		matched := false
		for _, key := range runbook.FindingKeys {
			if key == finding.Key {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(runbook.ResourceTypes) > 0 {
		matched := false
		for _, rt := range runbook.ResourceTypes {
			if rt == finding.ResourceType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func selectRunbookForAutoFix(finding *Finding) (Runbook, bool) {
	for _, runbook := range runbooksCatalog {
		if runbook.Risk != RunbookRiskLow {
			continue
		}
		if runbookApplies(runbook, finding) {
			return runbook, true
		}
	}
	return Runbook{}, false
}

func (p *PatrolService) shouldAutoFixFinding(finding *Finding) bool {
	if p.remediationLog == nil {
		return false
	}

	records := p.remediationLog.GetForFinding(finding.ID, 1)
	if len(records) == 0 {
		return true
	}

	last := records[0]
	if time.Since(last.Timestamp) < 6*time.Hour {
		return false
	}

	return true
}

func matchRunbooksForFinding(finding *Finding) []Runbook {
	var matches []Runbook
	for _, runbook := range runbooksCatalog {
		if runbookApplies(runbook, finding) {
			matches = append(matches, runbook)
		}
	}
	return matches
}

func getRunbookByID(id string) (Runbook, bool) {
	for _, runbook := range runbooksCatalog {
		if runbook.ID == id {
			return runbook, true
		}
	}
	return Runbook{}, false
}

var runbooksCatalog = []Runbook{
	{
		ID:          "docker-restart-loop",
		Title:       "Restart Docker container and verify",
		Description: "Collect recent logs, restart the container, then verify it is running.",
		Risk:        RunbookRiskLow,
		FindingKeys: []string{"restart-loop"},
		ResourceTypes: []string{
			"docker_container",
		},
		Steps: []RunbookStep{
			{
				Name:         "Fetch recent logs",
				Command:      "docker logs --tail 200 {{resource_name}}",
				RunOnHost:    true,
				AllowFailure: true,
			},
			{
				Name:      "Restart container",
				Command:   "docker restart {{resource_name}}",
				RunOnHost: true,
			},
		},
		Verification: &RunbookVerification{
			Name:         "Verify container running",
			Command:      "docker inspect -f '{{.State.Status}}' {{resource_name}}",
			RunOnHost:    true,
			SuccessRegex: "(?i)running",
		},
		ResolutionNote: "Restarted docker container and verified it is running.",
	},
	{
		ID:          "guest-disk-cleanup",
		Title:       "Clean package cache and old logs",
		Description: "Vacuum systemd journal and clean package cache to free disk space.",
		Risk:        RunbookRiskMedium,
		FindingKeys: []string{"high-disk"},
		ResourceTypes: []string{
			"vm",
			"container",
		},
		Steps: []RunbookStep{
			{
				Name:         "Vacuum systemd journal",
				Command:      "journalctl --vacuum-time=7d",
				RunOnHost:    false,
				AllowFailure: true,
			},
			{
				Name:         "Clean package cache",
				Command:      "apt-get clean",
				RunOnHost:    false,
				AllowFailure: true,
			},
		},
		Verification: &RunbookVerification{
			Name:      "Check root filesystem usage",
			Command:   "df -P /",
			RunOnHost: false,
			Verifier:  runbookVerifierDiskUsage,
		},
		ResolutionNote: "Cleaned logs and package cache, then verified disk usage.",
	},
	{
		ID:          "docker-high-memory-restart",
		Title:       "Restart Docker container to clear memory",
		Description: "Capture current memory usage, restart the container, then verify it is running.",
		Risk:        RunbookRiskMedium,
		FindingKeys: []string{"high-memory"},
		ResourceTypes: []string{
			"docker_container",
		},
		Steps: []RunbookStep{
			{
				Name:         "Capture memory usage",
				Command:      "docker stats --no-stream {{resource_name}}",
				RunOnHost:    true,
				AllowFailure: true,
			},
			{
				Name:      "Restart container",
				Command:   "docker restart {{resource_name}}",
				RunOnHost: true,
			},
		},
		Verification: &RunbookVerification{
			Name:         "Verify container running",
			Command:      "docker inspect -f '{{.State.Status}}' {{resource_name}}",
			RunOnHost:    true,
			SuccessRegex: "(?i)running",
		},
		ResolutionNote: "Restarted docker container to clear memory usage.",
	},
}
