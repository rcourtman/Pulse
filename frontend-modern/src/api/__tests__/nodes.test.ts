import { describe, expect, it, vi, beforeEach } from 'vitest';
import { NodesAPI } from '../nodes';
import { apiFetchJSON } from '@/utils/apiClient';
import type { NodeConfig } from '@/types/nodes';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('NodesAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getNodes', () => {
    it('fetches all nodes', async () => {
      const mockNodes: NodeConfig[] = [{ id: 'node1', name: 'Node 1' } as NodeConfig];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockNodes);

      const result = await NodesAPI.getNodes();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/config/nodes');
      expect(result).toEqual(mockNodes);
    });
  });

  describe('addNode', () => {
    it('adds a new node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      const node = { id: 'node1', name: 'Node 1' } as NodeConfig;
      await NodesAPI.addNode(node);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(node),
        })
      );
    });
  });

  describe('updateNode', () => {
    it('updates a node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      const node = { id: 'node1', name: 'Updated Node' } as NodeConfig;
      await NodesAPI.updateNode('node1', node);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify(node),
        })
      );
    });

    it('encodes special characters in node id', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      await NodesAPI.updateNode('node/1', {} as NodeConfig);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node%2F1',
        expect.objectContaining({ method: 'PUT' })
      );
    });
  });

  describe('deleteNode', () => {
    it('deletes a node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      await NodesAPI.deleteNode('node1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node1',
        expect.objectContaining({ method: 'DELETE' })
      );
    });
  });

  describe('testConnection', () => {
    it('tests connection to a node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ status: 'ok' });

      const node = { id: 'node1', name: 'Node 1' } as NodeConfig;
      await NodesAPI.testConnection(node);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/test-connection',
        expect.objectContaining({ method: 'POST' })
      );
    });
  });

  describe('testExistingNode', () => {
    it('tests connection to existing node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ status: 'ok', latency: 10 });

      const result = await NodesAPI.testExistingNode('node1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node1/test',
        expect.objectContaining({ method: 'POST' })
      );
      expect(result.latency).toBe(10);
    });
  });

  describe('refreshClusterNodes', () => {
    it('refreshes cluster nodes', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ status: 'ok', newNodeCount: 3 });

      const result = await NodesAPI.refreshClusterNodes('node1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node1/refresh-cluster',
        expect.objectContaining({ method: 'POST' })
      );
      expect(result.newNodeCount).toBe(3);
    });
  });

  describe('getAgentInstallCommand', () => {
    it('gets agent install command', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ command: 'curl ...' });

      const result = await NodesAPI.getAgentInstallCommand({ type: 'pve', enableProxmox: true });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agent-install-command',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ type: 'pve', enableProxmox: true }),
        })
      );
      expect(result.command).toBe('curl ...');
    });
  });
});
