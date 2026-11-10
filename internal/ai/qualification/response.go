package qualification

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Grade is the outcome of one response scenario run. It answers the question
// a user would ask: was the problem dealt with. Every grade is derived from
// structured state and the scenario's own terms.
type Grade string

const (
	// GradeFixed: the scenario's governed action ran on the right resource,
	// Pulse verified it, and the lab oracle independently saw the service
	// recover.
	GradeFixed Grade = "fixed"
	// GradeHandedOver: nothing was fixed, and the stored conclusion holds
	// every decisive fact with no statement the lab knows to be false.
	GradeHandedOver Grade = "handed_over"
	// GradePartial: the conclusion found something real that the alert did
	// not already say, and stopped short of the decisive fact or of a fix
	// that was within reach.
	GradePartial Grade = "partial"
	// GradeHandedBack: Patrol looked and the user is no further on.
	GradeHandedBack Grade = "handed_back"
	// GradeQuiet: nothing was wrong and Patrol raised nothing.
	GradeQuiet Grade = "quiet"
	// GradeNotLookedAt: a real fault produced no finding, even after the
	// harness asked for a run.
	GradeNotLookedAt Grade = "not_looked_at"
	// GradeFalseAlarm: nothing was wrong and Patrol raised a finding. Fault
	// set v1 never saw one, so the Phase 1 scorecard had no word for it.
	GradeFalseAlarm Grade = "false_alarm"
)

// Label is the scorecard wording.
func (g Grade) Label() string {
	switch g {
	case GradeFixed:
		return "Fixed"
	case GradeHandedOver:
		return "Handed over"
	case GradePartial:
		return "Partial"
	case GradeHandedBack:
		return "Handed back"
	case GradeQuiet:
		return "Quiet"
	case GradeNotLookedAt:
		return "Not looked at"
	case GradeFalseAlarm:
		return "False alarm"
	default:
		return "No grade"
	}
}

// Resolved reports whether the run counts toward "resolved or correctly
// handed over".
func (g Grade) Resolved() bool {
	return g == GradeFixed || g == GradeHandedOver || g == GradeQuiet
}

const (
	AgentProfileCommandsOn  = "commands-on"
	AgentProfileCommandsOff = "commands-off"
)

// AgentProfile is the first of the two dimensions a scorecard is read by: can
// Patrol read logs and act on this host, or is it the default install with
// commands off. The name comes from what Pulse itself reports for the host.
// Declared is what the operator said the run was, kept so a report shows the
// two agreed.
type AgentProfile struct {
	Name            string `json:"name"`
	CommandsEnabled bool   `json:"commands_enabled"`
	Declared        string `json:"declared,omitempty"`
	HostName        string `json:"host_name,omitempty"`
	HostResourceID  string `json:"host_resource_id,omitempty"`
}

func agentProfileName(commandsEnabled bool) string {
	if commandsEnabled {
		return AgentProfileCommandsOn
	}
	return AgentProfileCommandsOff
}

// ValidAgentProfile reports whether name is one an operator may declare.
func ValidAgentProfile(name string) bool {
	return name == AgentProfileCommandsOn || name == AgentProfileCommandsOff
}

const (
	WakeSelf          = "self"
	WakeAsked         = "asked"
	WakeNotApplicable = "not_applicable"
)

// WakeObservation is the second dimension: how the run started. Mode is self
// when a finding appeared before the harness asked for anything and the Patrol
// run that first reported it was not a manual one. TriggerReason is copied
// from that run record, so "woke by itself" is Pulse's own account.
type WakeObservation struct {
	Mode          string `json:"mode"`
	TriggerReason string `json:"trigger_reason,omitempty"`
	RunID         string `json:"run_id,omitempty"`
	// Asked is what the harness requested once the natural wait was over:
	// scoped_run, alert_investigate or full_run. Empty when it asked for
	// nothing.
	Asked     string `json:"asked,omitempty"`
	AskDetail string `json:"ask_detail,omitempty"`
	// InvestigationStart is self, or asked when the harness had to request
	// the investigation.
	InvestigationStart string        `json:"investigation_start,omitempty"`
	AlertAfter         time.Duration `json:"alert_after_ns,omitempty"`
	FindingAfter       time.Duration `json:"finding_after_ns,omitempty"`
	SettledAfter       time.Duration `json:"settled_after_ns,omitempty"`
}

type ObservedAlert struct {
	ID            string    `json:"id,omitempty"`
	Type          string    `json:"type"`
	Level         string    `json:"level"`
	ResourceAlias string    `json:"resource_alias"`
	ResourceName  string    `json:"resource_name,omitempty"`
	Message       string    `json:"message,omitempty"`
	StartTime     time.Time `json:"start_time,omitempty"`
}

// ResponseAction is the governed action an investigation proposed, as the
// harness saw it and dealt with it.
type ResponseAction struct {
	ActionID   string `json:"action_id,omitempty"`
	Capability string `json:"capability"`
	// TargetAlias is the lab resource the action targets. It is empty when
	// the target is not one of the scenario's resources.
	TargetAlias      string `json:"target_alias,omitempty"`
	TargetResourceID string `json:"target_resource_id,omitempty"`
	// Decision is approved_executed when the harness approved and executed
	// the scenario's own fix, and not_approved for anything else.
	Decision           string `json:"decision"`
	DecisionNote       string `json:"decision_note,omitempty"`
	FinalState         string `json:"final_state,omitempty"`
	VerificationStatus string `json:"verification_status,omitempty"`
	PlanHashBound      bool   `json:"plan_hash_bound"`
	OriginBound        bool   `json:"origin_bound"`
}

const (
	ActionDecisionApprovedExecuted = "approved_executed"
	ActionDecisionNotApproved      = "not_approved"
)

type ResponseFinding struct {
	ID                   string          `json:"id"`
	Severity             string          `json:"severity,omitempty"`
	Category             string          `json:"category,omitempty"`
	ResourceAlias        string          `json:"resource_alias"`
	ResourceName         string          `json:"resource_name,omitempty"`
	Title                string          `json:"title,omitempty"`
	InvestigationStatus  string          `json:"investigation_status,omitempty"`
	InvestigationOutcome string          `json:"investigation_outcome,omitempty"`
	TurnCount            int             `json:"turn_count,omitempty"`
	EvidenceCallCount    int             `json:"evidence_call_count,omitempty"`
	ToolsUsed            []string        `json:"tools_used,omitempty"`
	Conclusion           string          `json:"conclusion,omitempty"`
	Action               *ResponseAction `json:"action,omitempty"`
}

// ResponseEvidence is everything a grade is decided from. A live run fills it
// from the Pulse API and the lab oracle. The grader takes nothing else, so a
// stored report can be graded again and must come out the same.
type ResponseEvidence struct {
	AgentProfile AgentProfile    `json:"agent_profile"`
	Alerts       []ObservedAlert `json:"alerts,omitempty"`
	Wake         WakeObservation `json:"wake"`
	// Findings are the findings on the scenario's outcome resources.
	Findings []ResponseFinding `json:"findings,omitempty"`
	// OtherFindings were raised during the run on lab resources the fault
	// did not touch. They are reported and do not change the grade.
	OtherFindings []ResponseFinding `json:"other_findings,omitempty"`
	// Postconditions are the lab oracle's observations after an executed
	// action. They never come from Pulse.
	Postconditions []PredicateObservation `json:"postconditions,omitempty"`
}

// ResponseChecks is the machine-readable reasoning behind a grade.
type ResponseChecks struct {
	Looked            bool `json:"looked"`
	FixAvailable      bool `json:"fix_available"`
	ActionOnTarget    bool `json:"action_on_target"`
	ActionExecuted    bool `json:"action_executed"`
	LifecycleVerified bool `json:"lifecycle_verified"`
	PostconditionsMet bool `json:"postconditions_met"`
	ConclusionStored  bool `json:"conclusion_stored"`
	// DecisiveMissing and AreaMissing list the term groups the stored
	// conclusion did not satisfy.
	DecisiveMissing [][]string `json:"decisive_missing,omitempty"`
	AreaMissing     [][]string `json:"area_missing,omitempty"`
	// Misdirection lists the known-false statements the conclusion made.
	Misdirection []string `json:"misdirection,omitempty"`
}

type ResponseResult struct {
	Grade    Grade            `json:"grade"`
	Label    string           `json:"label"`
	Resolved bool             `json:"resolved"`
	Expect   string           `json:"expect"`
	Reason   string           `json:"reason"`
	Checks   ResponseChecks   `json:"checks"`
	Evidence ResponseEvidence `json:"evidence"`
}

// GradeResponse turns the evidence of one response run into a grade, by the
// scenario's rubric alone.
func GradeResponse(manifest Manifest, evidence ResponseEvidence) ResponseResult {
	result := ResponseResult{Evidence: evidence}
	spec := manifest.Outcome
	if spec == nil {
		return finishGrade(result, "", "scenario declares no outcome rubric")
	}
	result.Expect = spec.Expect

	outcomeAliases := manifest.outcomeResources()
	var findings []ResponseFinding
	for _, finding := range evidence.Findings {
		if contains(outcomeAliases, finding.ResourceAlias) {
			findings = append(findings, finding)
		}
	}
	result.Checks.Looked = len(findings) > 0

	if spec.Expect == OutcomeExpectQuiet {
		if len(findings) == 0 {
			return finishGrade(result, GradeQuiet, "nothing was wrong and no finding was raised")
		}
		return finishGrade(result, GradeFalseAlarm, fmt.Sprintf("%d finding(s) raised on a healthy resource", len(findings)))
	}
	if len(findings) == 0 {
		return finishGrade(result, GradeNotLookedAt, "no finding on the faulted resources")
	}

	result.Checks.FixAvailable = manifest.Remediation != nil && evidence.AgentProfile.CommandsEnabled
	if remediation := manifest.Remediation; remediation != nil {
		for _, finding := range findings {
			action := finding.Action
			if action == nil || action.TargetAlias != remediation.ActionTarget || !stringInFold(remediation.ExpectedCapabilities, action.Capability) {
				continue
			}
			result.Checks.ActionOnTarget = true
			result.Checks.ActionExecuted = action.Decision == ActionDecisionApprovedExecuted && action.FinalState == "completed"
			result.Checks.LifecycleVerified = !remediation.RequireLifecycleVerification ||
				stringInFold(remediation.AllowedVerificationStatuses, action.VerificationStatus)
			break
		}
		result.Checks.PostconditionsMet = allObservationsPassed(evidence.Postconditions)
		if result.Checks.ActionOnTarget && result.Checks.ActionExecuted && result.Checks.LifecycleVerified && result.Checks.PostconditionsMet {
			return finishGrade(result, GradeFixed, "the governed action ran, Pulse verified it and the lab saw the service recover")
		}
	}

	var conclusions []string
	for _, finding := range findings {
		if strings.EqualFold(finding.InvestigationStatus, "completed") && strings.TrimSpace(finding.Conclusion) != "" {
			conclusions = append(conclusions, finding.Conclusion)
		}
	}
	result.Checks.ConclusionStored = len(conclusions) > 0
	if !result.Checks.ConclusionStored {
		return finishGrade(result, GradeHandedBack, "a finding was raised and no investigation conclusion was stored")
	}
	text := newConclusionText(strings.Join(conclusions, "\n"))
	result.Checks.DecisiveMissing = text.missingGroups(spec.DecisiveTerms)
	result.Checks.AreaMissing = text.missingGroups(spec.AreaTerms)
	for _, term := range spec.MisdirectionTerms {
		if text.has(term) {
			result.Checks.Misdirection = append(result.Checks.Misdirection, term)
		}
	}
	decisive := len(result.Checks.DecisiveMissing) == 0
	area := len(spec.AreaTerms) > 0 && len(result.Checks.AreaMissing) == 0
	misdirected := len(result.Checks.Misdirection) > 0

	switch {
	case decisive && !misdirected && result.Checks.FixAvailable:
		return finishGrade(result, GradePartial, "the conclusion is right and the fix that was within reach was not carried out")
	case decisive && !misdirected:
		return finishGrade(result, GradeHandedOver, "the conclusion holds every decisive fact")
	case area:
		reason := "the conclusion found the right area and misses a decisive fact"
		if decisive {
			reason = "the conclusion found the cause and also sends the user the wrong way"
		}
		return finishGrade(result, GradePartial, reason)
	case misdirected:
		return finishGrade(result, GradeHandedBack, "the conclusion sends the user the wrong way")
	default:
		return finishGrade(result, GradeHandedBack, "the conclusion adds no decisive fact to the alert")
	}
}

func finishGrade(result ResponseResult, grade Grade, reason string) ResponseResult {
	result.Grade = grade
	result.Label = grade.Label()
	result.Resolved = grade.Resolved()
	result.Reason = reason
	return result
}

// conclusionText prepares a stored conclusion for term matching. Matching is
// by whole word, so "start" is not found inside "restart" and "database" is
// not found inside DATABASE_URL. A term made only of words and spaces also
// ignores punctuation in the text, so "agent is not connected" matches
// "command-agent is not connected". A term with its own punctuation, such as
// nextcloud-db or postgresql://, has to appear exactly.
type conclusionText struct {
	literal string
	words   string
}

func newConclusionText(value string) conclusionText {
	lower := strings.ToLower(value)
	var words strings.Builder
	words.Grow(len(lower) + 2)
	for _, r := range lower {
		if isTermWordRune(r) {
			words.WriteRune(r)
		} else {
			words.WriteByte(' ')
		}
	}
	return conclusionText{
		literal: strings.Join(strings.Fields(lower), " "),
		words:   " " + strings.Join(strings.Fields(words.String()), " ") + " ",
	}
}

func isTermWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
}

func (c conclusionText) has(term string) bool {
	term = strings.Join(strings.Fields(strings.ToLower(term)), " ")
	if term == "" {
		return false
	}
	plain := true
	for _, r := range term {
		if !isTermWordRune(r) && r != ' ' {
			plain = false
			break
		}
	}
	if plain {
		return strings.Contains(c.words, " "+term+" ")
	}
	first, last := rune(term[0]), rune(term[len(term)-1])
	for offset := 0; offset < len(c.literal); {
		index := strings.Index(c.literal[offset:], term)
		if index < 0 {
			return false
		}
		start, end := offset+index, offset+index+len(term)
		before := start == 0 || !isTermWordRune(first) || !isTermWordRune(rune(c.literal[start-1]))
		after := end == len(c.literal) || !isTermWordRune(last) || !isTermWordRune(rune(c.literal[end]))
		if before && after {
			return true
		}
		offset = start + 1
	}
	return false
}

// missingGroups returns the groups in which no accepted spelling appears.
func (c conclusionText) missingGroups(groups [][]string) [][]string {
	var missing [][]string
	for _, group := range groups {
		matched := false
		for _, term := range group {
			if c.has(term) {
				matched = true
				break
			}
		}
		if !matched {
			missing = append(missing, append([]string(nil), group...))
		}
	}
	return missing
}

// ScorecardCell is one fault under one model and agent profile.
type ScorecardCell struct {
	RunID         string        `json:"run_id"`
	ScenarioID    string        `json:"scenario_id"`
	Grade         Grade         `json:"grade"`
	Label         string        `json:"label"`
	Resolved      bool          `json:"resolved"`
	Reason        string        `json:"reason"`
	Wake          string        `json:"wake"`
	TriggerReason string        `json:"trigger_reason,omitempty"`
	SettledAfter  time.Duration `json:"settled_after_ns,omitempty"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

type ScorecardColumn struct {
	Model        string `json:"model"`
	AgentProfile string `json:"agent_profile"`
	// Resolved and Runs count runs, so a fault that ran twice counts twice.
	Resolved int `json:"resolved"`
	Runs     int `json:"runs"`
}

type ScorecardRow struct {
	FaultLabel string `json:"fault_label"`
	ScenarioID string `json:"scenario_id"`
	Title      string `json:"title"`
	// Expect is what a maintenance person would do with this fault.
	Expect string `json:"expect"`
	// Cells holds, per column, every run of this fault, oldest first.
	Cells [][]ScorecardCell `json:"cells"`
}

// Scorecard is the fault set table: one row per fault, one column per model
// and agent profile.
type Scorecard struct {
	FaultSet string            `json:"fault_set"`
	Columns  []ScorecardColumn `json:"columns"`
	Rows     []ScorecardRow    `json:"rows"`
	// Invalid lists runs that produced no grade, with the reason. They are
	// harness or lab failures, never a verdict on Patrol.
	Invalid []string `json:"invalid,omitempty"`
}

const (
	metadataFaultSet   = "fault_set"
	metadataFaultLabel = "fault_label"
)

// FaultSetManifests returns the response scenarios of one fault set in label
// order. A fault set is nothing more than a shared metadata value, so a later
// set, on another platform, is added by writing manifests.
func FaultSetManifests(catalog Catalog, faultSet string) []Manifest {
	var manifests []Manifest
	for _, manifest := range catalog.Manifests {
		if manifest.Track == TrackResponse && manifest.Metadata[metadataFaultSet] == faultSet {
			manifests = append(manifests, manifest)
		}
	}
	sort.SliceStable(manifests, func(i, j int) bool {
		return manifests[i].Metadata[metadataFaultLabel] < manifests[j].Metadata[metadataFaultLabel]
	})
	return manifests
}

// BuildScorecard grades every response report again from its stored evidence
// and lays the results out as the fault set table.
func BuildScorecard(faultSet string, reports []RunReport) Scorecard {
	card := Scorecard{FaultSet: faultSet}
	columnIndex := make(map[string]int)
	rowIndex := make(map[string]int)
	sort.SliceStable(reports, func(i, j int) bool { return reports[i].GeneratedAt.Before(reports[j].GeneratedAt) })
	type placed struct {
		row, column int
		cell        ScorecardCell
	}
	var cells []placed
	for _, report := range reports {
		manifest := report.Manifest
		if manifest.Track != TrackResponse || manifest.Metadata[metadataFaultSet] != faultSet {
			continue
		}
		if report.Response == nil || report.Response.Grade == "" {
			card.Invalid = append(card.Invalid, fmt.Sprintf("%s %s: %s", report.RunID, manifest.ID, strings.Join(report.Errors, "; ")))
			continue
		}
		graded := GradeResponse(manifest, report.Response.Evidence)
		label := manifest.Metadata[metadataFaultLabel]
		if _, ok := rowIndex[label]; !ok {
			rowIndex[label] = len(card.Rows)
			card.Rows = append(card.Rows, ScorecardRow{FaultLabel: label, ScenarioID: manifest.ID, Title: manifest.Title, Expect: graded.Expect})
		}
		columnKey := report.Environment.Model + "\x00" + graded.Evidence.AgentProfile.Name
		if _, ok := columnIndex[columnKey]; !ok {
			columnIndex[columnKey] = len(card.Columns)
			card.Columns = append(card.Columns, ScorecardColumn{Model: report.Environment.Model, AgentProfile: graded.Evidence.AgentProfile.Name})
		}
		cells = append(cells, placed{row: rowIndex[label], column: columnIndex[columnKey], cell: ScorecardCell{
			RunID: report.RunID, ScenarioID: manifest.ID, Grade: graded.Grade, Label: graded.Label, Resolved: graded.Resolved,
			Reason: graded.Reason, Wake: graded.Evidence.Wake.Mode, TriggerReason: graded.Evidence.Wake.TriggerReason,
			SettledAfter: graded.Evidence.Wake.SettledAfter, GeneratedAt: report.GeneratedAt,
		}})
	}
	// Columns and rows are sorted for a stable table, so cells are placed by
	// key after sorting, not by the order reports were read.
	columnOrder := make([]int, len(card.Columns))
	for i := range columnOrder {
		columnOrder[i] = i
	}
	sort.SliceStable(columnOrder, func(i, j int) bool {
		left, right := card.Columns[columnOrder[i]], card.Columns[columnOrder[j]]
		if left.Model != right.Model {
			return left.Model < right.Model
		}
		// commands-on sorts before commands-off, the order of the Phase 1 table.
		return left.AgentProfile > right.AgentProfile
	})
	rowOrder := make([]int, len(card.Rows))
	for i := range rowOrder {
		rowOrder[i] = i
	}
	sort.SliceStable(rowOrder, func(i, j int) bool { return card.Rows[rowOrder[i]].FaultLabel < card.Rows[rowOrder[j]].FaultLabel })
	columnPosition := make(map[int]int, len(columnOrder))
	sortedColumns := make([]ScorecardColumn, len(columnOrder))
	for position, original := range columnOrder {
		columnPosition[original] = position
		sortedColumns[position] = card.Columns[original]
	}
	rowPosition := make(map[int]int, len(rowOrder))
	sortedRows := make([]ScorecardRow, len(rowOrder))
	for position, original := range rowOrder {
		rowPosition[original] = position
		sortedRows[position] = card.Rows[original]
		sortedRows[position].Cells = make([][]ScorecardCell, len(sortedColumns))
	}
	for _, item := range cells {
		row, column := rowPosition[item.row], columnPosition[item.column]
		sortedRows[row].Cells[column] = append(sortedRows[row].Cells[column], item.cell)
		sortedColumns[column].Runs++
		if item.cell.Resolved {
			sortedColumns[column].Resolved++
		}
	}
	card.Columns, card.Rows = sortedColumns, sortedRows
	sort.Strings(card.Invalid)
	return card
}

func agentProfileLabel(name string) string {
	switch name {
	case AgentProfileCommandsOn:
		return "commands on"
	case AgentProfileCommandsOff:
		return "default agent (commands off)"
	default:
		return name
	}
}

func expectLabel(expect string) string {
	switch expect {
	case OutcomeExpectFixed:
		return "fix it"
	case OutcomeExpectHandedOver:
		return "say exactly what is wrong"
	case OutcomeExpectQuiet:
		return "nothing"
	default:
		return expect
	}
}

func wakeLabel(cell ScorecardCell) string {
	switch cell.Wake {
	case WakeSelf:
		if cell.TriggerReason != "" {
			return "woke by itself (" + cell.TriggerReason + ")"
		}
		return "woke by itself"
	case WakeAsked:
		return "had to be asked"
	default:
		return ""
	}
}

// RenderScorecardMarkdown prints the fault set table. Each cell carries both
// the grade and how the run started.
func RenderScorecardMarkdown(card Scorecard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patrol fault set %s scorecard\n\n", markdownCell(card.FaultSet))
	if len(card.Columns) == 0 {
		b.WriteString("No graded runs.\n")
	} else {
		b.WriteString("| Fault | A person would |")
		for _, column := range card.Columns {
			fmt.Fprintf(&b, " %s, %s |", markdownCell(column.Model), agentProfileLabel(column.AgentProfile))
		}
		b.WriteString("\n|---|---|")
		for range card.Columns {
			b.WriteString("---|")
		}
		b.WriteString("\n")
		for _, row := range card.Rows {
			fmt.Fprintf(&b, "| %s %s | %s |", markdownCell(row.FaultLabel), markdownCell(row.Title), expectLabel(row.Expect))
			for _, runs := range row.Cells {
				if len(runs) == 0 {
					b.WriteString(" not run |")
					continue
				}
				parts := make([]string, 0, len(runs))
				for _, cell := range runs {
					text := "**" + cell.Label + "**"
					if wake := wakeLabel(cell); wake != "" {
						text += ", " + wake
					}
					parts = append(parts, text)
				}
				fmt.Fprintf(&b, " %s |", markdownCell(strings.Join(parts, "<br>")))
			}
			b.WriteString("\n")
		}
		b.WriteString("| **Resolved or correctly handed over** | |")
		for _, column := range card.Columns {
			fmt.Fprintf(&b, " **%d of %d** |", column.Resolved, column.Runs)
		}
		b.WriteString("\n")
	}
	if len(card.Invalid) > 0 {
		b.WriteString("\nRuns with no grade (lab or harness failure, not a verdict on Patrol):\n\n")
		for _, line := range card.Invalid {
			fmt.Fprintf(&b, "- %s\n", sanitizeArtifactText(line))
		}
	}
	return sanitizeArtifactText(b.String())
}

// MissingHostFilesystems lists the host filesystem mounts the manifests need
// and the operator did not hand to the lab.
func MissingHostFilesystems(manifests []Manifest, provided []string) []string {
	var missing []string
	for _, manifest := range manifests {
		for _, resource := range manifest.Resources {
			if resource.Kind == resourceKindHostFilesystem && !contains(provided, resource.MountPath) && !contains(missing, resource.MountPath) {
				missing = append(missing, resource.MountPath)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// WriteScorecard writes the fault set table next to its machine-readable form.
func WriteScorecard(dir string, card Scorecard) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "scorecard.json"), card); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "scorecard.md"), []byte(RenderScorecardMarkdown(card)), 0o600)
}
