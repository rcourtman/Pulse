package agentcapabilities

import "testing"

func TestParsePulseQuestionToolInputValid(t *testing.T) {
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

	questions, err := ParsePulseQuestionToolInput(input)
	if err != nil {
		t.Fatalf("ParsePulseQuestionToolInput returned error: %v", err)
	}
	if len(questions) != 2 {
		t.Fatalf("questions len = %d, want 2", len(questions))
	}

	first := questions[0]
	if first.ID != "q1" || first.Type != PulseQuestionToolTypeSelect || first.Question != "Pick one" {
		t.Fatalf("first question = %#v", first)
	}
	if len(first.Options) != 1 {
		t.Fatalf("first options len = %d, want 1", len(first.Options))
	}
	if first.Options[0] != (PulseQuestionToolOption{Label: "A", Value: "A", Description: "Alpha"}) {
		t.Fatalf("first option = %#v", first.Options[0])
	}

	second := questions[1]
	if second.ID != "q2" || second.Type != PulseQuestionToolTypeText || second.Question != "Why?" || second.Header != "Context" {
		t.Fatalf("second question = %#v", second)
	}
}

func TestParsePulseQuestionToolInputErrors(t *testing.T) {
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
			_, err := ParsePulseQuestionToolInput(tc.input)
			if err == nil || err.Error() != tc.expectedErr {
				t.Fatalf("ParsePulseQuestionToolInput error = %v, want %q", err, tc.expectedErr)
			}
		})
	}
}
