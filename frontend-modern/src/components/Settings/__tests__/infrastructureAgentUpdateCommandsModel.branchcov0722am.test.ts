import { describe, expect, it } from 'vitest';
import type { Connection } from '@/api/connections';
import type {
  AgentFleetAgentDiagnostic,
  AgentFleetDiagnosticReason,
  AgentFleetDiagnosticStatus,
} from '@/api/agentDiagnostics';
import type { InfrastructureSystemRow } from '../connectionsTableModel';
import {
  collectInfrastructureAgentDoctorTargets,
  collectInfrastructureAgentUpdateTargets,
} from '../infrastructureAgentUpdateCommandsModel';
import type { InfrastructureAgentDoctorOptions } from '../infrastructureAgentUpdateCommandsModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling infrastructureAgentUpdateCommandsModel.branchcov0720pm.test.ts
// factories so every module-private branch is driven through the two exported
// orchestrators (collectInfrastructureAgentUpdateTargets /
// collectInfrastructureAgentDoctorTargets). This file targets the arms V8 still
// reported as zero-hit after the three sibling suites ran.

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
  lastSeen: '2026-07-22T09:00:00Z',
  lastError: null,
  source: 'agent',
  agentVersion: '6.1.0',
  expectedAgentVersion: '6.2.0',
  agentUpdateAvailable: true,
  agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
  ...overrides,
});

const emptyFleetRow = {
  fleetSignals: [],
  fleetHighlights: [],
} satisfies Pick<InfrastructureSystemRow, 'fleetSignals' | 'fleetHighlights'>;

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

// A non-agent primary so collectInfrastructureAgentUpdateTargets only builds
// targets from attachedConnections (isolates pushTarget shaping).
const pvePrimaryRow = (overrides: Partial<InfrastructureSystemRow> = {}): InfrastructureSystemRow =>
  row({
    id: 'pve:node',
    ownerType: 'pve',
    isAgent: false,
    name: 'node',
    connection: agentConnection({
      id: 'pve:node',
      type: 'pve',
      name: 'node',
      agentUpdateAvailable: false,
    }),
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

// An orphan diagnostic: no ledger connection binds to its connectionId, so the
// doctor surfaces it via diagnosticOnlyDoctorTarget (the diagnostic-only path).
const orphanDiagnostic = (
  overrides: Partial<AgentFleetAgentDiagnostic> = {},
): AgentFleetAgentDiagnostic =>
  diagnostic({
    connectionId: 'agent:orphan',
    rowKey: 'orphan-row',
    id: 'orphan-id',
    agentId: 'orphan-id',
    name: 'orphan',
    hostname: 'orphan-host',
    status: 'warning',
    types: ['host'],
    ...overrides,
  });

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

// Run with only orphan diagnostics (no ledger bindings at all).
const runOrphans = (
  diagnostics: readonly AgentFleetAgentDiagnostic[],
  options: Partial<InfrastructureAgentDoctorOptions> = {},
) =>
  runDoctor([], {
    diagnostics,
    diagnosticsAvailable: true,
    ...options,
  });

// ---- pushTarget: currentVersion optional-chain fallback ---------------------

describe('collectInfrastructureAgentUpdateTargets (currentVersion fallback)', () => {
  it('reports undefined currentVersion when a pushed target has no agentVersion', () => {
    // agentUpdateAvailable short-circuits connectionNeedsUpdate so the agent is
    // still pushed; agentVersion undefined forces `agentVersion?.trim() ||
    // undefined` to its right operand.
    const agent = agentConnection({
      id: 'agent:no-version',
      name: 'no-version',
      agentVersion: undefined,
      agentUpdateAvailable: true,
      agentIdentity: { hostname: 'no-version', platform: 'linux' },
    });

    const [target] = collectInfrastructureAgentUpdateTargets([
      pvePrimaryRow({ attachedConnections: [agent] }),
    ]);

    expect(target?.key).toBe('agent:no-version');
    expect(target?.currentVersion).toBeUndefined();
  });
});

// ---- collectAgentConnectionBindings: integrationSource guard ----------------

describe('collectInfrastructureAgentDoctorTargets (integrationSource guard)', () => {
  it('excludes platform-integrated agents from the doctor bindings', () => {
    // An agent whose telemetry comes from a platform integration (vSphere,
    // TrueNAS, ...) has no Pulse Agent to diagnose; the binding guard returns
    // early so no target is produced.
    const integrated = agentConnection({
      id: 'agent:integrated',
      name: 'integrated',
      integrationSource: 'vmware',
      agentUpdateAvailable: false,
    });

    expect(runDoctor([integrated])).toEqual([]);
  });
});

// ---- ledgerFallbackReasons: state/module ternary arms -----------------------

describe('collectInfrastructureAgentDoctorTargets (ledgerFallbackReasons ternary arms)', () => {
  it('emits ledger_unauthorized with empty evidence when stateReason is blank', () => {
    // The sibling suite only exercises unauthorized with a stateReason; the
    // `stateReason ? [...] : []` alternate (blank reason) was unhit.
    const [target] = runDoctor([
      agentConnection({ state: 'unauthorized', stateReason: '', agentUpdateAvailable: false }),
    ]);

    expect(target?.status).toBe('critical');
    expect(target?.reasons.find((r) => r.code === 'ledger_unauthorized')?.evidence).toEqual([]);
  });

  it('emits ledger_unreachable with evidence when stateReason is present', () => {
    // The sibling suite only exercises unreachable with a blank stateReason;
    // the consequent `[stateReason]` arm was unhit.
    const [target] = runDoctor([
      agentConnection({
        state: 'unreachable',
        stateReason: 'dial tcp: connection refused',
        agentUpdateAvailable: false,
      }),
    ]);

    expect(target?.reasons.find((r) => r.code === 'ledger_unreachable')?.evidence).toEqual([
      'dial tcp: connection refused',
    ]);
  });

  it('emits ledger_module_degraded with empty evidence when lastError is absent', () => {
    // The sibling suite only exercises a degraded module carrying a lastError;
    // the `lastError ? [...] : []` alternate was unhit.
    const [target] = runDoctor([
      agentConnection({
        agentUpdateAvailable: false,
        agentVersion: '6.2.0',
        expectedAgentVersion: '6.2.0',
        agentModules: [
          { name: 'docker', enabled: true, state: 'retrying', updatedAt: '2026-07-22T00:00:00Z' },
        ],
      }),
    ]);

    const reason = target?.reasons.find((r) => r.code === 'ledger_module_degraded');
    expect(reason?.message).toBe('docker is enabled but retrying.');
    expect(reason?.evidence).toEqual([]);
  });
});

// ---- doctorTargetFromBinding: diagnostic.reasons fallback -------------------

describe('collectInfrastructureAgentDoctorTargets (matching diagnostic reasons fallback)', () => {
  it('defaults to an empty reasons list when a matching diagnostic omits reasons', () => {
    // diagnosticsAvailable + matching diagnostic selects the diagnostic branch;
    // a nullish reasons exercises `diagnostic.reasons ?? []`.
    const [target] = runDoctor([agentConnection()], {
      diagnostics: [diagnostic({ reasons: undefined as unknown as AgentFleetDiagnosticReason[] })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });

    expect(target?.source).toBe('diagnostics');
    expect(target?.reasons).toEqual([]);
  });
});

// ---- doctorTargetFromBinding: currentVersion from diagnostic ----------------

describe('collectInfrastructureAgentDoctorTargets (currentVersion from diagnostic)', () => {
  it('derives currentVersion from the diagnostic when the ledger version is absent', () => {
    // agentVersion undefined forces the `|| diagnostic?.version?.trim()` arm.
    const [target] = runDoctor(
      [
        agentConnection({
          agentVersion: undefined,
          agentUpdateAvailable: true,
          expectedAgentVersion: '6.2.0',
        }),
      ],
      {
        diagnostics: [diagnostic({ version: '6.1.5' })],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );

    expect(target?.currentVersion).toBe('6.1.5');
  });

  it('reports undefined currentVersion when neither ledger nor diagnostic carry a version', () => {
    // agentVersion undefined AND diagnostic.version undefined forces the final
    // `|| undefined` arm.
    const [target] = runDoctor(
      [
        agentConnection({
          agentVersion: undefined,
          agentUpdateAvailable: true,
          expectedAgentVersion: '6.2.0',
        }),
      ],
      {
        diagnostics: [diagnostic({ version: undefined })],
        diagnosticsAvailable: true,
        targetVersion: '6.2.0',
      },
    );

    expect(target?.currentVersion).toBeUndefined();
  });
});

// ---- diagnosticOnlyDoctorTarget: non-removed orphan shaping -----------------

describe('collectInfrastructureAgentDoctorTargets (diagnostic-only orphan shaping)', () => {
  it('classifies an out-of-set diagnostic status as unknown', () => {
    // diagnosticDoctorStatus fails closed to 'unknown' for any status outside
    // {healthy, warning, critical, removed}. The schema type is closed, so an
    // unexpected value is injected via cast (defensive resolver behaviour).
    const [target] = runOrphans([
      orphanDiagnostic({
        status: 'waiting' as unknown as AgentFleetDiagnosticStatus,
      }),
    ]);

    expect(target?.status).toBe('unknown');
    expect(target?.source).toBe('diagnostics');
  });

  it("falls back to the 'Agent' context label for a non-removed orphan with nullish types", () => {
    // Nullish types + non-removed exercises both the `types ?? []` fallback and
    // the `removed ? 'Removed agent' : 'Agent'` alternate arm.
    const [target] = runOrphans([orphanDiagnostic({ types: undefined as unknown as string[] })]);

    expect(target?.contextLabel).toBe('Agent');
  });

  it('falls back to the diagnostic id in the key when rowKey is blank', () => {
    // `diagnostic.rowKey || diagnostic.id` right operand.
    const [target] = runOrphans([orphanDiagnostic({ rowKey: '', id: 'fallback-id' })]);

    expect(target?.key).toBe('diagnostic:fallback-id');
  });

  it('passes an unrecognised diagnostic type through verbatim in the context label', () => {
    // `DIAGNOSTIC_TYPE_LABELS[type] ?? type` alternate arm.
    const [target] = runOrphans([orphanDiagnostic({ types: ['lxc'] })]);

    expect(target?.contextLabel).toBe('lxc agent');
  });

  it('defaults to an empty reasons list when the orphan diagnostic omits reasons', () => {
    // `diagnostic.reasons ?? []` alternate arm on the diagnostic-only path.
    const [target] = runOrphans([
      orphanDiagnostic({ reasons: undefined as unknown as AgentFleetDiagnosticReason[] }),
    ]);

    expect(target?.reasons).toEqual([]);
  });

  it('defaults deployedProfileVersion to 0 in the orphan profileVersionLabel', () => {
    // `deployedProfileVersion || 0` alternate arm on the diagnostic-only path.
    const [target] = runOrphans([
      orphanDiagnostic({ profileVersion: 7, deployedProfileVersion: undefined }),
    ]);

    expect(target?.profileVersionLabel).toBe('Expected v7 · deployed v0');
  });
});
