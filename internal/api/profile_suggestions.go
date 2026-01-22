package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// ProfileSuggestionHandler handles AI-assisted profile suggestions
type ProfileSuggestionHandler struct {
	persistence *config.ConfigPersistence
	aiHandler   *AIHandler
}

// NewProfileSuggestionHandler creates a new suggestion handler
func NewProfileSuggestionHandler(persistence *config.ConfigPersistence, aiHandler *AIHandler) *ProfileSuggestionHandler {
	return &ProfileSuggestionHandler{
		persistence: persistence,
		aiHandler:   aiHandler,
	}
}

// SuggestionRequest is the request body for profile suggestions
type SuggestionRequest struct {
	Prompt string `json:"prompt"`
}

// ProfileSuggestion is the AI-generated profile suggestion
type ProfileSuggestion struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
	Rationale   []string               `json:"rationale"`
}

// HandleSuggestProfile handles POST /api/admin/profiles/suggestions
func (h *ProfileSuggestionHandler) HandleSuggestProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if AI is running
	if h.aiHandler == nil || !h.aiHandler.IsRunning(r.Context()) {
		http.Error(w, "AI service is not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request
	var req SuggestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate prompt is not empty
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Build context for the AI
	contextParts := []string{}

	// Add existing profiles for reference
	profiles, err := h.persistence.LoadAgentProfiles()
	if err == nil && len(profiles) > 0 {
		profileNames := make([]string, len(profiles))
		for i, p := range profiles {
			profileNames[i] = p.Name
		}
		contextParts = append(contextParts, fmt.Sprintf("Existing profiles: %s", strings.Join(profileNames, ", ")))
	}

	// Build config schema documentation from the actual definitions
	configDocs := buildConfigSchemaDoc()

	// Build the prompt for the AI (schema docs only in system prompt, not in context)
	systemPrompt := fmt.Sprintf(`You are an infrastructure configuration assistant for Pulse, a monitoring platform.
Your task is to suggest an agent configuration profile based on the user's request.

IMPORTANT: You must respond ONLY with a valid JSON object in this exact format:
{
  "name": "Profile Name",
  "description": "Brief description of what this profile is for",
  "config": {
    "key": "value"
  },
  "rationale": ["Reason 1", "Reason 2"]
}

Available configuration keys and their types:
%s

Only include settings that are relevant to the user's request. Do not include settings with default values.
`, configDocs)

	userPrompt := req.Prompt
	if len(contextParts) > 0 {
		userPrompt = fmt.Sprintf("Context:\n%s\n\nRequest: %s", strings.Join(contextParts, "\n"), req.Prompt)
	}

	fullPrompt := fmt.Sprintf("%s\n\nUser request: %s\n\nRespond with ONLY the JSON object, no markdown, no explanation.", systemPrompt, userPrompt)

	// Call the AI service
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	response, err := h.aiHandler.GetService(ctx).Execute(ctx, chat.ExecuteRequest{
		Prompt: fullPrompt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get AI suggestion")
		http.Error(w, "Failed to generate suggestion", http.StatusInternalServerError)
		return
	}

	fullResponse, _ := response["content"].(string)
	if fullResponse == "" {
		log.Error().Msg("AI returned empty response")
		http.Error(w, "AI returned empty response", http.StatusInternalServerError)
		return
	}

	suggestion, err := parseAISuggestion(fullResponse)
	if err != nil {
		log.Error().Err(err).Str("response", fullResponse).Msg("Failed to parse AI suggestion")
		// Return a friendly error with partial info if available
		http.Error(w, fmt.Sprintf("Failed to parse AI response: %v", err), http.StatusInternalServerError)
		return
	}

	// Validate the suggested config
	validator := models.NewProfileValidator()
	if suggestion.Config != nil {
		configMap := models.AgentConfigMap{}
		for k, v := range suggestion.Config {
			configMap[k] = v
		}
		result := validator.Validate(configMap)
		if !result.Valid {
			// Include warnings in response but don't fail
			log.Warn().Interface("errors", result.Errors).Msg("Suggestion has validation warnings")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestion)
}

// parseAISuggestion extracts the ProfileSuggestion from the AI response
func parseAISuggestion(text string) (*ProfileSuggestion, error) {
	// Try to find JSON in the response
	text = strings.TrimSpace(text)

	// Remove ALL markdown code block markers
	text = strings.ReplaceAll(text, "```json", "")
	text = strings.ReplaceAll(text, "```", "")
	text = strings.TrimSpace(text)

	// Find JSON object boundaries - use brace counting to find the complete JSON
	start := strings.Index(text, "{")
	if start == -1 {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	// Count braces to find the matching closing brace
	braceCount := 0
	end := -1
	inString := false
	escape := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			braceCount++
		} else if c == '}' {
			braceCount--
			if braceCount == 0 {
				end = i
				break
			}
		}
	}

	if end == -1 {
		return nil, fmt.Errorf("no complete JSON object found in response")
	}

	jsonStr := text[start : end+1]

	var suggestion ProfileSuggestion
	if err := json.Unmarshal([]byte(jsonStr), &suggestion); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate required fields
	if suggestion.Name == "" {
		suggestion.Name = "Suggested Profile"
	}
	if suggestion.Description == "" {
		suggestion.Description = "AI-generated configuration profile"
	}
	if suggestion.Config == nil {
		suggestion.Config = make(map[string]interface{})
	}
	if suggestion.Rationale == nil {
		suggestion.Rationale = []string{}
	}

	return &suggestion, nil
}

// buildConfigSchemaDoc generates documentation for all config keys from the schema
func buildConfigSchemaDoc() string {
	defs := models.GetConfigKeyDefinitions()
	var lines []string

	for _, def := range defs {
		var typeStr string
		switch def.Type {
		case models.ConfigTypeBool:
			typeStr = "boolean"
		case models.ConfigTypeString:
			typeStr = "string"
		case models.ConfigTypeInt:
			typeStr = "integer"
			if def.Min != nil || def.Max != nil {
				constraints := []string{}
				if def.Min != nil {
					constraints = append(constraints, fmt.Sprintf("min: %.0f", *def.Min))
				}
				if def.Max != nil {
					constraints = append(constraints, fmt.Sprintf("max: %.0f", *def.Max))
				}
				typeStr += " (" + strings.Join(constraints, ", ") + ")"
			}
		case models.ConfigTypeFloat:
			typeStr = "number"
			if def.Min != nil || def.Max != nil {
				constraints := []string{}
				if def.Min != nil {
					constraints = append(constraints, fmt.Sprintf("min: %.1f", *def.Min))
				}
				if def.Max != nil {
					constraints = append(constraints, fmt.Sprintf("max: %.1f", *def.Max))
				}
				typeStr += " (" + strings.Join(constraints, ", ") + ")"
			}
		case models.ConfigTypeDuration:
			typeStr = "duration string (e.g., \"30s\", \"1m\", \"5m\")"
		case models.ConfigTypeEnum:
			typeStr = fmt.Sprintf("enum: %s", strings.Join(def.Enum, ", "))
		default:
			typeStr = string(def.Type)
		}

		line := fmt.Sprintf("- %s (%s): %s", def.Key, typeStr, def.Description)
		if def.Default != nil && def.Default != "" {
			line += fmt.Sprintf(" [default: %v]", def.Default)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
