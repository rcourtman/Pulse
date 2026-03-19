package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	redactions := unifiedresources.ResourceRedactionLabelsFromHints(redactionHints)

	sensitivityFloor := unifiedresources.ExportSensitivityFloor(sensitivityCounts)
	decision, reason := unifiedresources.ExportDecisionForContext(sensitivityFloor, localOnlyCount, len(redactions))

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
