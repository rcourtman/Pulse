import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import approvalBannerSource from '../ApprovalBanner.tsx?raw';
import ApprovalBanner from '../ApprovalBanner';
import type { ApprovalRequest } from '@/api/ai';

const state = vi.hoisted(() => ({
  pendingApprovals: [] as ApprovalRequest[],
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get pendingApprovals() {
      return state.pendingApprovals;
    },
    get patrolPendingApprovals() {
      return state.pendingApprovals;
    },
  },
}));

describe('ApprovalBanner', () => {
  beforeEach(() => {
    state.pendingApprovals = [];
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-01T00:00:00Z'));
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
    vi.useRealTimers();
  });

  const renderBanner = () =>
    render(() => (
      <Router>
        <Route path="/" component={ApprovalBanner} />
      </Router>
    ));

  it('keeps approval risk chips on the shared MetadataBadge primitive', () => {
    expect(approvalBannerSource).toContain('MetadataBadge');
    expect(approvalBannerSource).toContain('APPROVAL_BANNER_BADGE_PROPS');
    expect(approvalBannerSource).toContain('firstApprovalRisk()!.badgeTone');
    expect(approvalBannerSource).not.toContain('firstApprovalRisk()!.badgeClass');
    expect(approvalBannerSource).not.toMatch(/px-1\.5 py-0\.5 text-\[10px\] font-medium rounded/);
  });

  it('keeps the Actions handoff on the shared ButtonLink primitive', () => {
    expect(approvalBannerSource).toContain('@/components/shared/Button');
    expect(approvalBannerSource).toContain('<ButtonLink');
    expect(approvalBannerSource).toContain('variant="warningSolid"');
    expect(approvalBannerSource).not.toContain('approveInvestigationFix');
    expect(approvalBannerSource).not.toContain('denyInvestigationFix');
  });

  it('deep-links a single approval to its exact governed action review', () => {
    state.pendingApprovals = [approvalRequest({ plan: { actionId: 'action-single' } })];

    renderBanner();

    expect(screen.getByRole('link', { name: 'Review in Actions' })).toHaveAttribute(
      'href',
      '/actions?action=action-single',
    );
    expect(screen.queryByRole('button', { name: /approve|reject/i })).not.toBeInTheDocument();
  });

  it('routes multiple approvals to the open Actions inbox', () => {
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

    renderBanner();

    expect(screen.getByRole('link', { name: 'Review in Actions' })).toHaveAttribute(
      'href',
      '/actions',
    );
  });

  it('renders a deterministic countdown label for a valid single approval expiry', () => {
    state.pendingApprovals = [approvalRequest({ expiresAt: '2026-03-01T00:06:00Z' })];

    renderBanner();

    expect(screen.getByText('expires 6m 0s')).toBeInTheDocument();
  });

  it('fails closed instead of rendering NaN when a single approval expiry is malformed', () => {
    state.pendingApprovals = [approvalRequest({ expiresAt: 'not-a-date' })];

    renderBanner();

    expect(screen.getByText('expiry unavailable')).toBeInTheDocument();
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
  });

  it('fails closed instead of rendering NaN when a single approval expiry is missing', () => {
    state.pendingApprovals = [
      approvalRequest({ expiresAt: undefined } as Partial<ApprovalRequest>),
    ];

    renderBanner();

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
