import { describe, expect, it } from 'vitest';
import type { AgentFleetAgentDiagnostic } from '@/api/agentDiagnostics';
import type { Connection } from '@/api/connections';
import type { InfrastructureSystemRow } from '../connectionsTableModel';
import {
  collectInfrastructureAgentDoctorTargets,
  diagnosticConnectionID,
  resolveKnownAgentCommandPlatform,
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
});
