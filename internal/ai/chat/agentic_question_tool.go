package chat

import (
	"bytes"
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

type questionToolType string

const (
	questionToolTypeText   questionToolType = "text"
	questionToolTypeSelect questionToolType = "select"
)

type questionToolInputPayload struct {
	Questions json.RawMessage `json:"questions"`
}

type questionToolInputQuestion struct {
	ID       string          `json:"id"`
	Type     string          `json:"type,omitempty"`
	Header   string          `json:"header,omitempty"`
	Question string          `json:"question"`
	Options  json.RawMessage `json:"options,omitempty"`
}

type questionToolInputOption struct {
	Label       string `json:"label"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}

func parseQuestionToolInput(input map[string]interface{}) ([]Question, error) {
	payloadBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("invalid question payload: %w", err)
	}

	var payload questionToolInputPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid question payload: %w", err)
	}

	if len(payload.Questions) == 0 {
		return nil, fmt.Errorf("missing required field: questions")
	}

	var rawQuestions []json.RawMessage
	if err := json.Unmarshal(payload.Questions, &rawQuestions); err != nil {
		return nil, fmt.Errorf("questions must be an array")
	}

	if len(rawQuestions) == 0 {
		return nil, fmt.Errorf("questions must not be empty")
	}

	questions := make([]Question, 0, len(rawQuestions))
	for _, rawQuestion := range rawQuestions {
		var parsedQuestion questionToolInputQuestion
		if err := json.Unmarshal(rawQuestion, &parsedQuestion); err != nil {
			return nil, fmt.Errorf("invalid question entry")
		}

		id := strings.TrimSpace(parsedQuestion.ID)
		questionText := strings.TrimSpace(parsedQuestion.Question)
		header := strings.TrimSpace(parsedQuestion.Header)

		if id == "" {
			return nil, fmt.Errorf("question.id is required")
		}
		if questionText == "" {
			return nil, fmt.Errorf("question.question is required")
		}

		opts, err := parseQuestionToolOptions(parsedQuestion.Options)
		if err != nil {
			return nil, err
		}

		qType, err := normalizeQuestionToolType(parsedQuestion.Type, len(opts) > 0)
		if err != nil {
			return nil, err
		}

		q := Question{
			ID:       id,
			Type:     string(qType),
			Question: questionText,
			Options:  opts,
		}
		if header != "" {
			q.Header = header
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func parseQuestionToolOptions(rawOptions json.RawMessage) ([]QuestionOption, error) {
	if len(rawOptions) == 0 {
		return nil, nil
	}

	trimmed := bytes.TrimSpace(rawOptions)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("question.options must be an array")
	}

	var decodedOptions []questionToolInputOption
	if err := json.Unmarshal(rawOptions, &decodedOptions); err != nil {
		return nil, fmt.Errorf("question.options must be an array")
	}

	options := make([]QuestionOption, 0, len(decodedOptions))
	for _, decodedOption := range decodedOptions {
		label := strings.TrimSpace(decodedOption.Label)
		value := strings.TrimSpace(decodedOption.Value)
		desc := strings.TrimSpace(decodedOption.Description)

		if label == "" {
			return nil, fmt.Errorf("option.label is required")
		}
		if value == "" {
			value = label
		}

		option := QuestionOption{Label: label, Value: value}
		if desc != "" {
			option.Description = desc
		}
		options = append(options, option)
	}

	return options, nil
}

func normalizeQuestionToolType(rawType string, hasOptions bool) (questionToolType, error) {
	normalizedType := strings.TrimSpace(strings.ToLower(rawType))
	if normalizedType == "" {
		if hasOptions {
			normalizedType = string(questionToolTypeSelect)
		} else {
			normalizedType = string(questionToolTypeText)
		}
	}

	switch questionToolType(normalizedType) {
	case questionToolTypeText:
		return questionToolTypeText, nil
	case questionToolTypeSelect:
		if !hasOptions {
			return "", fmt.Errorf("select questions must include options")
		}
		return questionToolTypeSelect, nil
	default:
		return "", fmt.Errorf("question.type must be 'text' or 'select'")
	}
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
