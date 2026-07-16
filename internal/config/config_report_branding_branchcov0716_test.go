package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This file adds branch coverage for the report-branding helpers in
// report_branding.go: DecodeReportBrandLogoBase64 and
// CanonicalReportBrandLogoFormat. Test functions use the BranchCov prefix so
// they can be selected with `-run BranchCov`.

// TestBranchCovDecodeReportBrandLogoBase64 exercises every branch of
// DecodeReportBrandLogoBase64:
//   - empty / whitespace-only input (returns nil, nil)
//   - input without a ";base64," data-URL separator (used verbatim)
//   - input with a ";base64," separator (prefix stripped)
//   - leading and repeated ";base64," separators
//   - the base64.StdEncoding success path
//   - the base64.StdEncoding failure / RawStdEncoding fallback success path
//   - both encodings failing (final error return)
//   - the length boundary at exactly ReportBrandLogoBase64MaxLength and one
//     character over it
func TestBranchCovDecodeReportBrandLogoBase64(t *testing.T) {
	// "AAAAAA" (6 chars, no padding) decodes cleanly under RawStdEncoding but
	// is rejected by StdEncoding (which expects padding to a multiple of 4),
	// so it forces the fallback path. "AAA" (3 chars) does the same.
	const rawFallbackInput = "AAAAAA"
	// "QUJD" is "ABC" in standard base64 (4 chars, multiple of 4) and succeeds
	// under StdEncoding on the first try.
	const stdSuccessInput = "QUJD"

	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
		errSub  string
	}{
		{
			name:  "empty input returns nil nil",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only trims to empty returns nil nil",
			input: "   \t\n ",
			want:  nil,
		},
		{
			name:  "plain std base64 no separator std success path",
			input: stdSuccessInput,
			want:  []byte("ABC"),
		},
		{
			name:  "plain raw base64 no separator raw fallback path",
			input: rawFallbackInput,
			want:  []byte{0, 0, 0, 0},
		},
		{
			name:    "non-base64 garbage both encodings fail",
			input:   "!!!!not-base64!!!!",
			wantErr: true,
			errSub:  "logoBase64 must be valid base64",
		},
		{
			name:    "five chars not multiple of four both fail",
			input:   "QUJDA",
			wantErr: true,
			errSub:  "logoBase64 must be valid base64",
		},
		{
			name:  "data url separator strips prefix and decodes",
			input: "data:image/png;base64," + stdSuccessInput,
			want:  []byte("ABC"),
		},
		{
			name:  "data url separator with raw payload uses fallback",
			input: "data:image/png;base64," + rawFallbackInput,
			want:  []byte{0, 0, 0, 0},
		},
		{
			name:  "leading base64 separator at index zero",
			input: ";base64," + stdSuccessInput,
			want:  []byte("ABC"),
		},
		{
			name:    "multiple separators only first is stripped second leaks into payload",
			input:   ";base64," + stdSuccessInput + ";base64,BBBB",
			wantErr: true,
			errSub:  "logoBase64 must be valid base64",
		},
		{
			name:  "data url with empty payload decodes to empty non-nil slice",
			input: "data:image/png;base64,",
			want:  []byte{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeReportBrandLogoBase64(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errSub != "" {
					assert.Contains(t, err.Error(), tc.errSub)
				}
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	// Boundary: exactly at the configured max length. Build a valid standard
	// base64 value whose length equals ReportBrandLogoBase64MaxLength (which is
	// a multiple of 4) so StdEncoding succeeds and the length check passes.
	t.Run("length exactly at limit succeeds", func(t *testing.T) {
		assert.Equal(t, 0, ReportBrandLogoBase64MaxLength%4,
			"max length must be a multiple of 4 for this boundary case to be well-defined")
		value := strings.Repeat("A", ReportBrandLogoBase64MaxLength)
		dec, err := DecodeReportBrandLogoBase64(value)
		assert.NoError(t, err)
		assert.Len(t, dec, ReportBrandLogoBase64MaxLength/4*3)
	})

	// Boundary: one character over the limit must be rejected by the length
	// check before any decoding is attempted, regardless of payload validity.
	t.Run("length one over limit rejected before decode", func(t *testing.T) {
		value := strings.Repeat("A", ReportBrandLogoBase64MaxLength+1)
		dec, err := DecodeReportBrandLogoBase64(value)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logoBase64 must be <= ")
		assert.Contains(t, err.Error(), "49152 characters")
		assert.Nil(t, dec)
	})

	// Boundary: length check is applied to the post-strip value, so a data URL
	// whose base64 payload alone exceeds the limit must still be rejected.
	t.Run("oversized payload after data url strip rejected", func(t *testing.T) {
		payload := strings.Repeat("A", ReportBrandLogoBase64MaxLength+1)
		dec, err := DecodeReportBrandLogoBase64("data:image/png;base64," + payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logoBase64 must be <= 49152 characters")
		assert.Nil(t, dec)
	})
}

// TestBranchCovCanonicalReportBrandLogoFormat covers every arm of the switch in
// CanonicalReportBrandLogoFormat (including the default rejection branch) and
// the leading/trailing whitespace plus case-normalisation performed before the
// switch is entered.
func TestBranchCovCanonicalReportBrandLogoFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOk bool
	}{
		{name: "empty string accepted as empty canonical", input: "", want: "", wantOk: true},
		{name: "whitespace only trims to empty accepted", input: "   \t ", want: "", wantOk: true},

		{name: "png canonical", input: "png", want: "png", wantOk: true},
		{name: "png uppercased normalised", input: "PNG", want: "png", wantOk: true},
		{name: "png mixed case trimmed", input: "  PnG  ", want: "png", wantOk: true},

		{name: "jpg canonical", input: "jpg", want: "jpg", wantOk: true},
		{name: "jpeg alias collapses to jpg", input: "jpeg", want: "jpg", wantOk: true},
		{name: "JPEG uppercased collapses to jpg", input: " JPEG ", want: "jpg", wantOk: true},

		{name: "gif canonical", input: "gif", want: "gif", wantOk: true},
		{name: "GIF uppercased normalised", input: "GIF", want: "gif", wantOk: true},

		{name: "unknown format webp rejected", input: "webp", want: "", wantOk: false},
		{name: "unknown format svg rejected", input: "svg", want: "", wantOk: false},
		{name: "png with trailing digit rejected by default", input: "png2", want: "", wantOk: false},
		{name: "mime style value rejected by default", input: "image/png", want: "", wantOk: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := CanonicalReportBrandLogoFormat(tc.input)
			assert.Equal(t, tc.wantOk, ok, "ok mismatch for input %q", tc.input)
			assert.Equal(t, tc.want, got, "canonical value mismatch for input %q", tc.input)
		})
	}
}
