import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { FindingsPanel } from '../FindingsPanel';

function makeFinding(overrides: Record<string, unknown>) {
  return {
    id: 'finding-1',
    source: 'ai-patrol',
    resourceId: 'node-a/storage/tank',
    resourceName: 'tank',
    resourceType: 'storage',
    category: 'capacity',
    severity: 'warning',
    title: 'Storage pool tank approaching threshold',
    description: 'Pool tank is filling up.',
    detectedAt: '2026-04-01T10:00:00Z',
    lastSeenAt: '2026-04-01T10:05:00Z',
    status: 'active',
    ...overrides,
  };
}

const mockState = vi.hoisted(() => {
  const loadFindings = vi.fn();
  const loadPatrolFindings = vi.fn();
  const loadRemediationPlans = vi.fn();
  return {
    findings: [] as unknown[],
    remediationPlans: [] as unknown[],
    loadFindings,
    loadPatrolFindings,
    loadRemediationPlans,
  };
});

vi.mock('@solidjs/router', () => ({
  A: (props: { href: string; children?: JSX.Element; [key: string]: unknown }) => (
    <a href={props.href} aria-label={props['aria-label'] as string} onClick={props.onClick as any}>
      {props.children}
    </a>
  ),
  useLocation: () => ({ hash: '' }),
}));

vi.mock('@/components/shared/Card', () => ({
  Card: (props: { children?: JSX.Element }) => <div>{props.children}</div>,
}));

vi.mock('@/components/patrol', () => ({
  InvestigationSection: () => null,
  ApprovalSection: () => null,
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: {
    open: vi.fn(),
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    approveRemediationPlan: vi.fn(),
  },
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    get: () => undefined,
  }),
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get findings() {
      return mockState.findings;
    },
    get findingsLoading() {
      return false;
    },
    get findingsError() {
      return null;
    },
    get patrolFindings() {
      return mockState.findings;
    },
    get patrolFindingsLoading() {
      return false;
    },
    get patrolFindingsError() {
      return null;
    },
    get findingsNeedingAttention() {
      return [];
    },
    get patrolFindingsNeedingAttention() {
      return [];
    },
    get findingsWithPendingApprovals() {
      return [];
    },
    get patrolFindingsWithPendingApprovals() {
      return [];
    },
    get remediationPlans() {
      return mockState.remediationPlans;
    },
    findingsSignal: () => mockState.findings,
    patrolFindingsSignal: () => mockState.findings,
    loadFindings: mockState.loadFindings,
    loadPatrolFindings: mockState.loadPatrolFindings,
    loadRemediationPlans: mockState.loadRemediationPlans,
  },
}));

beforeEach(() => {
  mockState.findings = [];
  mockState.remediationPlans = [];
  mockState.loadFindings.mockClear();
  mockState.loadPatrolFindings.mockClear();
  mockState.loadRemediationPlans.mockClear();
  if (typeof window.requestAnimationFrame !== 'function') {
    window.requestAnimationFrame = ((callback: FrameRequestCallback) =>
      window.setTimeout(() => callback(performance.now()), 0)) as typeof window.requestAnimationFrame;
  }
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('FindingsPanel overdue commitments filter', () => {
  // The Overdue chip lets operators see will_fix_later commitments whose
  // RemindAt deadline has passed without waiting for the hourly sweep
  // (SweepWillFixLaterReminders) to re-surface them. The filter must
  // include only past-deadline will_fix_later findings and exclude
  // future-dated commitments and unrelated active findings.
  it('renders only past-deadline will_fix_later findings when Overdue chip is clicked', async () => {
    const past = '2026-01-01T00:00:00Z';
    const future = '2099-01-01T00:00:00Z';

    const overdue = makeFinding({
      id: 'overdue-1',
      title: 'Storage pool past remind deadline',
      status: 'dismissed',
      dismissedReason: 'will_fix_later',
      remindAt: past,
    });
    const pending = makeFinding({
      id: 'pending-1',
      title: 'CPU growth still within commitment window',
      status: 'dismissed',
      dismissedReason: 'will_fix_later',
      remindAt: future,
    });
    const active = makeFinding({
      id: 'active-1',
      title: 'Active finding with no dismissal',
    });

    mockState.findings = [overdue, pending, active];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    // The Overdue chip appears in the filter bar.
    const overdueChip = screen.getByTestId('findings-panel-filter-overdue');
    expect(overdueChip).toHaveTextContent(/Overdue commitments \(1\)/);
    fireEvent.click(overdueChip);

    await waitFor(() => {
      expect(screen.getByText('Storage pool past remind deadline')).toBeInTheDocument();
    });
    expect(screen.queryByText('CPU growth still within commitment window')).not.toBeInTheDocument();
    expect(screen.queryByText('Active finding with no dismissal')).not.toBeInTheDocument();
  });

  it('hides the Overdue chip when no will_fix_later commitment is past its deadline', async () => {
    const future = '2099-01-01T00:00:00Z';
    mockState.findings = [
      makeFinding({
        id: 'pending-1',
        title: 'Future commitment',
        status: 'dismissed',
        dismissedReason: 'will_fix_later',
        remindAt: future,
      }),
      makeFinding({ id: 'active-1', title: 'Active finding' }),
    ];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());
    expect(screen.queryByTestId('findings-panel-filter-overdue')).not.toBeInTheDocument();
  });

  it('honors filterOverride="overdue" so callers can land directly on the filter', async () => {
    const past = '2026-01-01T00:00:00Z';
    mockState.findings = [
      makeFinding({
        id: 'overdue-1',
        title: 'Only overdue should render',
        status: 'dismissed',
        dismissedReason: 'will_fix_later',
        remindAt: past,
      }),
      makeFinding({ id: 'active-1', title: 'Active finding with no dismissal' }),
    ];

    render(() => <FindingsPanel findingsSource="patrol" filterOverride="overdue" />);

    await waitFor(() => {
      expect(screen.getByText('Only overdue should render')).toBeInTheDocument();
    });
    expect(screen.queryByText('Active finding with no dismissal')).not.toBeInTheDocument();
  });
});
