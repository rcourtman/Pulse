package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func parseTZOffsetMinutes(offsetMin int) string {
	// SQLite modifier format: "+HH:MM" or "-HH:MM"
	sign := "+"
	if offsetMin < 0 {
		sign = "-"
		offsetMin = -offsetMin
	}
	h := offsetMin / 60
	m := offsetMin % 60
	return fmt.Sprintf("%s%02d:%02d", sign, h, m)
}

// ListPointsSeries returns per-day counts for recovery points matching filters.
// It only counts points with completed_at_ms present (activity timeline).
func (s *Store) ListPointsSeries(ctx context.Context, opts recovery.ListPointsOptions, tzOffsetMinutes int) ([]recovery.PointsSeriesBucket, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	where := make([]string, 0, 10)
	args := make([]any, 0, 16)

	where = append(where, "completed_at_ms IS NOT NULL")

	if strings.TrimSpace(string(opts.Provider)) != "" {
		where = append(where, "provider = ?")
		args = append(args, string(opts.Provider))
	}
	if strings.TrimSpace(string(opts.Kind)) != "" {
		where = append(where, "kind = ?")
		args = append(args, string(opts.Kind))
	}
	if strings.TrimSpace(string(opts.Mode)) != "" {
		where = append(where, "mode = ?")
		args = append(args, string(opts.Mode))
	}
	if strings.TrimSpace(string(opts.Outcome)) != "" {
		where = append(where, "outcome = ?")
		args = append(args, string(opts.Outcome))
	}
	if strings.TrimSpace(opts.RollupID) != "" {
		rid := strings.TrimSpace(opts.RollupID)
		if !strings.HasPrefix(rid, "res:") && !strings.HasPrefix(rid, "ext:") {
			rid = "res:" + rid
		}
		where = append(where, "subject_key = ?")
		args = append(args, rid)
	} else if strings.TrimSpace(opts.SubjectResourceID) != "" {
		where = append(where, "subject_resource_id = ?")
		args = append(args, strings.TrimSpace(opts.SubjectResourceID))
	}
	if opts.From != nil && !opts.From.IsZero() {
		where = append(where, "completed_at_ms >= ?")
		args = append(args, opts.From.UTC().UnixMilli())
	}
	if opts.To != nil && !opts.To.IsZero() {
		where = append(where, "completed_at_ms <= ?")
		args = append(args, opts.To.UTC().UnixMilli())
	}
	if strings.TrimSpace(opts.ClusterLabel) != "" {
		where = append(where, "cluster_label = ?")
		args = append(args, strings.TrimSpace(opts.ClusterLabel))
	}
	if strings.TrimSpace(opts.NodeHostLabel) != "" {
		where = append(where, "node_host_label = ?")
		args = append(args, strings.TrimSpace(opts.NodeHostLabel))
	}
	if strings.TrimSpace(opts.NamespaceLabel) != "" {
		where = append(where, "namespace_label = ?")
		args = append(args, strings.TrimSpace(opts.NamespaceLabel))
	}
	if opts.WorkloadOnly {
		where = append(where, "is_workload = 1")
	}
	if v := strings.ToLower(strings.TrimSpace(opts.Verification)); v != "" {
		switch v {
		case "verified":
			where = append(where, "verified = 1")
		case "unverified":
			where = append(where, "verified = 0")
		case "unknown":
			where = append(where, "verified IS NULL")
		}
	}
	if q := strings.ToLower(strings.TrimSpace(opts.Query)); q != "" {
		needle := "%" + q + "%"
		where = append(where, `
			(
				LOWER(id) LIKE ? OR
				LOWER(subject_label) LIKE ? OR
				LOWER(subject_type) LIKE ? OR
				LOWER(cluster_label) LIKE ? OR
				LOWER(node_host_label) LIKE ? OR
				LOWER(namespace_label) LIKE ? OR
				LOWER(entity_id_label) LIKE ? OR
				LOWER(repository_label) LIKE ? OR
				LOWER(details_summary) LIKE ?
			)
		`)
		for i := 0; i < 9; i++ {
			args = append(args, needle)
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")
	offsetModifier := parseTZOffsetMinutes(tzOffsetMinutes)

	query := `
		SELECT
			date(completed_at_ms / 1000, 'unixepoch', ?) AS day,
			mode,
			COUNT(*) AS c
		FROM recovery_points
		` + whereSQL + `
		GROUP BY day, mode
		ORDER BY day ASC
	`

	// The timezone modifier is a SQL parameter to avoid injection.
	argsWithTZ := append([]any{offsetModifier}, args...)
	rows, err := s.db.QueryContext(ctx, query, argsWithTZ...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawRow struct {
		day  string
		mode string
		c    int
	}

	raw := make([]rawRow, 0, 128)
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.day, &r.mode, &r.c); err != nil {
			return nil, err
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build a dense series across the requested time window (if provided).
	var startDay, endDay time.Time
	if opts.From != nil && !opts.From.IsZero() {
		startDay = opts.From.UTC()
	} else {
		startDay = time.Now().UTC().Add(-29 * 24 * time.Hour)
	}
	if opts.To != nil && !opts.To.IsZero() {
		endDay = opts.To.UTC()
	} else {
		endDay = time.Now().UTC()
	}
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}
	// Normalize to day boundaries in UTC (client grouping offset is handled in SQLite).
	startDay = time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, time.UTC)
	endDay = time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, time.UTC)

	buckets := make(map[string]*recovery.PointsSeriesBucket, len(raw)+32)
	for _, r := range raw {
		b := buckets[r.day]
		if b == nil {
			b = &recovery.PointsSeriesBucket{Day: r.day}
			buckets[r.day] = b
		}
		b.Total += r.c
		switch strings.ToLower(strings.TrimSpace(r.mode)) {
		case "snapshot":
			b.Snapshot += r.c
		case "remote":
			b.Remote += r.c
		default:
			// Treat anything else as "local" for bucketing.
			b.Local += r.c
		}
	}

	out := make([]recovery.PointsSeriesBucket, 0, int(endDay.Sub(startDay).Hours()/24)+1)
	for d := startDay; !d.After(endDay); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		if b, ok := buckets[key]; ok {
			out = append(out, *b)
		} else {
			out = append(out, recovery.PointsSeriesBucket{Day: key, Total: 0, Snapshot: 0, Local: 0, Remote: 0})
		}
	}
	return out, nil
}

// ListPointsFacets returns distinct filter values and basic capability flags for a filtered point set.
func (s *Store) ListPointsFacets(ctx context.Context, opts recovery.ListPointsOptions) (recovery.PointsFacets, error) {
	if err := s.ensureInitialized(); err != nil {
		return recovery.PointsFacets{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// We reuse the list semantics for time-window filtering: include rows with NULL completed_at_ms.
	where := make([]string, 0, 10)
	args := make([]any, 0, 16)

	if strings.TrimSpace(string(opts.Provider)) != "" {
		where = append(where, "provider = ?")
		args = append(args, string(opts.Provider))
	}
	if strings.TrimSpace(string(opts.Kind)) != "" {
		where = append(where, "kind = ?")
		args = append(args, string(opts.Kind))
	}
	if strings.TrimSpace(string(opts.Mode)) != "" {
		where = append(where, "mode = ?")
		args = append(args, string(opts.Mode))
	}
	if strings.TrimSpace(string(opts.Outcome)) != "" {
		where = append(where, "outcome = ?")
		args = append(args, string(opts.Outcome))
	}
	if strings.TrimSpace(opts.RollupID) != "" {
		rid := strings.TrimSpace(opts.RollupID)
		if !strings.HasPrefix(rid, "res:") && !strings.HasPrefix(rid, "ext:") {
			rid = "res:" + rid
		}
		where = append(where, "subject_key = ?")
		args = append(args, rid)
	} else if strings.TrimSpace(opts.SubjectResourceID) != "" {
		where = append(where, "subject_resource_id = ?")
		args = append(args, strings.TrimSpace(opts.SubjectResourceID))
	}
	if opts.From != nil && !opts.From.IsZero() {
		where = append(where, "(completed_at_ms IS NULL OR completed_at_ms >= ?)")
		args = append(args, opts.From.UTC().UnixMilli())
	}
	if opts.To != nil && !opts.To.IsZero() {
		where = append(where, "(completed_at_ms IS NULL OR completed_at_ms <= ?)")
		args = append(args, opts.To.UTC().UnixMilli())
	}
	if strings.TrimSpace(opts.ClusterLabel) != "" {
		where = append(where, "cluster_label = ?")
		args = append(args, strings.TrimSpace(opts.ClusterLabel))
	}
	if strings.TrimSpace(opts.NodeHostLabel) != "" {
		where = append(where, "node_host_label = ?")
		args = append(args, strings.TrimSpace(opts.NodeHostLabel))
	}
	if strings.TrimSpace(opts.NamespaceLabel) != "" {
		where = append(where, "namespace_label = ?")
		args = append(args, strings.TrimSpace(opts.NamespaceLabel))
	}
	if opts.WorkloadOnly {
		where = append(where, "is_workload = 1")
	}
	if v := strings.ToLower(strings.TrimSpace(opts.Verification)); v != "" {
		switch v {
		case "verified":
			where = append(where, "verified = 1")
		case "unverified":
			where = append(where, "verified = 0")
		case "unknown":
			where = append(where, "verified IS NULL")
		}
	}
	if q := strings.ToLower(strings.TrimSpace(opts.Query)); q != "" {
		needle := "%" + q + "%"
		where = append(where, `
			(
				LOWER(id) LIKE ? OR
				LOWER(subject_label) LIKE ? OR
				LOWER(subject_type) LIKE ? OR
				LOWER(cluster_label) LIKE ? OR
				LOWER(node_host_label) LIKE ? OR
				LOWER(namespace_label) LIKE ? OR
				LOWER(entity_id_label) LIKE ? OR
				LOWER(repository_label) LIKE ? OR
				LOWER(details_summary) LIKE ?
			)
		`)
		for i := 0; i < 9; i++ {
			args = append(args, needle)
		}
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	} else {
		whereSQL = "WHERE 1=1"
	}

	distinctStrings := func(col string) ([]string, error) {
		q := `
			SELECT DISTINCT ` + col + `
			FROM recovery_points
			` + whereSQL + `
			AND ` + col + ` IS NOT NULL
			AND TRIM(` + col + `) != ''
			ORDER BY ` + col + ` ASC
		`
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]string, 0, 64)
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			v = strings.TrimSpace(v)
			if v != "" {
				out = append(out, v)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return out, nil
	}

	var facets recovery.PointsFacets
	var err1 error
	facets.Clusters, err1 = distinctStrings("cluster_label")
	if err1 != nil {
		return recovery.PointsFacets{}, err1
	}
	facets.NodesHosts, err1 = distinctStrings("node_host_label")
	if err1 != nil {
		return recovery.PointsFacets{}, err1
	}
	facets.Namespaces, err1 = distinctStrings("namespace_label")
	if err1 != nil {
		return recovery.PointsFacets{}, err1
	}

	exists := func(expr string) (bool, error) {
		row := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM recovery_points "+whereSQL+" AND "+expr+" LIMIT 1)", args...)
		var v int
		if err := row.Scan(&v); err != nil {
			return false, err
		}
		return v != 0, nil
	}

	if b, err := exists("size_bytes IS NOT NULL AND size_bytes > 0"); err == nil {
		facets.HasSize = b
	}
	if b, err := exists("verified IS NOT NULL"); err == nil {
		facets.HasVerification = b
	}
	if b, err := exists("entity_id_label IS NOT NULL AND TRIM(entity_id_label) != ''"); err == nil {
		facets.HasEntityID = b
	}

	return facets, nil
}
