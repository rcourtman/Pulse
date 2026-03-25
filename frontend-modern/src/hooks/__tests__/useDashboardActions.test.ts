import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getUnifiedFindings: vi.fn(),
    getPendingApprovals: vi.fn(),
  },
}));

vi.mock('@/api/patrol', () => ({
  acknowledgeFinding: vi.fn(),
  snoozeFinding: vi.fn(),
  dismissFinding: vi.fn(),
  setFindingNote: vi.fn(),
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (feature: string) => feature === 'ai_patrol',
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
import { useDashboardActions } from '@/hooks/useDashboardActions';

describe('useDashboardActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-01T00:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('drops expired Patrol approvals before the next dashboard refresh poll', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValue({
      findings: [
        {
          id: 'finding-queued',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'performance',
          resource_id: 'instance:node:200',
          resource_name: 'node-200',
          resource_type: 'host',
          title: 'Queued remediation',
          description: 'Patrol queued a remediation.',
          detected_at: '2026-03-01T00:00:00Z',
          status: 'active',
          investigation_outcome: 'fix_queued',
        },
      ],
      count: 1,
    } as never);

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValue([
      {
        id: 'approval-fix',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'finding-queued',
        targetName: 'node-200',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-03-01T00:01:00Z',
        expiresAt: '2026-03-01T00:00:20Z',
      },
    ] as never);

    let dispose!: () => void;
    let actions!: ReturnType<typeof useDashboardActions>;

    createRoot((d) => {
      dispose = d;
      const [alertsList] = createSignal<Alert[]>([]);
      actions = useDashboardActions(alertsList);
    });

    await Promise.resolve();
    await Promise.resolve();

    expect(actions.pendingApprovals().map((approval) => approval.id)).toEqual(['approval-fix']);
    expect(actions.findingsNeedingAttention()).toEqual([]);
    expect(actions.totalActionCount()).toBe(1);

    await vi.advanceTimersByTimeAsync(21_000);

    expect(actions.pendingApprovals()).toEqual([]);
    expect(actions.findingsNeedingAttention().map((finding) => finding.id)).toEqual([
      'finding-queued',
    ]);
    expect(actions.totalActionCount()).toBe(1);
    expect(vi.mocked(AIAPI.getPendingApprovals)).toHaveBeenCalledTimes(1);

    dispose();
  });

  it('surfaces the store-owned attention ordering unchanged', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValue({
      findings: [
        {
          id: 'infra-warning',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'infrastructure',
          resource_id: 'instance:node:101',
          resource_name: 'db-01',
          resource_type: 'host',
          title: 'Disk nearly full',
          description: 'Storage usage is high.',
          detected_at: '2026-03-01T00:00:00Z',
          status: 'active',
          investigation_outcome: 'fix_verification_unknown',
        },
        {
          id: 'runtime-warning',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'service',
          resource_id: 'ai-service',
          resource_name: 'Pulse Patrol Service',
          resource_type: 'service',
          title: 'Pulse Patrol: Insufficient API credits',
          description: 'Provider credits are exhausted.',
          detected_at: '2026-03-01T00:01:00Z',
          last_seen_at: '2026-03-01T00:05:00Z',
          status: 'active',
          investigation_outcome: 'fix_failed',
        },
      ],
      count: 2,
    } as never);
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValue([] as never);

    let dispose!: () => void;
    let actions!: ReturnType<typeof useDashboardActions>;

    createRoot((d) => {
      dispose = d;
      const [alertsList] = createSignal<Alert[]>([]);
      actions = useDashboardActions(alertsList);
    });

    await Promise.resolve();
    await Promise.resolve();

    expect(actions.findingsNeedingAttention().map((finding) => finding.id)).toEqual([
      'runtime-warning',
      'infra-warning',
    ]);

    dispose();
  });
});
