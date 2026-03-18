package licensing

import "context"

// FeatureChecker exposes feature-gate checks for the current tenant/license context.
type FeatureChecker interface {
	RequireFeature(feature string) error
	HasFeature(feature string) bool
}

// FeatureServiceResolver resolves a feature checker for the request context.
type FeatureServiceResolver interface {
	FeatureService(ctx context.Context) FeatureChecker
}
