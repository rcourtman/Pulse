package notifications

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// branchcovFailWriter is an io.Writer that allows the first `ok` Write calls to
// succeed and then fails every subsequent Write. It lets us exercise the error
// arms of writeMultipartBodyPart that are unreachable when the underlying writer
// is a *bytes.Buffer (whose Write never errors).
type branchcovFailWriter struct {
	ok      int
	written int
}

func (w *branchcovFailWriter) Write(p []byte) (int, error) {
	w.written++
	if w.written > w.ok {
		return 0, errors.New("branchcov: write blocked")
	}
	return len(p), nil
}

func branchcovBasicAddresses() resolvedEmailAddresses {
	return resolvedEmailAddresses{
		from: &mail.Address{Address: "sender@example.com"},
		to:   []*mail.Address{{Address: "recipient@example.com"}},
	}
}

func branchcovAddressesWithReplyTo() resolvedEmailAddresses {
	return resolvedEmailAddresses{
		from:    &mail.Address{Address: "sender@example.com"},
		to:      []*mail.Address{{Address: "recipient@example.com"}},
		replyTo: &mail.Address{Address: "replies@example.com"},
	}
}

// --- writeMultipartBodyPart (email_enhanced.go:171) ---

func TestBranchcov0724pmWriteMultipartBodyPart(t *testing.T) {
	t.Run("CreatePartError", func(t *testing.T) {
		// A writer that fails on the very first Write so multipart.Writer.CreatePart
		// cannot even emit the part boundary/headers.
		fw := &branchcovFailWriter{ok: 0}
		mw := multipart.NewWriter(fw)

		err := writeMultipartBodyPart(mw, "text/plain", "body")
		if err == nil {
			t.Fatal("expected create-part error, got nil")
		}
		if !strings.Contains(err.Error(), "create text/plain part") {
			t.Fatalf("error should mention create text/plain part, got %v", err)
		}
		if !strings.Contains(err.Error(), "write blocked") {
			t.Fatalf("error should wrap underlying write failure, got %v", err)
		}
	})

	t.Run("EncodeError", func(t *testing.T) {
		// Allow exactly one Write (the CreatePart header block) to succeed, then
		// fail. A body longer than one quoted-printable line (76 chars) forces a
		// flush during encoder.Write, hitting the encode-error arm.
		fw := &branchcovFailWriter{ok: 1}
		mw := multipart.NewWriter(fw)
		largeBody := strings.Repeat("x", 200)

		err := writeMultipartBodyPart(mw, "text/html", largeBody)
		if err == nil {
			t.Fatal("expected encode error, got nil")
		}
		if !strings.Contains(err.Error(), "encode text/html part") {
			t.Fatalf("error should mention encode text/html part, got %v", err)
		}
	})

	t.Run("FinalizeError", func(t *testing.T) {
		// Allow exactly one Write (CreatePart headers) to succeed. A short body
		// (2 chars) stays buffered inside the quoted-printable encoder during
		// Write; the buffer is only flushed on Close, hitting the finalize arm.
		fw := &branchcovFailWriter{ok: 1}
		mw := multipart.NewWriter(fw)

		err := writeMultipartBodyPart(mw, "text/plain", "hi")
		if err == nil {
			t.Fatal("expected finalize error, got nil")
		}
		if !strings.Contains(err.Error(), "finalize text/plain part") {
			t.Fatalf("error should mention finalize text/plain part, got %v", err)
		}
	})

	t.Run("SuccessSetsHeadersAndEncodesBody", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)

		if err := writeMultipartBodyPart(mw, "text/plain", "hello world"); err != nil {
			t.Fatalf("writeMultipartBodyPart() error = %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}

		reader := multipart.NewReader(&buf, mw.Boundary())
		part, err := reader.NextRawPart()
		if err != nil {
			t.Fatalf("NextRawPart() error = %v", err)
		}
		ct := part.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/plain") || !strings.Contains(ct, "charset=UTF-8") {
			t.Errorf("Content-Type = %q, want text/plain with charset=UTF-8", ct)
		}
		if cte := part.Header.Get("Content-Transfer-Encoding"); cte != "quoted-printable" {
			t.Errorf("Content-Transfer-Encoding = %q, want quoted-printable", cte)
		}
		decoded, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatalf("ReadAll(quotedprintable) error = %v", err)
		}
		if string(decoded) != normalizeEmailBodyLineEndings("hello world") {
			t.Errorf("decoded body = %q, want %q", decoded, normalizeEmailBodyLineEndings("hello world"))
		}
	})
}

// --- writeEmailThreadingHeaders (email_enhanced.go:204) ---
//
// The two fmt.Fprintf error arms (lines 210 and 213) are provably unreachable:
// the function signature accepts *bytes.Buffer, and bytes.Buffer.Write /
// WriteString never return an error. These subtests cover every reachable arm.

func TestBranchcov0724pmWriteEmailThreadingHeaders(t *testing.T) {
	t.Run("EmptyThreadIDWritesNothing", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeEmailThreadingHeaders(&buf, ""); err != nil {
			t.Fatalf("writeEmailThreadingHeaders() error = %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("buffer should be empty for blank threadID, got %q", buf.String())
		}
	})

	t.Run("WhitespaceOnlyThreadIDSanitizedToEmpty", func(t *testing.T) {
		var buf bytes.Buffer
		// CR/LF/spaces are all sanitized away, leaving an empty threadID.
		if err := writeEmailThreadingHeaders(&buf, "  \r\n  "); err != nil {
			t.Fatalf("writeEmailThreadingHeaders() error = %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("buffer should be empty for whitespace-only threadID, got %q", buf.String())
		}
	})

	t.Run("ValidThreadIDWritesBothHeaders", func(t *testing.T) {
		var buf bytes.Buffer
		threadID := "<abc@example.com>"
		if err := writeEmailThreadingHeaders(&buf, threadID); err != nil {
			t.Fatalf("writeEmailThreadingHeaders() error = %v", err)
		}
		raw := buf.String()
		if !strings.Contains(raw, "In-Reply-To: "+threadID+"\r\n") {
			t.Errorf("missing In-Reply-To header in:\n%s", raw)
		}
		if !strings.Contains(raw, "References: "+threadID+"\r\n") {
			t.Errorf("missing References header in:\n%s", raw)
		}
	})

	t.Run("ThreadIDCRLFSanitized", func(t *testing.T) {
		var buf bytes.Buffer
		// Embedded CRLF must be collapsed to spaces so it cannot inject headers.
		if err := writeEmailThreadingHeaders(&buf, "<id\r\nBcc: evil@test.com>"); err != nil {
			t.Fatalf("writeEmailThreadingHeaders() error = %v", err)
		}
		raw := buf.String()
		if strings.Contains(raw, "\r\nBcc:") {
			t.Fatalf("CRLF injection produced a Bcc header:\n%s", raw)
		}
		if !strings.Contains(raw, "<id  Bcc: evil@test.com>") {
			t.Fatalf("threadID CR/LF should be replaced with spaces:\n%s", raw)
		}
	})
}

// --- buildMultipartEmailMessage (email_enhanced.go:218) ---
//
// Every uncovered statement in this function is a fmt.Fprintf / WriteString
// error-return arm. The function allocates its own bytes.Buffer locally and
// writes all headers to it; bytes.Buffer never errors, so these arms are
// provably unreachable. The subtests below assert observable MIME behaviour
// for the scenarios requested (empty alternatives, header escaping, threading).

func TestBranchcov0724pmBuildMultipartEmailMessage(t *testing.T) {
	now := time.Unix(1711711711, 1234).UTC()

	t.Run("EmptyTextBody", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		msg, err := buildMultipartEmailMessage(addr, "Subject", "<p>HTML</p>", "", "", now)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessage() error = %v", err)
		}
		parsed, err := mail.ReadMessage(bytes.NewReader(msg))
		if err != nil {
			t.Fatalf("mail.ReadMessage() error = %v", err)
		}
		mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType() error = %v", err)
		}
		if mediaType != "multipart/alternative" {
			t.Fatalf("Content-Type = %q, want multipart/alternative", mediaType)
		}
		reader := multipart.NewReader(parsed.Body, params["boundary"])

		textPart, err := reader.NextRawPart()
		if err != nil {
			t.Fatalf("text NextRawPart() error = %v", err)
		}
		decoded, err := io.ReadAll(quotedprintable.NewReader(textPart))
		if err != nil {
			t.Fatalf("ReadAll(text) error = %v", err)
		}
		if string(decoded) != "" {
			t.Errorf("empty text body should decode to empty string, got %q", decoded)
		}

		htmlPart, err := reader.NextRawPart()
		if err != nil {
			t.Fatalf("html NextRawPart() error = %v", err)
		}
		decodedHTML, err := io.ReadAll(quotedprintable.NewReader(htmlPart))
		if err != nil {
			t.Fatalf("ReadAll(html) error = %v", err)
		}
		if string(decodedHTML) != normalizeEmailBodyLineEndings("<p>HTML</p>") {
			t.Errorf("html body = %q, want %q", decodedHTML, normalizeEmailBodyLineEndings("<p>HTML</p>"))
		}
	})

	t.Run("EmptyHTMLBody", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		msg, err := buildMultipartEmailMessage(addr, "Subject", "", "plain text body", "", now)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessage() error = %v", err)
		}
		parsed, err := mail.ReadMessage(bytes.NewReader(msg))
		if err != nil {
			t.Fatalf("mail.ReadMessage() error = %v", err)
		}
		_, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType() error = %v", err)
		}
		reader := multipart.NewReader(parsed.Body, params["boundary"])

		textPart, err := reader.NextRawPart()
		if err != nil {
			t.Fatalf("text NextRawPart() error = %v", err)
		}
		decodedText, err := io.ReadAll(quotedprintable.NewReader(textPart))
		if err != nil {
			t.Fatalf("ReadAll(text) error = %v", err)
		}
		if string(decodedText) != normalizeEmailBodyLineEndings("plain text body") {
			t.Errorf("text body = %q, want %q", decodedText, normalizeEmailBodyLineEndings("plain text body"))
		}

		htmlPart, err := reader.NextRawPart()
		if err != nil {
			t.Fatalf("html NextRawPart() error = %v", err)
		}
		decodedHTML, err := io.ReadAll(quotedprintable.NewReader(htmlPart))
		if err != nil {
			t.Fatalf("ReadAll(html) error = %v", err)
		}
		if string(decodedHTML) != "" {
			t.Errorf("empty html body should decode to empty string, got %q", decodedHTML)
		}
	})

	t.Run("SubjectCRLFHeaderInjectionSanitized", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		msg, err := buildMultipartEmailMessage(addr, "Safe\r\nBcc: evil@example.com", "<p>hi</p>", "hi", "", now)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessage() error = %v", err)
		}
		raw := string(msg)
		if strings.Contains(raw, "\r\nBcc: ") {
			t.Fatalf("CRLF injection in subject produced a Bcc header:\n%s", raw)
		}
		parsed, err := mail.ReadMessage(bytes.NewReader(msg))
		if err != nil {
			t.Fatalf("mail.ReadMessage() error = %v", err)
		}
		wantSubject := "Safe  Bcc: evil@example.com"
		if got := parsed.Header.Get("Subject"); got != wantSubject {
			t.Errorf("Subject = %q, want %q (CR/LF replaced with spaces)", got, wantSubject)
		}
	})

	t.Run("MissingThreadingReferencesProduceNoHeaders", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		msg, err := buildMultipartEmailMessage(addr, "Alert", "<p>hi</p>", "hi", "", now)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessage() error = %v", err)
		}
		parsed, err := mail.ReadMessage(bytes.NewReader(msg))
		if err != nil {
			t.Fatalf("mail.ReadMessage() error = %v", err)
		}
		if got := parsed.Header.Get("In-Reply-To"); got != "" {
			t.Errorf("In-Reply-To should be absent for empty threadID, got %q", got)
		}
		if got := parsed.Header.Get("References"); got != "" {
			t.Errorf("References should be absent for empty threadID, got %q", got)
		}
		if got := parsed.Header.Get("Message-ID"); got == "" {
			t.Error("Message-ID should always be present")
		}
	})

	t.Run("NoReplyToOmitsHeader", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		msg, err := buildMultipartEmailMessage(addr, "Subject", "<p>hi</p>", "hi", "", now)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessage() error = %v", err)
		}
		parsed, err := mail.ReadMessage(bytes.NewReader(msg))
		if err != nil {
			t.Fatalf("mail.ReadMessage() error = %v", err)
		}
		if got := parsed.Header.Get("Reply-To"); got != "" {
			t.Errorf("Reply-To should be absent when replyTo is nil, got %q", got)
		}
	})
}

// --- buildMultipartEmailMessageWithAttachments (email_enhanced.go:274) ---

func TestBranchcov0724pmBuildMultipartEmailMessageWithAttachments(t *testing.T) {
	now := time.Unix(1711711711, 1234).UTC()

	t.Run("ZeroAttachmentsDelegatesToAlternative", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		msg, err := buildMultipartEmailMessageWithAttachments(addr, "Subject", "<p>hi</p>", "hi", nil, "", now)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		raw := string(msg)
		if !strings.Contains(raw, "multipart/alternative") {
			t.Errorf("zero-attachment message should use multipart/alternative:\n%s", raw)
		}
		if strings.Contains(raw, "multipart/mixed") {
			t.Errorf("zero-attachment message should not use multipart/mixed:\n%s", raw)
		}
		if strings.Contains(raw, "Content-Disposition: attachment") {
			t.Errorf("zero-attachment message should not contain attachment disposition:\n%s", raw)
		}
	})

	t.Run("EmptyAttachmentDataSkipped", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		empty := EmailAttachment{Filename: "empty.bin", ContentType: "application/octet-stream", Data: nil}
		real := EmailAttachment{Filename: "real.txt", ContentType: "text/plain", Data: []byte("payload")}
		msg, err := buildMultipartEmailMessageWithAttachments(
			addr, "Subject", "<p>hi</p>", "hi",
			[]EmailAttachment{empty, real}, "", now,
		)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		raw := string(msg)
		if strings.Contains(raw, `filename="empty.bin"`) {
			t.Errorf("attachment with empty data should be skipped:\n%s", raw)
		}
		if !strings.Contains(raw, `filename="real.txt"`) {
			t.Errorf("real attachment should be present:\n%s", raw)
		}
		// Exactly one Content-Disposition header (for the real attachment only).
		if c := strings.Count(raw, "Content-Disposition: attachment"); c != 1 {
			t.Errorf("expected 1 attachment disposition, got %d:\n%s", c, raw)
		}
	})

	t.Run("EmptyFilenameDefaultsToAttachment", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		att := EmailAttachment{Filename: "", ContentType: "text/plain", Data: []byte("data")}
		msg, err := buildMultipartEmailMessageWithAttachments(
			addr, "Subject", "<p>hi</p>", "hi",
			[]EmailAttachment{att}, "", now,
		)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		raw := string(msg)
		if !strings.Contains(raw, `filename="attachment"`) {
			t.Errorf("empty filename should default to \"attachment\":\n%s", raw)
		}
		if !strings.Contains(raw, `name="attachment"`) {
			t.Errorf("Content-Type name should also default to \"attachment\":\n%s", raw)
		}
	})

	t.Run("WhitespaceFilenameDefaultsToAttachment", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		att := EmailAttachment{Filename: "   \r\n  ", ContentType: "text/plain", Data: []byte("data")}
		msg, err := buildMultipartEmailMessageWithAttachments(
			addr, "Subject", "<p>hi</p>", "hi",
			[]EmailAttachment{att}, "", now,
		)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		raw := string(msg)
		if !strings.Contains(raw, `filename="attachment"`) {
			t.Errorf("whitespace-only filename should default to \"attachment\":\n%s", raw)
		}
	})

	t.Run("EmptyContentTypeDefaultsToOctetStream", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		att := EmailAttachment{Filename: "mystery.bin", ContentType: "", Data: []byte("data")}
		msg, err := buildMultipartEmailMessageWithAttachments(
			addr, "Subject", "<p>hi</p>", "hi",
			[]EmailAttachment{att}, "", now,
		)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		raw := string(msg)
		if !strings.Contains(raw, "application/octet-stream") {
			t.Errorf("empty content type should default to application/octet-stream:\n%s", raw)
		}
	})

	t.Run("ReplyToHeaderPresent", func(t *testing.T) {
		addr := branchcovAddressesWithReplyTo()
		att := EmailAttachment{Filename: "file.txt", ContentType: "text/plain", Data: []byte("data")}
		msg, err := buildMultipartEmailMessageWithAttachments(
			addr, "Subject", "<p>hi</p>", "hi",
			[]EmailAttachment{att}, "", now,
		)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		parsed, err := mail.ReadMessage(bytes.NewReader(msg))
		if err != nil {
			t.Fatalf("mail.ReadMessage() error = %v", err)
		}
		if got := parsed.Header.Get("Reply-To"); got != addr.replyTo.String() {
			t.Errorf("Reply-To = %q, want %q", got, addr.replyTo.String())
		}
	})

	t.Run("FilenameCRLFSanitized", func(t *testing.T) {
		addr := branchcovBasicAddresses()
		att := EmailAttachment{
			Filename:    "report\r\nBcc: evil.pdf",
			ContentType: "application/pdf",
			Data:        []byte("%PDF-1.7"),
		}
		msg, err := buildMultipartEmailMessageWithAttachments(
			addr, "Subject", "<p>hi</p>", "hi",
			[]EmailAttachment{att}, "", now,
		)
		if err != nil {
			t.Fatalf("buildMultipartEmailMessageWithAttachments() error = %v", err)
		}
		raw := string(msg)
		if strings.Contains(raw, "\r\nBcc:") {
			t.Fatalf("CRLF in filename injected a Bcc header:\n%s", raw)
		}
		if !strings.Contains(raw, `filename="report  Bcc: evil.pdf"`) {
			t.Errorf("filename CR/LF should be replaced with spaces:\n%s", raw)
		}
	})
}

// --- alertNodeDisplay (email_template.go:33) ---

func TestBranchcov0724pmAlertNodeDisplay(t *testing.T) {
	t.Run("DisplayNameTakesPrecedence", func(t *testing.T) {
		alert := &alerts.Alert{Node: "node-raw", NodeDisplayName: "Pretty Name"}
		got := alertNodeDisplay(alert)
		if got != "Pretty Name" {
			t.Errorf("alertNodeDisplay() = %q, want %q (display name should win)", got, "Pretty Name")
		}
	})

	t.Run("FallsBackToNodeWhenDisplayNameEmpty", func(t *testing.T) {
		alert := &alerts.Alert{Node: "node-raw", NodeDisplayName: ""}
		got := alertNodeDisplay(alert)
		if got != "node-raw" {
			t.Errorf("alertNodeDisplay() = %q, want %q (should fall back to Node)", got, "node-raw")
		}
	})

	t.Run("BothBlankReturnsEmpty", func(t *testing.T) {
		alert := &alerts.Alert{Node: "", NodeDisplayName: ""}
		got := alertNodeDisplay(alert)
		if got != "" {
			t.Errorf("alertNodeDisplay() = %q, want empty string", got)
		}
	})
}

// --- copyWebhookConfig (notifications.go:324) ---
//
// The `len(clones) == 0` early-return arm is provably unreachable:
// copyWebhookConfig always passes a one-element slice to copyWebhookConfigs,
// which always returns a one-element slice for non-empty input.

func TestBranchcov0724pmCopyWebhookConfig(t *testing.T) {
	t.Run("HeadersAndCustomFieldsIndependent", func(t *testing.T) {
		src := WebhookConfig{
			ID:            "wh-1",
			Name:          "original",
			URL:           "https://example.com/hook",
			Method:        "POST",
			Headers:       map[string]string{"Authorization": "Bearer token", "X-Custom": "val"},
			CustomFields:  map[string]string{"channel": "general"},
			SigningSecret: "secret",
		}
		clone := copyWebhookConfig(src)

		// Mutate the clone in every map-bearing field.
		clone.Name = "modified"
		clone.Headers["Authorization"] = "Bearer changed"
		delete(clone.Headers, "X-Custom")
		clone.Headers["New-Key"] = "added"
		clone.CustomFields["channel"] = "changed"
		clone.CustomFields["new-key"] = "added"
		clone.SigningSecret = "leaked"

		// Assert the original is completely unchanged.
		if src.Name != "original" {
			t.Errorf("src.Name = %q, want %q", src.Name, "original")
		}
		if src.Headers["Authorization"] != "Bearer token" {
			t.Errorf("src.Headers[Authorization] = %q, want %q", src.Headers["Authorization"], "Bearer token")
		}
		if _, ok := src.Headers["X-Custom"]; !ok {
			t.Error("src.Headers[X-Custom] was deleted through the clone (shared map)")
		}
		if _, ok := src.Headers["New-Key"]; ok {
			t.Error("src.Headers[New-Key] was added through the clone (shared map)")
		}
		if src.CustomFields["channel"] != "general" {
			t.Errorf("src.CustomFields[channel] = %q, want %q", src.CustomFields["channel"], "general")
		}
		if _, ok := src.CustomFields["new-key"]; ok {
			t.Error("src.CustomFields[new-key] was added through the clone (shared map)")
		}
		if src.SigningSecret != "secret" {
			t.Errorf("src.SigningSecret = %q, want %q", src.SigningSecret, "secret")
		}

		// Assert the clone carries the modified values.
		if clone.Name != "modified" {
			t.Errorf("clone.Name = %q, want %q", clone.Name, "modified")
		}
		if clone.Headers["Authorization"] != "Bearer changed" {
			t.Errorf("clone.Headers[Authorization] = %q, want %q", clone.Headers["Authorization"], "Bearer changed")
		}
		if clone.CustomFields["channel"] != "changed" {
			t.Errorf("clone.CustomFields[channel] = %q, want %q", clone.CustomFields["channel"], "changed")
		}
	})

	t.Run("AllScalarFieldsPreserved", func(t *testing.T) {
		src := WebhookConfig{
			ID:            "wh-2",
			Name:          "test",
			URL:           "https://example.com",
			Method:        "PUT",
			Enabled:       true,
			Service:       "slack",
			Template:      "tmpl",
			Mention:       "@here",
			SigningSecret: "abc123",
			Headers:       map[string]string{"X-Test": "1"},
			CustomFields:  map[string]string{"key": "val"},
		}
		clone := copyWebhookConfig(src)

		if clone.ID != src.ID {
			t.Errorf("ID = %q, want %q", clone.ID, src.ID)
		}
		if clone.Name != src.Name {
			t.Errorf("Name = %q, want %q", clone.Name, src.Name)
		}
		if clone.URL != src.URL {
			t.Errorf("URL = %q, want %q", clone.URL, src.URL)
		}
		if clone.Method != src.Method {
			t.Errorf("Method = %q, want %q", clone.Method, src.Method)
		}
		if clone.Enabled != src.Enabled {
			t.Errorf("Enabled = %v, want %v", clone.Enabled, src.Enabled)
		}
		if clone.Service != src.Service {
			t.Errorf("Service = %q, want %q", clone.Service, src.Service)
		}
		if clone.Template != src.Template {
			t.Errorf("Template = %q, want %q", clone.Template, src.Template)
		}
		if clone.Mention != src.Mention {
			t.Errorf("Mention = %q, want %q", clone.Mention, src.Mention)
		}
		if clone.SigningSecret != src.SigningSecret {
			t.Errorf("SigningSecret = %q, want %q", clone.SigningSecret, src.SigningSecret)
		}
		if clone.Headers["X-Test"] != "1" {
			t.Errorf("Headers[X-Test] = %q, want %q", clone.Headers["X-Test"], "1")
		}
		if clone.CustomFields["key"] != "val" {
			t.Errorf("CustomFields[key] = %q, want %q", clone.CustomFields["key"], "val")
		}
	})
}
