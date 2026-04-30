import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { WorkloadsFilter } from '../WorkloadsFilter';
import { DEFAULT_WORKLOADS_SORT_KEY, type WorkloadsFilterProps } from '../workloadsFilterModel';

// Mock useBreakpoint — hoist so tests can control isMobile
const { isMobileMock } = vi.hoisted(() => {
  const isMobileMock = vi.fn(() => false);
  return { isMobileMock };
});

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: isMobileMock,
  }),
}));

// Mock SearchInput to avoid its side-effects (global keydown listener, history, etc.)
vi.mock('@/components/shared/SearchInput', () => ({
  SearchInput: (props: {
    value: () => string;
    onChange: (v: string) => void;
    placeholder?: string;
  }) => (
    <input
      data-testid="search-input"
      type="text"
      value={props.value()}
      onInput={(e) => props.onChange(e.currentTarget.value)}
      placeholder={props.placeholder}
    />
  ),
}));

// Mock ColumnPicker
vi.mock('@/components/shared/ColumnPicker', () => ({
  ColumnPicker: () => <div data-testid="column-picker">ColumnPicker</div>,
}));

/** Helper to build default required props, with optional overrides. */
function makeProps(overrides: Partial<WorkloadsFilterProps> = {}): WorkloadsFilterProps {
  return {
    search: vi.fn(() => ''),
    setSearch: vi.fn(),
    viewMode: vi.fn(() => 'all' as const),
    setViewMode: vi.fn(),
    statusMode: vi.fn(() => 'all' as const),
    setStatusMode: vi.fn(),
    groupingMode: vi.fn(() => 'grouped' as const),
    setGroupingMode: vi.fn(),
    setSortKey: vi.fn(),
    setSortDirection: vi.fn(),
    ...overrides,
  };
}

describe('WorkloadsFilter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isMobileMock.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
  });

  // --- Rendering ---

  describe('rendering', () => {
    it('renders the search input', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByTestId('search-input')).toBeInTheDocument();
    });

    it('renders the Type filter as a responsive compact toggle group with select fallback', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      const typeGroup = screen.getByRole('group', { name: 'Type' });
      expect(typeGroup).toBeInTheDocument();
      expect(typeGroup).toHaveClass('hidden');
      expect(typeGroup).toHaveClass('xl:inline-flex');
      expect(within(typeGroup).getByRole('button', { name: 'All' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      expect(within(typeGroup).getByRole('button', { name: 'VMs' })).toBeInTheDocument();
      expect(within(typeGroup).getByRole('button', { name: 'Containers' })).toBeInTheDocument();
      expect(within(typeGroup).getByRole('button', { name: 'Pods' })).toBeInTheDocument();

      const typeSelect = screen.getByLabelText('Type', { selector: 'select' });
      expect(typeSelect).toBeInTheDocument();
      expect(typeSelect).toHaveValue('all');
      expect(typeSelect.parentElement).toHaveClass('xl:hidden');
      const primaryControls = typeGroup.closest('.workloads-filter-primary-controls');
      expect(primaryControls).not.toBeNull();
      expect(primaryControls).toHaveClass('xl:flex-col');
      expect(primaryControls).toHaveClass('xl:items-start');
      expect(primaryControls).toContainElement(typeSelect.parentElement!);

      const options = typeSelect.querySelectorAll('option');
      const values = Array.from(options).map((o) => o.value);
      const labels = Array.from(options).map((o) => o.textContent);
      expect(values).toEqual(['all', 'vm', 'container', 'pod']);
      expect(labels).toEqual(['All', 'VMs', 'Containers', 'Pods']);
    });

    it('renders the Status filter as a compact toggle group', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      const statusGroup = screen.getByRole('group', { name: 'Status' });
      expect(statusGroup).toBeInTheDocument();
      expect(statusGroup).toHaveClass('hidden');
      expect(statusGroup).toHaveClass('xl:inline-flex');
      const statusSelect = screen.getByLabelText('Status', { selector: 'select' });
      expect(statusSelect).toBeInTheDocument();
      expect(statusSelect.parentElement).toHaveClass('xl:hidden');
      const primaryControls = statusGroup.closest('.workloads-filter-primary-controls');
      expect(primaryControls).not.toBeNull();
      expect(primaryControls).toContainElement(statusSelect.parentElement!);

      expect(within(statusGroup).getByRole('button', { name: 'All' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      expect(within(statusGroup).getByRole('button', { name: 'Running' })).toBeInTheDocument();
      expect(within(statusGroup).getByRole('button', { name: 'Degraded' })).toBeInTheDocument();
      expect(within(statusGroup).getByRole('button', { name: 'Stopped' })).toBeInTheDocument();
    });

    it('renders Grouped and List buttons', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByLabelText('Group by')).toBeInTheDocument();
      expect(screen.getByText('Grouped')).toBeInTheDocument();
      expect(screen.getByText('List')).toBeInTheDocument();
    });

    it('allows dense workload toolbar controls to wrap instead of clipping trailing actions', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'app-container' as const),
        hostFilter: {
          label: 'Node',
          value: '',
          options: [{ value: '', label: 'All nodes' }],
          onChange: vi.fn(),
        },
        platformFilter: {
          label: 'Platform',
          value: '',
          options: [{ value: '', label: 'All platforms' }],
          onChange: vi.fn(),
        },
        containerRuntimeFilter: {
          label: 'Runtime',
          value: '',
          options: [{ value: '', label: 'All runtimes' }],
          onChange: vi.fn(),
        },
        columnVisibility: {
          availableColumns: [{ id: 'image', label: 'Image' }],
          isColumnHidden: vi.fn(() => false),
          onColumnToggle: vi.fn(),
        },
        chartsCollapsed: vi.fn(() => false),
        onChartsToggle: vi.fn(),
      });
      const { container } = render(() => <WorkloadsFilter {...props} />);

      expect(screen.getByTestId('column-picker')).toBeInTheDocument();
      const primaryControls = container.querySelector<HTMLElement>(
        '.workloads-filter-primary-controls',
      );
      const secondaryControls = container.querySelector<HTMLElement>(
        '.workloads-filter-secondary-controls',
      );
      const controlDeck = container.querySelector<HTMLElement>('.page-controls-control-deck');
      expect(primaryControls).not.toBeNull();
      expect(secondaryControls).not.toBeNull();
      expect(controlDeck).not.toBeNull();
      expect(primaryControls!).toContainElement(screen.getByRole('group', { name: 'Type' }));
      expect(primaryControls!).toContainElement(screen.getByRole('group', { name: 'Status' }));
      expect(secondaryControls!).toContainElement(
        screen.getByLabelText('Node', { selector: 'select' }),
      );
      expect(secondaryControls!).toContainElement(
        screen.getByLabelText('Platform', { selector: 'select' }),
      );
      expect(secondaryControls!).toContainElement(
        screen.getByLabelText('Runtime', { selector: 'select' }),
      );
      expect(secondaryControls!.compareDocumentPosition(primaryControls!)).toBe(
        Node.DOCUMENT_POSITION_PRECEDING,
      );
      const toolbarActions = container.querySelector<HTMLElement>('.page-controls-toolbar-actions');
      expect(toolbarActions).not.toBeNull();
      expect(controlDeck!).toHaveClass('grid');
      expect(controlDeck!).toHaveClass('w-full');
      expect(controlDeck!).toHaveClass('bg-surface-alt');
      expect(controlDeck!).toContainElement(toolbarActions!);
      expect(controlDeck!).toContainElement(primaryControls!);
      expect(primaryControls!).toHaveClass('border');
      expect(primaryControls!).toHaveClass('page-controls-filter-section');
      expect(primaryControls!).toHaveClass('bg-surface');
      expect(secondaryControls!).toHaveClass('border');
      expect(secondaryControls!).toHaveClass('page-controls-filter-section');
      expect(secondaryControls!).toHaveClass('bg-surface');
      expect(toolbarActions!).toHaveClass('border');
      expect(toolbarActions!).toHaveClass('page-controls-filter-section');
      expect(toolbarActions!).toHaveClass('bg-surface');
      expect(toolbarActions!).toHaveClass('xl:justify-self-end');
      expect(toolbarActions!).not.toHaveClass('ml-auto');
      expect(toolbarActions!).not.toHaveClass('border-t');
      expect(toolbarActions!).toContainElement(screen.getByText('Grouped'));
      expect(toolbarActions!).toContainElement(screen.getByText('List'));
      expect(toolbarActions!).toContainElement(screen.getByTestId('column-picker'));
      expect(
        Array.from(container.querySelectorAll('.workloads-filter *')).some((element) =>
          (element.getAttribute('class') ?? '').includes('lg:flex-nowrap'),
        ),
      ).toBe(false);
    });

    it('maps legacy exact container modes to the user-facing Containers type control', () => {
      const props = makeProps({ viewMode: vi.fn(() => 'app-container' as const) });
      render(() => <WorkloadsFilter {...props} />);
      const typeGroup = screen.getByRole('group', { name: 'Type' });
      expect(within(typeGroup).getByRole('button', { name: 'Containers' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      expect(screen.getByLabelText('Type', { selector: 'select' })).toHaveValue('container');
    });

    it('does not show the Reset button when all filters are at defaults', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByText('Reset')).not.toBeInTheDocument();
    });
  });

  // --- Type filter interactions ---

  describe('type filter', () => {
    it('calls setViewMode when a different type is selected', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      const typeGroup = screen.getByRole('group', { name: 'Type' });
      fireEvent.click(within(typeGroup).getByRole('button', { name: 'Containers' }));
      expect(props.setViewMode).toHaveBeenCalledWith('container');
    });
  });

  // --- Status filter interactions ---

  describe('status filter', () => {
    it('calls setStatusMode when a different status is selected', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      const statusGroup = screen.getByRole('group', { name: 'Status' });
      fireEvent.click(within(statusGroup).getByRole('button', { name: 'Running' }));
      expect(props.setStatusMode).toHaveBeenCalledWith('running');
    });
  });

  // --- Grouping mode ---

  describe('grouping mode', () => {
    it('uses the canonical grouped/list table-mode titles', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByRole('button', { name: 'Grouped' })).toHaveAttribute(
        'title',
        'Grouped table view',
      );
      expect(screen.getByRole('button', { name: 'List' })).toHaveAttribute(
        'title',
        'Flat list view',
      );
    });

    it('calls setGroupingMode("flat") when List button is clicked', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.click(screen.getByText('List'));
      expect(props.setGroupingMode).toHaveBeenCalledWith('flat');
    });

    it('calls setGroupingMode("grouped") when Grouped button is clicked', () => {
      const props = makeProps({ groupingMode: vi.fn(() => 'flat' as const) });
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.click(screen.getByText('Grouped'));
      expect(props.setGroupingMode).toHaveBeenCalledWith('grouped');
    });
  });

  // --- Reset button ---

  describe('reset button', () => {
    it('shows when search is non-empty', () => {
      const props = makeProps({ search: vi.fn(() => 'hello') });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when viewMode is not "all"', () => {
      const props = makeProps({ viewMode: vi.fn(() => 'vm' as const) });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when statusMode is not "all"', () => {
      const props = makeProps({ statusMode: vi.fn(() => 'running' as const) });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when groupingMode is "flat"', () => {
      const props = makeProps({ groupingMode: vi.fn(() => 'flat' as const) });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when a host filter is active', () => {
      const props = makeProps({
        hostFilter: {
          value: 'host-1',
          options: [
            { value: '', label: 'All' },
            { value: 'host-1', label: 'Host 1' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('resets all filters when clicked', () => {
      const hostOnChange = vi.fn();
      const platformOnChange = vi.fn();
      const namespaceOnChange = vi.fn();
      const props = makeProps({
        search: vi.fn(() => 'test'),
        viewMode: vi.fn(() => 'vm' as const),
        statusMode: vi.fn(() => 'running' as const),
        groupingMode: vi.fn(() => 'flat' as const),
        hostFilter: {
          value: 'host-1',
          options: [
            { value: '', label: 'All' },
            { value: 'host-1', label: 'Host 1' },
          ],
          onChange: hostOnChange,
        },
        platformFilter: {
          value: 'truenas',
          options: [
            { value: '', label: 'All platforms' },
            { value: 'truenas', label: 'TrueNAS' },
          ],
          onChange: platformOnChange,
        },
        namespaceFilter: {
          value: 'ns-1',
          options: [
            { value: '', label: 'All' },
            { value: 'ns-1', label: 'NS 1' },
          ],
          onChange: namespaceOnChange,
        },
      });
      render(() => <WorkloadsFilter {...props} />);

      fireEvent.click(screen.getByText('Reset'));

      expect(props.setSearch).toHaveBeenCalledWith('');
      expect(props.setSortKey).toHaveBeenCalledWith(DEFAULT_WORKLOADS_SORT_KEY);
      expect(props.setSortDirection).toHaveBeenCalledWith('asc');
      expect(props.setViewMode).toHaveBeenCalledWith('all');
      expect(props.setStatusMode).toHaveBeenCalledWith('all');
      expect(props.setGroupingMode).toHaveBeenCalledWith('grouped');
      expect(hostOnChange).toHaveBeenCalledWith('');
      expect(platformOnChange).toHaveBeenCalledWith('');
      expect(namespaceOnChange).toHaveBeenCalledWith('');
    });

    it('does not reset containerRuntimeFilter when clicked', () => {
      const runtimeOnChange = vi.fn();
      const props = makeProps({
        search: vi.fn(() => 'test'),
        viewMode: vi.fn(() => 'app-container' as const),
        containerRuntimeFilter: {
          value: 'podman',
          options: [
            { value: '', label: 'All Runtimes' },
            { value: 'podman', label: 'Podman' },
          ],
          onChange: runtimeOnChange,
        },
      });
      render(() => <WorkloadsFilter {...props} />);

      fireEvent.click(screen.getByText('Reset'));

      expect(runtimeOnChange).not.toHaveBeenCalled();
    });

    it('shows when a container runtime filter is active', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'app-container' as const),
        containerRuntimeFilter: {
          value: 'podman',
          options: [
            { value: '', label: 'All Runtimes' },
            { value: 'podman', label: 'Podman' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });
  });

  // --- Host filter ---

  describe('host filter', () => {
    it('renders when hostFilter prop is provided', () => {
      const props = makeProps({
        hostFilter: {
          value: '',
          options: [
            { value: '', label: 'All Hosts' },
            { value: 'host-1', label: 'Host 1' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByLabelText('Agent')).toBeInTheDocument();
    });

    it('does not render when hostFilter prop is absent', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByLabelText('Agent')).not.toBeInTheDocument();
    });

    it('calls onChange when host selection changes', () => {
      const onChange = vi.fn();
      const props = makeProps({
        hostFilter: {
          value: '',
          options: [
            { value: '', label: 'All Hosts' },
            { value: 'host-1', label: 'Host 1' },
          ],
          onChange,
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.change(screen.getByLabelText('Agent'), { target: { value: 'host-1' } });
      expect(onChange).toHaveBeenCalledWith('host-1');
    });

    it('uses custom label when provided', () => {
      const props = makeProps({
        hostFilter: {
          label: 'Node',
          value: '',
          options: [{ value: '', label: 'All' }],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Node')).toBeInTheDocument();
    });
  });

  describe('platform filter', () => {
    it('renders when platformFilter prop is provided', () => {
      const props = makeProps({
        platformFilter: {
          value: '',
          options: [
            { value: '', label: 'All platforms' },
            { value: 'truenas', label: 'TrueNAS' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByLabelText('Platform')).toBeInTheDocument();
    });

    it('calls onChange when platform selection changes', () => {
      const onChange = vi.fn();
      const props = makeProps({
        platformFilter: {
          value: '',
          options: [
            { value: '', label: 'All platforms' },
            { value: 'truenas', label: 'TrueNAS' },
          ],
          onChange,
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.change(screen.getByLabelText('Platform'), { target: { value: 'truenas' } });
      expect(onChange).toHaveBeenCalledWith('truenas');
    });

    it('keeps the controlled platform value when async options arrive later', () => {
      const baseProps = makeProps();
      const [platformOptions, setPlatformOptions] = createSignal([
        { value: '', label: 'All platforms' },
      ]);

      render(() => (
        <WorkloadsFilter
          {...baseProps}
          platformFilter={{
            value: 'truenas',
            options: platformOptions(),
            onChange: vi.fn(),
          }}
        />
      ));

      setPlatformOptions([
        { value: '', label: 'All platforms' },
        { value: 'truenas', label: 'TrueNAS' },
      ]);

      expect(screen.getByLabelText('Platform')).toHaveValue('truenas');
      expect(screen.getByRole('option', { name: 'TrueNAS' })).toBeInTheDocument();
    });
  });

  // --- Namespace filter ---

  describe('namespace filter', () => {
    it('renders when namespaceFilter prop is provided', () => {
      const props = makeProps({
        namespaceFilter: {
          value: '',
          options: [
            { value: '', label: 'All Namespaces' },
            { value: 'default', label: 'default' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByLabelText('Namespace')).toBeInTheDocument();
    });

    it('calls onChange when namespace selection changes', () => {
      const onChange = vi.fn();
      const props = makeProps({
        namespaceFilter: {
          value: '',
          options: [
            { value: '', label: 'All Namespaces' },
            { value: 'kube-system', label: 'kube-system' },
          ],
          onChange,
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.change(screen.getByLabelText('Namespace'), { target: { value: 'kube-system' } });
      expect(onChange).toHaveBeenCalledWith('kube-system');
    });
  });

  // --- Container runtime filter ---

  describe('container runtime filter', () => {
    it('renders only when viewMode is "app-container" and containerRuntimeFilter is provided', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'app-container' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All Runtimes' },
            { value: 'docker', label: 'Docker' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByLabelText('Runtime')).toBeInTheDocument();
    });

    it('does not render when viewMode is not "app-container"', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'all' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All Runtimes' },
            { value: 'docker', label: 'Docker' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByLabelText('Runtime')).not.toBeInTheDocument();
    });

    it('calls onChange when runtime selection changes', () => {
      const onChange = vi.fn();
      const props = makeProps({
        viewMode: vi.fn(() => 'app-container' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All Runtimes' },
            { value: 'podman', label: 'Podman' },
          ],
          onChange,
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.change(screen.getByLabelText('Runtime'), { target: { value: 'podman' } });
      expect(onChange).toHaveBeenCalledWith('podman');
    });
  });

  // --- Charts toggle ---

  describe('charts toggle', () => {
    it('renders Charts button when onChartsToggle is provided', () => {
      const props = makeProps({
        chartsCollapsed: vi.fn(() => false),
        onChartsToggle: vi.fn(),
      });
      render(() => <WorkloadsFilter {...props} />);
      const chartsButton = screen.getByRole('button', { name: 'Hide charts' });
      expect(chartsButton).toBeInTheDocument();
      expect(chartsButton).toHaveTextContent('Charts');
      expect(chartsButton).toHaveAttribute('aria-pressed', 'true');
      expect(chartsButton).toHaveAttribute('title', 'Hide charts');
    });

    it('labels the Charts button as a show action when charts are collapsed', () => {
      const props = makeProps({
        chartsCollapsed: vi.fn(() => true),
        onChartsToggle: vi.fn(),
      });
      render(() => <WorkloadsFilter {...props} />);
      const chartsButton = screen.getByRole('button', { name: 'Show charts' });
      expect(chartsButton).toHaveTextContent('Charts');
      expect(chartsButton).toHaveAttribute('aria-pressed', 'false');
      expect(chartsButton).toHaveAttribute('title', 'Show charts');
    });

    it('does not render Charts button when onChartsToggle is not provided', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByText('Charts')).not.toBeInTheDocument();
    });

    it('calls onChartsToggle when Charts button is clicked', () => {
      const onChartsToggle = vi.fn();
      const props = makeProps({
        chartsCollapsed: vi.fn(() => false),
        onChartsToggle,
      });
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.click(screen.getByRole('button', { name: 'Hide charts' }));
      expect(onChartsToggle).toHaveBeenCalled();
    });
  });

  // --- Column picker ---

  describe('column picker', () => {
    it('renders ColumnPicker when all column props are provided', () => {
      const props = makeProps({
        columnVisibility: {
          availableColumns: [{ id: 'cpu', label: 'CPU' }],
          isColumnHidden: vi.fn(() => false),
          onColumnToggle: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByTestId('column-picker')).toBeInTheDocument();
    });

    it('does not render ColumnPicker when column props are missing', () => {
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByTestId('column-picker')).not.toBeInTheDocument();
    });
  });

  // --- Mobile behavior ---

  describe('mobile behavior', () => {
    it('shows the Filters toggle button on mobile', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('Filters')).toBeInTheDocument();
    });

    it('hides filter controls on mobile by default', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      // The Type filter should not be visible until Filters is toggled
      expect(screen.queryByLabelText('Type', { selector: 'select' })).not.toBeInTheDocument();
    });

    it('shows filter controls when Filters toggle is clicked on mobile', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      fireEvent.click(screen.getByText('Filters'));
      expect(screen.getByLabelText('Type', { selector: 'select' })).toBeInTheDocument();
      expect(screen.getByLabelText('Status', { selector: 'select' })).toBeInTheDocument();
    });

    it('hides filter controls when Filters toggle is clicked again', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      const toggle = screen.getByText('Filters');
      fireEvent.click(toggle);
      expect(screen.getByLabelText('Type', { selector: 'select' })).toBeInTheDocument();
      fireEvent.click(toggle);
      expect(screen.queryByLabelText('Type', { selector: 'select' })).not.toBeInTheDocument();
    });

    it('shows active filter count badge on mobile when filters are active', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps({
        search: vi.fn(() => 'test'),
        viewMode: vi.fn(() => 'vm' as const),
      });
      render(() => <WorkloadsFilter {...props} />);
      // The badge should show "2" (search + viewMode)
      expect(screen.getByText('2')).toBeInTheDocument();
    });

    it('does not show filter count badge when no filters are active', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <WorkloadsFilter {...props} />);
      // The badge is a span inside the Filters button — when count is 0, no badge renders.
      // Verify no numeric badge appears near the Filters button.
      const filtersButton = screen.getByText('Filters');
      const badge = filtersButton.parentElement?.querySelector('span.rounded-full');
      expect(badge).toBeNull();
    });
  });

  // --- Active filter count (desktop — badge not shown but logic still runs) ---

  describe('activeFilterCount memo', () => {
    it('counts search + viewMode + statusMode + hostFilter + namespaceFilter', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps({
        search: vi.fn(() => '  query  '),
        viewMode: vi.fn(() => 'pod' as const),
        statusMode: vi.fn(() => 'degraded' as const),
        hostFilter: {
          value: 'h1',
          options: [
            { value: '', label: 'All' },
            { value: 'h1', label: 'H1' },
          ],
          onChange: vi.fn(),
        },
        namespaceFilter: {
          value: 'ns',
          options: [
            { value: '', label: 'All' },
            { value: 'ns', label: 'NS' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      // 5 active: search, viewMode, statusMode, host, namespace
      expect(screen.getByText('5')).toBeInTheDocument();
    });

    it('does not count container runtime as an active filter', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps({
        viewMode: vi.fn(() => 'app-container' as const),
        containerRuntimeFilter: {
          value: 'docker',
          options: [
            { value: '', label: 'All runtimes' },
            { value: 'docker', label: 'Docker' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.getByText('1')).toBeInTheDocument();
    });

    it('does not count whitespace-only search as active', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps({ search: vi.fn(() => '   ') });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByText('1')).not.toBeInTheDocument();
    });
  });
});
