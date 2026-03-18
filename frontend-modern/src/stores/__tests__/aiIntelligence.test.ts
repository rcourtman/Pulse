import { beforeEach, describe, expect, it, vi } from 'vitest';

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

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { AIAPI } from '@/api/ai';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';

describe('aiIntelligenceStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it('loads unified findings with canonical alert identity first', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
      findings: [
        {
          id: 'finding-1',
          source: 'threshold',
          severity: 'warning',
          category: 'performance',
          resource_id: 'instance:node:100',
          resource_name: 'vm-100',
          resource_type: 'vm',
          title: 'CPU high',
          description: 'CPU usage is high',
          detected_at: '2026-03-01T00:00:00Z',
          alertIdentifier: 'instance:node:100::metric/cpu',
        },
      ],
      count: 1,
    });

    await aiIntelligenceStore.loadFindings();

    expect(aiIntelligenceStore.findings).toHaveLength(1);
    expect(aiIntelligenceStore.findings[0]).toMatchObject({
      alertIdentifier: 'instance:node:100::metric/cpu',
    });
  });

  it('treats queued fixes without a live approval as findings needing attention', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
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
    });

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);

    await aiIntelligenceStore.loadFindings();
    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.findingsNeedingAttention.map((finding) => finding.id)).toEqual([
      'finding-queued',
    ]);
    expect(aiIntelligenceStore.findingsWithPendingApprovals).toEqual([]);

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'finding-queued',
        targetName: 'node-200',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-03-01T00:01:00Z',
        expiresAt: '2026-04-01T00:06:00Z',
      },
    ]);

    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.findingsNeedingAttention).toEqual([]);
    expect(aiIntelligenceStore.findingsWithPendingApprovals.map((finding) => finding.id)).toEqual([
      'finding-queued',
    ]);
  });

  it('keeps Patrol approval state scoped to investigation_fix approvals', async () => {
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-chat',
        toolId: 'run_command',
        command: 'apt upgrade',
        targetType: 'host',
        targetId: 'host-1',
        targetName: 'node-1',
        context: 'Upgrade packages',
        riskLevel: 'high',
        status: 'pending',
        requestedAt: '2026-03-01T00:01:00Z',
        expiresAt: '2026-04-01T00:06:00Z',
      },
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
        expiresAt: '2026-04-01T00:06:00Z',
      },
    ]);

    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.pendingApprovals.map((approval) => approval.id)).toEqual([
      'approval-fix',
    ]);
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(1);
  });

  it('drops expired approvals from Patrol counts and restores needs-attention immediately', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-01T00:05:00Z'));

    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
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
    });

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
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
        expiresAt: '2026-03-01T00:06:00Z',
      },
    ]);

    await aiIntelligenceStore.loadFindings();
    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.pendingApprovalCount).toBe(1);
    expect(aiIntelligenceStore.findingsWithPendingApprovals.map((finding) => finding.id)).toEqual([
      'finding-queued',
    ]);
    expect(aiIntelligenceStore.findingsNeedingAttention).toEqual([]);

    await vi.advanceTimersByTimeAsync(61_000);

    expect(aiIntelligenceStore.pendingApprovalCount).toBe(0);
    expect(aiIntelligenceStore.findingsWithPendingApprovals).toEqual([]);
    expect(aiIntelligenceStore.findingsNeedingAttention.map((finding) => finding.id)).toEqual([
      'finding-queued',
    ]);

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    await aiIntelligenceStore.loadPendingApprovals();
    vi.useRealTimers();
  });
});
