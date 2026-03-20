package unifiedresources

import (
	"fmt"
	"strings"
)

func parseDelimitedChangeTokens(values []string) []string {
	tokens := make([]string, 0, len(values))
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			normalized := strings.TrimSpace(strings.ToLower(token))
			if normalized == "" {
				continue
			}
			tokens = append(tokens, normalized)
		}
	}
	return tokens
}

// ParseResourceChangeFilters canonicalizes timeline filter values for the
// shared resource-change contract.
func ParseResourceChangeFilters(kinds, sourceTypes, sourceAdapters []string) (ResourceChangeFilters, error) {
	filters := ResourceChangeFilters{}

	if parsed, err := parseResourceChangeKinds(kinds); err != nil {
		return ResourceChangeFilters{}, err
	} else {
		filters.Kinds = parsed
	}
	if parsed, err := parseResourceChangeSourceTypes(sourceTypes); err != nil {
		return ResourceChangeFilters{}, err
	} else {
		filters.SourceTypes = parsed
	}
	if parsed, err := parseResourceChangeSourceAdapters(sourceAdapters); err != nil {
		return ResourceChangeFilters{}, err
	} else {
		filters.SourceAdapters = parsed
	}

	return filters, nil
}

func parseResourceChangeKinds(values []string) ([]ChangeKind, error) {
	parsed := make([]ChangeKind, 0, len(values))
	for _, normalized := range parseDelimitedChangeTokens(values) {
		switch normalized {
		case string(ChangeStateTransition):
			parsed = append(parsed, ChangeStateTransition)
		case string(ChangeRestart):
			parsed = append(parsed, ChangeRestart)
		case string(ChangeConfigUpdate):
			parsed = append(parsed, ChangeConfigUpdate)
		case string(ChangeAnomaly):
			parsed = append(parsed, ChangeAnomaly)
		case string(ChangeRelationship):
			parsed = append(parsed, ChangeRelationship)
		case string(ChangeCapability):
			parsed = append(parsed, ChangeCapability)
		case string(ChangeAlertFired):
			parsed = append(parsed, ChangeAlertFired)
		case string(ChangeAlertAcknowledged):
			parsed = append(parsed, ChangeAlertAcknowledged)
		case string(ChangeAlertUnacknowledged):
			parsed = append(parsed, ChangeAlertUnacknowledged)
		case string(ChangeAlertResolved):
			parsed = append(parsed, ChangeAlertResolved)
		case string(ChangeCommandExecuted):
			parsed = append(parsed, ChangeCommandExecuted)
		case string(ChangeRunbookExecuted):
			parsed = append(parsed, ChangeRunbookExecuted)
		default:
			return nil, fmt.Errorf("invalid kind value %q", normalized)
		}
	}
	return parsed, nil
}

func parseResourceChangeSourceTypes(values []string) ([]ChangeSourceType, error) {
	parsed := make([]ChangeSourceType, 0, len(values))
	for _, normalized := range parseDelimitedChangeTokens(values) {
		switch normalized {
		case string(SourcePlatformEvent):
			parsed = append(parsed, SourcePlatformEvent)
		case string(SourcePulseDiff):
			parsed = append(parsed, SourcePulseDiff)
		case string(SourceHeuristic):
			parsed = append(parsed, SourceHeuristic)
		case string(SourceUserAction):
			parsed = append(parsed, SourceUserAction)
		case string(SourceAgentAction):
			parsed = append(parsed, SourceAgentAction)
		default:
			return nil, fmt.Errorf("invalid sourceType value %q", normalized)
		}
	}
	return parsed, nil
}

func parseResourceChangeSourceAdapters(values []string) ([]ChangeSourceAdapter, error) {
	parsed := make([]ChangeSourceAdapter, 0, len(values))
	for _, normalized := range parseDelimitedChangeTokens(values) {
		switch normalized {
		case string(AdapterDocker):
			parsed = append(parsed, AdapterDocker)
		case string(AdapterProxmox):
			parsed = append(parsed, AdapterProxmox)
		case string(AdapterTrueNAS):
			parsed = append(parsed, AdapterTrueNAS)
		case string(AdapterOpsAgent):
			parsed = append(parsed, AdapterOpsAgent)
		default:
			return nil, fmt.Errorf("invalid sourceAdapter value %q", normalized)
		}
	}
	return parsed, nil
}
