package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

const pulseQuestionToolName = agentcapabilities.PulseQuestionToolName

func parseQuestionToolInput(input map[string]interface{}) ([]Question, error) {
	parsedQuestions, err := agentcapabilities.ParsePulseQuestionToolInput(input)
	if err != nil {
		return nil, err
	}

	questions := make([]Question, 0, len(parsedQuestions))
	for _, parsedQuestion := range parsedQuestions {
		options := make([]QuestionOption, 0, len(parsedQuestion.Options))
		for _, parsedOption := range parsedQuestion.Options {
			option := QuestionOption{
				Label: parsedOption.Label,
				Value: parsedOption.Value,
			}
			if parsedOption.Description != "" {
				option.Description = parsedOption.Description
			}
			options = append(options, option)
		}

		question := Question{
			ID:       parsedQuestion.ID,
			Type:     string(parsedQuestion.Type),
			Question: parsedQuestion.Question,
			Options:  options,
		}
		if parsedQuestion.Header != "" {
			question.Header = parsedQuestion.Header
		}
		questions = append(questions, question)
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
			out, err := json.Marshal(payload)
			if err != nil {
				return fmt.Sprintf("Error: failed to encode answers: %v", err), true
			}
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
