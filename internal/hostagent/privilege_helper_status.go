package hostagent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

type privilegeHelperStatus struct {
	mu        sync.RWMutex
	failures  map[string]string
	updatedAt time.Time
	now       func() time.Time
}

func newPrivilegeHelperStatus() *privilegeHelperStatus {
	status := &privilegeHelperStatus{
		failures: make(map[string]string),
		now:      func() time.Time { return time.Now().UTC() },
	}
	status.updatedAt = status.now()
	return status
}

func (s *privilegeHelperStatus) record(operation string, err error) {
	if s == nil {
		return
	}
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		delete(s.failures, operation)
	} else {
		s.failures[operation] = classifiedPrivilegeHelperStatusError(err)
	}
	s.updatedAt = s.now().UTC()
}

func (s *privilegeHelperStatus) moduleStatus() agentshost.ModuleStatus {
	if s == nil {
		return agentshost.ModuleStatus{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	status := agentshost.ModuleStatus{
		Name:      agentshost.ModuleNameTypedPrivilegeHelper,
		Enabled:   true,
		State:     "running",
		UpdatedAt: s.updatedAt,
	}
	if len(s.failures) == 0 {
		return status
	}

	operations := make([]string, 0, len(s.failures))
	for operation := range s.failures {
		operations = append(operations, operation)
	}
	sort.Strings(operations)
	failures := make([]string, 0, len(operations))
	for _, operation := range operations {
		failures = append(failures, fmt.Sprintf("%s: %s", operation, s.failures[operation]))
	}
	status.State = "degraded"
	status.LastError = strings.Join(failures, "; ")
	return status
}

func classifiedPrivilegeHelperStatusError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "helper operation deadline exceeded"
	}
	if errors.Is(err, errPrivilegeHelperProxmoxInventoryUnavailable) {
		return "helper returned no Proxmox LXC filesystem inventory"
	}
	var remote *agenthelper.RemoteError
	if errors.As(err, &remote) {
		switch remote.Code {
		case agenthelper.ErrorProviderUnavailable:
			return "helper provider unavailable"
		case agenthelper.ErrorDeadlineExceeded:
			return "helper operation deadline exceeded"
		case agenthelper.ErrorArtifactInvalid:
			return "helper rejected an invalid artifact"
		case agenthelper.ErrorStateConflict:
			return "helper operation state conflict"
		default:
			return "helper operation failed"
		}
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return "helper transport unavailable"
	}
	return "helper operation failed"
}

func (a *Agent) recordPrivilegeHelperOperation(operation string, err error) {
	if a == nil || a.privilegeHelperHealth == nil {
		return
	}
	a.privilegeHelperHealth.record(operation, err)
}

const (
	privilegeHelperOperationSMART              = agenthelper.OperationSMARTSnapshot
	privilegeHelperOperationProxmoxFilesystems = agenthelper.OperationProxmoxLXCFilesystems
)
