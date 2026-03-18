import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import { RunHistoryEntry } from '../RunHistoryEntry';

const { findingsPanelState } = vi.hoisted(() => ({
  findingsPanelState: {
    latestProps: null as Record<string, unknown> | null,
  },
}));

vi.mock('@/components/AI/FindingsPanel', () => ({
  FindingsPanel: (props: Record<string, unknown>) => {
    findingsPanelState.latestProps = {
      filterFindingIds: Array.isArray(props.filterFindingIds)
        ? [...(props.filterFindingIds as string[])]
        : props.filterFindingIds,
      filterOverride: props.filterOverride,
      scopeResourceIds: Array.isArray(props.scopeResourceIds)
        ? [...(props.scopeResourceIds as string[])]
        : props.scopeResourceIds,
      scopeResourceTypes: Array.isArray(props.scopeResourceTypes)
        ? [...(props.scopeResourceTypes as string[])]
        : props.scopeResourceTypes,
      showControls: props.showControls,
      showScopeWarnings: props.showScopeWarnings,
    };
    return <div data-testid="findings-panel" />;
  },
}));

vi.mock('../RunToolCallTrace', () => ({
  RunToolCallTrace: () => <div data-testid="tool-call-trace" />,
}));

vi.mock('@/components/AI/aiChatUtils', () => ({
  renderMarkdown: (content: string) => content,
}));

describe('RunHistoryEntry', () => {
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

  const run: PatrolRunRecord = {
    id: 'run-1',
    started_at: '2026-03-12T10:00:00Z',
    completed_at: '2026-03-12T10:01:00Z',
    duration_ms: 60000,
    type: 'scoped',
    trigger_reason: 'alert_fired',
    scope_resource_ids: ['seed-resource'],
    effective_scope_resource_ids: [],
    scope_resource_types: ['vm'],
    resources_checked: 0,
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

  beforeEach(() => {
    findingsPanelState.latestProps = null;
  });

  afterEach(() => {
    cleanup();
  });

  it('renders an explicit empty snapshot with canonical scope props', () => {
    render(() => (
      <RunHistoryEntry
        run={run}
        isLive={false}
        patrolStream={patrolStream}
        selected={true}
        onSelect={vi.fn()}
      />
    ));

    expect(screen.getByTestId('findings-panel')).toBeInTheDocument();
    expect(findingsPanelState.latestProps).toMatchObject({
      filterFindingIds: [],
      filterOverride: 'all',
      scopeResourceIds: [],
      scopeResourceTypes: ['vm'],
      showControls: false,
      showScopeWarnings: true,
    });
  });

  it('does not claim all clear when the run still had existing issues', () => {
    render(() => (
      <RunHistoryEntry
        run={{
          ...run,
          id: 'run-existing-issues',
          existing_findings: 2,
          status: 'critical',
          findings_summary: 'Existing issues remain',
        }}
        isLive={false}
        patrolStream={patrolStream}
        selected={true}
        onSelect={vi.fn()}
      />
    ));

    expect(
      screen.getByText((_, element) =>
        element?.tagName === 'P' &&
        (element.textContent?.includes('No new issues, but 2 existing issues remain.') ?? false),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('All clear — no new issues.')).not.toBeInTheDocument();
    expect(screen.queryByText(/^All clear$/)).not.toBeInTheDocument();
  });

  it('surfaces deterministic triage runs that skipped the llm', () => {
    render(() => (
      <RunHistoryEntry
        run={{
          ...run,
          id: 'run-triage-only',
          triage_flags: 3,
          triage_skipped_llm: true,
          findings_summary: 'Quiet infrastructure',
        }}
        isLive={false}
        patrolStream={patrolStream}
        selected={true}
        onSelect={vi.fn()}
      />
    ));

    expect(screen.getByText('3 triage flags')).toBeInTheDocument();
    expect(screen.getByText('LLM skipped')).toBeInTheDocument();
  });
});
