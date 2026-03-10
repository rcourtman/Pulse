package alerts

const canonicalStateSeparator = "::"

func buildCanonicalStateID(resourceID, specID string) string {
	if resourceID == "" || specID == "" {
		return ""
	}
	return resourceID + canonicalStateSeparator + specID
}

func backfillCanonicalIdentity(alert *Alert) {
	if alert == nil {
		return
	}
	if alert.CanonicalSpecID == "" && alert.Metadata != nil {
		if specID, ok := alert.Metadata["canonicalSpecID"].(string); ok {
			alert.CanonicalSpecID = specID
		}
	}
	if alert.CanonicalKind == "" && alert.Metadata != nil {
		if kind, ok := alert.Metadata["canonicalAlertKind"].(string); ok {
			alert.CanonicalKind = kind
		}
	}
	if alert.CanonicalState == "" {
		alert.CanonicalState = buildCanonicalStateID(alert.ResourceID, alert.CanonicalSpecID)
	}
}

func applyCanonicalIdentity(alert *Alert, specID, kind string) {
	if alert == nil {
		return
	}
	alert.CanonicalSpecID = specID
	alert.CanonicalKind = kind
	alert.CanonicalState = buildCanonicalStateID(alert.ResourceID, specID)
	if alert.Metadata == nil {
		alert.Metadata = make(map[string]interface{}, 2)
	}
	alert.Metadata["canonicalSpecID"] = specID
	alert.Metadata["canonicalAlertKind"] = kind
}
