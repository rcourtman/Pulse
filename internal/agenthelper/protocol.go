package agenthelper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	ProtocolVersion = 1

	MaxRequestBytes  uint32 = 64 * 1024
	MaxResponseBytes uint32 = 10 * 1024 * 1024
)

const (
	ErrorInvalidFrame         = "invalid_frame"
	ErrorInvalidRequest       = "invalid_request"
	ErrorUnsupportedProtocol  = "unsupported_protocol"
	ErrorUnknownOperation     = "unknown_operation"
	ErrorUnsupportedOperation = "unsupported_operation_version"
	ErrorUnauthorizedPeer     = "unauthorized_peer"
	ErrorDeadlineExceeded     = "deadline_exceeded"
	ErrorProviderUnavailable  = "provider_unavailable"
	ErrorResponseTooLarge     = "response_too_large"
	ErrorInternal             = "internal_error"
)

const (
	OperationHealth                = "helper.health"
	OperationCapabilities          = "helper.capabilities"
	OperationSMARTSnapshot         = "smart.snapshot"
	OperationProxmoxLXCFilesystems = "proxmox.lxc_filesystems"
	OperationVersion1              = 1
)

// Request is the common envelope for one local helper operation. Payload is
// decoded again against the selected operation's exact schema.
type Request struct {
	ProtocolVersion  int             `json:"protocolVersion"`
	RequestID        string          `json:"requestId"`
	Operation        string          `json:"operation"`
	OperationVersion int             `json:"operationVersion"`
	DeadlineMillis   int64           `json:"deadlineMillis"`
	Payload          json.RawMessage `json:"payload,omitempty"`
}

type Response struct {
	ProtocolVersion  int             `json:"protocolVersion"`
	RequestID        string          `json:"requestId,omitempty"`
	Operation        string          `json:"operation,omitempty"`
	OperationVersion int             `json:"operationVersion,omitempty"`
	Success          bool            `json:"success"`
	Result           json.RawMessage `json:"result,omitempty"`
	Error            *ResponseError  `json:"error,omitempty"`
}

type ResponseError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type frameError struct {
	message string
}

func (e *frameError) Error() string { return e.message }

func readFrame(r io.Reader, limit uint32) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, &frameError{message: fmt.Sprintf("read frame length: %v", err)}
	}

	length := binary.BigEndian.Uint32(header[:])
	if length == 0 {
		return nil, &frameError{message: "frame payload is empty"}
	}
	if length > limit {
		return nil, &frameError{message: fmt.Sprintf("frame payload exceeds %d bytes", limit)}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, &frameError{message: fmt.Sprintf("read frame payload: %v", err)}
	}
	return payload, nil
}

func marshalFrame(value any, limit uint32) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal frame: %w", err)
	}
	if len(payload) == 0 || uint64(len(payload)) > uint64(limit) {
		return nil, &frameError{message: fmt.Sprintf("frame payload exceeds %d bytes", limit)}
	}

	framed := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(framed[:4], uint32(len(payload)))
	copy(framed[4:], payload)
	return framed, nil
}

func writeFrame(w io.Writer, value any, limit uint32) (int, error) {
	framed, err := marshalFrame(value, limit)
	if err != nil {
		return 0, err
	}
	written, err := writeAll(w, framed)
	if err != nil {
		return written, fmt.Errorf("write frame: %w", err)
	}
	return written, nil
}

func writeAll(w io.Writer, payload []byte) (int, error) {
	total := 0
	for total < len(payload) {
		written, err := w.Write(payload[total:])
		total += written
		if err != nil {
			return total, err
		}
		if written == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value is not allowed")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}

func decodeRequest(data []byte) (Request, error) {
	var request Request
	if err := decodeStrict(data, &request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	return decodeStrict(raw, target)
}
