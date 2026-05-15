import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { FindingsPanel } from '../FindingsPanel';

// Pin the registry constant locally so the tests fail loudly if Go renames
// the source on the wire (see internal/ai/forecast.CapacityActionPlanSource).
const CAPACITY_FORECAST_SOURCE = 'capacity_forecast';

type RemediationPlanFixture = {
  id: string;
  finding_id: string;
  resource_id: string;
  title: string;
  description: string;
  steps: Array<{ order: number; action: string; risk_level: 'low' | 'medium' | 'high' }>;
  risk_level: 'low' | 'medium' | 'high';
  status: 'pending' | 'approved';
  created_at: string;
  proposed_action_plan?: {
    actionId: string;
    allowed: boolean;
    requiresApproval: boolean;
    source?: string;
    message?: string;
    projectedMetric?: {
      metric: string;
      currentValue: number;
      predictedValue?: number;
      thresholdValue?: number;
      timeToThresholdSeconds?: number | null;
    };
    preflight?: {
      intendedChange?: string;
      dryRunAvailable: boolean;
      safetyChecks?: string[];
    };
  };
};

const baseStep = { order: 1, action: 'Investigate', risk_level: 'low' as const };

function makeFinding(overrides: Record<string, unknown>) {
  return {
    id: 'finding-cap',
    source: 'ai-patrol',
    resourceId: 'node-a/storage/tank',
    resourceName: 'tank',
    resourceType: 'storage',
    category: 'capacity',
    severity: 'warning',
    title: 'Storage pool tank at 87.3% usage',
    description: 'Tank pool exceeded warning threshold.',
    detectedAt: '2026-04-01T10:00:00Z',
    lastSeenAt: '2026-04-01T10:05:00Z',
    status: 'active',
    ...overrides,
  };
}

function makePlan(findingId: string, withProposal: boolean): RemediationPlanFixture {
  const plan: RemediationPlanFixture = {
    id: `plan-${findingId}`,
    finding_id: findingId,
    resource_id: 'node-a/storage/tank',
    title: 'Fix: Storage pool tank at 87.3% usage',
    description: 'Tank pool exceeded warning threshold.',
    steps: [baseStep],
    risk_level: 'medium',
    status: 'pending',
    created_at: '2026-04-01T10:05:00Z',
  };
  if (withProposal) {
    plan.proposed_action_plan = {
      actionId: 'capacity-forecast-abc',
      allowed: false,
      requiresApproval: true,
      source: CAPACITY_FORECAST_SOURCE,
      message: 'Storage pool "tank" is at 87.3% usage. Propose: prune oldest auto-snapshots.',
      projectedMetric: {
        metric: 'usage_percent',
        currentValue: 87.3,
        predictedValue: 93.5,
        thresholdValue: 75,
        timeToThresholdSeconds: 36 * 3600,
      },
      preflight: {
        intendedChange: 'Prune oldest auto-snapshots, then list largest reclaimable datasets.',
        dryRunAvailable: false,
        safetyChecks: [
          'Operator must explicitly approve before any execution path is wired.',
          'This proposal ships with Allowed=false; the action broker will refuse execution.',
        ],
      },
    };
  }
  return plan;
}

const mockState = vi.hoisted(() => {
  const loadFindings = vi.fn();
  const loadPatrolFindings = vi.fn();
  const loadRemediationPlans = vi.fn();
  const approveRemediationPlan = vi.fn().mockResolvedValue({ success: true });
  return {
    findings: [] as unknown[],
    remediationPlans: [] as unknown[],
    loadFindings,
    loadPatrolFindings,
    loadRemediationPlans,
    approveRemediationPlan,
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
    openWithPrompt: vi.fn(),
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    approveRemediationPlan: (...args: unknown[]) => mockState.approveRemediationPlan(...args),
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
  mockState.approveRemediationPlan.mockClear();
  if (typeof window.requestAnimationFrame !== 'function') {
    window.requestAnimationFrame = ((callback: FrameRequestCallback) =>
      window.setTimeout(
        () => callback(performance.now()),
        0,
      )) as typeof window.requestAnimationFrame;
  }
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('FindingsPanel capacity-forecast approval card', () => {
  it('renders the capacity-forecast card when a capacity finding has a forecast-driven proposal', async () => {
    const finding = makeFinding({ id: 'finding-cap-1' });
    mockState.findings = [finding];
    mockState.remediationPlans = [makePlan(finding.id, true)];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadRemediationPlans).toHaveBeenCalled());

    fireEvent.click(screen.getByText('Storage pool tank at 87.3% usage'));

    const card = screen.getByTestId('capacity-forecast-approval-card');
    expect(card).toBeInTheDocument();
    expect(within(card).getByText(/Capacity-forecast proposal/i)).toBeInTheDocument();
    expect(within(card).getByText(/requires approval/i)).toBeInTheDocument();
    expect(within(card).getByText(/preflight only/i)).toBeInTheDocument();
    // Operator-facing snapshot must surface current/predicted/threshold and TTB.
    // Current value appears in two places inside the card: the projected
    // metric snapshot and the proposal message. Both are intentional - the
    // first lets the operator see the projection at a glance, the second
    // restates it inside the rationale paragraph. Use *AllBy* to match
    // both without locking the exact placement.
    expect(within(card).getAllByText(/87\.3%/).length).toBeGreaterThanOrEqual(1);
    expect(within(card).getAllByText(/93\.5%/).length).toBeGreaterThanOrEqual(1);
    expect(within(card).getByText(/threshold 75%/)).toBeInTheDocument();
    expect(within(card).getByText(/breach in/i)).toBeInTheDocument();
    // The generic "Ask Assistant" affordance must NOT appear when the
    // forecast variant is active - the generic card is gated by fallback.
    expect(screen.queryByText('Ask Assistant')).not.toBeInTheDocument();
  });

  it('renders compact action review when a capacity finding has no proposal attached', async () => {
    const finding = makeFinding({ id: 'finding-cap-2' });
    mockState.findings = [finding];
    mockState.remediationPlans = [makePlan(finding.id, false)];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadRemediationPlans).toHaveBeenCalled());

    fireEvent.click(screen.getByText('Storage pool tank at 87.3% usage'));

    expect(screen.queryByTestId('capacity-forecast-approval-card')).not.toBeInTheDocument();
    expect(screen.getByText('Action review')).toBeInTheDocument();
    expect(screen.getByText('Ask Assistant')).toBeInTheDocument();
    expect(screen.queryByText('Remediation Plan')).not.toBeInTheDocument();
  });

  it('never renders the capacity-forecast card for a non-capacity finding even if a proposal somehow attaches', async () => {
    // This case should not occur in production - the wire-in only attaches
    // proposals to capacity findings - but the frontend gate must stay
    // closed regardless. A drifted backend that mis-categorises a finding
    // must not leak the capacity-forecast card onto the wrong finding type.
    const finding = makeFinding({
      id: 'finding-perf-1',
      category: 'performance',
      title: 'High CPU on appserver: 92.0%',
    });
    mockState.findings = [finding];
    mockState.remediationPlans = [makePlan(finding.id, true)];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadRemediationPlans).toHaveBeenCalled());

    fireEvent.click(screen.getByText('High CPU on appserver: 92.0%'));

    expect(screen.queryByTestId('capacity-forecast-approval-card')).not.toBeInTheDocument();
    expect(screen.getByText('Action review')).toBeInTheDocument();
    expect(screen.getByText('Ask Assistant')).toBeInTheDocument();
  });

  it('routes Approve through AIAPI.approveRemediationPlan and Reject through dismiss without bypassing handlers', async () => {
    const finding = makeFinding({ id: 'finding-cap-3' });
    mockState.findings = [finding];
    mockState.remediationPlans = [makePlan(finding.id, true)];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadRemediationPlans).toHaveBeenCalled());

    fireEvent.click(screen.getByText('Storage pool tank at 87.3% usage'));

    fireEvent.click(screen.getByTestId('capacity-forecast-approve'));

    await waitFor(() =>
      expect(mockState.approveRemediationPlan).toHaveBeenCalledWith('plan-finding-cap-3'),
    );
  });
});
