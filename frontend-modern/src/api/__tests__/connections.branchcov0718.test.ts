import { describe, expect, it, vi, beforeEach } from 'vitest';
import { ConnectionsAPI, type Connection, type ConnectionSystem } from '../connections';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);
const mockedApiFetch = vi.mocked(apiFetch);

const minimalConnection = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:node1',
  type: 'pve',
  name: 'node1',
  address: 'https://pve.lab:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms'],
  scope: { vms: true },
  lastSeen: null,
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const minimalSystem = (overrides: Partial<ConnectionSystem> = {}): ConnectionSystem => ({
  id: 'pve:node1',
  type: 'pve',
  components: [{ connectionId: 'pve:node1', type: 'pve', role: 'primary' }],
  ...overrides,
});

describe('ConnectionsAPI — branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('list() envelope normalization', () => {
    it('normalizes a wire envelope that only carries systems (connections omitted -> [])', async () => {
      const systems = [minimalSystem()];
      mockedApiFetchJSON.mockResolvedValueOnce({ systems } as unknown);

      const result = await ConnectionsAPI.list();

      expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/connections');
      expect(result).toEqual({ connections: [], systems });
    });

    it('normalizes a wire envelope that only carries connections (systems omitted -> [])', async () => {
      const connections = [minimalConnection()];
      mockedApiFetchJSON.mockResolvedValueOnce({ connections } as unknown);

      const result = await ConnectionsAPI.list();

      expect(result).toEqual({ connections, systems: [] });
    });
  });

  describe('probe() request shaping', () => {
    it('POSTs an empty-string address verbatim as {"address":""} to /api/connections/probe', async () => {
      const wire = { candidates: [], probedMs: 0 };
      mockedApiFetchJSON.mockResolvedValueOnce(wire);

      const result = await ConnectionsAPI.probe('');

      expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/connections/probe', {
        method: 'POST',
        body: JSON.stringify({ address: '' }),
      });
      expect(result).toEqual(wire);
    });

    it('JSON-escapes an address that contains a double quote and backslash without corrupting the body', async () => {
      const address = 'host"name\\path';
      mockedApiFetchJSON.mockResolvedValueOnce({ candidates: [], probedMs: 1 });

      await ConnectionsAPI.probe(address);

      expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/connections/probe', {
        method: 'POST',
        body: JSON.stringify({ address }),
      });
    });
  });

  describe('setEnabled() routing by connection type', () => {
    it('pve connection PUTs the full type-prefixed id (encoded) to /api/config/nodes with enabled body', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.setEnabled('pve:node/1', true);

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/config/nodes/pve%3Anode%2F1', {
        method: 'PUT',
        body: JSON.stringify({ enabled: true }),
      });
    });

    it('pbs connection PUTs the full type-prefixed id (encoded) to /api/config/nodes', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.setEnabled('pbs:store/1', false);

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/config/nodes/pbs%3Astore%2F1', {
        method: 'PUT',
        body: JSON.stringify({ enabled: false }),
      });
    });

    it('pmg connection PUTs the full type-prefixed id (encoded) to /api/config/nodes', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.setEnabled('pmg:mail/1', true);

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/config/nodes/pmg%3Amail%2F1', {
        method: 'PUT',
        body: JSON.stringify({ enabled: true }),
      });
    });

    it('vmware connection PUTs only the suffix (encoded) to /api/vmware/connections', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.setEnabled('vmware:vcenter/1', true);

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/vmware/connections/vcenter%2F1', {
        method: 'PUT',
        body: JSON.stringify({ enabled: true }),
      });
    });

    it('truenas connection PUTs only the suffix (encoded) to /api/truenas/connections', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.setEnabled('truenas:box/1', true);

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/truenas/connections/box%2F1', {
        method: 'PUT',
        body: JSON.stringify({ enabled: true }),
      });
    });

    it('throws Pause-not-supported for an agent connection id', async () => {
      await expect(ConnectionsAPI.setEnabled('agent:mini-pc', false)).rejects.toThrow(
        'Pause is not supported for agent connections',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
    });

    it('throws Pause-not-supported for a docker connection id', async () => {
      await expect(ConnectionsAPI.setEnabled('docker:runtime/1', true)).rejects.toThrow(
        'Pause is not supported for docker connections',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
    });

    it('throws Pause-not-supported for a kubernetes connection id', async () => {
      await expect(ConnectionsAPI.setEnabled('kubernetes:cluster/1', false)).rejects.toThrow(
        'Pause is not supported for kubernetes connections',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
    });

    it('throws Unknown-connection-type for an unrecognized prefix', async () => {
      await expect(ConnectionsAPI.setEnabled('foo:bar', true)).rejects.toThrow(
        'Unknown connection type: foo',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
    });

    it('rejects an id with no colon separator before reaching the type switch', async () => {
      await expect(ConnectionsAPI.setEnabled('pve-no-colon', true)).rejects.toThrow(
        'Invalid connection id: pve-no-colon',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
    });

    it('rejects an id whose suffix is empty (trailing colon)', async () => {
      await expect(ConnectionsAPI.setEnabled('pve:', false)).rejects.toThrow(
        'Invalid connection id (empty suffix): pve:',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
    });
  });

  describe('remove() routing by connection type', () => {
    it('pve connection DELETEs the full type-prefixed id (encoded) at /api/config/nodes', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.remove('pve:node/1');

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/config/nodes/pve%3Anode%2F1', {
        method: 'DELETE',
      });
    });

    it('vmware connection DELETEs only the suffix (encoded) at /api/vmware/connections', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.remove('vmware:vcenter/1');

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/vmware/connections/vcenter%2F1', {
        method: 'DELETE',
      });
    });

    it('truenas connection DELETEs only the suffix (encoded) at /api/truenas/connections', async () => {
      mockedApiFetchJSON.mockResolvedValueOnce({ success: true });

      await ConnectionsAPI.remove('truenas:box/1');

      expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/truenas/connections/box%2F1', {
        method: 'DELETE',
      });
    });

    it('agent connection delegates to MonitoringAPI.deleteAgent with the suffix path', async () => {
      mockedApiFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(''),
      } as unknown as Response);

      await ConnectionsAPI.remove('agent:mini-pc');

      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
      expect(mockedApiFetch).toHaveBeenCalledWith(
        '/api/agents/agent/mini-pc',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('throws Remove-not-yet-supported for a docker connection id', async () => {
      await expect(ConnectionsAPI.remove('docker:runtime/1')).rejects.toThrow(
        'Remove is not yet supported for docker connections',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
      expect(mockedApiFetch).not.toHaveBeenCalled();
    });

    it('throws Remove-not-yet-supported for a kubernetes connection id', async () => {
      await expect(ConnectionsAPI.remove('kubernetes:cluster/1')).rejects.toThrow(
        'Remove is not yet supported for kubernetes connections',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
      expect(mockedApiFetch).not.toHaveBeenCalled();
    });

    it('throws Unknown-connection-type for an unrecognized prefix on remove', async () => {
      await expect(ConnectionsAPI.remove('foo:bar')).rejects.toThrow(
        'Unknown connection type: foo',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
      expect(mockedApiFetch).not.toHaveBeenCalled();
    });

    it('rejects a remove id with no colon separator', async () => {
      await expect(ConnectionsAPI.remove('pve-no-colon')).rejects.toThrow(
        'Invalid connection id: pve-no-colon',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
      expect(mockedApiFetch).not.toHaveBeenCalled();
    });

    it('rejects a remove id with an empty suffix', async () => {
      await expect(ConnectionsAPI.remove('vmware:')).rejects.toThrow(
        'Invalid connection id (empty suffix): vmware:',
      );
      expect(mockedApiFetchJSON).not.toHaveBeenCalled();
      expect(mockedApiFetch).not.toHaveBeenCalled();
    });
  });
});
