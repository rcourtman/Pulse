package monitoring

import (
	"context"
)

func (m *Monitor) efficientQEMUWorkerCount(total int) int {
	if total <= 0 {
		return 0
	}

	limit := defaultGuestAgentVMMaxConcurrent
	if m != nil && m.guestAgentWorkSlots != nil && cap(m.guestAgentWorkSlots) > 0 {
		limit = cap(m.guestAgentWorkSlots)
	}
	if limit <= 0 || limit > total {
		return total
	}
	return limit
}

func (m *Monitor) acquireGuestAgentWorkSlot(ctx context.Context) bool {
	if m == nil || m.guestAgentWorkSlots == nil {
		return true
	}
	select {
	case m.guestAgentWorkSlots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *Monitor) releaseGuestAgentWorkSlot() {
	if m == nil || m.guestAgentWorkSlots == nil {
		return
	}
	select {
	case <-m.guestAgentWorkSlots:
	default:
	}
}

func (m *Monitor) guestAgentVMWorkContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if m == nil || m.guestAgentVMBudget <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, m.guestAgentVMBudget)
}

func (m *Monitor) runGuestAgentVMWork(ctx context.Context, work func(context.Context)) bool {
	if !m.acquireGuestAgentWorkSlot(ctx) {
		return false
	}
	defer m.releaseGuestAgentWorkSlot()

	workCtx, cancel := m.guestAgentVMWorkContext(ctx)
	defer cancel()
	work(workCtx)
	return true
}
