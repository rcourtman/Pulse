import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FilterBar } from './FilterBar';
import type { FilterDef } from './filterCatalog';

const search = {
  value: () => '',
  setValue: vi.fn(),
  placeholder: 'Search',
};

const inlineTypeFilter = (setValue = vi.fn(), value = 'all'): FilterDef => ({
  id: 'type',
  label: 'Type',
  inline: true,
  value: () => value,
  setValue,
  defaultValue: 'all',
  options: () => [
    { value: 'all', label: 'All' },
    { value: 'vm', label: 'VMs' },
  ],
});

const menuNodeFilter = (setValue = vi.fn(), value = ''): FilterDef => ({
  id: 'node',
  label: 'Node',
  group: 'scope',
  value: () => value,
  setValue,
  defaultValue: '',
  options: () => [
    { value: '', label: 'All nodes' },
    { value: 'pve1', label: 'pve1' },
  ],
});

describe('FilterBar', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders inline filters as one-click segmented controls outside the menu', () => {
    const setType = vi.fn();

    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(setType), menuNodeFilter()]}
        isMobile={() => false}
      />
    ));

    const typeGroup = screen.getByRole('group', { name: 'Type' });
    fireEvent.click(within(typeGroup).getByRole('button', { name: 'VMs' }));

    expect(setType).toHaveBeenCalledWith('vm');
    const filterSelect = screen.getByRole('combobox', { name: 'Filter' });
    expect(
      within(filterSelect).queryByRole('option', { name: 'Type: VMs' }),
    ).not.toBeInTheDocument();
    expect(within(filterSelect).getByRole('option', { name: 'Node: pve1' })).toBeInTheDocument();
  });

  it('hides the add-filter trigger when every filter is inline', () => {
    render(() => (
      <FilterBar search={search} filters={[inlineTypeFilter()]} isMobile={() => false} />
    ));

    expect(screen.getByRole('group', { name: 'Type' })).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Filter' })).not.toBeInTheDocument();
  });

  it('shows clear-all beside inline controls when only inline filters are active', () => {
    const onClearAll = vi.fn();

    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(vi.fn(), 'vm')]}
        isMobile={() => false}
        onClearAll={onClearAll}
      />
    ));

    expect(
      within(screen.getByRole('group', { name: 'Type' })).getByRole('button', { name: 'VMs' }),
    ).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(onClearAll).toHaveBeenCalledTimes(1);
  });
});
