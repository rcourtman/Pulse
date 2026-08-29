package agenthelper

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestFrameRoundTripUsesBigEndianLength(t *testing.T) {
	request := Request{
		ProtocolVersion:  ProtocolVersion,
		RequestID:        "request-1",
		Operation:        OperationHealth,
		OperationVersion: OperationVersion1,
		DeadlineMillis:   1000,
		Payload:          []byte(`{}`),
	}
	framed, err := marshalFrame(request, MaxRequestBytes)
	if err != nil {
		t.Fatalf("marshalFrame: %v", err)
	}
	if got := binary.BigEndian.Uint32(framed[:4]); got != uint32(len(framed)-4) {
		t.Fatalf("frame length = %d, want %d", got, len(framed)-4)
	}
	payload, err := readFrame(bytes.NewReader(framed), MaxRequestBytes)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	decoded, err := decodeRequest(payload)
	if err != nil {
		t.Fatalf("decodeRequest: %v", err)
	}
	if decoded.RequestID != request.RequestID || decoded.Operation != request.Operation {
		t.Fatalf("decoded request = %#v", decoded)
	}
}

func TestReadFrameRejectsInvalidLengthsAndTruncation(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{0, 0, 0, 0}},
		{name: "oversized", data: []byte{0, 1, 0, 1}},
		{name: "truncated", data: []byte{0, 0, 0, 4, '{', '}'}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readFrame(bytes.NewReader(test.data), MaxRequestBytes); err == nil {
				t.Fatal("invalid frame accepted")
			}
		})
	}
}

func TestDecodeStrictRejectsUnknownFieldAndTrailingJSON(t *testing.T) {
	for _, raw := range []string{
		`{"protocolVersion":1,"requestId":"r","operation":"helper.health","operationVersion":1,"deadlineMillis":1,"unknown":true}`,
		`{"protocolVersion":1,"requestId":"r","operation":"helper.health","operationVersion":1,"deadlineMillis":1}{}`,
	} {
		if _, err := decodeRequest([]byte(raw)); err == nil {
			t.Fatalf("invalid request accepted: %s", raw)
		}
	}
}

func TestMarshalFrameEnforcesLimit(t *testing.T) {
	value := strings.Repeat("x", 128)
	if _, err := marshalFrame(value, 32); err == nil {
		t.Fatal("oversized marshaled frame accepted")
	}
}

func TestCheckedFrameSizeRejectsInvalidAllocationSizes(t *testing.T) {
	for _, payloadBytes := range []int{-1, math.MaxInt - frameHeaderBytes + 1} {
		if _, err := checkedFrameSize(payloadBytes); err == nil {
			t.Fatalf("invalid payload size accepted: %d", payloadBytes)
		}
	}
	if got, err := checkedFrameSize(128); err != nil || got != frameHeaderBytes+128 {
		t.Fatalf("checkedFrameSize(128) = %d, %v", got, err)
	}
}
