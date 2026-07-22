package updates

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
)

type branchcov0722R2NetError struct {
	message   string
	timeout   bool
	temporary bool
}

func (e *branchcov0722R2NetError) Error() string   { return e.message }
func (e *branchcov0722R2NetError) Timeout() bool   { return e.timeout }
func (e *branchcov0722R2NetError) Temporary() bool { return e.temporary }

var _ net.Error = (*branchcov0722R2NetError)(nil)

func TestBranchcov0722R2IsRetryableUpdateRequestError(t *testing.T) {
	wrappedCanceled := fmt.Errorf("upstream cancelled: %w", context.Canceled)
	wrappedDeadline := fmt.Errorf("upstream deadline: %w", context.DeadlineExceeded)
	wrappedEOF := fmt.Errorf("read failed: %w", io.EOF)
	wrappedUnexpectedEOF := fmt.Errorf("short read: %w", io.ErrUnexpectedEOF)
	wrappedNetErr := fmt.Errorf("dial failed: %w", &branchcov0722R2NetError{message: "i/o timeout", timeout: true})

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error returns false", nil, false},
		{"context canceled directly returns false", context.Canceled, false},
		{"context deadline exceeded directly returns false", context.DeadlineExceeded, false},
		{"context canceled wrapped one level with %w still returns false", wrappedCanceled, false},
		{"context deadline exceeded wrapped one level with %w still returns false", wrappedDeadline, false},
		{"net.Error implementing timeout returns true", &branchcov0722R2NetError{message: "dial tcp: i/o timeout", timeout: true, temporary: true}, true},
		{"net.Error implementing only temporary returns true", &branchcov0722R2NetError{message: "dial tcp: temporary", timeout: false, temporary: true}, true},
		{"net.Error wrapped one level with %w still returns true via errors.As", wrappedNetErr, true},
		{"io.EOF directly returns true", io.EOF, true},
		{"io.ErrUnexpectedEOF directly returns true", io.ErrUnexpectedEOF, true},
		{"io.EOF wrapped one level with %w returns true", wrappedEOF, true},
		{"io.ErrUnexpectedEOF wrapped one level with %w returns true", wrappedUnexpectedEOF, true},
		{"string fragment connection reset returns true", errors.New("read tcp: connection reset by peer"), true},
		{"string fragment connection refused returns true", errors.New("dial tcp: connection refused"), true},
		{"string fragment connection aborted returns true", errors.New("write tcp: connection aborted"), true},
		{"string fragment broken pipe returns true", errors.New("write tcp: broken pipe"), true},
		{"string fragment temporary failure returns true", errors.New("temporary failure in name resolution"), true},
		{"string fragment timeout returns true", errors.New("operation timeout reached"), true},
		{"tls handshake message matches the earlier timeout fragment returns true", errors.New("net/http: TLS handshake timeout"), true},
		{"string fragment http2 server sent goaway returns true", errors.New("http2: server sent GOAWAY and closed the connection"), true},
		{"mixed case fragment lowercased before matching returns true", errors.New("CONNECTION REFUSED by host"), true},
		{"plain unrelated error returns false", errors.New("something completely different happened"), false},
		{"unrelated error with no matching fragment returns false", errors.New("permission denied"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryableUpdateRequestError(tc.err)
			if got != tc.want {
				t.Fatalf("isRetryableUpdateRequestError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
