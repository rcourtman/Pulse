import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionSystem,
  type ProbeResponse,
} from '../connections';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('ConnectionsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('list() calls GET /api/connections and returns the canonical ledger envelope', async () => {
    const connections: Connection[] = [
      {
        id: 'pve-lab',
        type: 'pve',
        name: 'lab',
        address: 'https://pve.lab:8006',
        hostAliases: ['lab', '192.168.0.2'],
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
    const systems: ConnectionSystem[] = [
      {
        id: 'pve-lab',
        type: 'pve',
        clusterName: 'homelab',
        components: [{ connectionId: 'pve-lab', type: 'pve', role: 'primary' }],
        members: [
          {
            id: 'node-lab',
            name: 'lab',
            endpoint: 'https://lab:8006',
            hostAliases: ['lab', '192.168.0.2'],
            state: 'active',
            lastSeen: '2026-04-23T12:00:00Z',
            primary: true,
          },
        ],
      },
    ];
    mockedApiFetchJSON.mockResolvedValueOnce({ connections, systems });

    const result = await ConnectionsAPI.list();

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/connections');
    expect(result).toEqual({ connections, systems });
  });

  it('list() normalizes missing fields to empty collections', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({} as { connections?: Connection[] });

    const result = await ConnectionsAPI.list();

    expect(result).toEqual({ connections: [], systems: [] });
  });

  it('list() preserves agent version metadata on agent-backed connections', async () => {
    const connections: Connection[] = [
      {
        id: 'agent:mini-pc',
        type: 'agent',
        name: 'mini-pc',
        address: 'mini-pc',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-22T20:00:00Z',
        lastError: null,
        source: 'agent',
        agentVersion: '6.0.0',
        expectedAgentVersion: '6.0.2',
        agentUpdateAvailable: true,
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    mockedApiFetchJSON.mockResolvedValueOnce({ connections });

    const result = await ConnectionsAPI.list();

    expect(result.connections[0]).toMatchObject({
      agentVersion: '6.0.0',
      expectedAgentVersion: '6.0.2',
      agentUpdateAvailable: true,
    });
  });

  it('list() preserves canonical host alias metadata on connections and grouped members', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({
      connections: [
        {
          id: 'agent:pi',
          type: 'agent',
          name: 'pi',
          address: 'pi',
          hostAliases: ['pi', '192.168.0.2'],
          state: 'active',
          stateReason: '',
          enabled: true,
          surfaces: ['host'],
          scope: { host: true },
          lastSeen: '2026-04-23T13:00:00Z',
          lastError: null,
          source: 'agent',
          capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
        },
      ],
      systems: [
        {
          id: 'pve:pi',
          type: 'pve',
          components: [
            { connectionId: 'pve:pi', type: 'pve', role: 'primary' },
            { connectionId: 'agent:pi', type: 'agent', role: 'attachment' },
          ],
          members: [
            {
              id: 'pve:pi:member',
              name: 'pi',
              endpoint: 'https://pi:8006',
              hostAliases: ['pi', '192.168.0.2'],
              state: 'active',
            },
          ],
        },
      ],
    });

    const result = await ConnectionsAPI.list();

    expect(result.connections[0]?.hostAliases).toEqual(['pi', '192.168.0.2']);
    expect(result.systems[0]?.members?.[0]?.hostAliases).toEqual(['pi', '192.168.0.2']);
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
