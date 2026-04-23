import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsAPI, type Connection, type ConnectionSystem } from '@/api/connections';
import { useConnectionsLedger } from '../useConnectionsLedger';

describe('useConnectionsLedger', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders a Proxmox cluster row from the canonical system metadata', async () => {
    const connections: Connection[] = [
      {
        id: 'pve:delly',
        type: 'pve',
        name: 'delly',
        address: 'https://delly:8006',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['vms', 'containers', 'storage', 'backups'],
        scope: { vms: true, containers: true, storage: true, backups: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
      {
        id: 'agent:agent-delly',
        type: 'agent',
        name: 'delly',
        address: 'delly',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
      {
        id: 'agent:agent-minipc',
        type: 'agent',
        name: 'minipc',
        address: 'minipc',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    const systems: ConnectionSystem[] = [
      {
        id: 'pve:delly',
        type: 'pve',
        clusterName: 'homelab',
        components: [
          { connectionId: 'pve:delly', type: 'pve', role: 'primary' },
          { connectionId: 'agent:agent-delly', type: 'agent', role: 'attachment' },
          { connectionId: 'agent:agent-minipc', type: 'agent', role: 'attachment' },
        ],
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0]).toMatchObject({
      id: 'pve:delly',
      ownerType: 'pve',
      name: 'homelab',
      source: 'both',
      host: 'https://delly:8006',
    });
    expect(result.rows()[0].attachedConnections.map((connection) => connection.id)).toEqual([
      'agent:agent-delly',
      'agent:agent-minipc',
    ]);
  });
});
