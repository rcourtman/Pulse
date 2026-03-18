package unifiedresources

import "time"

// DataSensitivity classifies data to govern what may leave the local boundary.
type DataSensitivity string

const (
	SensitivityPublic     DataSensitivity = "public"     // Completely safe for cloud model ingestion
	SensitivityInternal   DataSensitivity = "internal"   // Requires active "Cloud Export" organizational toggle
	SensitivitySensitive  DataSensitivity = "sensitive"  // Local or Private models only, unless explicitly redacted
	SensitivityRestricted DataSensitivity = "restricted" // Never allowed to leave the trust boundary (e.g. raw unstructured logs)
)

// ExportDecision defines the deterministic routing result for a piece of context.
type ExportDecision string

const (
	ExportAllowed         ExportDecision = "allowed"
	ExportRequiresConsent ExportDecision = "requires_consent"
	ExportRedacted        ExportDecision = "redacted"
	ExportDenied          ExportDecision = "denied"
)

// RedactionRule provides explicit structure on how to scrub outgoing data.
type RedactionRule struct {
	FieldPath            string            `json:"fieldPath"`     // e.g., "metadata.hostname"
	RedactionType        string            `json:"redactionType"` // e.g., "hash", "mask", "drop"
	ApplyToSensitivities []DataSensitivity `json:"applyToSensitivities"`
}

// ModelRouteDecision holds the result of the Privacy Engine's firewall evaluation.
type ModelRouteDecision struct {
	ResourceID        string         `json:"resourceId"`
	OriginalExport    ExportDecision `json:"originalExport"`
	FinalDecision     ExportDecision `json:"finalDecision"`
	AppliedRedactions []string       `json:"appliedRedactions,omitempty"`
	RoutingReason     string         `json:"routingReason"`
}

// ExportEnvelope represents the actual payload and its governing rules leaving the trust boundary.
type ExportEnvelope struct {
	DestinationModel string             `json:"destinationModel"` // e.g. "gpt-4-turbo" or "local-llama"
	DataPayload      map[string]any     `json:"dataPayload"`
	RouteDecision    ModelRouteDecision `json:"routeDecision"`
	SensitivityFloor DataSensitivity    `json:"sensitivityFloor"` // The highest classification within the payload
}

// ExportAuditRecord durably tracks data leaving the AI Firewall.
type ExportAuditRecord struct {
	ID           string         `json:"id"`
	Timestamp    time.Time      `json:"timestamp"`
	Actor        string         `json:"actor"`
	EnvelopeHash string         `json:"envelopeHash"`
	Decision     ExportDecision `json:"decision"`
	Destination  string         `json:"destination"`
	Redactions   []string       `json:"redactions,omitempty"`
}
