package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseQuestionToolInput_Valid(t *testing.T) {
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
	assert.Equal(t, "select", questions[0].Type)
	assert.Equal(t, "Pick one", questions[0].Question)
	require.Len(t, questions[0].Options, 1)
	assert.Equal(t, "A", questions[0].Options[0].Label)
	assert.Equal(t, "A", questions[0].Options[0].Value)
	assert.Equal(t, "Alpha", questions[0].Options[0].Description)

	assert.Equal(t, "q2", questions[1].ID)
	assert.Equal(t, "text", questions[1].Type)
	assert.Equal(t, "Why?", questions[1].Question)
	assert.Equal(t, "Context", questions[1].Header)
}

func TestParseQuestionToolInput_Errors(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expectedErr string
	}{
		{
			name:        "missing questions",
			input:       map[string]interface{}{},
			expectedErr: "missing required field: questions",
		},
		{
			name: "questions must be array",
			input: map[string]interface{}{
				"questions": map[string]interface{}{},
			},
			expectedErr: "questions must be an array",
		},
		{
			name: "questions must not be empty",
			input: map[string]interface{}{
				"questions": []interface{}{},
			},
			expectedErr: "questions must not be empty",
		},
		{
			name: "invalid question entry",
			input: map[string]interface{}{
				"questions": []interface{}{"not-an-object"},
			},
			expectedErr: "invalid question entry",
		},
		{
			name: "missing question id",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one",
					},
				},
			},
			expectedErr: "question.id is required",
		},
		{
			name: "missing question text",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"id": "q1",
					},
				},
			},
			expectedErr: "question.question is required",
		},
		{
			name: "invalid question type",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"id":       "q1",
						"type":     "binary",
						"question": "Pick one",
					},
				},
			},
			expectedErr: "question.type must be 'text' or 'select'",
		},
		{
			name: "select requires options",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"id":       "q1",
						"type":     "select",
						"question": "Pick one",
					},
				},
			},
			expectedErr: "select questions must include options",
		},
		{
			name: "options must be array",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"id":       "q1",
						"question": "Pick one",
						"options":  map[string]interface{}{},
					},
				},
			},
			expectedErr: "question.options must be an array",
		},
		{
			name: "null options rejected",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"id":       "q1",
						"question": "Pick one",
						"options":  nil,
					},
				},
			},
			expectedErr: "question.options must be an array",
		},
		{
			name: "option label required",
			input: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"id":       "q1",
						"question": "Pick one",
						"options": []interface{}{
							map[string]interface{}{
								"label": "  ",
							},
						},
					},
				},
			},
			expectedErr: "option.label is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseQuestionToolInput(tc.input)
			require.EqualError(t, err, tc.expectedErr)
		})
	}
}
