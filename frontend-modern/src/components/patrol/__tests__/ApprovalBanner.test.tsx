import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ApprovalBanner from '../ApprovalBanner';
import type { ApprovalRequest } from '@/api/ai';
import type { UnifiedFinding } from '@/stores/aiIntelligence';

const state = vi.hoisted(() => ({
  pendingApprovals: [] as ApprovalRequest[],
  findingsWithPendingApprovals: [] as UnifiedFinding[],
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get pendingApprovals() {
      return state.pendingApprovals;
    },
    get findingsWithPendingApprovals() {
      return state.findingsWithPendingApprovals;
    },
    approveInvestigationFix: vi.fn(),
    denyInvestigationFix: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe('ApprovalBanner', () => {
  beforeEach(() => {
    state.pendingApprovals = [];
    state.findingsWithPendingApprovals = [];
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-01T00:00:00Z'));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('reviews the first approval-linked finding in canonical urgency order', () => {
    state.pendingApprovals = [
      {
        id: 'approval-sooner',
        toolId: 'investigation_fix',
        command: 'restart sooner',
        targetType: 'host',
        targetId: 'finding-sooner',
        targetName: 'node-201',
        context: 'Sooner approval',
        riskLevel: 'high',
        status: 'pending',
        requestedAt: '2026-03-01T00:02:00Z',
        expiresAt: '2026-03-01T00:06:00Z',
      },
      {
        id: 'approval-later',
        toolId: 'investigation_fix',
        command: 'restart later',
        targetType: 'host',
        targetId: 'finding-later',
        targetName: 'node-200',
        context: 'Later approval',
        riskLevel: 'low',
        status: 'pending',
        requestedAt: '2026-03-01T00:01:00Z',
        expiresAt: '2026-03-01T00:10:00Z',
      },
    ] as ApprovalRequest[];

    state.findingsWithPendingApprovals = [
      { id: 'finding-sooner' },
      { id: 'finding-later' },
    ] as UnifiedFinding[];

    const onScrollToFinding = vi.fn();
    render(() => <ApprovalBanner onScrollToFinding={onScrollToFinding} />);

    fireEvent.click(screen.getByRole('button', { name: 'Review' }));

    expect(onScrollToFinding).toHaveBeenCalledWith('finding-sooner');
  });
});
