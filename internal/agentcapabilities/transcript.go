package agentcapabilities

import "encoding/json"

// TranscriptToolCall preserves a stored invocation and its observed result.
// Provider requests use ProviderToolCall instead of the product history shape.
type TranscriptToolCall struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Input            map[string]interface{} `json:"input"`
	Output           string                 `json:"output,omitempty"`
	Success          *bool                  `json:"success,omitempty"`
	ThoughtSignature json.RawMessage        `json:"thought_signature,omitempty"`
}

func (t TranscriptToolCall) NormalizeCollections() TranscriptToolCall {
	providerCall := ProviderToolCall{
		ID:               t.ID,
		Name:             t.Name,
		Input:            t.Input,
		ThoughtSignature: t.ThoughtSignature,
	}.NormalizeCollections()
	t.ID = providerCall.ID
	t.Name = providerCall.Name
	t.Input = providerCall.Input
	t.ThoughtSignature = providerCall.ThoughtSignature
	if t.Success != nil {
		success := *t.Success
		t.Success = &success
	}
	return t
}

// ProviderToolCall projects a stored Assistant transcript call back to the
// shared provider-facing shape, deliberately excluding in-app output/success
// display fields.
func (t TranscriptToolCall) ProviderToolCall() ProviderToolCall {
	t = t.NormalizeCollections()
	return ProviderToolCall{
		ID:               t.ID,
		Name:             t.Name,
		Input:            t.Input,
		ThoughtSignature: t.ThoughtSignature,
	}.NormalizeCollections()
}
