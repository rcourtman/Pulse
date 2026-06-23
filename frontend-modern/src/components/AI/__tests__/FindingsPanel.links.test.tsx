import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { FindingsPanel } from '../FindingsPanel';

const mockState = vi.hoisted(() => {
  const loadFindings = vi.fn();
  const loadPatrolFindings = vi.fn();
  const getRemediationPlans = vi.fn().mockResolvedValue({ plans: [] });
  const getResource = vi.fn((resourceId: string) =>
    resourceId === 'app-container:truenas-main:nextcloud'
      ? ({
          id: 'app-container:truenas-main:nextcloud',
          type: 'app-container',
          name: 'nextcloud',
          displayName: 'Nextcloud',
          parentId: 'truenas-main',
          parentName: 'TrueNAS Main',
          platformId: 'truenas-main',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'running',
          lastSeen: Date.now(),
          platformData: {
            sources: ['truenas'],
          },
        } as const)
      : undefined,
  );

  const findings = [
    {
      id: 'finding-truenas-app',
      source: 'ai-patrol',
      resourceId: 'app-container:truenas-main:nextcloud',
      resourceName: 'Nextcloud',
      resourceType: 'app-container',
      category: 'reliability',
      severity: 'warning',
      title: 'Nextcloud failed readiness checks',
      description: 'Pulse Patrol detected repeated readiness probe failures.',
      impact: 'Nextcloud is unreachable for users until readiness recovers.',
      detectedAt: '2026-03-30T10:00:00Z',
      lastSeenAt: '2026-03-30T10:05:00Z',
      status: 'active',
    },
  ];
  const patrolFindings: Array<Record<string, unknown>> = [
    {
      id: 'finding-provider-issue',
      source: 'ai-patrol',
      resourceId: 'instance:node:100',
      resourceName: 'vm-100',
      resourceType: 'vm',
      alertIdentifier: 'instance:node:100::patrol/provider',
      category: 'reliability',
      severity: 'warning',
      title: 'Provider connection issue',
      description: 'Pulse Patrol could not complete provider analysis.',
      impact: 'Patrol cannot analyze infrastructure until provider setup is fixed.',
      detectedAt: '2026-03-30T11:00:00Z',
      lastSeenAt: '2026-03-30T11:05:00Z',
      status: 'active',
    },
  ];

  return {
    initialFindings: findings,
    initialPatrolFindings: patrolFindings,
    findings,
    findingsError: null as string | null,
    findingsLoading: false,
    patrolFindings,
    patrolFindingsError: null as string | null,
    patrolFindingsLoading: false,
    patrolPendingApprovals: [] as Array<{
      status: string;
      toolId: string;
      targetId: string;
      expiresAt?: string;
    }>,
    getResource,
    getRemediationPlans,
    loadFindings,
    loadPatrolFindings,
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
    getRemediationPlans: mockState.getRemediationPlans,
  },
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    get: mockState.getResource,
  }),
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get findings() {
      return mockState.findings;
    },
    get findingsLoading() {
      return mockState.findingsLoading;
    },
    get findingsError() {
      return mockState.findingsError;
    },
    get patrolFindings() {
      return mockState.patrolFindings;
    },
    get patrolFindingsLoading() {
      return mockState.patrolFindingsLoading;
    },
    get patrolFindingsError() {
      return mockState.patrolFindingsError;
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
    get needsAttentionCount() {
      return 0;
    },
    get patrolNeedsAttentionCount() {
      return 0;
    },
    get pendingApprovalCount() {
      return 0;
    },
    get patrolPendingApprovalCount() {
      return 0;
    },
    get patrolPendingApprovals() {
      return mockState.patrolPendingApprovals;
    },
    get remediationPlans() {
      return [];
    },
    findingsSignal: () => mockState.findings,
    patrolFindingsSignal: () => mockState.patrolFindings,
    loadFindings: mockState.loadFindings,
    loadPatrolFindings: mockState.loadPatrolFindings,
    loadRemediationPlans: vi.fn(),
  },
}));

describe('FindingsPanel resource links', () => {
  beforeEach(() => {
    mockState.findings = [...mockState.initialFindings];
    mockState.patrolFindings = [...mockState.initialPatrolFindings];
    mockState.loadFindings.mockClear();
    mockState.loadPatrolFindings.mockClear();
    mockState.getRemediationPlans.mockClear();
    mockState.getResource.mockClear();
    mockState.findingsLoading = false;
    mockState.findingsError = null;
    mockState.patrolFindingsLoading = false;
    mockState.patrolFindingsError = null;
    mockState.patrolPendingApprovals = [];

    if (typeof window.requestAnimationFrame !== 'function') {
      window.requestAnimationFrame = ((callback: FrameRequestCallback) =>
        window.setTimeout(
          () => callback(performance.now()),
          0,
        )) as typeof window.requestAnimationFrame;
    }
  });

  it('renders the finding body without legacy surface cross-jump links', async () => {
    render(() => <FindingsPanel />);

    await waitFor(() => expect(mockState.loadFindings).toHaveBeenCalled());

    fireEvent.click(
      screen.getByRole('button', {
        name: 'View details for Nextcloud failed readiness checks',
      }),
    );

    expect(
      screen.getByText('Nextcloud is unreachable for users until readiness recovers.'),
    ).toBeInTheDocument();
    // Broad cross-jump chips into /infrastructure and aggregate workspaces
    // were retired with the platform-first migration;
    // Patrol findings now stay in place inside the panel.
    expect(
      screen.queryByRole('link', { name: 'Open related infrastructure for Nextcloud' }),
    ).toBeNull();
    expect(screen.queryByRole('link', { name: 'Open related workloads for Nextcloud' })).toBeNull();
  });

  it('keeps Patrol runtime setup findings primary-action-only when expanded', async () => {
    mockState.patrolFindings = [
      {
        id: 'finding-provider-runtime',
        source: 'ai-patrol',
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        resourceType: 'service',
        alertIdentifier: 'ai-service::patrol/runtime',
        category: 'runtime',
        severity: 'warning',
        title: 'Pulse Patrol: Provider billing or quota issue',
        description: 'Pulse Patrol could not maintain a healthy provider connection.',
        impact: 'Patrol cannot analyze infrastructure until provider setup is fixed.',
        detectedAt: '2026-03-30T11:00:00Z',
        lastSeenAt: '2026-03-30T11:05:00Z',
        status: 'active',
      },
    ];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.getByRole('link', { name: 'Open Provider & Models' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/provider',
    );

    fireEvent.click(
      screen.getByRole('button', {
        name: 'View details for Provider billing or quota issue',
      }),
    );

    expect(screen.getByRole('link', { name: 'Open Provider & Models' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/provider',
    );
    expect(screen.queryByRole('button', { name: 'Open in Assistant' })).not.toBeInTheDocument();
    expect(screen.queryByText('Manage')).not.toBeInTheDocument();
  });

  it('keeps finding disclosure separate from inline manual controls', async () => {
    render(() => <FindingsPanel />);

    await waitFor(() => expect(mockState.loadFindings).toHaveBeenCalled());

    const disclosure = screen.getByRole('button', {
      name: 'View details for Nextcloud failed readiness checks',
    });
    const acknowledge = screen.getByRole('button', { name: 'Acknowledge' });

    expect(disclosure).not.toContainElement(acknowledge);

    fireEvent.click(disclosure);

    expect(
      screen.getByText('Nextcloud is unreachable for users until readiness recovers.'),
    ).toBeInTheDocument();
  });

  it('keeps Patrol queue rows free of inline triage controls until expanded', async () => {
    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    const disclosure = screen.getByRole('button', {
      name: 'View details for Provider connection issue',
    });

    expect(
      screen.getAllByRole('button', {
        name: 'View details for Provider connection issue',
      }),
    ).toHaveLength(1);
    expect(disclosure).toHaveTextContent('Details');
    expect(screen.queryByRole('button', { name: 'Acknowledge' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Snooze 24h' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Dismiss' })).not.toBeInTheDocument();

    fireEvent.click(disclosure);

    expect(
      screen.getByRole('button', {
        name: 'Hide details for Provider connection issue',
      }),
    ).toHaveTextContent('Hide');
    expect(screen.getByText('Manage')).toBeInTheDocument();
    const assistantHandoff = screen.getByRole('button', { name: 'Open in Assistant' });
    expect(assistantHandoff.closest('details')).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Acknowledge' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Snooze 24h' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Dismiss: Later' })).toBeInTheDocument();
  });

  it('keeps default Patrol workflow states out of collapsed issue badges', async () => {
    mockState.patrolFindings = [
      {
        id: 'finding-default-workflow',
        source: 'ai-patrol',
        resourceId: 'vm-101',
        resourceName: 'database-01',
        resourceType: 'vm',
        category: 'reliability',
        severity: 'warning',
        title: 'Database latency spike',
        description: 'Patrol saw the database latency threshold trip.',
        impact: 'Database-backed apps may respond slowly.',
        detectedAt: '2026-03-30T11:00:00Z',
        lastSeenAt: '2026-03-30T11:05:00Z',
        status: 'active',
        loopState: 'detected',
      },
    ];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.getByText('warning')).toBeInTheDocument();
    expect(screen.getByText('Database latency spike')).toBeInTheDocument();
    expect(screen.queryByText('detected')).not.toBeInTheDocument();
    expect(screen.queryByText('Review finding')).not.toBeInTheDocument();
  });

  it('keeps Patrol process states out of collapsed issue badges', async () => {
    mockState.patrolFindings = [
      {
        id: 'finding-investigating',
        source: 'ai-patrol',
        resourceId: 'vm-101',
        resourceName: 'database-01',
        resourceType: 'vm',
        category: 'reliability',
        severity: 'warning',
        title: 'Database latency spike',
        description: 'Patrol saw the database latency threshold trip.',
        impact: 'Database-backed apps may respond slowly.',
        detectedAt: '2026-03-30T11:00:00Z',
        lastSeenAt: '2026-03-30T11:05:00Z',
        status: 'active',
        loopState: 'investigating',
        investigationStatus: 'running',
      },
      {
        id: 'finding-verifying',
        source: 'ai-patrol',
        resourceId: 'vm-102',
        resourceName: 'web-01',
        resourceType: 'vm',
        category: 'availability',
        severity: 'warning',
        title: 'Web service restart loop',
        description: 'Patrol saw repeated restarts.',
        detectedAt: '2026-03-30T11:01:00Z',
        lastSeenAt: '2026-03-30T11:06:00Z',
        status: 'active',
        investigationOutcome: 'fix_executed',
      },
    ];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.getByText('Database latency spike')).toBeInTheDocument();
    expect(screen.getByText('Web service restart loop')).toBeInTheDocument();
    expect(screen.queryByText('Patrol investigating')).not.toBeInTheDocument();
    expect(screen.queryByText('Verify outcome')).not.toBeInTheDocument();
    expect(screen.queryByText('Review finding')).not.toBeInTheDocument();
  });

  it('shows pending Patrol approval as the user decision, not as a process badge', async () => {
    mockState.patrolFindings = [
      {
        id: 'finding-approval',
        source: 'ai-patrol',
        resourceId: 'vm-103',
        resourceName: 'worker-01',
        resourceType: 'vm',
        category: 'availability',
        severity: 'critical',
        title: 'Worker disk is full',
        description: 'Patrol found the worker disk is full.',
        detectedAt: '2026-03-30T11:02:00Z',
        lastSeenAt: '2026-03-30T11:07:00Z',
        status: 'active',
        investigationOutcome: 'fix_queued',
      },
    ];
    mockState.patrolPendingApprovals = [
      {
        status: 'pending',
        toolId: 'investigation_fix',
        targetId: 'finding-approval',
        expiresAt: '2999-01-01T00:00:00Z',
      },
    ];

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    const approvalAction = screen.getByRole('button', { name: 'Approve or reject' });
    expect(approvalAction).toBeInTheDocument();
    expect(screen.queryByText('Review finding')).not.toBeInTheDocument();

    fireEvent.click(approvalAction);

    expect(
      screen.getByRole('button', {
        name: 'Hide details for Worker disk is full',
      }),
    ).toHaveTextContent('Hide');
  });

  it('keeps selected Patrol run issue rows simple and expandable', async () => {
    mockState.patrolFindings = [
      {
        id: 'finding-array-parity',
        source: 'ai-patrol',
        resourceId: 'storage:tower-array',
        resourceName: 'Tower Array',
        resourceType: 'storage',
        category: 'availability',
        severity: 'critical',
        title: 'Unraid array running without parity protection while at 86% capacity',
        description:
          'The Unraid array on Tower is running without parity protection while capacity is high.',
        impact: 'A disk failure before parity is rebuilt could cause unrecoverable data loss.',
        detectedAt: '2026-03-30T11:00:00Z',
        lastSeenAt: '2026-03-30T11:05:00Z',
        status: 'active',
        loopState: 'detected',
      },
      {
        id: 'finding-array-usage',
        source: 'ai-patrol',
        resourceId: 'storage:tower-array',
        resourceName: 'Tower Array',
        resourceType: 'storage',
        category: 'capacity',
        severity: 'warning',
        title: 'Storage pool Tower Array at 85.9% usage',
        description: 'Tower Array is nearing the configured storage capacity threshold.',
        detectedAt: '2026-03-30T11:01:00Z',
        lastSeenAt: '2026-03-30T11:05:00Z',
        status: 'active',
        loopState: 'detected',
      },
    ];

    render(() => (
      <FindingsPanel
        findingsSource="patrol"
        filterOverride="all"
        filterFindingIds={['finding-array-parity', 'finding-array-usage']}
        runSnapshot={{
          resources_checked: 72,
          scope_resource_ids: [],
          effective_scope_resource_ids: [],
          finding_ids: ['finding-array-parity', 'finding-array-usage'],
          status: 'critical',
          error_count: 0,
        }}
      />
    ));

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.getByText('warning')).toBeInTheDocument();
    expect(
      screen.getByText('Unraid array running without parity protection while at 86% capacity'),
    ).toBeInTheDocument();
    expect(screen.getByText('Storage pool Tower Array at 85.9% usage')).toBeInTheDocument();
    expect(screen.queryByText('detected')).not.toBeInTheDocument();
    expect(screen.queryByText('Review finding')).not.toBeInTheDocument();

    const disclosure = screen.getByRole('button', {
      name: 'View details for Unraid array running without parity protection while at 86% capacity',
    });
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(disclosure);

    expect(
      screen.getByRole('button', {
        name: 'Hide details for Unraid array running without parity protection while at 86% capacity',
      }),
    ).toHaveAttribute('aria-expanded', 'true');
    expect(
      screen.getByText(
        'The Unraid array on Tower is running without parity protection while capacity is high.',
      ),
    ).toBeInTheDocument();
  });

  it('can render Patrol findings from the direct Patrol source', async () => {
    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(mockState.loadFindings).not.toHaveBeenCalled();
    expect(screen.getByText('Provider connection issue')).toBeInTheDocument();
  });

  it('renders Patrol findings while the unified findings request is still loading', async () => {
    mockState.findingsLoading = true;
    mockState.patrolFindingsLoading = false;

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.queryByText('Loading findings...')).not.toBeInTheDocument();
    expect(screen.getByText('Provider connection issue')).toBeInTheDocument();
  });

  it('keeps loaded Patrol findings visible during a Patrol refresh', async () => {
    mockState.patrolFindingsLoading = true;

    render(() => <FindingsPanel findingsSource="patrol" />);

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.queryByText('Loading findings...')).not.toBeInTheDocument();
    expect(screen.getByText('Provider connection issue')).toBeInTheDocument();
  });

  it('renders a selected Patrol run empty snapshot while Patrol findings are loading', async () => {
    mockState.patrolFindings = [];
    mockState.patrolFindingsLoading = true;

    render(() => (
      <FindingsPanel
        findingsSource="patrol"
        filterOverride="all"
        filterFindingIds={[]}
        runSnapshot={{
          resources_checked: 67,
          scope_resource_ids: ['node-a'],
          effective_scope_resource_ids: ['node-a'],
          finding_ids: [],
          status: 'error',
          error_count: 1,
        }}
      />
    ));

    await waitFor(() => expect(mockState.loadPatrolFindings).toHaveBeenCalled());

    expect(screen.queryByText('Loading findings...')).not.toBeInTheDocument();
    expect(screen.getByText('No findings recorded for this run')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Checked 67 scoped resources. This run recorded no Patrol findings, but it ended with issues requiring review.',
      ),
    ).toBeInTheDocument();
  });
});
