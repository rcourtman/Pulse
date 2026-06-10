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
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"

	_ "modernc.org/sqlite"
)

const (
	cloudHandoffIssuer     = "pulse-cloud-control-plane"
	handoffPrivateDirPerm  = 0o700
	handoffPrivateFilePerm = 0o600
)

var errHandoffAuthorizationDenied = errors.New("handoff authorization denied")

type cloudHandoffClaims struct {
	AccountID  string `json:"account_id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	TargetPath string `json:"target_path,omitempty"`
	jwt.RegisteredClaims
}

type handoffAuthorization struct {
	UserID string
	Email  string
	Role   models.OrganizationRole
}

type jtiReplayStore struct {
	once sync.Once
	db   *sql.DB
	mu   sync.Mutex

	configDir string
	initErr   error
}

const deleteExpiredHandoffJTIQuery = `
DELETE FROM handoff_jti INDEXED BY idx_handoff_jti_expires_at
WHERE expires_at <= ?`

// openHardenedSecretsDB opens (creating if needed) a single-connection WAL
// sqlite database under <configDir>/secrets with private dir/file
// permissions, applying schema. dirLabel and label feed error wrapping
// ("handoff" / "handoff jti", "purchase return" / "purchase return
// redemption"). Single-sourcing keeps the permission-hardening policy shared
// by every secrets-backed store.
func openHardenedSecretsDB(configDir, fileName, dirLabel, label, schema string) (*sql.DB, error) {
	dir := filepath.Clean(configDir)
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("configDir is required")
	}
	secretsDir := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secretsDir, handoffPrivateDirPerm); err != nil {
		return nil, fmt.Errorf("create %s secrets dir: %w", dirLabel, err)
	}
	if err := os.Chmod(secretsDir, handoffPrivateDirPerm); err != nil {
		return nil, fmt.Errorf("chmod %s secrets dir: %w", dirLabel, err)
	}

	dbPath := filepath.Join(secretsDir, fileName)
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s db: %w", label, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init %s schema: %w", label, err)
	}
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := hardenPrivateFile(path, handoffPrivateFilePerm); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("harden %s file permissions: %w", label, err)
		}
	}
	return db, nil
}

func (s *jtiReplayStore) init() {
	s.once.Do(func() {
		schema := `
		CREATE TABLE IF NOT EXISTS handoff_jti (
			jti TEXT PRIMARY KEY,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_handoff_jti_expires_at ON handoff_jti(expires_at);
		`
		db, err := openHardenedSecretsDB(s.configDir, "handoff_jti.db", "handoff", "handoff jti", schema)
		if err != nil {
			s.initErr = err
			return
		}
		s.db = db
	})
}

func hardenPrivateFile(path string, mode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode().Perm() == mode {
		return nil
	}
	return os.Chmod(path, mode)
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
	if _, err := s.db.Exec(deleteExpiredHandoffJTIQuery, now); err != nil {
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

func (s *jtiReplayStore) delete(jti string) error {
	s.init()
	if s.initErr != nil {
		return s.initErr
	}
	if s.db == nil {
		return fmt.Errorf("handoff jti store not initialized")
	}
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return fmt.Errorf("jti is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(`DELETE FROM handoff_jti WHERE jti = ?`, jti); err != nil {
		return fmt.Errorf("delete handoff jti: %w", err)
	}
	return nil
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
	if hostedModeEnabledFromEnv() {
		if v := tenantIDFromPublicURL(strings.TrimSpace(os.Getenv("PULSE_PUBLIC_URL"))); v != "" {
			return v
		}
	}
	if hostedRuntimeTenantID := tenantIDFromHostedProxyRequest(r); hostedRuntimeTenantID != "" {
		return hostedRuntimeTenantID
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

func tenantIDFromHostedProxyRequest(r *http.Request) string {
	if r == nil || !hostedModeEnabledFromEnv() {
		return ""
	}
	if v := tenantIDFromPublicURL(strings.TrimSpace(r.Host)); v != "" {
		return v
	}
	return ""
}

func tenantIDFromPublicURL(publicURL string) string {
	if publicURL == "" {
		return ""
	}
	if !strings.Contains(publicURL, "://") {
		publicURL = "https://" + publicURL
	}
	parsed, err := url.Parse(publicURL)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return ""
	}
	// Hosted tenant URLs follow "<tenant-id>.<base-domain>" and must have a
	// distinct tenant label ahead of the shared cloud domain.
	if strings.Count(host, ".") < 3 {
		return ""
	}
	tenantID := host
	if i := strings.IndexByte(host, '.'); i > 0 {
		tenantID = host[:i]
	}
	if !isValidOrganizationID(tenantID) {
		return ""
	}
	return tenantID
}

func normalizeHandoffEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isEmailShapedHandoffUserID(userID string) bool {
	return strings.Contains(strings.TrimSpace(userID), "@")
}

// HandleHandoffExchange verifies a control-plane-minted handoff JWT, records its
// jti to prevent replay, then creates a tenant session and redirects to the app.
//
// Route (wiring happens elsewhere): POST /api/cloud/handoff/exchange
//
// If the caller requests JSON (`Accept: application/json` or `?format=json`),
// this returns a success payload instead of redirecting.
func HandleHandoffExchange(configDir string) http.HandlerFunc {
	configDir = filepath.Clean(configDir)
	InitPersistentAuthStores(configDir)
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
		claims.Email = normalizeHandoffEmail(claims.Email)
		subject := strings.TrimSpace(claims.Subject)
		if strings.TrimSpace(claims.ID) == "" || subject == "" || isEmailShapedHandoffUserID(subject) || claims.Email == "" {
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

		authz, err := authorizeHandoffOrganizationMembership(configDir, tenantID, subject, claims.Email, claims.Role)
		if err != nil {
			if errors.Is(err, errHandoffAuthorizationDenied) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Invalidate any pre-existing session to prevent session fixation attacks.
		InvalidateOldSessionFromRequest(r)

		sessionToken := generateSessionToken()
		if sessionToken == "" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		userAgent := r.Header.Get("User-Agent")
		clientIP := GetClientIP(r)
		sessionDuration := 24 * time.Hour
		GetSessionStore().CreateSession(sessionToken, sessionDuration, userAgent, clientIP, authz.UserID)
		TrackUserSession(authz.UserID, sessionToken)

		csrfToken := generateCSRFToken(sessionToken)
		isSecure, sameSitePolicy := getCookieSettings(r)
		cookieMaxAge := int(sessionDuration.Seconds())

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName(isSecure),
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     CookieNameCSRF,
			Value:    csrfToken,
			Path:     "/",
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     CookieNameOrgID,
			Value:    tenantID,
			Path:     "/",
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})

		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			resp := map[string]any{
				"ok":         true,
				"tenant_id":  tenantID,
				"account_id": claims.AccountID,
				"user_id":    authz.UserID,
				"email":      authz.Email,
				"role":       string(authz.Role),
				"jti":        claims.ID,
				"exp":        claims.ExpiresAt.Time.UTC().Format(time.RFC3339),
			}
			if targetPath := sanitizeCloudHandoffTargetPath(claims.TargetPath); targetPath != "" {
				resp["target_path"] = targetPath
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		targetPath := sanitizeCloudHandoffTargetPath(claims.TargetPath)
		if targetPath == "" {
			targetPath = "/"
		}
		http.Redirect(w, r, targetPath, http.StatusTemporaryRedirect)
	}
}

func sanitizeCloudHandoffTargetPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if strings.IndexFunc(raw, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") {
		return ""
	}
	return parsed.String()
}

func authorizeHandoffOrganizationMembership(configDir, tenantID, userID, email, role string) (*handoffAuthorization, error) {
	mtp := config.NewMultiTenantPersistence(configDir)
	org, err := mtp.LoadOrganization(tenantID)
	if err != nil {
		return nil, fmt.Errorf("load tenant organization %s: %w", tenantID, err)
	}
	if org == nil {
		return nil, fmt.Errorf("%w: tenant organization %s is missing", errHandoffAuthorizationDenied, tenantID)
	}

	userID = strings.TrimSpace(userID)
	email = normalizeHandoffEmail(email)
	if userID == "" {
		return nil, fmt.Errorf("%w: handoff user id is empty", errHandoffAuthorizationDenied)
	}
	if isEmailShapedHandoffUserID(userID) {
		return nil, fmt.Errorf("%w: handoff user id must be a stable subject", errHandoffAuthorizationDenied)
	}

	effectiveRole := org.GetMemberRoleForPrincipal(userID, email)
	if effectiveRole == "" {
		return nil, fmt.Errorf("%w: no pre-existing membership for %s", errHandoffAuthorizationDenied, userID)
	}
	if !models.IsValidOrganizationRole(effectiveRole) {
		return nil, fmt.Errorf("%w: stored role %q for %s is invalid", errHandoffAuthorizationDenied, effectiveRole, userID)
	}
	if changed := org.CanonicalizePrincipalIdentity(userID, email); changed {
		if err := mtp.SaveOrganization(org); err != nil {
			return nil, fmt.Errorf("save canonicalized handoff identity for %s: %w", tenantID, err)
		}
	}
	if effectiveRole == models.OrgRoleOwner && strings.TrimSpace(org.OwnerUserID) == "" {
		return nil, fmt.Errorf("%w: tenant organization %s has blank owner", errHandoffAuthorizationDenied, tenantID)
	}

	desiredRole := models.OrganizationRoleFromAccountRole(role)
	if !models.OrganizationRoleAtLeast(effectiveRole, desiredRole) {
		return nil, fmt.Errorf(
			"%w: requested role %q exceeds stored role %q for %s",
			errHandoffAuthorizationDenied,
			desiredRole,
			effectiveRole,
			userID,
		)
	}

	return &handoffAuthorization{
		UserID: userID,
		Email:  email,
		Role:   effectiveRole,
	}, nil
}
