package alerts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	maxIncidentSynthesisDepth        = 8
	maxIncidentSynthesisObservations = 100
	maxSupportedCauseLead            = 5 * time.Minute
)

type incidentSynthesisCandidate struct {
	storageKey   string
	alert        *Alert
	failureClass AlertFailureClass
	resource     unifiedresources.Resource
}

type incidentCauseLink struct {
	primaryKey string
	depth      int
	confidence float64
	relation   unifiedresources.RelationshipType
	supported  bool
}

// applyInfrastructureIncidentSynthesisNoLock derives presentation and
// notification context from the complete live detector set and canonical
// resource relationships. Detector identity and recovery remain independent;
// this method only replaces correlations it previously synthesized. Caller
// holds m.mu.
func (m *Manager) applyInfrastructureIncidentSynthesisNoLock(
	resourcesByID map[string]unifiedresources.Resource,
	desired map[string]*Alert,
) {
	if m == nil {
		return
	}

	candidates := make(map[string]*incidentSynthesisCandidate, len(m.activeAlerts)+len(desired))
	preservedCorrelationKeys := make(map[string]struct{})
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		if alert.Correlation != nil && alert.Correlation.Kind != AlertCorrelationKindInfrastructureIncident {
			preservedCorrelationKeys[storageKey] = struct{}{}
			continue
		}
		if alert.Correlation != nil && alert.Correlation.Kind == AlertCorrelationKindInfrastructureIncident {
			alert.Correlation = nil
		}
		candidates[storageKey] = newIncidentSynthesisCandidate(storageKey, alert, resourcesByID)
	}
	for storageKey, alert := range desired {
		if alert == nil {
			continue
		}
		if _, preserve := preservedCorrelationKeys[storageKey]; preserve {
			continue
		}
		if alert.Correlation != nil && alert.Correlation.Kind != AlertCorrelationKindInfrastructureIncident {
			continue
		}
		if alert.Correlation != nil && alert.Correlation.Kind == AlertCorrelationKindInfrastructureIncident {
			alert.Correlation = nil
		}
		candidates[storageKey] = newIncidentSynthesisCandidate(storageKey, alert, resourcesByID)
	}

	byResource := make(map[string][]*incidentSynthesisCandidate)
	for _, candidate := range candidates {
		resourceID := unifiedresources.CanonicalResourceID(candidate.alert.ResourceID)
		if resourceID == "" {
			continue
		}
		byResource[resourceID] = append(byResource[resourceID], candidate)
	}
	for resourceID := range byResource {
		sort.Slice(byResource[resourceID], func(i, j int) bool {
			return incidentCandidateLess(byResource[resourceID][i], byResource[resourceID][j])
		})
	}

	links := make(map[string]incidentCauseLink)
	for storageKey, candidate := range candidates {
		link, ok := strongestIncidentCause(candidate, resourcesByID, byResource, candidates)
		if ok && link.primaryKey != storageKey {
			links[storageKey] = link
		}
	}

	groups := make(map[string][]string)
	groupSupported := make(map[string]bool)
	for storageKey := range candidates {
		rootKey, supported := incidentSynthesisRoot(storageKey, links)
		if rootKey == storageKey {
			continue
		}
		groups[rootKey] = append(groups[rootKey], storageKey)
		if _, seen := groupSupported[rootKey]; !seen {
			groupSupported[rootKey] = supported
		} else {
			groupSupported[rootKey] = groupSupported[rootKey] && supported
		}
	}

	for primaryKey, supportingKeys := range groups {
		primary := candidates[primaryKey]
		if primary == nil || len(supportingKeys) == 0 {
			continue
		}
		sort.Slice(supportingKeys, func(i, j int) bool {
			return incidentCandidateLess(candidates[supportingKeys[i]], candidates[supportingKeys[j]])
		})
		memberKeys := append([]string{primaryKey}, supportingKeys...)
		observations, affected := incidentSynthesisEvidence(memberKeys, candidates)
		inference := AlertCorrelationInferenceObservationSet
		if groupSupported[primaryKey] && !incidentCauseContradicted(primary) {
			inference = AlertCorrelationInferenceSupportedCause
		}
		groupKey := "infrastructure:" + effectiveAlertID(primary.alert, primary.storageKey)
		primaryReason := fmt.Sprintf(
			"%s on %s is connected by current canonical relationships to %d dependent failure observation(s).",
			failureClassLabel(primary.failureClass),
			incidentCandidateName(primary),
			len(supportingKeys),
		)
		if inference == AlertCorrelationInferenceObservationSet {
			primaryReason = fmt.Sprintf(
				"%s and %d related failure observation(s) share canonical relationship context, but the current evidence does not establish this alert as the cause.",
				incidentCandidateName(primary),
				len(supportingKeys),
			)
		}
		primary.alert.Correlation = &AlertCorrelation{
			Key:                 groupKey,
			Kind:                AlertCorrelationKindInfrastructureIncident,
			Role:                AlertCorrelationRolePrimary,
			Reason:              primaryReason,
			FailureClass:        primary.failureClass,
			Inference:           inference,
			PrimaryAlertID:      effectiveAlertID(primary.alert, primary.storageKey),
			PrimaryResourceID:   primary.alert.ResourceID,
			AffectedResourceIDs: affected,
			Observations:        observations,
		}
		for _, supportingKey := range supportingKeys {
			supporting := candidates[supportingKey]
			if supporting == nil {
				continue
			}
			reason := fmt.Sprintf(
				"Grouped beneath %s because a current canonical resource relationship connects this %s observation to the primary failure.",
				incidentCandidateName(primary),
				failureClassLabel(supporting.failureClass),
			)
			if inference == AlertCorrelationInferenceObservationSet {
				reason = fmt.Sprintf(
					"Shown with %s as a related observation; Pulse has not established a causal direction.",
					incidentCandidateName(primary),
				)
			}
			supporting.alert.Correlation = &AlertCorrelation{
				Key:                 groupKey,
				Kind:                AlertCorrelationKindInfrastructureIncident,
				Role:                AlertCorrelationRoleSupporting,
				Reason:              reason,
				FailureClass:        supporting.failureClass,
				Inference:           inference,
				PrimaryAlertID:      effectiveAlertID(primary.alert, primary.storageKey),
				PrimaryResourceID:   primary.alert.ResourceID,
				AffectedResourceIDs: append([]string(nil), affected...),
				Observations:        cloneCorrelationObservations(observations),
			}
		}
	}
}

func newIncidentSynthesisCandidate(
	storageKey string,
	alert *Alert,
	resourcesByID map[string]unifiedresources.Resource,
) *incidentSynthesisCandidate {
	resource := resourcesByID[unifiedresources.CanonicalResourceID(alert.ResourceID)]
	return &incidentSynthesisCandidate{
		storageKey:   storageKey,
		alert:        alert,
		failureClass: classifyIncidentFailure(alert, resource),
		resource:     resource,
	}
}

func classifyIncidentFailure(alert *Alert, resource unifiedresources.Resource) AlertFailureClass {
	code := strings.ToLower(alertMetadataString(alert, "incidentCode"))
	alertType := strings.ToLower(strings.TrimSpace(alert.Type))
	if strings.Contains(code, "certificate") || strings.HasPrefix(code, "tls_") {
		return AlertFailureClassCertificate
	}
	if alertType == strings.ToLower(ExternalProbeUnavailableAlertType) ||
		strings.Contains(code, "coverage") || strings.Contains(code, "stale") || strings.Contains(code, "probe_unavailable") {
		return AlertFailureClassEvidenceCoverage
	}
	if code == "availability_unreachable" && resource.Availability != nil {
		availability := resource.Availability
		if strings.EqualFold(availability.AggregateState, "unknown") ||
			(availability.ExpectedLocations > 0 && availability.ReportingLocations < availability.ExpectedLocations) {
			return AlertFailureClassEvidenceCoverage
		}
		if strings.EqualFold(availability.TransportOutcome, "reachable") &&
			availability.ApplicationOutcome != "" &&
			!strings.EqualFold(availability.ApplicationOutcome, "passed") {
			return AlertFailureClassApplicationResponse
		}
		return AlertFailureClassNetworkPath
	}
	if code == "availability_unreachable" {
		return AlertFailureClassNetworkPath
	}
	if alertType == "offline" || alertType == "connectivity" || alertType == "powered-off" ||
		alertType == "docker-host-offline" || alertType == strings.ToLower(connectionDegradedAlertType) ||
		strings.Contains(code, "offline") || strings.Contains(code, "unreachable") {
		return AlertFailureClassRuntime
	}
	return AlertFailureClassDependency
}

func strongestIncidentCause(
	symptom *incidentSynthesisCandidate,
	resourcesByID map[string]unifiedresources.Resource,
	byResource map[string][]*incidentSynthesisCandidate,
	byStorageKey map[string]*incidentSynthesisCandidate,
) (incidentCauseLink, bool) {
	resourceID := unifiedresources.CanonicalResourceID(symptom.alert.ResourceID)
	if resourceID == "" {
		return incidentCauseLink{}, false
	}
	type frontierItem struct {
		resourceID string
		depth      int
		confidence float64
		relation   unifiedresources.RelationshipType
	}
	frontier := []frontierItem{{resourceID: resourceID, confidence: 1}}
	visited := map[string]struct{}{resourceID: {}}
	var best incidentCauseLink
	found := false
	for len(frontier) > 0 {
		current := frontier[0]
		frontier = frontier[1:]
		if current.depth >= maxIncidentSynthesisDepth {
			continue
		}
		resource, ok := resourcesByID[current.resourceID]
		if !ok {
			continue
		}
		for _, relationship := range unifiedresources.ResourceRelationshipsWithCanonicalParent(resource) {
			if !relationship.Active || unifiedresources.CanonicalResourceID(relationship.SourceID) != current.resourceID {
				continue
			}
			targetID := unifiedresources.CanonicalResourceID(relationship.TargetID)
			if targetID == "" || targetID == resourceID {
				continue
			}
			confidence := current.confidence
			if relationship.Confidence > 0 && relationship.Confidence < confidence {
				confidence = relationship.Confidence
			}
			depth := current.depth + 1
			for _, primary := range byResource[targetID] {
				if !incidentCauseAllowed(symptom, primary, relationship.Type) {
					continue
				}
				supported := confidence >= 0.8 && incidentCauseTimingSupported(symptom.alert, primary.alert)
				link := incidentCauseLink{
					primaryKey: primary.storageKey,
					depth:      depth,
					confidence: confidence,
					relation:   relationship.Type,
					supported:  supported,
				}
				if !found || incidentCauseLinkLess(link, best, primary, byStorageKey[best.primaryKey]) {
					best = link
					found = true
				}
			}
			if _, seen := visited[targetID]; !seen {
				visited[targetID] = struct{}{}
				frontier = append(frontier, frontierItem{resourceID: targetID, depth: depth, confidence: confidence, relation: relationship.Type})
			}
		}
	}
	return best, found
}

func incidentCauseAllowed(symptom, primary *incidentSynthesisCandidate, relation unifiedresources.RelationshipType) bool {
	if symptom == nil || primary == nil || symptom.storageKey == primary.storageKey {
		return false
	}
	switch relation {
	case unifiedresources.RelChecks,
		unifiedresources.RelRunsOn,
		unifiedresources.RelHostedBy,
		unifiedresources.RelOwnedBy,
		unifiedresources.RelMemberOf:
		return primary.failureClass == AlertFailureClassRuntime ||
			primary.failureClass == AlertFailureClassEvidenceCoverage ||
			primary.failureClass == AlertFailureClassDependency
	case unifiedresources.RelDependsOn,
		unifiedresources.RelMountedTo,
		unifiedresources.RelStoresOn,
		unifiedresources.RelProtectedBy,
		unifiedresources.RelAttachedTo,
		unifiedresources.RelExposedBy:
		return primary.failureClass != AlertFailureClassCertificate
	default:
		return false
	}
}

func incidentCauseTimingSupported(symptom, primary *Alert) bool {
	if symptom == nil || primary == nil || symptom.StartTime.IsZero() || primary.StartTime.IsZero() {
		return false
	}
	lead := symptom.StartTime.Sub(primary.StartTime)
	return lead >= 0 && lead <= maxSupportedCauseLead
}

func incidentCauseLinkLess(left, right incidentCauseLink, leftPrimary, rightPrimary *incidentSynthesisCandidate) bool {
	if left.supported != right.supported {
		return left.supported
	}
	if left.depth != right.depth {
		return left.depth < right.depth
	}
	if left.confidence != right.confidence {
		return left.confidence > right.confidence
	}
	if rightPrimary == nil {
		return true
	}
	return incidentCandidateLess(leftPrimary, rightPrimary)
}

func incidentSynthesisRoot(storageKey string, links map[string]incidentCauseLink) (string, bool) {
	current := storageKey
	supported := true
	visited := map[string]struct{}{storageKey: {}}
	for depth := 0; depth < maxIncidentSynthesisDepth; depth++ {
		link, ok := links[current]
		if !ok || link.primaryKey == "" {
			return current, supported
		}
		supported = supported && link.supported
		if _, cycle := visited[link.primaryKey]; cycle {
			return storageKey, false
		}
		visited[link.primaryKey] = struct{}{}
		current = link.primaryKey
	}
	return current, false
}

func incidentCauseContradicted(candidate *incidentSynthesisCandidate) bool {
	if candidate == nil {
		return true
	}
	if candidate.resource.ID != "" && candidate.resource.Status == unifiedresources.StatusOnline {
		return true
	}
	if availability := candidate.resource.Availability; availability != nil {
		if availability.Available || strings.EqualFold(availability.AggregateState, "healthy") || availability.Disagreement {
			return true
		}
	}
	return false
}

func incidentSynthesisEvidence(
	memberKeys []string,
	candidates map[string]*incidentSynthesisCandidate,
) ([]AlertCorrelationObservation, []string) {
	observations := make([]AlertCorrelationObservation, 0, min(len(memberKeys), maxIncidentSynthesisObservations))
	affectedSet := make(map[string]struct{})
	for index, storageKey := range memberKeys {
		candidate := candidates[storageKey]
		if candidate == nil || candidate.alert == nil {
			continue
		}
		if index > 0 {
			if resourceID := unifiedresources.CanonicalResourceID(candidate.alert.ResourceID); resourceID != "" {
				affectedSet[resourceID] = struct{}{}
			}
		}
		if len(observations) >= maxIncidentSynthesisObservations {
			continue
		}
		evidenceIDs := make([]string, 0, len(candidate.alert.Evidence))
		for _, evidence := range candidate.alert.Evidence {
			if id := strings.TrimSpace(evidence.ID); id != "" {
				evidenceIDs = append(evidenceIDs, id)
			}
		}
		sort.Strings(evidenceIDs)
		observedAt := candidate.alert.LastSeen
		if observedAt.IsZero() {
			observedAt = candidate.alert.StartTime
		}
		observations = append(observations, AlertCorrelationObservation{
			AlertID:      effectiveAlertID(candidate.alert, candidate.storageKey),
			ResourceID:   candidate.alert.ResourceID,
			ResourceName: candidate.alert.ResourceName,
			FailureClass: candidate.failureClass,
			Level:        candidate.alert.Level,
			ObservedAt:   observedAt,
			EvidenceIDs:  evidenceIDs,
		})
	}
	affected := make([]string, 0, len(affectedSet))
	for resourceID := range affectedSet {
		affected = append(affected, resourceID)
	}
	sort.Strings(affected)
	return observations, affected
}

func cloneCorrelationObservations(input []AlertCorrelationObservation) []AlertCorrelationObservation {
	if len(input) == 0 {
		return nil
	}
	output := make([]AlertCorrelationObservation, len(input))
	for index := range input {
		output[index] = input[index]
		output[index].EvidenceIDs = append([]string(nil), input[index].EvidenceIDs...)
	}
	return output
}

func incidentCandidateLess(left, right *incidentSynthesisCandidate) bool {
	if left == nil || right == nil {
		return right != nil
	}
	if left.alert.Level != right.alert.Level {
		return alertSeveritySortRank(*left.alert) > alertSeveritySortRank(*right.alert)
	}
	if !left.alert.StartTime.Equal(right.alert.StartTime) {
		return left.alert.StartTime.Before(right.alert.StartTime)
	}
	return effectiveAlertID(left.alert, left.storageKey) < effectiveAlertID(right.alert, right.storageKey)
}

func incidentCandidateName(candidate *incidentSynthesisCandidate) string {
	if candidate == nil || candidate.alert == nil {
		return "the related resource"
	}
	if name := strings.TrimSpace(candidate.alert.ResourceName); name != "" {
		return name
	}
	return candidate.alert.ResourceID
}

func failureClassLabel(failureClass AlertFailureClass) string {
	switch failureClass {
	case AlertFailureClassRuntime:
		return "Runtime failure"
	case AlertFailureClassNetworkPath:
		return "Network path failure"
	case AlertFailureClassApplicationResponse:
		return "Application response failure"
	case AlertFailureClassCertificate:
		return "Certificate failure"
	case AlertFailureClassEvidenceCoverage:
		return "Evidence coverage failure"
	default:
		return "Dependency failure"
	}
}
