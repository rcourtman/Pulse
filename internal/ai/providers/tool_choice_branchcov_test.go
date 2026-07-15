package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// toolChoiceAuto is a sentinel value that is neither ToolChoiceNone nor
// ToolChoiceRequired. It exercises the converters' default arm, which models
// the "let the model decide" / auto behavior Pulse leaves implicit.
const toolChoiceAuto ToolChoiceType = "auto"

// TestConvertToolChoiceToOpenAI_Branches exercises every branch of
// convertToolChoiceToOpenAI: the nil guard, the ToolChoiceNone and
// ToolChoiceRequired switch arms, and the default/auto fallthrough.
func TestConvertToolChoiceToOpenAI_Branches(t *testing.T) {
	tests := []struct {
		name string
		tc   *ToolChoice
		want interface{}
	}{
		{
			name: "nil tool choice omits the field",
			tc:   nil,
			want: nil,
		},
		{
			name: "none serializes to the OpenAI literal string",
			tc:   &ToolChoice{Type: ToolChoiceNone},
			want: "none",
		},
		{
			name: "required serializes to the OpenAI literal string",
			tc:   &ToolChoice{Type: ToolChoiceRequired},
			want: "required",
		},
		{
			name: "auto falls through to nil so the request omits tool_choice",
			tc:   &ToolChoice{Type: toolChoiceAuto},
			want: nil,
		},
		{
			name: "empty type value is treated as the default arm",
			tc:   &ToolChoice{Type: ""},
			want: nil,
		},
		{
			name: "unknown type value is treated as the default arm",
			tc:   &ToolChoice{Type: ToolChoiceType("bogus")},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToolChoiceToOpenAI(tt.tc)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConvertToolChoiceToAnthropic_Branches exercises every branch of
// convertToolChoiceToAnthropic: the nil guard, the none/required switch arms,
// and the default/auto fallthrough. It asserts the concrete struct shape that
// gets serialized into the request body (Type "none" and Type "any").
func TestConvertToolChoiceToAnthropic_Branches(t *testing.T) {
	tests := []struct {
		name string
		tc   *ToolChoice
		want *anthropicToolChoice
	}{
		{
			name: "nil tool choice returns nil",
			tc:   nil,
			want: nil,
		},
		{
			name: "none maps to Anthropic none mode",
			tc:   &ToolChoice{Type: ToolChoiceNone},
			want: &anthropicToolChoice{Type: "none"},
		},
		{
			name: "required maps to Anthropic any mode",
			tc:   &ToolChoice{Type: ToolChoiceRequired},
			want: &anthropicToolChoice{Type: "any"},
		},
		{
			name: "auto falls through to nil so tool_choice is omitted",
			tc:   &ToolChoice{Type: toolChoiceAuto},
			want: nil,
		},
		{
			name: "empty type value is treated as the default arm",
			tc:   &ToolChoice{Type: ""},
			want: nil,
		},
		{
			name: "unknown type value is treated as the default arm",
			tc:   &ToolChoice{Type: ToolChoiceType("bogus")},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToolChoiceToAnthropic(tt.tc)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			if assert.NotNil(t, got) {
				assert.Equal(t, tt.want.Type, got.Type)
			}
		})
	}
}

// TestConvertToolChoiceToGemini_Branches exercises every branch of
// convertToolChoiceToGemini: the nil guard, the NONE/ANY switch arms, and the
// default/auto fallthrough which returns an empty mode string.
func TestConvertToolChoiceToGemini_Branches(t *testing.T) {
	tests := []struct {
		name string
		tc   *ToolChoice
		want string
	}{
		{
			name: "nil tool choice returns empty mode",
			tc:   nil,
			want: "",
		},
		{
			name: "none maps to Gemini NONE mode",
			tc:   &ToolChoice{Type: ToolChoiceNone},
			want: "NONE",
		},
		{
			name: "required maps to Gemini ANY mode",
			tc:   &ToolChoice{Type: ToolChoiceRequired},
			want: "ANY",
		},
		{
			name: "auto falls through to empty mode so the config is omitted",
			tc:   &ToolChoice{Type: toolChoiceAuto},
			want: "",
		},
		{
			name: "empty type value is treated as the default arm",
			tc:   &ToolChoice{Type: ""},
			want: "",
		},
		{
			name: "unknown type value is treated as the default arm",
			tc:   &ToolChoice{Type: ToolChoiceType("bogus")},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToolChoiceToGemini(tt.tc)
			assert.Equal(t, tt.want, got)
		})
	}
}
