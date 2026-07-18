package alerts

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// Alert represents an active alert
type Alert struct {
	ID                string                                 `json:"id"`
	Type              string                                 `json:"type"` // cpu, memory, disk, etc.
	Level             AlertLevel                             `json:"level"`
	ResourceID        string                                 `json:"resourceId"` // guest or node ID
	CanonicalSpecID   string                                 `json:"canonicalSpecId,omitempty"`
	CanonicalKind     string                                 `json:"canonicalKind,omitempty"`
	CanonicalState    string                                 `json:"canonicalState,omitempty"`
	ResourceName      string                                 `json:"resourceName"`
	Node              string                                 `json:"node"`
	NodeDisplayName   string                                 `json:"nodeDisplayName,omitempty"`
	Instance          string                                 `json:"instance"`
	Message           string                                 `json:"message"`
	Value             float64                                `json:"value"`
	Threshold         float64                                `json:"threshold"`
	StartTime         time.Time                              `json:"startTime"`
	LastSeen          time.Time                              `json:"lastSeen"`
	Acknowledged      bool                                   `json:"acknowledged"`
	AckTime           *time.Time                             `json:"ackTime,omitempty"`
	AckUser           string                                 `json:"ackUser,omitempty"`
	Metadata          map[string]interface{}                 `json:"metadata,omitempty"`
	LastNotified      *time.Time                             `json:"lastNotified,omitempty"`
	LastEscalation    int                                    `json:"lastEscalation,omitempty"`
	EscalationTimes   []time.Time                            `json:"escalationTimes,omitempty"`
	OperationalRecord *operationaltrust.OperationalRecord    `json:"operationalRecord,omitempty"`
	LatestTransition  *operationaltrust.LifecycleTransition  `json:"latestTransition,omitempty"`
	Transitions       []operationaltrust.LifecycleTransition `json:"transitions,omitempty"`
	Evidence          []operationaltrust.EvidenceEnvelope    `json:"evidence,omitempty"`
}

// Clone returns a deep copy of the alert so it can be safely shared across goroutines.
func (a *Alert) Clone() *Alert {
	if a == nil {
		return nil
	}

	clone := *a

	if a.AckTime != nil {
		t := *a.AckTime
		clone.AckTime = &t
	}

	if a.LastNotified != nil {
		t := *a.LastNotified
		clone.LastNotified = &t
	}

	if len(a.EscalationTimes) > 0 {
		clone.EscalationTimes = append([]time.Time(nil), a.EscalationTimes...)
	}

	if a.Metadata != nil {
		clone.Metadata = cloneMetadata(a.Metadata)
	}

	if a.OperationalRecord != nil {
		value := a.OperationalRecord.Clone()
		clone.OperationalRecord = &value
	}

	if a.LatestTransition != nil {
		value := a.LatestTransition.Clone()
		clone.LatestTransition = &value
	}

	if len(a.Transitions) > 0 {
		clone.Transitions = make([]operationaltrust.LifecycleTransition, len(a.Transitions))
		for index := range a.Transitions {
			clone.Transitions[index] = a.Transitions[index].Clone()
		}
	}

	if len(a.Evidence) > 0 {
		clone.Evidence = make([]operationaltrust.EvidenceEnvelope, len(a.Evidence))
		for index := range a.Evidence {
			clone.Evidence[index] = a.Evidence[index].Clone()
		}
	}

	return &clone
}

func cloneMetadata(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}

	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = cloneMetadataValue(v)
	}
	return dst
}

func cloneMetadataValue(val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		return cloneMetadata(v)
	case map[string]string:
		m := make(map[string]interface{}, len(v))
		for key, value := range v {
			m[key] = value
		}
		return m
	case []interface{}:
		arr := make([]interface{}, len(v))
		for i, elem := range v {
			arr[i] = cloneMetadataValue(elem)
		}
		return arr
	case []string:
		arr := make([]string, len(v))
		copy(arr, v)
		return arr
	case []int:
		arr := make([]int, len(v))
		copy(arr, v)
		return arr
	case []float64:
		arr := make([]float64, len(v))
		copy(arr, v)
		return arr
	default:
		return v
	}
}

// ResolvedAlert represents a recently resolved alert
type ResolvedAlert struct {
	*Alert
	ResolvedTime time.Time `json:"resolvedTime"`
}
