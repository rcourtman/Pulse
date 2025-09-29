package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog/log"
)

// APIError represents a structured API error response
type APIError struct {
	ErrorMessage string            `json:"error"`
	Code         string            `json:"code,omitempty"`
	StatusCode   int               `json:"status_code"`
	Timestamp    int64             `json:"timestamp"`
	RequestID    string            `json:"request_id,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.ErrorMessage
}

// ErrorHandler is a middleware that handles panics and errors
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fix for issue #334: Normalize empty path to "/" before ServeMux processes it
		// This prevents the automatic redirect from "" to "./"
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}

		// Skip error handling for WebSocket endpoints
		if r.Header.Get("Upgrade") == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		// Create a custom response writer to capture status codes
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Recover from panics
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Interface("error", err).
					Str("path", r.URL.Path).
					Str("method", r.Method).
					Bytes("stack", debug.Stack()).
					Msg("Panic recovered in API handler")

				writeErrorResponse(w, http.StatusInternalServerError, "internal_error",
					"An unexpected error occurred", nil)
			}
		}()

		// Add request ID to context
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Log errors (4xx and 5xx)
		if rw.statusCode >= 400 {
			log.Warn().
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Int("status", rw.statusCode).
				Str("request_id", requestID).
				Msg("Request failed")
		}
	})
}

// TimeoutHandler wraps handlers with a timeout
func TimeoutHandler(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip timeout for WebSocket and SSE endpoints
			if r.Header.Get("Upgrade") == "websocket" || r.Header.Get("Accept") == "text/event-stream" {
				next.ServeHTTP(w, r)
				return
			}

			http.TimeoutHandler(next, timeout, "Request timeout").ServeHTTP(w, r)
		})
	}
}

// JSONHandler ensures proper JSON responses and error handling
func JSONHandler(handler func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := handler(w, r); err != nil {
			// Check if it's already an APIError
			if apiErr, ok := err.(*APIError); ok {
				writeErrorResponse(w, apiErr.StatusCode, apiErr.Code, apiErr.ErrorMessage, apiErr.Details)
				return
			}

			// Generic error
			log.Error().Err(err).
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Msg("Handler error")

			writeErrorResponse(w, http.StatusInternalServerError, "internal_error",
				"An error occurred processing the request", nil)
		}
	}
}

// writeErrorResponse writes a consistent error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIError{
		ErrorMessage: message,
		Code:         code,
		StatusCode:   statusCode,
		Timestamp:    time.Now().Unix(),
		Details:      details,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode error response")
	}
}

// responseWriter wraps http.ResponseWriter to capture status codes
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker interface
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

// NewAPIError creates a new API error
func NewAPIError(statusCode int, code, message string) error {
	return &APIError{
		ErrorMessage: message,
		Code:         code,
		StatusCode:   statusCode,
		Timestamp:    time.Now().Unix(),
	}
}

// ValidationError creates a validation error with field details
func ValidationError(fields map[string]string) error {
	return &APIError{
		ErrorMessage: "Validation failed",
		Code:         "validation_error",
		StatusCode:   http.StatusBadRequest,
		Timestamp:    time.Now().Unix(),
		Details:      fields,
	}
}
