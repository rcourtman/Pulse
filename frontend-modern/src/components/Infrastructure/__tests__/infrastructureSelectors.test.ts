import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { IODistributionStats } from '@/components/Infrastructure/infrastructureSelectors';
import {
  buildStatusOptions,
  collectAvailableSources,
  collectAvailableStatuses,
  computeIOScale,
  filterResources,
  getOutlierEmphasis,
  getResourceSources,
  groupResources,
  matchesSearch,
  sortResources,
  splitHostAndServiceResources,
  tokenizeSearch,
} from '@/components/Infrastructure/infrastructureSelectors';

const makeResource = (i: number, overrides: Partial<Resource> = {}): Resource => ({
  id: `resource-${i}`,
  type: 'host',
  name: `host-${i}`,
  displayName: `Host ${i}`,
  platformId: `platform-${i}`,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000 + i,
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

describe('infrastructureSelectors', () => {
  describe('tokenizeSearch', () => {
    it('handles empty and whitespace-only queries', () => {
      expect(tokenizeSearch('')).toEqual([]);
      expect(tokenizeSearch('   ')).toEqual([]);
    });

    it('tokenizes and normalizes single and multi-word input', () => {
      expect(tokenizeSearch('Host')).toEqual(['host']);
      expect(tokenizeSearch('  Alpha   Beta   GAMMA ')).toEqual(['alpha', 'beta', 'gamma']);
    });
  });

  describe('matchesSearch', () => {
    const resource = makeResource(1, {
      id: 'node-123',
      name: 'alpha-node',
      displayName: 'Alpha Node',
      identity: { hostname: 'alpha.local', ips: ['10.0.0.10', '192.168.1.10'] },
      tags: ['edge', 'database'],
    });

    it('matches by name/display/id', () => {
      expect(matchesSearch(resource, 'alpha')).toBe(true);
      expect(matchesSearch(resource, 'NODE-123')).toBe(true);
    });

    it('matches by ip and tag and returns false when missing', () => {
      expect(matchesSearch(resource, '192.168.1.10')).toBe(true);
      expect(matchesSearch(resource, 'database')).toBe(true);
      expect(matchesSearch(resource, 'not-found')).toBe(false);
    });
  });

  describe('getResourceSources and collectors', () => {
    it('normalizes and deduplicates resource sources', () => {
      const resource = makeResource(1, {
        platformData: { sources: ['PVE', 'proxmox-pve', 'k8s', 'kubernetes', 'invalid'] },
      });
      expect(getResourceSources(resource)).toEqual(['proxmox', 'kubernetes']);
    });

    it('collects available sources and statuses with deduplication', () => {
      const uppercaseStatus = makeResource(3, {
        status: 'ONLINE' as unknown as Resource['status'],
      });
      const resources = [
        makeResource(1, { status: 'online', platformData: { sources: ['proxmox', 'pve'] } }),
        makeResource(2, { status: 'offline', platformData: { sources: ['docker', 'docker'] } }),
        uppercaseStatus,
      ];

      expect(collectAvailableSources(resources)).toEqual(new Set(['proxmox', 'docker']));
      expect(collectAvailableStatuses(resources)).toEqual(new Set(['online', 'offline']));
    });
  });

  describe('buildStatusOptions', () => {
    it('orders by predefined status order and labels known statuses', () => {
      const statuses = new Set(['running', 'foo', 'offline', 'online']);
      expect(buildStatusOptions(statuses)).toEqual([
        { key: 'online', label: 'Online' },
        { key: 'offline', label: 'Offline' },
        { key: 'running', label: 'Running' },
        { key: 'foo', label: 'foo' },
      ]);
    });
  });

  describe('filterResources', () => {
    const resources = [
      makeResource(1, {
        name: 'alpha-node',
        status: 'online',
        identity: { ips: ['10.0.0.1'] },
        platformData: { sources: ['proxmox'] },
      }),
      makeResource(2, {
        name: 'beta-node',
        status: 'offline',
        identity: { ips: ['10.0.0.2'] },
        platformData: { sources: ['docker'] },
      }),
      makeResource(3, {
        name: 'gamma-node',
        status: 'degraded',
        tags: ['prod', 'db'],
        platformData: { sources: ['k8s'] },
      }),
    ];

    it('returns original array when no filters are active', () => {
      const result = filterResources(resources, new Set(), new Set(), []);
      expect(result).toBe(resources);
    });

    it('filters by sources only', () => {
      const result = filterResources(resources, new Set(['docker']), new Set(), []);
      expect(result.map((resource) => resource.id)).toEqual(['resource-2']);
    });

    it('filters by statuses only', () => {
      const result = filterResources(resources, new Set(), new Set(['degraded']), []);
      expect(result.map((resource) => resource.id)).toEqual(['resource-3']);
    });

    it('filters by search terms only', () => {
      const result = filterResources(resources, new Set(), new Set(), ['10.0.0.2']);
      expect(result.map((resource) => resource.id)).toEqual(['resource-2']);
    });

    it('applies combined filters and requires all terms to match', () => {
      const result = filterResources(resources, new Set(['kubernetes']), new Set(['degraded']), [
        'gamma',
        'db',
      ]);
      expect(result.map((resource) => resource.id)).toEqual(['resource-3']);
    });
  });

  describe('splitHostAndServiceResources', () => {
    it('splits pbs/pmg as services and everything else as hosts', () => {
      const resources = [
        makeResource(1, { type: 'host' }),
        makeResource(2, { type: 'node' }),
        makeResource(3, { type: 'pbs' }),
        makeResource(4, { type: 'pmg' }),
      ];
      const { hosts, services } = splitHostAndServiceResources(resources);
      expect(hosts.map((resource) => resource.id)).toEqual(['resource-1', 'resource-2']);
      expect(services.map((resource) => resource.id)).toEqual(['resource-3', 'resource-4']);
    });
  });

  describe('sortResources', () => {
    it('uses default sort: online first, then display name', () => {
      const resources = [
        makeResource(1, { status: 'offline', displayName: 'Alpha' }),
        makeResource(2, { status: 'online', displayName: 'Beta' }),
        makeResource(3, { status: 'online', displayName: 'Alpha' }),
      ];
      const sorted = sortResources(resources, 'default', 'asc');
      expect(sorted.map((resource) => resource.id)).toEqual([
        'resource-3',
        'resource-2',
        'resource-1',
      ]);
    });

    it('sorts by name in ascending order', () => {
      const resources = [
        makeResource(1, { displayName: 'Charlie' }),
        makeResource(2, { displayName: 'alpha' }),
        makeResource(3, { displayName: 'Bravo' }),
      ];
      const sorted = sortResources(resources, 'name', 'asc');
      expect(sorted.map((resource) => resource.id)).toEqual([
        'resource-2',
        'resource-3',
        'resource-1',
      ]);
    });

    it('sorts cpu with direction toggle and keeps default tie-breaker', () => {
      const resources = [
        makeResource(1, { displayName: 'Bravo', cpu: { current: 80 } }),
        makeResource(2, { displayName: 'Alpha', cpu: { current: 80 } }),
        makeResource(3, { displayName: 'Charlie', cpu: { current: 20 } }),
      ];

      const asc = sortResources(resources, 'cpu', 'asc');
      expect(asc.map((resource) => resource.id)).toEqual([
        'resource-3',
        'resource-2',
        'resource-1',
      ]);

      const desc = sortResources(resources, 'cpu', 'desc');
      expect(desc.map((resource) => resource.id)).toEqual([
        'resource-2',
        'resource-1',
        'resource-3',
      ]);
    });
  });

  describe('groupResources', () => {
    const sortedResources = [
      makeResource(1, { clusterId: 'zeta' }),
      makeResource(2, { clusterId: 'alpha' }),
      makeResource(3, { clusterId: '' }),
      makeResource(4, {}),
    ];

    it('returns single group in flat mode', () => {
      expect(groupResources(sortedResources, 'flat')).toEqual([
        { cluster: '', resources: sortedResources },
      ]);
    });

    it('groups by cluster and sorts clusters with empty cluster last', () => {
      expect(groupResources(sortedResources, 'grouped')).toEqual([
        { cluster: 'alpha', resources: [sortedResources[1]] },
        { cluster: 'zeta', resources: [sortedResources[0]] },
        { cluster: '', resources: [sortedResources[2], sortedResources[3]] },
      ]);
    });
  });

  describe('computeIOScale', () => {
    it('computes expected stats for network and disk io', () => {
      const resources = [
        makeResource(1, {
          network: { rxBytes: 1, txBytes: 1 },
          diskIO: { readRate: 1, writeRate: 1 },
        }),
        makeResource(2, {
          network: { rxBytes: 3, txBytes: 1 },
          diskIO: { readRate: 2, writeRate: 2 },
        }),
        makeResource(3, {}),
        makeResource(4, {
          network: { rxBytes: 10, txBytes: 0 },
          diskIO: { readRate: 8, writeRate: 0 },
        }),
      ];

      expect(computeIOScale(resources)).toEqual({
        network: { median: 4, mad: 2, max: 10, p97: 10, p99: 10, count: 3 },
        diskIO: { median: 4, mad: 2, max: 8, p97: 8, p99: 8, count: 3 },
      });
    });

    it('handles empty and single-resource inputs', () => {
      expect(computeIOScale([])).toEqual({
        network: { median: 0, mad: 0, max: 0, p97: 0, p99: 0, count: 0 },
        diskIO: { median: 0, mad: 0, max: 0, p97: 0, p99: 0, count: 0 },
      });

      expect(
        computeIOScale([
          makeResource(1, {
            network: { rxBytes: 7, txBytes: 3 },
            diskIO: { readRate: 11, writeRate: 9 },
          }),
        ]),
      ).toEqual({
        network: { median: 10, mad: 0, max: 10, p97: 10, p99: 10, count: 1 },
        diskIO: { median: 20, mad: 0, max: 20, p97: 20, p99: 20, count: 1 },
      });
    });
  });

  describe('getOutlierEmphasis', () => {
    const stats: IODistributionStats = {
      median: 10,
      mad: 2,
      max: 30,
      p97: 20,
      p99: 25,
      count: 10,
    };

    it('returns baseline styling below threshold', () => {
      expect(getOutlierEmphasis(12, stats)).toEqual({
        fontWeight: 'normal',
        color: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('returns medium emphasis for moderate outliers', () => {
      expect(getOutlierEmphasis(27, stats)).toEqual({
        fontWeight: '500',
        color: 'text-base-content',
        showOutlierHint: true,
      });
    });

    it('returns high emphasis for strong outliers', () => {
      expect(getOutlierEmphasis(30, stats)).toEqual({
        fontWeight: '600',
        color: 'text-base-content',
        showOutlierHint: true,
      });
    });
  });
});
