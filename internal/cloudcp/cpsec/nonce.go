// Package cpsec provides shared security primitives for the Cloud CP server
// and its sub-packages.
package cpsec

import "context"

// nonceKey is the context key for the per-request CSP nonce.
type nonceKey struct{}

// WithNonce returns a copy of ctx carrying the given CSP nonce.
func WithNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, nonceKey{}, nonce)
}

// NonceFromContext returns the CSP nonce stored in ctx, or "" if absent.
func NonceFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(nonceKey{}).(string); ok {
		return v
	}
	return ""
}
