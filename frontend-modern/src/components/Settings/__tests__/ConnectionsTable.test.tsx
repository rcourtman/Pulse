import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsTable } from '../ConnectionsTable';
import type { ConnectionRow } from '../connectionsTableModel';

const row = (overrides: Partial<ConnectionRow> = {}): ConnectionRow => ({
  id: 'pve:pve-1',
  kind: 'pve',
  kindLabel: 'Proxmox VE',
  method: 'api',
  methodLabel: 'API',
  name: 'production-pve',
  host: '10.0.0.1',
  status: 'reporting',
  statusLabel: 'Reporting',
  lastReportedMs: Date.now() - 5_000,
  ...overrides,
});

describe('ConnectionsTable', () => {
  afterEach(() => cleanup());

  it('renders an empty-state hint when no rows exist', () => {
    render(() => (
      <ConnectionsTable rows={() => []} />
    ) as any);
    expect(screen.getByText(/No systems connected yet/i)).toBeInTheDocument();
    expect(screen.queryByRole('table')).toBeNull();
  });

  it('renders one row per connection with kind, method, and status labels', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row(),
          row({
            id: 'agent:tower',
            kind: 'agent',
            kindLabel: 'Agent host',
            method: 'agent',
            methodLabel: 'Agent',
            name: 'tower',
            host: undefined,
          }),
        ]}
      />
    ) as any);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('production-pve')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getAllByText('Reporting')).toHaveLength(2);
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.getByText('Agent host')).toBeInTheDocument();
  });

  it('surfaces the add-system action only when an onAddSystem handler is provided', () => {
    const onAddSystem = vi.fn();
    render(() => (
      <ConnectionsTable rows={() => []} onAddSystem={onAddSystem} />
    ) as any);

    const button = screen.getByRole('button', { name: /Add a system/i });
    fireEvent.click(button);
    expect(onAddSystem).toHaveBeenCalledTimes(1);
  });
});
