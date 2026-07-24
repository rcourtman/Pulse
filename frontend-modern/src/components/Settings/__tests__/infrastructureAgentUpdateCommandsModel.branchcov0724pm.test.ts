import { describe, expect, it } from 'vitest';
import type { Connection } from '@/api/connections';
import type { AgentFleetAgentDiagnostic, AgentFleetDiagnosticReason } from '@/api/agentDiagnostics';
import {
  collectInfrastructureAgentDoctorTargets,
  formatInfrastructureAgentDoctorReport,
  getInfrastructureAgentDoctorUninstallHandoff,
} from '../infrastructureAgentUpdateCommandsModel';
import type {
  InfrastructureAgentDoctorOptions,
  InfrastructureAgentDoctorTarget,
} from '../infrastructureAgentUpdateCommandsModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling infrastructureAgentUpdateCommandsModel.branchcov0722am.test.ts
// factories. This file targets the final nine branch arms V8 still reported as
// zero-hit after the five sibling suites ran. Every arm is driven through the
// exported orchestrators (collectInfrastructureAgentDoctorTargets /
// formatInfrastructureAgentDoctorReport / getInfrastructureAgentDoctorUninstallHandoff).

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
  lastSeen: '2026-07-24T09:00:00Z',
  lastError: null,
  source: 'agent',
  agentVersion: '6.2.0',
  expectedAgentVersion: '6.2.0',
  agentUpdateAvailable: false,
  agentIdentity: { hostname: 'host-1', platform: 'linux', architecture: 'amd64' },
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
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
  version: '6.2.0',
  profileId: 'profile-linux',
  profileName: 'Linux servers',
  profileVersion: 4,
  deployedProfileVersion: 3,
  reasons: [],
  repairActions: [],
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

const runOrphans = (
  diagnostics: readonly AgentFleetAgentDiagnostic[],
  options: Partial<InfrastructureAgentDoctorOptions> = {},
) =>
  runDoctor([], {
    diagnostics,
    diagnosticsAvailable: true,
    ...options,
  });

// Minimal hand-built doctor target for the pure report formatter (which does
// not re-derive fields, only reads them).
const doctorTarget = (
  overrides: Partial<InfrastructureAgentDoctorTarget> = {},
): InfrastructureAgentDoctorTarget =>
  ({
    key: 'agent:host-1',
    connectionId: 'agent:host-1',
    displayName: 'host-1',
    contextLabel: 'Machine',
    installFlags: [],
    status: 'critical',
    reasons: [],
    evidence: [],
    needsUpdate: false,
    commandPlatform: null,
    source: 'ledger-fallback',
    ...overrides,
  }) as InfrastructureAgentDoctorTarget;

// ---- ledgerFallbackReasons: command_channel_disconnected via the && arm -------
// The sibling suites only ever drive this reason through the
// `remoteControl === 'disconnected'` short-circuit of the `||`, so the
// `(commandPolicy.status === 'blocked' && commandsEnabled)` right operand is
// never evaluated. Driving it with remoteControl !== 'disconnected' exercises
// both operands of the `&&`, and varying commandPolicy.reason exercises both
// arms of the evidence ternary on line 346.

describe('collectInfrastructureAgentDoctorTargets (command_channel_disconnected && arm)', () => {
  it('emits the reason via commandPolicy blocked + commandsEnabled with reason evidence', () => {
    // remoteControl 'enabled' forces the || past its left operand so the &&
    // (status === 'blocked' && commandsEnabled) is fully evaluated and true.
    const [target] = runDoctor([
      agentConnection({
        agentIdentity: { hostname: 'host-1', platform: 'linux', commandsEnabled: true },
        fleet: {
          versionDrift: 'current',
          remoteControl: 'enabled',
          commandPolicy: { status: 'blocked', reason: 'policy disabled by admin' },
        } as Connection['fleet'],
      }),
    ]);

    expect(target?.status).toBe('critical');
    expect(
      target?.reasons.find((reason) => reason.code === 'command_channel_disconnected')?.evidence,
    ).toEqual(['policy disabled by admin']);
  });

  it('emits the reason with empty evidence when commandPolicy has no reason', () => {
    // Same && arm, but commandPolicy.reason absent -> the `reason ? [reason] : []`
    // ternary takes its [] alternate (the only previously-unhit arm of line 346).
    const [target] = runDoctor([
      agentConnection({
        agentIdentity: { hostname: 'host-1', platform: 'linux', commandsEnabled: true },
        fleet: {
          versionDrift: 'current',
          remoteControl: 'enabled',
          commandPolicy: { status: 'blocked' },
        } as Connection['fleet'],
      }),
    ]);

    expect(
      target?.reasons.find((reason) => reason.code === 'command_channel_disconnected')?.evidence,
    ).toEqual([]);
  });
});

// ---- getInfrastructureAgentDoctorUninstallHandoff: identity agentId fallback --

describe('getInfrastructureAgentDoctorUninstallHandoff (agentId || undefined arm)', () => {
  it('resolves identity.agentId to undefined when both agentId and id are blank', () => {
    // `agentId?.trim() || id?.trim() || undefined` only reaches its final
    // `|| undefined` operand when both agentId and id trim to empty. A removed
    // diagnostic with name/hostname present still renders and resolves a known
    // platform, so the handoff is non-null and carries the undefined agentId.
    const [target] = runOrphans([
      diagnostic({
        connectionId: 'agent:gone',
        rowKey: 'gone-row',
        id: '',
        agentId: undefined,
        name: 'gone',
        hostname: 'gone-host',
        platform: 'debian',
        status: 'removed',
      }),
    ]);

    const handoff = getInfrastructureAgentDoctorUninstallHandoff(target!);
    expect(handoff).not.toBeNull();
    expect(handoff?.identity.agentId).toBeUndefined();
    expect(handoff?.identity.hostname).toBe('gone-host');
    expect(handoff?.commands.map((command) => command.platform)).toEqual(['linux']);
  });
});

// ---- doctorReportLastSeen: numeric + non-finite arms ------------------------
// The sibling formatter test only passes a parseable ISO string, so the
// `typeof value === 'number'` consequent and the `!Number.isFinite(timestamp)`
// return arms of doctorReportLastSeen are never taken.

describe('formatInfrastructureAgentDoctorReport (doctorReportLastSeen arms)', () => {
  it('formats a numeric lastSeen directly as an ISO timestamp', () => {
    // typeof value === 'number' -> timestamp = value (the consequent arm).
    const ms = Date.parse('2025-01-22T09:00:00Z');
    const report = formatInfrastructureAgentDoctorReport([doctorTarget({ lastSeen: ms })]);

    expect(report).toContain('Last seen 2025-01-22T09:00:00.000Z');
  });

  it('passes a non-parseable string lastSeen through verbatim', () => {
    // Date.parse('not-a-date') -> NaN -> !isFinite -> typeof === 'string' arm.
    const report = formatInfrastructureAgentDoctorReport([
      doctorTarget({ lastSeen: 'not-a-date' }),
    ]);

    expect(report).toContain('Last seen not-a-date');
  });

  it('omits Last seen when lastSeen is a non-finite number', () => {
    // A NaN number passes the typeof === 'number' filter, fails isFinite, and is
    // not a string -> doctorReportLastSeen returns undefined (the alternate arm).
    const report = formatInfrastructureAgentDoctorReport([doctorTarget({ lastSeen: Number.NaN })]);

    expect(report).not.toContain('Last seen');
  });
});

// ---- formatInfrastructureAgentDoctorReport: header + profile + evidence ------

describe('formatInfrastructureAgentDoctorReport (header / profile / reason-evidence arms)', () => {
  it('renders the header with no status breakdown for an empty target list', () => {
    // No targets -> every status count is zero -> countParts empty -> the
    // `countParts.length > 0 ? ... : ''` ternary takes its '' alternate.
    expect(formatInfrastructureAgentDoctorReport([])).toBe('Pulse Agent Doctor report (0 agents)');
  });

  it('renders the profile label without a version parenthetical when profileVersionLabel is absent', () => {
    // profileLabel present but profileVersionLabel undefined -> the
    // `profileVersionLabel ? \` (${...})\` : ''` ternary takes its '' alternate.
    const report = formatInfrastructureAgentDoctorReport([
      doctorTarget({ status: 'critical', profileLabel: 'default', profileVersionLabel: undefined }),
    ]);

    expect(report.split('\n')).toContain('  Profile default');
    expect(report).not.toContain('Profile default (');
  });

  it('renders a reason whose evidence is absent without any evidence sublines', () => {
    // `reason.evidence ?? []` -> when evidence is nullish the [] operand wins and
    // no 4-space-indented evidence line is emitted for that reason.
    const reasonWithoutEvidence: AgentFleetDiagnosticReason = {
      code: 'agent_stale',
      severity: 'critical',
      message: 'No heartbeat.',
      evidence: undefined,
    };
    const report = formatInfrastructureAgentDoctorReport([
      doctorTarget({ status: 'critical', reasons: [reasonWithoutEvidence], evidence: [] }),
    ]);

    expect(report).toContain('  - No heartbeat.');
    // No 4-space-indented evidence line should appear anywhere in the report.
    expect(report).not.toMatch(/\n {4}\S/);
  });
});
