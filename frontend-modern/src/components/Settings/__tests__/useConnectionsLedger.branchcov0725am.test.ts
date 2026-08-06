import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionSystem,
  type ConnectionType,
} from '@/api/connections';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { connectionToRow, useConnectionsLedger } from '../useConnectionsLedger';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling branchcov suites: a minimal valid Connection factory and
// the shared hook harness (reset retained cache + restore mocks each case).

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

// ---- connectionToRow (subtitleFor / sourceFor fallback arms) ----------------

describe('connectionToRow (line 135: subtitleFor unknown-type fallback)', () => {
  it('falls back to the raw type string when the type is absent from CONNECTION_TYPE_LABELS', () => {
    // Every valid ConnectionType is a key in CONNECTION_TYPE_LABELS, so the
    // ?? primaryConnection.type arm can only fire for a type the label map
    // does not know — simulating a backend sending an unrecognised type string.
    const row = connectionToRow(
      connectionFixture({
        id: 'mystery:host',
        type: 'mystery' as unknown as ConnectionType,
        name: 'mystery-host',
        address: 'mystery-host',
        surfaces: [],
        scope: {},
      }),
    );
    expect(row.subtitle).toBe('via mystery');
  });
});

describe('connectionToRow (line 161: sourceFor configured-availability fallback)', () => {
  it('reports source "probe" from configured presence when an availability target is not contributing', () => {
    // A stale availability target is not active/paused, so the contributor arm
    // (line 156) is skipped and sourceFor falls through to the configured-
    // presence availability arm (line 161) — distinct from the existing spec
    // which uses an active (contributing) availability target hitting line 156.
    const row = connectionToRow(
      connectionFixture({
        id: 'availability:stale',
        type: 'availability',
        name: 'stale-target',
        address: 'stale-target',
        state: 'stale',
        surfaces: ['availability'],
        scope: { availability: true },
      }),
    );
    expect(row.source).toBe('probe');
  });
});

// ---- useConnectionsLedger (hook-driven arms) --------------------------------

describe('useConnectionsLedger (line 206: stableRecordEntries null-scope signature)', () => {
  useLedgerHarness();

  it('renders a row whose connection has a null scope', async () => {
    // connectionRowSignature calls stableRecordEntries(connection.scope).
    // With scope === null the ?? {} arm fires so Object.entries does not throw.
    // The sibling suite covers null scope via connectionToRow directly (which
    // exercises coverageLabelsFor), but never through the hook's signature
    // path — a distinct code site on line 206.
    const conn = connectionFixture({
      id: 'agent:null-scope',
      type: 'agent',
      name: 'null-scope',
      address: 'null-scope',
      state: 'active',
      surfaces: ['host'],
      scope: null as unknown as Connection['scope'],
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const { result } = renderLedger([conn], []);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // Null scope propagates: coverageLabelsFor also falls back to surfaces.
    expect(result.rows()[0].coverageLabels).toEqual(['Host telemetry']);
  });
});

describe('useConnectionsLedger (line 302: buildMemberRow member-lastSeen fallback)', () => {
  useLedgerHarness();

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-07-22T12:00:00.000Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('derives member lastSeen from the paired agent when the member has none', async () => {
    // member.lastSeen ?? agentConnection?.lastSeen — the fallback arm fires
    // when the member omits lastSeen. The sibling suite always sets lastSeen
    // on every member, so the agent-fallback arm is cold without this case.
    const vmware = connectionFixture({
      id: 'vmware:vc',
      type: 'vmware',
      name: 'Lab vCenter',
      address: 'https://vcenter:443',
      surfaces: ['vms', 'hosts'],
      scope: { vms: true, hosts: true },
    });
    const attachedAgent = connectionFixture({
      id: 'agent:attached',
      type: 'agent',
      name: 'attached',
      address: 'attached',
      state: 'active',
      lastSeen: '2026-07-22T11:00:00Z',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const system: ConnectionSystem = {
      id: 'vmware:vc',
      type: 'vmware',
      components: [
        { connectionId: 'vmware:vc', type: 'vmware', role: 'primary' },
        { connectionId: 'agent:attached', type: 'agent', role: 'attachment' },
      ],
      members: [
        {
          id: 'host-1',
          name: 'esxi-01',
          state: 'active',
          lastSeen: null,
          agentConnectionId: 'agent:attached',
        },
      ],
    };
    const { result } = renderLedger([vmware, attachedAgent], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // member.lastSeen null → agentConnection.lastSeen (exactly 1h before pinned now).
    expect(result.rows()[0].members[0].lastActivityText).toBe('1h ago');
  });
});

describe('useConnectionsLedger (lines 543-544: fetcher nullish-field tolerance)', () => {
  useLedgerHarness();

  it('defaults to empty arrays when the list response omits connections and systems', async () => {
    // The fetcher does response.connections ?? [] / response.systems ?? [].
    // The sibling suites always supply both arrays; a malformed response that
    // omits them exercises both fallback arms.
    const listSpy = vi
      .spyOn(ConnectionsAPI, 'list')
      .mockResolvedValue({} as unknown as Awaited<ReturnType<typeof ConnectionsAPI.list>>);
    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(listSpy).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(result.loading()).toBe(false));
    // Both ?? [] arms fire; the hook exposes arrays, not undefined.
    expect(result.connections()).toEqual([]);
    expect(result.rows()).toEqual([]);
  });
});

describe('useConnectionsLedger (line 584: clusterMemberHosts null-members tolerance)', () => {
  useLedgerHarness();

  it('collects no cluster-member hosts when a pve cluster system omits members', async () => {
    // The clusterMemberHosts loop guards with system.members ?? []. A pve
    // system with a clusterName but members === undefined exercises the ?? arm
    // instead of throwing "undefined is not iterable". The sibling suites
    // always provide members (or omit clusterName, hitting the `continue`).
    const primary = connectionFixture({
      id: 'pve:nomembers',
      name: 'nomembers',
      address: 'https://nomembers:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const system: ConnectionSystem = {
      id: 'pve:nomembers',
      type: 'pve',
      clusterName: 'empty-cluster',
      components: [{ connectionId: 'pve:nomembers', type: 'pve', role: 'primary' }],
      members: undefined,
    };
    const { result } = renderLedger([primary], [system]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // rawMembers ?? [] → [] → no members built, isCluster stays false.
    expect(result.rows()[0].isCluster).toBe(false);
    expect(result.rows()[0].members).toEqual([]);
  });
});

describe('useConnectionsLedger (line 597: dedupe null-components tolerance)', () => {
  useLedgerHarness();

  it('dedupes an agent system whose components field is absent', async () => {
    // system.components?.length ?? 0 → 0 ≤ 1 → the standalone-agent dedupe
    // check runs even though components is undefined. The sibling suites
    // always supply components, so the ?? 0 arm is cold without this case.
    const pvePrimary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const dupAgent = connectionFixture({
      id: 'agent:nocomponents',
      type: 'agent',
      name: 'delly',
      address: 'delly',
      state: 'active',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const clusterSystem: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        {
          id: 'm-delly',
          name: 'delly',
          state: 'active',
          lastSeen: '2026-07-22T12:00:00Z',
          primary: true,
        },
      ],
    };
    const agentSystem: ConnectionSystem = {
      id: 'agent:nocomponents',
      type: 'agent',
      components: undefined as unknown as ConnectionSystem['components'],
    };
    const { result } = renderLedger([pvePrimary, dupAgent], [clusterSystem, agentSystem]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // components?.length ?? 0 → 0 ≤ 1 → dedupe runs; agent host matches member.
    expect(result.rows().some((row) => row.id === 'agent:nocomponents')).toBe(false);
  });
});

describe('useConnectionsLedger (line 602: dedupe via agentIdentity.hostname)', () => {
  useLedgerHarness();

  it('dedupes an agent whose agentIdentity.hostname matches a cluster member', async () => {
    // Exercises the non-short-circuit arm of primary?.agentIdentity?.hostname:
    // agentIdentity is defined and .hostname is read, then matched. The sibling
    // suites dedupe via name and hostAliases but never via agentIdentity.hostname
    // (their agents have no agentIdentity at all, so the ?. short-circuits).
    const pvePrimary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const identityAgent = connectionFixture({
      id: 'agent:identity',
      type: 'agent',
      name: 'unrelated-name',
      address: 'unrelated-addr',
      agentIdentity: { hostname: 'delly', platform: 'linux' },
      state: 'active',
      surfaces: ['host'],
      scope: { host: true },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const clusterSystem: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        {
          id: 'm-delly',
          name: 'delly',
          state: 'active',
          lastSeen: '2026-07-22T12:00:00Z',
          primary: true,
        },
      ],
    };
    const agentSystem: ConnectionSystem = {
      id: 'agent:identity',
      type: 'agent',
      components: [{ connectionId: 'agent:identity', type: 'agent', role: 'primary' }],
    };
    const { result } = renderLedger([pvePrimary, identityAgent], [clusterSystem, agentSystem]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // Neither name nor address match; agentIdentity.hostname is the hit.
    expect(result.rows().some((row) => row.id === 'agent:identity')).toBe(false);
  });

  it('evaluates dedupe candidates when the agent primary connection is absent', async () => {
    // Exercises the short-circuit arm of primary?.agentIdentity?.hostname (and
    // the sibling primary?. accesses on lines 600-601) when the agent system
    // references a connection that is not in the ledger. The existing orphan-
    // system spec drops a pve system; an agent orphan additionally enters the
    // line-597 dedupe block, which the pve path never reaches.
    const pvePrimary = connectionFixture({
      id: 'pve:cluster',
      name: 'cluster',
      address: 'https://cluster:8006',
      surfaces: ['vms'],
      scope: { vms: true },
    });
    const clusterSystem: ConnectionSystem = {
      id: 'pve:cluster',
      type: 'pve',
      clusterName: 'homelab',
      components: [{ connectionId: 'pve:cluster', type: 'pve', role: 'primary' }],
      members: [
        {
          id: 'm-delly',
          name: 'delly',
          state: 'active',
          lastSeen: '2026-07-22T12:00:00Z',
          primary: true,
        },
      ],
    };
    const orphanAgentSystem: ConnectionSystem = {
      id: 'agent:missing',
      type: 'agent',
      components: [{ connectionId: 'agent:missing', type: 'agent', role: 'primary' }],
    };
    const { result } = renderLedger([pvePrimary], [clusterSystem, orphanAgentSystem]);

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    // primary is undefined → all candidates undefined → dedupe fails →
    // systemToRow returns null → orphan dropped. Only the cluster survives.
    expect(result.rows()[0].id).toBe('pve:cluster');
    expect(result.rows().some((row) => row.id === 'agent:missing')).toBe(false);
  });
});
