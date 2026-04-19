import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConnectionDetailPanel } from '../ConnectionDetailPanel';
import type { Connection } from '@/api/connections';

const setEnabled = vi.fn<(connectionId: string, enabled: boolean) => Promise<void>>();
const remove = vi.fn<(connectionId: string) => Promise<void>>();

vi.mock('@/api/connections', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/connections')>();
  return {
    ...actual,
    ConnectionsAPI: {
      ...actual.ConnectionsAPI,
      setEnabled: (...args: Parameters<typeof actual.ConnectionsAPI.setEnabled>) =>
        setEnabled(...args),
      remove: (...args: Parameters<typeof actual.ConnectionsAPI.remove>) => remove(...args),
    },
  };
});

const pveConnection = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:tower',
  type: 'pve',
  name: 'tower',
  address: 'https://tower.local:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms', 'containers'],
  scope: { vms: true, containers: true },
  lastSeen: null,
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const agentConnection = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'agent:host-1',
  type: 'agent',
  name: 'tower.local',
  address: 'tower.local',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['host'],
  scope: { host: true },
  lastSeen: null,
  lastError: null,
  source: 'agent',
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
  ...overrides,
});

describe('ConnectionDetailPanel', () => {
  beforeEach(() => {
    setEnabled.mockReset();
    remove.mockReset();
  });
  afterEach(() => cleanup());

  it('hides pause for agent connections but still allows remove', () => {
    render(() => (
      <ConnectionDetailPanel
        connection={() => agentConnection()}
        onMutated={() => {}}
      />
    ));

    expect(screen.queryByRole('button', { name: /Pause/i })).toBeNull();
    expect(screen.getByRole('button', { name: /Remove/i })).toBeInTheDocument();
    expect(screen.getByText(/Removing stops recording this agent/i)).toBeInTheDocument();
  });

  it('shows Edit for pve connections and invokes onEdit with the connection', () => {
    const onEdit = vi.fn();
    render(() => (
      <ConnectionDetailPanel
        connection={() => pveConnection()}
        onMutated={() => {}}
        onEdit={onEdit}
      />
    ));

    const editButton = screen.getByRole('button', { name: 'Edit' });
    fireEvent.click(editButton);
    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onEdit.mock.calls[0][0]).toMatchObject({ id: 'pve:tower', type: 'pve' });
  });

  it('shows Edit for vmware connections', () => {
    const onEdit = vi.fn();
    render(() => (
      <ConnectionDetailPanel
        connection={() =>
          pveConnection({ id: 'vmware:abc', type: 'vmware', name: 'vcsa' })
        }
        onMutated={() => {}}
        onEdit={onEdit}
      />
    ));

    const editButton = screen.getByRole('button', { name: 'Edit' });
    fireEvent.click(editButton);
    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onEdit.mock.calls[0][0]).toMatchObject({ id: 'vmware:abc', type: 'vmware' });
  });

  it('shows Edit for truenas connections', () => {
    const onEdit = vi.fn();
    render(() => (
      <ConnectionDetailPanel
        connection={() =>
          pveConnection({ id: 'truenas:xyz', type: 'truenas', name: 'tower' })
        }
        onMutated={() => {}}
        onEdit={onEdit}
      />
    ));

    const editButton = screen.getByRole('button', { name: 'Edit' });
    fireEvent.click(editButton);
    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onEdit.mock.calls[0][0]).toMatchObject({ id: 'truenas:xyz', type: 'truenas' });
  });

  it('omits Edit for agent connections (edit is not yet supported)', () => {
    render(() => (
      <ConnectionDetailPanel
        connection={() => agentConnection()}
        onMutated={() => {}}
        onEdit={() => {}}
      />
    ));

    expect(screen.queryByRole('button', { name: 'Edit' })).toBeNull();
  });

  it('toggles pause via ConnectionsAPI and calls onMutated on success', async () => {
    setEnabled.mockResolvedValueOnce(undefined);
    const onMutated = vi.fn();

    render(() => (
      <ConnectionDetailPanel
        connection={() => pveConnection()}
        onMutated={onMutated}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));

    await waitFor(() => {
      expect(setEnabled).toHaveBeenCalledWith('pve:tower', false);
      expect(onMutated).toHaveBeenCalledTimes(1);
    });
  });

  it('shows the returned error inline when pause fails', async () => {
    setEnabled.mockRejectedValueOnce(new Error('license limit reached'));
    const onMutated = vi.fn();

    render(() => (
      <ConnectionDetailPanel
        connection={() => pveConnection()}
        onMutated={onMutated}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('license limit reached');
    });
    expect(onMutated).not.toHaveBeenCalled();
  });

  it('requires a second click to confirm removal, then calls remove + onRemoved + onMutated', async () => {
    remove.mockResolvedValueOnce(undefined);
    const onMutated = vi.fn();
    const onRemoved = vi.fn();

    render(() => (
      <ConnectionDetailPanel
        connection={() => pveConnection()}
        onMutated={onMutated}
        onRemoved={onRemoved}
      />
    ));

    const removeButton = screen.getByRole('button', { name: 'Remove' });
    fireEvent.click(removeButton);

    expect(remove).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: /Click again to confirm/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Click again to confirm/i }));

    await waitFor(() => {
      expect(remove).toHaveBeenCalledWith('pve:tower');
      expect(onMutated).toHaveBeenCalledTimes(1);
      expect(onRemoved).toHaveBeenCalledTimes(1);
    });
  });
});
