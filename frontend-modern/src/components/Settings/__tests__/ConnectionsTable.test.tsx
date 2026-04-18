import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsTable } from '../ConnectionsTable';
import type { InfrastructureSystemRow } from '../connectionsTableModel';

const row = (overrides: Partial<InfrastructureSystemRow> = {}): InfrastructureSystemRow => ({
  id: 'row-1',
  name: 'tower',
  subtitle: 'Monitored system',
  host: '10.0.0.1',
  coverageLabels: ['Host telemetry'],
  collectionLabel: 'Agent',
  statusLabel: 'online',
  statusClassName: 'bg-green-100 text-green-800',
  lastActivityText: '5s ago',
  manageLabel: 'View details',
  manage: { kind: 'inventory-active', rowKey: 'agent:tower' },
  ...overrides,
});

describe('ConnectionsTable', () => {
  afterEach(() => cleanup());

  it('renders an empty-state hint when no rows exist', () => {
    render(() => (
      <ConnectionsTable rows={() => []} />
    ) as any);

    expect(screen.getByText(/No monitored systems yet/i)).toBeInTheDocument();
    expect(screen.queryByRole('table')).toBeNull();
  });

  it('renders one row per top-level monitored system with coverage, collection, and status labels', () => {
    render(() => (
      <ConnectionsTable
        rows={() => [
          row(),
          row({
            id: 'row-2',
            name: 'pbs-docker',
            subtitle: 'Ignored by Pulse',
            host: undefined,
            coverageLabels: ['PBS data'],
            collectionLabel: 'API',
            statusLabel: 'Ignored',
            manageLabel: 'Review ignored',
            manage: { kind: 'inventory-ignored', rowKey: 'removed:pbs-docker' },
          }),
        ]}
      />
    ) as any);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('pbs-docker')).toBeInTheDocument();
    expect(screen.getByText('Monitored system')).toBeInTheDocument();
    expect(screen.getByText('Ignored by Pulse')).toBeInTheDocument();
    expect(screen.getByText('Host telemetry')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('Agent')).toBeInTheDocument();
    expect(screen.getByText('Ignored')).toBeInTheDocument();
    expect(screen.getByText('online')).toBeInTheDocument();
  });

  it('surfaces configured header actions when provided', () => {
    const onAddSystem = vi.fn();
    render(() => (
      <ConnectionsTable
        rows={() => []}
        headerActions={[
          {
            label: 'Add infrastructure',
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
    const button = screen.getByRole('button', { name: /Add infrastructure/i });
    fireEvent.click(button);
    expect(onAddSystem).toHaveBeenCalledTimes(1);
  });

  it('routes per-row manage actions through the provided callback', () => {
    const onManageRow = vi.fn();
    render(() => (
      <ConnectionsTable rows={() => [row()]} onManageRow={onManageRow} />
    ) as any);

    fireEvent.click(screen.getByRole('button', { name: 'View details' }));
    expect(onManageRow).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'row-1',
        manage: { kind: 'inventory-active', rowKey: 'agent:tower' },
      }),
    );
  });
});
