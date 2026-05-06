import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { InvestigationRecord } from '@/api/ai';
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
  getInvestigation: getInvestigationMock,
  reinvestigateFinding: reinvestigateFindingMock,
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

  it('renders the durable Patrol investigation record through the inline investigation surface', async () => {
    getInvestigationMock.mockResolvedValue(undefined as unknown as Investigation);

    const record: InvestigationRecord = {
      id: 'record-1',
      finding_id: 'finding-1',
      subject: { resource_id: 'vm-100', resource_name: 'web', resource_type: 'vm' },
      trigger: {
        title: 'High CPU usage',
        detected_at: '2026-05-06T12:00:00Z',
      },
      status: 'completed',
      outcome: 'fix_queued',
      confidence: 'high',
      evidence: [{ kind: 'metrics', summary: 'CPU stayed above 95% for 10 minutes' }],
      conclusion: 'Backup job saturated CPU.',
      recommended_action: 'Approve a controlled restart after the backup completes.',
      proposed_fix: {
        id: 'fix-1',
        description: 'Restart the workload service',
        commands: ['systemctl restart workload.service'],
        risk_level: 'medium',
        destructive: false,
        target_host: 'pve-1',
      },
      verification: ['CPU returned below 50%'],
      tools_used: ['metrics.history'],
      started_at: '2026-05-06T12:00:00Z',
    };

    render(() => <InvestigationSection findingId="finding-1" investigationRecord={record} />);

    expect(await screen.findByText('Patrol record')).toBeInTheDocument();
    expect(screen.getByText('Backup job saturated CPU.')).toBeInTheDocument();
    expect(screen.getByText('CPU stayed above 95% for 10 minutes')).toBeInTheDocument();
    expect(screen.getByText('1 command recorded for approval context')).toBeInTheDocument();
    expect(screen.queryByText(/No investigation data available/)).not.toBeInTheDocument();
    expect(screen.queryByText('systemctl restart workload.service')).not.toBeInTheDocument();
  });
});
