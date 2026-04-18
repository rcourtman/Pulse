import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsTable } from '../ConnectionsTable';
import type { ConnectionRow } from '../connectionsTableModel';

const row = (overrides: Partial<ConnectionRow> = {}): ConnectionRow => ({
  id: 'row-1',
  name: 'production-pve',
  subtitle: 'Configured platform connection',
  host: '10.0.0.1',
  coverageLabels: ['Proxmox VE data'],
  collectionLabel: 'API',
  statusLabel: 'Connected',
  statusClassName: 'bg-green-100 text-green-800',
  lastActivityText: '5s ago',
  manageLabel: 'Edit connection',
  manage: { kind: 'proxmox-node', nodeKind: 'pve', nodeId: 'pve-1' },
  ...overrides,
});

describe('ConnectionsTable', () => {
  afterEach(() => cleanup());

  it('renders an empty-state hint when no rows exist', () => {
    render(() => (
      <ConnectionsTable rows={() => []} />
    ) as any);

    expect(screen.getByText(/Nothing is configured or reporting yet/i)).toBeInTheDocument();
    expect(screen.queryByRole('table')).toBeNull();
  });

  it('renders one row per connection with coverage, collection, and status labels', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row(),
          row({
            id: 'row-2',
            name: 'tower',
            subtitle: 'Live reporting item',
            host: undefined,
            coverageLabels: ['Host telemetry', 'Docker runtime data'],
            collectionLabel: 'Agent',
            statusLabel: 'Reporting',
            manageLabel: 'View details',
            manage: { kind: 'inventory-active', rowKey: 'agent-tower' },
          }),
        ]}
      />
    ) as any);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('production-pve')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE data')).toBeInTheDocument();
    expect(screen.getByText('Host telemetry')).toBeInTheDocument();
    expect(screen.getByText('Docker runtime data')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('Agent')).toBeInTheDocument();
    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(screen.getByText('Reporting')).toBeInTheDocument();
  });

  it('surfaces configured header actions when provided', () => {
    const onAddSystem = vi.fn();
    render(() => (
      <ConnectionsTable
        rows={() => []}
        headerActions={[
          {
            label: '+ Add a system',
            onSelect: onAddSystem,
            tone: 'primary',
          },
          {
            label: 'Agent profiles',
            onSelect: vi.fn(),
            tone: 'secondary',
          },
        ]}
      />
    ) as any);

    expect(screen.getByRole('button', { name: 'Agent profiles' })).toBeInTheDocument();
    const button = screen.getByRole('button', { name: /\+ Add a system/i });
    fireEvent.click(button);
    expect(onAddSystem).toHaveBeenCalledTimes(1);
  });

  it('routes per-row manage actions through the provided callback', () => {
    const onManageRow = vi.fn();
    render(() => (
      <ConnectionsTable rows={() => [row()]} onManageRow={onManageRow} />
    ) as any);

    fireEvent.click(screen.getByRole('button', { name: 'Edit connection' }));
    expect(onManageRow).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'row-1',
        manage: { kind: 'proxmox-node', nodeKind: 'pve', nodeId: 'pve-1' },
      }),
    );
  });
});
