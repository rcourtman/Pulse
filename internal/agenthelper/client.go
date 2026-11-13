package agenthelper

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"
)

type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type RequestIDFunc func() (string, error)

type ClientConfig struct {
	SocketPath   string
	MaxDeadline  time.Duration
	DialContext  DialContextFunc
	NewRequestID RequestIDFunc
}

type Client struct {
	socketPath   string
	maxDeadline  time.Duration
	dialContext  DialContextFunc
	newRequestID RequestIDFunc
}

type RemoteError struct {
	RequestID string
	Code      string
	Message   string
	Retryable bool
}

func (e *RemoteError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("helper operation failed (%s): %s", e.Code, e.Message)
}

func NewClient(config ClientConfig) (*Client, error) {
	path := strings.TrimSpace(config.SocketPath)
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("helper socket path must be a clean absolute path")
	}
	maxDeadline := config.MaxDeadline
	if maxDeadline <= 0 {
		maxDeadline = defaultMaxOperationTimeout
	}
	if maxDeadline < time.Millisecond {
		return nil, errors.New("maximum helper deadline must be at least 1ms")
	}
	dial := config.DialContext
	if dial == nil {
		dialer := &net.Dialer{}
		dial = dialer.DialContext
	}
	requestID := config.NewRequestID
	if requestID == nil {
		requestID = randomRequestID
	}
	return &Client{
		socketPath:   path,
		maxDeadline:  maxDeadline,
		dialContext:  dial,
		newRequestID: requestID,
	}, nil
}

// Call performs exactly one typed operation over one Unix-socket connection.
// It returns the generated request ID even when the helper returns a typed
// remote error so callers can correlate local and helper audit records. The
// operation budget includes dialing and all socket I/O. Cancellation closes
// only this local connection: an accepted remote operation may still complete.
func (c *Client) Call(
	ctx context.Context,
	operation string,
	operationVersion int,
	deadline time.Duration,
	payload any,
	result any,
) (requestID string, callErr error) {
	if strings.TrimSpace(operation) == "" || operationVersion < 1 {
		return "", errors.New("helper operation and positive version are required")
	}
	if deadline <= 0 || deadline > c.maxDeadline {
		return "", errors.New("helper operation deadline is outside the allowed range")
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	operationDeadline, _ := ctx.Deadline()
	// Socket deadlines can fire before the context timer is scheduled. Preserve
	// context identity as well as the transport error in either ordering.
	defer func() {
		if callErr == nil {
			return
		}
		contextErr := ctx.Err()
		if contextErr == nil && !time.Now().Before(operationDeadline) {
			contextErr = context.DeadlineExceeded
		}
		if contextErr != nil && !errors.Is(callErr, contextErr) {
			callErr = errors.Join(callErr, contextErr)
		}
	}()
	deadlineMillis := time.Until(operationDeadline).Milliseconds()
	if deadlineMillis < 1 {
		deadlineMillis = 1
	}

	requestID, err := c.newRequestID()
	if err != nil {
		return "", fmt.Errorf("create helper request ID: %w", err)
	}
	if !validRequestID(requestID) {
		return "", errors.New("generated helper request ID is invalid")
	}

	payloadJSON := json.RawMessage(`{}`)
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return requestID, fmt.Errorf("encode helper operation payload: %w", err)
		}
		payloadJSON = encoded
	}
	request := Request{
		ProtocolVersion:  ProtocolVersion,
		RequestID:        requestID,
		Operation:        operation,
		OperationVersion: operationVersion,
		DeadlineMillis:   deadlineMillis,
		Payload:          payloadJSON,
	}
	framed, err := EncodeRequestFrame(request)
	if err != nil {
		return requestID, err
	}
	if err := ctx.Err(); err != nil {
		return requestID, err
	}

	conn, err := c.dialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return requestID, fmt.Errorf("connect to helper Unix socket: %w", err)
	}
	defer conn.Close()
	closeDone := make(chan struct{})
	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close() // Interrupt both a blocked request write and response read.
		close(closeDone)
	})
	defer func() {
		if !stopClose() {
			<-closeDone // The callback must not outlive this connection's owner.
		}
	}()
	if err := ctx.Err(); err != nil {
		return requestID, err // A canceled dial result must not send a new frame.
	}
	if err := conn.SetDeadline(operationDeadline); err != nil {
		return requestID, fmt.Errorf("set helper connection deadline: %w", err)
	}
	// Preparation and dialing consumed part of the same operation budget. Do
	// not grant that time again to the remote provider after connecting. Keep
	// the earlier frame validation before dialing, and reuse the encoded payload.
	remaining := time.Until(operationDeadline)
	if remaining <= 0 {
		return requestID, context.DeadlineExceeded
	}
	request.DeadlineMillis = max(1, remaining.Milliseconds())
	framed, err = EncodeRequestFrame(request)
	if err != nil {
		return requestID, err
	}
	if err := ctx.Err(); err != nil {
		return requestID, err
	}
	if _, err := writeAll(conn, framed); err != nil {
		return requestID, fmt.Errorf("write helper request: %w", err)
	}

	responseFrame, err := ReadResponseFrame(conn)
	if err != nil {
		return requestID, fmt.Errorf("read helper response: %w", err)
	}
	response, err := DecodeResponse(responseFrame)
	if err != nil {
		return requestID, fmt.Errorf("decode helper response: %w", err)
	}
	if err := validateResponseCorrelation(request, response); err != nil {
		return requestID, err
	}
	if !response.Success {
		return requestID, &RemoteError{
			RequestID: requestID,
			Code:      response.Error.Code,
			Message:   response.Error.Message,
			Retryable: response.Error.Retryable,
		}
	}
	if result != nil {
		if len(response.Result) == 0 {
			return requestID, errors.New("successful helper response contains no result")
		}
		if err := decodeStrict(response.Result, result); err != nil {
			return requestID, fmt.Errorf("decode helper operation result: %w", err)
		}
	}
	return requestID, nil
}

func validateResponseCorrelation(request Request, response Response) error {
	if response.ProtocolVersion != ProtocolVersion {
		return errors.New("helper response protocol version does not match")
	}
	if response.RequestID != request.RequestID ||
		response.Operation != request.Operation ||
		response.OperationVersion != request.OperationVersion {
		return errors.New("helper response correlation does not match request")
	}
	if response.Success {
		if response.Error != nil {
			return errors.New("successful helper response contains an error")
		}
		return nil
	}
	if response.Error == nil || strings.TrimSpace(response.Error.Code) == "" {
		return errors.New("failed helper response contains no typed error")
	}
	return nil
}

func randomRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
