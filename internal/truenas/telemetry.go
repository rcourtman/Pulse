package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var errNoSystemTelemetry = errors.New("truenas legacy REST reporting returned no system telemetry")

func systemTelemetryAvailability(system SystemInfo) SystemTelemetryAvailability {
	if system.Telemetry != nil {
		return *system.Telemetry
	}
	// Compatibility for static snapshots written before per-metric presence.
	// GetSystemInfo and both live telemetry parsers always set Telemetry, so
	// their collection timestamps never imply that missing samples were zero.
	hasTelemetry := !system.CollectedAt.IsZero() || system.IntervalSeconds > 0
	return SystemTelemetryAvailability{
		CPU: hasTelemetry, Memory: system.MemoryAvailableBytes > 0,
		NetIn: hasTelemetry, NetOut: hasTelemetry,
		DiskRead: hasTelemetry, DiskWrite: hasTelemetry,
	}
}

func systemTelemetryFailure(err error) *SystemTelemetryAvailability {
	availability := &SystemTelemetryAvailability{ErrorCategory: "collection_error"}
	var apiErr *APIError
	var authErr *RPCAuthError
	var rpcErr *RPCError
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.Is(err, context.Canceled):
		availability.ErrorCategory = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		availability.ErrorCategory = "timeout"
	case errors.Is(err, errNoSystemTelemetry):
		availability.ErrorCategory = "no_samples"
	case errors.As(err, &apiErr):
		availability.HTTPStatus = apiErr.StatusCode
		switch apiErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			availability.ErrorCategory = "authentication"
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			availability.ErrorCategory = "unsupported_endpoint"
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			availability.ErrorCategory = "request_rejected"
		case http.StatusTooManyRequests:
			availability.ErrorCategory = "rate_limited"
		default:
			availability.ErrorCategory = "http_error"
		}
	case errors.As(err, &authErr):
		availability.ErrorCategory = "authentication"
	case errors.As(err, &rpcErr):
		availability.ErrorCategory = "method_error"
	case errors.As(err, &syntaxErr), errors.As(err, &typeErr), errors.Is(err, io.ErrUnexpectedEOF):
		availability.ErrorCategory = "invalid_response"
	}
	return availability
}
