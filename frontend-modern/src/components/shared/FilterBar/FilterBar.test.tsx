import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FilterBar } from './FilterBar';
import type { FilterDef } from './filterCatalog';

// SavedViewsMenu URL-backs view application (useNavigate/useLocation), so
// savedViewsKey renders must sit inside a Router context.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/" component={component} />
    </Router>
  ));

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
    expect(filterSelect).toHaveValue('');
    expect(within(filterSelect).getByRole('option', { name: 'Add filter' })).toBeEnabled();
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

  it('shows saved views in the expanded mobile body when savedViewsKey is set', () => {
    renderInRouter(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter()]}
        isMobile={() => true}
        savedViewsKey="test-surface"
      />
    ));

    expect(screen.queryByRole('button', { name: 'Saved views' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Filters' }));
    expect(screen.getByRole('button', { name: 'Saved views' })).toBeInTheDocument();
  });

  it('shows saved views on mobile even when every filter is inline', () => {
    renderInRouter(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter()]}
        isMobile={() => true}
        savedViewsKey="test-surface"
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Filters' }));
    expect(screen.getByRole('button', { name: 'Saved views' })).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Filter' })).not.toBeInTheDocument();
  });

  it('renders a single clear-all on mobile when the saved-views row is shown', () => {
    renderInRouter(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(vi.fn(), 'vm')]}
        isMobile={() => true}
        savedViewsKey="test-surface"
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /^Filters/ }));
    expect(screen.getAllByRole('button', { name: 'Clear all' })).toHaveLength(1);
  });
});
