import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FilterBar } from './FilterBar';
import type { FilterDef } from './filterCatalog';
import { createSignal } from 'solid-js';

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

  it('renders a single clear-all on mobile when the action row is shown', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(vi.fn(), 'vm')]}
        isMobile={() => true}
        viewOptions={<span>Density</span>}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /^Filters/ }));
    expect(screen.getAllByRole('button', { name: 'Clear filters' })).toHaveLength(1);
  });

  it('keeps clear filters and View together in the mobile action rail', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(vi.fn(), 'vm')]}
        isMobile={() => true}
        viewOptions={<span>Density</span>}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /^Filters/ }));

    const actions = screen.getByRole('group', { name: 'Filter actions' });
    const actionCluster = actions.querySelector('[data-filter-action-cluster]');
    expect(actionCluster).not.toBeNull();
    expect(actionCluster).toContainElement(
      within(actions).getByRole('button', { name: 'Clear filters' }),
    );
    expect(actionCluster).toContainElement(within(actions).getByRole('button', { name: 'View' }));
  });

  it('keeps an otherwise orphaned Add filter with View on mobile', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter()]}
        isMobile={() => true}
        viewOptions={<span>Density</span>}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Filters' }));

    const actions = screen.getByRole('group', { name: 'Filter actions' });
    const actionCluster = actions.querySelector('[data-filter-action-cluster]');
    expect(actionCluster).not.toBeNull();
    expect(actionCluster).toContainElement(
      within(actions).getByRole('combobox', { name: 'Filter' }),
    );
    expect(actionCluster).toContainElement(within(actions).getByRole('button', { name: 'View' }));
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

  it('keeps desktop utilities split on one line and left-aligned after wrapping', () => {
    const { container } = render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter()]}
        isMobile={() => false}
        viewOptions={<span>Density</span>}
      />
    ));

    const actionCluster = container.querySelector('[data-filter-action-cluster]');
    const controlsRail = actionCluster?.parentElement;
    expect(actionCluster).not.toBeNull();
    expect(controlsRail).toHaveClass('flex-wrap', 'justify-between');
    expect(actionCluster).toHaveClass('max-w-full', 'justify-start');
    expect(actionCluster).not.toHaveClass('ml-auto', 'justify-end');
    expect(actionCluster).toContainElement(screen.getByRole('combobox', { name: 'Filter' }));
    expect(actionCluster).toContainElement(screen.getByRole('button', { name: 'View' }));
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

    const viewRegion = screen.getByRole('region', { name: 'View preferences' });
    const actionCluster = screen
      .getByRole('button', { name: 'View' })
      .closest('[data-filter-action-cluster]');
    expect(viewRegion).toHaveTextContent('Density');
    expect(actionCluster).not.toBeNull();
    expect(actionCluster).not.toContainElement(viewRegion);
    expect(viewRegion.closest('.filter-bar')).not.toBeNull();
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

  it('keeps frequent leading context visible while mobile filters are collapsed', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter()]}
        isMobile={() => true}
        leadingControls={<button type="button">Review attention</button>}
        trailingControls={<span>12 items</span>}
      />
    ));

    expect(screen.getByRole('button', { name: 'Review attention' })).toBeInTheDocument();
    expect(screen.queryByText('12 items')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Filters' }));

    const actions = screen.getByRole('group', { name: 'Filter actions' });
    expect(within(actions).getByRole('button', { name: 'Review attention' })).toBeInTheDocument();
    expect(within(actions).getByText('12 items')).toBeInTheDocument();
  });

  it('hides the redundant visual Filter label by default without changing the accessible name', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter()]}
        isMobile={() => false}
      />
    ));

    const select = screen.getByRole('combobox', { name: 'Filter' });
    const label = document.querySelector(`label[for="${select.id}"]`);
    expect(label).toHaveClass('sr-only');
    expect(select).toHaveClass('min-h-11');
    expect(select).toHaveClass('sm:h-7');
    expect(select).toHaveClass('sm:min-h-0');
    expect(select).toHaveClass('w-[7.5rem]');
    expect(select).not.toHaveClass('min-w-[7.5rem]');
    expect(select.parentElement).not.toHaveClass('p-0.5');
  });

  it('requires an explicit opt-in to show the visual Filter label', () => {
    render(() => (
      <FilterBar
        search={search}
        filters={[inlineTypeFilter(), menuNodeFilter()]}
        isMobile={() => false}
        showAddFilterLabel
      />
    ));

    const select = screen.getByRole('combobox', { name: 'Filter' });
    const label = document.querySelector(`label[for="${select.id}"]`);
    expect(label).not.toHaveClass('sr-only');
    expect(label).toHaveTextContent('Filter');
    expect(select.parentElement).toHaveClass('p-0.5');
  });

  it('completes a matching filter inline before pinning it as a chip on Enter', () => {
    const Harness = () => {
      const [query, setQuery] = createSignal('');
      const [node, setNode] = createSignal('');
      const nodeFilter: FilterDef = {
        id: 'node',
        label: 'Node',
        group: 'scope',
        value: node,
        setValue: setNode,
        defaultValue: '',
        options: () => [
          { value: '', label: 'All nodes' },
          { value: 'pve1', label: 'pve1' },
        ],
      };
      return (
        <FilterBar
          search={{ value: query, setValue: setQuery, placeholder: 'Search infrastructure' }}
          filters={[nodeFilter]}
          isMobile={() => false}
        />
      );
    };

    const { container } = render(() => <Harness />);
    const input = screen.getByPlaceholderText('Search infrastructure');
    input.focus();
    fireEvent.input(input, { target: { value: 'pv' } });
    expect(container.querySelector('[data-search-completion-suffix]')).toHaveTextContent('e1');

    fireEvent.keyDown(input, { key: 'Tab' });
    expect(input).toHaveValue('pve1');
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(screen.getByRole('button', { name: 'Remove Node filter' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Node: pve1' })).toBeInTheDocument();
    expect(input).toHaveValue('');
  });

  it('commits multiple infrastructure completions as removable inclusive search pills', () => {
    const Harness = () => {
      const [query, setQuery] = createSignal('');
      return (
        <>
          <FilterBar
            search={{
              value: query,
              setValue: setQuery,
              placeholder: 'Search objects',
              suggestions: () => [
                { id: 'node:pve1', label: 'pve1', value: 'pve1' },
                { id: 'host:docker-a', label: 'docker-a', value: 'docker-a' },
              ],
            }}
            filters={[]}
            isMobile={() => false}
          />
          <output data-testid="effective-search">{query()}</output>
        </>
      );
    };

    render(() => <Harness />);
    const input = screen.getByPlaceholderText('Search objects');
    input.focus();

    fireEvent.input(input, { target: { value: 'pv' } });
    fireEvent.keyDown(input, { key: 'Tab' });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByRole('button', { name: 'Remove search term pve1' })).toHaveClass(
      'min-h-6',
      'min-w-6',
    );

    fireEvent.input(input, { target: { value: 'doc' } });
    fireEvent.keyDown(input, { key: 'Tab' });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByRole('button', { name: 'Remove search term docker-a' })).toBeInTheDocument();
    expect(screen.getByTestId('effective-search')).toHaveTextContent('pve1, docker-a');

    fireEvent.click(screen.getByRole('button', { name: 'Remove search term pve1' }));
    expect(
      screen.queryByRole('button', { name: 'Remove search term pve1' }),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId('effective-search')).toHaveTextContent('docker-a');
  });

  it('commits a recognized abbreviated query without choosing one ambiguous object', () => {
    const Harness = () => {
      const [query, setQuery] = createSignal('');
      return (
        <>
          <FilterBar
            search={{
              value: query,
              setValue: setQuery,
              placeholder: 'Search Kubernetes objects',
              suggestions: () => [
                { id: 'deployment:checkout-api', label: 'checkout-api' },
                { id: 'deployment:checkout-web', label: 'checkout-web' },
                { id: 'pod:checkout-api-a', label: 'checkout-api-6c746d5bcf-c7z2p' },
              ],
            }}
            filters={[]}
            isMobile={() => false}
          />
          <output data-testid="effective-search">{query()}</output>
        </>
      );
    };

    const { container } = render(() => <Harness />);
    const input = screen.getByPlaceholderText('Search Kubernetes objects');
    input.focus();
    fireEvent.input(input, { target: { value: 'chec' } });

    expect(container.querySelector('[data-search-completion-suffix]')).toHaveTextContent('kout-');
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(input).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Remove search term chec' })).toBeInTheDocument();
    expect(screen.getByTestId('effective-search')).toHaveTextContent('chec');

    fireEvent.input(input, { target: { value: 'unmatched prose' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(input).toHaveValue('unmatched prose');
    expect(
      screen.queryByRole('button', { name: 'Remove search term unmatched prose' }),
    ).not.toBeInTheDocument();
  });
});
