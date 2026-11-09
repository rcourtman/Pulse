package api

import (
	"errors"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

var errOIDCCallbackPolicyChanged = errors.New("OIDC callback provider policy changed")

// publishSSOConfigLocked fences callbacks against successful policy changes.
// Reverting a policy to identical values does not revive the old admission.
// Presentation-only edits and changes to other providers preserve admission.
// The caller holds ssoMu; failed persistence never reaches publication.
func (r *Router) publishSSOConfigLocked(next *config.SSOConfig) {
	for id := range r.ssoPolicyRevisions {
		previous, current := r.ssoConfig.GetProvider(id), next.GetProvider(id)
		if !isEnabledOIDCProvider(current) || current.OIDC == nil {
			delete(r.ssoPolicyRevisions, id)
		} else if previous == nil || !ssoPresentationOnlyUpdate(*previous, *current) {
			r.ssoNextPolicyRevision++
			r.ssoPolicyRevisions[id] = r.ssoNextPolicyRevision
		}
	}
	r.ssoConfig = next
}

// currentOIDCCallbackPolicy captures the immutable provider and its admission
// together, before any remote token exchange or signature-key fetch.
func (r *Router) currentOIDCCallbackPolicy(id string) (*config.SSOProvider, uint64) {
	r.ssoMu.Lock()
	defer r.ssoMu.Unlock()
	provider := r.ssoConfig.GetProvider(id)
	if r.ssoAuthLoadFailed.Load() {
		return nil, 0
	}
	if !isEnabledOIDCProvider(provider) || provider.OIDC == nil {
		return provider, 0
	}
	if r.ssoPolicyRevisions == nil {
		r.ssoPolicyRevisions = make(map[string]uint64)
	}
	revision := r.ssoPolicyRevisions[id]
	if revision == 0 {
		r.ssoNextPolicyRevision++
		revision = r.ssoNextPolicyRevision
		r.ssoPolicyRevisions[id] = revision
	}
	return provider, revision
}

// commitOIDCCallback orders local role/identity/session effects against policy
// publication. No IdP or response-body I/O runs under the configuration lock.
// A stale callback must not even replace the browser's existing session.
func (r *Router) commitOIDCCallback(id string, revision uint64, provider *config.SSOProvider, complete func() error) error {
	r.ssoMu.RLock()
	defer r.ssoMu.RUnlock()
	current := r.ssoConfig.GetProvider(id)
	if revision == 0 || r.ssoPolicyRevisions[id] != revision || r.ssoAuthLoadFailed.Load() ||
		provider == nil || current == nil || !ssoPresentationOnlyUpdate(*provider, *current) {
		return errOIDCCallbackPolicyChanged
	}
	return complete()
}
