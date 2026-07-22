import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionState,
  type ConnectionSystem,
} from '@/api/connections';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { connectionToRow, useConnectionsLedger } from '../useConnectionsLedger';

// ---- Fixtures ---------------------------------------------------------------
// Minimal valid factories mirroring the sibling suite. connectionToRow is a
// pure (non-reactive) export, so its cases call it directly; the hook-driven
// cases (findById, systems, error, cache) mirror useConnectionsLedger.test.ts.

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:node-1',
  type: 'pve',
  name: 'node-1',
  address: 'https://node-1:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms', 'containers'],
  scope: { vms: true, containers: true },
  lastSeen: '2026-07-22T12:00:00Z',
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const PINNED_NOW = new Date('2026-07-22T12:00:00.000Z');

// Reset the retained non-suspending query cache around every hook-driven case
// so each render starts from the empty initial snapshot, exactly as the
// sibling suite does.
const useLedgerHarness = () => {
  beforeEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
  });
  afterEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
    vi.restoreAllMocks();
  });
};

const renderLedger = (connections: Connection[], systems: ConnectionSystem[]) => {
  vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems });
  return renderHook(() => useConnectionsLedger());
};

// ---- findById ---------------------------------------------------------------
// findById had ZERO hits in the measured coverage. Exercise every arm of the
// `connections().find((conn) => conn.id === id)` lookup through the live hook.

describe('useConnectionsLedger.findById', () => {
  useLedgerHarness();

  it('returns the matching connection when the id exists', async () => {
    const alpha = connectionFixture({ id: 'agent:alpha', name: 'alpha' });
    const { result } = renderLedger([alpha], []);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.findById('agent:alpha')?.name).toBe('alpha');
  });

  it('returns undefined when the id does not match any connection', async () => {
    const alpha = connectionFixture({ id: 'agent:alpha', name: 'alpha' });
    const { result } = renderLedger([alpha], []);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.findById('agent:missing')).toBeUndefined();
  });

  it('returns undefined for an empty id', async () => {
    const alpha = connectionFixture({ id: 'agent:alpha', name: 'alpha' });
    const { result } = renderLedger([alpha], []);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.findById('')).toBeUndefined();
  });

  it('returns undefined for a whitespace-only id', async () => {
    const alpha = connectionFixture({ id: 'agent:alpha', name: 'alpha' });
    const { result } = renderLedger([alpha], []);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.findById('   ')).toBeUndefined();
  });

  it('returns the first match when two connections share an id', async () => {
    // Array.prototype.find is first-match-wins; a duplicate id must not silently
    // surface the newer record.
    const first = connectionFixture({ id: 'agent:dup', name: 'first', address: 'first' });
    const second = connectionFixture({ id: 'agent:dup', name: 'second', address: 'second' });
    const { result } = renderLedger([first, second], []);

    await waitFor(() => expect(result.rows()).toHaveLength(2));
    expect(result.findById('agent:dup')?.name).toBe('first');
  });

  it('returns undefined when the ledger is empty', async () => {
    const { result } = renderLedger([], []);

    await waitFor(() => expect(result.rows()).toHaveLength(0));
    expect(result.findById('agent:alpha')).toBeUndefined();
    expect(result.connections()).toEqual([]);
  });
});

// ---- connectionToRow (standalone buildRow arms) -----------------------------
// connectionToRow is exported and pure, so these exercise buildRow directly
// without the async query machinery. Each case targets an arm the sibling
// suite (which renders agents, truenas, pve clusters and vmware) leaves cold.

describe('connectionToRow (standalone buildRow arms)', () => {
  // --- name fallback chain (name || address || id) ---
  it('falls back to the address when the name is blank', () => {
    const row = connectionToRow(connectionFixture({ name: '', address: 'https://pve-blank:8006' }));
    expect(row.name).toBe('https://pve-blank:8006');
  });

  it('falls back to the id when both name and address are blank', () => {
    const row = connectionToRow(connectionFixture({ id: 'pve:id-only', name: '', address: '' }));
    expect(row.name).toBe('pve:id-only');
  });

  // --- host (non-agent, non-cluster) ---
  it('omits host when the address equals the name for a non-agent row', () => {
    const row = connectionToRow(connectionFixture({ name: 'node-1', address: 'node-1' }));
    expect(row.host).toBeUndefined();
  });

  it('exposes the address as host when it differs from the name', () => {
    const row = connectionToRow(
      connectionFixture({ name: 'node-1', address: 'https://node-1:8006' }),
    );
    expect(row.host).toBe('https://node-1:8006');
  });

  // --- sourceFor arms ---
  it('reports source "probe" for a contributing availability target', () => {
    const row = connectionToRow(
      connectionFixture({
        id: 'availability:probe-1',
        type: 'availability',
        name: 'probe-1',
        address: 'probe-1',
        surfaces: ['availability'],
        scope: { availability: true },
      }),
    );
    expect(row.source).toBe('probe');
    expect(row.subtitle).toBe('via availability probe');
  });

  it('falls back to configured "agent" source when the agent is not contributing', () => {
    // A stale agent is not active/paused, so the contributor path is empty and
    // sourceFor falls through to configured-presence (the distinct fallback arm).
    const row = connectionToRow(
      connectionFixture({
        id: 'agent:stale-1',
        type: 'agent',
        name: 'stale-1',
        address: 'stale-1',
        state: 'stale',
        surfaces: ['host'],
        scope: { host: true },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      }),
    );
    expect(row.source).toBe('agent');
  });

  it('reports source "unknown" when no contributing or configured path matches', () => {
    const row = connectionToRow(
      connectionFixture({
        id: 'docker:down',
        type: 'docker',
        name: 'down',
        address: 'down',
        state: 'unauthorized',
        surfaces: [],
        scope: {},
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      }),
    );
    expect(row.source).toBe('unknown');
  });

  // --- subtitleFor product-label fallback ---
  it('uses the product-label subtitle for a docker source', () => {
    const row = connectionToRow(
      connectionFixture({
        id: 'docker:host',
        type: 'docker',
        name: 'dock',
        address: 'dock',
        state: 'active',
        surfaces: [],
        scope: {},
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      }),
    );
    expect(row.subtitle).toBe('via Docker');
  });

  // --- canEdit / canRemove ---
  it.each([
    { type: 'docker' as const, name: 'docker-host' },
    { type: 'kubernetes' as const, name: 'k8s-cluster' },
  ])('disables edit and removal for $type connections', ({ type, name }) => {
    const row = connectionToRow(
      connectionFixture({
        type,
        name,
        address: name,
        surfaces: [],
        scope: {},
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      }),
    );
    expect(row.canEdit).toBe(false);
    expect(row.canRemove).toBe(false);
  });

  // --- coverageLabelsFor scope handling ---
  it('uses only active scope keys for coverage labels', () => {
    const row = connectionToRow(
      connectionFixture({
        surfaces: ['vms', 'containers', 'storage'],
        scope: { vms: true, containers: false },
      }),
    );
    expect(row.coverageLabels).toEqual(['VMs']);
  });

  it('falls back to surfaces when no scope key is active', () => {
    const row = connectionToRow(
      connectionFixture({
        surfaces: ['vms', 'containers'],
        scope: { vms: false, containers: false },
      }),
    );
    expect(row.coverageLabels).toEqual(['VMs', 'Containers']);
  });

  it('falls back to surfaces when scope is null', () => {
    const row = connectionToRow(
      connectionFixture({ surfaces: ['host'], scope: null as unknown as Connection['scope'] }),
    );
    expect(row.coverageLabels).toEqual(['Host telemetry']);
  });

  // --- status presentation fallback ---
  it('falls back to the Pending presentation for an unrecognized state', () => {
    const row = connectionToRow(
      connectionFixture({ state: 'bogus' as unknown as ConnectionState }),
    );
    expect(row.statusLabel).toBe('Pending');
    expect(row.statusClassName).toBe('bg-surface-alt text-base-content');
  });

  // --- lastErrorMessage (runs raw text through the error presenter) ---
  it('humanizes a raw timeout error for display', () => {
    const row = connectionToRow(
      connectionFixture({
        lastError: { message: 'context deadline exceeded', at: '2026-07-22T12:00:00Z' },
      }),
    );
    expect(row.lastErrorMessage).toBe(
      'Connection timed out. Check the host is reachable, the port is correct, and the network path is open.',
    );
  });

  it('leaves lastErrorMessage undefined when there is no error', () => {
    const row = connectionToRow(connectionFixture({ lastError: null }));
    expect(row.lastErrorMessage).toBeUndefined();
  });
});

// ---- useConnectionsLedger (systemToRow & row-derivation arms) ---------------

describe('useConnectionsLedger (systemToRow & row-derivation arms)', () => {
  useLedgerHarness();

  it('drops a system whose primary connection is absent and falls back to standalone rows', async () => {
    // systemToRow returns null when connectionsByID.get(system.id) misses; with
    // every system dropped, groupedRows is empty and the memo falls back to
    // rendering raw connections as standalone rows.
    const solo = connectionFixture({
      id: 'agent:solo',
      type: 'agent',
      name: 'solo',
      address: 'solo',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const orphanSystem: ConnectionSystem = {
      id: 'pve:ghost',
      type: 'pve',
      components: [{ connectionId: 'pve:ghost', type: 'pve', role: 'primary' }],
    };
    const { result } = renderLedger([solo], [orphanSystem]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].id).toBe('agent:solo');
  });

  it('falls back to the primary connection when all component refs dangle', async () => {
    // componentConnections filters out missing refs; when none remain it pushes
    // the primary so the row still derives source/coverage from it.
    const primary = connectionFixture({
      id: 'pve:node',
      name: 'node',
      address: 'https://node:8006',
    });
    const system: ConnectionSystem = {
      id: 'pve:node',
      type: 'pve',
      components: [{ connectionId: 'pve:missing', type: 'pve', role: 'primary' }],
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].source).toBe('api');
    expect(result.rows()[0].coverageLabels).toEqual(['VMs', 'Containers']);
  });

  it('surfaces an unhealthy attached agent as the row problem on a non-agent system', async () => {
    const vmware = connectionFixture({
      id: 'vmware:vc-1',
      type: 'vmware',
      name: 'Lab vCenter',
      address: 'https://vcenter:443',
      surfaces: ['vms', 'hosts'],
      scope: { vms: true, hosts: true },
    });
    const staleAgent = connectionFixture({
      id: 'agent:stale-attached',
      type: 'agent',
      name: 'stale-attached',
      address: 'stale-attached',
      state: 'stale',
      stateReason: 'no keepalive',
      lastSeen: null,
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const system: ConnectionSystem = {
      id: 'vmware:vc-1',
      type: 'vmware',
      components: [
        { connectionId: 'vmware:vc-1', type: 'vmware', role: 'primary' },
        { connectionId: 'agent:stale-attached', type: 'agent', role: 'attachment' },
      ],
      members: [
        { id: 'host-1', name: 'esxi-01', state: 'active', lastSeen: '2026-07-22T12:00:00Z' },
      ],
    };
    const { result } = renderLedger([vmware, staleAgent], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].problem).toMatchObject({ label: 'Agent offline', tone: 'warning' });
  });

  it('drops a cluster member with a blank name and counts only valid members', async () => {
    // buildMemberRow returns null for a whitespace name; the member is filtered
    // out, leaving a single-node cluster (singular "node" subtitle).
    const primary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const system: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        {
          id: 'node-good',
          name: 'good-node',
          state: 'active',
          lastSeen: '2026-07-22T12:00:00Z',
          primary: true,
        },
        { id: 'node-blank', name: '   ', state: 'active', lastSeen: '2026-07-22T12:00:00Z' },
      ],
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].isCluster).toBe(true);
    expect(result.rows()[0].subtitle).toBe('Cluster · 1 node');
    expect(result.rows()[0].members).toHaveLength(1);
    expect(result.rows()[0].members[0].name).toBe('good-node');
  });

  it('derives the row error message from an attached connection when the primary has none', async () => {
    const primary = connectionFixture({
      id: 'pve:node',
      name: 'node',
      address: 'https://node:8006',
      lastError: null,
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const attached = connectionFixture({
      id: 'agent:attached',
      type: 'agent',
      name: 'attached',
      address: 'attached',
      state: 'active',
      lastError: { message: 'connection refused', at: '2026-07-22T12:00:00Z' },
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const system: ConnectionSystem = {
      id: 'pve:node',
      type: 'pve',
      components: [
        { connectionId: 'pve:node', type: 'pve', role: 'primary' },
        { connectionId: 'agent:attached', type: 'agent', role: 'attachment' },
      ],
    };
    const { result } = renderLedger([primary, attached], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].lastErrorMessage).toBe(
      'Connection refused. The host is reachable but rejected the connection on this port. Check the port is correct and the service is running.',
    );
  });

  it('surfaces the fetch error and keeps rows empty', async () => {
    vi.spyOn(ConnectionsAPI, 'list').mockRejectedValue(new Error('network down'));
    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.error()).toBeInstanceOf(Error));
    expect((result.error() as Error).message).toBe('network down');
    expect(result.rows()).toEqual([]);
    expect(result.loading()).toBe(false);
  });
});

// ---- standalone-agent dedupe vs cluster members ----------------------------

describe('useConnectionsLedger (standalone-agent cluster dedupe)', () => {
  useLedgerHarness();

  const clusterSystem = (memberName: string): ConnectionSystem => ({
    id: 'pve:cluster',
    type: 'pve',
    clusterName: 'homelab',
    components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
    members: [
      {
        id: 'm-delly',
        name: memberName,
        state: 'active',
        lastSeen: '2026-07-22T12:00:00Z',
        primary: true,
      },
    ],
  });

  const agentSystem = (agentId: string): ConnectionSystem => ({
    id: agentId,
    type: 'agent',
    components: [{ connectionId: agentId, type: 'agent', role: 'primary' }],
  });

  it('dedupes a standalone agent system whose name matches a cluster member', async () => {
    const pvePrimary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const dupAgent = connectionFixture({
      id: 'agent:dup',
      type: 'agent',
      name: 'delly',
      address: 'delly',
      state: 'active',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const { result } = renderLedger(
      [pvePrimary, dupAgent],
      [clusterSystem('delly'), agentSystem('agent:dup')],
    );

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].id).toBe('pve:cluster');
    expect(result.rows().some((row) => row.id === 'agent:dup')).toBe(false);
  });

  it('keeps a standalone agent system whose host is not a cluster member', async () => {
    const pvePrimary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const distinctAgent = connectionFixture({
      id: 'agent:other',
      type: 'agent',
      name: 'other-host',
      address: 'other-host',
      state: 'active',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const { result } = renderLedger(
      [pvePrimary, distinctAgent],
      [clusterSystem('delly'), agentSystem('agent:other')],
    );

    await waitFor(() => expect(result.rows()).toHaveLength(2));
    expect(
      result
        .rows()
        .map((row) => row.id)
        .sort(),
    ).toEqual(['agent:other', 'pve:cluster']);
  });

  it('dedupes a standalone agent via a matching hostAlias', async () => {
    const pvePrimary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const aliasAgent = connectionFixture({
      id: 'agent:alias',
      type: 'agent',
      name: 'unrelated',
      address: 'unrelated',
      hostAliases: ['delly'],
      state: 'active',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const { result } = renderLedger(
      [pvePrimary, aliasAgent],
      [clusterSystem('delly'), agentSystem('agent:alias')],
    );

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows().some((row) => row.id === 'agent:alias')).toBe(false);
  });
});

// ---- cluster rollup edge cases ---------------------------------------------
// oldestTimestamp (null/NaN skip + all-invalid -> undefined) and
// moreSevereState (left-wins + falsy-right arms). Only Date is faked so the
// testing-library waitFor polling keeps using real timers.

describe('useConnectionsLedger (cluster rollup edge cases)', () => {
  useLedgerHarness();

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(PINNED_NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('skips null and unparseable member timestamps and uses the oldest valid one', async () => {
    const primary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      lastSeen: null,
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const system: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        { id: 'm-bad', name: 'bad-node', state: 'active', lastSeen: 'not-a-date', primary: true },
        { id: 'm-good', name: 'good-node', state: 'active', lastSeen: '2026-07-22T11:00:00Z' },
      ],
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // null + 'not-a-date' skipped; '2026-07-22T11:00:00Z' is exactly 1h before now.
    expect(result.rows()[0].lastActivityText).toBe('1h ago');
  });

  it('reports "No activity yet" when every cluster timestamp is null or unparseable', async () => {
    const primary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      lastSeen: null,
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const system: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        { id: 'm-bad', name: 'bad-node', state: 'active', lastSeen: 'garbage', primary: true },
      ],
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].lastActivityText).toBe('No activity yet');
  });

  it('keeps the more severe primary state when a member is healthier', async () => {
    const primary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      state: 'unreachable',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const system: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        {
          id: 'm-active',
          name: 'active-node',
          state: 'active',
          lastSeen: '2026-07-22T12:00:00Z',
          primary: true,
        },
      ],
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // primary unreachable (severity 5) vs member active (0): after the reduce
    // from 'active' the unreachable member wins, then active cannot overtake it
    // (left-wins arm of moreSevereState).
    expect(result.rows()[0].statusLabel).toBe('Unreachable');
  });

  it('tolerates a malformed member state by keeping the accumulated rollup', async () => {
    const primary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      state: 'active',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const system: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        {
          id: 'm-undef',
          name: 'undef-node',
          state: undefined as unknown as ConnectionState,
          lastSeen: '2026-07-22T12:00:00Z',
          primary: true,
        },
      ],
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // The undefined member state is falsy, so moreSevereState short-circuits
    // (!right) and the cluster stays Active; the member row itself falls back
    // to the Pending presentation.
    expect(result.rows()[0].statusLabel).toBe('Active');
    expect(result.rows()[0].members[0].statusLabel).toBe('Pending');
  });
});
