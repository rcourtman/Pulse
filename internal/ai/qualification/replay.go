package qualification

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const ReplaySchemaVersion = "patrol.qualification.replay/v1"

// ReplayBundle is the deterministic record/replay layer beneath live
// qualification. It preserves the real run's tool transcript and final model
// products, while keeping scenario ground truth independent and immutable.
// Passing replay proves parser/scorer regressions only; it never substitutes
// for a live collection-path or real-model qualification run.
type ReplayBundle struct {
	SchemaVersion  string         `json:"schema_version"`
	CapturedAt     time.Time      `json:"captured_at"`
	RunID          string         `json:"run_id"`
	ManifestID     string         `json:"manifest_id"`
	ManifestDigest string         `json:"manifest_digest"`
	Model          string         `json:"model"`
	GroundTruth    GroundTruth    `json:"ground_truth"`
	Exchanges      []ToolExchange `json:"tool_exchanges"`
	AIAnalysis     string         `json:"ai_analysis,omitempty"`
	Findings       []Finding      `json:"findings"`
}

type ToolExchange struct {
	Sequence       int    `json:"sequence"`
	ToolCallID     string `json:"tool_call_id,omitempty"`
	ToolName       string `json:"tool_name"`
	CanonicalInput string `json:"canonical_input"`
	Output         string `json:"output"`
	Success        bool   `json:"success"`
}

func BuildReplayBundle(report RunReport) (ReplayBundle, error) {
	digest, err := report.Manifest.Digest()
	if err != nil {
		return ReplayBundle{}, err
	}
	bundle := ReplayBundle{
		SchemaVersion: ReplaySchemaVersion, CapturedAt: time.Now().UTC(), RunID: report.RunID,
		ManifestID: report.Manifest.ID, ManifestDigest: digest, Model: report.Environment.Model,
		GroundTruth: report.GroundTruth, AIAnalysis: report.PatrolRun.AIAnalysis,
		Findings: append([]Finding(nil), report.Findings...),
	}
	for index, call := range report.PatrolRun.ToolCalls {
		input, err := canonicalToolInput(call.Input)
		if err != nil {
			return ReplayBundle{}, fmt.Errorf("canonicalize tool call %s input: %w", call.ID, err)
		}
		bundle.Exchanges = append(bundle.Exchanges, ToolExchange{
			Sequence: index + 1, ToolCallID: call.ID, ToolName: call.ToolName,
			CanonicalInput: input, Output: call.Output, Success: call.Success,
		})
	}
	return bundle, nil
}

func canonicalToolInput(input string) (string, error) {
	input = string(bytes.TrimSpace([]byte(input)))
	if input == "" {
		return "{}", nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewBufferString(input))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return "", errors.New("multiple JSON values")
		}
		return "", fmt.Errorf("trailing JSON data: %w", err)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// ReplaySession consumes the capture in original order. Reordering, adding,
// dropping, or changing any tool input is a replay miss rather than silently
// returning a fixture chosen by tool name alone.
type ReplaySession struct {
	bundle ReplayBundle
	next   int
}

func NewReplaySession(bundle ReplayBundle) (*ReplaySession, error) {
	if bundle.SchemaVersion != ReplaySchemaVersion {
		return nil, fmt.Errorf("unsupported replay schema %q", bundle.SchemaVersion)
	}
	for index, exchange := range bundle.Exchanges {
		if exchange.Sequence != index+1 || exchange.ToolName == "" || exchange.CanonicalInput == "" {
			return nil, fmt.Errorf("invalid replay exchange at index %d", index)
		}
	}
	return &ReplaySession{bundle: bundle}, nil
}

func (s *ReplaySession) Call(toolName, input string) (ToolExchange, error) {
	if s.next >= len(s.bundle.Exchanges) {
		return ToolExchange{}, fmt.Errorf("unexpected extra replay tool call %q", toolName)
	}
	canonical, err := canonicalToolInput(input)
	if err != nil {
		return ToolExchange{}, err
	}
	expected := s.bundle.Exchanges[s.next]
	if expected.ToolName != toolName || expected.CanonicalInput != canonical {
		return ToolExchange{}, fmt.Errorf("replay mismatch at sequence %d: got %s %s, want %s %s", expected.Sequence, toolName, canonical, expected.ToolName, expected.CanonicalInput)
	}
	s.next++
	return expected, nil
}

func (s *ReplaySession) Complete() error {
	if s.next != len(s.bundle.Exchanges) {
		return fmt.Errorf("replay ended after %d of %d tool calls", s.next, len(s.bundle.Exchanges))
	}
	return nil
}

func LoadReplayBundle(path string) (ReplayBundle, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ReplayBundle{}, err
	}
	var bundle ReplayBundle
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return ReplayBundle{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ReplayBundle{}, errors.New("replay bundle contains multiple JSON values")
		}
		return ReplayBundle{}, err
	}
	if _, err := NewReplaySession(bundle); err != nil {
		return ReplayBundle{}, err
	}
	return bundle, nil
}
