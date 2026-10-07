package unifiedresources

import "time"

// An operator decides each resource pair one way: a link folds the pair into
// one resource, an exclusion keeps it apart (the resources API's unlink and
// report-merge record exclusions). The later decision replaces the earlier
// one. Stores drop the opposite row when they write either, and their reads
// pass through effectiveManualPairDecisions, so a pair that still carries both
// (rows written before writes replaced each other, or IDs that canonical-ID
// succession rewrote onto one pair) is decided by its later write. Every
// registry loads its links and exclusions from the store and resolves the two
// reads together, since a decision can land between them, so the monitor's
// rebuilds, the resources API and the views built from the monitor's links
// all apply the same decision.

// effectiveManualPairDecisions keeps, for each resource pair, only its latest
// decision: the newest link (a later row wins a tie, so a reversed-order copy
// that succession left behind cannot override a newer link's primary) unless
// an exclusion is at least as new. A link and an exclusion written at the
// same instant keep the pair apart: two separate rows lose no data and the
// operator can link again.
func effectiveManualPairDecisions(links []ResourceLink, exclusions []ResourceExclusion) ([]ResourceLink, []ResourceExclusion) {
	if len(links) == 0 {
		return links, exclusions
	}
	newestLink := make(map[string]int, len(links))
	for i, link := range links {
		key := exclusionKey(link.ResourceA, link.ResourceB)
		if j, ok := newestLink[key]; !ok || !links[j].CreatedAt.After(link.CreatedAt) {
			newestLink[key] = i
		}
	}
	latestExclusion := make(map[string]time.Time, len(exclusions))
	for _, exclusion := range exclusions {
		key := exclusionKey(exclusion.ResourceA, exclusion.ResourceB)
		if at, ok := latestExclusion[key]; !ok || exclusion.CreatedAt.After(at) {
			latestExclusion[key] = exclusion.CreatedAt
		}
	}

	keptLinks := make([]ResourceLink, 0, len(newestLink))
	for i, link := range links {
		key := exclusionKey(link.ResourceA, link.ResourceB)
		if newestLink[key] != i {
			continue
		}
		if at, ok := latestExclusion[key]; ok && !link.CreatedAt.After(at) {
			continue
		}
		keptLinks = append(keptLinks, link)
	}
	keptExclusions := make([]ResourceExclusion, 0, len(exclusions))
	for _, exclusion := range exclusions {
		if i, ok := newestLink[exclusionKey(exclusion.ResourceA, exclusion.ResourceB)]; ok && links[i].CreatedAt.After(exclusion.CreatedAt) {
			continue
		}
		keptExclusions = append(keptExclusions, exclusion)
	}
	return keptLinks, keptExclusions
}

// sameManualPair reports whether two ID pairs name the same resources in
// either order.
func sameManualPair(a1, b1, a2, b2 string) bool {
	return exclusionKey(a1, b1) == exclusionKey(a2, b2)
}
