import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import { ResourcePicker, type SelectedResource } from '../ResourcePicker';

let mockResources: Resource[] = [];
const showWarningMock = vi.fn();

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => mockResources,
  }),
  getDisplayName: (resource: { displayName?: string; name?: string; id: string }) =>
    resource.displayName || resource.name || resource.id,
}));

vi.mock('@/utils/toast', () => ({
  showWarning: (...args: unknown[]) => showWarningMock(...args),
}));

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'vm-1',
  type: 'vm',
  name: 'Alpha VM',
  displayName: 'Alpha VM',
  platformId: 'pve-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.now(),
  tags: [],
  ...overrides,
});

const createSelectedResources = (count: number): SelectedResource[] =>
  Array.from({ length: count }, (_, index) => ({
    id: `preselected-${index}`,
    type: 'vm',
    name: `Preselected ${index}`,
  }));

const renderPicker = (initialSelection: SelectedResource[] = []) => {
  const onSelectionChange = vi.fn();
  render(() => {
    const [selected, setSelected] = createSignal<SelectedResource[]>(initialSelection);
    const handleSelectionChange = (items: SelectedResource[]) => {
      setSelected(items);
      onSelectionChange(items);
    };
    return <ResourcePicker selected={selected} onSelectionChange={handleSelectionChange} />;
  });
  return { onSelectionChange };
};

beforeEach(() => {
  showWarningMock.mockReset();
  mockResources = [];
});

afterEach(() => {
  cleanup();
});

describe('ResourcePicker', () => {
  it('renders reportable resources from useResources()', async () => {
    mockResources = [
      makeResource({ id: 'node-1', type: 'node', name: 'Node One', displayName: 'Node One', status: 'online' }),
      makeResource({ id: 'vm-1', type: 'vm', name: 'Alpha VM', displayName: 'Alpha VM' }),
      makeResource({ id: 'truenas-1', type: 'truenas', name: 'TrueNAS', displayName: 'TrueNAS' }),
    ];

    renderPicker();

    expect(await screen.findByText('Node One')).toBeInTheDocument();
    expect(screen.getByText('Alpha VM')).toBeInTheDocument();
    expect(screen.queryByText('TrueNAS')).not.toBeInTheDocument();
  });

  it('applies type filter buttons', async () => {
    mockResources = [
      makeResource({ id: 'node-1', type: 'node', name: 'Node One', displayName: 'Node One', status: 'online' }),
      makeResource({ id: 'vm-1', type: 'vm', name: 'Workload VM', displayName: 'Workload VM' }),
      makeResource({ id: 'storage-1', type: 'storage', name: 'Storage Main', displayName: 'Storage Main', status: 'online' }),
      makeResource({ id: 'pbs-1', type: 'pbs', name: 'Backup Server', displayName: 'Backup Server', status: 'online' }),
    ];

    renderPicker();

    fireEvent.click(screen.getByRole('button', { name: 'Infrastructure' }));
    expect(await screen.findByText('Node One')).toBeInTheDocument();
    expect(screen.queryByText('Workload VM')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Workloads' }));
    expect(await screen.findByText('Workload VM')).toBeInTheDocument();
    expect(screen.queryByText('Storage Main')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Storage' }));
    expect(await screen.findByText('Storage Main')).toBeInTheDocument();
    expect(screen.queryByText('Backup Server')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Recovery' }));
    expect(await screen.findByText('Backup Server')).toBeInTheDocument();
    expect(screen.queryByText('Workload VM')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'All' }));
    expect(await screen.findByText('Node One')).toBeInTheDocument();
    expect(screen.getByText('Workload VM')).toBeInTheDocument();
  });

  it('filters resources by search on display name and ID', async () => {
    mockResources = [
      makeResource({ id: 'vm-prod-101', type: 'vm', name: 'Production VM', displayName: 'Production VM' }),
      makeResource({ id: 'host-dev-55', type: 'host', name: 'Edge Host', displayName: 'Edge Host', status: 'online' }),
    ];

    renderPicker();

    const searchInput = screen.getByPlaceholderText('Search by name or ID...');

    fireEvent.input(searchInput, { target: { value: 'production' } });
    expect(await screen.findByText('Production VM')).toBeInTheDocument();
    expect(screen.queryByText('Edge Host')).not.toBeInTheDocument();

    fireEvent.input(searchInput, { target: { value: 'host-dev' } });
    expect(await screen.findByText('Edge Host')).toBeInTheDocument();
    expect(screen.queryByText('Production VM')).not.toBeInTheDocument();
  });

  it('toggles individual selection on and off', async () => {
    mockResources = [
      makeResource({ id: 'vm-1', type: 'vm', name: 'Alpha VM', displayName: 'Alpha VM' }),
    ];

    const { onSelectionChange } = renderPicker();

    const resourceButton = (await screen.findByText('Alpha VM')).closest('button');
    expect(resourceButton).toBeTruthy();

    fireEvent.click(resourceButton!);
    await waitFor(() => {
      expect(screen.getByText('1 selected')).toBeInTheDocument();
    });
    expect(onSelectionChange).toHaveBeenCalledWith([
      { id: 'vm-1', type: 'vm', name: 'Alpha VM' },
    ]);

    fireEvent.click(resourceButton!);
    await waitFor(() => {
      expect(screen.getByText('0 selected')).toBeInTheDocument();
    });
  });

  it('enforces the max selection limit when toggling', async () => {
    mockResources = [
      makeResource({ id: 'vm-new', type: 'vm', name: 'Overflow VM', displayName: 'Overflow VM' }),
    ];

    const { onSelectionChange } = renderPicker(createSelectedResources(50));

    const resourceButton = (await screen.findByText('Overflow VM')).closest('button');
    expect(resourceButton).toBeTruthy();
    fireEvent.click(resourceButton!);

    expect(showWarningMock).toHaveBeenCalledWith('Maximum 50 resources can be selected');
    expect(screen.getByText('50 selected')).toBeInTheDocument();
    expect(onSelectionChange).not.toHaveBeenCalled();
  });

  it('supports select-all-visible and clear-all', async () => {
    mockResources = [
      makeResource({ id: 'node-1', type: 'node', name: 'Node One', displayName: 'Node One', status: 'online' }),
      makeResource({ id: 'vm-1', type: 'vm', name: 'Alpha VM', displayName: 'Alpha VM' }),
      makeResource({ id: 'storage-1', type: 'storage', name: 'Storage Main', displayName: 'Storage Main', status: 'online' }),
    ];

    renderPicker();

    fireEvent.click(screen.getByRole('button', { name: /Select all visible \(3\)/i }));
    await waitFor(() => {
      expect(screen.getByText('3 selected')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));
    await waitFor(() => {
      expect(screen.getByText('0 selected')).toBeInTheDocument();
    });
  });
});
