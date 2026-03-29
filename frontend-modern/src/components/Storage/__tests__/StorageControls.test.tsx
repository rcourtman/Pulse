import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { StorageControls } from '@/components/Storage/StorageControls';

describe('StorageControls', () => {
  it('renders the shared storage controls and node filter', () => {
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');
    const [search, setSearch] = createSignal('');
    const [groupBy, setGroupBy] = createSignal<'none' | 'node'>('node');
    const [sortKey, setSortKey] = createSignal<'priority' | 'name' | 'usage' | 'type'>('name');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
    const [statusFilter, setStatusFilter] =
      createSignal<'all' | 'warning' | 'critical'>('all');
    const [sourceFilter, setSourceFilter] = createSignal('all');
    const [sourceOptions] = createSignal([
      { key: 'all', label: 'All Sources', tone: 'slate' as const },
      { key: 'proxmox-pve', label: 'PVE', tone: 'orange' as const },
    ]);
    const [selectedNodeId, setSelectedNodeId] = createSignal('all');

    render(() => (
      <StorageControls
        view={view()}
        onViewChange={setView}
        search={search}
        setSearch={setSearch}
        groupBy={groupBy}
        setGroupBy={setGroupBy}
        sortKey={sortKey}
        setSortKey={setSortKey}
        sortDirection={sortDirection}
        setSortDirection={setSortDirection}
        statusFilter={statusFilter}
        setStatusFilter={setStatusFilter}
        sourceFilter={sourceFilter}
        setSourceFilter={setSourceFilter}
        sourceOptions={sourceOptions}
        nodeFilterOptions={[
          { value: 'all', label: 'All Nodes' },
          { value: 'node-1', label: 'pve1' },
        ]}
        selectedNodeId={selectedNodeId}
        setSelectedNodeId={setSelectedNodeId}
      />
    ));

    expect(screen.getByLabelText('Storage view')).toBeInTheDocument();
    expect(screen.getByLabelText('Node')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Node'), {
      target: { value: 'node-1' },
    });
    expect(selectedNodeId()).toBe('node-1');
  });

  it('updates source filter options when storage data arrives after first render', async () => {
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');
    const [search, setSearch] = createSignal('');
    const [groupBy, setGroupBy] = createSignal<'none' | 'node'>('node');
    const [sortKey, setSortKey] = createSignal<'priority' | 'name' | 'usage' | 'type'>('name');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
    const [statusFilter, setStatusFilter] =
      createSignal<'all' | 'warning' | 'critical'>('all');
    const [sourceFilter, setSourceFilter] = createSignal('truenas');
    const [sourceOptions, setSourceOptions] = createSignal([
      { key: 'all', label: 'All Sources', tone: 'slate' as const },
    ]);
    const [selectedNodeId, setSelectedNodeId] = createSignal('all');

    render(() => (
      <StorageControls
        view={view()}
        onViewChange={setView}
        search={search}
        setSearch={setSearch}
        groupBy={groupBy}
        setGroupBy={setGroupBy}
        sortKey={sortKey}
        setSortKey={setSortKey}
        sortDirection={sortDirection}
        setSortDirection={setSortDirection}
        statusFilter={statusFilter}
        setStatusFilter={setStatusFilter}
        sourceFilter={sourceFilter}
        setSourceFilter={setSourceFilter}
        sourceOptions={sourceOptions}
        nodeFilterOptions={[{ value: 'all', label: 'All Nodes' }]}
        selectedNodeId={selectedNodeId}
        setSelectedNodeId={setSelectedNodeId}
      />
    ));

    setSourceOptions([
      { key: 'all', label: 'All Sources', tone: 'slate' as const },
      { key: 'proxmox-pve', label: 'PVE', tone: 'orange' as const },
      { key: 'truenas', label: 'TrueNAS', tone: 'blue' as const },
    ]);

    expect(
      Array.from(screen.getByLabelText('Source').querySelectorAll('option')).map((option) => ({
        value: option.value,
        label: option.textContent,
      })),
    ).toEqual([
      { value: 'all', label: 'All Sources' },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'truenas', label: 'TrueNAS' },
    ]);
    await waitFor(() => {
      expect(screen.getByLabelText('Source')).toHaveValue('truenas');
    });
  });
});
