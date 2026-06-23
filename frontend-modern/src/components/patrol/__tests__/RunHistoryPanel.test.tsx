import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import { RunHistoryPanel } from '../RunHistoryPanel';

vi.mock('../RunHistoryEntry', () => ({
  RunHistoryEntry: (props: { run: PatrolRunRecord }) => (
    <div data-testid="run-history-entry">{props.run.id}</div>
  ),
}));

describe('RunHistoryPanel', () => {
  const patrolStream = {
    phase: () => '',
    currentTool: () => '',
    tokens: () => 0,
    resynced: () => false,
    resyncReason: () => '',
    bufferStartSeq: () => 0,
    bufferEndSeq: () => 0,
    outputTruncated: () => false,
    reconnectCount: () => 0,
    isStreaming: () => false,
    errorMessage: () => '',
  };

  const baseRun: PatrolRunRecord = {
    id: 'run-1',
    started_at: '2026-03-12T10:00:00Z',
    completed_at: '2026-03-12T10:01:00Z',
    duration_ms: 60000,
    type: 'patrol',
    trigger_reason: 'scheduled',
    scope_resource_ids: [],
    effective_scope_resource_ids: [],
    scope_resource_types: [],
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
    findings_summary: 'All clear',
    finding_ids: [],
    error_count: 0,
    status: 'healthy',
    triage_flags: 0,
    tool_call_count: 0,
  };

  afterEach(() => {
    cleanup();
  });

  it('frames history as Patrol checks even when older runs lack finding records', () => {
    render(() => (
      <RunHistoryPanel
        runs={[baseRun, { ...baseRun, id: 'run-legacy', finding_ids: undefined }]}
        loading={false}
        selectedRun={null}
        onSelectRun={vi.fn()}
        patrolStream={patrolStream}
      />
    ));

    expect(screen.queryByRole('heading', { name: 'History' })).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'Open a check to review what Patrol found. Older checks may not have issue lists.',
      ),
    ).toBeInTheDocument();
  });

  it('explains when the selected run lacks finding records', () => {
    const legacyRun = { ...baseRun, id: 'run-legacy', finding_ids: undefined };

    render(() => (
      <RunHistoryPanel
        runs={[legacyRun]}
        loading={false}
        selectedRun={legacyRun}
        onSelectRun={vi.fn()}
        patrolStream={patrolStream}
      />
    ));

    expect(
      screen.getByText(
        'This older check has no issue list.',
      ),
    ).toBeInTheDocument();
  });

  it('keeps older run snapshots behind an explicit expansion', () => {
    const runs = Array.from({ length: 10 }, (_, index) => ({
      ...baseRun,
      id: `run-${index + 1}`,
    }));

    render(() => (
      <RunHistoryPanel
        runs={runs}
        loading={false}
        selectedRun={null}
        onSelectRun={vi.fn()}
        patrolStream={patrolStream}
      />
    ));

    expect(screen.getAllByTestId('run-history-entry')).toHaveLength(8);
    expect(screen.getByText('run-1')).toBeInTheDocument();
    expect(screen.getByText('run-8')).toBeInTheDocument();
    expect(screen.queryByText('run-9')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show 2 older runs' }));

    expect(screen.getAllByTestId('run-history-entry')).toHaveLength(10);
    expect(screen.getByText('run-10')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show recent 8 runs' }));

    expect(screen.getAllByTestId('run-history-entry')).toHaveLength(8);
    expect(screen.queryByText('run-10')).not.toBeInTheDocument();
  });
});
