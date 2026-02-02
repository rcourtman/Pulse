package eval

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/stretchr/testify/assert"
)

func TestSeverityRank(t *testing.T) {
	assert.Equal(t, 3, severityRank("CRITICAL"))
	assert.Equal(t, 3, severityRank("critical"))
	assert.Equal(t, 2, severityRank("warning"))
	assert.Equal(t, 1, severityRank("watch"))
	assert.Equal(t, 0, severityRank("info"))
	assert.Equal(t, 0, severityRank("unknown"))
}

func TestResourceMatches(t *testing.T) {
	tests := []struct {
		name     string
		signal   ai.DetectedSignal
		finding  *PatrolFinding
		expected bool
	}{
		{
			name:   "Exact ID match",
			signal: ai.DetectedSignal{ResourceID: "res-123"},
			finding: &PatrolFinding{
				ResourceID: "res-123",
			},
			expected: true,
		},
		{
			name:   "Exact Name match",
			signal: ai.DetectedSignal{ResourceName: "my-pod"},
			finding: &PatrolFinding{
				ResourceName: "my-pod",
			},
			expected: true,
		},
		{
			name:   "Signal ID matches Finding Name",
			signal: ai.DetectedSignal{ResourceID: "my-pod"},
			finding: &PatrolFinding{
				ResourceName: "my-pod",
			},
			expected: true,
		},
		{
			name:   "Suffix match",
			signal: ai.DetectedSignal{ResourceID: "pod-1"},
			finding: &PatrolFinding{
				ResourceID: "cluster:ns:pod-1",
			},
			expected: true,
		},
		{
			name:   "No match",
			signal: ai.DetectedSignal{ResourceID: "pod-1"},
			finding: &PatrolFinding{
				ResourceID: "pod-2",
			},
			expected: false,
		},
		{
			name:     "Nil finding",
			signal:   ai.DetectedSignal{ResourceID: "pod-1"},
			finding:  nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, resourceMatches(tc.signal, tc.finding))
		})
	}
}

func TestMatchSignalToFinding(t *testing.T) {
	findings := []PatrolFinding{
		{
			ID:         "f1",
			ResourceID: "res-1",
			Category:   "performance",
			Severity:   "warning",
			Title:      "High CPU",
		},
		{
			ID:         "f2",
			ResourceID: "res-2",
			Category:   "security",
			Severity:   "critical",
			Title:      "Vulnerability",
		},
	}

	tests := []struct {
		name          string
		signal        ai.DetectedSignal
		expectedMatch bool
		expectedID    string
	}{
		{
			name: "Match Found",
			signal: ai.DetectedSignal{
				ResourceID:        "res-1",
				Category:          "performance",
				SuggestedSeverity: "warning",
			},
			expectedMatch: true,
			expectedID:    "f1",
		},
		{
			name: "Match Found (Severity Higher in Finding)",
			signal: ai.DetectedSignal{
				ResourceID:        "res-2",
				Category:          "security",
				SuggestedSeverity: "warning", // Finding is critical
			},
			expectedMatch: true,
			expectedID:    "f2",
		},
		{
			name: "No Match (Category)",
			signal: ai.DetectedSignal{
				ResourceID: "res-1",
				Category:   "security",
			},
			expectedMatch: false,
		},
		{
			name: "No Match (Severity Lower in Finding)",
			signal: ai.DetectedSignal{
				ResourceID:        "res-1",
				Category:          "performance",
				SuggestedSeverity: "critical", // Finding is warning
			},
			expectedMatch: false,
		},
		{
			name: "No Match (Resource)",
			signal: ai.DetectedSignal{
				ResourceID: "res-999",
				Category:   "performance",
			},
			expectedMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			match, _, f := matchSignalToFinding(tc.signal, findings)
			assert.Equal(t, tc.expectedMatch, match)
			if tc.expectedMatch {
				assert.NotNil(t, f)
				assert.Equal(t, tc.expectedID, f.ID)
			}
		})
	}
}

func TestEvaluatePatrolQuality_NoTools(t *testing.T) {
	result := &PatrolRunResult{
		ToolCalls: []ToolCallEvent{},
	}
	report := EvaluatePatrolQuality(result)
	assert.NotNil(t, report)
	assert.False(t, report.CoverageKnown)
	assert.Equal(t, 0, report.ToolCallsSeen)
}

func TestEvaluatePatrolQuality_NilResult(t *testing.T) {
	assert.Nil(t, EvaluatePatrolQuality(nil))
}
