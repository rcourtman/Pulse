import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsTable } from '../ConnectionsTable';
import {
  CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS,
  connectionAgentIdentitySummary,
  type InfrastructureSystemRow,
} from '../connectionsTableModel';
import type { Connection, ConnectionAgentIdentity } from '@/api/connections';
import type { ConnectionRowActions } from '../useConnectionRowActions';

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:tower',
  type: 'pve',
  name: 'tower',
  address: 'https://tower.local:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms'],
  scope: { vms: true },
  lastSeen: null,
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const row = (overrides: Partial<InfrastructureSystemRow> = {}): InfrastructureSystemRow => {
  const connection = overrides.connection ?? connectionFixture();
  return {
    id: overrides.id ?? connection.id,
    ownerType: overrides.ownerType ?? connection.type,
    name: overrides.name ?? 'tower',
    subtitle: undefined,
    source: 'api',
    host: '10.0.0.1',
    coverageLabels: ['Host telemetry'],
    statusLabel: 'online',
    statusClassName: 'bg-green-100 text-green-800',
    agentUpdateCount: 0,
    lastActivityText: '5s ago',
    lastErrorMessage: undefined,
    fleetSignals: [],
    fleetHighlights: [],
    enabled: connection.enabled,
    canEdit: true,
    canPause: connection.capabilities.supportsPause,
    canRemove: connection.type !== 'docker' && connection.type !== 'kubernetes',
    isAgent: connection.type === 'agent',
    isCluster: false,
    attachedConnections: [],
    members: [],
    connection,
    ...overrides,
  };
};

const makeActions = (overrides: Partial<ConnectionRowActions> = {}): ConnectionRowActions => ({
  pendingAction: () => null,
  actionError: () => null,
  confirmingRemove: () => false,
  togglePause: vi.fn(),
  requestRemove: vi.fn(),
  cancelRemove: vi.fn(),
  ...overrides,
});

describe('ConnectionsTable', () => {
  afterEach(() => cleanup());

  it('renders an empty-state hint when no rows exist', () => {
    render(() => <ConnectionsTable rows={() => []} />);

    expect(screen.getByText('Start monitoring infrastructure')).toBeInTheDocument();
    // Empty-state copy now leads with the action ('Add your first server,
    // cluster, or appliance to begin') and lists a curated short set of
    // supported sources rather than the full enumeration.
    expect(
      screen.getByText(/Add your first server, cluster, or appliance to begin/i),
    ).toBeInTheDocument();
    expect(screen.queryByRole('table')).toBeNull();
  });

  it('surfaces the primary action inline in the empty state when one is provided', () => {
    const onSelect = vi.fn();
    render(() => (
      <ConnectionsTable
        rows={() => []}
        headerActions={[{ label: 'Add infrastructure', tone: 'primary', onSelect }]}
      />
    ));

    const button = screen.getAllByRole('button', { name: 'Add infrastructure' });
    expect(button.length).toBeGreaterThan(0);
    fireEvent.click(button[button.length - 1]);
    expect(onSelect).toHaveBeenCalled();
  });

  it('renders one row per monitored system with coverage and status labels', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row(),
          row({
            id: 'truenas:nas',
            name: 'nas',
            subtitle: 'via platform API',
            host: undefined,
            coverageLabels: ['Datasets'],
            statusLabel: 'Paused',
            connection: connectionFixture({ id: 'truenas:nas', type: 'truenas', name: 'nas' }),
          }),
        ]}
      />
    ));

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('nas')).toBeInTheDocument();
    expect(screen.getByText('via platform API')).toBeInTheDocument();
    expect(screen.getByText('Datasets')).toBeInTheDocument();
    expect(screen.getByText('Paused')).toBeInTheDocument();
    expect(screen.getByText('online')).toBeInTheDocument();
  });

  it('surfaces configured header actions', () => {
    const onAdd = vi.fn();
    render(() => (
      <ConnectionsTable
        rows={() => []}
        headerActions={[{ label: 'Add connection', onSelect: onAdd, tone: 'primary' }]}
      />
    ));

    // Empty state now also surfaces the primary action inline so the
    // button appears in both the header and the empty-state body. Click
    // either; both wire to the same callback.
    const buttons = screen.getAllByRole('button', { name: /Add connection/i });
    expect(buttons.length).toBeGreaterThan(0);
    fireEvent.click(buttons[0]);
    expect(onAdd).toHaveBeenCalledTimes(1);
  });

  it('shows lastErrorMessage inline on the row when present', () => {
    render(() => (
      <ConnectionsTable rows={() => [row({ lastErrorMessage: 'certificate expired' })]} />
    ));

    expect(screen.getByText('certificate expired')).toBeInTheDocument();
  });

  it('renders compact fleet posture chips for operator attention states', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row({
            fleetHighlights: [
              {
                key: 'config-drift',
                label: 'Config drift',
                detail: 'Desired and applied fingerprints differ.',
                tone: 'warning',
              },
              {
                key: 'command-policy',
                label: 'Remote control disabled',
                detail: 'Commands are disabled by policy.',
                tone: 'info',
              },
              {
                key: 'config-drift',
                label: 'Config pending',
                detail: 'Waiting for applied configuration confirmation.',
                tone: 'warning',
              },
              {
                key: 'config-drift',
                label: 'Config unknown',
                detail: 'No desired/applied config fingerprint has been reported yet.',
                tone: 'warning',
              },
            ],
          }),
        ]}
      />
    ));

    expect(screen.getByText('Posture')).toBeInTheDocument();
    expect(screen.getByText('Config drift')).toHaveAttribute(
      'title',
      'Desired and applied fingerprints differ.',
    );
    expect(screen.getByText('Remote control disabled')).toHaveAttribute(
      'title',
      'Commands are disabled by policy.',
    );
    expect(screen.getByText('Config pending')).toHaveAttribute(
      'title',
      'Waiting for applied configuration confirmation.',
    );
    expect(screen.getByText('Config unknown')).toHaveAttribute(
      'title',
      'No desired/applied config fingerprint has been reported yet.',
    );
  });

  it('keeps large row sets bounded behind an explicit show-more path', () => {
    const rows = Array.from({ length: CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS + 5 }, (_, index) =>
      row({
        id: `system-${index}`,
        name: `system-${index}`,
        host: undefined,
        connection: connectionFixture({
          id: `system-${index}`,
          name: `system-${index}`,
        }),
      }),
    );

    render(() => <ConnectionsTable rows={() => rows} />);

    expect(screen.getByText('system-0')).toBeInTheDocument();
    expect(
      screen.getByText(`system-${CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS - 1}`),
    ).toBeInTheDocument();
    expect(screen.queryByText(`system-${CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS}`)).toBeNull();
    expect(
      screen.getAllByText(
        `Showing ${CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS} of ${rows.length} monitored systems. 5 more systems available.`,
      ),
    ).toHaveLength(2);

    fireEvent.click(screen.getByRole('button', { name: 'Show remaining 5 monitored systems.' }));

    expect(
      screen.getByText(`system-${CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS}`),
    ).toBeInTheDocument();
    expect(screen.getByText(`Showing all ${rows.length} monitored systems.`)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Show remaining/i })).toBeNull();
  });

  it('renders Edit / Pause / Remove buttons when actions and onEdit are provided', () => {
    const onEdit = vi.fn();
    const actions = makeActions();
    render(() => <ConnectionsTable rows={() => [row()]} actions={actions} onEdit={onEdit} />);

    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pause' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
  });

  it('invokes onEdit with the underlying Connection', () => {
    const onEdit = vi.fn();
    const connection = connectionFixture({ id: 'pve:zeus', name: 'zeus' });
    render(() => (
      <ConnectionsTable
        rows={() => [row({ connection })]}
        actions={makeActions()}
        onEdit={onEdit}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onEdit).toHaveBeenCalledWith(connection);
  });

  it('invokes togglePause with the Connection when Pause is clicked', () => {
    const togglePause = vi.fn();
    const actions = makeActions({ togglePause });
    const connection = connectionFixture();
    render(() => (
      <ConnectionsTable rows={() => [row({ connection })]} actions={actions} onEdit={vi.fn()} />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    expect(togglePause).toHaveBeenCalledWith(connection);
  });

  it('labels the pause button Resume when the connection is already paused', () => {
    const connection = connectionFixture({ enabled: false });
    render(() => (
      <ConnectionsTable
        rows={() => [row({ connection, enabled: false })]}
        actions={makeActions()}
      />
    ));

    expect(screen.getByRole('button', { name: 'Resume' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Pause' })).toBeNull();
  });

  it('surfaces the row-specific actionError inside an alert', () => {
    const actions = makeActions({ actionError: () => 'permission denied' });
    render(() => <ConnectionsTable rows={() => [row()]} actions={actions} onEdit={vi.fn()} />);

    expect(screen.getByRole('alert')).toHaveTextContent('permission denied');
  });

  it('swaps the remove button into a confirming state when confirmingRemove is true', () => {
    const actions = makeActions({ confirmingRemove: () => true });
    render(() => <ConnectionsTable rows={() => [row()]} actions={actions} onEdit={vi.fn()} />);

    expect(screen.getByRole('button', { name: /Click again to confirm/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remove' })).toBeNull();
  });

  it('tells the truth on Platform API remove confirm: history retained, platform untouched', () => {
    const actions = makeActions({ confirmingRemove: () => true });
    const pveConnection = connectionFixture({
      id: 'pve:zeus',
      type: 'pve',
      name: 'zeus',
    });
    render(() => (
      <ConnectionsTable
        rows={() => [row({ connection: pveConnection, isAgent: false })]}
        actions={actions}
        onEdit={vi.fn()}
      />
    ));

    expect(screen.getByText(/history is retained/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Credentials on the platform itself are untouched/i),
    ).toBeInTheDocument();
    // No uninstall block for Platform API rows — only agents get that courtesy.
    expect(screen.queryByText(/--uninstall/)).toBeNull();
  });

  it('reveals the agent uninstall commands while confirming removal of an agent row', () => {
    const actions = makeActions({ confirmingRemove: () => true });
    const agentConnection = connectionFixture({
      id: 'agent:host-1',
      type: 'agent',
      name: 'host-1',
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    render(() => (
      <ConnectionsTable
        rows={() => [
          row({
            connection: agentConnection,
            isAgent: true,
            canPause: false,
          }),
        ]}
        actions={actions}
        onEdit={vi.fn()}
        agentUninstallCommands={{
          linux: 'curl -fsSL http://pulse/install.sh | bash -s -- --uninstall',
          windows:
            '$env:PULSE_URL="http://pulse"; $env:PULSE_UNINSTALL="true"; iwr /install.ps1 | iex',
        }}
      />
    ));

    expect(screen.getByText(/Removing forgets this agent/i)).toBeInTheDocument();
    expect(
      screen.getByText(/curl -fsSL http:\/\/pulse\/install\.sh \| bash -s -- --uninstall/),
    ).toBeInTheDocument();
    expect(screen.getByText(/\$env:PULSE_UNINSTALL="true"/)).toBeInTheDocument();
  });

  it('does not render action buttons when the row model forbids them', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row({
            canEdit: false,
            canPause: false,
            canRemove: false,
          }),
        ]}
        actions={makeActions()}
        onEdit={vi.fn()}
      />
    ));

    expect(screen.queryByRole('button', { name: 'Edit' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Pause' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Remove' })).toBeNull();
  });
});

describe('connectionAgentIdentitySummary', () => {
  const withAgentIdentity = (
    agentIdentity: Partial<ConnectionAgentIdentity> | undefined,
  ): Connection =>
    connectionFixture({ agentIdentity: agentIdentity as ConnectionAgentIdentity | undefined });

  it('uses osName platform identity over the broader agent platform family', () => {
    // Pi/delly/minipc shape: agent reports platform="debian"/"raspbian" because
    // Proxmox VE is Debian-based, but osName carries the canonical identity.
    // Display must surface the platform identity, not the OS family.
    const summary = connectionAgentIdentitySummary(
      withAgentIdentity({
        platform: 'debian',
        osName: 'Proxmox VE',
        osVersion: '9.1.9',
      }),
    );
    expect(summary).toBe('Proxmox VE 9.1.9');
  });

  it('uses Unraid osName over Linux platform family', () => {
    const summary = connectionAgentIdentitySummary(
      withAgentIdentity({
        platform: 'linux',
        osName: 'Unraid',
        osVersion: '7.2.2',
      }),
    );
    expect(summary).toBe('Unraid 7.2.2');
  });

  it('falls back to prettified platform when osName is missing', () => {
    const summary = connectionAgentIdentitySummary(
      withAgentIdentity({
        platform: 'linux',
        osVersion: '6.1.0',
      }),
    );
    expect(summary).toBe('Linux 6.1.0');
  });

  it('returns null when no agent identity is present', () => {
    const summary = connectionAgentIdentitySummary(withAgentIdentity(undefined));
    expect(summary).toBeNull();
  });
});
