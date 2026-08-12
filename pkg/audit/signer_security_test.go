package audit

import (
	"strings"
	"testing"
	"time"
)

func securityTestSigner(t *testing.T) *Signer {
	t.Helper()
	signer, err := NewSignerWithKey([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewSignerWithKey: %v", err)
	}
	return signer
}

func securityTestEvent() Event {
	return Event{
		ID:        "event-123",
		Timestamp: time.Unix(1_725_555_555, 0).UTC(),
		EventType: "security",
		User:      "operator",
		IP:        "2001:db8::1",
		Path:      "/api/audit",
		Success:   true,
		Details:   "detected",
	}
}

func TestSignerV2RejectsEquivalentLegacyBoundaryShifts(t *testing.T) {
	signer := securityTestSigner(t)

	tests := []struct {
		name   string
		mutate func(original, forged *Event)
	}{
		{
			name: "event type and user",
			mutate: func(original, forged *Event) {
				original.EventType, original.User = "security", "alert|system"
				forged.EventType, forged.User = "security|alert", "system"
			},
		},
		{
			name: "user and IP",
			mutate: func(original, forged *Event) {
				original.User, original.IP = "alice", "admin|127.0.0.1"
				forged.User, forged.IP = "alice|admin", "127.0.0.1"
			},
		},
		{
			name: "IP and path",
			mutate: func(original, forged *Event) {
				original.IP, original.Path = "10.0.0.1", "internal|/api/audit"
				forged.IP, forged.Path = "10.0.0.1|internal", "/api/audit"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := securityTestEvent()
			forged := original
			tt.mutate(&original, &forged)

			if signer.legacyZeroOneCanonicalForm(original) != signer.legacyZeroOneCanonicalForm(forged) {
				t.Fatal("test pair does not reproduce the historical boundary collision")
			}
			original.Signature = signer.Sign(original)
			if original.Signature == signer.Sign(forged) {
				t.Fatal("v2 signatures collided for distinct tuples")
			}
			forged.Signature = original.Signature
			if signer.Verify(forged) {
				t.Fatal("v2 signature verified after a boundary shift")
			}
		})
	}
}

func TestSignerV2AuthenticatesEveryPersistedField(t *testing.T) {
	signer := securityTestSigner(t)
	original := securityTestEvent()
	original.Signature = signer.Sign(original)

	tests := []struct {
		name   string
		mutate func(*Event)
	}{
		{name: "ID", mutate: func(e *Event) { e.ID += "-tampered" }},
		{name: "timestamp", mutate: func(e *Event) { e.Timestamp = e.Timestamp.Add(time.Second) }},
		{name: "event type", mutate: func(e *Event) { e.EventType += "-tampered" }},
		{name: "user", mutate: func(e *Event) { e.User += "-tampered" }},
		{name: "IP", mutate: func(e *Event) { e.IP += "-tampered" }},
		{name: "path", mutate: func(e *Event) { e.Path += "-tampered" }},
		{name: "success", mutate: func(e *Event) { e.Success = !e.Success }},
		{name: "details", mutate: func(e *Event) { e.Details += "-tampered" }},
		{name: "field order", mutate: func(e *Event) { e.EventType, e.User = e.User, e.EventType }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tampered := original
			tt.mutate(&tampered)
			if signer.Verify(tampered) {
				t.Fatal("signature verified after tampering")
			}
		})
	}
}

func TestSignerV2PreservesArbitraryStringContent(t *testing.T) {
	signer := securityTestSigner(t)
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "pipes", value: "a||b|c"},
		{name: "unicode", value: "監査ログ 🔐 café"},
		{name: "newlines and null", value: "first\nsecond\r\n\x00last"},
		{name: "large", value: strings.Repeat("界|\n", 350_000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := securityTestEvent()
			event.User = tt.value
			event.Details = tt.value
			event.Signature = signer.Sign(event)
			if DetectSignatureVersion(event.Signature) != SignatureVersionV2 {
				t.Fatalf("signature version = %q, want v2", DetectSignatureVersion(event.Signature))
			}
			if !signer.Verify(event) {
				t.Fatal("valid v2 signature did not verify")
			}
			tampered := event
			tampered.Details += "x"
			if signer.Verify(tampered) {
				t.Fatal("tampered arbitrary content verified")
			}
		})
	}
}

func TestSignerV2CanonicalRepresentationFixture(t *testing.T) {
	signer := securityTestSigner(t)
	event := Event{
		ID:        "id|1",
		Timestamp: time.Unix(-1, 0).UTC(),
		EventType: "監査",
		User:      "",
		IP:        "\x00",
		Path:      "\n",
		Success:   true,
		Details:   "done|ok",
	}
	want := "pulse.audit.event\x00v2\x00" +
		"\x00\x00\x00\x00\x00\x00\x00\x04id|1" +
		"\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\x00\x00\x00\x00\x00\x00\x00\x06監査" +
		"\x00\x00\x00\x00\x00\x00\x00\x00" +
		"\x00\x00\x00\x00\x00\x00\x00\x01\x00" +
		"\x00\x00\x00\x00\x00\x00\x00\x01\n" +
		"\x01" +
		"\x00\x00\x00\x00\x00\x00\x00\x07done|ok"
	if got := string(signer.canonicalV2Form(event)); got != want {
		t.Fatalf("canonical v2 bytes changed:\n got %x\nwant %x", got, want)
	}
}

func TestSignerVersionDispatchFailsClosedWithoutDowngrade(t *testing.T) {
	signer := securityTestSigner(t)
	event := securityTestEvent()
	v2Signature := signer.Sign(event)
	legacySignature := signer.signCanonical(signer.legacyZeroOneCanonicalForm(event))

	for _, signature := range []string{
		"v2:",
		"v2:not-hex",
		"v2:" + strings.Repeat("0", 62),
		"v2:" + strings.Repeat("0", 66),
		"v3:" + strings.TrimPrefix(v2Signature, signatureV2Prefix),
		"unknown:" + strings.TrimPrefix(v2Signature, signatureV2Prefix),
		strings.TrimPrefix(v2Signature, signatureV2Prefix),
		"v2:" + legacySignature,
		legacySignature + "00",
	} {
		t.Run(signature, func(t *testing.T) {
			tamperedEnvelope := event
			tamperedEnvelope.Signature = signature
			if signer.Verify(tamperedEnvelope) {
				t.Fatalf("unexpected verification for %q", signature)
			}
		})
	}

	if DetectSignatureVersion(v2Signature) != SignatureVersionV2 {
		t.Fatal("valid v2 signature was not identified")
	}
	if DetectSignatureVersion(legacySignature) != SignatureVersionLegacy {
		t.Fatal("valid legacy envelope was not identified")
	}
	if DetectSignatureVersion("v3:"+strings.TrimPrefix(v2Signature, signatureV2Prefix)) != SignatureVersionUnknown {
		t.Fatal("unknown version did not fail closed")
	}
}

func TestSignerLegacyCompatibilityIsExplicitlyBoundaryAmbiguous(t *testing.T) {
	signer := securityTestSigner(t)
	original := securityTestEvent()
	original.EventType, original.User = "security", "alert|system"
	forged := original
	forged.EventType, forged.User = "security|alert", "system"

	// Historical unversioned records remain readable, but cannot retroactively
	// prove which side of a pipe a value belonged to. The legacy envelope makes
	// that lower assurance identifiable without rewriting stored history.
	original.Signature = signer.signCanonical(signer.legacyZeroOneCanonicalForm(original))
	if DetectSignatureVersion(original.Signature) != SignatureVersionLegacy {
		t.Fatal("historical signature was not identified as legacy")
	}
	if !signer.Verify(original) {
		t.Fatal("historical signature did not verify")
	}
	forged.Signature = original.Signature
	if !signer.Verify(forged) {
		t.Fatal("legacy fixture no longer demonstrates its documented boundary ambiguity")
	}
}
