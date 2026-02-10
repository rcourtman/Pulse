package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

func userQuestionTool() providers.Tool {
	return providers.Tool{
		Name:        pulseQuestionToolName,
		Description: "Ask the user for missing information using a structured prompt. Use this when you must clarify before proceeding (e.g., choose a target, confirm a risky action, or select among options).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"questions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "Stable identifier for this question (used in the answer payload).",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"description": "Question type. Use 'text' for free-form input or 'select' for predefined options.",
								"enum":        []string{"text", "select"},
							},
							"header": map[string]interface{}{
								"type":        "string",
								"description": "Optional short context shown above the question.",
							},
							"question": map[string]interface{}{
								"type":        "string",
								"description": "The question shown to the user.",
							},
							"options": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"label": map[string]interface{}{"type": "string"},
										"value": map[string]interface{}{"type": "string"},
										"description": map[string]interface{}{
											"type":        "string",
											"description": "Optional detail shown under the label.",
										},
									},
									"required": []string{"label"},
								},
							},
						},
						"required": []string{"id", "question"},
					},
				},
			},
			"required": []string{"questions"},
		},
	}
}

const pulseQuestionToolName = "pulse_question"

func parseQuestionToolInput(input map[string]interface{}) ([]Question, error) {
	rawQs, ok := input["questions"]
	if !ok {
		return nil, fmt.Errorf("missing required field: questions")
	}
	rawSlice, ok := rawQs.([]interface{})
	if !ok {
		return nil, fmt.Errorf("questions must be an array")
	}

	var questions []Question
	for _, raw := range rawSlice {
		m, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid question entry")
		}

		id, _ := m["id"].(string)
		questionText, _ := m["question"].(string)
		header, _ := m["header"].(string)
		qType, _ := m["type"].(string)

		id = strings.TrimSpace(id)
		questionText = strings.TrimSpace(questionText)
		header = strings.TrimSpace(header)
		qType = strings.TrimSpace(strings.ToLower(qType))

		if id == "" {
			return nil, fmt.Errorf("question.id is required")
		}
		if questionText == "" {
			return nil, fmt.Errorf("question.question is required")
		}

		var opts []QuestionOption
		if rawOpts, ok := m["options"]; ok {
			arr, ok := rawOpts.([]interface{})
			if !ok {
				return nil, fmt.Errorf("question.options must be an array")
			}
			for _, rawOpt := range arr {
				om, ok := rawOpt.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid option entry")
				}
				label, _ := om["label"].(string)
				value, _ := om["value"].(string)
				desc, _ := om["description"].(string)

				label = strings.TrimSpace(label)
				value = strings.TrimSpace(value)
				desc = strings.TrimSpace(desc)

				if label == "" {
					return nil, fmt.Errorf("option.label is required")
				}
				if value == "" {
					value = label
				}
				opt := QuestionOption{Label: label, Value: value}
				if desc != "" {
					opt.Description = desc
				}
				opts = append(opts, opt)
			}
		}

		if qType == "" {
			if len(opts) > 0 {
				qType = "select"
			} else {
				qType = "text"
			}
		}
		if qType != "text" && qType != "select" {
			return nil, fmt.Errorf("question.type must be 'text' or 'select'")
		}
		if qType == "select" && len(opts) == 0 {
			return nil, fmt.Errorf("select questions must include options")
		}

		q := Question{
			ID:       id,
			Type:     qType,
			Question: questionText,
			Options:  opts,
		}
		if header != "" {
			q.Header = header
		}
		questions = append(questions, q)
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("questions must not be empty")
	}
	return questions, nil
}

func (a *AgenticLoop) executeQuestionTool(ctx context.Context, sessionID string, tc providers.ToolCall, callback StreamCallback) (string, bool) {
	a.mu.Lock()
	autonomous := a.autonomousMode
	a.mu.Unlock()
	if autonomous {
		return "Error: pulse_question cannot be used in autonomous mode (no interactive user available). Proceed with safe defaults or ask the user in plain text.", true
	}

	questions, err := parseQuestionToolInput(tc.Input)
	if err != nil {
		return fmt.Sprintf("Error: invalid pulse_question input: %v", err), true
	}

	questionID := uuid.New().String()
	ch := make(chan []QuestionAnswer, 1)

	a.mu.Lock()
	a.pendingQs[questionID] = ch
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.pendingQs, questionID)
		a.mu.Unlock()
	}()

	jsonData, _ := json.Marshal(QuestionData{
		SessionID:  sessionID,
		QuestionID: questionID,
		Questions:  questions,
	})
	callback(StreamEvent{Type: "question", Data: jsonData})

	log.Info().
		Str("session_id", sessionID).
		Str("question_id", questionID).
		Int("questions", len(questions)).
		Msg("[AgenticLoop] Waiting for user to answer pulse_question")

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Sprintf("Error: question canceled: %v", ctx.Err()), true
		case answers := <-ch:
			payload := map[string]interface{}{
				"question_id": questionID,
				"answers":     answers,
			}
			out, _ := json.Marshal(payload)
			return string(out), false
		case <-ticker.C:
			a.mu.Lock()
			aborted := a.aborted[sessionID]
			a.mu.Unlock()
			if aborted {
				return "Error: question canceled: session aborted", true
			}
		}
	}
}
