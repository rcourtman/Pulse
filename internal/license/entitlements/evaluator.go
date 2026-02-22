package entitlements

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// Evaluator is the canonical entitlement evaluator used by all runtime surfaces.
type Evaluator = pkglicensing.Evaluator

// NewEvaluator creates a new evaluator with the given source.
func NewEvaluator(source EntitlementSource) *Evaluator {
	return pkglicensing.NewEvaluator(source)
}
