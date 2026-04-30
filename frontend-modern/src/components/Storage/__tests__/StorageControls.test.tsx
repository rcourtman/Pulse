import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import { StorageControls } from '@/components/Storage/StorageControls';
import type { StorageSourceOption } from '@/utils/storageSources';

describe('StorageControls', () => {
  it('renders the shared storage controls and node filter', () => {
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');
    const [search, setSearch] = createSignal('');
    const [groupBy, setGroupBy] = createSignal<'none' | 'node'>('node');
    const [sortKey, setSortKey] = createSignal<'priority' | 'name' | 'usage' | 'type'>('name');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
    const [statusFilter, setStatusFilter] = createSignal<'all' | 'warning' | 'critical'>('all');
    const [sourceFilter, setSourceFilter] = createSignal('all');
    const [sourceOptions] = createSignal<StorageSourceOption[]>([
      { key: 'all', label: 'All sources', tone: 'slate' as const },
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
          { value: 'all', label: 'All nodes' },
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
    const [statusFilter, setStatusFilter] = createSignal<'all' | 'warning' | 'critical'>('all');
    const [sourceFilter, setSourceFilter] = createSignal('truenas');
    const [sourceOptions, setSourceOptions] = createSignal<StorageSourceOption[]>([
      { key: 'all', label: 'All sources', tone: 'slate' as const },
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
        nodeFilterOptions={[{ value: 'all', label: 'All nodes' }]}
        selectedNodeId={selectedNodeId}
        setSelectedNodeId={setSelectedNodeId}
      />
    ));

    setSourceOptions([
      { key: 'all', label: 'All sources', tone: 'slate' as const },
      { key: 'proxmox-pve', label: 'PVE', tone: 'orange' as const },
      { key: 'truenas', label: 'TrueNAS', tone: 'blue' as const },
    ]);

    expect(
      Array.from(screen.getByLabelText('Source').querySelectorAll('option')).map((option) => ({
        value: option.value,
        label: option.textContent,
      })),
    ).toEqual([
      { value: 'all', label: 'All sources' },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'truenas', label: 'TrueNAS' },
    ]);
    await waitFor(() => {
      expect(screen.getByLabelText('Source')).toHaveValue('truenas');
    });
  });

  it('routes the charts toggle through the shared toolbar action rail', () => {
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');
    const [search, setSearch] = createSignal('');
    const [groupBy, setGroupBy] = createSignal<'none' | 'node'>('node');
    const [sortKey, setSortKey] = createSignal<'priority' | 'name' | 'usage' | 'type'>('name');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
    const [statusFilter, setStatusFilter] = createSignal<'all' | 'warning' | 'critical'>('all');
    const [sourceFilter, setSourceFilter] = createSignal('all');
    const [sourceOptions] = createSignal<StorageSourceOption[]>([
      { key: 'all', label: 'All sources', tone: 'slate' as const },
    ]);
    const [selectedNodeId, setSelectedNodeId] = createSignal('all');
    const [chartsCollapsed] = createSignal(false);
    const onChartsToggle = vi.fn();

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
        nodeFilterOptions={[{ value: 'all', label: 'All nodes' }]}
        selectedNodeId={selectedNodeId}
        setSelectedNodeId={setSelectedNodeId}
        chartsCollapsed={chartsCollapsed}
        onChartsToggle={onChartsToggle}
      />
    ));

    const chartsButton = screen.getByRole('button', { name: 'Hide charts' });
    expect(chartsButton.closest('.page-controls-toolbar-actions')).not.toBeNull();
    expect(chartsButton).toHaveTextContent('Charts');
    expect(chartsButton).toHaveAttribute('aria-pressed', 'true');
    expect(chartsButton).toHaveAttribute('title', 'Hide charts');

    fireEvent.click(chartsButton);

    expect(onChartsToggle).toHaveBeenCalledTimes(1);
  });
});
