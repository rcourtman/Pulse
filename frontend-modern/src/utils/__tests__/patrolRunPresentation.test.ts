import { describe, expect, it } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import {
  formatPatrolActivityBreakdown,
  getPatrolActivityBreakdown,
  getPatrolLatestRunPresentation,
  getPatrolRunOperatorRecordPresentation,
  getPatrolRunRecordSummaryPresentation,
  getPatrolRunKindLabel,
  getPatrolRunCoverageSummary,
  getPatrolRunPrimaryActionPresentation,
  getPatrolRunResourcesHeading,
  getPatrolRunStatusPresentation,
  getPatrolTriggerStatusSummary,
  isPatrolRunHealthy,
  getRunHistoryLoadingState,
  getRunHistorySelectionHint,
  getToolCallsLoadingState,
  getToolCallsUnavailableState,
  getToolCallResultBadgeTone,
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
    expect(getPatrolRunStatusPresentation('error').variant).toBe('danger');
  });

  it('maps issues found to warning styling', () => {
    const presentation = getPatrolRunStatusPresentation('issues_found');
    expect(presentation.badgeClass).toContain('amber-100');
    expect(presentation.variant).toBe('warning');
    expect(presentation.label).toBe('issues found');
  });

  it('maps healthy runs to success styling', () => {
    expect(getPatrolRunStatusPresentation('healthy').badgeClass).toContain('green-100');
    expect(getPatrolRunStatusPresentation('healthy').variant).toBe('success');
  });

  it('downgrades legacy healthy runs without finding records to a neutral completed state', () => {
    expect(getPatrolRunStatusPresentation('healthy', 0, false)).toEqual({
      badgeClass: 'bg-surface-alt text-base-content',
      variant: 'muted',
      label: 'completed',
    });
  });

  it('treats erroring runs as error presentation even when the raw status says healthy', () => {
    const presentation = getPatrolRunStatusPresentation('healthy', 2);
    expect(presentation.label).toBe('error');
    expect(presentation.badgeClass).toContain('red-100');
    expect(presentation.variant).toBe('danger');
  });

  it('treats only healthy runs without errors as healthy', () => {
    expect(isPatrolRunHealthy('healthy', 0)).toBe(true);
    expect(isPatrolRunHealthy('critical', 0)).toBe(false);
    expect(isPatrolRunHealthy('issues_found', 0)).toBe(false);
    expect(isPatrolRunHealthy('healthy', 1)).toBe(false);
  });

  it('offers Patrol provider settings for runtime-failed runs only', () => {
    expect(
      getPatrolRunPrimaryActionPresentation({
        error_count: 1,
        error_summary: 'Selected model does not support Patrol tools',
        error_detail: 'Provider rejected tool_choice',
      }),
    ).toEqual({
      label: 'Open Provider & Models',
      href: '/settings/pulse-intelligence/provider',
    });
    expect(
      getPatrolRunPrimaryActionPresentation({
        error_count: 0,
        error_summary: '',
        error_detail: '',
      }),
    ).toBeUndefined();
    expect(
      getPatrolRunPrimaryActionPresentation({
        error_count: 1,
        error_summary: '',
        error_detail: '',
      }),
    ).toBeUndefined();
  });

  it('summarizes selected Patrol run records without requiring users to expand history internals', () => {
    expect(
      getPatrolRunRecordSummaryPresentation({
        auto_fix_count: 0,
        duration_ms: 3_480_000,
        effective_scope_resource_ids: [],
        error_count: 1,
        error_detail:
          "agentic patrol failed: API error (404): No endpoints found that support the provided 'tool_choice' value.",
        error_summary: 'Selected model does not support Patrol tools',
        existing_findings: 0,
        finding_ids: ['runtime-provider'],
        new_findings: 0,
        resources_checked: 72,
        scope_resource_ids: [],
        status: 'error',
      }),
    ).toEqual({
      action: {
        label: 'Open Provider & Models',
        href: '/settings/pulse-intelligence/provider',
      },
      summary: 'Checked 72 resources in 58m.',
      outcome:
        'Patrol ended with a runtime issue: Selected model does not support Patrol tools: Provider rejected Patrol tool calls. Choose a Patrol model and endpoint with tool-call support.',
    });

    expect(
      getPatrolRunRecordSummaryPresentation({
        auto_fix_count: 0,
        duration_ms: 60_000,
        effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
        error_count: 0,
        error_detail: '',
        error_summary: '',
        existing_findings: 0,
        finding_ids: [],
        new_findings: 0,
        resources_checked: 2,
        scope_resource_ids: ['seed-resource'],
        status: 'healthy',
      }),
    ).toEqual({
      summary: 'Checked 2 scoped resources in 1m.',
      outcome: 'No issues recorded for this run.',
    });
  });

  it('summarizes run history as operator actions before telemetry details', () => {
    expect(
      getPatrolRunOperatorRecordPresentation({
        auto_fix_count: 1,
        duration_ms: 60_000,
        effective_scope_resource_ids: [],
        error_count: 0,
        error_detail: '',
        error_summary: '',
        existing_findings: 0,
        finding_ids: ['finding-1', 'finding-2'],
        new_findings: 2,
        resources_checked: 72,
        resolved_findings: 0,
        scope_resource_ids: [],
        status: 'healthy',
      }),
    ).toEqual({
      headline: 'Fixed 1 issue',
      detail: 'Checked 72 resources in 1m. Found 2 new issues and fixed 1 issue.',
    });

    expect(
      getPatrolRunOperatorRecordPresentation({
        auto_fix_count: 0,
        duration_ms: 453,
        effective_scope_resource_ids: ['vm-1', 'vm-2'],
        error_count: 1,
        error_detail:
          "agentic patrol failed: API error (404): No endpoints found that support the provided 'tool_choice' value.",
        error_summary: 'Selected model does not support Patrol tools',
        existing_findings: 0,
        finding_ids: ['runtime-provider'],
        new_findings: 0,
        resources_checked: 1,
        resolved_findings: 0,
        scope_resource_ids: ['vm-1'],
        status: 'error',
      }),
    ).toEqual({
      headline: 'Patrol needs attention',
      detail:
        'Checked 1 of 2 scoped resources in 453ms. Runtime issue: Selected model does not support Patrol tools: Provider rejected Patrol tool calls. Choose a Patrol model and endpoint with tool-call support.',
    });

    expect(
      getPatrolRunOperatorRecordPresentation({
        auto_fix_count: 0,
        duration_ms: 120_000,
        effective_scope_resource_ids: [],
        error_count: 1,
        error_detail:
          'agentic patrol failed: provider error: stream read error: stream chunk timed out after 12s',
        error_summary: 'Provider analysis error',
        existing_findings: 0,
        finding_ids: ['runtime-provider'],
        new_findings: 0,
        resources_checked: 66,
        resolved_findings: 0,
        scope_resource_ids: [],
        status: 'error',
      }),
    ).toEqual({
      headline: 'Patrol needs attention',
      detail:
        'Checked 66 resources in 2m. Runtime issue: Provider connection failed. Check provider reachability before retrying Patrol.',
    });

    expect(
      getPatrolRunOperatorRecordPresentation({
        auto_fix_count: 0,
        duration_ms: 60_000,
        effective_scope_resource_ids: [],
        error_count: 0,
        error_detail: '',
        error_summary: '',
        existing_findings: 0,
        finding_ids: [],
        new_findings: 0,
        resources_checked: 72,
        resolved_findings: 0,
        scope_resource_ids: [],
        status: 'healthy',
      }),
    ).toEqual({
      headline: 'All clear',
      detail: 'Checked 72 resources in 1m. No issues recorded.',
    });
  });

  it('normalizes unknown status labels safely', () => {
    const presentation = getPatrolRunStatusPresentation(' Needs Review ');
    expect(presentation.label).toBe('needs review');
    expect(presentation.badgeClass).toContain('bg-surface-alt');
    expect(presentation.variant).toBe('muted');
  });

  it('maps tool call success and failure to canonical badge tones', () => {
    expect(getToolCallResultBadgeTone(true)).toBe('success');
    expect(getToolCallResultBadgeTone(false)).toBe('danger');
    expect(getToolCallResultTextClass(true)).toContain('text-emerald-600');
    expect(getToolCallResultTextClass(false)).toContain('text-red-600');
  });

  it('returns canonical patrol run kind labels', () => {
    expect(getPatrolRunKindLabel('scoped')).toBe('Targeted check');
    expect(getPatrolRunKindLabel('verification')).toBe('Follow-up check');
    expect(getPatrolRunKindLabel('patrol')).toBe('Patrol check');
    expect(getPatrolRunKindLabel('')).toBe('Patrol check');
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
      kindLabel: 'Targeted check',
      status: {
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
        variant: 'danger',
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

  it('hides non-actionable runtime-blocked event triggers unless Patrol is already running', () => {
    expect(
      getPatrolTriggerStatusSummary({
        running: true,
        pending_triggers: 4,
        current_interval_ms: 300000,
        recent_events: 6,
        is_busy_mode: true,
        alert_triggers_enabled: true,
        anomaly_triggers_enabled: true,
        event_triggers_blocked: true,
        event_triggers_blocked_reason: 'background_automation_disabled',
        event_triggers_blocked_message:
          'Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
      }),
    ).toBeUndefined();

    expect(
      getPatrolTriggerStatusSummary(
        {
          running: true,
          pending_triggers: 4,
          current_interval_ms: 300000,
          recent_events: 6,
          is_busy_mode: true,
          alert_triggers_enabled: true,
          anomaly_triggers_enabled: true,
          event_triggers_blocked: true,
          event_triggers_blocked_reason: 'background_automation_disabled',
          event_triggers_blocked_message:
            'Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
        },
        {
          manualRunAvailable: false,
          manualRunBlockedReason: 'Provider connection issue (last preflight 49m ago).',
        },
      ),
    ).toBeUndefined();

    expect(
      getPatrolTriggerStatusSummary(
        {
          running: true,
          pending_triggers: 0,
          current_interval_ms: 300000,
          recent_events: 0,
          is_busy_mode: false,
          alert_triggers_enabled: true,
          anomaly_triggers_enabled: true,
          event_triggers_blocked: true,
          event_triggers_blocked_reason: 'background_automation_disabled',
          event_triggers_blocked_message:
            'Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
        },
        {
          manualRunAvailable: false,
          manualRunBlockedReason: 'Patrol is already running',
        },
      ),
    ).toBe(
      'A Patrol run is already in progress. New automatic and manual runs are paused until it finishes.',
    );
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
      '1 full check, 1 alert-triggered check, 1 anomaly-triggered check',
    );
  });

  it('returns canonical patrol run loading and unavailable copy', () => {
    expect(getRunHistoryLoadingState()).toBe('Loading history...');
    expect(getToolCallsLoadingState()).toBe('Loading tool calls...');
    expect(getToolCallsUnavailableState()).toBe('Tool call details not available for this run.');
  });

  it('warns when visible runs include legacy finding records', () => {
    expect(getRunHistorySelectionHint([{ finding_ids: [] }, { finding_ids: undefined }])).toBe(
      'Open a check to review what Patrol found. Older checks may not have issue lists.',
    );
  });

  it('explains selected legacy runs in the run-history shell', () => {
    expect(getRunHistorySelectionHint([{ finding_ids: [] }], { finding_ids: undefined })).toBe(
      'This older check has no issue list.',
    );
  });
});
