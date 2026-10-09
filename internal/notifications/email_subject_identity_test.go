package notifications

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/textproto"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func subjectIdentityAlert(node string, level alerts.AlertLevel) *alerts.Alert {
	return &alerts.Alert{ID: "docker-" + node + "-nginx-cpu", ResourceID: "docker/" + node + "/nginx",
		ResourceName: "nginx", Node: node, Type: "cpu", Level: level,
		Message: "CPU above threshold", StartTime: time.Date(2026, 10, 7, 2, 0, 0, 123, time.UTC)}
}

// These controls use existing entry points only so identical final test bytes
// can reject the exact parent, rather than test a hand-built replacement subject.
func TestEmailSubjectIdentitySingle(t *testing.T) {
	for _, tc := range []struct {
		name, node, display, resource, resourceID, want string
		level                                           alerts.AlertLevel
	}{
		{"raw", "docker-one", "", "nginx", "id", "[Pulse Alert] Critical: CPU on nginx (docker-one)", alerts.AlertLevelCritical},
		{"display and raw", "docker-two", "Kitchen", "nginx", "id", "[Pulse Alert] Warning: CPU on nginx (Kitchen [docker-two])", alerts.AlertLevelWarning},
		{"display only", "", "Kitchen", "nginx", "id", "[Pulse Alert] Info: CPU on nginx (Kitchen)", alerts.AlertLevelInfo},
		{"same display", "docker-three", "docker-three", "nginx", "id", "[Pulse Alert] Warning: CPU on nginx (docker-three)", alerts.AlertLevelWarning},
		{"no host", "", "", "nginx", "id", "[Pulse Alert] Warning: CPU on nginx (unknown host)", alerts.AlertLevelWarning},
		{"resource ID fallback", "docker-one", "", " \t", "container-123", "[Pulse Alert] Warning: CPU on container-123 (docker-one)", alerts.AlertLevelWarning},
		{"unknown identity and severity", "", "", "", "", "[Pulse Alert] Warning: CPU on Unknown resource (unknown host)", alerts.AlertLevel("unknown")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := subjectIdentityAlert(tc.node, tc.level)
			a.NodeDisplayName, a.ResourceName, a.ResourceID = tc.display, tc.resource, tc.resourceID
			a.Instance = "connection-is-not-a-host"
			before := a.Clone()
			subject, html, text := EmailTemplate([]*alerts.Alert{a}, true)
			if subject != tc.want {
				t.Fatalf("subject = %q, want %q", subject, tc.want)
			}
			if !strings.Contains(text, "- Node: "+alertNodeDisplay(a)) || !strings.Contains(html, "CPU") {
				t.Fatal("identity repair lost the full original body")
			}
			if !reflect.DeepEqual(a, before) {
				t.Fatal("subject construction mutated the incident")
			}
		})
	}
}

func TestEmailSubjectIdentityDigest(t *testing.T) {
	batch := []*alerts.Alert{subjectIdentityAlert("docker-three", alerts.AlertLevelInfo),
		subjectIdentityAlert("docker-one", alerts.AlertLevelCritical), subjectIdentityAlert("docker-two", alerts.AlertLevelWarning)}
	for _, a := range batch {
		a.NodeDisplayName = "Kitchen"
	}
	want := "[Pulse Alert] 1 Critical, 1 Warning, 1 Info alerts: nginx (Kitchen [docker-one]), nginx (Kitchen [docker-three]), nginx (Kitchen [docker-two])"
	subject, html, text := EmailTemplate(batch, false)
	if subject != want {
		t.Fatalf("subject = %q, want %q", subject, want)
	}
	for _, a := range batch {
		if !strings.Contains(text, a.ResourceName) || !strings.Contains(html, a.ResourceName) {
			t.Fatal("digest lost resource body")
		}
	}
	reversed := []*alerts.Alert{batch[2], batch[1], batch[0]}
	reordered, _, _ := EmailTemplate(reversed, false)
	if reordered != subject {
		t.Fatalf("same identities changed subject on reorder: %q", reordered)
	}
	one, _, _ := EmailTemplate(batch[:1], false)
	if one != "[Pulse Alert] 1 Info alert: nginx (Kitchen [docker-three])" {
		t.Fatalf("one-alert digest = %q", one)
	}
}

func TestEmailSubjectIdentityBounds(t *testing.T) {
	batch := make([]*alerts.Alert, 0, 22)
	for i := 19; i >= 0; i-- {
		a := subjectIdentityAlert(fmt.Sprintf("host-%02d", i), alerts.AlertLevelWarning)
		a.ResourceName = "nœud-東京-🚨-" + strings.Repeat("界", 200)
		a.NodeDisplayName = strings.Repeat("🚨", 200)
		batch = append(batch, a)
	}
	// Different metrics for the same resource/host label count in severity,
	// not twice in the subject's identity list.
	batch = append(batch, batch[0].Clone(), batch[0].Clone())
	subject, html, text := EmailTemplate(batch, false)
	if !strings.HasPrefix(subject, "[Pulse Alert] 22 Warning alerts:") || !strings.HasSuffix(subject, "+17 more") {
		t.Fatalf("incorrect alert count or full-label overflow: %q", subject)
	}
	if len(subject) > 600 || !utf8.ValidString(subject) {
		t.Fatalf("unbounded/invalid UTF-8 subject (%d bytes)", len(subject))
	}
	for _, host := range []string{"host-00", "host-01", "host-02"} {
		if !strings.Contains(subject, host) {
			t.Fatalf("deterministic leading identity %s omitted", host)
		}
	}
	if strings.Contains(subject, "host-03") || !strings.Contains(subject, "…") {
		t.Fatal("identity/byte bounds not applied")
	}
	if !strings.Contains(html, batch[0].ResourceName) || !strings.Contains(text, batch[0].NodeDisplayName) {
		t.Fatal("bounding removed full body labels")
	}
	// Four distinct full labels with the same truncated hint must still report
	// an omitted label, not silently collapse into one.
	for i := range batch[:4] {
		batch[i].Node, batch[i].NodeDisplayName = strings.Repeat("x", 100)+fmt.Sprint(i), ""
	}
	collision, _, _ := EmailTemplate(batch[:4], false)
	if !strings.HasSuffix(collision, "+1 more") {
		t.Fatalf("truncated hints collapsed full identities: %q", collision)
	}
	empty, _, _ := EmailTemplate(nil, false)
	if strings.Contains(empty, "unknown host") || strings.Contains(empty, ": ") {
		t.Fatalf("empty digest invented identity: %q", empty)
	}
}

func readIdentityEmail(t *testing.T, raw []byte) (*mail.Message, string) {
	t.Helper()
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("Bcc") != "" || message.Header.Get("X-Injected") != "" {
		t.Fatal("identity injected an email header")
	}
	if !utf8.ValidString(subject) {
		t.Fatal("invalid decoded UTF-8")
	}
	for _, r := range subject {
		if unicode.IsControl(r) {
			t.Fatalf("subject retained control %U", r)
		}
	}
	return message, subject
}

func TestEmailSubjectIdentityLiteralEncodedWord(t *testing.T) {
	a := subjectIdentityAlert("=?UTF-8?q?different_host?=", alerts.AlertLevelWarning)
	a.ResourceName = "=?UTF-8?q?different_resource?="
	subject, html, text := EmailTemplate([]*alerts.Alert{a}, true)
	manager := newEmailDeliveryManager(EmailConfig{From: "pulse@example.test"}, []string{"recipient@example.test"})
	addresses, err := manager.resolveEmailAddresses()
	if err != nil {
		t.Fatal(err)
	}
	for _, attachments := range [][]EmailAttachment{nil, {{Filename: "report.txt", Data: []byte("safe")}}} {
		raw, err := buildMultipartEmailMessageWithAttachments(addresses, subject, html, text, attachments, "", a.StartTime)
		if err != nil {
			t.Fatal(err)
		}
		_, decoded := readIdentityEmail(t, raw)
		if decoded != subject {
			t.Fatalf("literal name interpreted as MIME syntax: %q, want %q", decoded, subject)
		}
	}
}

func TestEmailSubjectIdentityMIME(t *testing.T) {
	a := subjectIdentityAlert("docker-東京\r\nBcc: hidden@example.test", alerts.AlertLevelCritical)
	a.ResourceName = "café-🚨\x00\t\r\nX-Injected: no"
	a.NodeDisplayName = "Kitchen\nHouse"
	a.Type = strings.Repeat("界", 100)
	b := a.Clone()
	b.Node = strings.Repeat("🚨", 100)
	c := a.Clone()
	c.Node = strings.Repeat("界", 100)
	subject, html, text := EmailTemplate([]*alerts.Alert{a, b, c}, false)
	for _, single := range []bool{false, true} {
		if single {
			subject, html, text = EmailTemplate([]*alerts.Alert{a}, true)
		}
		for _, attachments := range [][]EmailAttachment{nil, {{Filename: "report.txt", ContentType: "text/plain", Data: []byte("safe attachment")}}} {
			manager := newEmailDeliveryManager(EmailConfig{From: "pulse@example.test"}, []string{"recipient@example.test"})
			addresses, err := manager.resolveEmailAddresses()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := buildMultipartEmailMessageWithAttachments(addresses, subject, html, text, attachments, alertListThreadID([]*alerts.Alert{a}), a.StartTime)
			if err != nil {
				t.Fatal(err)
			}
			parsed, decoded := readIdentityEmail(t, raw)
			if decoded != subject {
				t.Fatalf("MIME round trip = %q, want %q", decoded, subject)
			}
			header := strings.SplitN(string(raw), "\r\n\r\n", 2)[0]
			if !strings.Contains(header, "=?UTF-8?q?") {
				t.Fatal("Unicode identity not RFC 2047 encoded")
			}
			inSubject := false
			for _, line := range strings.Split(header, "\r\n") {
				if strings.HasPrefix(line, "Subject:") {
					inSubject = true
				} else if !strings.HasPrefix(line, " ") {
					inSubject = false
				}
				if inSubject && len(line) > 78 {
					t.Fatalf("unfolded subject line exceeds 78 bytes: %d", len(line))
				}
			}
			media, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			if media != "multipart/alternative" && media != "multipart/mixed" {
				t.Fatalf("incorrect MIME type %q", media)
			}
			reader := multipart.NewReader(parsed.Body, params["boundary"])
			part, err := reader.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			if len(attachments) != 0 {
				_, nested, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
				if err != nil {
					t.Fatal(err)
				}
				part, err = multipart.NewReader(part, nested["boundary"]).NextPart()
				if err != nil {
					t.Fatal(err)
				}
			}
			body, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != normalizeEmailBodyLineEndings(text) {
				t.Fatal("MIME identity repair changed full text body")
			}
		}
	}
}

// Only in-memory net.Pipe connections: no listeners, provider, credentials,
// induced alert or real SMTP. Capture the actual MIME send from the existing
// normal single/grouped/resolved methods and their incident threading boundary.
func captureSubjectEmails(t *testing.T) <-chan []byte {
	t.Helper()
	messages := make(chan []byte, 32)
	var servers sync.WaitGroup
	original := smtpDialTimeout
	t.Cleanup(func() { smtpDialTimeout = original; servers.Wait() })
	smtpDialTimeout = func(network, address string, timeout time.Duration) (net.Conn, error) {
		if address != "smtp.example.test:25" {
			t.Fatalf("unexpected transport address %q", address)
		}
		client, server := net.Pipe()
		servers.Add(1)
		go func() {
			defer servers.Done()
			defer server.Close()
			_ = server.SetDeadline(time.Now().Add(5 * time.Second))
			conn := textproto.NewConn(server)
			if err := conn.PrintfLine("220 fixture ESMTP"); err != nil {
				t.Error(err)
				return
			}
			for {
				line, err := conn.ReadLine()
				if err != nil {
					t.Error(err)
					return
				}
				fields := strings.Fields(line)
				if len(fields) == 0 {
					t.Error("empty SMTP command")
					return
				}
				switch fields[0] {
				case "EHLO", "HELO", "MAIL", "RCPT":
					err = conn.PrintfLine("250 OK")
				case "DATA":
					if err = conn.PrintfLine("354 Send message"); err != nil {
						t.Error(err)
						return
					}
					raw, readErr := conn.ReadDotBytes()
					if readErr != nil {
						t.Error(readErr)
						return
					}
					messages <- raw
					err = conn.PrintfLine("250 Accepted by fixture")
				case "QUIT":
					_ = conn.PrintfLine("221 Bye")
					return
				default:
					t.Errorf("unexpected command %q", fields[0])
					return
				}
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
		return client, nil
	}
	return messages
}

func TestEmailSubjectIdentityNormalSend(t *testing.T) {
	messages := captureSubjectEmails(t)
	config := EmailConfig{SMTPHost: "smtp.example.test", SMTPPort: 25, From: "pulse@example.test", To: []string{"recipient@example.test"}, RateLimit: 1000}
	manager := &NotificationManager{}
	batch := []*alerts.Alert{subjectIdentityAlert("docker-one", alerts.AlertLevelCritical),
		subjectIdentityAlert("docker-two", alerts.AlertLevelWarning), subjectIdentityAlert("docker-three", alerts.AlertLevelInfo)}
	check := func(want, thread string) {
		t.Helper()
		select {
		case raw := <-messages:
			parsed, subject := readIdentityEmail(t, raw)
			if subject != want {
				t.Fatalf("normal sent subject = %q, want %q", subject, want)
			}
			if parsed.Header.Get("References") != thread || parsed.Header.Get("In-Reply-To") != thread {
				t.Fatal("identity changed occurrence threading")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("normal send produced no MIME message")
		}
	}
	for _, a := range batch {
		if err := manager.sendSingleEmailWithError(a, config); err != nil {
			t.Fatal(err)
		}
		check(fmt.Sprintf("[Pulse Alert] %s: CPU on nginx (%s)", titleCase(string(a.Level)), a.Node), alertThreadMessageID(a.ID, a.StartTime))
	}
	if err := manager.sendGroupedEmail(config, batch); err != nil {
		t.Fatal(err)
	}
	check("[Pulse Alert] 1 Critical, 1 Warning, 1 Info alerts: nginx (docker-one), nginx (docker-three), nginx (docker-two)", "")
	if err := manager.sendGroupedEmail(config, batch[:1]); err != nil {
		t.Fatal(err)
	}
	thread := alertThreadMessageID(batch[0].ID, batch[0].StartTime)
	check("[Pulse Alert] 1 Critical alert: nginx (docker-one)", thread)
	if err := manager.sendResolvedEmail(config, batch[:1], time.Now()); err != nil {
		t.Fatal(err)
	}
	check("Pulse alert resolved: nginx (docker-one)", thread)
	if err := manager.sendResolvedEmail(config, batch, time.Now()); err != nil {
		t.Fatal(err)
	}
	check("Pulse alerts resolved (3): nginx (docker-one), nginx (docker-three), nginx (docker-two)", "")
	moved := movedNodeMemoryAlert()
	if err := manager.sendResolvedEmail(config, []*alerts.Alert{moved}, time.Now()); err != nil {
		t.Fatal(err)
	}
	check("Pulse alert moved: pve1 (pve1)", alertThreadMessageID(moved.ID, moved.StartTime))
	next := batch[0].Clone()
	next.StartTime = next.StartTime.Add(time.Nanosecond)
	if err := manager.sendSingleEmailWithError(next, config); err != nil {
		t.Fatal(err)
	}
	nextThread := alertThreadMessageID(next.ID, next.StartTime)
	if nextThread == thread {
		t.Fatal("adjacent occurrence reused incident thread")
	}
	check("[Pulse Alert] Critical: CPU on nginx (docker-one)", nextThread)
	if err := manager.sendResolvedEmail(config, nil, time.Now()); err == nil {
		t.Fatal("empty resolution admitted")
	}
	if err := manager.sendResolvedEmail(config, []*alerts.Alert{nil}, time.Now()); err == nil {
		t.Fatal("nil-only resolution admitted")
	}
	select {
	case <-messages:
		t.Fatal("unexpected extra message")
	default:
	}
}
