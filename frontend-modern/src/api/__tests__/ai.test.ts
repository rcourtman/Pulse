import { describe, expect, it, beforeEach, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { AIAPI } from '@/api/ai';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

describe('AIAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchMock.mockReset();
  });

  it('calls the expected endpoints for settings and models', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ configured: true } as any);
    await AIAPI.getSettings();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/settings/ai');

    apiFetchJSONMock.mockResolvedValueOnce({ configured: true } as any);
    await AIAPI.updateSettings({ enabled: true });
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/settings/ai/update', {
      method: 'PUT',
      body: JSON.stringify({ enabled: true }),
    });

    apiFetchJSONMock.mockResolvedValueOnce({ models: [] } as any);
    await AIAPI.getModels();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/models');
  });

  it('includes query parameters for cost endpoints', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getCostSummary();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/cost/summary?days=30');

    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getCostSummary(7);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/cost/summary?days=7');
  });

  it('normalizes unified finding alert identifiers', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      findings: [
        {
          id: 'f1',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          source: 'threshold',
          severity: 'warning',
          category: 'performance',
          title: 'CPU',
          description: 'high',
          detected_at: '2026-03-01T00:00:00Z',
          alert_identifier: 'canonical-alert-1',
          investigation_record: {
            id: 'investigation-1',
            finding_id: 'f1',
            subject: { resource_id: 'r1' },
            trigger: { detected_at: '2026-03-01T00:00:00Z' },
            status: 'completed',
            evidence: [],
            verification: [],
            rollback: [],
            tools_used: [],
            started_at: '2026-03-01T00:00:00Z',
          },
        },
      ],
      count: 1,
    } as any);

    const result = await AIAPI.getUnifiedFindings();

    expect(result.findings[0]).toMatchObject({
      alertIdentifier: 'canonical-alert-1',
      investigation_record: {
        id: 'investigation-1',
        finding_id: 'f1',
      },
    });
  });

  it('preserves the structured investigation-record shape including impact, rollback, and trigger cause', async () => {
    apiFetchJSONMock.mockResolvedValue({
      findings: [
        {
          id: 'f-impact',
          resource_id: 'vm-101',
          source: 'threshold',
          severity: 'critical',
          category: 'storage',
          title: 'Datastore quota exhausted',
          detected_at: '2026-05-08T12:00:00Z',
          alert_identifier: 'canonical-alert-impact',
          investigation_record: {
            id: 'investigation-impact',
            finding_id: 'f-impact',
            subject: { resource_id: 'vm-101' },
            trigger: {
              detected_at: '2026-05-08T12:00:00Z',
              title: 'Datastore quota exhausted',
              cause: 'storage_quota_exceeded',
            },
            status: 'completed',
            outcome: 'fix_queued',
            confidence: 'medium',
            conclusion: 'Datastore quota exhausted.',
            impact: 'Nightly backups will be skipped; recovery window grows by one day per skip.',
            recommended_action: 'Free 200GB before the next backup window.',
            evidence: [{ kind: 'metrics', summary: 'datastore 99% full' }],
            verification: ['Backup job exits 0 on next run'],
            rollback: ['Restore prior retention policy'],
            tools_used: [],
            started_at: '2026-05-08T12:00:00Z',
          },
        },
      ],
      count: 1,
    } as any);

    const result = await AIAPI.getUnifiedFindings();
    const record = result.findings[0].investigation_record;

    expect(record).toBeDefined();
    expect(record!.impact).toBe(
      'Nightly backups will be skipped; recovery window grows by one day per skip.',
    );
    expect(record!.rollback).toEqual(['Restore prior retention policy']);
    expect(record!.trigger.cause).toBe('storage_quota_exceeded');
  });

  it('preserves previous_resolved_fix_summary on UnifiedFinding payloads for operational memory', async () => {
    // The previous-fix summary is captured at regression time and surfaced
    // both in chat context and on the finding card. The TS API client must
    // round-trip the snake_case field so the store normalizer can promote
    // it to the camelCase previousResolvedFixSummary on UnifiedFinding.
    apiFetchJSONMock.mockResolvedValue({
      findings: [
        {
          id: 'f-regress',
          resource_id: 'vm-100',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'reliability',
          title: 'Service stalled',
          description: 'Service stopped responding again',
          previous_resolved_fix_summary:
            'Restart the workload service after backup window clears',
          detected_at: '2026-05-08T12:00:00Z',
        },
      ],
      count: 1,
    } as any);

    const result = await AIAPI.getUnifiedFindings();
    expect(result.findings[0].previous_resolved_fix_summary).toBe(
      'Restart the workload service after backup window clears',
    );
  });

  it('preserves UnifiedFinding-level impact text alongside description and recommendation', async () => {
    apiFetchJSONMock.mockResolvedValue({
      findings: [
        {
          id: 'uf-impact',
          resource_id: 'patrol-runtime',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'reliability',
          title: 'Pulse Patrol: Provider connection issue',
          description:
            'Pulse Patrol could not maintain a healthy connection to the configured provider during analysis.',
          impact:
            'While Patrol cannot analyze, alerts continue to fire without evidence or recommended actions, and AI Intelligence summaries cannot refresh.',
          recommendation:
            'Check provider reachability, base URL, firewall or proxy rules, and provider availability, then rerun Patrol.',
          detected_at: '2026-05-08T12:00:00Z',
          alert_identifier: 'patrol-runtime-failure',
        },
      ],
      count: 1,
    } as any);

    const result = await AIAPI.getUnifiedFindings();
    expect(result.findings[0].impact).toBe(
      'While Patrol cannot analyze, alerts continue to fire without evidence or recommended actions, and AI Intelligence summaries cannot refresh.',
    );
  });

  it('encodes dynamic provider, approval, finding, and plan identifiers', async () => {
    apiFetchJSONMock.mockResolvedValue({} as any);

    await AIAPI.testProvider('openai/internal');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/test/openai%2Finternal', {
      method: 'POST',
    });

    await AIAPI.getRemediationPlan('plan/1?x=1');
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/remediation/plan?plan_id=plan%2F1%3Fx%3D1',
    );

    await AIAPI.approveInvestigationFix('approval/root');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/approvals/approval%2Froot/approve', {
      method: 'POST',
    });

    await AIAPI.reapproveInvestigationFix('finding/root');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/findings/finding%2Froot/reapprove', {
      method: 'POST',
    });
  });

  it('returns provider preflight diagnostics without narrowing the API payload', async () => {
    const diagnostic = {
      success: false,
      message: 'Provider authentication issue',
      provider: 'openrouter',
      model: 'openrouter:deepseek/deepseek-r1',
      cause: 'provider_auth',
      summary:
        'Pulse Patrol cannot analyze your infrastructure because the provider rejected the configured credentials or account access.',
      recommendation:
        'Check the API key or provider authentication in Patrol provider settings, then rerun Patrol.',
      action: 'open_provider_settings',
    };
    apiFetchJSONMock.mockResolvedValueOnce(diagnostic as any);

    await expect(AIAPI.testProvider('openrouter')).resolves.toMatchObject(diagnostic);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/test/openrouter', {
      method: 'POST',
    });
  });

  it('fetches canonical intelligence summaries with encoded resource ids', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getIntelligenceSummary();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/intelligence');

    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getResourceIntelligence('vm/100?filter=all');
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/intelligence?resource_id=vm%2F100%3Ffilter%3Dall',
    );

    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getCorrelations('storage/1?filter=all');
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/intelligence/correlations?resource_id=storage%2F1%3Ffilter%3Dall',
    );
  });

  it('treats 402 responses from optional AI paywalled collections as empty state', async () => {
    const paymentRequiredError = Object.assign(
      new Error('Approval management requires Pulse Pro'),
      {
        status: 402,
      },
    );

    apiFetchJSONMock.mockRejectedValueOnce(paymentRequiredError);
    await expect(AIAPI.getPendingApprovals()).resolves.toEqual([]);

    apiFetchJSONMock.mockRejectedValueOnce(paymentRequiredError);
    await expect(AIAPI.getRemediationPlans()).resolves.toEqual({ plans: [] });
  });

  it('does not treat status text without canonical error status as payment required', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(new Error('402'));

    await expect(AIAPI.getPendingApprovals()).rejects.toThrow('402');
  });

  it('normalizes missing or malformed approval collections to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ approvals: 'bad' } as any);

    await expect(AIAPI.getPendingApprovals()).resolves.toEqual([]);
  });

  it('preserves approval requester identity from pending approvals', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      approvals: [
        {
          id: 'approval-1',
          requestedBy: 'pulse_patrol',
        },
      ],
    } as any);

    await expect(AIAPI.getPendingApprovals()).resolves.toEqual([
      expect.objectContaining({
        id: 'approval-1',
        requestedBy: 'pulse_patrol',
      }),
    ]);
  });

  it('normalizes remediation plans from optional or legacy collection payloads', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ plans: [{ id: 'plan-1' }] } as any);
    await expect(AIAPI.getRemediationPlans()).resolves.toEqual({
      plans: [{ id: 'plan-1' }],
    });

    apiFetchJSONMock.mockResolvedValueOnce({ executions: [{ id: 'exec-1' }] } as any);
    await expect(AIAPI.getRemediationPlans()).resolves.toEqual({ plans: [] });
  });

  it('returns null only for canonical not-found investigation lookups', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(Object.assign(new Error('Not Found'), { status: 404 }));

    await expect(AIAPI.getInvestigation('finding-1')).resolves.toBeNull();
  });

  it('does not swallow non-404 investigation lookup failures', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(
      Object.assign(new Error('Payment Required'), { status: 402 }),
    );
    await expect(AIAPI.getInvestigation('finding-2')).rejects.toThrow('Payment Required');

    apiFetchJSONMock.mockRejectedValueOnce(new Error('backend down'));
    await expect(AIAPI.getInvestigation('finding-3')).rejects.toThrow('backend down');
  });

  it('sanitizes runCommand payload consistently', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ output: 'ok', success: true } as any);
    await AIAPI.runCommand({
      command: 'echo hi',
      target_type: 'vm',
      target_id: 'vm-101',
      run_on_host: undefined as any,
      vmid: 101,
      target_host: 'delly',
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/run-command', {
      method: 'POST',
      body: JSON.stringify({
        command: 'echo hi',
        target_type: 'vm',
        target_id: 'vm-101',
        run_on_host: false,
        vmid: '101',
        target_host: 'delly',
      }),
    });
  });

  it('throws useful errors for investigateAlert failures', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response('backend error', { status: 500 }));
    await expect(
      AIAPI.investigateAlert(
        {
          alertIdentifier: 'a1',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        },
        () => undefined,
      ),
    ).rejects.toThrow('backend error');
  });

  it('throws when investigateAlert has no streaming body', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 200 }));
    await expect(
      AIAPI.investigateAlert(
        {
          alertIdentifier: 'a1',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        },
        () => undefined,
      ),
    ).rejects.toThrow('No response body');
  });

  it('clears read timeout timers when investigateAlert stream reads complete', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();
    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIAPI.investigateAlert(
      {
        alertIdentifier: 'a1',
        resource_id: 'r1',
        resource_name: 'res',
        resource_type: 'vm',
        alert_type: 'cpu',
        level: 'warning',
        value: 1,
        threshold: 2,
        message: 'msg',
        duration: '1m',
      },
      () => undefined,
    );

    expect(read).toHaveBeenCalledTimes(1);
    expect(releaseLock).toHaveBeenCalledTimes(1);
    expect(clearTimeoutSpy).toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });

  it('ignores invalid investigateAlert stream events through the shared JSON-text helper', async () => {
    const encoder = new TextEncoder();
    const read = vi
      .fn()
      .mockResolvedValueOnce({
        done: false,
        value: encoder.encode('data: not valid json\n\n'),
      })
      .mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();
    const onEvent = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIAPI.investigateAlert(
      {
        alertIdentifier: 'a1',
        resource_id: 'r1',
        resource_name: 'res',
        resource_type: 'vm',
        alert_type: 'cpu',
        level: 'warning',
        value: 1,
        threshold: 2,
        message: 'msg',
        duration: '1m',
      },
      onEvent,
    );

    expect(onEvent).not.toHaveBeenCalled();
    expect(logger.error).toHaveBeenCalledWith('[AI] Failed to parse investigation event:', {
      line: 'data: not valid json',
    });
    expect(releaseLock).toHaveBeenCalledTimes(1);
  });

  it('sends canonical alertIdentifier for investigateAlert', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 200 }));

    await expect(
      AIAPI.investigateAlert(
        {
          alertIdentifier: 'instance:node:100::metric/cpu',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        },
        () => undefined,
      ),
    ).rejects.toThrow('No response body');

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/investigate-alert',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          alertIdentifier: 'instance:node:100::metric/cpu',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        }),
      }),
    );
  });

  it('preserves auto_resolved on UnifiedFinding payloads for operator-vs-Pulse closure attribution', async () => {
    // The TS UnifiedFindingRecord must round-trip the auto_resolved boolean
    // verbatim from the backend so the surface can attribute Mark resolved
    // closures (auto_resolved=false) distinctly from Pulse's auto-detection
    // (auto_resolved=true). Without this the operator timeline flattens
    // "you closed this" into generic "condition cleared" copy.
    apiFetchJSONMock.mockResolvedValueOnce({
      findings: [
        {
          id: 'f-manual-close',
          resource_id: 'vm-101',
          resource_name: 'db-01',
          resource_type: 'vm',
          source: 'threshold',
          severity: 'warning',
          category: 'performance',
          title: 'CPU high',
          description: 'CPU usage was high',
          detected_at: '2026-05-09T10:00:00Z',
          last_seen_at: '2026-05-09T10:30:00Z',
          resolved_at: '2026-05-09T11:00:00Z',
          auto_resolved: false,
        },
        {
          id: 'f-auto-close',
          resource_id: 'vm-102',
          resource_name: 'web-01',
          resource_type: 'vm',
          source: 'threshold',
          severity: 'warning',
          category: 'performance',
          title: 'CPU high',
          description: 'CPU usage was high',
          detected_at: '2026-05-09T10:00:00Z',
          last_seen_at: '2026-05-09T10:30:00Z',
          resolved_at: '2026-05-09T11:00:00Z',
          auto_resolved: true,
        },
      ],
      count: 2,
    } as never);

    const result = await AIAPI.getUnifiedFindings();
    expect(result.findings[0]?.auto_resolved).toBe(false);
    expect(result.findings[1]?.auto_resolved).toBe(true);
  });
});
