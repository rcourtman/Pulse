package audit

// EventProjection is the API representation of an audit event with its
// authoritative signature version, status, and assurance. SignatureValid is a
// compatibility field; clients must not use it to equate compatibility and
// strong evidence.
type EventProjection struct {
	Event
	SignatureVersion   SignatureVersion   `json:"signature_version"`
	SignatureStatus    VerificationStatus `json:"signature_status"`
	SignatureAssurance SignatureAssurance `json:"signature_assurance"`
	SignatureValid     *bool              `json:"signature_valid,omitempty"`
}

// ProjectEvent classifies one event for API and UI consumers.
func ProjectEvent(event Event, verifier any) EventProjection {
	result := ClassifySignature(verifier, event)
	projection := EventProjection{
		Event:              event,
		SignatureVersion:   result.Version,
		SignatureStatus:    result.Status,
		SignatureAssurance: result.Assurance,
	}
	if result.Status != VerificationStatusUnsigned {
		valid := result.Verified
		projection.SignatureValid = &valid
	}
	return projection
}

// ProjectEvents classifies an event page without changing its ordering.
func ProjectEvents(events []Event, verifier any) []EventProjection {
	projected := make([]EventProjection, len(events))
	for i, event := range events {
		projected[i] = ProjectEvent(event, verifier)
	}
	return projected
}

// VerificationMessage is the operator-facing explanation for a classified
// signature result.
func VerificationMessage(result SignatureVerification) string {
	switch result.Status {
	case VerificationStatusStrong:
		return "Event signature strongly verified"
	case VerificationStatusCompatibility:
		return "Historical signature verified with compatibility assurance only"
	case VerificationStatusInvalid:
		return "Event signature verification failed"
	case VerificationStatusUnsigned:
		return "Event is unsigned"
	default:
		return "Signature version is unknown or the verification key is unavailable"
	}
}
