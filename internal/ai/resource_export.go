package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

type unifiedResourceExportDigest struct {
	OrgID               string                              `json:"orgId"`
	DestinationModel    string                              `json:"destinationModel"`
	Summary             string                              `json:"summary"`
	ResourceCount       int                                 `json:"resourceCount"`
	InfrastructureCount int                                 `json:"infrastructureCount"`
	WorkloadCount       int                                 `json:"workloadCount"`
	SensitivityFloor    unifiedresources.DataSensitivity    `json:"sensitivityFloor"`
	RouteDecision       unifiedresources.ModelRouteDecision `json:"routeDecision"`
}

func (s *Service) recordUnifiedResourceExport(destinationModel, summary string, stats unifiedresources.ResourceStats, sensitivityCounts map[unifiedresources.ResourceSensitivity]int, localOnlyCount int, redactionHints []unifiedresources.ResourceRedactionHint) {
	destinationModel = strings.TrimSpace(destinationModel)
	summary = strings.TrimSpace(summary)
	if s == nil || destinationModel == "" || summary == "" {
		return
	}

	s.mu.RLock()
	store := s.resourceExportStore
	orgID := strings.TrimSpace(s.orgID)
	storeOrgID := strings.TrimSpace(s.resourceExportStoreOrgID)
	s.mu.RUnlock()

	if store == nil || (storeOrgID != "" && storeOrgID != orgID) {
		return
	}

	redactions := make([]string, 0, len(redactionHints))
	for _, hint := range redactionHints {
		redaction := strings.TrimSpace(string(hint))
		if redaction == "" {
			continue
		}
		redactions = append(redactions, redaction)
	}
	sort.Strings(redactions)
	redactions = uniqueStrings(redactions)

	sensitivityFloor := unifiedResourceExportSensitivityFloor(sensitivityCounts)
	decision, reason := unifiedResourceExportDecision(sensitivityFloor, localOnlyCount, len(redactions))

	infraCount := stats.ByType[unifiedresources.ResourceTypeAgent] +
		stats.ByType[unifiedresources.ResourceTypeK8sCluster] +
		stats.ByType[unifiedresources.ResourceTypeK8sNode]
	workloadCount := stats.ByType[unifiedresources.ResourceTypeVM] +
		stats.ByType[unifiedresources.ResourceTypeSystemContainer] +
		stats.ByType[unifiedresources.ResourceTypeAppContainer] +
		stats.ByType[unifiedresources.ResourceTypePod] +
		stats.ByType[unifiedresources.ResourceTypeK8sDeployment]

	envelope := unifiedresources.ExportEnvelope{
		DestinationModel: destinationModel,
		DataPayload: map[string]any{
			"summary":             summary,
			"resourceCount":       stats.Total,
			"infrastructureCount": infraCount,
			"workloadCount":       workloadCount,
			"localOnlyCount":      localOnlyCount,
		},
		RouteDecision: unifiedresources.ModelRouteDecision{
			ResourceID:        "unified-resource-context",
			OriginalExport:    unifiedresources.ExportAllowed,
			FinalDecision:     decision,
			AppliedRedactions: redactions,
			RoutingReason:     reason,
		},
		SensitivityFloor: sensitivityFloor,
	}

	hashInput := unifiedResourceExportDigest{
		OrgID:               orgID,
		DestinationModel:    envelope.DestinationModel,
		Summary:             summary,
		ResourceCount:       stats.Total,
		InfrastructureCount: infraCount,
		WorkloadCount:       workloadCount,
		SensitivityFloor:    envelope.SensitivityFloor,
		RouteDecision:       envelope.RouteDecision,
	}
	payload, err := json.Marshal(hashInput)
	if err != nil {
		log.Warn().
			Err(err).
			Str("orgID", orgID).
			Str("destinationModel", destinationModel).
			Msg("failed to hash unified resource export envelope")
		return
	}

	sum := sha256.Sum256(payload)
	record := unifiedresources.ExportAuditRecord{
		ID:           uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Actor:        fmt.Sprintf("ai-service:%s", orgID),
		EnvelopeHash: hex.EncodeToString(sum[:]),
		Decision:     decision,
		Destination:  destinationModel,
		Redactions:   redactions,
	}
	if err := store.RecordExportAudit(record); err != nil {
		log.Warn().
			Err(err).
			Str("orgID", orgID).
			Str("destinationModel", destinationModel).
			Msg("failed to persist unified resource export audit")
	}
}

func unifiedResourceExportSensitivityFloor(counts map[unifiedresources.ResourceSensitivity]int) unifiedresources.DataSensitivity {
	if counts == nil {
		return unifiedresources.SensitivityPublic
	}
	if counts[unifiedresources.ResourceSensitivityRestricted] > 0 {
		return unifiedresources.SensitivityRestricted
	}
	if counts[unifiedresources.ResourceSensitivitySensitive] > 0 {
		return unifiedresources.SensitivitySensitive
	}
	if counts[unifiedresources.ResourceSensitivityInternal] > 0 {
		return unifiedresources.SensitivityInternal
	}
	return unifiedresources.SensitivityPublic
}

func unifiedResourceExportDecision(sensitivityFloor unifiedresources.DataSensitivity, localOnlyCount int, redactionCount int) (unifiedresources.ExportDecision, string) {
	if localOnlyCount > 0 || redactionCount > 0 {
		return unifiedresources.ExportRedacted, "governed unified resource context exported in redacted form"
	}

	switch sensitivityFloor {
	case unifiedresources.SensitivityRestricted, unifiedresources.SensitivitySensitive:
		return unifiedresources.ExportRedacted, "governed unified resource context exported in redacted form"
	case unifiedresources.SensitivityInternal:
		return unifiedresources.ExportRequiresConsent, "internal unified resource context requires export consent"
	default:
		return unifiedresources.ExportAllowed, "public unified resource context"
	}
}

func uniqueStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	out := make([]string, 0, len(values))
	last := ""
	for _, value := range values {
		if value == "" || value == last {
			continue
		}
		out = append(out, value)
		last = value
	}
	return out
}
