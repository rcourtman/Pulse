package notifications

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"

	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

// Independently encode first and insert CRLF only between 76-character lines.
// This oracle does not use the production writer or its chunking algorithm.
func expectedAttachmentEncoding(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var lines []string
	for len(encoded) > 76 {
		lines = append(lines, encoded[:76])
		encoded = encoded[76:]
	}
	if encoded != "" {
		lines = append(lines, encoded)
	}
	return strings.Join(lines, "\r\n")
}

func TestBase64LineWriterStreamingParity(t *testing.T) {
	for _, size := range []int{0, 1, 2, 3, 55, 56, 57, 58, 113, 114, 115, 1023, 1024, 1025, 8193} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i*37 + 11)
		}
		for _, chunk := range []int{1, 2, 3, 56, 57, 58, 76, 77, 1024, 8193} {
			t.Run(fmt.Sprintf("size=%d/chunk=%d", size, chunk), func(t *testing.T) {
				var output bytes.Buffer
				encoder := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(&output))
				for rest := data; len(rest) > 0; {
					length := min(chunk, len(rest))
					n, err := encoder.Write(rest[:length])
					if err != nil || n != length {
						t.Fatalf("Write = %d, %v; want %d, nil", n, err, length)
					}
					rest = rest[length:]
				}
				if err := encoder.Close(); err != nil {
					t.Fatal(err)
				}
				if want := expectedAttachmentEncoding(data); output.String() != want {
					t.Fatalf("encoded attachment differs at size=%d, chunk=%d", size, chunk)
				}
			})
		}
	}
}

func TestBase64LineWriterDefersLineBreakUntilNextByte(t *testing.T) {
	var output bytes.Buffer
	writer := newBase64LineWriter(&output)
	first := bytes.Repeat([]byte{'A'}, 76)
	if n, err := writer.Write(first); err != nil || n != 76 {
		t.Fatalf("first line = %d, %v", n, err)
	}
	if n, err := writer.Write(nil); n != 0 || err != nil {
		t.Fatalf("empty write = %d, %v", n, err)
	}
	if output.String() != string(first) {
		t.Fatal("an exactly full line or empty write acquired a trailing CRLF")
	}
	if n, err := writer.Write([]byte("BC")); n != 2 || err != nil {
		t.Fatalf("next line = %d, %v", n, err)
	}
	if want := string(first) + "\r\nBC"; output.String() != want {
		t.Fatal("next write lost or duplicated the pending line break")
	}
}

type attachmentCountingWriter struct{ calls, bytes int }

func (w *attachmentCountingWriter) Write(p []byte) (int, error) {
	w.calls++
	w.bytes += len(p)
	return len(p), nil
}

func TestEmailAttachmentEncodingHasBoundedWriteOverhead(t *testing.T) {
	data := make([]byte, 1<<20)
	var sink attachmentCountingWriter
	var encodeErr error
	var encoded int
	allocations := testing.AllocsPerRun(1, func() {
		sink = attachmentCountingWriter{}
		encoder := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(&sink))
		encoded, encodeErr = encoder.Write(data)
		if encodeErr == nil {
			encodeErr = encoder.Close()
		}
	})
	if encodeErr != nil || encoded != len(data) {
		t.Fatalf("encode = %d, %v", encoded, encodeErr)
	}
	characters := base64.StdEncoding.EncodedLen(len(data))
	wantBytes := characters + 2*((characters-1)/76)
	if sink.bytes != wantBytes {
		t.Fatalf("wire bytes = %d; want %d", sink.bytes, wantBytes)
	}
	t.Logf("attachment_bytes=%d wire_bytes=%d writes=%d allocations=%.0f", len(data), sink.bytes, sink.calls, allocations)
	// One encoded line needs a content write and (except the last) a CRLF.
	// Allow a further split for the base64 encoder's own buffer boundaries.
	if maxCalls := 3 * ((characters + 75) / 76); sink.calls > maxCalls {
		t.Errorf("encoded attachment needed %d writes; bound is %d", sink.calls, maxCalls)
	}
	// The encoder and wrapping writer may allocate once, never per byte/line.
	if allocations > 16 {
		t.Errorf("encoding allocated %.0f objects; want at most 16", allocations)
	}
}

func largeEmailAttachmentFixture() (resolvedEmailAddresses, []EmailAttachment) {
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte(i*31 + 7)
	}
	attachments := []EmailAttachment{
		{Filename: "report.bin", ContentType: "application/octet-stream", Data: data},
		{Filename: "empty.csv", ContentType: "text/csv"},
		{Filename: "summary.csv", ContentType: "text/csv", Data: []byte("node,usage\nfixture,42\n")},
	}
	addresses := resolvedEmailAddresses{
		from:    &mail.Address{Address: "pulse@example.test"},
		to:      []*mail.Address{{Address: "ops@example.test"}, {Address: "reports@example.test"}},
		replyTo: &mail.Address{Address: "reply@example.test"},
	}
	return addresses, attachments
}

func TestMultipartLargeAttachmentContentIsUnchanged(t *testing.T) {
	addresses, attachments := largeEmailAttachmentFixture()
	raw, err := buildMultipartEmailMessageWithAttachments(addresses, "Scheduled report", "<p>Attached</p>", "Attached", attachments,
		"<synthetic-thread@example.test>", time.Unix(1700000000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	verifyLargeAttachmentMessage(t, raw, attachments, "<synthetic-thread@example.test>")
}

func verifyLargeAttachmentMessage(t *testing.T, raw []byte, attachments []EmailAttachment, threadID string) {
	t.Helper()
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for header, want := range map[string]string{
		"From": "<pulse@example.test>", "To": "<ops@example.test>, <reports@example.test>",
		"Reply-To": "<reply@example.test>", "Subject": "Scheduled report",
		"In-Reply-To": threadID, "References": threadID,
	} {
		if got := message.Header.Get(header); got != want {
			t.Errorf("%s = %q; want %q", header, got, want)
		}
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("outer MIME type = %q, %v", mediaType, err)
	}
	mixed := multipart.NewReader(message.Body, params["boundary"])
	part, err := mixed.NextRawPart()
	if err != nil {
		t.Fatal(err)
	}
	mediaType, params, err = mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" {
		t.Fatalf("inner MIME type = %q, %v", mediaType, err)
	}
	alternative := multipart.NewReader(part, params["boundary"])
	for _, content := range []struct{ mimeType, body string }{{"text/plain", "Attached"}, {"text/html", "<p>Attached</p>"}} {
		p, err := alternative.NextRawPart()
		if err != nil {
			t.Fatal(err)
		}
		mediaType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil || mediaType != content.mimeType || p.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
			t.Fatalf("alternative part headers = %v", p.Header)
		}
		body, err := io.ReadAll(quotedprintable.NewReader(p))
		if err != nil || string(body) != content.body {
			t.Fatalf("alternative body changed: %q, %v", body, err)
		}
	}
	if _, err := alternative.NextRawPart(); err != io.EOF {
		t.Fatalf("extra alternative part: %v", err)
	}
	for _, attachment := range attachments {
		if len(attachment.Data) == 0 {
			continue
		}
		part, err := mixed.NextRawPart()
		if err != nil {
			t.Fatal(err)
		}
		if part.FileName() != attachment.Filename || part.Header.Get("Content-Transfer-Encoding") != "base64" {
			t.Fatalf("attachment headers changed: %v", part.Header)
		}
		mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil || mediaType != attachment.ContentType {
			t.Fatalf("attachment type changed: %q, %v", mediaType, err)
		}
		encoded, err := io.ReadAll(part)
		// SMTP DotReader normalises CRLF to LF; direct construction retains CRLF.
		if err != nil || strings.ReplaceAll(string(encoded), "\r\n", "\n") != strings.ReplaceAll(expectedAttachmentEncoding(attachment.Data), "\r\n", "\n") {
			t.Fatalf("attachment wire content changed for %s: %v", attachment.Filename, err)
		}
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(encoded)))
		if err != nil || !bytes.Equal(decoded, attachment.Data) {
			t.Fatalf("attachment decoded content changed for %s: %v", attachment.Filename, err)
		}
	}
	if _, err := mixed.NextRawPart(); err != io.EOF {
		t.Fatalf("extra mixed part or missing final boundary: %v", err)
	}
}

func BenchmarkEmailAttachmentEncoding(b *testing.B) {
	for _, size := range []int{1 << 10, 1 << 20} {
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			data := make([]byte, size)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				encoder := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(io.Discard))
				if _, err := encoder.Write(data); err != nil {
					b.Fatal(err)
				}
				if err := encoder.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMultipartEmailAttachment(b *testing.B) {
	addresses := resolvedEmailAddresses{from: &mail.Address{Address: "pulse@example.test"}, to: []*mail.Address{{Address: "ops@example.test"}}}
	attachment := EmailAttachment{Filename: "report.bin", ContentType: "application/octet-stream", Data: make([]byte, 1<<20)}
	b.SetBytes(int64(len(attachment.Data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildMultipartEmailMessageWithAttachments(addresses, "Report", "<p>Attached</p>", "Attached", []EmailAttachment{attachment}, "", time.Unix(1700000000, 0)); err != nil {
			b.Fatal(err)
		}
	}
}

// A bounded destination exposes both accepted byte counts and write failures.
// Production MIME writes to a bytes.Buffer, but the wrapper must obey io.Writer
// when its destination refuses a content segment or a line separator.
type attachmentFailingWriter struct {
	remaining int
	err       error
	output    bytes.Buffer
	calls     int
}

func (w *attachmentFailingWriter) Write(p []byte) (int, error) {
	w.calls++
	n := min(len(p), w.remaining)
	_, _ = w.output.Write(p[:n])
	w.remaining -= n
	if w.remaining == 0 {
		return n, w.err
	}
	return n, nil
}

func TestBase64LineWriterStopsOnDestinationFailure(t *testing.T) {
	cause := fmt.Errorf("synthetic attachment write failure")
	for _, tc := range []struct {
		name                          string
		limit, wantWritten, wantCalls int
		err, wantErr                  error
	}{
		{"content error", 13, 13, 1, cause, cause},
		{"line break error", 76, 76, 2, cause, cause},
		{"partial line break error", 77, 76, 2, cause, cause},
		{"next content error", 87, 85, 3, cause, cause},
		{"short content", 13, 13, 1, nil, io.ErrShortWrite},
		{"short line break", 77, 76, 2, nil, io.ErrShortWrite},
		{"no progress", 0, 0, 1, nil, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &attachmentFailingWriter{remaining: tc.limit, err: tc.err}
			n, err := newBase64LineWriter(sink).Write(bytes.Repeat([]byte{'A'}, 100))
			if n != tc.wantWritten || err != tc.wantErr || sink.calls != tc.wantCalls {
				t.Fatalf("Write = %d, %v, calls=%d; want %d, %v, %d", n, err, sink.calls, tc.wantWritten, tc.wantErr, tc.wantCalls)
			}
			// The count is input bytes only: any CRLF is output, not consumed input.
			want := strings.Repeat("A", 76) + "\r\n" + strings.Repeat("A", 24)
			if sink.output.String() != want[:tc.limit] {
				t.Fatalf("bytes before error changed for %s", tc.name)
			}
		})
	}
}

func TestSMTPLargeAttachmentContentSurvivesAllTransports(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		t.Run(mode, func(t *testing.T) {
			messages := make(chan []byte, 1)
			manager, connections, accepted := acceptanceSMTPServer(t, mode, 250, "221 Bye", messages)
			addresses, attachments := largeEmailAttachmentFixture()
			manager.config.From = addresses.from.Address
			manager.config.To = envelopeRecipients(addresses.to)
			manager.config.ReplyTo = addresses.replyTo.Address
			require.NoError(t, manager.SendEmailWithAttachments("Scheduled report", "<p>Attached</p>", "Attached", attachments))
			require.Equal(t, int32(1), connections.Load())
			require.Equal(t, int32(1), accepted.Load())
			select {
			case raw := <-messages:
				verifyLargeAttachmentMessage(t, raw, attachments, "")
			default:
				t.Fatal("missing accepted SMTP message")
			}
		})
	}
}
