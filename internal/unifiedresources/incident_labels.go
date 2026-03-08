package unifiedresources

func IncidentLabelForResource(resource *Resource, incident ResourceIncident, category string) string {
	switch category {
	case IncidentCategoryProtection:
		return "Protection Reduced"
	case IncidentCategoryRebuild:
		return "Rebuild In Progress"
	case IncidentCategoryCapacity:
		return "Capacity Pressure"
	case IncidentCategoryRecoverability:
		return "Backup Coverage At Risk"
	case IncidentCategoryDiskHealth:
		return "Disk Health Risk"
	case IncidentCategoryAvailability:
		return "Storage Availability Issue"
	default:
		return "Storage Health Issue"
	}
}
