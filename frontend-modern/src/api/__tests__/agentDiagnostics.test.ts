import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { AgentDiagnosticsAPI } from '../agentDiagnostics';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('AgentDiagnosticsAPI', () => {
  beforeEach(() => vi.clearAllMocks());

  it('loads the read-only fleet diagnostic endpoint', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({
      schemaVersion: 1,
      generatedAt: 123,
      serverVersion: '6.2.0',
      agentUpdateTargetVersion: '6.2.0',
      summary: { total: 1, warning: 1 },
      agents: [
        {
          connectionId: 'agent:host-1',
          rowKey: 'agent-host-1',
          id: 'host-1',
          name: 'host-1',
          types: ['host'],
          status: 'warning',
          reasons: [],
        },
      ],
    });

    const result = await AgentDiagnosticsAPI.getFleetDiagnostics();

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/agents/diagnostics');
    expect(result.summary).toEqual({
      total: 1,
      healthy: 0,
      warning: 1,
      critical: 0,
      removed: 0,
    });
    expect(result.schemaVersion).toBe(1);
    expect(result.agentUpdateTargetVersion).toBe('6.2.0');
    expect(result.agents[0].connectionId).toBe('agent:host-1');
  });

  it('normalizes omitted collections for rolling upgrades', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({});

    await expect(AgentDiagnosticsAPI.getFleetDiagnostics()).resolves.toMatchObject({
      schemaVersion: 0,
      generatedAt: 0,
      agents: [],
      summary: { total: 0, healthy: 0, warning: 0, critical: 0, removed: 0 },
    });
  });
});
