package unifiedresources

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	MetadataActivityType     = "activity_type"
	MetadataActivityNativeID = "activity_native_id"
	MetadataActivityTitle    = "activity_title"
	MetadataActivityState    = "activity_state"
	MetadataActivityMessage  = "activity_message"
)

// PlatformActivityChange captures read-only provider activity that should be
// preserved on the canonical resource timeline as investigation context.
type PlatformActivityChange struct {
	SourceAdapter    ChangeSourceAdapter
	ActivityType     string
	NativeID         string
	Title            string
	State            string
	Message          string
	Actor            string
	OccurredAt       time.Time
	RelatedResources []string
	Metadata         map[string]any
}

// BuildPlatformActivityChange constructs a canonical timeline change for
// provider-emitted read-side activity such as VMware tasks or events.
func BuildPlatformActivityChange(resourceID string, activity PlatformActivityChange) *ResourceChange {
	resourceID = CanonicalResourceID(resourceID)
	if resourceID == "" {
		return nil
	}

	sourceAdapter := ChangeSourceAdapter(strings.TrimSpace(string(activity.SourceAdapter)))
	if sourceAdapter == "" {
		return nil
	}

	activityType := strings.TrimSpace(activity.ActivityType)
	nativeID := strings.TrimSpace(activity.NativeID)
	title := strings.TrimSpace(activity.Title)
	state := strings.TrimSpace(activity.State)
	message := strings.TrimSpace(activity.Message)
	if activityType == "" || (nativeID == "" && title == "" && message == "") {
		return nil
	}

	observedAt := activity.OccurredAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	change := &ResourceChange{
		ID:            platformActivityChangeID(resourceID, sourceAdapter, activityType, nativeID, observedAt, title, message),
		ObservedAt:    observedAt,
		ResourceID:    resourceID,
		Kind:          ChangeActivity,
		SourceType:    SourcePlatformEvent,
		SourceAdapter: sourceAdapter,
		Confidence:    ConfidenceHigh,
		Actor:         strings.TrimSpace(activity.Actor),
		Reason:        platformActivityReason(activityType, title, state, message),
		Metadata:      cloneChangeMetadata(activity.Metadata),
	}
	if !activity.OccurredAt.IsZero() {
		occurredAt := activity.OccurredAt.UTC()
		change.OccurredAt = &occurredAt
	}

	if _, exists := change.Metadata[MetadataActivityType]; !exists && activityType != "" {
		change.Metadata[MetadataActivityType] = activityType
	}
	if _, exists := change.Metadata[MetadataActivityNativeID]; !exists && nativeID != "" {
		change.Metadata[MetadataActivityNativeID] = nativeID
	}
	if _, exists := change.Metadata[MetadataActivityTitle]; !exists && title != "" {
		change.Metadata[MetadataActivityTitle] = title
	}
	if _, exists := change.Metadata[MetadataActivityState]; !exists && state != "" {
		change.Metadata[MetadataActivityState] = state
	}
	if _, exists := change.Metadata[MetadataActivityMessage]; !exists && message != "" {
		change.Metadata[MetadataActivityMessage] = message
	}

	change.RelatedResources = uniqueTrimmed(activity.RelatedResources...)
	return change
}

func platformActivityChangeID(resourceID string, sourceAdapter ChangeSourceAdapter, activityType, nativeID string, occurredAt time.Time, title, message string) string {
	base := nativeID
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("%s|%s|%s", occurredAt.UTC().Format(time.RFC3339Nano), strings.TrimSpace(title), strings.TrimSpace(message))
	}
	sum := sha1.Sum([]byte(strings.Join([]string{
		CanonicalResourceID(resourceID),
		strings.TrimSpace(string(sourceAdapter)),
		strings.TrimSpace(activityType),
		base,
	}, "|")))
	return "activity-" + hex.EncodeToString(sum[:])
}

func platformActivityReason(activityType, title, state, message string) string {
	activityType = strings.TrimSpace(strings.ReplaceAll(activityType, "_", " "))
	title = strings.TrimSpace(title)
	state = strings.TrimSpace(state)
	message = strings.TrimSpace(message)

	if title != "" && state != "" {
		return fmt.Sprintf("%s (%s)", title, state)
	}
	if title != "" {
		return title
	}
	if message != "" {
		return message
	}
	if activityType == "" {
		return "Provider activity recorded"
	}
	return fmt.Sprintf("%s recorded", strings.ToUpper(activityType[:1])+activityType[1:])
}
