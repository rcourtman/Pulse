import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { InvestigationRecord } from '@/api/ai';
import type { Investigation } from '@/api/patrol';
import {
  getInvestigationOutcomeBadgeTone,
  getInvestigationOutcomeLabel,
  getInvestigationStatusBadgeTone,
} from '@/utils/aiFindingPresentation';
import investigationSectionSource from '../InvestigationSection.tsx?raw';
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

  it('keeps investigation loading indicators on the shared LoadingSpinner primitive', () => {
    expect(investigationSectionSource).toContain('LoadingSpinner');
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
    expect(getInvestigationStatusBadgeTone('running')).toBe('info');
    expect(statusBadge.className).toContain('inline-flex');
    expect(statusBadge.className).toContain('border-blue-200');
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
    expect(getInvestigationOutcomeBadgeTone('needs_attention')).toBe('warning');
    expect(outcomeBadge.className).toContain('inline-flex');
    expect(outcomeBadge.className).toContain('border-amber-200');
  });

  it('offers another run after a run that ended without a clear conclusion', async () => {
    getInvestigationMock.mockResolvedValue({
      id: 'inv-unclassified',
      finding_id: 'finding-unclassified',
      session_id: 'session-unclassified',
      status: 'completed',
      outcome: 'unclassified',
      started_at: '2026-10-22T10:00:00.000Z',
      turn_count: 6,
      summary: 'The service restarted twice; the logs do not say why.',
    } satisfies Investigation);

    render(() => (
      <InvestigationSection findingId="finding-unclassified" investigationStatus="completed" />
    ));

    const outcomeBadge = await screen.findByText('No clear conclusion');
    expect(outcomeBadge.className).toContain('border-amber-200');
    expect(screen.getByText('The service restarted twice; the logs do not say why.')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Re-investigate' })).toBeInTheDocument();
  });

  it('presents a run cut short by a restart as information, not as an error', async () => {
    const note = 'Interrupted: Pulse restarted before this investigation finished.';
    getInvestigationMock.mockResolvedValue({
      id: 'inv-interrupted',
      finding_id: 'finding-interrupted',
      session_id: 'session-interrupted',
      status: 'failed',
      outcome: 'interrupted',
      started_at: '2026-10-22T10:00:00.000Z',
      turn_count: 2,
      error: note,
    } satisfies Investigation);

    render(() => (
      <InvestigationSection
        findingId="finding-interrupted"
        investigationStatus="failed"
        investigationRecord={{
          id: 'record-interrupted',
          finding_id: 'finding-interrupted',
          subject: { resource_id: 'vm-102' },
          trigger: { title: 'Service keeps restarting', detected_at: '2026-10-22T10:00:00Z' },
          status: 'failed',
          outcome: 'interrupted',
          evidence: [],
          verification: [],
          rollback: [],
          tools_used: [],
          started_at: '2026-10-22T10:00:00Z',
          error: note,
        }}
      />
    ));

    // Wait for the session, whose badge heads the section above the record's.
    await screen.findByRole('button', { name: 'Show investigation thread' });
    const badges = screen.getAllByText('Interrupted — will retry');
    expect(badges).toHaveLength(2);
    for (const badge of badges) expect(badge.className).toContain('border-border');
    // The record's failed run status would contradict "will retry".
    expect(screen.queryByText('Failed')).not.toBeInTheDocument();
    // The record and the session carry the same note, so it shows once.
    const noteBox = screen.getByText(note);
    expect(noteBox.className).toContain('text-muted');
    expect(noteBox.className).not.toContain('bg-red-50');
    expect(screen.getByRole('button', { name: 'Re-investigate' })).toBeInTheDocument();
  });

  it('keeps a failed run error in the error style', async () => {
    getInvestigationMock.mockResolvedValue({
      id: 'inv-failed',
      finding_id: 'finding-failed',
      session_id: 'session-failed',
      status: 'failed',
      outcome: 'timed_out',
      started_at: '2026-10-22T10:00:00.000Z',
      turn_count: 12,
      error: 'The provider stopped responding.',
    } satisfies Investigation);

    render(() => <InvestigationSection findingId="finding-failed" investigationStatus="failed" />);

    expect((await screen.findByText('The provider stopped responding.')).className).toContain(
      'bg-red-50',
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
      rollback: [],
      tools_used: ['metrics.history'],
      started_at: '2026-05-06T12:00:00Z',
    };

    render(() => <InvestigationSection findingId="finding-1" investigationRecord={record} />);

    expect(await screen.findByText('Patrol record')).toBeInTheDocument();
    expect(screen.getByText('Fix queued')).toBeInTheDocument();
    expect(screen.queryByText('Completed')).not.toBeInTheDocument();
    expect(screen.getByText('Backup job saturated CPU.')).toBeInTheDocument();
    expect(screen.getByText('CPU stayed above 95% for 10 minutes')).toBeInTheDocument();
    expect(screen.getByText('1 command recorded for approval context')).toBeInTheDocument();
    expect(screen.queryByText(/No investigation data available/)).not.toBeInTheDocument();
    expect(screen.queryByText('systemctl restart workload.service')).not.toBeInTheDocument();
  });

  it('shows the record run status until the run has an outcome', async () => {
    getInvestigationMock.mockResolvedValue(undefined as unknown as Investigation);

    render(() => (
      <InvestigationSection
        findingId="finding-running"
        investigationRecord={{
          id: 'record-running',
          finding_id: 'finding-running',
          subject: { resource_id: 'vm-103' },
          trigger: { title: 'Service keeps restarting', detected_at: '2026-10-22T10:00:00Z' },
          status: 'running',
          started_at: '2026-10-22T10:00:00Z',
          evidence: [],
          verification: [],
          rollback: [],
          tools_used: [],
        }}
      />
    ));

    expect(await screen.findByText('Patrol record')).toBeInTheDocument();
    expect(screen.getByText('Running')).toBeInTheDocument();
  });

  it.each([false, true])(
    'preserves readable evidence and distinct summaries (different: %s)',
    async (different) => {
      const conclusion =
        '### Root cause\n\nThe cause is **unknown**.\n\n- The health check failed.\n- Logs are unavailable.';
      getInvestigationMock.mockResolvedValue({
        id: 'inv-markdown',
        finding_id: 'finding-markdown',
        session_id: 'session-markdown',
        status: 'completed',
        started_at: '2026-09-06T17:00:00Z',
        turn_count: 2,
        summary: different
          ? '### Follow-up\n\nAdditional evidence remains unavailable.'
          : conclusion,
      } satisfies Investigation);
      render(() => (
        <InvestigationSection
          findingId="finding-markdown"
          investigationRecord={{
            id: 'record-markdown',
            finding_id: 'finding-markdown',
            subject: { resource_id: 'container-1' },
            trigger: { title: 'Health check failed', detected_at: '2026-09-06T17:00:00Z' },
            status: 'completed',
            conclusion,
            started_at: '2026-09-06T17:00:00Z',
            evidence: [],
            verification: [],
            rollback: [],
            tools_used: [],
          }}
        />
      ));
      await screen.findByRole('button', { name: 'Show investigation thread' });
      expect(screen.getAllByRole('heading', { name: 'Root cause' })).toHaveLength(1);
      expect(screen.getByText('unknown').tagName).toBe('STRONG');
      expect(screen.getAllByRole('listitem')).toHaveLength(2);
      expect(screen.queryByRole('heading', { name: 'Follow-up' }) !== null).toBe(different);
    },
  );
});
