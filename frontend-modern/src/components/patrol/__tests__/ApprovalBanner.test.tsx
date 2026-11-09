import { createSignal } from 'solid-js';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ApprovalBanner from '../ApprovalBanner';
import type { ApprovalRequest } from '@/api/ai';

const state = vi.hoisted(() => ({
  pendingApprovals: [] as ApprovalRequest[],
  approvalsError: (() => null) as () => string | null,
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get approvalsError() {
      return state.approvalsError();
    },
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
    state.approvalsError = () => null;
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
        <Route path="/" component={() => <ApprovalBanner />} />
      </Router>
    ));

  it('distinguishes an unavailable cold read from no waiting requests', () => {
    state.approvalsError = () => 'Approval organization is unavailable';
    renderBanner();
    expect(screen.getByRole('status')).toHaveTextContent('Approvals unavailable');
    expect(screen.getByRole('status')).toHaveTextContent('There may still be requests waiting.');
    expect(screen.queryByRole('link', { name: 'Review in Actions' })).not.toBeInTheDocument();
  });

  it('keeps last known requests and review controls visible during a refused refresh', () => {
    state.approvalsError = () => 'Approval organization is unavailable';
    state.pendingApprovals = [approvalRequest({ plan: { actionId: 'retained-action' } })];
    renderBanner();
    expect(screen.getByRole('status')).toHaveTextContent('Last known requests remain below.');
    expect(screen.getByText('Single approval')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Review in Actions' })).toHaveAttribute(
      'href',
      '/actions?action=retained-action',
    );
    expect(screen.queryByRole('button', { name: /approve|reject/i })).not.toBeInTheDocument();
  });

  it('clears the unavailable notice after recovery without hiding retained requests', () => {
    const [error, setError] = createSignal<string | null>('temporary failure');
    state.approvalsError = error;
    state.pendingApprovals = [approvalRequest()];
    renderBanner();
    expect(screen.getByRole('status')).toHaveTextContent('Approvals unavailable');
    setError(null);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Review in Actions' })).toBeInTheDocument();
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

  it("shows only the approvals for a scoped resource's findings", () => {
    state.pendingApprovals = [
      approvalRequest({
        id: 'approval-elsewhere',
        targetId: 'finding-elsewhere',
        context: 'Restart elsewhere',
      }),
      approvalRequest({
        id: 'approval-here',
        targetId: 'finding-here',
        context: 'Restart here',
        expiresAt: '2026-03-01T00:04:00Z',
        plan: { actionId: 'action-here' },
      }),
    ] as ApprovalRequest[];

    render(() => (
      <Router>
        <Route
          path="/"
          component={() => <ApprovalBanner targetIds={new Set(['finding-here'])} />}
        />
      </Router>
    ));

    // The resource's own approval keeps its countdown and exact action.
    expect(screen.getByText('Action awaiting approval')).toBeInTheDocument();
    expect(screen.getByText('Restart here')).toBeInTheDocument();
    expect(screen.getByText('expires 4m 0s')).toBeInTheDocument();
    expect(screen.queryByText(/actions awaiting your approval/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Review in Actions' })).toHaveAttribute(
      'href',
      '/actions?action=action-here',
    );
  });

  it('stays hidden when no approval waits on the scoped findings', () => {
    state.pendingApprovals = [approvalRequest()];

    render(() => (
      <Router>
        <Route
          path="/"
          component={() => <ApprovalBanner targetIds={new Set(['finding-other'])} />}
        />
      </Router>
    ));

    expect(screen.queryByRole('link', { name: 'Review in Actions' })).not.toBeInTheDocument();
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
