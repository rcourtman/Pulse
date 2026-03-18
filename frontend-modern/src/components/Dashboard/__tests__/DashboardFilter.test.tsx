import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@solidjs/testing-library';
import { DashboardFilter } from '../DashboardFilter';

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
function makeProps(overrides: Partial<Parameters<typeof DashboardFilter>[0]> = {}) {
  return {
    search: vi.fn(() => ''),
    setSearch: vi.fn(),
    isSearchLocked: vi.fn(() => false),
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

describe('DashboardFilter', () => {
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
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByTestId('search-input')).toBeInTheDocument();
    });

    it('renders the Type filter select with all options', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      const typeSelect = screen.getByLabelText('Type');
      expect(typeSelect).toBeInTheDocument();
      expect(typeSelect).toHaveValue('all');

      const options = typeSelect.querySelectorAll('option');
      const values = Array.from(options).map((o) => o.value);
      expect(values).toEqual(['all', 'vm', 'system-container', 'app-container', 'pod']);
    });

    it('renders the Status filter select with all options', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      const statusSelect = screen.getByLabelText('Status');
      expect(statusSelect).toBeInTheDocument();
      expect(statusSelect).toHaveValue('all');

      const options = statusSelect.querySelectorAll('option');
      const values = Array.from(options).map((o) => o.value);
      expect(values).toEqual(['all', 'running', 'degraded', 'stopped']);
    });

    it('renders Grouped and List buttons', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Grouped')).toBeInTheDocument();
      expect(screen.getByText('List')).toBeInTheDocument();
    });

    it('does not show the Reset button when all filters are at defaults', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      expect(screen.queryByText('Reset')).not.toBeInTheDocument();
    });
  });

  // --- Type filter interactions ---

  describe('type filter', () => {
    it('calls setViewMode when a different type is selected', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      const typeSelect = screen.getByLabelText('Type');
      fireEvent.change(typeSelect, { target: { value: 'vm' } });
      expect(props.setViewMode).toHaveBeenCalledWith('vm');
    });
  });

  // --- Status filter interactions ---

  describe('status filter', () => {
    it('calls setStatusMode when a different status is selected', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      const statusSelect = screen.getByLabelText('Status');
      fireEvent.change(statusSelect, { target: { value: 'running' } });
      expect(props.setStatusMode).toHaveBeenCalledWith('running');
    });
  });

  // --- Grouping mode ---

  describe('grouping mode', () => {
    it('calls setGroupingMode("flat") when List button is clicked', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      fireEvent.click(screen.getByText('List'));
      expect(props.setGroupingMode).toHaveBeenCalledWith('flat');
    });

    it('calls setGroupingMode("grouped") when Grouped button is clicked', () => {
      const props = makeProps({ groupingMode: vi.fn(() => 'flat' as const) });
      render(() => <DashboardFilter {...props} />);
      fireEvent.click(screen.getByText('Grouped'));
      expect(props.setGroupingMode).toHaveBeenCalledWith('grouped');
    });
  });

  // --- Reset button ---

  describe('reset button', () => {
    it('shows when search is non-empty', () => {
      const props = makeProps({ search: vi.fn(() => 'hello') });
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when viewMode is not "all"', () => {
      const props = makeProps({ viewMode: vi.fn(() => 'vm' as const) });
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when statusMode is not "all"', () => {
      const props = makeProps({ statusMode: vi.fn(() => 'running' as const) });
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('shows when groupingMode is "flat"', () => {
      const props = makeProps({ groupingMode: vi.fn(() => 'flat' as const) });
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    it('resets all filters when clicked', () => {
      const hostOnChange = vi.fn();
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
        namespaceFilter: {
          value: 'ns-1',
          options: [
            { value: '', label: 'All' },
            { value: 'ns-1', label: 'NS 1' },
          ],
          onChange: namespaceOnChange,
        },
      });
      render(() => <DashboardFilter {...props} />);

      fireEvent.click(screen.getByText('Reset'));

      expect(props.setSearch).toHaveBeenCalledWith('');
      expect(props.setSortKey).toHaveBeenCalledWith('name');
      expect(props.setSortDirection).toHaveBeenCalledWith('asc');
      expect(props.setViewMode).toHaveBeenCalledWith('all');
      expect(props.setStatusMode).toHaveBeenCalledWith('all');
      expect(props.setGroupingMode).toHaveBeenCalledWith('grouped');
      expect(hostOnChange).toHaveBeenCalledWith('');
      expect(namespaceOnChange).toHaveBeenCalledWith('');
    });

    it('does not reset containerRuntimeFilter on reset (current behavior)', () => {
      // NOTE: The reset handler does not clear containerRuntimeFilter.
      // This test documents the current behavior.
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
      render(() => <DashboardFilter {...props} />);

      fireEvent.click(screen.getByText('Reset'));

      // containerRuntimeFilter.onChange is NOT called by reset
      expect(runtimeOnChange).not.toHaveBeenCalled();
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
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByLabelText('Agent')).toBeInTheDocument();
    });

    it('does not render when hostFilter prop is absent', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Node')).toBeInTheDocument();
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
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Charts')).toBeInTheDocument();
    });

    it('does not render Charts button when onChartsToggle is not provided', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      expect(screen.queryByText('Charts')).not.toBeInTheDocument();
    });

    it('calls onChartsToggle when Charts button is clicked', () => {
      const onChartsToggle = vi.fn();
      const props = makeProps({
        chartsCollapsed: vi.fn(() => false),
        onChartsToggle,
      });
      render(() => <DashboardFilter {...props} />);
      fireEvent.click(screen.getByText('Charts'));
      expect(onChartsToggle).toHaveBeenCalled();
    });
  });

  // --- Column picker ---

  describe('column picker', () => {
    it('renders ColumnPicker when all column props are provided', () => {
      const props = makeProps({
        availableColumns: [{ id: 'cpu', label: 'CPU' }],
        isColumnHidden: vi.fn(() => false),
        onColumnToggle: vi.fn(),
      });
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByTestId('column-picker')).toBeInTheDocument();
    });

    it('does not render ColumnPicker when column props are missing', () => {
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      expect(screen.queryByTestId('column-picker')).not.toBeInTheDocument();
    });
  });

  // --- Mobile behavior ---

  describe('mobile behavior', () => {
    it('shows the Filters toggle button on mobile', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      expect(screen.getByText('Filters')).toBeInTheDocument();
    });

    it('hides filter controls on mobile by default', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      // The Type filter should not be visible until Filters is toggled
      expect(screen.queryByLabelText('Type')).not.toBeInTheDocument();
    });

    it('shows filter controls when Filters toggle is clicked on mobile', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      fireEvent.click(screen.getByText('Filters'));
      expect(screen.getByLabelText('Type')).toBeInTheDocument();
      expect(screen.getByLabelText('Status')).toBeInTheDocument();
    });

    it('hides filter controls when Filters toggle is clicked again', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
      const toggle = screen.getByText('Filters');
      fireEvent.click(toggle);
      expect(screen.getByLabelText('Type')).toBeInTheDocument();
      fireEvent.click(toggle);
      expect(screen.queryByLabelText('Type')).not.toBeInTheDocument();
    });

    it('shows active filter count badge on mobile when filters are active', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps({
        search: vi.fn(() => 'test'),
        viewMode: vi.fn(() => 'vm' as const),
      });
      render(() => <DashboardFilter {...props} />);
      // The badge should show "2" (search + viewMode)
      expect(screen.getByText('2')).toBeInTheDocument();
    });

    it('does not show filter count badge when no filters are active', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps();
      render(() => <DashboardFilter {...props} />);
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
      render(() => <DashboardFilter {...props} />);
      // 5 active: search, viewMode, statusMode, host, namespace
      expect(screen.getByText('5')).toBeInTheDocument();
    });

    it('does not count whitespace-only search as active', () => {
      isMobileMock.mockReturnValue(true);
      const props = makeProps({ search: vi.fn(() => '   ') });
      render(() => <DashboardFilter {...props} />);
      expect(screen.queryByText('1')).not.toBeInTheDocument();
    });
  });
});
