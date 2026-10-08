import { describe, expect, it } from 'vitest';
import {
  getPatrolRecencyPresentation,
  getPatrolVerificationPresentation,
} from '@/utils/patrolSummaryPresentation';

describe('Patrol recency and verification presentation', () => {
  it('reports a recent successful Patrol check', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
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
            existing_findings: 1,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'issues_found',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Recently checked',
      description: 'The most recent Patrol check completed successfully and covered 58 resources.',
      compactLabel: 'Recently checked',
      tone: 'success',
      lastFullRunAt: '2026-03-12T09:57:00Z',
    });
  });

  it('adds a check mix when targeted runs make recent activity look busy', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-scoped-alert',
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
            findings_summary: '',
            finding_ids: [],
            error_count: 0,
            status: 'healthy',
            triage_flags: 0,
            tool_call_count: 0,
          },
          {
            id: 'run-scoped-anomaly',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
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
            findings_summary: '',
            finding_ids: [],
            error_count: 0,
            status: 'healthy',
            triage_flags: 0,
            tool_call_count: 0,
          },
          {
            id: 'run-full',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
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
            existing_findings: 1,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'issues_found',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Recently checked',
      description: 'The most recent Patrol check completed successfully and covered 58 resources.',
      compactLabel: 'Recently checked',
      tone: 'success',
      lastFullRunAt: '2026-03-12T09:57:00Z',
      activityMixLabel: '1 full check, 1 alert-triggered check, 1 anomaly-triggered check',
    });
  });

  it('reports a partial check when only targeted runs are recent', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
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
            findings_summary: '',
            finding_ids: [],
            error_count: 1,
            status: 'error',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Needs full check',
      description: 'Recent targeted checks covered 1 resource. Run Patrol to check everything.',
      compactLabel: 'Partial check',
      tone: 'warning',
    });
  });

  it('reports a partial check when only follow-up checks are recent', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
            duration_ms: 60000,
            type: 'verification',
            trigger_reason: 'verification',
            resources_checked: 1,
            nodes_checked: 1,
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
            resolved_findings: 1,
            auto_fix_count: 0,
            findings_summary: 'Verification: issue resolved',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'healthy',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Needs full check',
      description: 'Recent follow-up checks covered 1 resource. Run Patrol to check everything.',
      compactLabel: 'Partial check',
      tone: 'warning',
    });
  });

  it('labels targeted recency as the last check', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
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
            findings_summary: '',
            finding_ids: [],
            error_count: 1,
            status: 'error',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-03-12T09:59:00Z',
      resourcesChecked: 1,
      resourcesCheckedLabel: 'checked 1 resource',
    });
  });

  it('labels completed Patrol recency as the last check without claiming verified outcomes', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
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
            existing_findings: 1,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'issues_found',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-03-12T09:57:00Z',
      resourcesChecked: 58,
      resourcesCheckedLabel: 'checked 58 resources',
    });
  });

  it('uses checked coverage wording when a full patrol ends with errors', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-error',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
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
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 1,
            status: 'error',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-03-12T09:57:00Z',
      resourcesChecked: 58,
      resourcesCheckedLabel: 'checked 58 resources',
    });
  });

  it('prefers explicit last activity transport over last full patrol transport when no run history is loaded', () => {
    expect(
      getPatrolRecencyPresentation({
        lastPatrolAt: '2026-03-12T09:57:00Z',
        lastActivityAt: '2026-03-12T09:59:00Z',
      }),
    ).toEqual({
      label: 'Last activity',
      timestamp: '2026-03-12T09:59:00Z',
    });
  });

  it('omits resourcesChecked when the most recent completed run reports zero coverage', () => {
    // A run that completed without checking any resources (e.g. an early
    // failure or a no-op trigger fire) should not surface a "checked 0
    // resources" line on the page header — that reads as an alarm signal
    // when it's just a degenerate run. The presentation must omit the
    // field instead of returning resourcesChecked: 0 so render code's
    // truthy <Show> gate cleanly hides the coverage span.
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-empty',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:51:00Z',
            duration_ms: 60000,
            type: 'patrol',
            resources_checked: 0,
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
            findings_summary: '',
            finding_ids: [],
            error_count: 0,
            status: 'no_issues',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-03-12T09:51:00Z',
    });
  });
});
