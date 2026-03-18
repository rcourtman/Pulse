import { fireEvent, render, screen } from '@solidjs/testing-library';
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
        sourceOptions={[
          { key: 'all', label: 'All Sources', tone: 'slate' },
          { key: 'proxmox-pve', label: 'PVE', tone: 'orange' },
        ]}
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
});
