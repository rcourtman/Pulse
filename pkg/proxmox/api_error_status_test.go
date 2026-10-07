package proxmox

import (
	"errors"
	"fmt"
	"testing"
)

func TestAPIErrorStatusUsesWireEvidenceNotBody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		known  bool
	}{
		{"wire-401", fmt.Errorf("wrapped: %w", &apiResponseError{statusCode: 401, cause: errors.New("API error 403: quoted body")}), 401, true},
		{"wire-403", &apiResponseError{statusCode: 403, cause: errors.New("body says 401")}, 403, true},
		{"proxy-502", &apiResponseError{statusCode: 502, cause: errors.New("API error 403: permission denied")}, 502, true},
		{"login-401", &authHTTPError{status: 401, body: "untrusted 403"}, 401, true},
		{"text-only", errors.New("403 permission denied"), 0, false},
		{"nil", nil, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, known := APIErrorStatus(tc.err)
			if status != tc.status || known != tc.known {
				t.Fatalf("status/known=%d/%v want %d/%v", status, known, tc.status, tc.known)
			}
		})
	}
}
