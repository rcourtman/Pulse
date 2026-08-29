package agenthelper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const defaultMaxOperationTimeout = 30 * time.Second
const responseWriteTimeout = 5 * time.Second

type Peer struct {
	UID uint32
	GID uint32
	PID int32
}

type PeerResolver interface {
	Resolve(net.Conn) (Peer, error)
}

type PeerResolverFunc func(net.Conn) (Peer, error)

func (f PeerResolverFunc) Resolve(conn net.Conn) (Peer, error) { return f(conn) }

// AuditEvent intentionally contains only transport and operation metadata.
// Request payloads and result data never reach the audit hook.
type AuditEvent struct {
	RequestID        string
	Operation        string
	OperationVersion int
	Peer             Peer
	StartedAt        time.Time
	Duration         time.Duration
	Success          bool
	ErrorCode        string
	RequestBytes     int
	ResponseBytes    int
}

type AuditHook func(AuditEvent)

type ServerConfig struct {
	AllowedUID          uint32
	PeerResolver        PeerResolver
	Registry            *Registry
	MaxOperationTimeout time.Duration
	Audit               AuditHook
	Now                 func() time.Time
}

type Server struct {
	allowedUID          uint32
	peerResolver        PeerResolver
	registry            *Registry
	maxOperationTimeout time.Duration
	audit               AuditHook
	now                 func() time.Time
}

func NewServer(config ServerConfig) (*Server, error) {
	if config.PeerResolver == nil {
		return nil, errors.New("peer resolver is required")
	}
	if config.Registry == nil {
		return nil, errors.New("operation registry is required")
	}
	maxTimeout := config.MaxOperationTimeout
	if maxTimeout <= 0 {
		maxTimeout = defaultMaxOperationTimeout
	}
	if maxTimeout < time.Millisecond {
		return nil, errors.New("maximum operation timeout must be at least 1ms")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Server{
		allowedUID:          config.AllowedUID,
		peerResolver:        config.PeerResolver,
		registry:            config.Registry,
		maxOperationTimeout: maxTimeout,
		audit:               config.Audit,
		now:                 now,
	}, nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				continue
			}
			return fmt.Errorf("accept helper connection: %w", err)
		}
		go s.HandleConnection(ctx, conn)
	}
}

func (s *Server) HandleConnection(parent context.Context, conn net.Conn) {
	defer conn.Close()
	started := s.now()
	event := AuditEvent{StartedAt: started}
	defer func() {
		event.Duration = s.now().Sub(started)
		if s.audit != nil {
			s.audit(event)
		}
	}()

	_ = conn.SetDeadline(started.Add(s.maxOperationTimeout))
	peer, err := s.peerResolver.Resolve(conn)
	event.Peer = peer
	if err != nil || peer.UID != s.allowedUID {
		s.writeError(conn, &event, Request{}, ErrorUnauthorizedPeer, "local peer is not authorized", false)
		return
	}

	frame, err := readFrame(conn, MaxRequestBytes)
	if err != nil {
		s.writeError(conn, &event, Request{}, ErrorInvalidFrame, "invalid request frame", false)
		return
	}
	event.RequestBytes = len(frame)

	request, err := decodeRequest(frame)
	if err != nil {
		s.writeError(conn, &event, Request{}, ErrorInvalidRequest, "invalid request envelope: "+err.Error(), false)
		return
	}
	event.RequestID = request.RequestID
	event.Operation = request.Operation
	event.OperationVersion = request.OperationVersion

	if validationError := s.validateRequest(request); validationError != nil {
		s.writeResponse(conn, &event, errorResponse(request, validationError))
		return
	}

	operation, ok := s.registry.lookup(request.Operation, request.OperationVersion)
	if !ok {
		if s.registry.hasName(request.Operation) {
			s.writeError(conn, &event, request, ErrorUnsupportedOperation, "operation version is not supported", false)
		} else {
			s.writeError(conn, &event, request, ErrorUnknownOperation, "operation is not supported", false)
		}
		return
	}

	operationContext, cancel := context.WithTimeout(parent, time.Duration(request.DeadlineMillis)*time.Millisecond)
	defer cancel()
	result, operationError := operation.handle(operationContext, request.Payload)
	if operationError == nil && operationContext.Err() != nil {
		operationError = &ResponseError{Code: ErrorDeadlineExceeded, Message: "operation deadline exceeded", Retryable: true}
	}
	if operationError != nil && errors.Is(operationContext.Err(), context.DeadlineExceeded) {
		operationError = &ResponseError{Code: ErrorDeadlineExceeded, Message: "operation deadline exceeded", Retryable: true}
	}

	response := Response{
		ProtocolVersion:  ProtocolVersion,
		RequestID:        request.RequestID,
		Operation:        request.Operation,
		OperationVersion: request.OperationVersion,
		Success:          operationError == nil,
		Result:           result,
		Error:            operationError,
	}
	s.writeResponse(conn, &event, response)
}

func (s *Server) validateRequest(request Request) *ResponseError {
	if request.ProtocolVersion != ProtocolVersion {
		return &ResponseError{Code: ErrorUnsupportedProtocol, Message: "protocol version is not supported"}
	}
	if !validRequestID(request.RequestID) {
		return &ResponseError{Code: ErrorInvalidRequest, Message: "requestId must contain 1 to 128 safe characters"}
	}
	if strings.TrimSpace(request.Operation) == "" {
		return &ResponseError{Code: ErrorInvalidRequest, Message: "operation is required"}
	}
	if request.OperationVersion < 1 {
		return &ResponseError{Code: ErrorInvalidRequest, Message: "operationVersion must be positive"}
	}
	if request.DeadlineMillis < 1 || request.DeadlineMillis > s.maxOperationTimeout.Milliseconds() {
		return &ResponseError{Code: ErrorInvalidRequest, Message: "deadlineMillis is outside the allowed range"}
	}
	return nil
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.' || char == ':' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) writeError(conn net.Conn, event *AuditEvent, request Request, code, message string, retryable bool) {
	s.writeResponse(conn, event, errorResponse(request, &ResponseError{Code: code, Message: message, Retryable: retryable}))
}

func errorResponse(request Request, responseError *ResponseError) Response {
	return Response{
		ProtocolVersion:  ProtocolVersion,
		RequestID:        request.RequestID,
		Operation:        request.Operation,
		OperationVersion: request.OperationVersion,
		Success:          false,
		Error:            responseError,
	}
}

func (s *Server) writeResponse(conn net.Conn, event *AuditEvent, response Response) {
	_ = conn.SetWriteDeadline(s.now().Add(responseWriteTimeout))
	framed, err := marshalFrame(response, MaxResponseBytes)
	if err != nil {
		response = errorResponse(Request{
			RequestID:        response.RequestID,
			Operation:        response.Operation,
			OperationVersion: response.OperationVersion,
		}, &ResponseError{Code: ErrorResponseTooLarge, Message: "operation response exceeds the allowed size"})
		framed, err = marshalFrame(response, MaxResponseBytes)
		if err != nil {
			event.Success = false
			event.ErrorCode = ErrorInternal
			return
		}
	}

	written, writeErr := writeAll(conn, framed)
	event.ResponseBytes = max(written-4, 0)
	event.Success = response.Success && writeErr == nil
	if response.Error != nil {
		event.ErrorCode = response.Error.Code
	} else if writeErr != nil {
		event.ErrorCode = ErrorInternal
	}
}

// DecodeResponse is the client-side strict response decoder.
func DecodeResponse(data []byte) (Response, error) {
	var response Response
	if err := decodeStrict(data, &response); err != nil {
		return Response{}, err
	}
	return response, nil
}

// EncodeRequestFrame is exposed for the collector-side client package that
// will be wired in a later bounded slice.
func EncodeRequestFrame(request Request) ([]byte, error) {
	return marshalFrame(request, MaxRequestBytes)
}

// ReadResponseFrame reads one bounded helper response.
func ReadResponseFrame(reader interface{ Read([]byte) (int, error) }) ([]byte, error) {
	return readFrame(reader, MaxResponseBytes)
}
