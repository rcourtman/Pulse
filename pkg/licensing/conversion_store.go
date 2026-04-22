package licensing

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	_ "modernc.org/sqlite"
)

const (
	privateDirPerm  = 0o700
	privateFilePerm = 0o600
)

type ConversionStore struct {
	db *sql.DB
}

type StoredConversionEvent struct {
	ID             int64
	OrgID          string
	EventType      string
	Surface        string
	Capability     string
	IdempotencyKey string
	CreatedAt      time.Time
}

type FunnelStageCounts struct {
	PricingViewed           int64 `json:"pricing_viewed"`
	PaywallViewed           int64 `json:"paywall_viewed"`
	TrialStarted            int64 `json:"trial_started"`
	UpgradeClicked          int64 `json:"upgrade_clicked"`
	CheckoutClicked         int64 `json:"checkout_clicked"`
	CheckoutStarted         int64 `json:"checkout_started"`
	CheckoutCompleted       int64 `json:"checkout_completed"`
	LicenseActivated        int64 `json:"license_activated"`
	LicenseActivationFailed int64 `json:"license_activation_failed"`
}

type FunnelSummary struct {
	FunnelStageCounts
	Period struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"period"`
}

type FunnelDayBreakdown struct {
	Day string `json:"day"`
	FunnelStageCounts
}

type FunnelDimensionBreakdown struct {
	Key string `json:"key"`
	FunnelStageCounts
}

type FunnelReport struct {
	Summary      FunnelSummary              `json:"summary"`
	Daily        []FunnelDayBreakdown       `json:"daily"`
	Surfaces     []FunnelDimensionBreakdown `json:"surfaces"`
	Capabilities []FunnelDimensionBreakdown `json:"capabilities"`
}

type InfrastructureOnboardingStageCounts struct {
	Opened            int64 `json:"opened"`
	APIPathSelected   int64 `json:"api_path_selected"`
	AgentPathSelected int64 `json:"agent_path_selected"`
	ProbeDetected     int64 `json:"probe_detected"`
	ProbeNoMatch      int64 `json:"probe_no_match"`
	ProbeError        int64 `json:"probe_error"`
	CatalogSelected   int64 `json:"catalog_selected"`
	CredentialsOpened int64 `json:"credentials_opened"`
}

type InfrastructureOnboardingSummary struct {
	InfrastructureOnboardingStageCounts
	Period struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"period"`
}

type InfrastructureOnboardingDayBreakdown struct {
	Day string `json:"day"`
	InfrastructureOnboardingStageCounts
}

type InfrastructureOnboardingPathBreakdown struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type InfrastructureOnboardingPlatformBreakdown struct {
	Key               string `json:"key"`
	CatalogSelected   int64  `json:"catalog_selected"`
	CredentialsOpened int64  `json:"credentials_opened"`
}

type InfrastructureOnboardingReport struct {
	Summary   InfrastructureOnboardingSummary             `json:"summary"`
	Daily     []InfrastructureOnboardingDayBreakdown      `json:"daily"`
	Paths     []InfrastructureOnboardingPathBreakdown     `json:"paths"`
	Platforms []InfrastructureOnboardingPlatformBreakdown `json:"platforms"`
}

var infrastructureOnboardingEventTypes = []string{
	EventInfrastructureOnboardingOpened,
	EventInfrastructureOnboardingPathSelected,
	EventInfrastructureOnboardingProbeResult,
	EventInfrastructureOnboardingCatalogSelected,
	EventInfrastructureOnboardingCredentialsOpened,
}

var infrastructureOnboardingPlatformCapabilities = map[string]struct{}{
	"vmware":  {},
	"truenas": {},
	"pve":     {},
	"pbs":     {},
	"pmg":     {},
}

func applyFunnelStageCount(counts *FunnelStageCounts, eventType string, count int64) {
	if counts == nil {
		return
	}

	switch strings.TrimSpace(eventType) {
	case EventPricingViewed:
		counts.PricingViewed = count
	case EventPaywallViewed:
		counts.PaywallViewed = count
	case EventTrialStarted:
		counts.TrialStarted = count
	case EventUpgradeClicked:
		counts.UpgradeClicked = count
	case EventCheckoutClicked:
		counts.CheckoutClicked = count
	case EventCheckoutStarted:
		counts.CheckoutStarted = count
	case EventCheckoutCompleted:
		counts.CheckoutCompleted = count
	case EventLicenseActivated:
		counts.LicenseActivated = count
	case EventLicenseActivationFailed:
		counts.LicenseActivationFailed = count
	}
}

func compareFunnelBreakdowns(a, b FunnelDimensionBreakdown) int {
	switch {
	case a.CheckoutClicked != b.CheckoutClicked:
		if a.CheckoutClicked > b.CheckoutClicked {
			return -1
		}
		return 1
	case a.PricingViewed != b.PricingViewed:
		if a.PricingViewed > b.PricingViewed {
			return -1
		}
		return 1
	case a.LicenseActivated != b.LicenseActivated:
		if a.LicenseActivated > b.LicenseActivated {
			return -1
		}
		return 1
	case a.TrialStarted != b.TrialStarted:
		if a.TrialStarted > b.TrialStarted {
			return -1
		}
		return 1
	case a.Key < b.Key:
		return -1
	case a.Key > b.Key:
		return 1
	default:
		return 0
	}
}

func applyInfrastructureOnboardingStageCount(
	counts *InfrastructureOnboardingStageCounts,
	eventType string,
	capability string,
	count int64,
) {
	if counts == nil {
		return
	}

	switch strings.TrimSpace(eventType) {
	case EventInfrastructureOnboardingOpened:
		counts.Opened += count
	case EventInfrastructureOnboardingPathSelected:
		switch strings.TrimSpace(capability) {
		case "api":
			counts.APIPathSelected += count
		case "agent":
			counts.AgentPathSelected += count
		}
	case EventInfrastructureOnboardingProbeResult:
		switch strings.TrimSpace(capability) {
		case "detected":
			counts.ProbeDetected += count
		case "no-match":
			counts.ProbeNoMatch += count
		case "error":
			counts.ProbeError += count
		}
	case EventInfrastructureOnboardingCatalogSelected:
		counts.CatalogSelected += count
	case EventInfrastructureOnboardingCredentialsOpened:
		counts.CredentialsOpened += count
	}
}

func compareInfrastructureOnboardingPathBreakdowns(
	a,
	b InfrastructureOnboardingPathBreakdown,
) int {
	switch {
	case a.Count != b.Count:
		if a.Count > b.Count {
			return -1
		}
		return 1
	case a.Key < b.Key:
		return -1
	case a.Key > b.Key:
		return 1
	default:
		return 0
	}
}

func compareInfrastructureOnboardingPlatformBreakdowns(
	a,
	b InfrastructureOnboardingPlatformBreakdown,
) int {
	switch {
	case a.CredentialsOpened != b.CredentialsOpened:
		if a.CredentialsOpened > b.CredentialsOpened {
			return -1
		}
		return 1
	case a.CatalogSelected != b.CatalogSelected:
		if a.CatalogSelected > b.CatalogSelected {
			return -1
		}
		return 1
	case a.Key < b.Key:
		return -1
	case a.Key > b.Key:
		return 1
	default:
		return 0
	}
}

func isInfrastructureOnboardingPlatformCapability(capability string) bool {
	_, ok := infrastructureOnboardingPlatformCapabilities[strings.TrimSpace(capability)]
	return ok
}

func ensureOwnerOnlyDir(dir string) error {
	return securityutil.EnsureSecureStorageDir(dir, privateDirPerm)
}

func rejectSymlinkOrNonRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe sqlite path %q: symlink is not allowed", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe sqlite path %q: non-regular file is not allowed", path)
	}
	return nil
}

func hardenSQLiteFile(path string) error {
	if err := rejectSymlinkOrNonRegular(path); err != nil {
		return err
	}
	return os.Chmod(path, privateFilePerm)
}

func hardenSQLiteArtifacts(dbPath string) error {
	artifacts := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	for _, path := range artifacts {
		if err := hardenSQLiteFile(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func NewConversionStore(dbPath string) (*ConversionStore, error) {
	dbPath = filepath.Clean(strings.TrimSpace(dbPath))
	if dbPath == "" {
		return nil, fmt.Errorf("conversion db path is required")
	}

	dir := filepath.Dir(dbPath)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create conversion db directory: %w", err)
	}
	if err := rejectSymlinkOrNonRegular(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
		},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversion db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &ConversionStore{db: db}
	if err := store.initSchema(); err != nil {
		initErr := fmt.Errorf("initialize conversion schema: %w", err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(initErr, fmt.Errorf("close conversion db after init failure: %w", closeErr))
		}
		return nil, initErr
	}
	if err := hardenSQLiteArtifacts(dbPath); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to secure conversion db files: %w", err)
	}
	return store, nil
}

func (s *ConversionStore) ensureInitialized() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("conversion store is not initialized")
	}
	return nil
}

func formatTimeForDB(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func (s *ConversionStore) initSchema() error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS conversion_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		org_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		surface TEXT NOT NULL DEFAULT '',
		capability TEXT NOT NULL DEFAULT '',
		idempotency_key TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(idempotency_key)
	);
	CREATE INDEX IF NOT EXISTS idx_conversion_events_org_time ON conversion_events(org_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_conversion_events_type ON conversion_events(event_type, created_at);
	CREATE INDEX IF NOT EXISTS idx_conversion_events_time ON conversion_events(created_at);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize conversion schema: %w", err)
	}

	return nil
}

func (s *ConversionStore) Record(event StoredConversionEvent) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	orgID := strings.TrimSpace(event.OrgID)
	if orgID == "" {
		return fmt.Errorf("org_id is required")
	}
	eventType := strings.TrimSpace(event.EventType)
	if eventType == "" {
		return fmt.Errorf("event_type is required")
	}
	idempotencyKey := strings.TrimSpace(event.IdempotencyKey)
	if idempotencyKey == "" {
		return fmt.Errorf("idempotency_key is required")
	}

	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	createdAtValue := formatTimeForDB(createdAt)

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO conversion_events (org_id, event_type, surface, capability, idempotency_key, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		orgID,
		eventType,
		strings.TrimSpace(event.Surface),
		strings.TrimSpace(event.Capability),
		idempotencyKey,
		createdAtValue,
	)
	if err != nil {
		return fmt.Errorf("failed to insert conversion event: %w", err)
	}
	return nil
}

func (s *ConversionStore) Query(orgID string, from, to time.Time, eventType string) (events []StoredConversionEvent, retErr error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	where := make([]string, 0, 8)
	args := make([]any, 0, 8)

	where = append(where, "org_id = ?")
	args = append(args, orgID)
	eventType = strings.TrimSpace(eventType)
	if eventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, eventType)
	}
	if !from.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, formatTimeForDB(from))
	}
	if !to.IsZero() {
		where = append(where, "created_at < ?")
		args = append(args, formatTimeForDB(to))
	}

	query := `
		SELECT
			id,
			org_id,
			event_type,
			surface,
			capability,
			idempotency_key,
			CAST(strftime('%s', created_at) AS INTEGER) AS created_at_unix
		FROM conversion_events
	`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at ASC, id ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversion events: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close conversion event rows: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	events = make([]StoredConversionEvent, 0)
	for rows.Next() {
		var ev StoredConversionEvent
		var createdAtUnix int64
		if err := rows.Scan(
			&ev.ID,
			&ev.OrgID,
			&ev.EventType,
			&ev.Surface,
			&ev.Capability,
			&ev.IdempotencyKey,
			&createdAtUnix,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversion event: %w", err)
		}
		ev.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversion events: %w", err)
	}
	return events, nil
}

func (s *ConversionStore) FunnelSummary(orgID string, from, to time.Time) (summary *FunnelSummary, retErr error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
	}
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("from/to are required")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	where := []string{"created_at >= ?", "created_at < ?", "org_id = ?"}
	args := []any{
		from.UTC().Truncate(time.Second).Format(time.RFC3339),
		to.UTC().Truncate(time.Second).Format(time.RFC3339),
		orgID,
	}

	query := `
		SELECT event_type, COUNT(1)
		FROM conversion_events
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY event_type
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnel summary: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close funnel summary rows: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	summary = &FunnelSummary{}
	summary.Period.From = from.UTC()
	summary.Period.To = to.UTC()

	for rows.Next() {
		var eventType string
		var count int64
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan funnel summary row: %w", err)
		}
		applyFunnelStageCount(&summary.FunnelStageCounts, eventType, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate funnel summary rows: %w", err)
	}
	return summary, nil
}

func (s *ConversionStore) FunnelDailyBreakdown(orgID string, from, to time.Time) (breakdown []FunnelDayBreakdown, retErr error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
	}
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("from/to are required")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	startDay := from.UTC().Truncate(24 * time.Hour)
	endDay := to.UTC().Truncate(24 * time.Hour)
	if !to.UTC().Equal(endDay) {
		endDay = endDay.Add(24 * time.Hour)
	}
	if !startDay.Before(endDay) {
		return []FunnelDayBreakdown{}, nil
	}

	query := `
		SELECT strftime('%Y-%m-%d', created_at) AS bucket_day, event_type, COUNT(1)
		FROM conversion_events
		WHERE created_at >= ? AND created_at < ? AND org_id = ?
		GROUP BY bucket_day, event_type
		ORDER BY bucket_day ASC
	`

	rows, err := s.db.Query(
		query,
		from.UTC().Truncate(time.Second).Format(time.RFC3339),
		to.UTC().Truncate(time.Second).Format(time.RFC3339),
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnel daily breakdown: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close funnel daily rows: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	buckets := make(map[string]*FunnelDayBreakdown)
	breakdown = make([]FunnelDayBreakdown, 0, int(endDay.Sub(startDay)/(24*time.Hour)))
	for day := startDay; day.Before(endDay); day = day.Add(24 * time.Hour) {
		entry := FunnelDayBreakdown{Day: day.Format("2006-01-02")}
		breakdown = append(breakdown, entry)
		buckets[entry.Day] = &breakdown[len(breakdown)-1]
	}

	for rows.Next() {
		var day string
		var eventType string
		var count int64
		if err := rows.Scan(&day, &eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan funnel daily row: %w", err)
		}
		if entry := buckets[strings.TrimSpace(day)]; entry != nil {
			applyFunnelStageCount(&entry.FunnelStageCounts, eventType, count)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate funnel daily rows: %w", err)
	}

	return breakdown, nil
}

func (s *ConversionStore) FunnelDimensionBreakdown(orgID string, from, to time.Time, dimension string) (breakdown []FunnelDimensionBreakdown, retErr error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
	}
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("from/to are required")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	column := ""
	switch strings.TrimSpace(dimension) {
	case "surface":
		column = "surface"
	case "capability":
		column = "capability"
	default:
		return nil, fmt.Errorf("unsupported breakdown dimension %q", dimension)
	}

	query := fmt.Sprintf(`
		SELECT %s AS bucket_key, event_type, COUNT(1)
		FROM conversion_events
		WHERE created_at >= ? AND created_at < ? AND org_id = ? AND TRIM(%s) <> ''
		GROUP BY bucket_key, event_type
	`, column, column)

	rows, err := s.db.Query(
		query,
		from.UTC().Truncate(time.Second).Format(time.RFC3339),
		to.UTC().Truncate(time.Second).Format(time.RFC3339),
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnel %s breakdown: %w", dimension, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close funnel %s rows: %w", dimension, closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	buckets := make(map[string]int)
	for rows.Next() {
		var key string
		var eventType string
		var count int64
		if err := rows.Scan(&key, &eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan funnel %s row: %w", dimension, err)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		entryIndex, ok := buckets[key]
		if !ok {
			breakdown = append(breakdown, FunnelDimensionBreakdown{Key: key})
			entryIndex = len(breakdown) - 1
			buckets[key] = entryIndex
		}
		applyFunnelStageCount(&breakdown[entryIndex].FunnelStageCounts, eventType, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate funnel %s rows: %w", dimension, err)
	}

	sort.SliceStable(breakdown, func(i, j int) bool {
		return compareFunnelBreakdowns(breakdown[i], breakdown[j]) < 0
	})

	return breakdown, nil
}

func (s *ConversionStore) FunnelReport(orgID string, from, to time.Time) (*FunnelReport, error) {
	summary, err := s.FunnelSummary(orgID, from, to)
	if err != nil {
		return nil, err
	}
	daily, err := s.FunnelDailyBreakdown(orgID, from, to)
	if err != nil {
		return nil, err
	}
	surfaces, err := s.FunnelDimensionBreakdown(orgID, from, to, "surface")
	if err != nil {
		return nil, err
	}
	capabilities, err := s.FunnelDimensionBreakdown(orgID, from, to, "capability")
	if err != nil {
		return nil, err
	}

	return &FunnelReport{
		Summary:      *summary,
		Daily:        daily,
		Surfaces:     surfaces,
		Capabilities: capabilities,
	}, nil
}

func (s *ConversionStore) InfrastructureOnboardingReport(
	orgID string,
	from,
	to time.Time,
) (*InfrastructureOnboardingReport, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
	}
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("from/to are required")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	startDay := from.UTC().Truncate(24 * time.Hour)
	endDay := to.UTC().Truncate(24 * time.Hour)
	if !to.UTC().Equal(endDay) {
		endDay = endDay.Add(24 * time.Hour)
	}

	report := &InfrastructureOnboardingReport{}
	report.Summary.Period.From = from.UTC()
	report.Summary.Period.To = to.UTC()

	dayBuckets := make(map[string]*InfrastructureOnboardingDayBreakdown)
	if startDay.Before(endDay) {
		report.Daily = make([]InfrastructureOnboardingDayBreakdown, 0, int(endDay.Sub(startDay)/(24*time.Hour)))
		for day := startDay; day.Before(endDay); day = day.Add(24 * time.Hour) {
			entry := InfrastructureOnboardingDayBreakdown{Day: day.Format("2006-01-02")}
			report.Daily = append(report.Daily, entry)
			dayBuckets[entry.Day] = &report.Daily[len(report.Daily)-1]
		}
	}

	pathBuckets := make(map[string]int)
	platformBuckets := make(map[string]int)

	recordEvent := func(event StoredConversionEvent) {
		capability := strings.TrimSpace(event.Capability)
		applyInfrastructureOnboardingStageCount(
			&report.Summary.InfrastructureOnboardingStageCounts,
			event.EventType,
			capability,
			1,
		)

		dayKey := event.CreatedAt.UTC().Format("2006-01-02")
		if dayEntry := dayBuckets[dayKey]; dayEntry != nil {
			applyInfrastructureOnboardingStageCount(
				&dayEntry.InfrastructureOnboardingStageCounts,
				event.EventType,
				capability,
				1,
			)
		}

		if event.EventType == EventInfrastructureOnboardingPathSelected && capability != "" {
			entryIndex, ok := pathBuckets[capability]
			if !ok {
				report.Paths = append(report.Paths, InfrastructureOnboardingPathBreakdown{Key: capability})
				entryIndex = len(report.Paths) - 1
				pathBuckets[capability] = entryIndex
			}
			report.Paths[entryIndex].Count++
		}

		if !isInfrastructureOnboardingPlatformCapability(capability) {
			return
		}
		if event.EventType != EventInfrastructureOnboardingCatalogSelected &&
			event.EventType != EventInfrastructureOnboardingCredentialsOpened {
			return
		}

		entryIndex, ok := platformBuckets[capability]
		if !ok {
			report.Platforms = append(report.Platforms, InfrastructureOnboardingPlatformBreakdown{Key: capability})
			entryIndex = len(report.Platforms) - 1
			platformBuckets[capability] = entryIndex
		}
		switch event.EventType {
		case EventInfrastructureOnboardingCatalogSelected:
			report.Platforms[entryIndex].CatalogSelected++
		case EventInfrastructureOnboardingCredentialsOpened:
			report.Platforms[entryIndex].CredentialsOpened++
		}
	}

	for _, eventType := range infrastructureOnboardingEventTypes {
		events, err := s.Query(orgID, from, to, eventType)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			recordEvent(event)
		}
	}

	sort.SliceStable(report.Paths, func(i, j int) bool {
		return compareInfrastructureOnboardingPathBreakdowns(report.Paths[i], report.Paths[j]) < 0
	})
	sort.SliceStable(report.Platforms, func(i, j int) bool {
		return compareInfrastructureOnboardingPlatformBreakdowns(report.Platforms[i], report.Platforms[j]) < 0
	})

	return report, nil
}

func (s *ConversionStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
