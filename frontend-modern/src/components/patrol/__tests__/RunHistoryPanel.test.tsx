import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import { RunHistoryPanel } from '../RunHistoryPanel';

vi.mock('../RunHistoryEntry', () => ({
  RunHistoryEntry: () => <div data-testid="run-history-entry" />,
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

  it('warns that some older runs do not include findings snapshots', () => {
    render(() => (
      <RunHistoryPanel
        runs={[baseRun, { ...baseRun, id: 'run-legacy', finding_ids: undefined }]}
        loading={false}
        selectedRun={null}
        onSelectRun={vi.fn()}
        patrolStream={patrolStream}
      />
    ));

    expect(
      screen.getByText(
        'Select a run to filter findings when available. Some older runs do not include findings snapshots.',
      ),
    ).toBeInTheDocument();
  });

  it('explains when the selected run predates findings snapshots', () => {
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
        'Selected run predates findings snapshots; run-scoped findings cannot be fully verified.',
      ),
    ).toBeInTheDocument();
  });
});
