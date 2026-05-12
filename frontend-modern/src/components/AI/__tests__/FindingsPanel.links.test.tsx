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
  const patrolFindings = [
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
      detectedAt: '2026-03-30T11:00:00Z',
      lastSeenAt: '2026-03-30T11:05:00Z',
      status: 'active',
    },
  ];

  return {
    findings,
    findingsError: null as string | null,
    findingsLoading: false,
    patrolFindings,
    patrolFindingsError: null as string | null,
    patrolFindingsLoading: false,
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
    openWithPrompt: vi.fn(),
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
    mockState.loadFindings.mockClear();
    mockState.loadPatrolFindings.mockClear();
    mockState.getRemediationPlans.mockClear();
    mockState.getResource.mockClear();
    mockState.findingsLoading = false;
    mockState.findingsError = null;
    mockState.patrolFindingsLoading = false;
    mockState.patrolFindingsError = null;

    if (typeof window.requestAnimationFrame !== 'function') {
      window.requestAnimationFrame = ((callback: FrameRequestCallback) =>
        window.setTimeout(
          () => callback(performance.now()),
          0,
        )) as typeof window.requestAnimationFrame;
    }
  });

  it('surfaces canonical handoff links for TrueNAS workload findings', async () => {
    render(() => <FindingsPanel />);

    await waitFor(() => expect(mockState.loadFindings).toHaveBeenCalled());

    fireEvent.click(screen.getByText('Nextcloud failed readiness checks'));

    expect(
      screen.getByText('Nextcloud is unreachable for users until readiness recovers.'),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Open related infrastructure for Nextcloud' }),
    ).toHaveAttribute('href', '/infrastructure?resource=app-container%3Atruenas-main%3Anextcloud');
    expect(
      screen.getByRole('link', { name: 'Open related workloads for Nextcloud' }),
    ).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
    );
  });

  it('keeps finding disclosure separate from inline manual controls', async () => {
    render(() => <FindingsPanel />);

    await waitFor(() => expect(mockState.loadFindings).toHaveBeenCalled());

    const disclosure = screen.getByRole('button', {
      name: /Nextcloud failed readiness checks/,
    });
    const acknowledge = screen.getByRole('button', { name: 'Acknowledge' });

    expect(disclosure).not.toContainElement(acknowledge);

    fireEvent.click(disclosure);

    expect(
      screen.getByText('Nextcloud is unreachable for users until readiness recovers.'),
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
});
