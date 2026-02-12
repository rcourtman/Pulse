package finding

import "time"

// Finding represents a patrol finding with investigation metadata.
// This is the canonical type shared between the patrol system and
// the investigation orchestrator.
type Finding struct {
	ID                     string     `json:"id"`
	Key                    string     `json:"key,omitempty"`
	Severity               string     `json:"severity"`
	Category               string     `json:"category"`
	ResourceID             string     `json:"resource_id"`
	ResourceName           string     `json:"resource_name"`
	ResourceType           string     `json:"resource_type"`
	Title                  string     `json:"title"`
	Description            string     `json:"description"`
	Recommendation         string     `json:"recommendation,omitempty"`
	Evidence               string     `json:"evidence,omitempty"`
	InvestigationSessionID string     `json:"investigation_session_id,omitempty"`
	InvestigationStatus    string     `json:"investigation_status,omitempty"`
	InvestigationOutcome   string     `json:"investigation_outcome,omitempty"`
	LastInvestigatedAt     *time.Time `json:"last_investigated_at,omitempty"`
	InvestigationAttempts  int        `json:"investigation_attempts"`
}
