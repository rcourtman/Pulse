import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsTable } from '../ConnectionsTable';
import type { InfrastructureSystemRow } from '../connectionsTableModel';
import type { Connection } from '@/api/connections';
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
    host: '10.0.0.1',
    coverageLabels: ['Host telemetry'],
    sourceBadges: [],
    statusLabel: 'online',
    statusClassName: 'bg-green-100 text-green-800',
    agentUpdateCount: 0,
    lastActivityText: '5s ago',
    lastErrorMessage: undefined,
    enabled: connection.enabled,
    canEdit: true,
    canPause: connection.capabilities.supportsPause,
    canRemove: connection.type !== 'docker' && connection.type !== 'kubernetes',
    isAgent: connection.type === 'agent',
    attachedConnections: [],
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
    expect(
      screen.getByText(/Source types available: VMware vCenter, TrueNAS SCALE/i),
    ).toBeInTheDocument();
    expect(screen.queryByRole('table')).toBeNull();
  });

  it('renders one row per monitored system with coverage and status labels', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row(),
          row({
            id: 'truenas:nas',
            name: 'nas',
            subtitle: 'Platform API · TrueNAS',
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
    expect(screen.getByText('Platform API · TrueNAS')).toBeInTheDocument();
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

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    expect(onAdd).toHaveBeenCalledTimes(1);
  });

  it('shows lastErrorMessage inline on the row when present', () => {
    render(() => (
      <ConnectionsTable rows={() => [row({ lastErrorMessage: 'certificate expired' })]} />
    ));

    expect(screen.getByText('certificate expired')).toBeInTheDocument();
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
