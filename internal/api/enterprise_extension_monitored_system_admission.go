package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	monitoredSystemAdmissionPolicyMu sync.RWMutex
	resolveMonitoredSystemPolicyFunc extensions.ResolveMonitoredSystemAdmissionPolicyFunc
)

// SetResolveMonitoredSystemAdmissionPolicy registers a private commercial
// policy hook for monitored-system admission decisions.
func SetResolveMonitoredSystemAdmissionPolicy(fn extensions.ResolveMonitoredSystemAdmissionPolicyFunc) {
	monitoredSystemAdmissionPolicyMu.Lock()
	defer monitoredSystemAdmissionPolicyMu.Unlock()
	resolveMonitoredSystemPolicyFunc = fn
}

func getResolveMonitoredSystemAdmissionPolicy() extensions.ResolveMonitoredSystemAdmissionPolicyFunc {
	monitoredSystemAdmissionPolicyMu.RLock()
	defer monitoredSystemAdmissionPolicyMu.RUnlock()
	return resolveMonitoredSystemPolicyFunc
}
