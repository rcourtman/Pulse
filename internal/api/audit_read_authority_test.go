package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

func TestAuditReadCurrentAuthorityAndPrefix(t *testing.T) {
	responseCurrentAuthorityAndPrefix(t, config.ScopeAuditRead, responseMediaAudit)
}

func TestAuditReadKeepsOrganizationBoundAuthority(t *testing.T) {
	responseKeepsOrganizationBoundAuthority(t, config.ScopeAuditRead, responseMediaAudit)
}

// Optional endpoint replacements receive the same disclosure boundary as the
// public adapter, and cannot widen audit JSON/CSV to executable media.
type syntheticAuditReadEndpoints struct {
	extensions.AuditAdminEndpoints
	respond http.HandlerFunc
}

func (e syntheticAuditReadEndpoints) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	e.respond(w, r)
}
func (e syntheticAuditReadEndpoints) HandleVerifyEvent(w http.ResponseWriter, r *http.Request) {
	e.respond(w, r)
}
func (e syntheticAuditReadEndpoints) HandleExportEvents(w http.ResponseWriter, r *http.Request) {
	e.respond(w, r)
}
func (e syntheticAuditReadEndpoints) HandleSummary(w http.ResponseWriter, r *http.Request) {
	e.respond(w, r)
}

func TestAuditReadExtensionBoundary(t *testing.T) {
	for _, stage := range []string{"authorised", "withdrawn", "prefix"} {
		for _, tc := range auditReadCases()[:6] {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				var cfg *config.Config
				SetAuditAdminEndpointsBinder(func(defaults extensions.AuditAdminEndpoints, _ extensions.AuditAdminRuntime) extensions.AuditAdminEndpoints {
					return syntheticAuditReadEndpoints{AuditAdminEndpoints: defaults, respond: func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "text/html")
						w.Header().Set("Content-Disposition", "attachment; filename=synthetic-private.html")
						w.Header().Set("X-Event-Count", "7")
						if stage == "prefix" {
							if _, err := w.Write([]byte("admitted-prefix\n")); err != nil {
								t.Fatal(err)
							}
						}
						if stage != "authorised" {
							config.Mu.Lock()
							cfg.APITokens = cfg.APITokens[1:]
							config.Mu.Unlock()
						}
						n, err := w.Write([]byte("<script>synthetic extension evidence</script>"))
						if stage == "authorised" {
							if err != nil || n == 0 {
								t.Fatal("authorised extension response lost")
							}
						} else if n != 0 || err != io.ErrClosedPipe {
							t.Fatalf("withdrawn extension write=%d %v", n, err)
						}
						if r.Context().Err() != nil {
							t.Fatal("withdrawal cancelled storage work")
						}
					}}
				})
				t.Cleanup(func() { SetAuditAdminEndpointsBinder(nil) })
				router, configValue, server, _ := newHeldAuditReadRouter(t)
				cfg = configValue
				req := auditReadRequest(t, context.Background(), server, tc.path, logStreamToken, "header")
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				switch stage {
				case "authorised":
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "synthetic extension evidence") {
						t.Fatal("extension body changed")
					}
					fallthrough
				case "prefix":
					if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
						t.Fatal("extension widened non-executable audit media")
					}
					if stage == "prefix" && (rec.Code != http.StatusOK || rec.Body.String() != "admitted-prefix\n") {
						t.Fatal("withdrawal appended or disclosed after admitted prefix")
					}
				default:
					if rec.Code != http.StatusUnauthorized || rec.Body.String() != "Authentication required\n" || rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("X-Event-Count") != "" {
						t.Fatal("withdrawn extension escaped generic refusal")
					}
				}
			})
		}
	}
}
