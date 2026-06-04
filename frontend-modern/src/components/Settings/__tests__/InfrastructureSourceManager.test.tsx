import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { InfrastructureSourceManager } from '../InfrastructureSourceManager';
import {
  primaryRowProblem,
  type FleetGovernanceSignal,
  type InfrastructureSystemMemberRow,
  type InfrastructureSystemRow,
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
  const fleetHighlights = overrides.fleetHighlights ?? [signal({})];
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
    fleetHighlights,
    problem: overrides.problem ?? primaryRowProblem(fleetHighlights),
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
): InfrastructureSystemMemberRow => {
  const fleetHighlights = overrides.fleetHighlights ?? [];
  return {
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
    fleetHighlights,
    problem: overrides.problem ?? primaryRowProblem(fleetHighlights),
    primary: overrides.primary ?? false,
    agentConnection: overrides.agentConnection,
  };
};

describe('InfrastructureSourceManager setup summary', () => {
  afterEach(() => cleanup());

  it('renders manual discovery as a visible operator action', () => {
    const onRunDiscovery = vi.fn();
    const onOpenDiscoverySettings = vi.fn();
    const onAddSourceStep = vi.fn();

    render(() => (
      <InfrastructureSourceManager
        rows={() => []}
        discoveredNodes={() => []}
        discoveryEnabled
        discoveryScanStatus={() => ({ scanning: false })}
        readOnly={false}
        onAddSourceStep={onAddSourceStep}
        onRunDiscovery={onRunDiscovery}
        onOpenDiscoverySettings={onOpenDiscoverySettings}
      />
    ));

    expect(screen.queryByRole('button', { name: /^Monitor endpoint$/i })).toBeNull();
    expect(onAddSourceStep).not.toHaveBeenCalled();
    const discovery = screen.getByRole('region', { name: /^Network discovery$/i });
    expect(within(discovery).getByText('Ready to scan configured networks')).toBeInTheDocument();
    expect(
      within(discovery).getByText(
        /Run discovery to look for unattached Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway APIs/i,
      ),
    ).toBeInTheDocument();

    fireEvent.click(within(discovery).getByRole('button', { name: /^Run discovery$/i }));
    expect(onRunDiscovery).toHaveBeenCalledTimes(1);

    fireEvent.click(within(discovery).getByRole('button', { name: /^Discovery settings$/i }));
    expect(onOpenDiscoverySettings).toHaveBeenCalledTimes(1);
  });

  it('keeps manual discovery observable while a scan is running', () => {
    const onRunDiscovery = vi.fn();

    render(() => (
      <InfrastructureSourceManager
        rows={() => []}
        discoveredNodes={() => []}
        discoveryEnabled
        discoveryScanStatus={() => ({
          scanning: true,
          subnet: '10.0.0.0/24',
          lastScanStartedAt: Date.now(),
        })}
        readOnly={false}
        onRunDiscovery={onRunDiscovery}
      />
    ));

    const discovery = screen.getByRole('region', { name: /^Network discovery$/i });
    expect(within(discovery).getByText('Scanning configured networks')).toBeInTheDocument();
    expect(
      within(discovery).getByText(
        /Pulse is scanning 10\.0\.0\.0\/24 for Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway APIs/i,
      ),
    ).toBeInTheDocument();
    expect(within(discovery).getByRole('button', { name: /^Scanning\.\.\.$/i })).toBeDisabled();
  });

  it('keeps discovered candidates in the discovery monitor instead of hiding them in the table', () => {
    const onReviewDiscoveredSource = vi.fn();
    const candidate = {
      ip: '10.0.0.55',
      port: 8006,
      type: 'pve' as const,
      version: '8.2.2',
      hostname: 'discovered-pve.lab',
    };

    render(() => (
      <InfrastructureSourceManager
        rows={() => []}
        discoveredNodes={() => [candidate]}
        discoveryEnabled
        discoveryScanStatus={() => ({ scanning: false, lastResultAt: Date.now() })}
        readOnly={false}
        onReviewDiscoveredSource={onReviewDiscoveredSource}
      />
    ));

    const discovery = screen.getByRole('region', { name: /^Network discovery$/i });
    expect(within(discovery).getByText('1 candidate ready to review')).toBeInTheDocument();
    expect(
      within(discovery).getByText(/Review and add credentials before Pulse starts monitoring it/i),
    ).toBeInTheDocument();

    fireEvent.click(within(discovery).getByRole('button', { name: /^Review candidate$/i }));
    expect(onReviewDiscoveredSource).toHaveBeenCalledWith(candidate);
    expect(screen.getByText('discovered-pve.lab')).toBeInTheDocument();
  });

  it('keeps setup status compact while preserving row-level attention signals', () => {
    render(() => (
      <InfrastructureSourceManager
        rows={() => [
          row({
            id: 'availability:meter',
            ownerType: 'availability',
            name: 'MQTT power meter',
            subtitle: 'via availability probe',
            source: 'probe',
            host: 'power-meter-01.lab.local',
            coverageLabels: ['Availability'],
            connection: connectionFixture({
              id: 'availability:meter',
              type: 'availability',
              name: 'MQTT power meter',
              address: 'power-meter-01.lab.local',
              source: 'manual',
              surfaces: ['availability'],
              scope: { availability: true },
            }),
            fleetSignals: [signal({ key: 'liveness', label: 'Live', tone: 'ok' })],
            fleetHighlights: [],
          }),
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
    expect(screen.queryByText('Endpoints')).toBeNull();
    expect(screen.queryByText('MQTT power meter')).toBeNull();
    expect(screen.getByText('Live')).toBeInTheDocument();
    expect(screen.getByText('Needs attention')).toBeInTheDocument();
    expect(screen.getByText('Needs agent')).toBeInTheDocument();
    expect(screen.queryByText('Fleet governance')).toBeNull();
    // The row now surfaces a single plain-English problem line under its
    // status badge instead of a chip-per-signal stack. Info-toned signals
    // (e.g. "Remote control enabled") aren't problems and don't render.
    expect(screen.getByText('Credentials invalid')).toBeInTheDocument();
    expect(screen.queryByText('Remote control enabled')).toBeNull();
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
    // "Fleet OK" is no longer rendered as a synthetic ok-chip; an empty
    // problem line is the absence of trouble. The cluster-member row
    // surfaces its single actionable problem ("Config drift") with the
    // detail attached as a tooltip.
    expect(screen.queryByText('Fleet OK')).toBeNull();
    expect(screen.getByText('Config drift')).toHaveAttribute(
      'title',
      'Desired and applied fingerprints differ.',
    );
  });
});
