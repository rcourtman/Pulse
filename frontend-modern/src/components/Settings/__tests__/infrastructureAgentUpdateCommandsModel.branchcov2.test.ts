import { describe, expect, it } from 'vitest';
import type { Connection, ConnectionType } from '@/api/connections';
import type {
  InfrastructureSystemMemberRow,
  InfrastructureSystemRow,
} from '../connectionsTableModel';
import { collectInfrastructureAgentUpdateTargets } from '../infrastructureAgentUpdateCommandsModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling infrastructureAgentUpdateCommandsModel.test.ts factory
// shape. The six functions under test (rowContextLabel, updateInstallFlagsForRow,
// normalizeAgentConnectionID, connectionDisplayName, expectedVersionFor,
// pushTarget) are all module-private, so every branch is driven through the
// single exported orchestrator `collectInfrastructureAgentUpdateTargets`.

const connection = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:homelab',
  type: 'pve',
  name: 'homelab',
  address: 'https://pve.lab:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms'],
  scope: { vms: true },
  lastSeen: new Date().toISOString(),
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const emptyFleetRow = {
  fleetSignals: [],
  fleetHighlights: [],
} satisfies Pick<InfrastructureSystemRow, 'fleetSignals' | 'fleetHighlights'>;

const emptyFleetMember = {
  fleetSignals: [],
  fleetHighlights: [],
} satisfies Pick<InfrastructureSystemMemberRow, 'fleetSignals' | 'fleetHighlights'>;

const row = (overrides: Partial<InfrastructureSystemRow> = {}): InfrastructureSystemRow => {
  const primary = connection();
  return {
    id: primary.id,
    ownerType: 'pve',
    name: 'homelab',
    subtitle: 'Cluster · 2 nodes',
    source: 'both',
    host: primary.address,
    coverageLabels: ['VMs', 'Host telemetry'],
    statusLabel: 'Active',
    statusClassName: 'bg-green-100 text-green-800',
    agentUpdateCount: 0,
    lastActivityText: '1m ago',
    ...emptyFleetRow,
    enabled: true,
    canEdit: true,
    canPause: true,
    canRemove: true,
    isAgent: false,
    isCluster: false,
    attachedConnections: [],
    members: [],
    connection: primary,
    ...overrides,
  };
};

const member = (
  overrides: Partial<InfrastructureSystemMemberRow> = {},
): InfrastructureSystemMemberRow => ({
  id: 'node-1',
  name: 'node-1',
  subtitle: 'Primary node',
  source: 'both',
  host: 'https://node-1:8006',
  coverageLabels: ['Host telemetry'],
  statusLabel: 'Active',
  statusClassName: 'bg-green-100 text-green-800',
  lastActivityText: '1m ago',
  ...emptyFleetMember,
  primary: true,
  ...overrides,
});

// A stale agent is an agent-type connection that connectionNeedsUpdate() reports
// true for. agentUpdateAvailable: true short-circuits the version comparison in
// connectionNeedsUpdate, so the agent always reaches the target-building body
// of pushTarget unless stated otherwise.
const staleAgent = (overrides: Partial<Connection> = {}): Connection =>
  connection({
    id: 'agent:agent-1',
    type: 'agent',
    name: 'agent-1',
    address: 'agent-1',
    surfaces: ['host'],
    scope: { host: true },
    source: 'agent',
    agentVersion: '5.1.34',
    expectedAgentVersion: '6.0.0',
    agentUpdateAvailable: true,
    agentIdentity: { hostname: 'agent-1', platform: 'linux' },
    capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    ...overrides,
  });

// ---- normalizeAgentConnectionID --------------------------------------------
// Reachable via scopedAgentIds.map(...) and via the scope filter
// (normalizeAgentConnectionID(target.key)). Branches: the (value || '') falsy
// arm, the !trimmed early return, and both ternary arms of the agent: prefix
// check.

describe('normalizeAgentConnectionID (via scopedAgentIds + scope filter)', () => {
  it('drops null/undefined/blank scope entries, keeps already-prefixed ids as-is, and prefixes bare ids', () => {
    const agentA = staleAgent({
      id: 'agent:host-1',
      name: 'host-1',
      address: 'host-1',
      agentIdentity: { hostname: 'host-1', platform: 'linux' },
    });
    const agentB = staleAgent({
      id: 'host-2',
      name: 'host-2',
      address: 'host-2',
      agentIdentity: { hostname: 'host-2', platform: 'linux' },
    });
    const agentC = staleAgent({
      id: 'agent:host-3',
      name: 'host-3',
      address: 'host-3',
      agentIdentity: { hostname: 'host-3', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [row({ attachedConnections: [agentA, agentB, agentC] })],
      undefined,
      // null/undefined exercise the (value || '') falsy operand; '' and '   '
      // exercise the !trimmed early return; 'agent:host-1' exercises the
      // already-prefixed ternary arm; 'host-2' exercises the prefix-adding arm.
      [null, undefined, '', '   ', 'agent:host-1', 'host-2'] as unknown as readonly string[],
    );

    // Only host-1 and host-2 are in the normalised scope set; host-3 is filtered
    // out. Results are sorted by displayName ascending.
    expect(targets.map((target) => target.key)).toEqual(['agent:host-1', 'host-2']);
  });

  it('retains a target whose raw id is not agent-prefixed when a bare scope entry normalises to the same key', () => {
    // target.key 'host-9' normalises to 'agent:host-9' inside the filter; the
    // scope entry 'host-9' also normalises to 'agent:host-9', so both sides hit
    // the prefix-adding ternary arm and the target is retained.
    const agent = staleAgent({
      id: 'host-9',
      name: 'host-9',
      address: 'host-9',
      agentIdentity: { hostname: 'host-9', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [row({ attachedConnections: [agent] })],
      undefined,
      ['host-9'],
    );

    expect(targets).toHaveLength(1);
    expect(targets[0]?.key).toBe('host-9');
  });
});

// ---- updateInstallFlagsForRow -----------------------------------------------
// Reachable via pushTarget -> installFlags. Branches: every switch arm (pve,
// pbs, docker, kubernetes) plus the implicit default (no case matches).

describe('updateInstallFlagsForRow (via target.installFlags)', () => {
  it.each([
    ['pve', ['--enable-proxmox', '--proxmox-type pve']],
    ['pbs', ['--enable-proxmox', '--proxmox-type pbs']],
    ['docker', ['--enable-docker']],
    ['kubernetes', ['--enable-kubernetes']],
    ['agent', []],
    ['pmg', []],
    ['vmware', []],
    ['truenas', []],
    ['availability', []],
  ] as const satisfies ReadonlyArray<[ConnectionType, string[]]>)(
    'emits the expected install flags for ownerType %s',
    (ownerType, expectedFlags) => {
      const agent = staleAgent({
        id: `agent:${ownerType}-node`,
        name: `${ownerType}-node`,
        address: `${ownerType}-node`,
        agentIdentity: { hostname: `${ownerType}-node`, platform: 'linux' },
      });

      const targets = collectInfrastructureAgentUpdateTargets([
        row({ ownerType, attachedConnections: [agent] }),
      ]);

      expect(targets).toHaveLength(1);
      expect(targets[0]?.installFlags).toStrictEqual(expectedFlags);
    },
  );
});

// ---- connectionDisplayName --------------------------------------------------
// Reachable via pushTarget -> displayName. Branches: every operand of the
// short-circuit || chain (hostname, name, address, id) plus the optional-chain
// miss when agentIdentity is undefined.

describe('connectionDisplayName (via target.displayName)', () => {
  it('prefers the agentIdentity hostname when it is present and non-empty', () => {
    const agent = staleAgent({
      id: 'agent:h1',
      name: 'named-host',
      address: '10.0.0.1',
      agentIdentity: { hostname: 'real-hostname', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.displayName).toBe('real-hostname');
  });

  it('falls back to connection.name when agentIdentity is undefined (optional-chain miss)', () => {
    const agent = staleAgent({
      id: 'agent:h2',
      name: 'named-host',
      address: '10.0.0.2',
      agentIdentity: undefined,
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.displayName).toBe('named-host');
  });

  it('falls back to connection.name when hostname trims to empty', () => {
    const agent = staleAgent({
      id: 'agent:h3',
      name: 'named-host',
      address: '10.0.0.3',
      agentIdentity: { hostname: '   ', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.displayName).toBe('named-host');
  });

  it('falls back to address when both hostname and name trim to empty', () => {
    const agent = staleAgent({
      id: 'agent:h4',
      name: '   ',
      address: '10.0.0.4',
      agentIdentity: { hostname: '   ', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.displayName).toBe('10.0.0.4');
  });

  it('falls back to the connection id when hostname, name, and address are all blank', () => {
    const agent = staleAgent({
      id: 'agent:h5',
      name: '',
      address: '',
      agentIdentity: { hostname: '', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.displayName).toBe('agent:h5');
  });
});

// ---- rowContextLabel --------------------------------------------------------
// Reachable via pushTarget -> contextLabel. Branches: the cluster arm
// (isCluster && name.trim()), the ownerType==='agent' arm, and the three-way
// return fallback (row.name.trim() || connection.name || ownerType).

describe('rowContextLabel (via target.contextLabel)', () => {
  it('returns the raw (untrimmed) row name on the cluster arm', () => {
    const agent = staleAgent({ id: 'agent:c1' });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ isCluster: true, name: '  my-cluster  ', attachedConnections: [agent] }),
    ]);

    // The cluster arm returns row.name verbatim (no .trim()), unlike the
    // non-cluster return arm. See GLM_REPORT for the suspected inconsistency.
    expect(targets[0]?.contextLabel).toBe('  my-cluster  ');
  });

  it('falls through the cluster arm when the cluster name trims to empty', () => {
    const agent = staleAgent({ id: 'agent:c2' });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ isCluster: true, name: '   ', attachedConnections: [agent] }),
    ]);

    // isCluster true but name.trim() falsy -> cluster condition fails; ownerType
    // pve (not agent); row.name.trim() falsy; falls back to connection.name.
    expect(targets[0]?.contextLabel).toBe('homelab');
  });

  it("returns 'Machine' for an agent-type row", () => {
    const agent = staleAgent({ id: 'agent:c3' });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ ownerType: 'agent', isAgent: true, name: 'whatever', attachedConnections: [agent] }),
    ]);

    expect(targets[0]?.contextLabel).toBe('Machine');
  });

  it('returns the trimmed row name for a non-cluster, non-agent row with a name', () => {
    const agent = staleAgent({ id: 'agent:c4' });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ ownerType: 'docker', name: '  docker-host  ', attachedConnections: [agent] }),
    ]);

    expect(targets[0]?.contextLabel).toBe('docker-host');
  });

  it('falls back to connection.name when the row name is blank', () => {
    const primary = connection({ name: 'fallback-name' });
    const agent = staleAgent({ id: 'agent:c5' });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ ownerType: 'docker', name: '', connection: primary, attachedConnections: [agent] }),
    ]);

    expect(targets[0]?.contextLabel).toBe('fallback-name');
  });

  it('falls back to ownerType when both row name and connection name are blank', () => {
    const primary = connection({ name: '' });
    const agent = staleAgent({ id: 'agent:c6' });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ ownerType: 'docker', name: '', connection: primary, attachedConnections: [agent] }),
    ]);

    expect(targets[0]?.contextLabel).toBe('docker');
  });
});

// ---- expectedVersionFor -----------------------------------------------------
// Reachable via pushTarget -> expectedVersion. Branches: every operand of the
// short-circuit || chain (expectedAgentVersion, formatAgentVersionDisplay,
// undefined).

describe('expectedVersionFor (via target.expectedVersion)', () => {
  it('returns the trimmed expectedAgentVersion when the connection carries one', () => {
    const agent = staleAgent({
      id: 'agent:e1',
      expectedAgentVersion: '  6.0.1  ',
      agentUpdateAvailable: true,
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.expectedVersion).toBe('6.0.1');
  });

  it('formats targetVersion when expectedAgentVersion is absent but targetVersion parses', () => {
    const agent = staleAgent({
      id: 'agent:e2',
      expectedAgentVersion: undefined,
      agentVersion: '6.0.0-rc.5',
      agentUpdateAvailable: false,
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [row({ attachedConnections: [agent] })],
      '6.0.0-rc.6',
    );

    // connectionNeedsUpdate compares 6.0.0-rc.5 < 6.0.0-rc.6 -> true;
    // formatAgentVersionDisplay normalises to a leading 'v'.
    expect(targets[0]?.expectedVersion).toBe('v6.0.0-rc.6');
  });

  it('returns undefined when neither expectedAgentVersion nor a targetVersion is present', () => {
    const agent = staleAgent({
      id: 'agent:e3',
      expectedAgentVersion: undefined,
      agentUpdateAvailable: true,
    });

    const targets = collectInfrastructureAgentUpdateTargets([row({ attachedConnections: [agent] })]);

    expect(targets[0]?.expectedVersion).toBeUndefined();
  });

  it('returns undefined when expectedAgentVersion is absent and targetVersion is unparseable', () => {
    const agent = staleAgent({
      id: 'agent:e4',
      expectedAgentVersion: undefined,
      agentUpdateAvailable: true,
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [row({ attachedConnections: [agent] })],
      'not-a-version',
    );

    // agentUpdateAvailable true keeps the agent eligible; formatAgentVersionDisplay
    // ('not-a-version') returns '' so the final || undefined operand wins.
    expect(targets).toHaveLength(1);
    expect(targets[0]?.expectedVersion).toBeUndefined();
  });
});

// ---- pushTarget -------------------------------------------------------------
// Branches: the !connection guard, the type!=='agent' guard, the
// !connectionNeedsUpdate guard, the has(key) dedupe guard, and the happy-path
// set.

describe('pushTarget (guard + dedupe branches)', () => {
  it('skips a member whose agentConnection is undefined (the !connection guard)', () => {
    const targets = collectInfrastructureAgentUpdateTargets([
      row({ members: [member({ agentConnection: undefined })] }),
    ]);

    expect(targets).toEqual([]);
  });

  it('skips a non-agent primary connection (the connection.type !== "agent" guard)', () => {
    // Default row.connection is the pve primary; no attached/members agents.
    const targets = collectInfrastructureAgentUpdateTargets([row()]);

    expect(targets).toEqual([]);
  });

  it('skips a current agent whose version equals the target (the !connectionNeedsUpdate guard)', () => {
    const current = staleAgent({
      id: 'agent:current',
      agentVersion: '6.0.0',
      expectedAgentVersion: undefined,
      agentUpdateAvailable: false,
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [row({ attachedConnections: [current] })],
      'v6.0.0',
    );

    // compareAgentVersions('6.0.0', 'v6.0.0') === 0 -> not < 0 -> needsUpdate
    // false -> pushTarget returns early.
    expect(targets).toEqual([]);
  });

  it('dedupes an agent that appears in both attachedConnections and a member (the has(key) guard)', () => {
    const shared = staleAgent({
      id: 'agent:shared',
      name: 'shared',
      address: 'shared',
      agentIdentity: { hostname: 'shared', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({
        attachedConnections: [shared],
        members: [member({ agentConnection: shared })],
      }),
    ]);

    expect(targets).toHaveLength(1);
    expect(targets[0]?.key).toBe('agent:shared');
  });

  it('pushes when the primary connection itself is a stale agent (happy path via row.connection)', () => {
    const agent = staleAgent({
      id: 'agent:primary-agent',
      name: 'primary-agent',
      address: 'primary-agent',
      agentIdentity: { hostname: 'primary-agent', platform: 'linux' },
    });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({ ownerType: 'agent', isAgent: true, connection: agent }),
    ]);

    expect(targets).toHaveLength(1);
    expect(targets[0]?.key).toBe('agent:primary-agent');
  });
});
