import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { FindingsPanel } from '../FindingsPanel';

const mockState = vi.hoisted(() => {
  const loadFindings = vi.fn();
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
      detectedAt: '2026-03-30T10:00:00Z',
      lastSeenAt: '2026-03-30T10:05:00Z',
      status: 'active',
    },
  ];

  return {
    findings,
    getResource,
    getRemediationPlans,
    loadFindings,
  };
});

vi.mock('@solidjs/router', () => ({
  A: (props: { href: string; children: unknown; [key: string]: unknown }) => (
    <a href={props.href} aria-label={props['aria-label'] as string} onClick={props.onClick as any}>
      {props.children}
    </a>
  ),
  useLocation: () => ({ hash: '' }),
}));

vi.mock('@/components/shared/Card', () => ({
  Card: (props: { children: unknown }) => <div>{props.children}</div>,
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
      return false;
    },
    get findingsError() {
      return null;
    },
    get findingsNeedingAttention() {
      return [];
    },
    get findingsWithPendingApprovals() {
      return [];
    },
    get needsAttentionCount() {
      return 0;
    },
    get pendingApprovalCount() {
      return 0;
    },
    findingsSignal: () => mockState.findings,
    loadFindings: mockState.loadFindings,
  },
}));

describe('FindingsPanel resource links', () => {
  beforeEach(() => {
    mockState.loadFindings.mockClear();
    mockState.getRemediationPlans.mockClear();
    mockState.getResource.mockClear();

    if (typeof window.requestAnimationFrame !== 'function') {
      window.requestAnimationFrame = ((callback: FrameRequestCallback) =>
        window.setTimeout(() => callback(performance.now()), 0)) as typeof window.requestAnimationFrame;
    }
  });

  it('surfaces canonical handoff links for TrueNAS workload findings', async () => {
    render(() => <FindingsPanel />);

    await waitFor(() => expect(mockState.loadFindings).toHaveBeenCalled());

    fireEvent.click(screen.getByText('Nextcloud failed readiness checks'));

    expect(
      screen.getByRole('link', { name: 'Open related infrastructure for Nextcloud' }),
    ).toHaveAttribute(
      'href',
      '/infrastructure?resource=app-container%3Atruenas-main%3Anextcloud',
    );
    expect(
      screen.getByRole('link', { name: 'Open related workloads for Nextcloud' }),
    ).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
    );
  });
});
