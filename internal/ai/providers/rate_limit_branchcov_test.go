package providers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRateLimitInfo exercises every branch of rateLimitInfo: the nil-response
// guard, the empty-value-slice skip, the non-rate-limit-header filter (all four
// substring arms), the empty-joined-value skip, the no-entries short circuit,
// the sort, and the maxEntries truncation boundary.
func TestRateLimitInfo(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		want string
	}{
		{
			name: "nil response returns empty string",
			resp: nil,
			want: "",
		},
		{
			name: "empty header map returns empty string",
			resp: &http.Response{Header: http.Header{}},
			want: "",
		},
		{
			name: "non rate limit headers are skipped leaving no entries",
			resp: &http.Response{Header: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer secret"},
				"X-Request-Id":  []string{"abc"},
			}},
			want: "",
		},
		{
			name: "empty value slice is skipped but matching header survives",
			resp: &http.Response{Header: http.Header{
				"X-RateLimit-Stale":     []string{},
				"X-RateLimit-Remaining": []string{"100"},
			}},
			want: "rate_limit: x-ratelimit-remaining=100",
		},
		{
			name: "empty joined value is skipped but matching header survives",
			resp: &http.Response{Header: http.Header{
				"X-RateLimit-Empty":     []string{""},
				"X-RateLimit-Remaining": []string{"100"},
			}},
			want: "rate_limit: x-ratelimit-remaining=100",
		},
		{
			name: "all four substring filters match and entries are sorted",
			resp: &http.Response{Header: http.Header{
				"X-RateLimit-Limit":      []string{"60"},
				"X-Rate-Limit-Remaining": []string{"59"},
				"Retry-After":            []string{"30"},
				"X-Quota-Remaining":      []string{"1000"},
			}},
			want: "rate_limit: retry-after=30, x-quota-remaining=1000, x-rate-limit-remaining=59, x-ratelimit-limit=60",
		},
		{
			name: "multiple header values are joined with comma",
			resp: &http.Response{Header: http.Header{
				"X-RateLimit-Reset": []string{"1", "2", "3"},
			}},
			want: "rate_limit: x-ratelimit-reset=1,2,3",
		},
		{
			name: "exactly maxEntries headers are not truncated",
			resp: &http.Response{Header: http.Header{
				"X-RateLimit-A": []string{"1"},
				"X-RateLimit-B": []string{"2"},
				"X-RateLimit-C": []string{"3"},
				"X-RateLimit-D": []string{"4"},
				"X-RateLimit-E": []string{"5"},
				"X-RateLimit-F": []string{"6"},
			}},
			want: "rate_limit: x-ratelimit-a=1, x-ratelimit-b=2, x-ratelimit-c=3, x-ratelimit-d=4, x-ratelimit-e=5, x-ratelimit-f=6",
		},
		{
			name: "more than maxEntries headers are truncated to the sorted first six",
			resp: &http.Response{Header: http.Header{
				"X-RateLimit-A": []string{"1"},
				"X-RateLimit-B": []string{"2"},
				"X-RateLimit-C": []string{"3"},
				"X-RateLimit-D": []string{"4"},
				"X-RateLimit-E": []string{"5"},
				"X-RateLimit-F": []string{"6"},
				"X-RateLimit-G": []string{"7"},
				"X-RateLimit-H": []string{"8"},
			}},
			want: "rate_limit: x-ratelimit-a=1, x-ratelimit-b=2, x-ratelimit-c=3, x-ratelimit-d=4, x-ratelimit-e=5, x-ratelimit-f=6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rateLimitInfo(tt.resp)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRateLimitInfo_TruncationDropsOverflow ensures the entries dropped by the
// maxEntries cap are genuinely removed from the output (the sorted tail), not
// just visually absent.
func TestRateLimitInfo_TruncationDropsOverflow(t *testing.T) {
	resp := &http.Response{Header: http.Header{
		"X-RateLimit-A": []string{"1"},
		"X-RateLimit-B": []string{"2"},
		"X-RateLimit-C": []string{"3"},
		"X-RateLimit-D": []string{"4"},
		"X-RateLimit-E": []string{"5"},
		"X-RateLimit-F": []string{"6"},
		"X-RateLimit-G": []string{"7"},
		"X-RateLimit-H": []string{"8"},
	}}
	got := rateLimitInfo(resp)
	require.NotEmpty(t, got)
	assert.NotContains(t, got, "x-ratelimit-g=7")
	assert.NotContains(t, got, "x-ratelimit-h=8")
}

// TestAppendRateLimitInfo covers the passthrough (empty info returns message
// untouched) and the append path (info is wrapped and concatenated).
func TestAppendRateLimitInfo(t *testing.T) {
	tests := []struct {
		name    string
		message string
		resp    *http.Response
		want    string
	}{
		{
			name:    "nil response passes message through unchanged",
			message: "upstream error",
			resp:    nil,
			want:    "upstream error",
		},
		{
			name:    "response without rate limit headers passes message through",
			message: "upstream error",
			resp:    &http.Response{Header: http.Header{"Content-Type": []string{"application/json"}}},
			want:    "upstream error",
		},
		{
			name:    "rate limit info is appended to message",
			message: "429 too many requests",
			resp:    &http.Response{Header: http.Header{"Retry-After": []string{"30"}}},
			want:    "429 too many requests (rate_limit: retry-after=30)",
		},
		{
			name:    "empty message still receives appended info",
			message: "",
			resp:    &http.Response{Header: http.Header{"X-RateLimit-Remaining": []string{"0"}}},
			want:    " (rate_limit: x-ratelimit-remaining=0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendRateLimitInfo(tt.message, tt.resp)
			assert.Equal(t, tt.want, got)
		})
	}
}
