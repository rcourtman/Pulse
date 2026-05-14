import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { InfrastructureSourceManager } from '../InfrastructureSourceManager';
import type {
  FleetGovernanceSignal,
  InfrastructureSystemMemberRow,
  InfrastructureSystemRow,
} from '../connectionsTableModel';
import type { Connection } from '@/api/connections';

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'agent:host-1',
  type: 'agent',
  name: 'host-1',
  address: 'host-1',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['host'],
  scope: { host: true },
  lastSeen: '2026-04-23T12:00:00Z',
  lastError: null,
  source: 'agent',
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
  ...overrides,
});

const signal = (overrides: Partial<FleetGovernanceSignal>): FleetGovernanceSignal => ({
  key: 'liveness',
  label: 'Fleet OK',
  detail: 'No fleet warnings.',
  tone: 'ok',
  ...overrides,
});

const row = (overrides: Partial<InfrastructureSystemRow> = {}): InfrastructureSystemRow => {
  const connection = overrides.connection ?? connectionFixture();
  const fleetSignals = overrides.fleetSignals ?? [
    signal({ key: 'liveness', label: 'Live', tone: 'ok' }),
  ];
  return {
    id: connection.id,
    ownerType: connection.type,
    name: connection.name,
    subtitle: 'via Pulse Agent',
    source: 'agent',
    host: connection.address,
    coverageLabels: ['Host telemetry'],
    statusLabel: 'Active',
    statusClassName: 'bg-green-100 text-green-800',
    agentUpdateCount: 0,
    lastActivityText: '5s ago',
    fleetSignals,
    fleetHighlights: overrides.fleetHighlights ?? [signal({})],
    enabled: connection.enabled,
    canEdit: false,
    canPause: false,
    canRemove: true,
    isAgent: connection.type === 'agent',
    isCluster: false,
    attachedConnections: [],
    members: [],
    connection,
    ...overrides,
  };
};

const member = (
  overrides: Partial<InfrastructureSystemMemberRow> = {},
): InfrastructureSystemMemberRow => ({
  id: overrides.id ?? 'member-1',
  name: overrides.name ?? 'member-1',
  subtitle: overrides.subtitle ?? 'Cluster member',
  source: overrides.source ?? 'agent',
  host: overrides.host ?? 'https://member-1:8006',
  coverageLabels: overrides.coverageLabels ?? ['Host telemetry'],
  statusLabel: overrides.statusLabel ?? 'Active',
  statusClassName: overrides.statusClassName ?? 'bg-green-100 text-green-800',
  lastActivityText: overrides.lastActivityText ?? '5s ago',
  fleetSignals: overrides.fleetSignals ?? [],
  fleetHighlights: overrides.fleetHighlights ?? [],
  primary: overrides.primary ?? false,
  agentConnection: overrides.agentConnection,
});

describe('InfrastructureSourceManager setup summary', () => {
  afterEach(() => cleanup());

  it('keeps setup status compact while preserving row-level attention signals', () => {
    render(() => (
      <InfrastructureSourceManager
        rows={() => [
          row({
            fleetSignals: [
              signal({ key: 'liveness', label: 'Live', tone: 'ok' }),
              signal({ key: 'remote-control', label: 'Remote control enabled', tone: 'info' }),
            ],
            fleetHighlights: [
              signal({ key: 'remote-control', label: 'Remote control enabled', tone: 'info' }),
            ],
          }),
          row({
            id: 'pve:lab',
            ownerType: 'pve',
            name: 'lab',
            source: 'api',
            connection: connectionFixture({
              id: 'pve:lab',
              type: 'pve',
              name: 'lab',
              source: 'manual',
              capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
            }),
            fleetSignals: [
              signal({ key: 'liveness', label: 'Unauthorized', tone: 'critical' }),
              signal({ key: 'credentials', label: 'Credentials invalid', tone: 'critical' }),
            ],
            fleetHighlights: [
              signal({ key: 'credentials', label: 'Credentials invalid', tone: 'critical' }),
            ],
          }),
        ]}
        discoveredNodes={() => []}
        discoveryEnabled
        discoveryScanStatus={() => ({ scanning: false })}
        readOnly
      />
    ));

    expect(screen.getByText('Setup status')).toBeInTheDocument();
    expect(screen.getByText('Systems')).toBeInTheDocument();
    expect(screen.getByText('Live')).toBeInTheDocument();
    expect(screen.getByText('Needs attention')).toBeInTheDocument();
    expect(screen.getByText('Needs agent')).toBeInTheDocument();
    expect(screen.queryByText('Fleet governance')).toBeNull();
    expect(screen.getByText('Credentials invalid')).toBeInTheDocument();
    expect(screen.getByText('Remote control enabled')).toBeInTheDocument();
  });

  it('does not count hidden passive agent config handshakes as setup attention', () => {
    render(() => (
      <InfrastructureSourceManager
        rows={() => [
          row({
            fleetSignals: [
              signal({
                key: 'config-drift',
                label: 'Config pending',
                detail:
                  'Pulse has not received a comparable applied agent configuration fingerprint yet',
                tone: 'warning',
              }),
              signal({
                key: 'rollout',
                label: 'Rollout pending',
                detail: 'waiting for the agent to report an applied configuration fingerprint',
                tone: 'warning',
              }),
              signal({
                key: 'command-policy',
                label: 'Remote control disabled',
                tone: 'info',
              }),
            ],
            fleetHighlights: [
              signal({
                key: 'command-policy',
                label: 'Remote control disabled',
                tone: 'info',
              }),
            ],
          }),
        ]}
        discoveredNodes={() => []}
        discoveryEnabled
        discoveryScanStatus={() => ({ scanning: false })}
        readOnly
      />
    ));

    expect(screen.getByText('Needs attention').nextElementSibling?.textContent).toBe('0 systems');
    expect(screen.queryByText('Config pending')).toBeNull();
    expect(screen.queryByText('Rollout pending')).toBeNull();
  });

  it('still counts actionable member posture when the cluster parent is healthy', () => {
    render(() => (
      <InfrastructureSourceManager
        rows={() => [
          row({
            id: 'pve:homelab',
            ownerType: 'pve',
            name: 'homelab',
            source: 'both',
            isCluster: true,
            fleetSignals: [signal({ key: 'liveness', label: 'Live', tone: 'ok' })],
            fleetHighlights: [signal({ key: 'liveness', label: 'Fleet OK', tone: 'ok' })],
            members: [
              member({
                id: 'node-delly',
                name: 'delly',
                fleetSignals: [
                  signal({
                    key: 'config-drift',
                    label: 'Config drift',
                    detail: 'Desired and applied fingerprints differ.',
                    tone: 'warning',
                  }),
                ],
                fleetHighlights: [
                  signal({
                    key: 'config-drift',
                    label: 'Config drift',
                    detail: 'Desired and applied fingerprints differ.',
                    tone: 'warning',
                  }),
                ],
                primary: true,
              }),
            ],
          }),
        ]}
        discoveredNodes={() => []}
        discoveryEnabled
        discoveryScanStatus={() => ({ scanning: false })}
        readOnly
      />
    ));

    expect(screen.getByText('Needs attention').nextElementSibling?.textContent).toBe('1 system');
    expect(screen.getByText('Fleet OK')).toBeInTheDocument();
    expect(screen.getByText('Config drift')).toHaveAttribute(
      'title',
      'Desired and applied fingerprints differ.',
    );
  });
});
