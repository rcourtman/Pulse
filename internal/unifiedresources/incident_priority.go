package unifiedresources

func IncidentPriorityForResource(resource *Resource, incident ResourceIncident, category string) int {
	priority := incidentSeverityRank(incident.Severity) * 1000

	switch category {
	case IncidentCategoryRecoverability:
		priority += 500
	case IncidentCategoryProtection:
		priority += 400
	case IncidentCategoryRebuild:
		priority += 300
	case IncidentCategoryCapacity:
		priority += 200
	case IncidentCategoryAvailability:
		priority += 150
	case IncidentCategoryDiskHealth:
		priority += 100
	default:
		priority += 50
	}

	impact := IncidentImpactCount(resource)
	if impact > 99 {
		impact = 99
	}
	priority += impact

	return priority
}

func IncidentImpactCount(resource *Resource) int {
	if resource == nil {
		return 0
	}
	if resource.PBS != nil {
		if resource.PBS.ProtectedWorkloadCount > 0 {
			return resource.PBS.ProtectedWorkloadCount
		}
		if resource.PBS.AffectedDatastoreCount > 0 {
			return resource.PBS.AffectedDatastoreCount
		}
	}
	if resource.Storage != nil {
		return resource.Storage.ConsumerCount
	}
	return 0
}
