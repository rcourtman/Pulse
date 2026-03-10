package specs

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func BuildUnifiedResourceAlertSpecs(resources []unifiedresources.Resource) []ResourceAlertSpec {
	prepared := make([]unifiedresources.Resource, 0, len(resources))
	childrenByParent := make(map[string][]string, len(resources))

	for _, resource := range resources {
		preparedResource := resource
		preparedResource.ID = unifiedresources.CanonicalResourceID(resource.ID)
		preparedResource.Type = unifiedresources.CanonicalResourceType(resource.Type)
		if resource.ParentID != nil {
			parentID := unifiedresources.CanonicalResourceID(*resource.ParentID)
			if parentID != "" {
				preparedResource.ParentID = &parentID
				childrenByParent[parentID] = append(childrenByParent[parentID], preparedResource.ID)
			} else {
				preparedResource.ParentID = nil
			}
		}
		prepared = append(prepared, preparedResource)
	}

	for parentID := range childrenByParent {
		sort.Strings(childrenByParent[parentID])
	}

	specs := make([]ResourceAlertSpec, 0, len(prepared)*2)
	for _, resource := range prepared {
		specs = append(specs, buildProviderIncidentSpecs(resource, childrenByParent)...)
		if rollup := buildIncidentRollupSpec(resource, childrenByParent); rollup != nil {
			specs = append(specs, *rollup)
		}
	}

	sort.SliceStable(specs, func(i, j int) bool {
		if specs[i].ResourceID != specs[j].ResourceID {
			return specs[i].ResourceID < specs[j].ResourceID
		}
		if specs[i].Kind != specs[j].Kind {
			return specs[i].Kind < specs[j].Kind
		}
		return specs[i].ID < specs[j].ID
	})

	return specs
}

func buildProviderIncidentSpecs(resource unifiedresources.Resource, childrenByParent map[string][]string) []ResourceAlertSpec {
	if len(resource.Incidents) == 0 {
		return nil
	}

	out := make([]ResourceAlertSpec, 0, len(resource.Incidents))
	for _, incident := range resource.Incidents {
		spec := ResourceAlertSpec{
			ID:               hashedSpecID(AlertSpecKindProviderIncident, resource.ID, incident.Provider, incident.NativeID, incident.Code),
			ResourceID:       resource.ID,
			ResourceType:     resource.Type,
			ParentResourceID: parentResourceID(resource),
			ChildResourceIDs: append([]string(nil), childrenByParent[resource.ID]...),
			SuppressionKeys:  suppressionKeys([]unifiedresources.ResourceIncident{incident}),
			Kind:             AlertSpecKindProviderIncident,
			Severity:         riskSeverity(incident.Severity),
			Title:            firstTrimmed(incident.Summary, incident.Code),
			ProviderIncident: &ProviderIncidentSpec{
				Provider:  strings.TrimSpace(incident.Provider),
				Codes:     canonicalStringSet([]string{incident.Code}),
				NativeIDs: canonicalStringSet([]string{incident.NativeID}),
			},
		}
		out = append(out, spec)
	}
	return out
}

func buildIncidentRollupSpec(resource unifiedresources.Resource, childrenByParent map[string][]string) *ResourceAlertSpec {
	if len(resource.Incidents) == 0 {
		return nil
	}

	primary := primaryRollupIncident(resource)
	if primary == nil {
		return nil
	}

	spec := ResourceAlertSpec{
		ID:               hashedSpecID(AlertSpecKindResourceIncidentRollup, resource.ID, resource.IncidentCode, primary.Code),
		ResourceID:       resource.ID,
		ResourceType:     resource.Type,
		ParentResourceID: parentResourceID(resource),
		ChildResourceIDs: append([]string(nil), childrenByParent[resource.ID]...),
		SuppressionKeys:  suppressionKeys(resource.Incidents),
		Kind:             AlertSpecKindResourceIncidentRollup,
		Severity:         riskSeverity(primary.Severity),
		Title:            firstTrimmed(resource.IncidentSummary, primary.Summary, primary.Code),
		ResourceIncidentRollup: &ResourceIncidentRollupSpec{
			Code:          firstTrimmed(resource.IncidentCode, primary.Code),
			IncidentCount: len(resource.Incidents),
			StartedAt:     primary.StartedAt,
		},
	}

	return &spec
}

func primaryRollupIncident(resource unifiedresources.Resource) *unifiedresources.ResourceIncident {
	if len(resource.Incidents) == 0 {
		return nil
	}

	code := strings.TrimSpace(resource.IncidentCode)
	summary := strings.TrimSpace(resource.IncidentSummary)
	for i := range resource.Incidents {
		incident := &resource.Incidents[i]
		if code != "" && strings.TrimSpace(incident.Code) != code {
			continue
		}
		if summary != "" && strings.TrimSpace(incident.Summary) != summary {
			continue
		}
		if resource.IncidentSeverity != "" && incident.Severity != resource.IncidentSeverity {
			continue
		}
		return incident
	}

	return &resource.Incidents[0]
}

func parentResourceID(resource unifiedresources.Resource) string {
	if resource.ParentID == nil {
		return ""
	}
	return unifiedresources.CanonicalResourceID(*resource.ParentID)
}

func suppressionKeys(incidents []unifiedresources.ResourceIncident) []string {
	if len(incidents) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(incidents))
	keys := make([]string, 0, len(incidents))
	for _, incident := range incidents {
		key := strings.TrimSpace(incident.Provider) + "|" + strings.TrimSpace(incident.NativeID) + "|" + strings.TrimSpace(incident.Code)
		if key == "||" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	return keys
}

func riskSeverity(level storagehealth.RiskLevel) AlertSeverity {
	switch level {
	case storagehealth.RiskCritical:
		return AlertSeverityCritical
	case storagehealth.RiskWarning:
		return AlertSeverityWarning
	default:
		return AlertSeverityInfo
	}
}

func hashedSpecID(kind AlertSpecKind, parts ...string) string {
	normalized := make([]string, 0, len(parts)+1)
	normalized = append(normalized, string(kind))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x1f")))
	return "alertspec:" + string(kind) + ":" + hex.EncodeToString(sum[:8])
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
