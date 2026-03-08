import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Investigation } from '@/api/patrol';
import { getInvestigationStatusBadgeClasses } from '@/utils/aiFindingPresentation';
import InvestigationSection from '../InvestigationSection';

const getInvestigationMock = vi.hoisted(() => vi.fn<() => Promise<Investigation>>());
const reinvestigateFindingMock = vi.hoisted(() => vi.fn());
const loadFindingsMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/patrol', () => ({
  getInvestigation: (...args: unknown[]) => getInvestigationMock(...args),
  reinvestigateFinding: (...args: unknown[]) => reinvestigateFindingMock(...args),
  investigationStatusLabels: {
    pending: 'Pending',
    running: 'Running',
    completed: 'Completed',
    failed: 'Failed',
    needs_attention: 'Needs Attention',
  },
  investigationOutcomeLabels: {
    resolved: 'Resolved',
    needs_attention: 'Needs Attention',
  },
  investigationOutcomeColors: {
    resolved:
      'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
    needs_attention:
      'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  },
  formatTimestamp: (value: string) => value,
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    loadFindings: loadFindingsMock,
  },
}));

vi.mock('../InvestigationMessages', () => ({
  InvestigationMessages: () => <div>Mock investigation messages</div>,
}));

describe('InvestigationSection', () => {
  beforeEach(() => {
    getInvestigationMock.mockReset();
    reinvestigateFindingMock.mockReset();
    loadFindingsMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('renders investigation status badges from the shared finding presentation contract', async () => {
    getInvestigationMock.mockResolvedValue({
      id: 'inv-1',
      finding_id: 'finding-1',
      session_id: 'session-1',
      status: 'running',
      started_at: '2026-03-08T10:00:00.000Z',
      turn_count: 2,
      summary: 'Investigation is active.',
      tools_used: ['ssh'],
    } satisfies Investigation);

    render(() => <InvestigationSection findingId="finding-1" investigationStatus="running" />);

    const statusBadge = await screen.findByText('Running');
    expect(statusBadge.className).toContain(getInvestigationStatusBadgeClasses('running'));
  });
});
