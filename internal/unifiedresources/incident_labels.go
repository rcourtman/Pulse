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
		switch resourceBaseType(resource) {
		case ResourceTypeAgent:
			return "Host Availability Issue"
		case ResourceTypeVM:
			return "VM Availability Issue"
		default:
			return "Availability Issue"
		}
	default:
		switch resourceBaseType(resource) {
		case ResourceTypeAgent:
			return "Host Health Issue"
		case ResourceTypeVM:
			return "VM Health Issue"
		default:
			return "Resource Health Issue"
		}
	}
}

func resourceBaseType(resource *Resource) ResourceType {
	if resource == nil {
		return ""
	}
	return CanonicalResourceType(resource.Type)
}
