import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  getPatrolStatus,
  getPatrolRun,
  getPatrolFindings,
  getPatrolRunHistory,
  getPatrolRunHistoryWithToolCalls,
  getPatrolRunWithToolCalls,
  createSuppressionRuleFromFinding,
  resolveFinding,
  triggerPatrolRun,
  createPatrolAutopilotAcknowledgement,
  revokePatrolAutopilotAcknowledgement,
  updatePatrolAutonomySettings,
  type Finding as PatrolFinding,
} from '@/api/patrol';
import { apiFetchJSON } from '@/utils/apiClient';

describe('patrol api', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchJSONMock.mockResolvedValue([] as any);
  });

  it('uses server acknowledgement and activation endpoints for Autopilot', async () => {
    await createPatrolAutopilotAcknowledgement('ack/one');
    expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/autonomy/acknowledgements', {
      method: 'POST',
      body: JSON.stringify({ acknowledgement_id: 'ack/one' }),
    });

    await updatePatrolAutonomySettings({
      autonomy_level: 'full',
      acknowledgement_id: 'ack/one',
      investigation_budget: 15,
      investigation_timeout_sec: 300,
    });
    expect(apiFetchJSONMock).toHaveBeenLastCalledWith('/api/ai/patrol/autonomy', {
      method: 'PUT',
      body: JSON.stringify({
        autonomy_level: 'full',
        acknowledgement_id: 'ack/one',
        investigation_budget: 15,
        investigation_timeout_sec: 300,
      }),
    });

    await revokePatrolAutopilotAcknowledgement('ack/one', 'operator revoked');
    expect(apiFetchJSONMock).toHaveBeenLastCalledWith(
      '/api/ai/patrol/autonomy/acknowledgements/ack%2Fone',
      { method: 'DELETE', body: JSON.stringify({ reason: 'operator revoked' }) },
    );
  });

  it('normalizes invalid limits for patrol history queries', async () => {
    await getPatrolRunHistory(Number.POSITIVE_INFINITY);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=30');

    await getPatrolRunHistory(0);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=30');

    await getPatrolRunHistory(-5);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=30');
  });

  it('caps oversized patrol history limits to the backend maximum', async () => {
    await getPatrolRunHistory(101);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=100');

    await getPatrolRunHistoryWithToolCalls(999);
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/runs?include=tool_calls&limit=100',
    );
  });

  it('builds tool-call history query with normalized limit', async () => {
    await getPatrolRunHistoryWithToolCalls(25.9);
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/runs?include=tool_calls&limit=25',
    );
  });

  it('fetches a single patrol run by id', async () => {
    await getPatrolRun('run/25');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs/run%2F25');

    await getPatrolRunWithToolCalls('run/25');
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/runs/run%2F25?include=tool_calls',
    );
  });

  it('preserves the canonical patrol runtime state payload', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      runtime_state: 'blocked',
      blocked_reason: 'Connect a provider to power Pulse Assistant and Patrol.',
      blocked_cause: 'provider_not_configured',
      healthy: false,
      readiness: {
        status: 'not_ready',
        ready: false,
        cause: 'model_unsupported_tools',
        summary:
          'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
        provider: 'ollama',
        model: 'ollama:deepseek-r1:7b-llama-distill-q4_K_M',
        checks: [
          {
            id: 'tools',
            status: 'not_ready',
            cause: 'model_unsupported_tools',
            label: 'Patrol tools',
            message:
              'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
            action: 'open_provider_settings',
          },
        ],
      },
    } as any);

    await expect(getPatrolStatus()).resolves.toMatchObject({
      runtime_state: 'blocked',
      blocked_reason: 'Connect a provider to power Pulse Assistant and Patrol.',
      blocked_cause: 'provider_not_configured',
      healthy: false,
      readiness: {
        status: 'not_ready',
        ready: false,
        cause: 'model_unsupported_tools',
        provider: 'ollama',
        model: 'ollama:deepseek-r1:7b-llama-distill-q4_K_M',
        checks: [
          {
            id: 'tools',
            status: 'not_ready',
            cause: 'model_unsupported_tools',
            action: 'open_provider_settings',
          },
        ],
      },
    });
  });

  it('preserves split patrol recency transport fields', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      runtime_state: 'active',
      healthy: true,
      last_patrol_at: '2026-03-12T09:30:00Z',
      last_activity_at: '2026-03-12T09:59:00Z',
      trigger_status: {
        running: true,
        pending_triggers: 2,
        current_interval_ms: 300000,
        recent_events: 6,
        is_busy_mode: true,
        alert_triggers_enabled: true,
        anomaly_triggers_enabled: false,
        event_triggers_blocked: true,
        event_triggers_blocked_reason: 'background_automation_disabled',
        event_triggers_blocked_message:
          'Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
      },
    } as any);

    await expect(getPatrolStatus()).resolves.toMatchObject({
      runtime_state: 'active',
      healthy: true,
      last_patrol_at: '2026-03-12T09:30:00Z',
      last_activity_at: '2026-03-12T09:59:00Z',
      trigger_status: {
        pending_triggers: 2,
        is_busy_mode: true,
        anomaly_triggers_enabled: false,
        event_triggers_blocked: true,
        event_triggers_blocked_reason: 'background_automation_disabled',
      },
    });
  });

  it('normalizes patrol run alert identifiers', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([
      {
        id: 'run-1',
        started_at: '2026-03-01T00:00:00Z',
        completed_at: '2026-03-01T00:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 1,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        findings_summary: 'ok',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        error_summary: 'Selected model does not support Patrol tools',
        error_detail: 'provider rejected tool_choice',
        triage_flags: 2,
        triage_skipped_llm: true,
        tool_call_count: 0,
        alert_identifier: 'canonical-alert-1',
        effective_scope_resource_ids: ['resource-1', 'resource-2'],
      },
    ] as any);

    const result = await getPatrolRunHistory();

    expect(result[0]).toMatchObject({
      alertIdentifier: 'canonical-alert-1',
      effective_scope_resource_ids: ['resource-1', 'resource-2'],
      truenas_checked: 1,
      error_summary: 'Selected model does not support Patrol tools',
      error_detail: 'provider rejected tool_choice',
      triage_flags: 2,
      triage_skipped_llm: true,
    });
  });

  it('fetches direct Patrol findings and preserves canonical alert identity', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([
      {
        id: 'finding-1',
        severity: 'warning',
        category: 'reliability',
        resource_id: 'instance:node:100',
        resource_name: 'vm-100',
        resource_type: 'vm',
        title: 'Provider connection issue',
        description: 'Patrol could not complete provider analysis.',
        detected_at: '2026-03-01T00:00:00Z',
        last_seen_at: '2026-03-01T00:05:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
        alert_identifier: 'instance:node:100::patrol/provider',
      },
    ] as any);

    const findings = await getPatrolFindings();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/findings');
    expect(findings).toHaveLength(1);
    expect(findings[0]).toMatchObject({
      id: 'finding-1',
      alertIdentifier: 'instance:node:100::patrol/provider',
      title: 'Provider connection issue',
    });
  });

  it('bounds include-resolved Patrol finding queries', async () => {
    await getPatrolFindings({ includeResolved: true });
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/findings?include_resolved=1&limit=200',
    );

    await getPatrolFindings({ includeResolved: true, limit: 25.9 });
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/findings?include_resolved=1&limit=25',
    );

    await getPatrolFindings({ includeResolved: true, limit: 9999 });
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/findings?include_resolved=1&limit=500',
    );
  });

  it('normalizes single patrol run payloads', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      id: 'run-2',
      started_at: '2026-03-01T00:00:00Z',
      completed_at: '2026-03-01T00:01:00Z',
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
      findings_summary: 'ok',
      finding_ids: [],
      error_count: 0,
      status: 'healthy',
      triage_flags: 0,
      tool_call_count: 1,
      alert_identifier: 'canonical-alert-2',
    } as any);

    await expect(getPatrolRun('run-2')).resolves.toMatchObject({
      id: 'run-2',
      alertIdentifier: 'canonical-alert-2',
      truenas_checked: 0,
      tool_call_count: 1,
    });
  });

  it('normalizes malformed patrol history payloads to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ runs: [] } as any);

    await expect(getPatrolRunHistory()).resolves.toEqual([]);
  });

  it('normalizes malformed tool-call patrol history payloads to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(null as any);

    await expect(getPatrolRunHistoryWithToolCalls()).resolves.toEqual([]);
  });

  it('normalizes malformed single-run payloads to null', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([] as any);

    await expect(getPatrolRunWithToolCalls('run-3')).resolves.toBeNull();
  });

  it('preserves the FindingsTrustSummary block on patrol status responses', async () => {
    // The trust block surfaces FindingsStore.GetTrustSummary on the patrol
    // page. The TS mirror must round-trip the exact field names from the
    // backend snake_case payload so the workspace strip can read them.
    apiFetchJSONMock.mockResolvedValueOnce({
      runtime_state: 'active',
      healthy: true,
      summary: { critical: 0, warning: 1, watch: 0, info: 0 },
      trust: {
        tracked: 4,
        currently_active: 1,
        resolved: 2,
        auto_resolved: 1,
        fix_verified: 1,
        fix_failed: 0,
        dismissed_as_noise: 1,
        dismissed_as_expected: 0,
        dismissed_as_later: 0,
        suppressed: 0,
        regressed_at_least_once: 1,
      },
    } as any);

    await expect(getPatrolStatus()).resolves.toMatchObject({
      trust: {
        tracked: 4,
        fix_verified: 1,
        dismissed_as_noise: 1,
        regressed_at_least_once: 1,
      },
    });
  });

  it('posts the canonical manual-resolve payload for /api/ai/patrol/resolve', async () => {
    // /api/ai/patrol/resolve was already wired server-side but had no TS
    // client. Pin the canonical request shape so future refactors can't
    // silently drift the body or method — the server returns 405 for GET
    // and 400 for missing finding_id, so the contract is strict.
    apiFetchJSONMock.mockResolvedValueOnce({ success: true, message: 'ok' } as any);

    await resolveFinding('finding-resolve-123');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/resolve', {
      method: 'POST',
      body: JSON.stringify({ finding_id: 'finding-resolve-123' }),
    });
  });

  it('refuses to create broad suppression rules from a finding shortcut', async () => {
    await expect(
      createSuppressionRuleFromFinding({
        resourceId: '',
        resourceName: 'Any resource',
        category: 'capacity',
        description: 'Known pattern',
      }),
    ).rejects.toThrow('resource and category');
    expect(apiFetchJSONMock).not.toHaveBeenCalled();

    await expect(
      createSuppressionRuleFromFinding({
        resourceId: 'resource-1',
        resourceName: 'resource-1',
        category: '',
        description: 'Known pattern',
      }),
    ).rejects.toThrow('resource and category');
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });

  it('trims scoped suppression rules created from findings', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      success: true,
      message: 'ok',
      rule: { id: 'rule-1' },
    } as any);

    await createSuppressionRuleFromFinding({
      resourceId: ' resource-1 ',
      resourceName: ' node-1 ',
      category: ' backup ',
      description: ' Known backup exception ',
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/suppressions', {
      method: 'POST',
      body: JSON.stringify({
        resource_id: 'resource-1',
        resource_name: 'node-1',
        category: 'backup',
        description: 'Known backup exception',
      }),
    });
  });

  it('round-trips remind_at on dismissed-as-will_fix_later patrol findings', async () => {
    // The backend treats will_fix_later as an operator commitment with a
    // wake-up deadline (Finding.RemindAt, default 7 days). The TS API client
    // must mirror remind_at verbatim so the surface can preview the deadline
    // at dismiss-confirm time and badge the dismissed row with the pending
    // reminder. Without this round-trip, the deadline is invisible to the
    // operator until the reminder fires a week later.
    const willFixLater: PatrolFinding = {
      id: 'finding-wfl',
      severity: 'warning',
      category: 'reliability',
      resource_id: 'vm-101',
      resource_name: 'db-01',
      resource_type: 'vm',
      title: 'Disk pressure',
      description: 'Pulse will surface this again on the deadline',
      detected_at: '2026-05-09T10:00:00Z',
      last_seen_at: '2026-05-09T10:05:00Z',
      auto_resolved: false,
      times_raised: 1,
      suppressed: false,
      investigation_attempts: 0,
      dismissed_reason: 'will_fix_later',
      remind_at: '2026-05-16T10:00:00Z',
    };
    apiFetchJSONMock.mockResolvedValueOnce([willFixLater] as any);

    const findings = await getPatrolFindings();
    expect(findings).toHaveLength(1);
    expect(findings[0]?.dismissed_reason).toBe('will_fix_later');
    expect(findings[0]?.remind_at).toBe('2026-05-16T10:00:00Z');
  });

  it('round-trips the deterministic capacity_forecast block on patrol findings', async () => {
    // capacity_forecast is a backend-computed fact (days-to-full, current %,
    // daily change) that the surface renders as the operator's primary urgency
    // signal instead of the model-authored prose. The TS client must mirror it
    // verbatim so the deterministic reading reaches the panel unchanged.
    const filling: PatrolFinding = {
      id: 'finding-forecast',
      severity: 'warning',
      category: 'capacity',
      resource_id: 'storage-tower',
      resource_name: 'Tower Array',
      resource_type: 'storage',
      title: 'Storage pool Tower Array at 86% usage',
      description: 'Pool is filling.',
      detected_at: '2026-05-09T10:00:00Z',
      last_seen_at: '2026-05-09T10:05:00Z',
      auto_resolved: false,
      times_raised: 1,
      suppressed: false,
      investigation_attempts: 0,
      capacity_forecast: {
        metric: 'storage',
        current_pct: 86,
        daily_change: 1.4,
        days_to_full: 10,
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce([filling] as any);

    const findings = await getPatrolFindings();
    expect(findings).toHaveLength(1);
    expect(findings[0]?.capacity_forecast).toEqual({
      metric: 'storage',
      current_pct: 86,
      daily_change: 1.4,
      days_to_full: 10,
    });
  });
});

describe('triggerPatrolRun scope body', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchJSONMock.mockResolvedValue({
      success: true,
      message: 'Triggered targeted Patrol check',
    });
  });

  it('sends no body for a fleet-wide run', async () => {
    await triggerPatrolRun();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/run', { method: 'POST' });
  });

  it('sends a JSON scope body for a targeted check', async () => {
    const scope = {
      resource_ids: ['vm-101'],
      alert_identifier: 'alert-1',
      alert_type: 'cpu',
      context: 'Manual targeted check from alert: cpu',
    };
    await triggerPatrolRun(scope);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/run', {
      method: 'POST',
      body: JSON.stringify(scope),
      headers: { 'Content-Type': 'application/json' },
    });
  });

  it('treats a scope with no real ids as a fleet-wide run', async () => {
    await triggerPatrolRun({ resource_ids: [], resource_types: [] });
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/run', { method: 'POST' });
  });
});
