package agentcapabilities

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewErrorEnvelopeOmitsEmptyDetails(t *testing.T) {
	envelope := NewErrorEnvelope(AgentErrCodeResourceNotFound, "Resource not found", nil)
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if string(body) != `{"error":"resource_not_found","message":"Resource not found"}` {
		t.Fatalf("ErrorEnvelope JSON = %s", body)
	}
}

func TestNewErrorEnvelopePreservesDetails(t *testing.T) {
	envelope := NewErrorEnvelope(AgentErrCodeOperatorStateInvalid, "Invalid operator state", map[string]string{
		"intentionallyOffline": "must be boolean",
	})
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if !strings.Contains(string(body), `"details":{"intentionallyOffline":"must be boolean"}`) {
		t.Fatalf("ErrorEnvelope JSON must include details: %s", body)
	}
}

func TestDecodeErrorEnvelopeRecognizesSharedShape(t *testing.T) {
	envelope, ok := DecodeErrorEnvelope([]byte(`{"error":"resource_not_found","message":"missing"}`))
	if !ok {
		t.Fatal("DecodeErrorEnvelope did not recognize shared shape")
	}
	if envelope.Error != "resource_not_found" || envelope.Message != "missing" {
		t.Fatalf("decoded envelope = %+v", envelope)
	}

	if _, ok := DecodeErrorEnvelope([]byte(`{"code":"resource_not_found","error":"human text"}`)); ok {
		t.Fatal("DecodeErrorEnvelope recognized the platform-wide API error shape")
	}
	if _, ok := DecodeErrorEnvelope([]byte(`not-json`)); ok {
		t.Fatal("DecodeErrorEnvelope recognized non-JSON")
	}
	if _, ok := DecodeErrorEnvelope([]byte(`{"message":"missing error"}`)); ok {
		t.Fatal("DecodeErrorEnvelope recognized envelope without error code")
	}
	if _, ok := DecodeErrorEnvelope([]byte(`{"error":"missing_message"}`)); ok {
		t.Fatal("DecodeErrorEnvelope recognized envelope without message")
	}
}
