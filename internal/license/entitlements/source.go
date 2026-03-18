package entitlements

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// EntitlementSource provides entitlement data from any backing store.
// Implementation A: TokenSource (stateless JWT claims for self-hosted).
// Implementation B: DatabaseSource (direct DB lookup for SaaS/hosted) - future.
type EntitlementSource = pkglicensing.EntitlementSource
