import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Investigation } from '@/api/patrol';
import {
  getInvestigationOutcomeBadgeClasses,
  getInvestigationOutcomeLabel,
  getInvestigationStatusBadgeClasses,
} from '@/utils/aiFindingPresentation';
import InvestigationSection from '../InvestigationSection';

const getInvestigationMock = vi.hoisted(() => vi.fn<() => Promise<Investigation>>());
const reinvestigateFindingMock = vi.hoisted(() => vi.fn());
const loadFindingsMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/patrol', () => ({
  getInvestigation: (...args: unknown[]) => getInvestigationMock(...args),
  reinvestigateFinding: (...args: unknown[]) => reinvestigateFindingMock(...args),
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

  it('renders investigation outcome badges from the shared finding presentation contract', async () => {
    getInvestigationMock.mockResolvedValue({
      id: 'inv-2',
      finding_id: 'finding-2',
      session_id: 'session-2',
      status: 'completed',
      outcome: 'needs_attention',
      started_at: '2026-03-08T10:00:00.000Z',
      turn_count: 2,
      summary: 'Manual review needed.',
      tools_used: ['ssh'],
    } satisfies Investigation);

    render(() => <InvestigationSection findingId="finding-2" investigationStatus="completed" />);

    const outcomeBadge = await screen.findByText(getInvestigationOutcomeLabel('needs_attention'));
    expect(outcomeBadge.className).toContain(
      getInvestigationOutcomeBadgeClasses('needs_attention'),
    );
  });
});
