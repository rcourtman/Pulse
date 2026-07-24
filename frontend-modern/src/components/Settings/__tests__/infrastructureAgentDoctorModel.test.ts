import { describe, expect, it } from 'vitest';
import type { AgentFleetAgentDiagnostic } from '@/api/agentDiagnostics';
import type { Connection } from '@/api/connections';
import type { InfrastructureSystemRow } from '../connectionsTableModel';
import {
  collectInfrastructureAgentDoctorTargets,
  diagnosticConnectionID,
  formatInfrastructureAgentDoctorReport,
  getInfrastructureAgentDoctorUninstallHandoff,
  resolveKnownAgentCommandPlatform,
  type InfrastructureAgentDoctorTarget,
} from '../infrastructureAgentUpdateCommandsModel';

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'agent:host-1',
  type: 'agent',
  name: 'host-1',
  address: 'host-1.lab',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['host'],
  scope: { host: true },
  lastSeen: '2026-07-13T09:00:00Z',
  lastError: null,
  source: 'agent',
  agentVersion: '6.1.0',
  expectedAgentVersion: '6.2.0',
  agentUpdateAvailable: true,
  agentIdentity: { hostname: 'host-1', platform: 'ubuntu', architecture: 'amd64' },
  fleet: { versionDrift: 'behind' } as Connection['fleet'],
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
  ...overrides,
});

const rowFixture = (connection: Connection): InfrastructureSystemRow =>
  ({
    id: connection.id,
    ownerType: 'agent',
    name: connection.name,
    subtitle: 'via Pulse Agent',
    source: 'agent',
    host: connection.address,
    coverageLabels: ['Host telemetry'],
    statusLabel: 'Active',
    statusClassName: 'bg-green-100',
    agentUpdateCount: connection.agentUpdateAvailable ? 1 : 0,
    lastActivityText: '1m ago',
    fleetSignals: [],
    fleetHighlights: [],
    enabled: true,
    canEdit: false,
    canPause: false,
    canRemove: true,
    isAgent: true,
    isCluster: false,
    attachedConnections: [],
    members: [],
    connection,
  }) as InfrastructureSystemRow;

const diagnosticFixture = (
  overrides: Partial<AgentFleetAgentDiagnostic> = {},
): AgentFleetAgentDiagnostic => ({
  connectionId: 'agent:host-1',
  rowKey: 'agent-host-1',
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
      evidence: ['Reported v6.1.0; target v6.2.0'],
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

describe('Agent Doctor model', () => {
  it('enriches authoritative connection rows with structured reasons and profile drift', () => {
    const connection = connectionFixture();
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture()],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });

    expect(targets).toHaveLength(1);
    expect(targets[0]).toMatchObject({
      connectionId: 'agent:host-1',
      source: 'diagnostics',
      status: 'warning',
      needsUpdate: true,
      commandPlatform: 'linux',
      profileLabel: 'Linux servers',
      profileVersionLabel: 'Expected v4 · deployed v3',
    });
    expect(targets[0].reasons[0].code).toBe('agent_version_stale');
    expect(targets[0].evidence).toContain('Reported v6.1.0; target v6.2.0');
    expect(targets[0].commandBlockedReason).toBeUndefined();
  });

  it('does not report healthy when telemetry is fresh but command admission is disconnected', () => {
    const connection = connectionFixture({
      agentUpdateAvailable: false,
      agentVersion: '6.1.1',
      expectedAgentVersion: '6.1.1',
      agentIdentity: {
        hostname: 'host-1',
        platform: 'ubuntu',
        architecture: 'amd64',
        commandsEnabled: true,
      },
      fleet: {
        versionDrift: 'current',
        remoteControl: 'disconnected',
        commandPolicy: {
          status: 'blocked',
          desired: 'enabled',
          applied: 'enabled',
          enforcement: 'blocked',
          reason: 'Agent reports commands enabled, but no admitted command channel is connected.',
        },
      } as Connection['fleet'],
    });
    const [target] = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture({ status: 'healthy', reasons: [] })],
      diagnosticsAvailable: true,
      targetVersion: '6.1.1',
    });

    expect(target.status).toBe('critical');
    expect(target.reasons).toContainEqual(
      expect.objectContaining({
        code: 'command_channel_disconnected',
        severity: 'critical',
      }),
    );
  });

  it('falls back to ledger classification and blocks commands for unknown platforms', () => {
    const connection = connectionFixture({
      state: 'stale',
      stateReason: 'no heartbeat in 3m',
      agentIdentity: { hostname: 'host-1', platform: 'haiku' },
    });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnosticsAvailable: false,
      targetVersion: '6.2.0',
    });

    expect(targets[0]).toMatchObject({
      source: 'ledger-fallback',
      status: 'warning',
      needsUpdate: true,
      commandPlatform: null,
    });
    expect(targets[0].reasons.map((reason) => reason.code)).toEqual([
      'ledger_stale',
      'agent_version_stale',
    ]);
    expect(targets[0].commandBlockedReason).toContain('will not guess');
  });

  it('offers the Linux update handoff for a legacy long-tail distro identifier', () => {
    const connection = connectionFixture({
      agentIdentity: { hostname: 'mageia-host', platform: 'mageia', osName: 'Mageia' },
    });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnosticsAvailable: false,
      targetVersion: '6.2.0',
    });

    expect(targets[0]).toMatchObject({
      source: 'ledger-fallback',
      needsUpdate: true,
      commandPlatform: 'linux',
    });
    expect(targets[0].commandBlockedReason).toBeUndefined();
  });

  it('classifies eligible v6 convergence as waiting but keeps the manual command available', () => {
    const connection = connectionFixture({
      agentUpdate: {
        state: 'checking',
        autoUpdate: true,
        lastCheckedAt: '2026-07-13T09:01:00Z',
      },
    });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [
        diagnosticFixture({
          agentUpdate: {
            state: 'checking',
            autoUpdate: true,
            lastCheckedAt: '2026-07-13T09:01:00Z',
          },
        }),
      ],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });

    expect(targets[0]).toMatchObject({
      status: 'waiting',
      updaterLabel: 'Checking for an automatic update',
    });
    expect(targets[0].commandBlockedReason).toBeUndefined();
    expect(targets[0].evidence).toContain('Last updater check: 2026-07-13T09:01:00Z');
  });

  it('withholds the manual command only while an update is actively applying', () => {
    const nowMs = Date.parse('2026-07-13T09:05:00Z');
    const agentUpdate = {
      state: 'updating' as const,
      autoUpdate: true,
      lastAttemptAt: '2026-07-13T09:01:00Z',
    };
    const connection = connectionFixture({ agentUpdate });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture({ agentUpdate })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
      nowMs,
    });

    expect(targets[0].commandBlockedReason).toContain('applying an update right now');
  });

  it('restores the manual command when an in-flight update stalls', () => {
    const nowMs = Date.parse('2026-07-13T10:00:00Z');
    const agentUpdate = {
      state: 'updating' as const,
      autoUpdate: true,
      lastAttemptAt: '2026-07-13T09:01:00Z',
    };
    const connection = connectionFixture({ agentUpdate });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture({ agentUpdate })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
      nowMs,
    });

    expect(targets[0].commandBlockedReason).toBeUndefined();
  });

  it('keeps the manual command available when updating has no attempt timestamp', () => {
    const agentUpdate = {
      state: 'updating' as const,
      autoUpdate: true,
    };
    const connection = connectionFixture({ agentUpdate });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture({ agentUpdate })],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
      nowMs: Date.parse('2026-07-13T09:05:00Z'),
    });

    expect(targets[0].commandBlockedReason).toBeUndefined();
  });

  it('withholds FreeBSD and pfSense update commands until installer state is proven', () => {
    const connection = connectionFixture({
      agentIdentity: { hostname: 'firewall', platform: 'pfSense', architecture: 'amd64' },
    });
    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture()],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });

    expect(targets[0].commandPlatform).toBe('freebsd');
    expect(targets[0].commandBlockedReason).toContain('cannot verify saved FreeBSD or pfSense');
  });

  it('includes removed endpoint records only in unscoped fleet drilldown', () => {
    const connection = connectionFixture({ agentUpdateAvailable: false });
    const removed = diagnosticFixture({
      connectionId: 'agent:removed-host',
      rowKey: 'removed-host-removed-host',
      id: 'removed-host',
      agentId: undefined,
      name: 'removed-host',
      status: 'removed',
      reasons: [
        {
          code: 'agent_removed_blocked',
          severity: 'warning',
          message: 'Agent is intentionally removed.',
        },
      ],
      repairActions: [
        {
          code: 'allow_reenroll',
          label: 'Allow re-enroll',
          description: 'Use the existing removed-agent action.',
          supported: true,
        },
      ],
    });
    const base = {
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [removed],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    };

    expect(collectInfrastructureAgentDoctorTargets(base).map((target) => target.status)).toContain(
      'removed',
    );
    expect(
      collectInfrastructureAgentDoctorTargets({
        ...base,
        scopedAgentIds: ['agent:host-1'],
      }).map((target) => target.status),
    ).not.toContain('removed');
  });

  it('hands removed targets an uninstall command scoped to their reported platform', () => {
    const removedWindows = diagnosticFixture({
      connectionId: 'agent:removed-win',
      rowKey: 'removed-win',
      id: 'removed-win',
      agentId: 'removed-win-agent',
      name: 'removed-win',
      hostname: 'win-host',
      platform: 'Windows Server 2022',
      status: 'removed',
    });
    const removedLinux = diagnosticFixture({
      connectionId: 'agent:removed-deb',
      rowKey: 'removed-deb',
      id: 'removed-deb',
      agentId: 'removed-deb-agent',
      name: 'removed-deb',
      hostname: 'deb-host',
      platform: 'debian',
      status: 'removed',
    });

    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [],
      diagnostics: [removedWindows, removedLinux],
      diagnosticsAvailable: true,
    });
    const windowsTarget = targets.find((target) => target.displayName === 'removed-win');
    const linuxTarget = targets.find((target) => target.displayName === 'removed-deb');

    expect(windowsTarget?.commandPlatform).toBe('windows');
    expect(getInfrastructureAgentDoctorUninstallHandoff(windowsTarget!)).toEqual({
      identity: { agentId: 'removed-win-agent', hostname: 'win-host' },
      commands: [{ label: 'Windows PowerShell', platform: 'windows' }],
    });

    expect(linuxTarget?.commandPlatform).toBe('linux');
    expect(getInfrastructureAgentDoctorUninstallHandoff(linuxTarget!)).toEqual({
      identity: { agentId: 'removed-deb-agent', hostname: 'deb-host' },
      commands: [{ label: 'Linux / macOS / FreeBSD', platform: 'linux' }],
    });
  });

  it('offers both labeled uninstall families when a removed agent has no recognized platform', () => {
    const removed = diagnosticFixture({
      connectionId: 'agent:removed-host',
      rowKey: 'removed-host',
      id: 'removed-host',
      agentId: undefined,
      name: 'removed-host',
      hostname: undefined,
      status: 'removed',
    });

    const [target] = collectInfrastructureAgentDoctorTargets({
      rows: [],
      diagnostics: [removed],
      diagnosticsAvailable: true,
    });

    expect(target.commandPlatform).toBeNull();
    expect(getInfrastructureAgentDoctorUninstallHandoff(target)).toEqual({
      identity: { agentId: 'removed-host', hostname: undefined },
      commands: [
        { label: 'Linux / macOS / FreeBSD', platform: 'linux' },
        { label: 'Windows PowerShell', platform: 'windows' },
      ],
    });
  });

  it('keeps the uninstall handoff off non-removed targets', () => {
    const connection = connectionFixture();
    const [target] = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [diagnosticFixture()],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });

    expect(target.status).not.toBe('removed');
    expect(getInfrastructureAgentDoctorUninstallHandoff(target)).toBeNull();
  });

  it('keeps integration-monitored machines out of the doctor', () => {
    const agent = connectionFixture({ agentUpdateAvailable: false });
    const esxi = connectionFixture({
      id: 'agent:vc-1:host:host-101',
      name: 'esxi-01.lab.local',
      agentVersion: undefined,
      expectedAgentVersion: undefined,
      agentUpdateAvailable: false,
      integrationSource: 'vmware',
      agentIdentity: { hostname: 'esxi-01.lab.local', platform: 'vmware-vsphere' },
      fleet: {} as Connection['fleet'],
    });

    const targets = collectInfrastructureAgentDoctorTargets({
      rows: [rowFixture(agent), rowFixture(esxi)],
      connections: [agent, esxi],
      diagnostics: [diagnosticFixture()],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    });

    expect(targets.map((target) => target.connectionId)).toEqual(['agent:host-1']);
  });

  it('surfaces diagnostics-only agents the ledger does not carry', () => {
    const connection = connectionFixture({ agentUpdateAvailable: false });
    const dockerOnly = diagnosticFixture({
      connectionId: 'agent:docker-edge-1',
      rowKey: 'agent-docker-edge-1',
      id: 'docker-edge-1',
      agentId: 'docker-edge-1',
      name: 'Edge Apps 01',
      hostname: 'edge-apps-01',
      types: ['docker'],
      status: 'critical',
      profileId: undefined,
      profileName: undefined,
      profileVersion: undefined,
      deployedProfileVersion: undefined,
      reasons: [
        {
          code: 'agent_module_failed',
          severity: 'critical',
          message: 'Enabled agent module "docker" failed.',
        },
      ],
      repairActions: [],
    });
    const base = {
      rows: [rowFixture(connection)],
      connections: [connection],
      diagnostics: [dockerOnly],
      diagnosticsAvailable: true,
      targetVersion: '6.2.0',
    };

    const targets = collectInfrastructureAgentDoctorTargets(base);
    const surfaced = targets.find((target) => target.connectionId === 'agent:docker-edge-1');
    expect(surfaced).toMatchObject({
      status: 'critical',
      source: 'diagnostics',
      needsUpdate: false,
      commandPlatform: null,
    });
    expect(surfaced?.reasons[0]?.code).toBe('agent_module_failed');

    // Scoping still applies to diagnostics-only rows.
    expect(
      collectInfrastructureAgentDoctorTargets({
        ...base,
        scopedAgentIds: ['agent:host-1'],
      }).map((target) => target.connectionId),
    ).toEqual(['agent:host-1']);
  });

  it('canonicalizes compatibility diagnostics that predate connectionId', () => {
    expect(
      diagnosticConnectionID(
        diagnosticFixture({ connectionId: undefined, agentId: 'host-1', id: 'legacy-id' }),
      ),
    ).toBe('agent:host-1');
  });

  it.each(['AlmaLinux', 'Proxmox VE', 'QNAP', 'Synology DSM', 'openSUSE Leap', 'Debian GNU/Linux'])(
    'recognizes supported Linux platform caption %s',
    (platform) => {
      expect(resolveKnownAgentCommandPlatform(platform)).toBe('linux');
    },
  );

  it('fails closed for missing and unsupported platform captions', () => {
    expect(resolveKnownAgentCommandPlatform('')).toBeNull();
    expect(resolveKnownAgentCommandPlatform('Haiku')).toBeNull();
  });

  describe('formatInfrastructureAgentDoctorReport', () => {
    const doctorTargetFixture = (
      overrides: Partial<InfrastructureAgentDoctorTarget> = {},
    ): InfrastructureAgentDoctorTarget => ({
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
      source: 'diagnostics',
      ...overrides,
    });

    it('reports fleet counts, per-agent state, and diagnosis detail', () => {
      const report = formatInfrastructureAgentDoctorReport([
        doctorTargetFixture({
          currentVersion: '5.1.34',
          expectedVersion: '6.1.0',
          lastSeen: '2026-07-22T09:00:00Z',
          updaterLabel: 'systemd unit',
          profileLabel: 'default',
          profileVersionLabel: 'v3',
          reasons: [
            {
              code: 'agent_stale',
              severity: 'critical',
              message: 'No report has arrived for 10m.',
              evidence: ['Expected report interval: 30s'],
            },
          ],
          evidence: ['hostname host-1'],
        }),
        doctorTargetFixture({
          key: 'agent:host-2',
          connectionId: 'agent:host-2',
          displayName: 'host-2',
          status: 'healthy',
        }),
      ]);

      expect(report).toContain('Pulse Agent Doctor report (2 agents; 1 critical, 1 healthy)');
      expect(report).toContain('host-1 (Machine)');
      expect(report).toContain('  Connection agent:host-1');
      expect(report).toContain('  Status Critical');
      expect(report).toContain('  Reported agent 5.1.34');
      expect(report).toContain('  Supported target 6.1.0');
      expect(report).toContain('  Last seen 2026-07-22T09:00:00.000Z');
      expect(report).toContain('  Updater systemd unit');
      expect(report).toContain('  Profile default (v3)');
      expect(report).toContain('  - No report has arrived for 10m.');
      expect(report).toContain('    Expected report interval: 30s');
      expect(report).toContain('  Identity evidence');
      expect(report).toContain('  - hostname host-1');
      expect(report).toContain('host-2 (Machine)');
      expect(report).toContain('  Status Healthy');
    });

    it('omits absent fields and never embeds update commands', () => {
      const report = formatInfrastructureAgentDoctorReport([
        doctorTargetFixture({
          needsUpdate: true,
          commandBlockedReason: 'The reported platform is unsupported.',
          diagnostic: {
            connectionId: 'agent:host-1',
            repairActions: [
              {
                code: 'copy_upgrade_command',
                label: 'Copy update command',
                description: 'Copy the host-local update command.',
                supported: true,
              },
              {
                code: 'reissue_token',
                label: 'Reissue token',
                description: 'Generate a fresh install token.',
                supported: true,
              },
            ],
          } as unknown as InfrastructureAgentDoctorTarget['diagnostic'],
        }),
      ]);

      expect(report).toContain('Pulse Agent Doctor report (1 agent; 1 critical)');
      expect(report).not.toContain('Reported agent');
      expect(report).not.toContain('Supported target');
      expect(report).not.toContain('Last seen');
      expect(report).not.toContain('Updater');
      expect(report).not.toContain('Profile');
      // The update-command handoff stays on the page; reports carry the
      // blocked reason and non-command repairs only.
      expect(report).not.toContain('Copy update command');
      expect(report).toContain('  - Reissue token. Generate a fresh install token.');
      expect(report).toContain('  Update command blocked. The reported platform is unsupported.');
    });
  });
});
