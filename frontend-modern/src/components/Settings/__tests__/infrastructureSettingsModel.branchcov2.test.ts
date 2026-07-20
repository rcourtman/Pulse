/**
 * Branch-coverage tests for the still-uncovered named helpers in
 * infrastructureSettingsModel:
 *   normalizeInfrastructureHost, collectRepresentedDiscoveryHosts,
 *   filterRepresentedDiscoveredServers, addClusterEndpointHosts,
 *   matchConfiguredNodeToResource.
 *
 * `addClusterEndpointHosts` is module-private, so its branches are driven
 * indirectly through the exported `collectRepresentedDiscoveryHosts` (the only
 * call site) by passing `pve` nodes that carry `clusterEndpoints`.
 *
 * Every if/else arm, optional-chain, `??`, regex branch and early-return is
 * exercised with concrete inputs and asserted against the exact emitted shape
 * (no truthiness-only checks).
 */
import { describe, expect, it } from 'vitest';

import type { ClusterEndpoint, NodeConfigWithStatus } from '@/types/nodes';
import type { Resource } from '@/types/resource';

import {
  collectRepresentedDiscoveryHosts,
  filterRepresentedDiscoveredServers,
  matchConfiguredNodeToResource,
  normalizeInfrastructureHost,
} from '../infrastructureSettingsModel';

// ---- Fixtures ---------------------------------------------------------------

/**
 * Build a minimal `NodeConfigWithStatus`. The helpers under test only read
 * `id`, `name`, `type`, `host`, `guestURL` and (optionally) `clusterEndpoints`,
 * so a partial object cast through `unknown` is sufficient and keeps the tests
 * focused on the branches rather than full node construction.
 */
const makeNode = (overrides: Record<string, unknown>): NodeConfigWithStatus =>
  ({
    id: 'node-id',
    name: 'node-name',
    type: 'pve',
    host: undefined,
    guestURL: undefined,
    ...overrides,
  }) as unknown as NodeConfigWithStatus;

/** Minimal `Resource` carrying only the identity fields the matcher reads. */
const makeResource = (id: string, name: string): Resource => ({ id, name }) as unknown as Resource;

/** A fully-populated `ClusterEndpoint` with overridable fields. */
const makeEndpoint = (overrides: Partial<ClusterEndpoint> = {}): ClusterEndpoint => ({
  nodeId: 'node-1',
  nodeName: 'node-1',
  host: 'host.local',
  ip: '127.0.0.1',
  online: true,
  lastSeen: '2026-01-01T00:00:00Z',
  ...overrides,
});

// ---- normalizeInfrastructureHost -------------------------------------------

describe('normalizeInfrastructureHost', () => {
  it('returns null for undefined input (optional-chain + empty-trimmed arm)', () => {
    expect(normalizeInfrastructureHost(undefined)).toBeNull();
  });

  it('returns null for null input', () => {
    expect(normalizeInfrastructureHost(null)).toBeNull();
  });

  it('returns null for an empty string', () => {
    expect(normalizeInfrastructureHost('')).toBeNull();
  });

  it('returns null for a whitespace-only string (trim produces empty)', () => {
    expect(normalizeInfrastructureHost('   ')).toBeNull();
  });

  it('returns null for a bare scheme with no host ("http://")', () => {
    // URL constructor throws; after stripping the scheme, withoutPath is empty.
    expect(normalizeInfrastructureHost('http://')).toBeNull();
  });

  it('lowercases + returns the parsed hostname of a trimmed https URL, dropping port/path', () => {
    expect(normalizeInfrastructureHost('  https://Example.COM:8006/Path  ')).toBe('example.com');
  });

  it('returns the parsed hostname for an http URL with an IPv4 host', () => {
    expect(normalizeInfrastructureHost('http://10.0.0.5:8006')).toBe('10.0.0.5');
  });

  it('falls through to string heuristics when a URL parses but has no hostname', () => {
    // "foo:bar" is a valid non-special-scheme URL with an empty hostname, so
    // the `parsed.hostname` arm is skipped; the heuristic then returns the
    // whole string lowercased (no "://" to strip, no port to drop).
    expect(normalizeInfrastructureHost('foo:bar')).toBe('foo:bar');
  });

  it('extracts a bare IPv4 address that is not a valid URL', () => {
    expect(normalizeInfrastructureHost('192.168.1.1')).toBe('192.168.1.1');
  });

  it('strips a numeric port from a bare host via the host:port heuristic', () => {
    expect(normalizeInfrastructureHost('192.168.1.1:8006')).toBe('192.168.1.1');
  });

  it('extracts a bracketed IPv6 address with a trailing port', () => {
    expect(normalizeInfrastructureHost('[::1]:8006')).toBe('::1');
  });

  it('lowercases a bracketed IPv6 address with no port', () => {
    expect(normalizeInfrastructureHost('[FE80::1]')).toBe('fe80::1');
  });

  it('keeps an unbracketed IPv6 + port whole (hostPortMatch arm rejected when capture contains ":")', () => {
    // The hostPortMatch regex matches, but its `[1]` still contains a colon,
    // so the guard skips it and the function falls through to returning the
    // full string lowercased. (See GLM_REPORT.md — suspected source bug.)
    expect(normalizeInfrastructureHost('::1:8006')).toBe('::1:8006');
  });

  it('strips a trailing path from a bare host', () => {
    expect(normalizeInfrastructureHost('MyHost/path')).toBe('myhost');
  });

  it('lowercases a plain hostname with no scheme, port or path', () => {
    expect(normalizeInfrastructureHost('MyHost')).toBe('myhost');
  });
});

// ---- collectRepresentedDiscoveryHosts (+ private addClusterEndpointHosts) ---

describe('collectRepresentedDiscoveryHosts', () => {
  it('returns three empty sets when given no nodes and no rows', () => {
    const result = collectRepresentedDiscoveryHosts([], []);
    expect(result).toStrictEqual({
      pve: new Set<string>(),
      pbs: new Set<string>(),
      pmg: new Set<string>(),
    });
  });

  it('defaults the rows argument to empty (only nodes contribute)', () => {
    const result = collectRepresentedDiscoveryHosts([
      makeNode({ type: 'pbs', name: 'backupbox', host: 'https://backupbox:8007' }),
    ]);
    expect(result.pbs).toStrictEqual(new Set(['backupbox']));
    expect(result.pve).toStrictEqual(new Set());
    expect(result.pmg).toStrictEqual(new Set());
  });

  it('buckets each platform type into its own set, normalizing name + host + guestURL', () => {
    const result = collectRepresentedDiscoveryHosts([
      makeNode({
        type: 'pve',
        name: 'pve-name',
        host: 'https://10.0.0.1:8006',
        guestURL: 'https://10.0.0.1:8006',
      }),
      makeNode({ type: 'pbs', name: 'pbs-name', host: '10.0.0.2:8007' }),
      makeNode({ type: 'pmg', name: 'pmg-name', host: 'pmg-host' }),
    ]);
    expect(result.pve).toStrictEqual(new Set(['pve-name', '10.0.0.1']));
    expect(result.pbs).toStrictEqual(new Set(['pbs-name', '10.0.0.2']));
    expect(result.pmg).toStrictEqual(new Set(['pmg-name', 'pmg-host']));
  });

  it('does not invoke cluster aggregation for a pve node missing the clusterEndpoints key', () => {
    // `'clusterEndpoints' in node` is false → addClusterEndpointHosts is never
    // called; only name/host/guestURL contribute.
    const result = collectRepresentedDiscoveryHosts([
      makeNode({ type: 'pve', name: 'standalone', host: 'https://10.0.0.50:8006' }),
    ]);
    // name 'standalone' + normalized host '10.0.0.50'; no cluster contribution.
    expect(result.pve).toStrictEqual(new Set(['standalone', '10.0.0.50']));
  });

  it('no-ops cluster aggregation when clusterEndpoints is explicitly undefined (addClusterEndpointHosts undefined arm)', () => {
    const result = collectRepresentedDiscoveryHosts([
      makeNode({
        type: 'pve',
        name: 'cluster',
        host: 'https://primary:8006',
        clusterEndpoints: undefined,
      }),
    ]);
    // `'clusterEndpoints' in node` is true, but `endpoints?.forEach` is a no-op.
    expect(result.pve).toStrictEqual(new Set(['cluster', 'primary']));
  });

  it('no-ops cluster aggregation when clusterEndpoints is an empty array', () => {
    const result = collectRepresentedDiscoveryHosts([
      makeNode({
        type: 'pve',
        name: 'cluster',
        host: 'https://primary:8006',
        clusterEndpoints: [],
      }),
    ]);
    expect(result.pve).toStrictEqual(new Set(['cluster', 'primary']));
  });

  it('aggregates every cluster-endpoint host alias into the pve bucket (addClusterEndpointHosts populated arm)', () => {
    const result = collectRepresentedDiscoveryHosts([
      makeNode({
        type: 'pve',
        name: 'cluster',
        host: 'https://primary:8006',
        guestURL: 'https://primary:8006',
        clusterEndpoints: [
          makeEndpoint({
            nodeId: 'id-alpha',
            nodeName: 'alpha',
            host: 'alpha.local',
            ip: '10.0.0.11',
            guestURL: 'https://alpha:8006',
          }),
          makeEndpoint({
            nodeName: 'beta',
            host: 'beta.local',
            ip: '10.0.0.12',
          }),
        ],
      }),
    ]);

    // Node-level: name + normalized host/guestURL host.
    // Endpoint alpha: ip, host, guestURL-host, nodeName, nodeId.
    // Endpoint beta: ip, host, nodeName, default nodeId ('node-1'); guestURL
    //   is undefined and is dropped by normalize→null.
    expect(result.pve).toStrictEqual(
      new Set([
        'cluster',
        'primary',
        '10.0.0.11',
        'alpha.local',
        'alpha',
        'id-alpha',
        '10.0.0.12',
        'beta.local',
        'beta',
        'node-1',
      ]),
    );
    expect(result.pbs).toStrictEqual(new Set());
    expect(result.pmg).toStrictEqual(new Set());
  });

  it('keeps cluster endpoints out of non-pve node types (short-circuit of the pve guard)', () => {
    // A pbs node cannot carry clusterEndpoints in its type, but even if a
    // malformed input does, the `node.type === 'pve' &&` guard skips it.
    const result = collectRepresentedDiscoveryHosts([
      makeNode({
        type: 'pbs',
        name: 'backup',
        host: 'https://backup:8007',
        clusterEndpoints: [makeEndpoint({ ip: '10.0.0.99', nodeName: 'should-not-appear' })],
      }),
    ]);
    expect(result.pbs).toStrictEqual(new Set(['backup']));
    expect(result.pve).toStrictEqual(new Set());
  });
});

// ---- filterRepresentedDiscoveredServers ------------------------------------

describe('filterRepresentedDiscoveredServers', () => {
  it('removes a server whose normalized IP is represented by a configured node (IP arm)', () => {
    const nodes = [makeNode({ type: 'pve', name: 'pve-node', host: 'https://10.0.0.5:8006' })];
    const filtered = filterRepresentedDiscoveredServers(
      [
        { ip: '10.0.0.5', port: 8006, type: 'pve', version: '8.2.2' },
        { ip: '10.0.0.99', port: 8006, type: 'pve', version: '8.2.2' },
      ],
      nodes,
    );
    expect(filtered).toEqual([{ ip: '10.0.0.99', port: 8006, type: 'pve', version: '8.2.2' }]);
  });

  it('removes a server whose hostname is represented even when its IP is not (hostname arm)', () => {
    const nodes = [makeNode({ type: 'pve', name: 'proxmox', host: 'https://proxmox:8006' })];
    const filtered = filterRepresentedDiscoveredServers(
      [{ ip: '10.0.0.5', port: 8006, type: 'pve', version: '8.2.2', hostname: 'proxmox' }],
      nodes,
    );
    expect(filtered).toEqual([]);
  });

  it('keeps a server whose IP and hostname are both unrepresented', () => {
    const nodes = [makeNode({ type: 'pve', name: 'pve-node', host: 'https://10.0.0.5:8006' })];
    const filtered = filterRepresentedDiscoveredServers(
      [{ ip: '10.0.0.99', port: 8006, type: 'pve', version: '8.2.2', hostname: 'other' }],
      nodes,
    );
    expect(filtered).toEqual([
      { ip: '10.0.0.99', port: 8006, type: 'pve', version: '8.2.2', hostname: 'other' },
    ]);
  });

  it('keeps a server with no hostname when its IP is unrepresented (normalizedHostname falsy arm)', () => {
    const nodes = [makeNode({ type: 'pve', name: 'pve-node', host: 'https://10.0.0.5:8006' })];
    const filtered = filterRepresentedDiscoveredServers(
      [{ ip: '10.0.0.99', port: 8006, type: 'pve', version: '8.2.2' }],
      nodes,
    );
    expect(filtered).toEqual([{ ip: '10.0.0.99', port: 8006, type: 'pve', version: '8.2.2' }]);
  });

  it('keeps a server whose IP normalizes to null when nothing represents its hostname (normalizedIP falsy arm)', () => {
    const nodes = [makeNode({ type: 'pve', name: 'pve-node', host: 'https://10.0.0.5:8006' })];
    const filtered = filterRepresentedDiscoveredServers(
      [{ ip: '   ', port: 8006, type: 'pve', version: '8.2.2', hostname: 'unrepresented' }],
      nodes,
    );
    expect(filtered).toEqual([
      { ip: '   ', port: 8006, type: 'pve', version: '8.2.2', hostname: 'unrepresented' },
    ]);
  });

  it('only matches within the same platform type (pve host does not suppress a pbs server)', () => {
    const nodes = [makeNode({ type: 'pve', name: 'pve-node', host: 'https://10.0.0.5:8006' })];
    const filtered = filterRepresentedDiscoveredServers(
      [{ ip: '10.0.0.5', port: 8007, type: 'pbs', version: '3.0.1' }],
      nodes,
    );
    expect(filtered).toEqual([{ ip: '10.0.0.5', port: 8007, type: 'pbs', version: '3.0.1' }]);
  });
});

// ---- matchConfiguredNodeToResource -----------------------------------------

describe('matchConfiguredNodeToResource', () => {
  it('returns undefined when nodeResources is undefined', () => {
    const result = matchConfiguredNodeToResource(makeNode({ id: 'a', name: 'a' }), undefined);
    expect(result).toBeUndefined();
  });

  it('returns undefined when nodeResources is an empty array', () => {
    const result = matchConfiguredNodeToResource(makeNode({ id: 'a', name: 'a' }), []);
    expect(result).toBeUndefined();
  });

  it('matches when resource.id === configNode.id', () => {
    const matched = makeResource('shared-id', 'different-name');
    const result = matchConfiguredNodeToResource(
      makeNode({ id: 'shared-id', name: 'config-name' }),
      [makeResource('other', 'other'), matched],
    );
    expect(result).toBe(matched);
  });

  it('matches when resource.name === configNode.name (ids differ)', () => {
    const matched = makeResource('r1', 'shared-name');
    const result = matchConfiguredNodeToResource(makeNode({ id: 'c1', name: 'shared-name' }), [
      matched,
      makeResource('r2', 'other'),
    ]);
    expect(result).toBe(matched);
  });

  it('matches when the ".lan"-stripped name bases are equal', () => {
    // configNode.name 'host.lan' and resource.name 'host' differ literally and
    // by id, but both strip to 'host'.
    const matched = makeResource('r1', 'host');
    const result = matchConfiguredNodeToResource(makeNode({ id: 'c1', name: 'host.lan' }), [
      matched,
    ]);
    expect(result).toBe(matched);
  });

  it('matches when resource.id contains configNode.name', () => {
    // id 'prefix-sub-suffix' includes 'sub'; names differ; bases differ.
    const matched = makeResource('prefix-sub-suffix', 'unrelated');
    const result = matchConfiguredNodeToResource(makeNode({ id: 'c1', name: 'sub' }), [matched]);
    expect(result).toBe(matched);
  });

  it('matches when configNode.name contains resource.name', () => {
    // 'bigname'.includes('big'); id and name and bases all differ.
    const matched = makeResource('r1', 'big');
    const result = matchConfiguredNodeToResource(makeNode({ id: 'c1', name: 'bigname' }), [
      matched,
    ]);
    expect(result).toBe(matched);
  });

  it('returns undefined when no predicate matches', () => {
    const result = matchConfiguredNodeToResource(makeNode({ id: 'c1', name: 'zzz' }), [
      makeResource('r1', 'yyy'),
    ]);
    expect(result).toBeUndefined();
  });
});
