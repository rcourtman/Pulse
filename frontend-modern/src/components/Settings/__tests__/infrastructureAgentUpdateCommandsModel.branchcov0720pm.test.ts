import { describe, expect, it } from 'vitest';
import type { Connection, ConnectionAgentUpdateStatus } from '@/api/connections';
import type { AgentFleetAgentDiagnostic } from '@/api/agentDiagnostics';
import type {
  InfrastructureSystemMemberRow,
  InfrastructureSystemRow,
} from '../connectionsTableModel';
import {
  collectInfrastructureAgentDoctorTargets,
  collectInfrastructureAgentUpdateTargets,
  diagnosticConnectionID,
  normalizeAgentConnectionID,
  resolveKnownAgentCommandPlatform,
  summarizeInfrastructureAgentDoctorTargets,
} from '../infrastructureAgentUpdateCommandsModel';
import type {
  InfrastructureAgentDoctorOptions,
  InfrastructureAgentDoctorStatus,
  InfrastructureAgentDoctorTarget,
} from '../infrastructureAgentUpdateCommandsModel';

// ---- Fixtures ---------------------------------------------------------------
// Minimal valid factories. The agent is the only connection type the doctor
// bindings retain, so every fixture defaults to type: 'agent'.

const agentConnection = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'agent:host-1',
  type: 'agent',
  name: 'host-1',
  address: 'host-1.lab',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['host'],
  scope: { host: true },
  lastSeen: '2026-07-20T09:00:00Z',
  lastError: null,
  source: 'agent',
  agentVersion: '6.1.0',
  expectedAgentVersion: '6.2.0',
  agentUpdateAvailable: true,
  agentIdentity: { hostname: 'host-1', platform: 'linux', architecture: 'amd64' },
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
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
  const primary = agentConnection();
  return {
    id: primary.id,
    ownerType: 'agent',
    name: primary.name,
    subtitle: 'via Pulse Agent',
    source: 'agent',
    host: primary.address,
    coverageLabels: ['Host telemetry'],
    statusLabel: 'Active',
    statusClassName: 'bg-green-100',
    agentUpdateCount: 0,
    lastActivityText: '1m ago',
    ...emptyFleetRow,
    enabled: true,
    canEdit: false,
    canPause: false,
    canRemove: true,
    isAgent: true,
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
  statusClassName: 'bg-green-100',
  lastActivityText: '1m ago',
  ...emptyFleetMember,
  primary: true,
  ...overrides,
});

const diagnostic = (
  overrides: Partial<AgentFleetAgentDiagnostic> = {},
): AgentFleetAgentDiagnostic => ({
  connectionId: 'agent:host-1',
  rowKey: 'host-1',
  id: 'host-1',
  agentId: 'host-1',
  name: 'host-1',
  hostname: 'host-1',
  types: ['host'],
  status: 'warning',
  version: '6.1.0',
  profileId: 'profile-linux',
  profileName: 'Linux servers',
  profileVersion: 4,
  deployedProfileVersion: 3,
  reasons: [
    {
      code: 'agent_version_stale',
      severity: 'warning',
      message: 'Agent is behind the supported server version.',
    },
  ],
  repairActions: [
    {
      code: 'copy_upgrade_command',
      label: 'Copy upgrade command',
      description: 'Run on the affected host.',
      supported: true,
      scope: 'local_admin_shell',
    },
  ],
  ...overrides,
});

// Run the doctor collector in the common "ledger fallback" shape: bindings are
// derived purely from the `connections` ledger (no rows), so each binding
// carries contextLabel 'Machine' and installFlags [].
const runDoctor = (
  agents: readonly Connection[],
  options: Partial<InfrastructureAgentDoctorOptions> = {},
) =>
  collectInfrastructureAgentDoctorTargets({
    rows: [],
    connections: agents,
    diagnosticsAvailable: false,
    ...options,
  });

// ---- resolveKnownAgentCommandPlatform ---------------------------------------

describe('resolveKnownAgentCommandPlatform', () => {
  it('fails closed (null) for missing input', () => {
    expect(resolveKnownAgentCommandPlatform(undefined)).toBeNull();
    expect(resolveKnownAgentCommandPlatform(null)).toBeNull();
  });

  it('fails closed (null) for blank input', () => {
    expect(resolveKnownAgentCommandPlatform('')).toBeNull();
    expect(resolveKnownAgentCommandPlatform('    ')).toBeNull();
  });

  it('fails closed (null) for an unrecognized caption', () => {
    expect(resolveKnownAgentCommandPlatform('Haiku')).toBeNull();
    expect(resolveKnownAgentCommandPlatform('plan9')).toBeNull();
  });

  it.each([
    ['windows', 'windows'],
    ['Windows 11 Pro', 'windows'],
    ['Microsoft Windows Server', 'windows'],
    ['WINDOWS', 'windows'],
  ])('classifies %s as windows', (caption, expected) => {
    expect(resolveKnownAgentCommandPlatform(caption)).toBe(expected);
  });

  it.each([
    ['darwin', 'macos'],
    ['mac', 'macos'],
    ['macos', 'macos'],
    ['Mac OS X', 'macos'],
    ['OS X Yosemite', 'macos'],
    ['mac os sonoma', 'macos'],
  ])('classifies %s as macos', (caption, expected) => {
    expect(resolveKnownAgentCommandPlatform(caption)).toBe(expected);
  });

  it.each([
    ['freebsd', 'freebsd'],
    ['FreeBSD 14.1', 'freebsd'],
    ['pfSense', 'freebsd'],
    ['pfSense Community Edition', 'freebsd'],
    ['OPNsense', 'freebsd'],
    ['opnsense 24.7', 'freebsd'],
  ])('classifies %s as freebsd', (caption, expected) => {
    expect(resolveKnownAgentCommandPlatform(caption)).toBe(expected);
  });

  it('classifies the canonical linux runtime family', () => {
    expect(resolveKnownAgentCommandPlatform('linux')).toBe('linux');
    expect(resolveKnownAgentCommandPlatform('LINUX')).toBe('linux');
  });

  it('classifies legacy Linux distro identifiers without a distro allowlist', () => {
    expect(resolveKnownAgentCommandPlatform('mageia')).toBe('linux');
    expect(resolveKnownAgentCommandPlatform('ubuntu')).toBe('linux');
    expect(resolveKnownAgentCommandPlatform('ubuntufork')).toBe('linux');
  });
});

// ---- normalizeAgentConnectionID ---------------------------------------------

describe('normalizeAgentConnectionID', () => {
  it('returns "" for null/undefined', () => {
    expect(normalizeAgentConnectionID(null)).toBe('');
    expect(normalizeAgentConnectionID(undefined)).toBe('');
  });

  it('returns "" for blank strings', () => {
    expect(normalizeAgentConnectionID('')).toBe('');
    expect(normalizeAgentConnectionID('   ')).toBe('');
  });

  it('keeps an already-prefixed id (after trimming)', () => {
    expect(normalizeAgentConnectionID('agent:foo')).toBe('agent:foo');
    expect(normalizeAgentConnectionID('  agent:foo  ')).toBe('agent:foo');
  });

  it('prefixes a bare id with agent:', () => {
    expect(normalizeAgentConnectionID('foo')).toBe('agent:foo');
    expect(normalizeAgentConnectionID('  foo  ')).toBe('agent:foo');
  });
});

// ---- diagnosticConnectionID -------------------------------------------------

describe('diagnosticConnectionID', () => {
  it('returns the explicit connectionId when present', () => {
    expect(diagnosticConnectionID(diagnostic({ connectionId: 'agent:explicit' }))).toBe(
      'agent:explicit',
    );
  });

  it('trims whitespace around the explicit connectionId', () => {
    expect(diagnosticConnectionID(diagnostic({ connectionId: '  agent:trim  ' }))).toBe(
      'agent:trim',
    );
  });

  it('falls back through agentId when connectionId is whitespace-only', () => {
    expect(
      diagnosticConnectionID(diagnostic({ connectionId: '   ', agentId: 'host-9', id: 'legacy' })),
    ).toBe('agent:host-9');
  });

  it('falls back through agentId when connectionId is undefined', () => {
    expect(
      diagnosticConnectionID(
        diagnostic({ connectionId: undefined, agentId: 'host-9', id: 'legacy' }),
      ),
    ).toBe('agent:host-9');
  });

  it('falls back through id (prefixed) when both connectionId and agentId are absent', () => {
    expect(
      diagnosticConnectionID(
        diagnostic({ connectionId: undefined, agentId: undefined, id: 'legacy-id' }),
      ),
    ).toBe('agent:legacy-id');
  });

  it('prefers agentId over id when both are present and connectionId is absent', () => {
    expect(
      diagnosticConnectionID(
        diagnostic({ connectionId: undefined, agentId: 'agent-1', id: 'row-1' }),
      ),
    ).toBe('agent:agent-1');
  });
});

// ---- collectInfrastructureAgentUpdateTargets --------------------------------

describe('collectInfrastructureAgentUpdateTargets', () => {
  // A non-agent (pve) primary so only attached agents become targets.
  const pvePrimary = agentConnection({
    id: 'pve:node',
    type: 'pve',
    name: 'node',
    agentUpdateAvailable: false,
  });
  const pveRow = (overrides: Partial<InfrastructureSystemRow> = {}) =>
    row({
      id: 'pve:node',
      ownerType: 'pve',
      isAgent: false,
      name: 'node',
      connection: pvePrimary,
      ...overrides,
    });

  it('returns [] for empty rows', () => {
    expect(collectInfrastructureAgentUpdateTargets([])).toEqual([]);
  });

  it('returns [] when rows contain only non-agent connections', () => {
    expect(collectInfrastructureAgentUpdateTargets([pveRow()])).toEqual([]);
  });

  it('sorts targets by displayName ascending', () => {
    const zeta = agentConnection({
      id: 'agent:zeta',
      name: 'zeta',
      agentIdentity: { hostname: 'zeta', platform: 'linux' },
    });
    const alpha = agentConnection({
      id: 'agent:alpha',
      name: 'alpha',
      agentIdentity: { hostname: 'alpha', platform: 'linux' },
    });
    const targets = collectInfrastructureAgentUpdateTargets([
      pveRow({ attachedConnections: [zeta, alpha] }),
    ]);
    expect(targets.map((target) => target.key)).toEqual(['agent:alpha', 'agent:zeta']);
  });

  it('retains all targets when no scope is provided', () => {
    const a = agentConnection({
      id: 'agent:a',
      name: 'a',
      agentIdentity: { hostname: 'a', platform: 'linux' },
    });
    const b = agentConnection({
      id: 'agent:b',
      name: 'b',
      agentIdentity: { hostname: 'b', platform: 'linux' },
    });
    const targets = collectInfrastructureAgentUpdateTargets([
      pveRow({ attachedConnections: [a, b] }),
    ]);
    expect(targets).toHaveLength(2);
  });
});

// ---- collectInfrastructureAgentDoctorTargets: top-level shaping -------------

describe('collectInfrastructureAgentDoctorTargets (top-level)', () => {
  it('returns [] for fully empty inputs using all defaults', () => {
    // rows required; connections/diagnostics/scopedAgentIds default to [].
    expect(
      collectInfrastructureAgentDoctorTargets({
        rows: [],
        diagnosticsAvailable: false,
      }),
    ).toEqual([]);
  });

  it('skips non-agent connections in the ledger (binding type guard)', () => {
    const pve = { ...agentConnection(), type: 'pve' as const, id: 'pve:node' };
    expect(runDoctor([pve])).toEqual([]);
  });

  it('dedupes a connection repeated in the ledger (binding has() guard)', () => {
    const agent = agentConnection();
    expect(runDoctor([agent, agent])).toHaveLength(1);
  });

  it('binds a ledger-only connection with contextLabel "Machine" and empty flags', () => {
    const [target] = runDoctor([agentConnection()]);
    expect(target?.contextLabel).toBe('Machine');
    expect(target?.installFlags).toEqual([]);
  });

  it('excludes removed diagnostics from the binding map (no live target built)', () => {
    const removed = diagnostic({
      connectionId: 'agent:host-1',
      status: 'removed',
    });
    // diagnosticsAvailable true + removed diagnostic + matching connection.
    // The removed diagnostic is filtered out of the binding map; the connection
    // still binds and is classified via the live path (source ledger-fallback
    // because no live diagnostic matches).
    const [target] = runDoctor([agentConnection({ agentUpdateAvailable: false })], {
      diagnostics: [removed],
      diagnosticsAvailable: true,
    });
    expect(target?.status).not.toBe('removed');
  });

  it('filters bindings by scopedAgentIds', () => {
    const inScope = agentConnection({ id: 'agent:in', name: 'in' });
    const outScope = agentConnection({ id: 'agent:out', name: 'out' });
    const targets = runDoctor([inScope, outScope], {
      scopedAgentIds: ['agent:in'],
    });
    expect(targets.map((target) => target.connectionId)).toEqual(['agent:in']);
  });

  it('sorts by status rank desc then displayName asc', () => {
    const critical = agentConnection({
      id: 'agent:c',
      name: 'c-critical',
      state: 'unauthorized',
      stateReason: 'bad token',
      agentUpdateAvailable: false,
    });
    const warning = agentConnection({
      id: 'agent:w',
      name: 'w-warning',
      state: 'paused',
      agentUpdateAvailable: false,
    });
    const healthy = agentConnection({
      id: 'agent:h',
      name: 'h-healthy',
      agentUpdateAvailable: false,
      agentVersion: '6.2.0',
      expectedAgentVersion: '6.2.0',
      fleet: { versionDrift: 'current' } as Connection['fleet'],
    });
    const targets = runDoctor([healthy, warning, critical]);
    expect(targets.map((target) => target.status)).toEqual(['critical', 'warning', 'healthy']);
  });

  it('breaks status-rank ties by displayName ascending', () => {
    const zeta = agentConnection({
      id: 'agent:zeta',
      name: 'zeta',
      state: 'paused',
      agentUpdateAvailable: false,
      agentIdentity: { hostname: 'zeta', platform: 'linux' },
    });
    const alpha = agentConnection({
      id: 'agent:alpha',
      name: 'alpha',
      state: 'paused',
      agentUpdateAvailable: false,
      agentIdentity: { hostname: 'alpha', platform: 'linux' },
    });
    const targets = runDoctor([zeta, alpha]);
    expect(targets.map((target) => target.displayName)).toEqual(['alpha', 'zeta']);
  });
});

// ---- collectInfrastructureAgentDoctorTargets: removed records ---------------

describe('collectInfrastructureAgentDoctorTargets (removed records)', () => {
  const removedDiag = (overrides: Partial<AgentFleetAgentDiagnostic> = {}) =>
    diagnostic({
      connectionId: 'agent:gone',
      rowKey: 'gone-gone',
      id: 'gone',
      name: 'gone',
      hostname: 'gone-host',
      status: 'removed',
      reasons: [],
      repairActions: [],
      ...overrides,
    });

  it('adds removed targets only when diagnostics are available and unscoped', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    const base = {
      rows: [],
      connections: [live],
      diagnostics: [removedDiag()],
      diagnosticsAvailable: true,
    } as const;

    expect(
      collectInfrastructureAgentDoctorTargets(base)
        .map((target) => target.status)
        .includes('removed'),
    ).toBe(true);

    // Scoped drilldown suppresses removed records.
    expect(
      collectInfrastructureAgentDoctorTargets({ ...base, scopedAgentIds: ['agent:host-1'] })
        .map((target) => target.status)
        .includes('removed'),
    ).toBe(false);
  });

  it('suppresses removed targets when diagnostics are unavailable', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    const targets = runDoctor([live], {
      diagnostics: [removedDiag()],
      diagnosticsAvailable: false,
    });
    expect(targets.map((target) => target.status).includes('removed')).toBe(false);
  });

  it('derives displayName from name, then hostname, then id', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    const withName = runDoctor([live], {
      diagnostics: [removedDiag({ name: 'gone', hostname: 'ghost', id: 'x' })],
      diagnosticsAvailable: true,
    });
    expect(withName.find((target) => target.status === 'removed')?.displayName).toBe('gone');

    const noName = runDoctor([live], {
      diagnostics: [removedDiag({ name: '', hostname: 'ghost', id: 'x' })],
      diagnosticsAvailable: true,
    });
    expect(noName.find((target) => target.status === 'removed')?.displayName).toBe('ghost');

    const nothing = runDoctor([live], {
      diagnostics: [removedDiag({ name: '', hostname: undefined, id: 'only-id' })],
      diagnosticsAvailable: true,
    });
    expect(nothing.find((target) => target.status === 'removed')?.displayName).toBe('only-id');
  });

  it('derives contextLabel from labelled types joined by " + ", falling back to "Removed agent"', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    const typed = runDoctor([live], {
      diagnostics: [removedDiag({ types: ['host', 'docker'] })],
      diagnosticsAvailable: true,
    });
    expect(typed.find((target) => target.status === 'removed')?.contextLabel).toBe(
      'Host + Docker agent',
    );

    const untyped = runDoctor([live], {
      diagnostics: [removedDiag({ types: [] })],
      diagnosticsAvailable: true,
    });
    expect(untyped.find((target) => target.status === 'removed')?.contextLabel).toBe(
      'Removed agent',
    );
  });

  it('carries currentVersion only when the diagnostic version is present', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    const withVersion = runDoctor([live], {
      diagnostics: [removedDiag({ version: '5.0.0' })],
      diagnosticsAvailable: true,
    });
    expect(withVersion.find((target) => target.status === 'removed')?.currentVersion).toBe('5.0.0');

    const noVersion = runDoctor([live], {
      diagnostics: [removedDiag({ version: undefined })],
      diagnosticsAvailable: true,
    });
    expect(noVersion.find((target) => target.status === 'removed')?.currentVersion).toBe(undefined);
  });

  it('omits profileVersionLabel and derives profileLabel for a removed record', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    // profileVersion undefined exercises the `: undefined` arm of the
    // removedDoctorTarget profileVersionLabel ternary.
    const withProfile = runDoctor([live], {
      diagnostics: [removedDiag({ profileName: 'Drifted', profileVersion: undefined })],
      diagnosticsAvailable: true,
    });
    const target = withProfile.find((candidate) => candidate.status === 'removed');
    expect(target?.profileVersionLabel).toBeUndefined();
    expect(target?.profileLabel).toBe('Drifted');

    const noProfile = runDoctor([live], {
      diagnostics: [removedDiag({ profileName: '', profileId: '', profileVersion: undefined })],
      diagnosticsAvailable: true,
    });
    expect(noProfile.find((candidate) => candidate.status === 'removed')?.profileLabel).toBe(
      undefined,
    );
  });
});

// ---- collectInfrastructureAgentDoctorTargets: ledger-fallback status --------

describe('collectInfrastructureAgentDoctorTargets (fallbackStatus via ledger)', () => {
  it('classifies a clean current agent as healthy', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        fleet: { versionDrift: 'current' } as Connection['fleet'],
      }),
    ]);
    expect(target?.status).toBe('healthy');
    expect(target?.source).toBe('ledger-fallback');
    expect(target?.commandBlockedReason).toBeUndefined();
  });

  it('classifies an active agent without versionDrift as unknown', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
      }),
    ]);
    expect(target?.status).toBe('unknown');
  });

  it('classifies an unauthorized agent as critical (state reason w/ evidence)', () => {
    const [target] = runDoctor([
      agentConnection({
        state: 'unauthorized',
        stateReason: 'token rejected',
        agentUpdateAvailable: false,
      }),
    ]);
    expect(target?.status).toBe('critical');
    expect(target?.reasons.map((reason) => reason.code)).toContain('ledger_unauthorized');
    expect(target?.reasons.find((r) => r.code === 'ledger_unauthorized')?.evidence).toEqual([
      'token rejected',
    ]);
  });

  it('classifies an unreachable agent as critical (no state reason -> empty evidence)', () => {
    const [target] = runDoctor([
      agentConnection({
        state: 'unreachable',
        stateReason: '',
        agentUpdateAvailable: false,
      }),
    ]);
    expect(target?.status).toBe('critical');
    const reason = target?.reasons.find((r) => r.code === 'ledger_unreachable');
    expect(reason?.evidence).toEqual([]);
  });
});

// ---- collectInfrastructureAgentDoctorTargets: ledgerFallbackReasons ---------

describe('collectInfrastructureAgentDoctorTargets (ledgerFallbackReasons state arms)', () => {
  it('emits ledger_stale with evidence from stateReason', () => {
    const [target] = runDoctor([
      agentConnection({
        state: 'stale',
        stateReason: 'no heartbeat',
        agentUpdateAvailable: false,
      }),
    ]);
    const reason = target?.reasons.find((r) => r.code === 'ledger_stale');
    expect(reason?.severity).toBe('warning');
    expect(reason?.evidence).toEqual(['no heartbeat']);
  });

  it('emits ledger_stale with empty evidence when stateReason is blank', () => {
    const [target] = runDoctor([
      agentConnection({ state: 'stale', stateReason: '', agentUpdateAvailable: false }),
    ]);
    expect(target?.reasons.find((r) => r.code === 'ledger_stale')?.evidence).toEqual([]);
  });

  it('emits ledger_pending (no evidence variant)', () => {
    const [target] = runDoctor([
      agentConnection({ state: 'pending', agentUpdateAvailable: false }),
    ]);
    const reason = target?.reasons.find((r) => r.code === 'ledger_pending');
    expect(reason?.severity).toBe('warning');
    // fallbackReason() defaults evidence to [] (not undefined).
    expect(reason?.evidence).toEqual([]);
  });

  it('emits ledger_paused', () => {
    const [target] = runDoctor([agentConnection({ state: 'paused', agentUpdateAvailable: false })]);
    expect(target?.reasons.map((r) => r.code)).toContain('ledger_paused');
  });

  it('emits no state reason for an active connection', () => {
    const [target] = runDoctor([
      agentConnection({
        state: 'active',
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
      }),
    ]);
    const stateReasons = target?.reasons
      .filter((r) => r.code.startsWith('ledger_'))
      .map((r) => r.code);
    expect(stateReasons).toEqual([]);
  });

  it('emits ledger_update_error with lastError evidence when agentUpdate state is error', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        agentUpdate: {
          state: 'error',
          autoUpdate: false,
          lastError: 'install failed',
        },
      }),
    ]);
    const reason = target?.reasons.find((r) => r.code === 'ledger_update_error');
    expect(reason?.evidence).toEqual(['install failed']);
  });

  it('emits ledger_update_error with empty evidence when lastError is absent', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        agentUpdate: { state: 'error', autoUpdate: false },
      }),
    ]);
    expect(target?.reasons.find((r) => r.code === 'ledger_update_error')?.evidence).toEqual([]);
  });

  it('emits ledger_module_degraded for an enabled, non-running module', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        agentModules: [
          {
            name: 'docker',
            enabled: true,
            state: 'starting',
            lastError: 'startup stalled',
            updatedAt: '2026-07-20T00:00:00Z',
          },
        ],
      }),
    ]);
    const reason = target?.reasons.find((r) => r.code === 'ledger_module_degraded');
    expect(reason?.message).toBe('docker is enabled but starting.');
    expect(reason?.evidence).toEqual(['startup stalled']);
  });

  it('skips modules that are disabled or already running', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        agentModules: [
          { name: 'host', enabled: false, state: 'disabled', updatedAt: '2026-07-20T00:00:00Z' },
          { name: 'docker', enabled: true, state: 'running', updatedAt: '2026-07-20T00:00:00Z' },
        ],
      }),
    ]);
    expect(target?.reasons.find((r) => r.code === 'ledger_module_degraded')).toBeUndefined();
  });

  it('appends agent_version_stale when the agent needs an update', () => {
    const [target] = runDoctor([
      agentConnection({
        state: 'active',
        agentUpdateAvailable: true,
        expectedAgentVersion: '6.2.0',
      }),
    ]);
    expect(target?.reasons.map((r) => r.code)).toContain('agent_version_stale');
  });
});

// ---- collectInfrastructureAgentDoctorTargets: diagnostics path --------------

describe('collectInfrastructureAgentDoctorTargets (diagnostics path)', () => {
  it('uses diagnostic.status and diagnostic.reasons (source diagnostics)', () => {
    const [target] = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ status: 'critical' })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });
    expect(target?.source).toBe('diagnostics');
    expect(target?.status).toBe('critical');
    expect(target?.reasons[0].code).toBe('agent_version_stale');
  });

  it('upgrades a healthy diagnostic to warning when fallback reasons exist', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          state: 'stale',
          stateReason: 'no heartbeat',
        }),
      ],
      {
        diagnostics: [diagnostic({ status: 'healthy' })],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    // diagnostic.status healthy, but ledger fallback reasons (ledger_stale +
    // agent_version_stale) are non-empty -> upgraded to warning.
    expect(target?.status).toBe('warning');
  });

  it('reclassifies to waiting when the updater is handling it asynchronously', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          state: 'active',
          agentUpdate: {
            state: 'checking',
            autoUpdate: true,
            lastCheckedAt: '2026-07-20T09:01:00Z',
          },
        }),
      ],
      {
        diagnostics: [diagnostic({ status: 'warning' })],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    // reasons only contain agent_version_stale -> nonVersionReasons empty;
    // active + updater.waiting + not critical -> waiting.
    expect(target?.status).toBe('waiting');
    expect(target?.updaterLabel).toBe('Checking for an automatic update');
  });

  it('keeps the original status when a non-version reason blocks the waiting reclass', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          state: 'active',
          agentUpdate: { state: 'checking', autoUpdate: true },
        }),
      ],
      {
        diagnostics: [
          diagnostic({
            status: 'warning',
            reasons: [
              { code: 'agent_version_stale', severity: 'warning', message: 'stale' },
              { code: 'module_down', severity: 'warning', message: 'module down' },
            ],
          }),
        ],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    expect(target?.status).toBe('warning');
  });

  it('falls back to the ledger path when diagnostics are available but no diagnostic matches', () => {
    const [target] = runDoctor(
      [agentConnection({ id: 'agent:other', agentUpdateAvailable: false })],
      {
        diagnostics: [diagnostic({ connectionId: 'agent:host-1' })],
        diagnosticsAvailable: true,
      },
    );
    expect(target?.source).toBe('ledger-fallback');
  });
});

// ---- collectInfrastructureAgentDoctorTargets: commandBlockedReason ----------

describe('collectInfrastructureAgentDoctorTargets (commandBlockedReason arms)', () => {
  it('blocks when an update is needed but no expected version is available', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: true,
        expectedAgentVersion: undefined,
      }),
    ]);
    // targetVersion undefined -> expectedVersion undefined -> arm 1.
    expect(target?.expectedVersion).toBeUndefined();
    expect(target?.commandBlockedReason).toContain('No supported target version');
  });

  it('blocks when the expected version is not a supported release', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: true,
        expectedAgentVersion: 'not-a-version',
      }),
    ]);
    expect(target?.commandBlockedReason).toContain('not a supported release version');
  });

  it('blocks when the agent did not report a recognized platform', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: true,
        expectedAgentVersion: '6.2.0',
        agentIdentity: { hostname: 'host-1', platform: 'haiku' },
      }),
    ]);
    expect(target?.commandPlatform).toBeNull();
    expect(target?.commandBlockedReason).toContain('will not guess');
  });

  it('blocks FreeBSD/pfSense until installer state is proven', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: true,
        expectedAgentVersion: '6.2.0',
        agentIdentity: { hostname: 'fw', platform: 'pfSense', architecture: 'amd64' },
      }),
    ]);
    expect(target?.commandPlatform).toBe('freebsd');
    expect(target?.commandBlockedReason).toContain('cannot verify saved FreeBSD');
  });

  it('blocks while the updater is actively applying an update', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          state: 'active',
          agentUpdateAvailable: true,
          expectedAgentVersion: '6.2.0',
          agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
          agentUpdate: {
            state: 'updating',
            autoUpdate: true,
            lastAttemptAt: '2026-07-13T09:01:00Z',
          },
        }),
      ],
      { nowMs: Date.parse('2026-07-13T09:03:00Z') },
    );
    expect(target?.commandBlockedReason).toContain('applying an update right now');
  });

  it('keeps the manual command available while the updater merely waits', () => {
    const [target] = runDoctor([
      agentConnection({
        state: 'active',
        agentUpdateAvailable: true,
        expectedAgentVersion: '6.2.0',
        agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
        agentUpdate: { state: 'checking', autoUpdate: true },
      }),
    ]);
    expect(target?.commandBlockedReason).toBeUndefined();
  });

  it('blocks when diagnostics offer no supported structured upgrade action', () => {
    const [target] = runDoctor([agentConnection()], {
      diagnostics: [
        diagnostic({
          repairActions: [
            { code: 'restart_agent', label: 'Restart', description: '...', supported: true },
          ],
        }),
      ],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });
    expect(target?.commandBlockedReason).toContain('did not offer a supported update repair');
  });

  it('also blocks when the structured upgrade action is present but unsupported', () => {
    const [target] = runDoctor([agentConnection()], {
      diagnostics: [
        diagnostic({
          repairActions: [
            {
              code: 'copy_upgrade_command',
              label: 'Copy',
              description: '...',
              supported: false,
            },
          ],
        }),
      ],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });
    expect(target?.commandBlockedReason).toContain('did not offer a supported update repair');
  });

  it('does not block when a supported structured upgrade action is available', () => {
    const [target] = runDoctor([agentConnection()], {
      diagnostics: [diagnostic()],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });
    expect(target?.commandPlatform).toBe('linux');
    expect(target?.commandBlockedReason).toBeUndefined();
  });

  it('does not block when the agent does not need an update', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
      }),
    ]);
    expect(target?.needsUpdate).toBe(false);
    expect(target?.commandBlockedReason).toBeUndefined();
  });
});

// ---- collectInfrastructureAgentDoctorTargets: updaterPresentation -----------

describe('collectInfrastructureAgentDoctorTargets (updaterPresentation arms)', () => {
  const updaterFor = (state: ConnectionAgentUpdateStatus['state'], autoUpdate: boolean) =>
    runDoctor(
      [
        agentConnection({
          agentUpdateAvailable: true,
          expectedAgentVersion: '6.2.0',
          agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
          agentUpdate: { state, autoUpdate },
        }),
      ],
      // Use diagnostics with a supported upgrade action so commandBlockedReason
      // doesn't interfere with status / label assertions.
      {
        diagnostics: [diagnostic()],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    )[0]?.updaterLabel;

  it('returns no label when the connection has no agentUpdate', () => {
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        agentUpdate: undefined,
      }),
    ]);
    expect(target?.updaterLabel).toBeUndefined();
  });

  it.each([
    ['updating', 'Updating automatically'],
    ['disabled', 'Automatic updates disabled'],
    ['error', 'Last update attempt failed'],
  ] as const)('labels %s state', (state, expected) => {
    expect(updaterFor(state, false)).toBe(expected);
  });

  it('labels checking + autoUpdate as "Checking for an automatic update"', () => {
    expect(updaterFor('checking', true)).toBe('Checking for an automatic update');
  });

  it('labels checking + manual as "Checking for an update"', () => {
    expect(updaterFor('checking', false)).toBe('Checking for an update');
  });

  it('labels update-available + autoUpdate as "Update queued automatically"', () => {
    expect(updaterFor('update-available', true)).toBe('Update queued automatically');
  });

  it('labels update-available + manual as "Update available; manual action required"', () => {
    expect(updaterFor('update-available', false)).toBe('Update available; manual action required');
  });

  it('labels idle + needsUpdate + autoUpdate as "Waiting for the next automatic check"', () => {
    expect(updaterFor('idle', true)).toBe('Waiting for the next automatic check');
  });

  it('labels idle + current + autoUpdate as "Automatic updates ready"', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          agentUpdateAvailable: false,
          agentVersion: '6.2.0',
          expectedAgentVersion: '6.2.0',
          agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
          agentUpdate: { state: 'idle', autoUpdate: true },
        }),
      ],
      {
        diagnostics: [diagnostic()],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    expect(target?.updaterLabel).toBe('Automatic updates ready');
  });

  it('labels idle + manual as "Manual updates only"', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          agentUpdateAvailable: false,
          agentVersion: '6.2.0',
          expectedAgentVersion: '6.2.0',
          agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
          agentUpdate: { state: 'idle', autoUpdate: false },
        }),
      ],
      {
        diagnostics: [diagnostic()],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    expect(target?.updaterLabel).toBe('Manual updates only');
  });

  it('surfaces an unknown updater state as "Updater state: <state>"', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          agentUpdateAvailable: false,
          agentVersion: '6.2.0',
          expectedAgentVersion: '6.2.0',
          agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
          agentUpdate: {
            state: 'syncing',
            autoUpdate: false,
          } as unknown as ConnectionAgentUpdateStatus,
        }),
      ],
      {
        diagnostics: [diagnostic()],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    expect(target?.updaterLabel).toBe('Updater state: syncing');
  });

  it('returns no label for a blank updater state', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          agentUpdateAvailable: false,
          agentVersion: '6.2.0',
          expectedAgentVersion: '6.2.0',
          agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
          agentUpdate: {
            state: '   ',
            autoUpdate: false,
          } as unknown as ConnectionAgentUpdateStatus,
        }),
      ],
      {
        diagnostics: [diagnostic()],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );
    expect(target?.updaterLabel).toBeUndefined();
  });
});

// ---- collectInfrastructureAgentDoctorTargets: evidence & profile -----------

describe('collectInfrastructureAgentDoctorTargets (evidence + profile labels)', () => {
  it('aggregates connection, identity, updater, and diagnostic evidence', () => {
    const [target] = runDoctor(
      [
        agentConnection({
          agentIdentity: {
            hostname: 'host-1',
            platform: 'ubuntu',
            architecture: 'amd64',
            reportIp: '10.0.0.5',
          },
          agentUpdate: {
            state: 'idle',
            autoUpdate: true,
            lastCheckedAt: '2026-07-20T01:00:00Z',
            lastAttemptAt: '2026-07-20T02:00:00Z',
            lastSuccessAt: '2026-07-20T03:00:00Z',
          },
        }),
      ],
      {
        diagnostics: [
          diagnostic({
            machineIdFingerprint: 'machine-abc',
            interfaceAddresses: ['10.0.0.6', '10.0.0.7'],
            reasons: [
              {
                code: 'agent_version_stale',
                severity: 'warning',
                message: 'stale',
                evidence: ['Reported v6.1.0; target v6.2.0'],
              },
            ],
          }),
        ],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );

    expect(target?.evidence).toEqual(
      expect.arrayContaining([
        'Connection: agent:host-1',
        'Hostname: host-1',
        'Platform: ubuntu / amd64',
        'Reported IP: 10.0.0.5',
        'Last updater check: 2026-07-20T01:00:00Z',
        'Last update attempt: 2026-07-20T02:00:00Z',
        'Last successful update: 2026-07-20T03:00:00Z',
        'Machine identity: machine-abc',
        'Reported interface: 10.0.0.6',
        'Reported interface: 10.0.0.7',
        'Reported v6.1.0; target v6.2.0',
      ]),
    );
  });

  it('prefers profileName for profileLabel, falling back to profileId then undefined', () => {
    const withName = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ profileName: 'Linux servers', profileId: 'pid' })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(withName?.profileLabel).toBe('Linux servers');

    const withIdOnly = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ profileName: '', profileId: 'pid' })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(withIdOnly?.profileLabel).toBe('pid');

    const none = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ profileName: '', profileId: '' })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(none?.profileLabel).toBeUndefined();
  });

  it('builds profileVersionLabel when profileVersion is present (defaulting deployed to 0)', () => {
    const withDeployed = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ profileVersion: 4, deployedProfileVersion: 3 })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(withDeployed?.profileVersionLabel).toBe('Expected v4 · deployed v3');

    const noDeployed = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ profileVersion: 4, deployedProfileVersion: undefined })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(noDeployed?.profileVersionLabel).toBe('Expected v4 · deployed v0');

    const absent = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ profileVersion: undefined })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(absent?.profileVersionLabel).toBeUndefined();
  });

  it('uses connection.lastSeen, falling back to diagnostic.lastSeen', () => {
    const fromConnection = runDoctor([agentConnection({ lastSeen: '2026-07-20T00:00:00Z' })])[0];
    expect(fromConnection?.lastSeen).toBe('2026-07-20T00:00:00Z');

    const fromDiagnostic = runDoctor([agentConnection({ lastSeen: null })], {
      diagnostics: [diagnostic({ lastSeen: 1700000000 })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    })[0];
    expect(fromDiagnostic?.lastSeen).toBe(1700000000);
  });

  it('binds install flags from a host row when the agent is attached to it', () => {
    const agent = agentConnection({
      id: 'agent:docker-host',
      name: 'docker-host',
      agentIdentity: { hostname: 'docker-host', platform: 'linux' },
    });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [
        row({
          ownerType: 'docker',
          isAgent: false,
          name: 'docker-host',
          connection: agentConnection({ id: 'docker:host', type: 'docker', name: 'docker-host' }),
          attachedConnections: [agent],
        }),
      ],
      connections: [agent],
      diagnosticsAvailable: false,
    });
    const target = targets.find((candidate) => candidate.connectionId === 'agent:docker-host');
    expect(target?.installFlags).toEqual(['--enable-docker']);
  });

  it('binds member agent connections with the host row install flags', () => {
    const agent = agentConnection({
      id: 'agent:member',
      name: 'member',
      agentIdentity: { hostname: 'member', platform: 'linux' },
    });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [
        row({
          ownerType: 'pve',
          isAgent: false,
          name: 'cluster',
          connection: agentConnection({ id: 'pve:cluster', type: 'pve', name: 'cluster' }),
          attachedConnections: [],
          members: [member({ agentConnection: agent })],
        }),
      ],
      connections: [agent],
      diagnosticsAvailable: false,
    });
    const target = targets.find((candidate) => candidate.connectionId === 'agent:member');
    expect(target?.installFlags).toEqual(['--enable-proxmox', '--proxmox-type pve']);
  });

  it('skips a member whose agentConnection is undefined', () => {
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [
        row({
          ownerType: 'pve',
          isAgent: false,
          name: 'cluster',
          connection: agentConnection({ id: 'pve:cluster', type: 'pve', name: 'cluster' }),
          members: [member({ agentConnection: undefined })],
        }),
      ],
      connections: [],
      diagnosticsAvailable: false,
    });
    expect(targets).toEqual([]);
  });
});

// ---- summarizeInfrastructureAgentDoctorTargets ------------------------------

describe('summarizeInfrastructureAgentDoctorTargets', () => {
  const target = (status: InfrastructureAgentDoctorStatus): InfrastructureAgentDoctorTarget =>
    ({ status }) as unknown as InfrastructureAgentDoctorTarget;

  it('returns all-zero counts for an empty list', () => {
    expect(summarizeInfrastructureAgentDoctorTargets([])).toEqual({
      total: 0,
      healthy: 0,
      waiting: 0,
      warning: 0,
      critical: 0,
      unknown: 0,
      removed: 0,
    });
  });

  it('counts each status arm independently', () => {
    const targets: InfrastructureAgentDoctorTarget[] = [
      target('healthy'),
      target('healthy'),
      target('waiting'),
      target('warning'),
      target('warning'),
      target('warning'),
      target('critical'),
      target('unknown'),
      target('removed'),
    ];
    expect(summarizeInfrastructureAgentDoctorTargets(targets)).toEqual({
      total: 9,
      healthy: 2,
      waiting: 1,
      warning: 3,
      critical: 1,
      unknown: 1,
      removed: 1,
    });
  });

  it('reflects the full live+removed pipeline end to end', () => {
    const live = agentConnection({ agentUpdateAvailable: false });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [],
      connections: [live],
      diagnostics: [diagnostic({ connectionId: 'agent:gone', status: 'removed', reasons: [] })],
      diagnosticsAvailable: true,
    });
    // One healthy/unknown ledger target + one removed record.
    expect(summarizeInfrastructureAgentDoctorTargets(targets).total).toBeGreaterThanOrEqual(2);
    expect(summarizeInfrastructureAgentDoctorTargets(targets).removed).toBe(1);
  });
});
