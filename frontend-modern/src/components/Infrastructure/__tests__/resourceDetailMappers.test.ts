import { describe, expect, it } from 'vitest';
import {
  normalizeHealthLabel,
  healthToneClass,
  formatInteger,
  formatSourceType,
  toDiscoveryConfig,
  toAgentFromResource,
  toNodeFromProxmox,
} from '@/components/Infrastructure/resourceDetailMappers';
import type { Resource } from '@/types/resource';

const createHybridHostResource = (): Resource =>
  ({
    id: 'resource:host:hash-1',
    type: 'agent',
    name: 'tower',
    displayName: 'Tower',
    platformId: 'tower',
    platformType: 'proxmox-pve',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 15 },
    memory: { total: 1024, used: 256, free: 768 },
    disk: { total: 2048, used: 512, free: 1536 },
    platformData: {
      proxmox: {
        nodeName: 'pve-node-1',
      },
      agent: {
        agentId: 'agent-canonical',
        agentVersion: '1.2.3',
        hostname: 'tower.local',
        osName: 'Unraid',
        kernelVersion: '6.1.0',
      },
    },
  }) as Resource;

describe('resourceDetailMappers', () => {
  describe('normalizeHealthLabel', () => {
    it('returns Unknown for empty string', () => {
      expect(normalizeHealthLabel('')).toBe('Unknown');
    });

    it('returns Unknown for undefined', () => {
      expect(normalizeHealthLabel(undefined)).toBe('Unknown');
    });

    it('returns uppercase for short values (3 chars or less)', () => {
      expect(normalizeHealthLabel('ok')).toBe('OK');
      expect(normalizeHealthLabel('up')).toBe('UP');
      expect(normalizeHealthLabel('io')).toBe('IO');
    });

    it('capitalizes first letter for longer values', () => {
      expect(normalizeHealthLabel('online')).toBe('Online');
      expect(normalizeHealthLabel('running')).toBe('Running');
    });

    it('trims whitespace', () => {
      expect(normalizeHealthLabel('  online  ')).toBe('Online');
    });
  });

  describe('healthToneClass', () => {
    it('returns emerald for online', () => {
      expect(healthToneClass('online')).toContain('emerald');
    });

    it('returns emerald for running', () => {
      expect(healthToneClass('running')).toContain('emerald');
    });

    it('returns emerald for healthy', () => {
      expect(healthToneClass('healthy')).toContain('emerald');
    });

    it('returns emerald for connected', () => {
      expect(healthToneClass('connected')).toContain('emerald');
    });

    it('returns emerald for ok', () => {
      expect(healthToneClass('ok')).toContain('emerald');
    });

    it('returns amber for degraded', () => {
      expect(healthToneClass('degraded')).toContain('amber');
    });

    it('returns amber for warning', () => {
      expect(healthToneClass('warning')).toContain('amber');
    });

    it('returns amber for stale', () => {
      expect(healthToneClass('stale')).toContain('amber');
    });

    it('returns red for offline', () => {
      expect(healthToneClass('offline')).toContain('red');
    });

    it('returns red for down', () => {
      expect(healthToneClass('down')).toContain('red');
    });

    it('returns red for disconnected', () => {
      expect(healthToneClass('disconnected')).toContain('red');
    });

    it('returns red for error', () => {
      expect(healthToneClass('error')).toContain('red');
    });

    it('returns red for failed', () => {
      expect(healthToneClass('failed')).toContain('red');
    });

    it('returns base content for unknown', () => {
      expect(healthToneClass('unknown')).toContain('base-content');
    });

    it('is case insensitive', () => {
      expect(healthToneClass('ONLINE')).toContain('emerald');
      expect(healthToneClass('Error')).toContain('red');
    });
  });

  describe('formatInteger', () => {
    it('returns dash for undefined', () => {
      expect(formatInteger(undefined)).toBe('—');
    });

    it('returns dash for null', () => {
      expect(formatInteger(undefined)).toBe('—');
    });

    it('returns dash for NaN', () => {
      expect(formatInteger(NaN)).toBe('—');
    });

    it('formats integer with commas', () => {
      expect(formatInteger(1000)).toBe('1,000');
      expect(formatInteger(1000000)).toBe('1,000,000');
    });

    it('rounds decimal values', () => {
      expect(formatInteger(1000.7)).toBe('1,001');
      expect(formatInteger(1000.3)).toBe('1,000');
    });

    it('handles zero', () => {
      expect(formatInteger(0)).toBe('0');
    });

    it('handles negative numbers', () => {
      expect(formatInteger(-1000)).toBe('-1,000');
    });
  });

  describe('formatSourceType', () => {
    it('returns Hybrid for hybrid', () => {
      expect(formatSourceType('hybrid')).toBe('Hybrid');
    });

    it('returns Agent for agent', () => {
      expect(formatSourceType('agent')).toBe('Agent');
    });

    it('returns API for api', () => {
      expect(formatSourceType('api')).toBe('API');
    });

    it('returns unknown source type as-is', () => {
      expect(formatSourceType('unknown-source' as any)).toBe('unknown-source');
    });
  });

  describe('toNodeFromProxmox', () => {
    it('preserves canonical linkedAgentId for hybrid hosts', () => {
      const node = toNodeFromProxmox(createHybridHostResource());

      expect(node?.linkedAgentId).toBe('agent-canonical');
    });
  });

  describe('toAgentFromResource', () => {
    it('uses the canonical actionable agent id instead of the hashed resource id', () => {
      const agent = toAgentFromResource(createHybridHostResource());

      expect(agent?.id).toBe('agent-canonical');
      expect(agent?.id).not.toBe('resource:host:hash-1');
    });
  });

  describe('toDiscoveryConfig', () => {
    it('prefers the typed canonical hostname for explicit discovery targets', () => {
      const config = toDiscoveryConfig({
        ...createHybridHostResource(),
        displayName: '',
        canonicalIdentity: {
          hostname: 'tower.canonical',
          displayName: 'Tower',
          primaryId: 'node:instance-pve1',
        },
        discoveryTarget: {
          resourceType: 'agent',
          agentId: 'agent-canonical',
          resourceId: 'agent-canonical',
        },
      });

      expect(config?.hostname).toBe('tower.canonical');
    });
  });
});
