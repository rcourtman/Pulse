package recovery

import "strings"

// FilterPoints applies the canonical recovery query to graph-owned in-memory evidence.
// Persisted reads use the equivalent indexed store query.
func FilterPoints(all []RecoveryPoint, opts ListPointsOptions) []RecoveryPoint {
	if len(all) == 0 {
		return nil
	}

	provider := strings.TrimSpace(string(opts.Provider))
	kind := strings.TrimSpace(string(opts.Kind))
	mode := strings.TrimSpace(string(opts.Mode))
	outcome := strings.TrimSpace(string(opts.Outcome))
	subjectRID := strings.TrimSpace(opts.SubjectResourceID)
	rollupID := strings.TrimSpace(opts.RollupID)
	cluster := strings.TrimSpace(opts.ClusterLabel)
	node := strings.TrimSpace(opts.NodeHostLabel)
	namespace := strings.TrimSpace(opts.NamespaceLabel)
	itemType := NormalizeRecoveryItemType(opts.ItemType)
	q := strings.ToLower(strings.TrimSpace(opts.Query))
	workloadOnly := opts.WorkloadOnly
	verification := strings.ToLower(strings.TrimSpace(opts.Verification))
	if rollupID != "" && !strings.HasPrefix(rollupID, "res:") && !strings.HasPrefix(rollupID, "ext:") {
		rollupID = "res:" + rollupID
	}

	out := make([]RecoveryPoint, 0, len(all))
	for _, p := range all {
		// Ensure display/index data exists for filtering parity with sqlite store.
		if p.Display == nil {
			idx := DeriveIndex(p)
			p.Display = idx.ToDisplay()
		}
		disp := p.Display

		if provider != "" && strings.TrimSpace(string(p.Provider)) != provider {
			continue
		}
		if kind != "" && strings.TrimSpace(string(p.Kind)) != kind {
			continue
		}
		if mode != "" && strings.TrimSpace(string(p.Mode)) != mode {
			continue
		}
		if outcome != "" && strings.TrimSpace(string(p.Outcome)) != outcome {
			continue
		}
		if rollupID != "" {
			if strings.TrimSpace(SubjectKeyForPoint(p)) != rollupID {
				continue
			}
		} else if subjectRID != "" && strings.TrimSpace(p.SubjectResourceID) != subjectRID {
			continue
		}
		if opts.From != nil && !opts.From.IsZero() {
			// Match store semantics: completed_at_ms IS NULL OR completed_at_ms >= from
			if p.CompletedAt != nil && !p.CompletedAt.IsZero() && p.CompletedAt.Before(opts.From.UTC()) {
				continue
			}
		}
		if opts.To != nil && !opts.To.IsZero() {
			// Match store semantics: completed_at_ms IS NULL OR completed_at_ms <= to
			if p.CompletedAt != nil && !p.CompletedAt.IsZero() && p.CompletedAt.After(opts.To.UTC()) {
				continue
			}
		}

		if cluster != "" && strings.TrimSpace(getDisplayClusterLabel(p.Display)) != cluster {
			continue
		}
		if node != "" && strings.TrimSpace(getDisplayNodeHostLabel(p.Display)) != node {
			continue
		}
		if namespace != "" && strings.TrimSpace(getDisplayNamespaceLabel(p.Display)) != namespace {
			continue
		}
		if itemType != "" && strings.TrimSpace(getDisplayItemType(p.Display)) != itemType {
			continue
		}
		if workloadOnly && !(disp != nil && disp.IsWorkload) {
			continue
		}
		if verification != "" {
			v := p.Verified
			switch verification {
			case "verified":
				if v == nil || !*v {
					continue
				}
			case "unverified":
				if v == nil || *v {
					continue
				}
			case "unknown":
				if v != nil {
					continue
				}
			}
		}
		if q != "" {
			// Best-effort match across normalized display fields.
			var subjectLabel, subjectType, itemTypeLabel, clusterLabel, nodeLabel, nsLabel, entityID, repoLabel, detailSummary string
			if disp != nil {
				subjectLabel = disp.SubjectLabel
				subjectType = disp.SubjectType
				itemTypeLabel = disp.ItemType
				clusterLabel = disp.ClusterLabel
				nodeLabel = disp.NodeHostLabel
				nsLabel = disp.NamespaceLabel
				entityID = disp.EntityIDLabel
				repoLabel = disp.RepositoryLabel
				detailSummary = disp.DetailsSummary
			}

			hay := strings.ToLower(strings.Join([]string{
				strings.TrimSpace(p.ID),
				strings.TrimSpace(string(p.Provider)),
				strings.TrimSpace(string(p.Kind)),
				strings.TrimSpace(string(p.Mode)),
				strings.TrimSpace(string(p.Outcome)),
				strings.TrimSpace(subjectLabel),
				strings.TrimSpace(subjectType),
				strings.TrimSpace(itemTypeLabel),
				strings.TrimSpace(clusterLabel),
				strings.TrimSpace(nodeLabel),
				strings.TrimSpace(nsLabel),
				strings.TrimSpace(entityID),
				strings.TrimSpace(repoLabel),
				strings.TrimSpace(detailSummary),
			}, " "))
			if !strings.Contains(hay, q) {
				continue
			}
		}

		out = append(out, p)
	}
	return out
}

func getDisplayClusterLabel(d *RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	return d.ClusterLabel
}

func getDisplayNodeHostLabel(d *RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	return d.NodeHostLabel
}

func getDisplayNamespaceLabel(d *RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	return d.NamespaceLabel
}

func getDisplayItemType(d *RecoveryPointDisplay) string {
	if d == nil {
		return ""
	}
	if v := NormalizeRecoveryItemType(d.ItemType); v != "" {
		return v
	}
	return NormalizeRecoveryItemType(d.SubjectType)
}

// PaginatePoints applies the same bounds as the persisted recovery store.
func PaginatePoints(points []RecoveryPoint, page, limit int) []RecoveryPoint {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = DefaultListPageLimit
	}
	if limit > MaxListPageLimit {
		limit = MaxListPageLimit
	}
	// Compare before multiplying so a caller-supplied page cannot overflow.
	if len(points) == 0 || page > (len(points)-1)/limit+1 {
		return []RecoveryPoint{}
	}
	offset := (page - 1) * limit
	end := offset + limit
	if end > len(points) {
		end = len(points)
	}
	return points[offset:end]
}
