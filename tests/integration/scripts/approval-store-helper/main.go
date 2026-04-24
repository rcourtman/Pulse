package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const defaultTargetType = "agent"

var osExit = os.Exit

type cliArgs struct {
	action      string
	approvalID  string
	command     string
	context     string
	dataDir     string
	executionID string
	orgID       string
	risk        string
	targetID    string
	targetName  string
	targetType  string
	timeout     time.Duration
	toolID      string
}

type helperResult struct {
	Action     string                    `json:"action"`
	DataDir    string                    `json:"dataDir"`
	Found      bool                      `json:"found"`
	OrgID      string                    `json:"orgId"`
	ApprovalID string                    `json:"approvalId"`
	Approval   *approval.ApprovalRequest `json:"approval,omitempty"`
}

func usage(message string) {
	if message != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n\n", message)
	}
	fmt.Fprintln(os.Stderr, "usage: approval-store-helper <create|get> --data-dir <dir> --org-id <id> --approval-id <id> [--command <command>] [--target-type <type>] [--target-id <id>] [--target-name <name>] [--context <text>] [--risk <low|medium|high>] [--tool-id <id>] [--execution-id <id>] [--timeout <duration>]")
	osExit(2)
}

func parseArgs(argv []string) cliArgs {
	if len(argv) == 0 {
		usage("missing action")
	}

	args := cliArgs{
		action:     strings.TrimSpace(argv[0]),
		targetType: defaultTargetType,
		timeout:    15 * time.Minute,
		toolID:     "pulse_control",
	}
	flags := flag.NewFlagSet("approval-store-helper", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&args.approvalID, "approval-id", "", "approval id")
	flags.StringVar(&args.command, "command", "", "command awaiting approval")
	flags.StringVar(&args.context, "context", "", "approval context")
	flags.StringVar(&args.dataDir, "data-dir", "", "approval store data directory")
	flags.StringVar(&args.executionID, "execution-id", "", "execution id")
	flags.StringVar(&args.orgID, "org-id", "", "organization id")
	flags.StringVar(&args.risk, "risk", "", "risk level")
	flags.StringVar(&args.targetID, "target-id", "", "approval target id")
	flags.StringVar(&args.targetName, "target-name", "", "approval target name")
	flags.StringVar(&args.targetType, "target-type", defaultTargetType, "approval target type")
	flags.DurationVar(&args.timeout, "timeout", args.timeout, "approval timeout")
	flags.StringVar(&args.toolID, "tool-id", args.toolID, "tool id")

	if err := flags.Parse(argv[1:]); err != nil {
		usage(err.Error())
	}
	if flags.NArg() != 0 {
		usage(fmt.Sprintf("unexpected argument %q", flags.Arg(0)))
	}
	if args.action != "create" && args.action != "get" {
		usage(fmt.Sprintf("unsupported action %q", args.action))
	}
	if strings.TrimSpace(args.dataDir) == "" {
		usage("--data-dir is required")
	}
	if strings.TrimSpace(args.approvalID) == "" {
		usage("--approval-id is required")
	}
	if args.timeout <= 0 {
		usage("--timeout must be positive")
	}
	if args.action == "create" {
		if strings.TrimSpace(args.command) == "" {
			usage("--command is required for create")
		}
		if strings.TrimSpace(args.targetID) == "" {
			usage("--target-id is required for create")
		}
	}

	args.orgID = approval.NormalizeOrgID(args.orgID)
	args.targetType = strings.ToLower(strings.TrimSpace(args.targetType))
	if args.targetType == "" {
		args.targetType = defaultTargetType
	}
	if args.executionID == "" {
		args.executionID = "mobile-release-proof-" + strings.TrimSpace(args.approvalID)
	}
	if args.context == "" {
		args.context = args.command
	}
	if args.risk == "" {
		args.risk = string(approval.AssessRiskLevel(args.command, args.targetType))
	}

	return args
}

func main() {
	args := parseArgs(os.Args[1:])
	if err := run(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args cliArgs) error {
	dataDir, err := filepath.Abs(args.dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:        dataDir,
		DefaultTimeout: args.timeout,
		MaxApprovals:   100,
	})
	if err != nil {
		return fmt.Errorf("open approval store: %w", err)
	}

	var req *approval.ApprovalRequest
	found := false
	switch args.action {
	case "create":
		req = buildApprovalRequest(args)
		if err := store.CreateApproval(req); err != nil {
			return fmt.Errorf("create approval: %w", err)
		}
		store.Flush()
		found = true
	case "get":
		req, found = store.GetApproval(args.approvalID)
	}

	result := helperResult{
		Action:     args.action,
		DataDir:    dataDir,
		Found:      found,
		OrgID:      args.orgID,
		ApprovalID: args.approvalID,
		Approval:   req,
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	fmt.Printf("%s\n", encoded)
	return nil
}

func buildApprovalRequest(args cliArgs) *approval.ApprovalRequest {
	now := time.Now().UTC()
	expiresAt := now.Add(args.timeout)
	actionID := uuid.NewString()
	capabilityName := approvalCapabilityForTargetType(args.targetType)
	resourceID := approvalAuditResourceID(args.targetType, args.targetID, args.targetName)
	plan := &unifiedresources.ActionPlan{
		ActionID:             actionID,
		RequestID:            args.approvalID,
		Allowed:              true,
		RequiresApproval:     true,
		ApprovalPolicy:       unifiedresources.ApprovalAdmin,
		PredictedBlastRadius: approvalBlastRadius(args.targetType, args.command),
		RollbackAvailable:    approvalRollbackAvailable(args.targetType, args.command),
		Message:              strings.TrimSpace(args.context),
		PlannedAt:            now,
		ExpiresAt:            expiresAt,
		PolicyVersion:        "v6-mobile-release-proof",
		PlanHash:             approvalPlanHash(actionID, args.approvalID, capabilityName, resourceID, args.command, args.targetType, args.targetID, args.context),
	}

	req := &approval.ApprovalRequest{
		ID:          args.approvalID,
		OrgID:       args.orgID,
		ExecutionID: args.executionID,
		ToolID:      args.toolID,
		Command:     args.command,
		TargetType:  args.targetType,
		TargetID:    args.targetID,
		TargetName:  args.targetName,
		Context:     args.context,
		RiskLevel:   approval.RiskLevel(args.risk),
		RequestedAt: now,
		ExpiresAt:   expiresAt,
		Plan:        plan,
	}
	req.ContextConfidence = approvalContextConfidence(req)
	req.Preflight = approvalPreflight(req)
	return req
}

func approvalCapabilityForTargetType(targetType string) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "docker", "app-container", "docker-service":
		return "pulse_docker"
	case "file":
		return "pulse_file_edit"
	case "kubernetes", "pod", "k8s-cluster", "k8s-node", "k8s-deployment":
		return "pulse_kubernetes"
	default:
		return "pulse_control"
	}
}

func approvalAuditResourceID(targetType, targetID, targetName string) string {
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		targetID = strings.TrimSpace(targetName)
	}
	if targetID == "" {
		return targetType
	}
	if targetType == "" || strings.Contains(targetID, ":") {
		return targetID
	}
	return targetType + ":" + targetID
}

func approvalBlastRadius(targetType, command string) []string {
	commandLower := strings.ToLower(strings.TrimSpace(command))
	switch {
	case strings.Contains(commandLower, "delete") || strings.HasPrefix(commandLower, "rm ") || strings.Contains(commandLower, " rm "):
		return []string{"destructive target change"}
	case strings.Contains(commandLower, "restart") || strings.Contains(commandLower, "reboot") || strings.Contains(commandLower, "shutdown") || strings.Contains(commandLower, "stop"):
		return []string{"service interruption on target"}
	case strings.ToLower(strings.TrimSpace(targetType)) == "file":
		return []string{"file contents on target"}
	case strings.Contains(strings.ToLower(strings.TrimSpace(targetType)), "k8s") || strings.ToLower(strings.TrimSpace(targetType)) == "kubernetes":
		return []string{"kubernetes workload state"}
	default:
		return []string{"target resource state"}
	}
}

func approvalRollbackAvailable(targetType, command string) bool {
	commandLower := strings.ToLower(strings.TrimSpace(command))
	if strings.ToLower(strings.TrimSpace(targetType)) == "file" || strings.Contains(commandLower, "delete") || strings.HasPrefix(commandLower, "rm ") || strings.Contains(commandLower, " rm ") {
		return false
	}
	return strings.Contains(commandLower, "restart") || strings.Contains(commandLower, "start") || strings.Contains(commandLower, "stop")
}

func approvalContextConfidence(req *approval.ApprovalRequest) *approval.ContextConfidence {
	targetType := strings.TrimSpace(req.TargetType)
	targetID := strings.TrimSpace(req.TargetID)
	targetName := strings.TrimSpace(req.TargetName)
	evidence := make([]string, 0, 3)
	if targetType != "" {
		evidence = append(evidence, fmt.Sprintf("Target type resolved as %s.", targetType))
	}
	if targetID != "" {
		evidence = append(evidence, fmt.Sprintf("Target identifier bound to %s.", targetID))
	}
	if targetName != "" {
		evidence = append(evidence, fmt.Sprintf("Display target resolved as %s.", targetName))
	}

	level := approval.ContextConfidenceUnknown
	summary := "Pulse Assistant could not bind this action to a resolved target."
	switch {
	case targetType != "" && targetID != "":
		level = approval.ContextConfidenceVerified
		summary = "Target was resolved to a concrete resource before approval."
	case targetType != "" && targetName != "":
		level = approval.ContextConfidencePartial
		summary = "Target type and display name were resolved before approval."
	case targetType != "" || targetName != "":
		level = approval.ContextConfidenceInferred
		summary = "Target context was inferred from the requested action."
	}

	return &approval.ContextConfidence{
		Level:    level,
		Summary:  summary,
		Evidence: evidence,
	}
}

func approvalPreflight(req *approval.ApprovalRequest) *approval.ActionPreflight {
	target := approvalAuditResourceID(req.TargetType, req.TargetID, req.TargetName)
	intendedChange := strings.TrimSpace(req.Context)
	if intendedChange == "" {
		intendedChange = strings.TrimSpace(req.Command)
	}

	return &approval.ActionPreflight{
		Target:          target,
		CurrentState:    fmt.Sprintf("Resolved approval target: %s.", target),
		IntendedChange:  intendedChange,
		DryRunAvailable: false,
		DryRunSummary:   "No provider-supported dry run is available for this action; Pulse will hold execution until approval and validate the approval binding before dispatch.",
		SafetyChecks: []string{
			"Approval is scoped to the current organization.",
			"Command hash must match before execution.",
			"Approval can be consumed only once.",
			"Target type and identifier must match the planned action.",
		},
		VerificationSteps: []string{
			"Persist unified action audit lifecycle.",
			"Dispatch only after approval is granted.",
			"Capture command result or execution error.",
			"Require Assistant read-after-write verification before final response.",
		},
		GeneratedAt: time.Now().UTC(),
	}
}

func approvalPlanHash(actionID, requestID, capabilityName, resourceID, command, targetType, targetID, context string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		actionID,
		requestID,
		capabilityName,
		strings.TrimSpace(resourceID),
		command,
		strings.TrimSpace(targetType),
		strings.TrimSpace(targetID),
		strings.TrimSpace(context),
	}, "|")))
	return hex.EncodeToString(sum[:])
}
