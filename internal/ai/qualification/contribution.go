package qualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ContributionSchemaVersion = "patrol.qualification.contribution/v1"

var contributionChallengePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{16,128}$`)
var sha256HexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ContributionBundle is an allowlist-only, local export for voluntary
// community benchmark evidence. It deliberately contains no raw report,
// finding, resource, prompt, tool, action, log, error, or infrastructure
// endpoint content. Creating a bundle never uploads it.
type ContributionBundle struct {
	SchemaVersion string                `json:"schema_version"`
	GeneratedAt   time.Time             `json:"generated_at"`
	EvidenceClass string                `json:"evidence_class"`
	Notice        string                `json:"notice"`
	NetworkUpload bool                  `json:"network_upload_performed"`
	Privacy       ContributionPrivacy   `json:"privacy"`
	Challenge     ContributionChallenge `json:"challenge"`
	Runs          []ContributionRun     `json:"runs"`
	Comparison    ComparisonReport      `json:"comparison"`
}

type ContributionPrivacy struct {
	AllowlistOnly      bool     `json:"allowlist_only"`
	ManualReviewNeeded bool     `json:"manual_review_required_before_sharing"`
	Excluded           []string `json:"excluded"`
}

type ContributionChallenge struct {
	Nonce       string `json:"nonce,omitempty"`
	BoundAtRun  bool   `json:"bound_at_run"`
	Consistent  bool   `json:"consistent_across_runs"`
	Explanation string `json:"explanation"`
}

type ContributionRun struct {
	EvidenceDigest  string                   `json:"evidence_digest_sha256"`
	ReportDigest    string                   `json:"source_report_sha256"`
	ReplayDigest    string                   `json:"source_replay_sha256,omitempty"`
	ScenarioID      string                   `json:"scenario_id"`
	ScenarioVersion int                      `json:"scenario_version"`
	ManifestDigest  string                   `json:"manifest_digest"`
	Track           Track                    `json:"track"`
	Model           string                   `json:"model"`
	Provider        string                   `json:"provider"`
	HarnessGitSHA   string                   `json:"harness_git_sha"`
	HarnessGitDirty bool                     `json:"harness_git_dirty"`
	PulseVersion    string                   `json:"pulse_version"`
	CapturedAt      time.Time                `json:"captured_at"`
	ChallengeBound  bool                     `json:"challenge_bound"`
	Passed          bool                     `json:"passed"`
	Execution       ContributionExecution    `json:"execution"`
	Score           ContributionScore        `json:"score"`
	Safety          ContributionSafety       `json:"safety"`
	FailureCounts   ContributionFailureCount `json:"failure_counts"`
}

type ContributionExecution struct {
	PreflightPassed   bool `json:"preflight_passed"`
	CollectionPassed  bool `json:"collection_passed"`
	ModelPatrolPassed bool `json:"model_patrol_passed"`
}

type ContributionScore struct {
	Faults                  int          `json:"faults"`
	TruePositives           int          `json:"true_positives"`
	MissedFaults            int          `json:"missed_faults"`
	FalsePositives          int          `json:"false_positives"`
	Recall                  float64      `json:"recall"`
	ResourceAccuracy        float64      `json:"resource_accuracy"`
	ResourceTypeAccuracy    float64      `json:"resource_type_accuracy"`
	CategoryAccuracy        float64      `json:"category_accuracy"`
	SeverityAccuracy        float64      `json:"severity_accuracy"`
	EvidenceGrounding       float64      `json:"evidence_grounding"`
	RecommendationSafety    float64      `json:"recommendation_safety"`
	InvestigationCompletion float64      `json:"investigation_completion"`
	InvestigationGrounding  float64      `json:"investigation_grounding"`
	RootCauseGrounding      float64      `json:"root_cause_grounding"`
	AffectedGrounding       float64      `json:"affected_resource_grounding"`
	FindingsPerCausalGroup  float64      `json:"findings_per_causal_group"`
	ToolCalls               int          `json:"tool_calls"`
	FailedToolCalls         int          `json:"failed_tool_calls"`
	DuplicateToolCalls      int          `json:"duplicate_tool_calls"`
	InputTokens             int          `json:"input_tokens"`
	OutputTokens            int          `json:"output_tokens"`
	Cost                    CostEstimate `json:"cost"`
	CollectionLatencyMS     int64        `json:"collection_latency_ms"`
	PatrolLatencyMS         int64        `json:"patrol_latency_ms"`
	EndToEndLatencyMS       int64        `json:"end_to_end_latency_ms"`
}

type ContributionSafety struct {
	PostPatrolOracleCount  int  `json:"post_patrol_oracle_count"`
	PostPatrolOraclePassed bool `json:"post_patrol_oracle_passed"`
	RevertOracleCount      int  `json:"revert_oracle_count"`
	RevertOraclePassed     bool `json:"revert_oracle_passed"`
	TeardownPassed         bool `json:"teardown_passed"`
	SecondCleanupNoop      bool `json:"second_cleanup_noop"`
	InventoryUnchanged     bool `json:"inventory_unchanged"`
}

type ContributionFailureCount struct {
	HardFailures int `json:"hard_failures"`
	GateFailures int `json:"gate_failures"`
	RunErrors    int `json:"run_errors"`
}

func ValidateContributionChallenge(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !contributionChallengePattern.MatchString(value) {
		return fmt.Errorf("community challenge must be 16-128 ASCII letters, digits, dots, underscores, or hyphens")
	}
	return nil
}

func BuildContributionBundle(paths []string, catalog Catalog, track Track) (ContributionBundle, error) {
	if track != TrackWatch && track != TrackInvestigation && track != TrackRemediation {
		return ContributionBundle{}, fmt.Errorf("community export requires a qualification track")
	}
	comparison, err := CompareReports(paths)
	if err != nil {
		return ContributionBundle{}, err
	}
	if err := ApplyQualificationGates(&comparison, catalog, track); err != nil {
		return ContributionBundle{}, err
	}

	bundle := ContributionBundle{
		SchemaVersion: ContributionSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		EvidenceClass: "community-candidate",
		Notice:        "Local allowlist-only evidence export. It is not Pulse certification and has not been uploaded.",
		NetworkUpload: false,
		Privacy: ContributionPrivacy{
			AllowlistOnly:      true,
			ManualReviewNeeded: true,
			Excluded: []string{
				"action identifiers, plans, parameters, and results",
				"Docker targets, inventory, container identifiers, and network names",
				"error and failure prose",
				"findings, descriptions, evidence text, and resource identifiers",
				"hostnames, IP addresses, Pulse URLs, and collected topology",
				"logs, prompts, model output, and transcripts",
				"tool names, arguments, outputs, and provider call identifiers",
			},
		},
		Comparison: comparison,
	}

	challenges := make(map[string]struct{})
	for _, path := range paths {
		report, err := LoadReport(path)
		if err != nil {
			return ContributionBundle{}, fmt.Errorf("load %s: %w", path, err)
		}
		if err := ValidateContributionChallenge(report.Environment.ChallengeNonce); err != nil {
			return ContributionBundle{}, fmt.Errorf("load %s: %w", path, err)
		}
		if err := report.Manifest.Validate(); err != nil {
			return ContributionBundle{}, fmt.Errorf("load %s: invalid embedded manifest: %w", path, err)
		}
		if report.Manifest.Track != track {
			return ContributionBundle{}, fmt.Errorf("load %s: report track %q does not match requested community export track %q", path, report.Manifest.Track, track)
		}
		for label, value := range map[string]string{
			"harness revision": report.Environment.GitSHA,
			"model":            report.Environment.Model,
			"provider":         report.Environment.Provider,
			"Pulse version":    report.Environment.PulseVersion,
			"scenario id":      report.Manifest.ID,
			"cost model":       report.Score.Cost.Model,
			"cost provider":    report.Score.Cost.Provider,
		} {
			if err := validateContributionIdentity(label, value); err != nil {
				return ContributionBundle{}, fmt.Errorf("load %s: %w", path, err)
			}
		}
		run, err := contributionRun(path, report)
		if err != nil {
			return ContributionBundle{}, err
		}
		bundle.Runs = append(bundle.Runs, run)
		if challenge := strings.TrimSpace(report.Environment.ChallengeNonce); challenge != "" {
			challenges[challenge] = struct{}{}
		}
	}
	sort.Slice(bundle.Runs, func(i, j int) bool {
		left, right := bundle.Runs[i], bundle.Runs[j]
		if left.Model != right.Model {
			return left.Model < right.Model
		}
		if left.ScenarioID != right.ScenarioID {
			return left.ScenarioID < right.ScenarioID
		}
		if !left.CapturedAt.Equal(right.CapturedAt) {
			return left.CapturedAt.Before(right.CapturedAt)
		}
		return left.EvidenceDigest < right.EvidenceDigest
	})
	if len(challenges) > 1 {
		return ContributionBundle{}, fmt.Errorf("reports contain multiple community challenges; export one challenge campaign at a time")
	}
	if len(challenges) == 1 && !everyRunChallengeBound(bundle.Runs) {
		return ContributionBundle{}, fmt.Errorf("reports mix challenge-bound and unbound runs; export one campaign at a time")
	}

	bundle.Challenge = ContributionChallenge{
		BoundAtRun:  len(challenges) == 1,
		Consistent:  true,
		Explanation: "A server-issued challenge is useful only when supplied before every live run; it does not make a self-reported bundle Pulse-certified.",
	}
	if len(challenges) == 1 {
		for challenge := range challenges {
			bundle.Challenge.Nonce = challenge
		}
	}
	return bundle, nil
}

func contributionRun(path string, report RunReport) (ContributionRun, error) {
	manifestDigest, err := report.Manifest.Digest()
	if err != nil {
		return ContributionRun{}, fmt.Errorf("digest scenario %s: %w", report.Manifest.ID, err)
	}
	reportDigest, err := digestFile(path)
	if err != nil {
		return ContributionRun{}, err
	}
	replayDigest := ""
	replayPath := filepath.Join(filepath.Dir(path), "replay.json")
	if _, err := os.Stat(replayPath); err == nil {
		replayDigest, err = digestFile(replayPath)
		if err != nil {
			return ContributionRun{}, err
		}
	} else if !os.IsNotExist(err) {
		return ContributionRun{}, err
	}

	run := ContributionRun{
		ReportDigest:    reportDigest,
		ReplayDigest:    replayDigest,
		ScenarioID:      report.Manifest.ID,
		ScenarioVersion: report.Manifest.Version,
		ManifestDigest:  manifestDigest,
		Track:           report.Manifest.Track,
		Model:           report.Environment.Model,
		Provider:        report.Environment.Provider,
		HarnessGitSHA:   report.Environment.GitSHA,
		HarnessGitDirty: report.Environment.GitDirty,
		PulseVersion:    report.Environment.PulseVersion,
		CapturedAt:      report.Environment.CapturedAt,
		ChallengeBound:  strings.TrimSpace(report.Environment.ChallengeNonce) != "",
		Passed:          report.Passed,
		Execution: ContributionExecution{
			PreflightPassed:   reportPhasePassed(report.Phases, "preflight"),
			CollectionPassed:  reportPhasePassed(report.Phases, "normal_collection_convergence"),
			ModelPatrolPassed: reportPhasePassed(report.Phases, "real_model_patrol"),
		},
		Score: ContributionScore{
			Faults: report.Score.Faults, TruePositives: report.Score.TruePositives,
			MissedFaults: report.Score.MissedFaults, FalsePositives: report.Score.FalsePositives,
			Recall: report.Score.Recall, ResourceAccuracy: report.Score.ResourceAccuracy,
			ResourceTypeAccuracy: report.Score.ResourceTypeAccuracy, CategoryAccuracy: report.Score.CategoryAccuracy,
			SeverityAccuracy: report.Score.SeverityAccuracy, EvidenceGrounding: report.Score.EvidenceGrounding,
			RecommendationSafety:    report.Score.RecommendationSafety,
			InvestigationCompletion: report.Score.InvestigationCompletion, InvestigationGrounding: report.Score.InvestigationGrounding,
			RootCauseGrounding: report.Score.RootCauseGrounding, AffectedGrounding: report.Score.AffectedGrounding,
			FindingsPerCausalGroup: report.Score.FindingsPerCausalGroup,
			ToolCalls:              report.Score.ToolCalls, FailedToolCalls: report.Score.FailedToolCalls,
			DuplicateToolCalls: report.Score.DuplicateToolCalls, InputTokens: report.Score.InputTokens,
			OutputTokens: report.Score.OutputTokens, Cost: report.Score.Cost,
			CollectionLatencyMS: report.Score.CollectionLatency.Milliseconds(), PatrolLatencyMS: report.Score.PatrolLatency.Milliseconds(),
			EndToEndLatencyMS: report.Score.EndToEndLatency.Milliseconds(),
		},
		Safety: ContributionSafety{
			PostPatrolOracleCount: len(report.PostPatrol), PostPatrolOraclePassed: observationsPassedOrEmpty(report.PostPatrol),
			RevertOracleCount: len(report.Revert), RevertOraclePassed: observationsPassedOrEmpty(report.Revert),
			TeardownPassed: report.Teardown.Passed, SecondCleanupNoop: report.Teardown.SecondCleanupNoop,
			InventoryUnchanged: report.Teardown.InventoryUnchanged,
		},
		FailureCounts: ContributionFailureCount{
			HardFailures: len(report.Score.HardFailures), GateFailures: len(report.Score.GateFailures), RunErrors: len(report.Errors),
		},
	}
	digestPayload := run
	digestPayload.EvidenceDigest = ""
	payload, err := json.Marshal(digestPayload)
	if err != nil {
		return ContributionRun{}, err
	}
	digest := sha256.Sum256(payload)
	run.EvidenceDigest = hex.EncodeToString(digest[:])
	return run, nil
}

func WriteContributionBundle(dir string, bundle ContributionBundle) error {
	if err := bundle.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := writeContributionJSON(filepath.Join(dir, "contribution.json"), bundle); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(renderContributionReadme(bundle)), 0o600); err != nil {
		return err
	}
	return writeChecksums(dir, []string{"README.md", "contribution.json"})
}

func writeContributionJSON(path string, bundle ContributionBundle) error {
	payload, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o600)
}

func (bundle ContributionBundle) Validate() error {
	if bundle.SchemaVersion != ContributionSchemaVersion {
		return fmt.Errorf("unsupported contribution schema %q", bundle.SchemaVersion)
	}
	if bundle.EvidenceClass != "community-candidate" {
		return fmt.Errorf("unsupported contribution evidence class %q", bundle.EvidenceClass)
	}
	if bundle.NetworkUpload {
		return fmt.Errorf("local contribution bundle cannot claim a network upload")
	}
	if !bundle.Privacy.AllowlistOnly || !bundle.Privacy.ManualReviewNeeded || len(bundle.Privacy.Excluded) == 0 {
		return fmt.Errorf("contribution privacy boundary is incomplete")
	}
	if len(bundle.Runs) == 0 {
		return fmt.Errorf("contribution bundle contains no runs")
	}
	if err := ValidateContributionChallenge(bundle.Challenge.Nonce); err != nil {
		return err
	}
	if bundle.Challenge.BoundAtRun && (bundle.Challenge.Nonce == "" || !bundle.Challenge.Consistent) {
		return fmt.Errorf("bound community challenge must be present and consistent across runs")
	}
	for index, run := range bundle.Runs {
		if run.ScenarioID == "" || run.Model == "" || run.ManifestDigest == "" || run.HarnessGitSHA == "" {
			return fmt.Errorf("contribution run %d has incomplete model, scenario, manifest, or harness provenance", index)
		}
		for label, digest := range map[string]string{
			"evidence": run.EvidenceDigest,
			"manifest": run.ManifestDigest,
			"report":   run.ReportDigest,
		} {
			if !sha256HexPattern.MatchString(digest) {
				return fmt.Errorf("contribution run %d has invalid %s digest", index, label)
			}
		}
		if run.ReplayDigest != "" && !sha256HexPattern.MatchString(run.ReplayDigest) {
			return fmt.Errorf("contribution run %d has invalid replay digest", index)
		}
		if bundle.Challenge.BoundAtRun != run.ChallengeBound {
			return fmt.Errorf("contribution run %d challenge binding disagrees with bundle", index)
		}
	}
	return nil
}

func renderContributionReadme(bundle ContributionBundle) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Pulse Patrol community evidence candidate")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This directory was created locally. No network upload was performed. Review every file before sharing it.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Community evidence helps shortlist models; it is not Pulse certification and cannot independently authorize a hosted-model recommendation.")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Runs: %d\n", len(bundle.Runs))
	fmt.Fprintf(&b, "- Challenge bound at run time: %t\n", bundle.Challenge.BoundAtRun)
	fmt.Fprintf(&b, "- Challenge consistent across runs: %t\n", bundle.Challenge.Consistent)
	fmt.Fprintln(&b, "- Export policy: allowlisted aggregate and provenance fields only")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Excluded by construction:")
	fmt.Fprintln(&b)
	for _, excluded := range bundle.Privacy.Excluded {
		fmt.Fprintf(&b, "- %s\n", excluded)
	}
	return b.String()
}

func everyRunChallengeBound(runs []ContributionRun) bool {
	for _, run := range runs {
		if !run.ChallengeBound {
			return false
		}
	}
	return len(runs) > 0
}

func observationsPassedOrEmpty(observations []PredicateObservation) bool {
	if len(observations) == 0 {
		return true
	}
	for _, observation := range observations {
		if !observation.Passed {
			return false
		}
	}
	return true
}

func reportPhasePassed(phases []PhaseTiming, name string) bool {
	for _, phase := range phases {
		if phase.Name == name {
			return phase.Passed
		}
	}
	return false
}

func digestFile(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func validateContributionIdentity(label, value string) error {
	if len(value) > 512 {
		return fmt.Errorf("%s exceeds the community export length limit", label)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s contains a line break", label)
	}
	if strings.Contains(value, "[REDACTED]") {
		return fmt.Errorf("%s was redacted and cannot provide usable community provenance", label)
	}
	if sanitizeArtifactText(value) != value {
		return fmt.Errorf("%s resembles secret-bearing content and cannot be exported", label)
	}
	return nil
}
