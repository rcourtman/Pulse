package api

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiError APIError
		want     string
	}{
		{
			name:     "simple error message",
			apiError: APIError{ErrorMessage: "something went wrong"},
			want:     "something went wrong",
		},
		{
			name:     "empty error message",
			apiError: APIError{ErrorMessage: ""},
			want:     "",
		},
		{
			name: "error with all fields",
			apiError: APIError{
				ErrorMessage: "unauthorized",
				Code:         "AUTH_FAILED",
				StatusCode:   401,
				Timestamp:    1234567890,
				RequestID:    "req-123",
				Details:      map[string]string{"reason": "invalid token"},
			},
			want: "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.apiError.Error()
			if got != tt.want {
				t.Errorf("APIError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIError_ImplementsError(t *testing.T) {
	var _ error = &APIError{}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name               string
		codes              []int
		wantStatusCode     int
		wantWrittenCount   int
		wantUnderlyingCode int
	}{
		{
			name:               "single write",
			codes:              []int{http.StatusOK},
			wantStatusCode:     http.StatusOK,
			wantWrittenCount:   1,
			wantUnderlyingCode: http.StatusOK,
		},
		{
			name:               "first write wins",
			codes:              []int{http.StatusCreated, http.StatusBadRequest, http.StatusInternalServerError},
			wantStatusCode:     http.StatusCreated,
			wantWrittenCount:   1,
			wantUnderlyingCode: http.StatusCreated,
		},
		{
			name:               "error code",
			codes:              []int{http.StatusNotFound},
			wantStatusCode:     http.StatusNotFound,
			wantWrittenCount:   1,
			wantUnderlyingCode: http.StatusNotFound,
		},
		{
			name:               "server error",
			codes:              []int{http.StatusInternalServerError},
			wantStatusCode:     http.StatusInternalServerError,
			wantWrittenCount:   1,
			wantUnderlyingCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

			for _, code := range tt.codes {
				rw.WriteHeader(code)
			}

			if rw.statusCode != tt.wantStatusCode {
				t.Errorf("statusCode = %d, want %d", rw.statusCode, tt.wantStatusCode)
			}

			if rec.Code != tt.wantUnderlyingCode {
				t.Errorf("underlying Code = %d, want %d", rec.Code, tt.wantUnderlyingCode)
			}
		})
	}
}

func TestResponseWriter_WriteHeader_WrittenFlag(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	if rw.written {
		t.Error("written should be false initially")
	}

	rw.WriteHeader(http.StatusCreated)

	if !rw.written {
		t.Error("written should be true after WriteHeader")
	}
}

func TestResponseWriter_Write(t *testing.T) {
	tests := []struct {
		name               string
		preWriteHeader     bool
		preWriteHeaderCode int
		writeData          []byte
		wantStatusCode     int
		wantWritten        bool
	}{
		{
			name:           "write without prior WriteHeader",
			preWriteHeader: false,
			writeData:      []byte("hello"),
			wantStatusCode: http.StatusOK,
			wantWritten:    true,
		},
		{
			name:               "write with prior WriteHeader",
			preWriteHeader:     true,
			preWriteHeaderCode: http.StatusCreated,
			writeData:          []byte("created"),
			wantStatusCode:     http.StatusCreated,
			wantWritten:        true,
		},
		{
			name:           "empty write without prior WriteHeader",
			preWriteHeader: false,
			writeData:      []byte{},
			wantStatusCode: http.StatusOK,
			wantWritten:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

			if tt.preWriteHeader {
				rw.WriteHeader(tt.preWriteHeaderCode)
			}

			n, err := rw.Write(tt.writeData)
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			if n != len(tt.writeData) {
				t.Errorf("Write() = %d bytes, want %d", n, len(tt.writeData))
			}

			if rw.statusCode != tt.wantStatusCode {
				t.Errorf("statusCode = %d, want %d", rw.statusCode, tt.wantStatusCode)
			}

			if rw.written != tt.wantWritten {
				t.Errorf("written = %v, want %v", rw.written, tt.wantWritten)
			}

			if string(rec.Body.Bytes()) != string(tt.writeData) {
				t.Errorf("body = %q, want %q", rec.Body.String(), string(tt.writeData))
			}
		})
	}
}

func TestResponseWriter_Write_MultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// First write triggers implicit WriteHeader
	_, err := rw.Write([]byte("first"))
	if err != nil {
		t.Fatalf("first Write() error = %v", err)
	}

	// Second write should not change status
	_, err = rw.Write([]byte(" second"))
	if err != nil {
		t.Fatalf("second Write() error = %v", err)
	}

	if rw.statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusOK)
	}

	if rec.Body.String() != "first second" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "first second")
	}
}

func TestResponseWriter_StatusCode(t *testing.T) {
	tests := []struct {
		name           string
		rw             *responseWriter
		wantStatusCode int
	}{
		{
			name:           "nil receiver",
			rw:             nil,
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "default status",
			rw:             &responseWriter{statusCode: http.StatusOK},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "custom status",
			rw:             &responseWriter{statusCode: http.StatusNotFound},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "server error status",
			rw:             &responseWriter{statusCode: http.StatusBadGateway},
			wantStatusCode: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rw.StatusCode()
			if got != tt.wantStatusCode {
				t.Errorf("StatusCode() = %d, want %d", got, tt.wantStatusCode)
			}
		})
	}
}

// mockHijacker implements http.Hijacker for testing
type mockHijacker struct {
	http.ResponseWriter
	conn net.Conn
	rw   *bufio.ReadWriter
	err  error
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return m.conn, m.rw, m.err
}

func TestResponseWriter_Hijack_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	conn, brw, err := rw.Hijack()
	if err == nil {
		t.Error("Hijack() should return error when underlying writer doesn't support it")
	}
	if conn != nil {
		t.Error("Hijack() should return nil conn on error")
	}
	if brw != nil {
		t.Error("Hijack() should return nil bufio.ReadWriter on error")
	}
	if err.Error() != "ResponseWriter does not implement http.Hijacker" {
		t.Errorf("Hijack() error = %q, want specific error message", err.Error())
	}
}

func TestResponseWriter_Hijack_Supported(t *testing.T) {
	// Create a mock hijacker that supports hijacking
	mockConn := &net.TCPConn{}
	mockRW := bufio.NewReadWriter(bufio.NewReader(nil), bufio.NewWriter(nil))
	hijacker := &mockHijacker{
		ResponseWriter: httptest.NewRecorder(),
		conn:           mockConn,
		rw:             mockRW,
		err:            nil,
	}

	rw := &responseWriter{ResponseWriter: hijacker, statusCode: http.StatusOK}

	conn, brw, err := rw.Hijack()
	if err != nil {
		t.Errorf("Hijack() error = %v, want nil", err)
	}
	if conn != mockConn {
		t.Error("Hijack() returned unexpected conn")
	}
	if brw != mockRW {
		t.Error("Hijack() returned unexpected bufio.ReadWriter")
	}
}

// mockFlusher implements http.Flusher for testing
type mockFlusher struct {
	http.ResponseWriter
	flushed bool
}

func (m *mockFlusher) Flush() {
	m.flushed = true
}

func TestResponseWriter_Flush_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// This should not panic even though underlying doesn't support Flush
	rw.Flush()
}

func TestResponseWriter_Flush_Supported(t *testing.T) {
	flusher := &mockFlusher{ResponseWriter: httptest.NewRecorder()}
	rw := &responseWriter{ResponseWriter: flusher, statusCode: http.StatusOK}

	if flusher.flushed {
		t.Error("flushed should be false initially")
	}

	rw.Flush()

	if !flusher.flushed {
		t.Error("Flush() should call underlying Flusher.Flush()")
	}
}

// Note: httptest.ResponseRecorder implements http.Flusher
func TestResponseWriter_Flush_WithRecorder(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write some data
	rw.Write([]byte("test"))

	// Flush should work with httptest.ResponseRecorder (it implements Flusher)
	rw.Flush()

	// If we got here without panic, the test passes
}

func TestResponseWriter_Header(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Set a header through the wrapper
	rw.Header().Set("X-Custom-Header", "test-value")

	// Verify it was set on the underlying writer
	if got := rec.Header().Get("X-Custom-Header"); got != "test-value" {
		t.Errorf("Header().Get() = %q, want %q", got, "test-value")
	}
}

func TestResponseWriter_FullFlow(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Set headers
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("X-Request-ID", "test-123")

	// Write status
	rw.WriteHeader(http.StatusCreated)

	// Write body
	n, err := rw.Write([]byte(`{"status":"created"}`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 20 {
		t.Errorf("Write() = %d bytes, want 20", n)
	}

	// Verify all values
	if rw.StatusCode() != http.StatusCreated {
		t.Errorf("StatusCode() = %d, want %d", rw.StatusCode(), http.StatusCreated)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("rec.Code = %d, want %d", rec.Code, http.StatusCreated)
	}
	if rec.Body.String() != `{"status":"created"}` {
		t.Errorf("body = %q, want %q", rec.Body.String(), `{"status":"created"}`)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
	}
}

func TestResponseWriter_EdgeCases(t *testing.T) {
	t.Run("zero status code preserved", func(t *testing.T) {
		rw := &responseWriter{statusCode: 0}
		if rw.StatusCode() != 0 {
			t.Errorf("StatusCode() = %d, want 0", rw.StatusCode())
		}
	})

	t.Run("write after write maintains written flag", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.Write([]byte("a"))
		if !rw.written {
			t.Error("written should be true after first Write")
		}

		rw.Write([]byte("b"))
		if !rw.written {
			t.Error("written should remain true after second Write")
		}
	})

	t.Run("WriteHeader after Write is no-op", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.Write([]byte("data"))
		rw.WriteHeader(http.StatusNotFound)

		// Status should remain 200 since Write triggered implicit WriteHeader
		if rw.statusCode != http.StatusOK {
			t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusOK)
		}
	})
}

func TestErrorHandler_EmptyPath(t *testing.T) {
	// Test that empty path is normalized to "/" (fix for issue #334)
	var capturedPath string
	handler := ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.URL.Path = "" // explicitly set empty path
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedPath != "/" {
		t.Errorf("expected empty path to be normalized to '/', got %q", capturedPath)
	}
}
