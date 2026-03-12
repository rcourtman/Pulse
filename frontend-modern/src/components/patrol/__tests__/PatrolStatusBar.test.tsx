import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import { PatrolStatusBar } from '../PatrolStatusBar';

const getPatrolRunHistoryMock = vi.hoisted(() => vi.fn<() => Promise<PatrolRunRecord[]>>());

vi.mock('@/api/patrol', () => ({
  getPatrolRunHistory: (...args: unknown[]) => getPatrolRunHistoryMock(...args),
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

  it('shows healthy status only for healthy patrol runs', async () => {
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
      expect(screen.getByText('Running normally')).toBeInTheDocument();
    });
  });

  it('shows issues detected when the last patrol run found issues without transport errors', async () => {
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
      expect(screen.getByText('Issues detected')).toBeInTheDocument();
    });

    expect(screen.queryByText('Running normally')).not.toBeInTheDocument();
  });

  it('does not show healthy status when the run finished with errors', async () => {
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-error-count',
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
      expect(screen.getByText('Issues detected')).toBeInTheDocument();
    });

    expect(screen.queryByText('Running normally')).not.toBeInTheDocument();
  });
});
