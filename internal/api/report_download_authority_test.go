package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestReportDownloadMediaPoliciesStaySeparate(t *testing.T) {
	for _, policy := range []responseMediaPolicy{responseMediaJSON, responseMediaZIP, responseMediaReport, responseMediaAudit, responseMediaPolicy(255)} {
		for _, provided := range []string{"", "application/json", "application/zip", "application/pdf", "text/csv", "text/csv; charset=utf-8", "text/csv; charset=windows-1252", "text/html", "image/svg+xml", "application/json; invalid"} {
			t.Run(fmt.Sprint(policy)+"/"+provided, func(t *testing.T) {
				base := httptest.NewRecorder()
				w := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return true }, media: policy}
				w.Header()["content-type"] = []string{"text/html"}
				w.Header()["x-content-type-options"] = []string{"sniff"}
				w.Header().Set("Content-Type", provided)
				const payload = "<script>synthetic data, never executable</script>"
				_, err := w.Write([]byte(payload))
				if err != nil {
					t.Fatal(err)
				}
				want := "text/plain; charset=utf-8"
				switch policy {
				case responseMediaJSON:
					if provided == "application/json" {
						want = provided
					}
				case responseMediaZIP:
					if provided == "application/zip" {
						want = provided
					}
				case responseMediaAudit:
					switch {
					case provided == "application/json":
						want = provided
					case strings.HasPrefix(provided, "text/csv"):
						want = "text/csv; charset=utf-8"
					}
				case responseMediaReport:
					switch {
					case provided == "application/json", provided == "application/pdf":
						want = provided
					case strings.HasPrefix(provided, "text/csv"):
						want = "text/csv; charset=utf-8"
					}
				}
				if base.Header().Get("Content-Type") != want || base.Header().Get("X-Content-Type-Options") != "nosniff" || base.Body.String() != payload {
					t.Fatalf("media or payload changed: %v", base.Header())
				}
				for _, alias := range []string{"content-type", "x-content-type-options"} {
					if _, ok := base.Header()[alias]; ok {
						t.Fatal("noncanonical media header escaped")
					}
				}
			})
		}
	}
}

func TestReportDownloadCurrentAuthorityAndPrefix(t *testing.T) {
	responseCurrentAuthorityAndPrefix(t, config.ScopeSettingsRead, responseMediaReport)
}

func responseCurrentAuthorityAndPrefix(t *testing.T, scope string, media responseMediaPolicy) {
	for _, transport := range []string{"header", "bearer", "session"} {
		changes := []string{"revoked", "expired", "scope", "org", "identity", "principal", "unrelated_scope"}
		if transport == "session" {
			changes = []string{"invalidated", "expired", "principal", "replaced", "recovery"}
		}
		for _, change := range changes {
			for _, prefix := range []bool{false, true} {
				name := change + "/before_output"
				if prefix {
					name = change + "/after_prefix"
				}
				t.Run(transport+"/"+name, func(t *testing.T) {
					cfg := newTestConfigWithTokens(t, newTokenRecord(t, logStreamToken, []string{scope, config.ScopeAIExecute}, nil))
					const session = "synthetic-report-prefix-session"
					if transport == "session" {
						InitSessionStore(t.TempDir())
						GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", "operator")
						t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
					}
					req := httptest.NewRequest(http.MethodGet, "/synthetic-report", nil)
					switch transport {
					case "bearer":
						req.Header.Set("Authorization", "Bearer "+logStreamToken)
					case "session":
						req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
					default:
						req.Header.Set("X-API-Token", logStreamToken)
					}
					base := httptest.NewRecorder()
					var late http.ResponseWriter
					guard := retainResponseAuthority(cfg, scope, false, media, func(w http.ResponseWriter, r *http.Request) {
						late = w
						w.Header().Set("Content-Type", "text/csv; charset=utf-8")
						w.Header().Set("Content-Disposition", "attachment; filename=synthetic.csv")
						if prefix {
							if _, err := w.Write([]byte("admitted,prefix\n")); err != nil {
								t.Fatal(err)
							}
						}
						if transport == "session" {
							mutateReadSession(session, change)
						} else {
							config.Mu.Lock()
							switch change {
							case "revoked":
								cfg.RemoveAPIToken(cfg.APITokens[0].ID)
							case "expired":
								past := time.Now().Add(-time.Second)
								cfg.APITokens[0].ExpiresAt = &past
							case "scope":
								cfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
							case "org":
								cfg.APITokens[0].OrgID = "different-organization"
							case "identity":
								cfg.APITokens[0].ID = "different-credential"
							case "principal":
								cfg.APITokens[0].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "different-owner"}
							case "unrelated_scope":
								cfg.APITokens[0].Scopes = []string{scope}
							}
							config.Mu.Unlock()
						}
						n, err := w.Write([]byte("private,result\n"))
						if change == "unrelated_scope" {
							if err != nil || n == 0 {
								t.Fatal("unrelated scope removal broke report read")
							}
						} else if n != 0 || err != io.ErrClosedPipe {
							t.Fatalf("withdrawn write=%d %v", n, err)
						}
						if r.Context().Err() != nil {
							t.Fatal("withdrawal cancelled independent generation")
						}
					})
					AuthContextMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if !CheckAuth(cfg, w, r) {
							t.Fatal("synthetic admission failed")
						}
						guard(w, r)
					})).ServeHTTP(base, req)
					if _, err := late.Write([]byte("late output")); err != io.ErrClosedPipe {
						t.Fatal("returned writer reopened")
					}
					switch {
					case change == "unrelated_scope":
						if base.Code != http.StatusOK || !strings.Contains(base.Body.String(), "private,result") {
							t.Fatal("independent scope denied")
						}
					case prefix:
						if base.Code != http.StatusOK || base.Body.String() != "admitted,prefix\n" {
							t.Fatalf("admitted prefix changed: %d %s", base.Code, base.Body)
						}
					default:
						if base.Code != http.StatusUnauthorized || base.Body.String() != "Authentication required\n" || base.Header().Get("Content-Disposition") != "" {
							t.Fatal("private report response escaped after withdrawal")
						}
					}
				})
			}
		}
	}
}

func TestReportDownloadKeepsOrganizationBoundAuthority(t *testing.T) {
	responseKeepsOrganizationBoundAuthority(t, config.ScopeSettingsRead, responseMediaReport)
}

func responseKeepsOrganizationBoundAuthority(t *testing.T, scope string, media responseMediaPolicy) {
	record := newTokenRecord(t, logStreamToken, []string{scope}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)
	req := httptest.NewRequest(http.MethodGet, "/synthetic-report", nil).WithContext(orgScopeContext("org-a"))
	req.Header.Set("X-API-Token", logStreamToken)
	base := httptest.NewRecorder()
	handler := retainResponseAuthority(cfg, scope, false, media, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		if _, err := w.Write([]byte("own-org,report\n")); err != nil {
			t.Fatal(err)
		}
	})
	AuthContextMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !CheckAuth(cfg, w, r) {
			t.Fatal("org credential admission failed")
		}
		handler(w, r)
	})).ServeHTTP(base, req)
	if base.Code != http.StatusOK || base.Body.String() != "own-org,report\n" {
		t.Fatalf("org settings reader acquired an instance-admin requirement: %d %s", base.Code, base.Body)
	}
}
