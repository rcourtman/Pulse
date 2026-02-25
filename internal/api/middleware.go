package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
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

		// Add request ID to context, honoring any incoming header value.
		incomingID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		ctxWithID, requestID := logging.WithRequestID(r.Context(), incomingID)
		r = r.WithContext(ctxWithID)

		// Create a custom response writer to capture status codes
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		rw.Header().Set("X-Request-ID", requestID)

		start := time.Now()
		routeLabel := normalizeRoute(r.URL.Path)
		method := r.Method

		defer func() {
			elapsed := time.Since(start)
			recordAPIRequest(method, routeLabel, rw.StatusCode(), elapsed)
		}()

		// Recover from panics
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Interface("error", err).
					Str("path", r.URL.Path).
					Str("method", r.Method).
					Str("request_id", requestID).
					Bytes("stack", debug.Stack()).
					Msg("Panic recovered in API handler")

				writeErrorResponse(rw, http.StatusInternalServerError, "internal_error",
					"An unexpected error occurred", nil)
			}
		}()

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

// sanitizeErrorForClient returns a generic, safe message for an internal error.
// The raw error is logged server-side; the client only sees the generic message.
// Use this instead of passing err.Error() to http.Error or writeErrorResponse.
func sanitizeErrorForClient(err error, genericMsg string) string {
	if err != nil {
		log.Error().Err(err).Msg(genericMsg)
	}
	return genericMsg
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

func (rw *responseWriter) StatusCode() int {
	if rw == nil {
		return http.StatusInternalServerError
	}
	return rw.statusCode
}

// Hijack implements http.Hijacker interface
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

// Flush implements http.Flusher when the underlying writer supports it.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
