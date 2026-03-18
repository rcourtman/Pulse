import { describe, expect, it, vi, beforeEach } from 'vitest';
import { NodesAPI } from '../nodes';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import type { NodeConfig, PVENodeConfig } from '@/types/nodes';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: vi.fn(),
}));

describe('NodesAPI', () => {
  const makePveNode = (
    overrides: Partial<PVENodeConfig & { type: 'pve' }> = {},
  ): PVENodeConfig & { type: 'pve' } => ({
    id: 'pve-1',
    type: 'pve',
    name: 'PVE 1',
    host: 'https://pve.local:8006',
    user: 'root@pam',
    verifySSL: true,
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    monitorPhysicalDisks: false,
    ...overrides,
  });

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

    it('normalizes canonical cluster endpoint fields', async () => {
      const mockNodes = [
        makePveNode({
          clusterEndpoints: [
            {
              nodeId: ' node-1 ',
              nodeName: ' pve-1 ',
              host: ' https://pve-1.local:8006 ',
              guestURL: ' https://guest.local ',
              ip: ' 10.0.0.1 ',
              ipOverride: ' 10.0.0.10 ',
              fingerprint: ' fp ',
              online: true,
              lastSeen: ' 2026-03-04T00:00:00Z ',
              pulseReachable: null,
              lastPulseCheck: ' 2026-03-04T00:00:01Z ',
              pulseError: ' timeout ',
            },
          ],
        }),
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockNodes);

      const result = await NodesAPI.getNodes();
      const endpoint = (
        result[0] as NodeConfig & { clusterEndpoints: Array<Record<string, unknown>> }
      ).clusterEndpoints[0];

      expect(endpoint).toMatchObject({
        nodeId: 'node-1',
        nodeName: 'pve-1',
        host: 'https://pve-1.local:8006',
        guestURL: 'https://guest.local',
        ip: '10.0.0.1',
        ipOverride: '10.0.0.10',
        fingerprint: 'fp',
        online: true,
        lastSeen: '2026-03-04T00:00:00Z',
        lastPulseCheck: '2026-03-04T00:00:01Z',
        pulseError: 'timeout',
      });
      expect(endpoint.pulseReachable).toBeUndefined();
    });

    it('uses canonical scalar coercion for non-string and invalid cluster endpoint fields', async () => {
      const mockNodes = [
        makePveNode({
          id: 'pve-2',
          name: 'PVE 2',
          clusterEndpoints: [
            {
              nodeId: 22,
              nodeName: null,
              host: ' https://pve-2.local:8006 ',
              guestURL: '   ',
              ip: 10,
              online: 'true',
              lastSeen: undefined,
              pulseReachable: false,
            },
          ] as unknown as PVENodeConfig['clusterEndpoints'],
        }),
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockNodes);

      const result = await NodesAPI.getNodes();
      const endpoint = (
        result[0] as NodeConfig & { clusterEndpoints: Array<Record<string, unknown>> }
      ).clusterEndpoints[0];

      expect(endpoint).toMatchObject({
        nodeId: '22',
        nodeName: '',
        host: 'https://pve-2.local:8006',
        guestURL: undefined,
        ip: '10',
        online: false,
        lastSeen: '',
        pulseReachable: false,
      });
    });

    it('preserves nodes without array clusterEndpoints', async () => {
      const mockNodes = [
        {
          id: 'pve-3',
          type: 'pve',
          name: 'PVE 3',
          clusterEndpoints: 'not-an-array',
        } as unknown as NodeConfig,
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockNodes);

      const result = await NodesAPI.getNodes();

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
        }),
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
        }),
      );
    });

    it('encodes special characters in node id', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      await NodesAPI.updateNode('node/1', {} as NodeConfig);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node%2F1',
        expect.objectContaining({ method: 'PUT' }),
      );
    });
  });

  describe('deleteNode', () => {
    it('deletes a node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      await NodesAPI.deleteNode('node1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node1',
        expect.objectContaining({ method: 'DELETE' }),
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
        expect.objectContaining({ method: 'POST' }),
      );
    });
  });

  describe('testExistingNode', () => {
    it('tests connection to existing node', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ status: 'ok', latency: 10 });

      const result = await NodesAPI.testExistingNode('node1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/config/nodes/node1/test',
        expect.objectContaining({ method: 'POST' }),
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
        expect.objectContaining({ method: 'POST' }),
      );
      expect(result.newNodeCount).toBe(3);
    });
  });

  describe('getAgentInstallCommand', () => {
    it('gets agent install command', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ command: ' curl ... ', token: 'secret-token' });

      const result = await NodesAPI.getAgentInstallCommand({ type: 'pve', enableProxmox: true });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agent-install-command',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ type: 'pve', enableProxmox: true }),
        }),
      );
      expect(result).toEqual({ command: 'curl ...' });
      expect(result).not.toHaveProperty('token');
    });

    it('supports the PBS proxmox install command contract', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ command: 'curl pbs ...' });

      const result = await NodesAPI.getAgentInstallCommand({ type: 'pbs', enableProxmox: true });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agent-install-command',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ type: 'pbs', enableProxmox: true }),
        }),
      );
      expect(result.command).toBe('curl pbs ...');
    });

    it('rejects blank install commands', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ command: '   ', token: 'secret-token' });

      await expect(
        NodesAPI.getAgentInstallCommand({ type: 'pve', enableProxmox: true }),
      ).rejects.toThrow('Invalid agent install command response');
    });
  });

  describe('getProxmoxSetupCommand', () => {
    it('uses the canonical setup-script-url contract for PVE while keeping raw setup tokens inside the shared client boundary', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pve',
        host: 'https://pve.example:8006',
        url: 'https://pulse.example/api/setup-script?type=pve',
        downloadURL: 'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
        scriptFileName: 'pulse-setup-pve.sh',
        command: 'curl pve ...',
        setupToken: 'setup-token-123',
        tokenHint: 'set…123',
        expires: 1_900_000_000,
      });

      const result = await NodesAPI.getProxmoxSetupCommand({
        type: 'pve',
        host: 'pve.example',
        backupPerms: true,
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/setup-script-url',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            type: 'pve',
            host: 'pve.example',
            backupPerms: true,
          }),
        }),
      );
      expect(result.type).toBe('pve');
      expect(result.host).toBe('https://pve.example:8006');
      expect(result.downloadURL).toBe(
        'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
      );
      expect(result.scriptFileName).toBe('pulse-setup-pve.sh');
      expect(result.tokenHint).toBe('set…123');
      expect(result.expires).toBe(1_900_000_000);
      expect(result).not.toHaveProperty('setupToken');
    });

    it('normalizes the canonical setup-script-url response fields', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: ' pve ',
        host: ' https://pve.example:8006 ',
        url: ' https://pulse.example/api/setup-script?type=pve ',
        downloadURL: ' https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123 ',
        scriptFileName: ' pulse-setup-pve.sh ',
        command: ' curl pve ... ',
        commandWithEnv: ' curl env pve ... ',
        commandWithoutEnv: ' curl bare pve ... ',
        setupToken: ' setup-token-123 ',
        expires: 1_900_000_000,
        tokenHint: ' set…123 ',
      });

      const result = await NodesAPI.getProxmoxSetupCommand({
        type: 'pve',
        host: 'pve.example',
        backupPerms: true,
      });

      expect(result).toEqual({
        type: 'pve',
        host: 'https://pve.example:8006',
        url: 'https://pulse.example/api/setup-script?type=pve',
        downloadURL: 'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
        scriptFileName: 'pulse-setup-pve.sh',
        command: 'curl pve ...',
        commandWithEnv: 'curl env pve ...',
        commandWithoutEnv: 'curl bare pve ...',
        expires: 1_900_000_000,
        tokenHint: 'set…123',
      });
    });

    it('supports the PBS proxmox setup command contract', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pbs',
        host: 'https://pbs.example:8007',
        url: 'https://pulse.example/api/setup-script?type=pbs',
        downloadURL: 'https://pulse.example/api/setup-script?type=pbs&setup_token=pbs-token-123',
        scriptFileName: 'pulse-setup-pbs.sh',
        command: 'curl pbs ...',
        setupToken: 'pbs-token-123',
        tokenHint: 'pbs…123',
        expires: 1_900_000_000,
      });

      const result = await NodesAPI.getProxmoxSetupCommand({
        type: 'pbs',
        host: 'pbs.example',
        backupPerms: false,
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/setup-script-url',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            type: 'pbs',
            host: 'pbs.example',
            backupPerms: false,
          }),
        }),
      );
      expect(result.command).toBe('curl pbs ...');
      expect(result.type).toBe('pbs');
      expect(result.host).toBe('https://pbs.example:8007');
      expect(result.downloadURL).toBe(
        'https://pulse.example/api/setup-script?type=pbs&setup_token=pbs-token-123',
      );
      expect(result.scriptFileName).toBe('pulse-setup-pbs.sh');
      expect(result.tokenHint).toBe('pbs…123');
      expect(result).not.toHaveProperty('setupToken');
    });

    it('falls back to command for commandWithEnv when the backend omits it', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pbs',
        host: 'https://pbs.example:8007',
        url: 'https://pulse.example/api/setup-script?type=pbs',
        downloadURL: 'https://pulse.example/api/setup-script?type=pbs&setup_token=pbs-token-123',
        scriptFileName: 'pulse-setup-pbs.sh',
        command: 'curl pbs ...',
        setupToken: 'pbs-token-123',
        tokenHint: 'pbs…123',
        expires: 1_900_000_000,
      });

      const result = await NodesAPI.getProxmoxSetupCommand({
        type: 'pbs',
        host: 'pbs.example',
        backupPerms: false,
      });

      expect(result.commandWithEnv).toBe('curl pbs ...');
    });

    it('rejects malformed canonical setup-script-url responses', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pbs',
        host: 'https://pbs.example:8007',
        url: '',
        downloadURL: 'https://pulse.example/api/setup-script?type=pbs&setup_token=pbs-token-123',
        scriptFileName: 'pulse-setup-pbs.sh',
        command: 'curl pbs ...',
        setupToken: 'pbs-token-123',
        tokenHint: 'pbs…123',
        expires: 1_900_000_000,
      });

      await expect(
        NodesAPI.getProxmoxSetupCommand({
          type: 'pve',
          host: 'pbs.example',
          backupPerms: false,
        }),
      ).rejects.toThrow('Invalid Proxmox setup response type');
    });

    it('rejects expired canonical setup-script-url responses', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pve',
        host: 'https://pve.example:8006',
        url: 'https://pulse.example/api/setup-script?type=pve',
        downloadURL: 'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
        scriptFileName: 'pulse-setup-pve.sh',
        command: 'curl pve ...',
        setupToken: 'setup-token-123',
        tokenHint: 'set…123',
        expires: 1,
      });

      await expect(
        NodesAPI.getProxmoxSetupCommand({
          type: 'pve',
          host: 'pve.example',
          backupPerms: true,
        }),
      ).rejects.toThrow('Invalid Proxmox setup response expiry');
    });

    it('rejects setup-script-url responses missing the canonical script filename', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pve',
        host: 'https://pve.example:8006',
        url: 'https://pulse.example/api/setup-script?type=pve',
        downloadURL: 'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
        command: 'curl pve ...',
        setupToken: 'setup-token-123',
        tokenHint: 'set…123',
        expires: 1_900_000_000,
      });

      await expect(
        NodesAPI.getProxmoxSetupCommand({
          type: 'pve',
          host: 'pve.example',
          backupPerms: true,
        }),
      ).rejects.toThrow('Invalid Proxmox setup response scriptFileName');
    });

    it('rejects setup-script-url responses missing the canonical token hint', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        type: 'pve',
        host: 'https://pve.example:8006',
        url: 'https://pulse.example/api/setup-script?type=pve',
        downloadURL: 'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
        scriptFileName: 'pulse-setup-pve.sh',
        command: 'curl pve ...',
        setupToken: 'setup-token-123',
        expires: 1_900_000_000,
      });

      await expect(
        NodesAPI.getProxmoxSetupCommand({
          type: 'pve',
          host: 'pve.example',
          backupPerms: true,
        }),
      ).rejects.toThrow('Invalid Proxmox setup response token hint');
    });
  });

  describe('downloadProxmoxSetupScript', () => {
    it('downloads the PVE setup script through the canonical client path', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response('#!/bin/bash\necho pve', {
          status: 200,
          headers: {
            'Content-Type': 'text/x-shellscript; charset=utf-8',
            'Content-Disposition': 'attachment; filename="pulse-setup-pve.sh"',
          },
        }),
      );

      const result = await NodesAPI.downloadProxmoxSetupScript({
        type: 'pve',
        host: 'https://pve.example:8006',
        url: 'https://pulse.example/base/api/setup-script?type=pve',
        downloadURL: 'https://pulse.example/base/api/setup-script?type=pve&setup_token=setup-token-123',
        scriptFileName: 'pulse-setup-pve.sh',
        command: 'curl pve ...',
        commandWithEnv: 'curl env pve ...',
        commandWithoutEnv: 'curl bare pve ...',
        expires: 1_900_000_000,
        tokenHint: 'set…123',
      });

      expect(apiFetch).toHaveBeenCalledWith(
        'https://pulse.example/base/api/setup-script?type=pve&setup_token=setup-token-123',
      );
      expect(result).toEqual({
        content: '#!/bin/bash\necho pve',
        contentType: 'text/x-shellscript; charset=utf-8',
        fileName: 'pulse-setup-pve.sh',
      });
    });

    it('downloads the PBS setup script without the PVE-only backup perms flag', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response('#!/bin/bash\necho pbs', {
          status: 200,
          headers: {
            'Content-Type': 'text/x-shellscript; charset=utf-8',
            'Content-Disposition': 'attachment; filename="pulse-setup-pbs.sh"',
          },
        }),
      );

      const result = await NodesAPI.downloadProxmoxSetupScript({
        type: 'pbs',
        host: 'pbs.example',
        url: 'https://pulse.example/base/api/setup-script?type=pbs',
        downloadURL: 'https://pulse.example/base/api/setup-script?type=pbs&setup_token=pbs-token-123',
        scriptFileName: 'pulse-setup-pbs.sh',
        command: 'curl pbs ...',
        commandWithEnv: 'curl env pbs ...',
        commandWithoutEnv: 'curl bare pbs ...',
        expires: 1_900_000_000,
        tokenHint: 'pbs…123',
      });

      expect(apiFetch).toHaveBeenCalledWith(
        'https://pulse.example/base/api/setup-script?type=pbs&setup_token=pbs-token-123',
      );
      expect(result).toEqual({
        content: '#!/bin/bash\necho pbs',
        contentType: 'text/x-shellscript; charset=utf-8',
        fileName: 'pulse-setup-pbs.sh',
      });
    });

    it('rejects malformed canonical setup-script download responses', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response('#!/bin/bash\necho bad', {
          status: 200,
          headers: {
            'Content-Type': 'text/plain; charset=utf-8',
            'Content-Disposition': 'attachment; filename="pulse-setup-pve.sh"',
          },
        }),
      );

      await expect(
        NodesAPI.downloadProxmoxSetupScript({
          type: 'pve',
          host: 'https://pve.example:8006',
          url: 'https://pulse.example/base/api/setup-script?type=pve',
          downloadURL: 'https://pulse.example/base/api/setup-script?type=pve&setup_token=setup-token-123',
          scriptFileName: 'pulse-setup-pve.sh',
          command: 'curl pve ...',
          commandWithEnv: 'curl env pve ...',
          commandWithoutEnv: 'curl bare pve ...',
          expires: 1_900_000_000,
          tokenHint: 'set…123',
        }),
      ).rejects.toThrow('Invalid Proxmox setup script content type');

      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response('#!/bin/bash\necho bad', {
          status: 200,
          headers: {
            'Content-Type': 'text/x-shellscript; charset=utf-8',
          },
        }),
      );

      await expect(
        NodesAPI.downloadProxmoxSetupScript({
          type: 'pve',
          host: 'https://pve.example:8006',
          url: 'https://pulse.example/base/api/setup-script?type=pve',
          downloadURL: 'https://pulse.example/base/api/setup-script?type=pve&setup_token=setup-token-123',
          scriptFileName: 'pulse-setup-pve.sh',
          command: 'curl pve ...',
          commandWithEnv: 'curl env pve ...',
          commandWithoutEnv: 'curl bare pve ...',
          expires: 1_900_000_000,
          tokenHint: 'set…123',
        }),
      ).rejects.toThrow('Invalid Proxmox setup script filename');
    });

    it('rejects setup-script downloads whose filename drifts from bootstrap metadata', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response('#!/bin/bash\necho bad', {
          status: 200,
          headers: {
            'Content-Type': 'text/x-shellscript; charset=utf-8',
            'Content-Disposition': 'attachment; filename="pulse-setup-pbs.sh"',
          },
        }),
      );

      await expect(
        NodesAPI.downloadProxmoxSetupScript({
          type: 'pve',
          host: 'https://pve.example:8006',
          url: 'https://pulse.example/base/api/setup-script?type=pve',
          downloadURL: 'https://pulse.example/base/api/setup-script?type=pve&setup_token=setup-token-123',
          scriptFileName: 'pulse-setup-pve.sh',
          command: 'curl pve ...',
          commandWithEnv: 'curl env pve ...',
          commandWithoutEnv: 'curl bare pve ...',
          expires: 1_900_000_000,
          tokenHint: 'set…123',
        }),
      ).rejects.toThrow('Invalid Proxmox setup script filename');
    });
  });
});
