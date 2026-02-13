package unifiedresources

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	_ "modernc.org/sqlite"
)

// ResourceStore persists identity overrides and manual links.
type ResourceStore interface {
	AddLink(link ResourceLink) error
	AddExclusion(exclusion ResourceExclusion) error
	GetLinks() ([]ResourceLink, error)
	GetExclusions() ([]ResourceExclusion, error)
	Close() error
}

// ResourceLink represents a manual merge.
type ResourceLink struct {
	ResourceA string
	ResourceB string
	PrimaryID string
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// ResourceExclusion represents a manual split.
type ResourceExclusion struct {
	ResourceA string
	ResourceB string
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// SQLiteResourceStore stores overrides in SQLite.
type SQLiteResourceStore struct {
	db     *sql.DB
	dbPath string
	mu     sync.Mutex
}

const maxOrgIDLength = 64

// NewSQLiteResourceStore opens or creates the SQLite store.
func NewSQLiteResourceStore(dataDir, orgID string) (*SQLiteResourceStore, error) {
	if dataDir == "" {
		dataDir = filepath.Join(utils.GetDataDir(), "resources")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create resources directory: %w", err)
	}

	orgPart := sanitizeOrgID(orgID)
	fileName := "unified_resources.db"
	if orgPart != "" && orgPart != "default" {
		fileName = fmt.Sprintf("unified_resources_%s.db", orgPart)
	}
	path := filepath.Join(dataDir, fileName)

	dsn := path + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
			"cache_size(-64000)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open resources db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &SQLiteResourceStore{db: db, dbPath: path}
	if err := store.initSchema(); err != nil {
		wrappedInitErr := fmt.Errorf("initialize resource store schema for %q: %w", path, err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(
				wrappedInitErr,
				fmt.Errorf("close resources db %q after init failure: %w", path, closeErr),
			)
		}
		return nil, wrappedInitErr
	}
	return store, nil
}

func (s *SQLiteResourceStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS resource_identities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		machine_id TEXT,
		dmi_uuid TEXT,
		primary_hostname TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_identities_machine_id ON resource_identities(machine_id) WHERE machine_id IS NOT NULL;
	CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_identities_dmi_uuid ON resource_identities(dmi_uuid) WHERE dmi_uuid IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_resource_identities_canonical ON resource_identities(canonical_id);
	CREATE INDEX IF NOT EXISTS idx_resource_identities_hostname ON resource_identities(primary_hostname);

	CREATE TABLE IF NOT EXISTS resource_hostnames (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL,
		hostname TEXT NOT NULL,
		source TEXT NOT NULL,
		is_primary BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(hostname, source)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_hostnames_canonical ON resource_hostnames(canonical_id);
	CREATE INDEX IF NOT EXISTS idx_resource_hostnames_hostname ON resource_hostnames(hostname);

	CREATE TABLE IF NOT EXISTS resource_ips (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		source TEXT NOT NULL,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(ip_address, source)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_ips_canonical ON resource_ips(canonical_id);
	CREATE INDEX IF NOT EXISTS idx_resource_ips_ip ON resource_ips(ip_address);

	CREATE TABLE IF NOT EXISTS resource_links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		resource_a TEXT NOT NULL,
		resource_b TEXT NOT NULL,
		primary_id TEXT NOT NULL,
		reason TEXT,
		created_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(resource_a, resource_b)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_links_primary ON resource_links(primary_id);

	CREATE TABLE IF NOT EXISTS resource_exclusions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		resource_a TEXT NOT NULL,
		resource_b TEXT NOT NULL,
		reason TEXT,
		created_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(resource_a, resource_b)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_exclusions_a ON resource_exclusions(resource_a);
	CREATE INDEX IF NOT EXISTS idx_resource_exclusions_b ON resource_exclusions(resource_b);

	CREATE TABLE IF NOT EXISTS resource_metadata (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL UNIQUE,
		custom_url TEXT,
		custom_name TEXT,
		notes TEXT,
		tags TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_by TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_resource_metadata_canonical ON resource_metadata(canonical_id);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize resource store schema: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) AddLink(link ResourceLink) error {
	if link.ResourceA == "" || link.ResourceB == "" {
		return fmt.Errorf("resource IDs required")
	}
	if link.PrimaryID == "" {
		link.PrimaryID = link.ResourceA
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}

	a, b := normalizePair(link.ResourceA, link.ResourceB)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO resource_links (resource_a, resource_b, primary_id, reason, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(resource_a, resource_b) DO UPDATE SET
			primary_id=excluded.primary_id,
			reason=excluded.reason,
			created_by=excluded.created_by,
			created_at=excluded.created_at
	`, a, b, link.PrimaryID, link.Reason, link.CreatedBy, link.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert resource link %q<->%q: %w", a, b, err)
	}
	return nil
}

func (s *SQLiteResourceStore) AddExclusion(exclusion ResourceExclusion) error {
	if exclusion.ResourceA == "" || exclusion.ResourceB == "" {
		return fmt.Errorf("resource IDs required")
	}
	if exclusion.CreatedAt.IsZero() {
		exclusion.CreatedAt = time.Now().UTC()
	}
	a, b := normalizePair(exclusion.ResourceA, exclusion.ResourceB)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO resource_exclusions (resource_a, resource_b, reason, created_by, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(resource_a, resource_b) DO UPDATE SET
			reason=excluded.reason,
			created_by=excluded.created_by,
			created_at=excluded.created_at
	`, a, b, exclusion.Reason, exclusion.CreatedBy, exclusion.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert resource exclusion %q<->%q: %w", a, b, err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetLinks() (links []ResourceLink, err error) {
	rows, err := s.db.Query(`SELECT resource_a, resource_b, primary_id, reason, created_by, created_at FROM resource_links`)
	if err != nil {
		return nil, fmt.Errorf("query resource links: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close resource links rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, wrappedCloseErr)
				return
			}
			err = wrappedCloseErr
		}
	}()

	for rows.Next() {
		var link ResourceLink
		if scanErr := rows.Scan(&link.ResourceA, &link.ResourceB, &link.PrimaryID, &link.Reason, &link.CreatedBy, &link.CreatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan resource link row: %w", scanErr)
		}
		links = append(links, link)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate resource links rows: %w", rowsErr)
	}
	return links, nil
}

func (s *SQLiteResourceStore) GetExclusions() (exclusions []ResourceExclusion, err error) {
	rows, err := s.db.Query(`SELECT resource_a, resource_b, reason, created_by, created_at FROM resource_exclusions`)
	if err != nil {
		return nil, fmt.Errorf("query resource exclusions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close resource exclusions rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, wrappedCloseErr)
				return
			}
			err = wrappedCloseErr
		}
	}()

	for rows.Next() {
		var exclusion ResourceExclusion
		if scanErr := rows.Scan(&exclusion.ResourceA, &exclusion.ResourceB, &exclusion.Reason, &exclusion.CreatedBy, &exclusion.CreatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan resource exclusion row: %w", scanErr)
		}
		exclusions = append(exclusions, exclusion)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate resource exclusions rows: %w", rowsErr)
	}
	return exclusions, nil
}

func (s *SQLiteResourceStore) Close() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("close resources db %q: %w", s.dbPath, err)
		}
	}
	return nil
}

// MemoryStore is an in-memory implementation for tests.
type MemoryStore struct {
	mu         sync.RWMutex
	links      []ResourceLink
	exclusions []ResourceExclusion
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) AddLink(link ResourceLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links = append(m.links, link)
	return nil
}

func (m *MemoryStore) AddExclusion(exclusion ResourceExclusion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exclusions = append(m.exclusions, exclusion)
	return nil
}

func (m *MemoryStore) GetLinks() ([]ResourceLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ResourceLink, len(m.links))
	copy(out, m.links)
	return out, nil
}

func (m *MemoryStore) GetExclusions() ([]ResourceExclusion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ResourceExclusion, len(m.exclusions))
	copy(out, m.exclusions)
	return out, nil
}

func (m *MemoryStore) Close() error {
	return nil
}

func normalizePair(a, b string) (string, string) {
	if a > b {
		return b, a
	}
	return a, b
}

func sanitizeOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(orgID))
	lastWasUnderscore := false
	for _, r := range orgID {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			r == '_' {
			if b.Len() >= maxOrgIDLength {
				break
			}
			b.WriteRune(r)
			lastWasUnderscore = false
			continue
		}

		if !lastWasUnderscore {
			if b.Len() >= maxOrgIDLength {
				break
			}
			b.WriteByte('_')
			lastWasUnderscore = true
		}
	}

	sanitized := strings.Trim(b.String(), "_-")
	if len(sanitized) > maxOrgIDLength {
		sanitized = sanitized[:maxOrgIDLength]
	}
	return sanitized
}
