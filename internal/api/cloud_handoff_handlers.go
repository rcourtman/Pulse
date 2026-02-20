package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	_ "modernc.org/sqlite"
)

const (
	cloudHandoffIssuer = "pulse-cloud-control-plane"
)

type cloudHandoffClaims struct {
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

type jtiReplayStore struct {
	once sync.Once
	db   *sql.DB
	mu   sync.Mutex

	configDir string
	initErr   error
}

func (s *jtiReplayStore) init() {
	s.once.Do(func() {
		dir := filepath.Clean(s.configDir)
		if strings.TrimSpace(dir) == "" {
			s.initErr = fmt.Errorf("configDir is required")
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			s.initErr = fmt.Errorf("create config dir: %w", err)
			return
		}

		dbPath := filepath.Join(dir, "handoff_jti.db")
		dsn := dbPath + "?" + url.Values{
			"_pragma": []string{
				"busy_timeout(30000)",
				"journal_mode(WAL)",
				"synchronous(NORMAL)",
			},
		}.Encode()

		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			s.initErr = fmt.Errorf("open handoff jti db: %w", err)
			return
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)

		schema := `
		CREATE TABLE IF NOT EXISTS handoff_jti (
			jti TEXT PRIMARY KEY,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_handoff_jti_expires_at ON handoff_jti(expires_at);
		`
		if _, err := db.Exec(schema); err != nil {
			_ = db.Close()
			s.initErr = fmt.Errorf("init handoff jti schema: %w", err)
			return
		}

		s.db = db
	})
}

func (s *jtiReplayStore) checkAndStore(jti string, expiresAt time.Time) (stored bool, err error) {
	s.init()
	if s.initErr != nil {
		return false, s.initErr
	}
	if s.db == nil {
		return false, fmt.Errorf("handoff jti store not initialized")
	}
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return false, fmt.Errorf("jti is required")
	}
	expiresAt = expiresAt.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Unix()
	if _, err := s.db.Exec(`DELETE FROM handoff_jti WHERE expires_at <= ?`, now); err != nil {
		return false, fmt.Errorf("cleanup handoff jti: %w", err)
	}

	_, err = s.db.Exec(`INSERT INTO handoff_jti (jti, expires_at) VALUES (?, ?)`, jti, expiresAt.Unix())
	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return false, nil
		}
		return false, fmt.Errorf("store handoff jti: %w", err)
	}
	return true, nil
}

func isSQLiteUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") || strings.Contains(s, "constraint failed")
}

func tenantIDFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(os.Getenv("PULSE_TENANT_ID")); v != "" {
		if isValidOrganizationID(v) {
			return v
		}
		return ""
	}
	if r == nil {
		return ""
	}

	peerIP := extractRemoteIP(r.RemoteAddr)
	trustedProxy := isTrustedProxyIP(peerIP)

	rawHost := ""
	if trustedProxy {
		rawHost = firstForwardedValue(r.Header.Get("X-Forwarded-Host"))
	}
	if rawHost == "" {
		// Only trust direct Host for loopback requests (local development/tests).
		if ip := net.ParseIP(peerIP); ip != nil && ip.IsLoopback() {
			rawHost = strings.TrimSpace(r.Host)
		}
	}
	if rawHost == "" {
		return ""
	}

	_, host := sanitizeForwardedHost(rawHost)
	if host == "" {
		return ""
	}

	tenantID := host
	// Host is expected to be "<tenant-id>.<baseDomain>".
	if i := strings.IndexByte(host, '.'); i > 0 {
		tenantID = host[:i]
	}
	if !isValidOrganizationID(tenantID) {
		return ""
	}
	return tenantID
}

// HandleHandoffExchange verifies a control-plane-minted handoff JWT and records its jti to prevent replay.
//
// Route (wiring happens elsewhere): POST /api/cloud/handoff/exchange
//
// This minimal implementation returns success JSON containing user info derived from the token.
// Wiring into RBAC/session minting is intentionally deferred.
func HandleHandoffExchange(configDir string) http.HandlerFunc {
	configDir = filepath.Clean(configDir)
	keyPath := filepath.Join(configDir, "secrets", "handoff.key")
	replay := &jtiReplayStore{configDir: configDir}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		key, err := os.ReadFile(keyPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		tenantID := tenantIDFromRequest(r)
		if tenantID == "" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		tokenStr := strings.TrimSpace(r.FormValue("token"))
		if tokenStr == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}

		var claims cloudHandoffClaims
		parsed, err := jwt.ParseWithClaims(
			tokenStr,
			&claims,
			func(t *jwt.Token) (any, error) { return key, nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(cloudHandoffIssuer),
			jwt.WithAudience(tenantID),
		)
		if err != nil || parsed == nil || !parsed.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		if claims.ExpiresAt == nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ok, err := replay.checkAndStore(claims.ID, claims.ExpiresAt.Time)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "replayed token", http.StatusUnauthorized)
			return
		}

		resp := map[string]any{
			"ok":         true,
			"tenant_id":  tenantID,
			"account_id": claims.AccountID,
			"user_id":    claims.Subject,
			"email":      claims.Email,
			"role":       claims.Role,
			"jti":        claims.ID,
			"exp":        claims.ExpiresAt.Time.UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
