import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import approvalBannerSource from '../ApprovalBanner.tsx?raw';
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
    get patrolPendingApprovals() {
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

  it('keeps approval risk chips on the shared MetadataBadge primitive', () => {
    expect(approvalBannerSource).toContain('MetadataBadge');
    expect(approvalBannerSource).toContain('LoadingSpinner');
    expect(approvalBannerSource).toContain('APPROVAL_BANNER_BADGE_PROPS');
    expect(approvalBannerSource).toContain('firstApprovalRisk()!.badgeTone');
    expect(approvalBannerSource).not.toContain('firstApprovalRisk()!.badgeClass');
    expect(approvalBannerSource).not.toMatch(/px-1\.5 py-0\.5 text-\[10px\] font-medium rounded/);
  });

  it('keeps approval action controls on the shared Button primitive', () => {
    expect(approvalBannerSource).toContain('@/components/shared/Button');
    expect(approvalBannerSource).toContain('<Button');
    expect(approvalBannerSource).toContain('variant="success"');
    expect(approvalBannerSource).toContain('variant="secondary"');
    expect(approvalBannerSource).toContain('variant="warningSolid"');
    expect(approvalBannerSource).not.toContain('px-3 py-1.5 bg-green-600 hover:bg-green-700');
    expect(approvalBannerSource).not.toContain('px-3 py-1.5 bg-surface-alt hover:bg-surface-hover');
    expect(approvalBannerSource).not.toContain('px-3 py-1.5 bg-amber-600 hover:bg-amber-700');
  });

  it('uses governed decision wording for a single approval', () => {
    state.pendingApprovals = [approvalRequest()];

    render(() => <ApprovalBanner />);

    expect(screen.getByRole('button', { name: 'Approve fix' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument();
    expect(screen.queryByText('Approve & Execute')).not.toBeInTheDocument();
    expect(screen.queryByText('Deny')).not.toBeInTheDocument();
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

  it('renders a deterministic countdown label for a valid single approval expiry', () => {
    state.pendingApprovals = [approvalRequest({ expiresAt: '2026-03-01T00:06:00Z' })];

    render(() => <ApprovalBanner />);

    expect(screen.getByText('expires 6m 0s')).toBeInTheDocument();
  });

  it('fails closed instead of rendering NaN when a single approval expiry is malformed', () => {
    state.pendingApprovals = [approvalRequest({ expiresAt: 'not-a-date' })];

    render(() => <ApprovalBanner />);

    expect(screen.getByText('expiry unavailable')).toBeInTheDocument();
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
  });

  it('fails closed instead of rendering NaN when a single approval expiry is missing', () => {
    state.pendingApprovals = [
      approvalRequest({ expiresAt: undefined } as Partial<ApprovalRequest>),
    ];

    render(() => <ApprovalBanner />);

    expect(screen.getByText('expiry unavailable')).toBeInTheDocument();
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
  });
});

function approvalRequest(overrides: Partial<ApprovalRequest> = {}): ApprovalRequest {
  return {
    id: 'approval-single',
    toolId: 'investigation_fix',
    command: 'restart service',
    targetType: 'host',
    targetId: 'finding-single',
    targetName: 'node-201',
    context: 'Single approval',
    riskLevel: 'high',
    status: 'pending',
    requestedAt: '2026-03-01T00:01:00Z',
    expiresAt: '2026-03-01T00:05:00Z',
    ...overrides,
  };
}
