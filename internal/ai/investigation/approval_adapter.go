package investigation

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

// ApprovalAdapter adapts the approval.Store to the ApprovalStore interface
type ApprovalAdapter struct {
	store *approval.Store
}

// NewApprovalAdapter creates a new approval adapter
func NewApprovalAdapter(store *approval.Store) *ApprovalAdapter {
	return &ApprovalAdapter{store: store}
}

// Create creates an approval request for an investigation fix
func (a *ApprovalAdapter) Create(appr *Approval) error {
	if a.store == nil {
		return nil // No store configured, skip
	}

	// Map risk level
	riskLevel := approval.RiskLow
	switch appr.RiskLevel {
	case "low":
		riskLevel = approval.RiskLow
	case "medium":
		riskLevel = approval.RiskMedium
	case "high", "critical":
		riskLevel = approval.RiskHigh
	}

	req := &approval.ApprovalRequest{
		ID:         appr.ID,
		ToolID:     "investigation_fix",
		Command:    appr.Command,
		TargetType: "investigation",
		TargetID:   appr.FindingID,
		TargetName: strings.TrimSpace(appr.TargetHost),
		Context:    "Automated fix from patrol investigation: " + appr.Description,
		RiskLevel:  riskLevel,
	}
	if req.TargetName == "" {
		req.TargetName = appr.Description
	}

	return a.store.CreateApproval(req)
}
