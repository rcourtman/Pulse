import { createSignal } from 'solid-js';
import { cleanup, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PatrolRunRecord, PatrolTriggerStatus } from '@/api/patrol';
import { PatrolStatusBar } from '../PatrolStatusBar';

const getPatrolRunHistoryMock = vi.hoisted(() => vi.fn<() => Promise<PatrolRunRecord[]>>());

vi.mock('@/api/patrol', () => ({
  getPatrolRunHistory: getPatrolRunHistoryMock,
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    circuitBreakerStatus: null,
  },
}));

describe('PatrolStatusBar', () => {
  beforeEach(() => {
    getPatrolRunHistoryMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('shows recent activity instead of a healthy verdict for healthy patrol runs', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-healthy',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'All clear',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => <PatrolStatusBar refreshTrigger={1} />);

    await waitFor(() => {
      expect(screen.getByText('Recent activity')).toBeInTheDocument();
    });

    expect(screen.getByText('Latest: Full patrol')).toBeInTheDocument();
    expect(screen.getByText('healthy')).toBeInTheDocument();
  });

  it('flags latest runs whose findings snapshot is unavailable', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-legacy-healthy',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'All clear',
        finding_ids: undefined,
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => <PatrolStatusBar refreshTrigger={1} />);

    await waitFor(() => {
      expect(screen.getByText('Recent activity')).toBeInTheDocument();
    });

    const latestRunSection = screen.getByText('Latest: Full patrol').closest('span');
    expect(latestRunSection).not.toBeNull();
    expect(latestRunSection).toHaveTextContent(
      'Latest: Full patrol · completed · Findings snapshot unavailable',
    );
  });

  it('shows patrol paused when the runtime is blocked even if the last run was healthy', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-healthy',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'All clear',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => (
      <PatrolStatusBar
        refreshTrigger={1}
        runtimeState="blocked"
        blockedReason="Quickstart credits exhausted. Connect your API key to continue using Patrol."
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Patrol Paused')).toBeInTheDocument();
    });

    expect(screen.queryByText('Recent activity')).not.toBeInTheDocument();
  });

  it('shows the latest full-patrol result when the last patrol run found issues', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-critical',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 1,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'Critical issue detected',
        finding_ids: ['finding-1'],
        error_count: 0,
        status: 'critical',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => <PatrolStatusBar refreshTrigger={1} />);

    await waitFor(() => {
      expect(screen.getByText('Recent activity')).toBeInTheDocument();
    });

    expect(screen.getByText('Latest: Full patrol')).toBeInTheDocument();
    expect(screen.getByText('critical')).toBeInTheDocument();
  });

  it('shows scoped/erroring activity factually instead of a healthy summary', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-error-count',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'alert_fired',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'Run completed with errors',
        finding_ids: [],
        error_count: 2,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => <PatrolStatusBar refreshTrigger={1} />);

    await waitFor(() => {
      expect(screen.getByText('Recent activity')).toBeInTheDocument();
    });

    const latestRunSection = screen.getByText('Latest: Scoped run').closest('span');
    expect(latestRunSection).not.toBeNull();
    expect(latestRunSection).toHaveTextContent('Latest: Scoped run · error');
    expect(within(latestRunSection as HTMLElement).queryByText('healthy')).not.toBeInTheDocument();
    expect(within(latestRunSection as HTMLElement).getByText('error')).toBeInTheDocument();
  });

  it('keeps running patrols factual with a run-in-progress indicator', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-startup',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        trigger_reason: 'startup',
        resources_checked: 57,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'Completed with errors',
        finding_ids: [],
        error_count: 1,
        status: 'error',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => <PatrolStatusBar refreshTrigger={1} runtimeState="running" />);

    await waitFor(() => {
      expect(screen.getByText('Recent activity')).toBeInTheDocument();
    });

    expect(screen.getByText('Run in progress')).toBeInTheDocument();
    expect(screen.queryByText('Patrol Running')).not.toBeInTheDocument();
  });

  it('shows a same-day activity breakdown and scoped trigger state when Patrol is noisy', async () => {
    const referenceDay = new Date();
    const atTime = (hour: number, minute: number) =>
      new Date(
        referenceDay.getFullYear(),
        referenceDay.getMonth(),
        referenceDay.getDate(),
        hour,
        minute,
        0,
      ).toISOString();

    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-alert',
        started_at: atTime(10, 0),
        completed_at: atTime(10, 1),
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'alert_fired',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'ok',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
      {
        id: 'run-anomaly',
        started_at: atTime(9, 0),
        completed_at: atTime(9, 1),
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'anomaly',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'ok',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
      {
        id: 'run-full',
        started_at: atTime(8, 0),
        completed_at: atTime(8, 5),
        duration_ms: 300000,
        type: 'patrol',
        resources_checked: 58,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'ok',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    render(() => (
      <PatrolStatusBar
        refreshTrigger={1}
        triggerStatus={{
          running: true,
          pending_triggers: 4,
          current_interval_ms: 300000,
          recent_events: 6,
          is_busy_mode: true,
          alert_triggers_enabled: true,
          anomaly_triggers_enabled: false,
        }}
      />
    ));

    await waitFor(() => {
      expect(
        screen.getByText('Breakdown: 1 full, 1 alert-triggered, 1 anomaly-triggered'),
      ).toBeInTheDocument();
    });

    expect(
      screen.getByText('Scoped triggers: 4 queued · busy mode · anomalies off'),
    ).toBeInTheDocument();
  });

  it('updates scoped trigger status when trigger data arrives after the initial render', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-busy-scoped',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'alert_fired',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'ok',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);

    const [triggerStatus, setTriggerStatus] = createSignal<PatrolTriggerStatus | undefined>(
      undefined,
    );

    render(() => <PatrolStatusBar refreshTrigger={1} triggerStatus={triggerStatus()} />);

    await waitFor(() => {
      expect(screen.getByText('Recent activity')).toBeInTheDocument();
    });

    expect(screen.queryByText(/Scoped triggers:/)).not.toBeInTheDocument();

    setTriggerStatus({
      running: true,
      pending_triggers: 4,
      current_interval_ms: 300000,
      recent_events: 6,
      is_busy_mode: true,
      alert_triggers_enabled: true,
      anomaly_triggers_enabled: false,
    });

    await waitFor(() => {
      expect(
        screen.getByText('Scoped triggers: 4 queued · busy mode · anomalies off'),
      ).toBeInTheDocument();
    });
  });
});
