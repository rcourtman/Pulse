package ai

import "time"

// observeProxmoxGuestLifecycle runs the built-in stopped-guest check, applies
// what it resolves and returns the findings it raises.
//
// The check compares each guest with what it saw last time, so it must always
// be given the whole collected state, never a scoped one: a guest missing from
// its input is forgotten, and a finding on it is resolved as removed. It used
// to run only in a full run, so a guest that died between two full runs got
// nothing from it for hours, while the alert runs that looked at the guest in
// the meantime never asked it.
func (p *PatrolService) observeProxmoxGuestLifecycle(state patrolRuntimeState) []*Finding {
	if p == nil || p.proxmoxGuestLifecycleWatcher == nil || p.findings == nil {
		return nil
	}
	emit, resolve := p.proxmoxGuestLifecycleWatcher.Observe(state, p.findings.GetActive(FindingSeverityInfo), time.Now())
	for _, sentinel := range resolve {
		if p.findings.ResolveWithReason(sentinel.DedupKey, sentinel.Reason) {
			p.mu.RLock()
			resolveUnified := p.unifiedFindingResolver
			p.mu.RUnlock()
			if resolveUnified != nil {
				resolveUnified(sentinel.DedupKey)
			}
		}
	}
	return emit
}
