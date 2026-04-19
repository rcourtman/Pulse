import { describe, expect, it, vi, beforeEach } from 'vitest';
import { ConnectionsAPI, type Connection, type ProbeResponse } from '../connections';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('ConnectionsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('list() calls GET /api/connections and returns the connections array', async () => {
    const connections: Connection[] = [
      {
        id: 'pve-lab',
        type: 'pve',
        name: 'lab',
        address: 'https://pve.lab:8006',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['vms', 'containers'],
        scope: { vms: true, containers: true },
        lastSeen: '2026-04-19T10:00:00Z',
        lastError: null,
        source: 'manual',
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
    ];
    mockedApiFetchJSON.mockResolvedValueOnce({ connections });

    const result = await ConnectionsAPI.list();

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/connections');
    expect(result).toEqual(connections);
  });

  it('list() returns [] when the backend omits the connections field', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({} as { connections?: Connection[] });

    const result = await ConnectionsAPI.list();

    expect(result).toEqual([]);
  });

  it('probe() POSTs the address JSON and returns the candidates envelope', async () => {
    const response: ProbeResponse = {
      candidates: [
        {
          type: 'pve',
          host: 'https://pve.lab:8006',
          port: 8006,
          hints: { product: 'Proxmox VE', version: '8.2.4' },
        },
      ],
      probedMs: 812,
    };
    mockedApiFetchJSON.mockResolvedValueOnce(response);

    const result = await ConnectionsAPI.probe('pve.lab');

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/connections/probe', {
      method: 'POST',
      body: JSON.stringify({ address: 'pve.lab' }),
    });
    expect(result).toEqual(response);
  });
});
