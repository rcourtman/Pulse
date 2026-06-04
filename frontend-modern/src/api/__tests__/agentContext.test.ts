import { afterEach, describe, expect, it, vi } from 'vitest';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: apiFetchJSONMock,
}));

import { AgentContextAPI } from '@/api/agentContext';

describe('AgentContextAPI', () => {
  afterEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('fetches resource context with encoded canonical or source IDs', async () => {
    apiFetchJSONMock.mockResolvedValue({
      canonicalId: 'system-container-6adaf34f529d241a',
      resourceType: 'system-container',
      resourceName: 'homeassistant',
      discoveryReadiness: {
        state: 'missing',
        reason: 'Discovery has not run for this resource.',
        resourceType: 'system-container',
        targetId: 'agent-delly',
        resourceId: '101',
        generatedAt: '2026-06-04T15:00:00Z',
      },
      activeFindings: [],
      pendingApprovals: [],
      recentActions: [],
      contextSections: [],
      generatedAt: '2026-06-04T15:00:00Z',
    });

    const context = await AgentContextAPI.getResourceContext('delly:delly:101');

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/agent/resource-context/delly%3Adelly%3A101',
    );
    expect(context.discoveryReadiness).toMatchObject({
      state: 'missing',
      targetId: 'agent-delly',
      resourceId: '101',
    });
  });
});
