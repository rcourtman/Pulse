import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getUnifiedFindings: vi.fn(),
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
          alert_identifier: 'instance:node:100::metric/cpu',
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

  it('falls back to compatibility alert_id when canonical identifier is absent', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
      findings: [
        {
          id: 'finding-2',
          source: 'ai-patrol',
          severity: 'info',
          category: 'ops',
          resource_id: 'agent:node-1',
          resource_name: 'node-1',
          resource_type: 'host',
          title: 'Observed issue',
          description: 'details',
          detected_at: '2026-03-01T00:00:00Z',
          alert_id: 'legacy-alert-id',
        },
      ],
      count: 1,
    });

    await aiIntelligenceStore.loadFindings();

    expect(aiIntelligenceStore.findings[0]).toMatchObject({
      alertIdentifier: 'legacy-alert-id',
    });
  });
});
