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
    const placeholder = filterSelect.querySelector('option[value=""]');
    expect(placeholder).toHaveTextContent('Add filter');
    expect(placeholder).toBeDisabled();
    expect(placeholder).toHaveAttribute('hidden');
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

  it('hides the add-filter trigger when every menu filter is already active', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter(vi.fn(), 'pve1')]}
        isMobile={() => false}
      />
    ));

    expect(screen.getByRole('button', { name: 'Remove Node filter' })).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Filter' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
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

    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
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
    expect(screen.getAllByRole('button', { name: 'Clear filters' })).toHaveLength(1);
  });

  it('keeps Saved, clear filters, and View together in the mobile action rail', () => {
    renderInRouter(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(vi.fn(), 'vm')]}
        isMobile={() => true}
        savedViewsKey="test-surface"
        viewOptions={<span>Density</span>}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /^Filters/ }));

    const actions = screen.getByRole('group', { name: 'Filter actions' });
    expect(within(actions).getByRole('button', { name: 'Saved views' })).toBeInTheDocument();
    expect(within(actions).getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
    expect(within(actions).getByRole('button', { name: 'View' })).toBeInTheDocument();
  });

  it('keeps clear filters with filter actions before the presentation controls', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(vi.fn(), 'vm')]}
        isMobile={() => false}
        viewOptions={<span>Density</span>}
      />
    ));

    const labels = screen
      .getAllByRole('button')
      .map((button) => button.textContent?.trim())
      .filter(Boolean);
    expect(labels.indexOf('Clear filters')).toBeLessThan(labels.indexOf('View'));
  });

  it('owns View composition while keeping contextual and orientation controls visible', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter()]}
        isMobile={() => false}
        leadingControls={<button type="button">Clear selection</button>}
        viewOptions={<span>Density</span>}
        trailingControls={<span>12 items</span>}
      />
    ));

    expect(screen.getByRole('button', { name: 'Clear selection' })).toBeInTheDocument();
    expect(screen.getByText('12 items')).toBeInTheDocument();
    expect(screen.queryByText('Density')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View' }));

    expect(screen.getByRole('dialog', { name: 'View preferences' })).toHaveTextContent('Density');
  });

  it('reveals the shared View trigger with persistent controls in the expanded mobile rail', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter()]}
        isMobile={() => true}
        viewOptions={<span>Density</span>}
        trailingControls={<span>12 items</span>}
      />
    ));

    expect(screen.queryByRole('button', { name: 'View' })).not.toBeInTheDocument();
    expect(screen.queryByText('12 items')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Filters' }));

    expect(screen.getByRole('button', { name: 'View' })).toBeInTheDocument();
    expect(screen.getByText('12 items')).toBeInTheDocument();
  });

  it('can hide the redundant visual Filter label without changing the accessible name', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter()]}
        isMobile={() => false}
        showAddFilterLabel={false}
      />
    ));

    const select = screen.getByRole('combobox', { name: 'Filter' });
    const label = document.querySelector(`label[for="${select.id}"]`);
    expect(label).toHaveClass('sr-only');
    expect(select).toHaveClass('h-7');
    expect(select.parentElement).not.toHaveClass('p-0.5');
  });
});
