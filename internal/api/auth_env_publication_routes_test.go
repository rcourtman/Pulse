package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Both deployment branches must publish private, restart-readable credentials
// before acknowledging the existing password-change response. The other saved
// settings and old open readers must not be rewritten along with the password.
func TestAuthEnvPublicationPasswordChangeConsumers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX open-reader replacement fixture")
	}
	for _, docker := range []string{"true", "false"} {
		t.Run("docker="+docker, func(t *testing.T) {
			t.Setenv("PULSE_DOCKER", docker)
			router := newAuthRouter(t)
			path := filepath.Join(router.config.ConfigPath, ".env")
			content := "# retained settings\nPULSE_AUTH_USER='admin'\nPULSE_AUTH_PASS='" + router.config.AuthPass + "'\nPULSE_PORT='7654'\n"
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0644); err != nil {
				t.Fatal(err)
			}
			reader, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(`{"currentPassword":"currentpassword","newPassword":"NewFixturePassword!123"}`))
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
			rec := httptest.NewRecorder()
			router.handleChangePassword(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("password change status = %d, want 200", rec.Code)
			}
			values, err := godotenv.Read(path)
			if err != nil || values["PULSE_AUTH_USER"] != "admin" || values["PULSE_PORT"] != "7654" {
				t.Fatal("password change lost restart-time settings", err)
			}
			if values["PULSE_AUTH_PASS"] != router.config.AuthPass || !auth.CheckPasswordHash("NewFixturePassword!123", values["PULSE_AUTH_PASS"]) || auth.CheckPasswordHash("currentpassword", values["PULSE_AUTH_PASS"]) {
				t.Fatal("saved and live password authority differ")
			}
			got, err := os.ReadFile(path)
			if err != nil || strings.Contains(string(got), "NewFixturePassword!123") || !strings.HasPrefix(string(got), "# retained settings\n") {
				t.Fatal("password change lost the comment or stored the raw password", err)
			}
			checkAuthEnvPublication(t, path, got)
			oldValues, err := godotenv.Parse(reader)
			if err != nil || !auth.CheckPasswordHash("currentpassword", oldValues["PULSE_AUTH_PASS"]) {
				t.Fatal("password change rewrote a reader's existing configuration descriptor", err)
			}
		})
	}
}
