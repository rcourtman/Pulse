package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Rotating the administrator credentials through quick setup must end the
// sessions opened with the replaced login, as a password change does.
func TestQuickSecuritySetupRotationEndsExistingAdminSessions(t *testing.T) {
	openSession := func(t *testing.T, username string) string {
		t.Helper()
		token := generateSessionToken()
		GetSessionStore().CreateSession(token, time.Hour, "test-agent", "192.0.2.50", username)
		TrackUserSession(username, token)
		return token
	}

	for _, tc := range []struct {
		name         string
		newUsername  string
		failTokens   bool
		wantStatus   int
		wantNewLogin bool
	}{
		{name: "same username", newUsername: "admin", wantStatus: http.StatusOK, wantNewLogin: true},
		{name: "renamed administrator", newUsername: "newadmin", wantStatus: http.StatusOK, wantNewLogin: true},
		{name: "administrator renamed by case only", newUsername: "Admin", wantStatus: http.StatusOK, wantNewLogin: true},
		{name: "token save fails after the password is replaced", newUsername: "admin", failTokens: true, wantStatus: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
			resetTrustedProxyConfig()

			dataDir := t.TempDir()
			InitPersistentAuthStores(dataDir)
			hashed, err := internalauth.HashPassword("OldPassword!1")
			if err != nil {
				t.Fatalf("hash password: %v", err)
			}
			cfg := &config.Config{
				AuthUser:   "admin",
				AuthPass:   hashed,
				DataPath:   dataDir,
				ConfigPath: dataDir,
			}
			persistence := config.NewConfigPersistence(cfg.DataPath)
			if tc.failTokens {
				persistence.SetFileSystem(failingWriteFileSystem{})
			}
			router := &Router{config: cfg, persistence: persistence}

			staleSession := openSession(t, "admin")
			staleCSRF := generateCSRFToken(staleSession)
			otherUserSession := openSession(t, "someone-else")
			t.Cleanup(func() { InvalidateUserSessions("someone-else") })

			authLimiter.Reset("198.51.100.19")
			payload := `{"username":"` + tc.newUsername + `","password":"NewPassword!1","apiToken":"` + strings.Repeat("bb", 32) + `","force":true}`
			req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
			req.RemoteAddr = "198.51.100.19:54321"
			req.SetBasicAuth("admin", "OldPassword!1")
			rec := httptest.NewRecorder()

			handleQuickSecuritySetupFixed(router)(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if !internalauth.CheckPasswordHash("NewPassword!1", cfg.AuthPass) {
				t.Fatal("password was not replaced")
			}
			if ValidateSession(staleSession) {
				t.Fatal("session opened with the replaced login survived the rotation")
			}
			if GetCSRFStore().ValidateCSRFToken(staleSession, staleCSRF) {
				t.Fatal("CSRF token of the replaced login's session survived the rotation")
			}
			if !ValidateSession(otherUserSession) {
				t.Fatal("rotation ended another user's session")
			}

			sessionCookie := findCookie(rec.Result().Cookies(), sessionCookieName(false))
			if !tc.wantNewLogin {
				if sessionCookie != nil {
					t.Fatalf("failed rotation issued a session cookie")
				}
				return
			}
			if sessionCookie == nil || !ValidateSession(sessionCookie.Value) {
				t.Fatal("rotation did not leave the caller with a valid new session")
			}
			if got := GetSessionUsername(sessionCookie.Value); got != tc.newUsername {
				t.Fatalf("new session username = %q, want %q", got, tc.newUsername)
			}
			InvalidateUserSessions(tc.newUsername)
		})
	}
}
