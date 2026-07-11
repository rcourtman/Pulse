import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import type { Override } from '../types';
import {
  agentDiskResourceId,
  buildNodeHeaderMeta,
  createOverridesMap,
  dockerHostOverrideIdCandidates,
  getFriendlyNodeName,
  hostActionId,
  storageCoords,
} from '../thresholdsResourceModel';

/**
 * Build a minimal Resource fixture. `as unknown as Resource` keeps strict
 * TypeScript clean while letting us exercise the runtime guards with
 * deliberately partial / malformed payloads.
 */
const makeNode = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'node-id',
    type: 'agent',
    name: 'node-name',
    displayName: 'node-display',
    platformId: 'platform-id',
    platformType: 'generic',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 0,
    ...overrides,
  }) as unknown as Resource;

const makeOverride = (id: string, thresholds: Override['thresholds'] = {}): Override =>
  ({
    id,
    name: id,
    type: 'guest',
    thresholds,
  }) as Override;

describe('thresholdsResourceModel branch coverage', () => {
  describe('getFriendlyNodeName', () => {
    it('returns empty string unchanged (guard arm)', () => {
      expect(getFriendlyNodeName('')).toBe('');
    });

    it('passes through a simple single-word name unchanged', () => {
      expect(getFriendlyNodeName('titan')).toBe('titan');
    });

    it('strips a trailing domain suffix to the first label', () => {
      expect(getFriendlyNodeName('titan.example.com')).toBe('titan');
    });

    it('prefers a divergent parenthetical alias over the base token', () => {
      expect(getFriendlyNodeName('titan (relay)')).toBe('relay');
    });

    it('returns the base token when the parenthetical normalizes to the same value', () => {
      expect(getFriendlyNodeName('titan (titan)')).toBe('titan');
    });

    it('strips a cluster-name token from a multi-word name when clusterName is supplied', () => {
      expect(getFriendlyNodeName('node1 prod-cluster', 'prod-cluster')).toBe('node1');
    });

    it('falls back to the trimmed raw value when the cluster filter empties the token', () => {
      // value is exactly the cluster name; after filtering nothing remains,
      // so the function falls back to `value.trim()`.
      expect(getFriendlyNodeName('prod-cluster', 'prod-cluster')).toBe('prod-cluster');
    });

    it('ignores a whitespace-only clusterName (no filtering applied)', () => {
      expect(getFriendlyNodeName('titan', '   ')).toBe('titan');
    });

    it('returns an empty string for a whitespace-only value', () => {
      expect(getFriendlyNodeName('   ')).toBe('');
    });
  });

  describe('buildNodeHeaderMeta', () => {
    it('uses guestURL as-is when it already starts with http', () => {
      const node = makeNode({
        platformData: { guestURL: 'http://titan.local:8006' },
      });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('http://titan.local:8006');
    });

    it('prepends https:// when guestURL has no scheme', () => {
      const node = makeNode({ platformData: { guestURL: 'titan.local' } });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('https://titan.local');
    });

    it('uses host verbatim when it starts with http and guestURL is absent', () => {
      const node = makeNode({ platformData: { host: 'https://titan.local' } });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('https://titan.local');
    });

    it('appends the default :8006 port when host lacks both scheme and port', () => {
      const node = makeNode({ platformData: { host: '192.0.2.10' } });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('https://192.0.2.10:8006');
    });

    it('keeps an explicit port on host without appending :8006', () => {
      const node = makeNode({ platformData: { host: '192.0.2.10:9000' } });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('https://192.0.2.10:9000');
    });

    it('falls back to node.name with default port when neither guestURL nor host is set', () => {
      const node = makeNode({ name: 'titan-node' });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('https://titan-node:8006');
    });

    it('keeps an explicit port on node.name without appending :8006', () => {
      const node = makeNode({ name: 'titan-node:9000' });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBe('https://titan-node:9000');
    });

    it('leaves host undefined when guestURL, host and name are all absent', () => {
      const node = makeNode({ name: '', displayName: 'titan' });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.host).toBeUndefined();
    });

    it('marks the header as a cluster member with the trimmed cluster name', () => {
      const node = makeNode({
        platformData: { isClusterMember: true, clusterName: '  prod-cluster  ' },
      });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.isClusterMember).toBe(true);
      expect(headerMeta.clusterName).toBe('prod-cluster');
    });

    it('falls back to the literal "Cluster" when isClusterMember is true but clusterName is empty', () => {
      const node = makeNode({
        platformData: { isClusterMember: true, clusterName: '   ' },
      });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.isClusterMember).toBe(true);
      expect(headerMeta.clusterName).toBe('Cluster');
    });

    it('derives isClusterMember from node.clusterId when platformData omits it', () => {
      const node = makeNode({ clusterId: 'cluster-7', platformData: {} });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.isClusterMember).toBe(true);
    });

    it('reports isClusterMember=false and clusterName=undefined when no cluster signal is present', () => {
      const node = makeNode({});
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.isClusterMember).toBe(false);
      expect(headerMeta.clusterName).toBeUndefined();
    });

    it('threads node.status through and tags the header type as node', () => {
      const node = makeNode({ status: 'warning' });
      const { headerMeta } = buildNodeHeaderMeta(node);
      expect(headerMeta.type).toBe('node');
      expect(headerMeta.status).toBe('warning');
    });

    it('collects distinct non-empty name/displayName/friendlyName tokens into the keys set', () => {
      // displayName "titan" -> friendlyName "titan"; name differs.
      const node = makeNode({ name: 'titan-host', displayName: 'titan' });
      const { keys } = buildNodeHeaderMeta(node);
      expect(keys.has('titan-host')).toBe(true);
      expect(keys.has('titan')).toBe(true);
      expect(keys.size).toBe(2);
    });

    it('collapses to a single key when name, display and friendly names agree', () => {
      const node = makeNode({ name: 'titan', displayName: 'titan' });
      const { keys } = buildNodeHeaderMeta(node);
      expect(keys.size).toBe(1);
      expect(keys.has('titan')).toBe(true);
    });
  });

  describe('dockerHostOverrideIdCandidates', () => {
    it('leads with discoveryTarget.resourceId when the discovery type is app-container', () => {
      const node = makeNode({
        id: 'runtime-1',
        discoveryTarget: {
          resourceType: 'app-container',
          resourceId: 'container-123',
          agentId: 'agent-7',
        },
        platformData: {
          docker: { hostSourceId: 'docker-platform' },
          hostSourceId: 'host-platform',
        },
      });
      expect(dockerHostOverrideIdCandidates(node)).toEqual([
        'container-123',
        'docker-platform',
        'host-platform',
        'agent-7',
        'runtime-1',
      ]);
    });

    it('omits discoveryTarget.resourceId when the discovery type is not app-container', () => {
      const node = makeNode({
        id: 'runtime-1',
        discoveryTarget: {
          resourceType: 'agent',
          resourceId: 'should-not-appear',
          agentId: 'agent-7',
        },
      });
      const result = dockerHostOverrideIdCandidates(node);
      expect(result).not.toContain('should-not-appear');
      expect(result).toEqual(['agent-7', 'runtime-1']);
    });

    it('returns just resource.id when no discovery target or platform data is present', () => {
      const node = makeNode({ id: 'lone-host' });
      expect(dockerHostOverrideIdCandidates(node)).toEqual(['lone-host']);
    });

    it('dedupes repeated hostSourceId values across docker record and top-level platform data', () => {
      const node = makeNode({
        id: 'host-1',
        platformData: {
          docker: { hostSourceId: 'shared' },
          hostSourceId: 'shared',
        },
      });
      const result = dockerHostOverrideIdCandidates(node);
      expect(result).toEqual(['shared', 'host-1']);
    });
  });

  describe('storageCoords', () => {
    it('uses pbsInstanceId / pbsInstanceName for datastore rows', () => {
      const storage = makeNode({
        type: 'datastore',
        platformData: { pbsInstanceId: 'pbs-1', pbsInstanceName: 'backup-server' },
      });
      expect(storageCoords(storage)).toEqual({ node: 'backup-server', instance: 'pbs-1' });
    });

    it('falls back to parentId then platformId then "pbs" for the datastore instance', () => {
      const fromParent = makeNode({ type: 'datastore', parentId: 'parent-1' });
      expect(storageCoords(fromParent).instance).toBe('parent-1');

      const fromPlatform = makeNode({
        type: 'datastore',
        platformId: 'platform-1',
      });
      expect(storageCoords(fromPlatform).instance).toBe('platform-1');

      const fallback = makeNode({ type: 'datastore', platformId: '', parentId: undefined });
      expect(storageCoords(fallback).instance).toBe('pbs');
      // node mirrors instance when pbsInstanceName is absent.
      expect(storageCoords(fallback).node).toBe('pbs');
    });

    it('reads node/instance from platformData for non-datastore rows', () => {
      const storage = makeNode({
        type: 'storage',
        platformData: { node: 'node-a', instance: 'instance-b' },
      });
      expect(storageCoords(storage)).toEqual({ node: 'node-a', instance: 'instance-b' });
    });

    it('returns empty node and platformId instance when platformData is absent on a non-datastore row', () => {
      const storage = makeNode({ type: 'storage', platformId: 'platform-9' });
      const coords = storageCoords(storage);
      expect(coords.node).toBe('');
      expect(coords.instance).toBe('platform-9');
    });

    it('returns empty strings for node and instance when nothing is available on a non-datastore row', () => {
      const storage = makeNode({ type: 'storage', platformId: '' });
      const coords = storageCoords(storage);
      expect(coords.node).toBe('');
      expect(coords.instance).toBe('');
    });
  });

  describe('agentDiskResourceId', () => {
    it('falls back to the literal "disk" label when mountpoint and device are both empty', () => {
      expect(agentDiskResourceId('agent-1', '', '')).toBe('agent:agent-1/disk:disk');
    });

    it('falls back to "disk" when mountpoint and device are whitespace-only', () => {
      expect(agentDiskResourceId('agent-1', '   ', '   ')).toBe('agent:agent-1/disk:disk');
    });

    it('collapses non-alphanumeric runs into single dashes and strips edges', () => {
      // Multiple slashes/spaces collapse; leading slash is stripped.
      expect(agentDiskResourceId('agent-1', '/var///lib  data', '')).toBe(
        'agent:agent-1/disk:var-lib-data',
      );
    });

    it('uses the "unknown" sentinel when the sanitized label is empty', () => {
      // punctuation-only input becomes all dashes -> stripped to empty -> "unknown".
      expect(agentDiskResourceId('agent-1', '!!!', '')).toBe('agent:agent-1/disk:unknown');
    });

    it('prefers a non-empty mountpoint over device', () => {
      expect(agentDiskResourceId('agent-1', '/mnt/data', '/dev/sda1')).toBe(
        'agent:agent-1/disk:mnt-data',
      );
    });
  });

  describe('createOverridesMap', () => {
    it('returns an empty Map for undefined input', () => {
      expect(createOverridesMap(undefined).size).toBe(0);
    });

    it('returns an empty Map for an empty array', () => {
      expect(createOverridesMap([]).size).toBe(0);
    });

    it('keys the map by override.id and preserves insertion order', () => {
      const a = makeOverride('a', { cpu: 80 });
      const b = makeOverride('b', { memory: 70 });
      const map = createOverridesMap([a, b]);
      expect([...map.keys()]).toEqual(['a', 'b']);
      expect(map.get('a')).toBe(a);
      expect(map.get('b')).toBe(b);
    });

    it('lets later duplicates shadow earlier entries with the same id (Map semantics)', () => {
      const first = makeOverride('a', { cpu: 1 });
      const second = makeOverride('a', { cpu: 2 });
      const map = createOverridesMap([first, second]);
      expect(map.size).toBe(1);
      expect(map.get('a')).toBe(second);
    });
  });

  describe('hostActionId', () => {
    it('returns the first host override candidate when candidates are non-empty', () => {
      const node = makeNode({
        id: 'runtime-1',
        discoveryTarget: {
          resourceType: 'agent',
          resourceId: 'agent-discovery',
          agentId: 'agent-7',
        },
      });
      // hostOverrideIdCandidates leads with getAgentDiscoveryResourceId -> "agent-discovery".
      expect(hostActionId(node)).toBe('agent-discovery');
    });

    it('falls back to resource.id when every candidate (including id) is invalid', () => {
      // All sources blank -> uniqueIds returns [] -> [0] is undefined -> fallback.
      const node = makeNode({ id: '' });
      expect(hostActionId(node)).toBe('');
    });

    it('falls back to resource.id when no discovery/agent/platform signals exist', () => {
      const node = makeNode({ id: 'lone-agent' });
      expect(hostActionId(node)).toBe('lone-agent');
    });
  });
});
