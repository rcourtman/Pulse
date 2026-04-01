import { describe, expect, it } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import {
  formatPatrolActivityBreakdown,
  getPatrolActivityBreakdown,
  getPatrolLatestRunPresentation,
  getPatrolRunKindLabel,
  getPatrolRunCoverageSummary,
  getPatrolRunResourcesHeading,
  getPatrolRunStatusPresentation,
  getPatrolTriggerStatusSummary,
  isPatrolRunHealthy,
  getRunHistoryLoadingState,
  getRunHistorySelectionHint,
  getToolCallsLoadingState,
  getToolCallsUnavailableState,
  getToolCallResultBadgeClass,
  getToolCallResultTextClass,
} from '@/utils/patrolRunPresentation';

describe('patrolRunPresentation', () => {
  const scopedCoverageRun: Pick<
    PatrolRunRecord,
    'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'
  > = {
    resources_checked: 1,
    scope_resource_ids: ['seed-resource'],
    effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
  };

  const fullCoverageRun: Pick<
    PatrolRunRecord,
    'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'
  > = {
    resources_checked: 58,
    scope_resource_ids: [],
    effective_scope_resource_ids: [],
  };

  it('maps critical and error runs to danger styling', () => {
    expect(getPatrolRunStatusPresentation('critical').badgeClass).toContain('red-100');
    expect(getPatrolRunStatusPresentation('error').badgeClass).toContain('red-100');
  });

  it('maps issues found to warning styling', () => {
    const presentation = getPatrolRunStatusPresentation('issues_found');
    expect(presentation.badgeClass).toContain('amber-100');
    expect(presentation.label).toBe('issues found');
  });

  it('maps healthy runs to success styling', () => {
    expect(getPatrolRunStatusPresentation('healthy').badgeClass).toContain('green-100');
  });

  it('downgrades legacy healthy runs without findings snapshots to a neutral completed state', () => {
    expect(getPatrolRunStatusPresentation('healthy', 0, false)).toEqual({
      badgeClass: 'bg-surface-alt text-base-content',
      label: 'completed',
    });
  });

  it('treats erroring runs as error presentation even when the raw status says healthy', () => {
    const presentation = getPatrolRunStatusPresentation('healthy', 2);
    expect(presentation.label).toBe('error');
    expect(presentation.badgeClass).toContain('red-100');
  });

  it('treats only healthy runs without errors as healthy', () => {
    expect(isPatrolRunHealthy('healthy', 0)).toBe(true);
    expect(isPatrolRunHealthy('critical', 0)).toBe(false);
    expect(isPatrolRunHealthy('issues_found', 0)).toBe(false);
    expect(isPatrolRunHealthy('healthy', 1)).toBe(false);
  });

  it('normalizes unknown status labels safely', () => {
    const presentation = getPatrolRunStatusPresentation(' Needs Review ');
    expect(presentation.label).toBe('needs review');
    expect(presentation.badgeClass).toContain('bg-surface-alt');
  });

  it('maps tool call success and failure to canonical colors', () => {
    expect(getToolCallResultBadgeClass(true)).toContain('green-100');
    expect(getToolCallResultBadgeClass(false)).toContain('red-100');
    expect(getToolCallResultTextClass(true)).toContain('text-emerald-600');
    expect(getToolCallResultTextClass(false)).toContain('text-red-600');
  });

  it('returns canonical patrol run kind labels', () => {
    expect(getPatrolRunKindLabel('scoped')).toBe('Scoped run');
    expect(getPatrolRunKindLabel('verification')).toBe('Verification check');
    expect(getPatrolRunKindLabel('patrol')).toBe('Full patrol');
    expect(getPatrolRunKindLabel('')).toBe('Full patrol');
    expect(getPatrolRunKindLabel('unexpected')).toBe('Patrol run');
  });

  it('derives the latest run presentation from recent completed history', () => {
    expect(
      getPatrolLatestRunPresentation([
        {
          id: 'run-latest',
          started_at: '2026-03-12T10:00:00Z',
          completed_at: '2026-03-12T10:01:00Z',
          duration_ms: 60000,
          type: 'scoped',
          trigger_reason: 'alert_fired',
          scope_resource_ids: ['seed-resource'],
          effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
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
          error_count: 1,
          status: 'healthy',
          triage_flags: 0,
          tool_call_count: 0,
          finding_ids: undefined,
        },
      ] satisfies PatrolRunRecord[]),
    ).toEqual({
      coverageSummary: 'Checked 1 of 2 scoped resources',
      findingsSnapshotAvailable: false,
      kindLabel: 'Scoped run',
      status: {
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
        label: 'error',
      },
      timestamp: '2026-03-12T10:01:00Z',
    });
  });

  it('summarizes scoped trigger state for compact activity surfaces', () => {
    expect(
      getPatrolTriggerStatusSummary({
        running: true,
        pending_triggers: 4,
        current_interval_ms: 300000,
        recent_events: 6,
        is_busy_mode: true,
        alert_triggers_enabled: true,
        anomaly_triggers_enabled: false,
      }),
    ).toBe('4 queued · busy mode · anomalies off');
  });

  it('expresses scoped run coverage against the effective scope', () => {
    expect(getPatrolRunCoverageSummary(scopedCoverageRun)).toBe('Checked 1 of 2 scoped resources');
    expect(getPatrolRunResourcesHeading(scopedCoverageRun)).toBe(
      'Resources checked (1 of 2 scoped)',
    );
  });

  it('uses checked-resource language for non-scoped coverage summaries', () => {
    expect(getPatrolRunCoverageSummary(fullCoverageRun)).toBe('Checked 58 resources');
    expect(getPatrolRunResourcesHeading(fullCoverageRun)).toBe('Resources checked (58)');
  });

  it('fails closed on zero-coverage scoped runs', () => {
    expect(
      getPatrolRunCoverageSummary({
        resources_checked: 0,
        scope_resource_ids: ['seed-resource'],
        effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
      }),
    ).toBe('Checked 0 of 2 scoped resources');
  });

  it('summarizes today activity by full patrol versus trigger source', () => {
    const summary = getPatrolActivityBreakdown(
      [
        {
          id: 'full-1',
          started_at: '2026-03-12T08:00:00Z',
          completed_at: '2026-03-12T08:05:00Z',
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
          new_findings: 1,
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
          id: 'alert-1',
          started_at: '2026-03-12T09:00:00Z',
          completed_at: '2026-03-12T09:01:00Z',
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
          id: 'anomaly-1',
          started_at: '2026-03-12T10:00:00Z',
          completed_at: '2026-03-12T10:01:00Z',
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
          new_findings: 2,
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
      ] satisfies PatrolRunRecord[],
      new Date('2026-03-12T12:00:00Z'),
    );

    expect(summary).toMatchObject({
      totalRuns: 3,
      fullPatrols: 1,
      alertTriggeredRuns: 1,
      anomalyTriggeredRuns: 1,
      newFindings: 3,
    });
    expect(formatPatrolActivityBreakdown(summary)).toBe(
      '1 full, 1 alert-triggered, 1 anomaly-triggered',
    );
  });

  it('returns canonical patrol run loading and unavailable copy', () => {
    expect(getRunHistoryLoadingState()).toBe('Loading run history…');
    expect(getToolCallsLoadingState()).toBe('Loading tool calls...');
    expect(getToolCallsUnavailableState()).toBe('Tool call details not available for this run.');
  });

  it('warns when visible runs include legacy findings snapshots', () => {
    expect(getRunHistorySelectionHint([{ finding_ids: [] }, { finding_ids: undefined }])).toBe(
      'Select a run to filter findings when available. Some older runs do not include findings snapshots.',
    );
  });

  it('explains selected legacy runs in the run-history shell', () => {
    expect(getRunHistorySelectionHint([{ finding_ids: [] }], { finding_ids: undefined })).toBe(
      'Selected run predates findings snapshots; run-scoped findings cannot be fully verified.',
    );
  });
});
