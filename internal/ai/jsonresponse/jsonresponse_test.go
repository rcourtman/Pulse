package jsonresponse

import "testing"

func TestExtractObject(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
		wantOK   bool
	}{
		{
			name:     "plain object",
			response: `{"service_type":"nginx"}`,
			want:     `{"service_type":"nginx"}`,
			wantOK:   true,
		},
		{
			name:     "markdown block",
			response: "```json\n{\"service_type\":\"redis\"}\n```",
			want:     `{"service_type":"redis"}`,
			wantOK:   true,
		},
		{
			name:     "inline markdown fence with language tag",
			response: "```json {\"service_type\":\"debian\",\"facts\":[{\"key\":\"OS Codename\",\"value\":\"trixie\"}]} ```",
			want:     `{"service_type":"debian","facts":[{"key":"OS Codename","value":"trixie"}]}`,
			wantOK:   true,
		},
		{
			name:     "single backtick and language tag",
			response: "`json {\"service_type\":\"debian\"}`",
			want:     `{"service_type":"debian"}`,
			wantOK:   true,
		},
		{
			name:     "surrounding prose with braces before object",
			response: `Analysis {not json}: {"service_type":"postgres","reasoning":"brace { in string } stays valid"} done.`,
			want:     `{"service_type":"postgres","reasoning":"brace { in string } stays valid"}`,
			wantOK:   true,
		},
		{
			name:     "invalid",
			response: "not json at all",
			wantOK:   false,
		},
		{
			name:     "incomplete object",
			response: `json {"service_type":"nginx"`,
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractObject(tt.response)
			if ok != tt.wantOK {
				t.Fatalf("ExtractObject() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("ExtractObject() = %q, want %q", got, tt.want)
			}
		})
	}
}
