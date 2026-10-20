package pbs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Synthetic provider failures exercise actual authentication and API readers.
// No PBS listener, external credential or guest operation is used. Never print
// a failing payload: size/privacy counters are sufficient for the baseline.
func testPBSFailureDiagnostics(t *testing.T) {
	priorLogger, priorLevel := log.Logger, zerolog.GlobalLevel()
	defer func() { log.Logger = priorLogger; zerolog.SetGlobalLevel(priorLevel) }()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	const marker = "synthetic-private-response-"
	for _, mode := range []string{"api", "auth"} {
		for _, status := range []int{400, 401, 403, 404, 409, 415, 429, 500, 503, 599} {
			for _, shape := range []string{"json", "html", "oversized", "read-error"} {
				t.Run(fmt.Sprintf("%s/%d/%s", mode, status, shape), func(t *testing.T) {
					body := `{"errors":{"message":"` + marker + strings.Repeat("x", 1024) + `"},"data":{"ticket":"` + marker + `ticket","value":"` + marker + `token"}}`
					if shape == "html" {
						body = "<html>" + marker + strings.Repeat("x", 1<<20) + "</html>"
					}
					if shape == "oversized" {
						body = marker + strings.Repeat("x", int(maxResponseBodyBytes))
					}
					var captured bytes.Buffer
					log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
					client, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "fixture@pbs!pulse", TokenValue: "fixture-token"})
					if err != nil {
						t.Fatal(err)
					}
					calls, closes := 0, 0
					client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
						calls++
						wantPath, wantMethod := "/api2/json/version", http.MethodGet
						if mode == "auth" {
							wantPath, wantMethod = "/api2/json/access/ticket", http.MethodPost
						}
						if r.URL.Path != wantPath || r.Method != wantMethod {
							t.Error("diagnostic handling changed the request")
						}
						b := &privacyDiagnosticBody{Reader: strings.NewReader(body), closes: &closes}
						if shape == "read-error" {
							b.tail = errors.New(marker + "reader")
						}
						return &http.Response{StatusCode: status, Header: make(http.Header), Body: b, Request: r}, nil
					})
					if mode == "auth" {
						client.config.Password = "fixture-password"
						client.auth.tokenName, client.auth.tokenValue = "", ""
						err = client.authenticate(context.Background())
						if client.authSnapshot().ticket != "" {
							t.Error("refused auth published a session")
						}
					} else {
						_, err = client.GetVersion(context.Background())
					}
					if err == nil {
						t.Fatal("provider refusal became success")
					}
					if got, ok := HTTPStatus(fmt.Errorf("wrapped: %w", err)); !ok || got != status {
						t.Error("lost exact HTTP status")
					}
					wantCalls := 1
					if mode == "auth" && (status == 400 || status == 415) {
						wantCalls = 2
					}
					if calls != wantCalls || closes != wantCalls {
						t.Errorf("request/closure counts: %d/%d want %d", calls, closes, wantCalls)
					}
					diagnostic := fmt.Sprintf("%v\n%+v\n%#v", err, err, err)
					private := strings.Contains(diagnostic, marker) || strings.Contains(captured.String(), marker)
					if private {
						t.Error("provider content crossed the error/log boundary")
					}
					if len(err.Error()) > 256 || captured.Len() > 2048 {
						t.Error("provider response inflated diagnostics")
					}
					if shape == "oversized" && !strings.Contains(err.Error(), "response body exceeds") {
						t.Error("lost useful size-limit diagnostic")
					}
					t.Logf("response_bytes=%d error_bytes=%d log_bytes=%d private=%t requests=%d closed=%d", len(body), len(err.Error()), captured.Len(), private, calls, closes)
					if !t.Failed() && shape == "json" && (status == 401 || status == 403 || status == 429 || status == 503) {
						frame, _ := json.Marshal(map[string]any{"mode": mode, "status": status, "message": err.Error(), "lines": strings.Split(strings.TrimSpace(captured.String()), "\n")})
						t.Logf("PBS_ERROR_FRAME %s", base64.StdEncoding.EncodeToString(frame))
					}
				})
			}
		}
	}
}

type privacyDiagnosticBody struct {
	io.Reader
	tail   error
	closes *int
}

func (b *privacyDiagnosticBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF && b.tail != nil {
		return n, b.tail
	}
	return n, err
}
func (b *privacyDiagnosticBody) Close() error { *b.closes++; return nil }

func testPBSTokenFailureDiagnostics(t *testing.T) {
	priorLogger, priorLevel := log.Logger, zerolog.GlobalLevel()
	defer func() { log.Logger = priorLogger; zerolog.SetGlobalLevel(priorLevel) }()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	const marker = "synthetic-private-token-"
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"malformed", `{"data":{"value":"` + marker + `secret"}`, false},
		{"wrong-type", `{"data":{"value":{"private":"` + marker + `secret"}}}`, false},
		{"missing-value", `{"data":{"tokenid":"` + marker + `identity"},"private":"` + marker + `secret"}`, false},
		{"valid", `{"data":{"tokenid":"fixture@pbs!pulse","value":"` + marker + `secret"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var captured bytes.Buffer
			log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
			c, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "fixture@pbs!admin", TokenValue: "fixture-admin-token"})
			if err != nil {
				t.Fatal(err)
			}
			calls, closes := 0, 0
			c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api2/json/access/users/fixture@pbs/token/pulse" {
					t.Error("unexpected token operation")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &privacyDiagnosticBody{Reader: strings.NewReader(tc.body), closes: &closes}}, nil
			})
			token, err := c.CreateUserToken(context.Background(), "fixture@pbs", "pulse")
			if tc.valid {
				if err != nil || token == nil || token.Value != marker+"secret" || token.TokenID != "fixture@pbs!pulse" {
					t.Error("valid token changed")
				}
			} else if err == nil || token != nil {
				t.Error("malformed token response became usable")
			}
			if strings.Contains(fmt.Sprint(err), marker) || strings.Contains(captured.String(), marker) {
				t.Error("token response crossed the diagnostic boundary")
			}
			if calls != 1 || closes != 1 {
				t.Error("token diagnostic caused extra operations or leaked a body")
			}
			t.Logf("response_bytes=%d error_bytes=%d log_bytes=%d", len(tc.body), len(fmt.Sprint(err)), captured.Len())
		})
	}
}

func TestPBSCollisionClassificationDoesNotDependOnDiagnosticText(t *testing.T) {
	if isAlreadyExistsError(errors.New("transport already exists")) || isAlreadyExistsError(nil) {
		t.Fatal("non-provider error became a collision")
	}
	for _, status := range []int{400, 401, 403, 409, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "fixture@pbs!admin", TokenValue: "fixture-token"})
			if err != nil {
				t.Fatal(err)
			}
			c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("API token ALREADY EXISTS: synthetic-private-token-secret"))}, nil
			})
			_, err = c.CreateUserToken(context.Background(), "fixture@pbs", "pulse")
			want := status == 400 || status == 409 || status == 500
			if err == nil || isAlreadyExistsError(fmt.Errorf("wrapped: %w", err)) != want {
				t.Fatal("lost typed provider collision classification")
			}
			if strings.Contains(err.Error(), "synthetic-private-token-secret") {
				t.Fatal("collision text disclosed provider content")
			}
		})
	}
}
