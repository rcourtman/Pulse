package email

import (
	"strings"
	"testing"
)

// TestRenderMagicLinkEmail covers RenderMagicLinkEmail in templates.go.
//
// The function's only error arm comes from magicLinkTemplate.Execute, which is
// unreachable at runtime: magicLinkTemplate is a package-level template.Must of
// a constant parse string, so Execute cannot fail for the single string field
// MagicLinkData.MagicLinkURL. That arm is therefore not fabricated here.
func TestRenderMagicLinkEmail(t *testing.T) {
	t.Run("populated URL substituted into html and text", func(t *testing.T) {
		const url = "https://pulse.example.com/auth/magic?token=abc123XYZ"

		htmlBody, textBody, err := RenderMagicLinkEmail(MagicLinkData{MagicLinkURL: url})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if htmlBody == "" {
			t.Fatal("expected non-empty html body")
		}
		if textBody == "" {
			t.Fatal("expected non-empty text body")
		}

		if !strings.Contains(htmlBody, url) {
			t.Errorf("html body does not contain magic link URL;\nwant substring: %s\ngot: %s", url, htmlBody)
		}
		if !strings.Contains(htmlBody, "Sign in to Pulse") {
			t.Errorf("html body does not contain expected heading text;\ngot: %s", htmlBody)
		}
		if !strings.Contains(htmlBody, `href="`+url+`"`) {
			t.Errorf("html body does not contain href with the URL;\ngot: %s", htmlBody)
		}

		if !strings.Contains(textBody, url) {
			t.Errorf("text body does not contain magic link URL;\nwant substring: %s\ngot: %s", url, textBody)
		}
		if !strings.Contains(textBody, "Sign in to Pulse") {
			t.Errorf("text body does not contain expected greeting;\ngot: %s", textBody)
		}
		if !strings.Contains(textBody, "expires in 15 minutes") {
			t.Errorf("text body does not contain expiry note;\ngot: %s", textBody)
		}
	})

	t.Run("html template escapes ampersands in URL, text body does not", func(t *testing.T) {
		const rawURL = "https://pulse.example.com/auth/magic?token=abc&uid=42"

		htmlBody, textBody, err := RenderMagicLinkEmail(MagicLinkData{MagicLinkURL: rawURL})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		const escapedURL = "https://pulse.example.com/auth/magic?token=abc&amp;uid=42"
		if !strings.Contains(htmlBody, escapedURL) {
			t.Errorf("html body should contain HTML-escaped URL %q;\ngot: %s", escapedURL, htmlBody)
		}
		if strings.Contains(htmlBody, `href="`+rawURL+`"`) {
			t.Errorf("html body must not contain the raw unescaped URL inside href;\ngot: %s", htmlBody)
		}

		if !strings.Contains(textBody, rawURL) {
			t.Errorf("text body should contain the raw URL %q (no HTML escaping);\ngot: %s", rawURL, textBody)
		}
		if strings.Contains(textBody, escapedURL) {
			t.Errorf("text body must not contain HTML-escaped ampersands;\ngot: %s", textBody)
		}
	})

	t.Run("empty URL still renders without error", func(t *testing.T) {
		htmlBody, textBody, err := RenderMagicLinkEmail(MagicLinkData{MagicLinkURL: ""})
		if err != nil {
			t.Fatalf("unexpected error for empty URL: %v", err)
		}
		if htmlBody == "" {
			t.Fatal("expected non-empty html body even with empty URL")
		}
		if textBody == "" {
			t.Fatal("expected non-empty text body even with empty URL")
		}
		if !strings.Contains(htmlBody, "Sign in to Pulse") {
			t.Errorf("html body missing heading for empty URL;\ngot: %s", htmlBody)
		}
		if !strings.Contains(textBody, "Sign in to Pulse") {
			t.Errorf("text body missing greeting for empty URL;\ngot: %s", textBody)
		}
	})
}
