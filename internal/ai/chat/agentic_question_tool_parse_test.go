package chat

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseQuestionToolInputMapsSharedQuestionPayloadToChatQuestions(t *testing.T) {
	input := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"id":       " q1 ",
				"question": " Pick one ",
				"options": []interface{}{
					map[string]interface{}{
						"label":       " A ",
						"value":       "",
						"description": " Alpha ",
					},
				},
			},
			map[string]interface{}{
				"id":       "q2",
				"type":     " TEXT ",
				"question": " Why? ",
				"header":   " Context ",
			},
		},
	}

	questions, err := parseQuestionToolInput(input)
	require.NoError(t, err)
	require.Len(t, questions, 2)

	assert.Equal(t, "q1", questions[0].ID)
	assert.Equal(t, string(agentcapabilities.PulseQuestionToolTypeSelect), questions[0].Type)
	assert.Equal(t, "Pick one", questions[0].Question)
	require.Len(t, questions[0].Options, 1)
	assert.Equal(t, QuestionOption{Label: "A", Value: "A", Description: "Alpha"}, questions[0].Options[0])

	assert.Equal(t, "q2", questions[1].ID)
	assert.Equal(t, string(agentcapabilities.PulseQuestionToolTypeText), questions[1].Type)
	assert.Equal(t, "Why?", questions[1].Question)
	assert.Equal(t, "Context", questions[1].Header)
}

func TestParseQuestionToolInputReturnsSharedParserErrors(t *testing.T) {
	_, err := parseQuestionToolInput(map[string]interface{}{})
	require.EqualError(t, err, "missing required field: questions")
}
