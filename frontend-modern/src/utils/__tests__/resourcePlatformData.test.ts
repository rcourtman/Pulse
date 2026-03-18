import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getProxmoxData,
  getAgentData,
  getLinkedAgentId,
  type ProxmoxPlatformData,
  type AgentPlatformData,
} from '../resourcePlatformData';

const createMockResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'test-1',
  type: 'vm',
  name: 'test-vm',
  displayName: 'Test VM',
  platformId: 'pve1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.now(),
  cpu: { current: 0 },
  memory: { current: 0, total: 0, used: 0 },
  disk: { current: 0, total: 0, used: 0 },
  ...overrides,
});

describe('resourcePlatformData', () => {
  describe('getProxmoxData', () => {
    it('returns undefined when no platformData', () => {
      const resource = createMockResource();
      expect(getProxmoxData(resource)).toBeUndefined();
    });

    it('returns undefined when platformData is empty', () => {
      const resource = createMockResource({ platformData: {} });
      expect(getProxmoxData(resource)).toBeUndefined();
    });

    it('extracts proxmox data from platformData', () => {
      const proxmoxData: ProxmoxData = {
        proxmox: {
          nodeName: 'pve-node-1',
          clusterName: 'my-cluster',
          instance: '100',
          uptime: 86400,
          temperature: 45,
          pveVersion: '8.0',
          cpuInfo: { cores: 8, model: 'Intel', sockets: 1 },
        },
      };
      const resource = createMockResource({ platformData: proxmoxData as any });

      const result = getProxmoxData(resource);
      expect(result).toEqual({
        nodeName: 'pve-node-1',
        clusterName: 'my-cluster',
        instance: '100',
        uptime: 86400,
        temperature: 45,
        pveVersion: '8.0',
        cpuInfo: { cores: 8, model: 'Intel', sockets: 1 },
      });
    });

    it('returns undefined when proxmox key does not exist', () => {
      const resource = createMockResource({ platformData: { docker: {} } as any });
      expect(getProxmoxData(resource)).toBeUndefined();
    });
  });

  describe('getAgentData', () => {
    it('returns undefined when no platformData', () => {
      const resource = createMockResource();
      expect(getAgentData(resource)).toBeUndefined();
    });

    it('extracts agent data from platformData', () => {
      const platformData = {
        agent: {
          agentId: 'agent-123',
          hostname: 'my-host',
          uptimeSeconds: 3600,
          temperature: 50,
        },
      };
      const resource = createMockResource({ platformData: platformData as any });

      const result = getAgentData(resource);
      expect(result).toEqual({
        agentId: 'agent-123',
        hostname: 'my-host',
        uptimeSeconds: 3600,
        temperature: 50,
      });
    });

    it('returns undefined when agent key does not exist', () => {
      const resource = createMockResource({ platformData: { proxmox: {} } as any });
      expect(getAgentData(resource)).toBeUndefined();
    });
  });

  describe('getLinkedAgentId', () => {
    it('returns undefined when no agent data', () => {
      const resource = createMockResource();
      expect(getLinkedAgentId(resource)).toBeUndefined();
    });

    it('returns agentId from agent data', () => {
      const platformData = {
        agent: { agentId: 'agent-456' },
      };
      const resource = createMockResource({ platformData: platformData as any });

      expect(getLinkedAgentId(resource)).toBe('agent-456');
    });

    it('returns undefined when agentId is not present', () => {
      const platformData = {
        agent: { hostname: 'test' },
      };
      const resource = createMockResource({ platformData: platformData as any });

      expect(getLinkedAgentId(resource)).toBeUndefined();
    });
  });
});

type ProxmoxData = {
  proxmox?: ProxmoxPlatformData;
  agent?: AgentPlatformData;
};
