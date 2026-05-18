import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, within } from '@solidjs/testing-library';
import { WorkloadsFilter } from '../WorkloadsFilter';
import {
  DEFAULT_WORKLOADS_GROUPING_MODE,
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_STATUS_MODE,
  DEFAULT_WORKLOADS_VIEW_MODE,
  type WorkloadsFilterProps,
} from '../workloadsFilterModel';

const { isMobileMock } = vi.hoisted(() => {
  const isMobileMock = vi.fn(() => false);
  return { isMobileMock };
});

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: isMobileMock,
  }),
}));

// Mock @solidjs/router so the saved-views menu can call useNavigate /
// useLocation outside a Router context. The tests don't exercise navigation.
vi.mock('@solidjs/router', () => ({
  useNavigate: () => () => undefined,
  useLocation: () => ({ pathname: '/workloads', search: '', hash: '', state: null, query: {} }),
}));

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

vi.mock('@/components/shared/ColumnPicker', () => ({
  ColumnPicker: () => <div data-testid="column-picker">ColumnPicker</div>,
}));

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

const openAddFilterMenu = () => {
  fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
};

const inlineFilterGroup = (label: string) => screen.getByRole('group', { name: label });

const pickInlineFilter = (filterLabel: string, optionLabel: string) => {
  fireEvent.click(
    within(inlineFilterGroup(filterLabel)).getByRole('button', { name: optionLabel }),
  );
};

const pickFromMenu = (menuItem: string, optionLabel: string) => {
  openAddFilterMenu();
  fireEvent.click(screen.getByRole('menuitem', { name: menuItem }));
  const menu = screen.getByRole('menu');
  fireEvent.click(within(menu).getByRole('button', { name: optionLabel }));
};

describe('WorkloadsFilter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isMobileMock.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
  });

  describe('rendering', () => {
    it('renders the search input', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.getByTestId('search-input')).toBeInTheDocument();
    });

    it('exposes Type and Status as one-click inline controls', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(inlineFilterGroup('Type')).toBeInTheDocument();
      expect(inlineFilterGroup('Status')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Filter' })).not.toBeInTheDocument();
    });

    it('renders Grouped and List view-option buttons', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.getByRole('button', { name: 'Grouped' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'List' })).toBeInTheDocument();
    });

    it('maps legacy container view modes onto the canonical "Containers" type chip', () => {
      render(() => (
        <WorkloadsFilter {...makeProps({ viewMode: vi.fn(() => 'app-container' as const) })} />
      ));
      expect(
        within(inlineFilterGroup('Type')).getByRole('button', { name: 'Containers' }),
      ).toHaveAttribute('aria-pressed', 'true');
    });

    it('can suppress the Type filter for platform-owned workload type scopes', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            viewMode: vi.fn(() => 'app-container' as const),
            suppressTypeFilter: true,
          })}
        />
      ));
      expect(screen.queryByRole('group', { name: 'Type' })).not.toBeInTheDocument();
      expect(inlineFilterGroup('Status')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Clear all' })).not.toBeInTheDocument();
    });
  });

  describe('type filter', () => {
    it('calls setViewMode when a different inline type is selected', () => {
      const setViewMode = vi.fn();
      render(() => <WorkloadsFilter {...makeProps({ setViewMode })} />);
      pickInlineFilter('Type', 'VMs');
      expect(setViewMode).toHaveBeenCalledWith('vm');
    });
  });

  describe('status filter', () => {
    it('calls setStatusMode when a different inline status is selected', () => {
      const setStatusMode = vi.fn();
      render(() => <WorkloadsFilter {...makeProps({ setStatusMode })} />);
      pickInlineFilter('Status', 'Running');
      expect(setStatusMode).toHaveBeenCalledWith('running');
    });
  });

  describe('grouping mode', () => {
    it('calls setGroupingMode("flat") when the List view-option is clicked', () => {
      const setGroupingMode = vi.fn();
      render(() => <WorkloadsFilter {...makeProps({ setGroupingMode })} />);
      fireEvent.click(screen.getByRole('button', { name: 'List' }));
      expect(setGroupingMode).toHaveBeenCalledWith('flat');
    });

    it('calls setGroupingMode("grouped") when the Grouped view-option is clicked', () => {
      const setGroupingMode = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({ groupingMode: vi.fn(() => 'flat' as const), setGroupingMode })}
        />
      ));
      fireEvent.click(screen.getByRole('button', { name: 'Grouped' }));
      expect(setGroupingMode).toHaveBeenCalledWith('grouped');
    });
  });

  describe('clear all', () => {
    it('does not render when all filters are at their defaults', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByRole('button', { name: 'Clear all' })).not.toBeInTheDocument();
    });

    it('renders when search is non-empty', () => {
      render(() => <WorkloadsFilter {...makeProps({ search: vi.fn(() => 'foo') })} />);
      expect(screen.getByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    });

    it('renders when viewMode is not the default', () => {
      render(() => <WorkloadsFilter {...makeProps({ viewMode: vi.fn(() => 'vm' as const) })} />);
      expect(screen.getByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    });

    it('does not count a suppressed platform-owned viewMode as an active filter', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            viewMode: vi.fn(() => 'app-container' as const),
            suppressTypeFilter: true,
          })}
        />
      ));
      expect(screen.queryByRole('button', { name: 'Clear all' })).not.toBeInTheDocument();
    });

    it('renders when statusMode is not the default', () => {
      render(() => (
        <WorkloadsFilter {...makeProps({ statusMode: vi.fn(() => 'running' as const) })} />
      ));
      expect(screen.getByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    });

    it('renders when groupingMode is "flat"', () => {
      render(() => (
        <WorkloadsFilter {...makeProps({ groupingMode: vi.fn(() => 'flat' as const) })} />
      ));
      expect(screen.getByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    });

    it('renders when a host filter is active', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            hostFilter: {
              value: 'pve1',
              options: [
                { value: '', label: 'All nodes' },
                { value: 'pve1', label: 'pve1' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      expect(screen.getByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    });

    it('renders when a container runtime filter is active', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            viewMode: vi.fn(() => 'container' as const),
            containerRuntimeFilter: {
              value: 'docker',
              options: [
                { value: '', label: 'All runtimes' },
                { value: 'docker', label: 'docker' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      expect(screen.getByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    });

    it('resets all canonical filter state when clicked', () => {
      const setSearch = vi.fn();
      const setViewMode = vi.fn();
      const setStatusMode = vi.fn();
      const setGroupingMode = vi.fn();
      const setSortKey = vi.fn();
      const setSortDirection = vi.fn();
      const hostOnChange = vi.fn();
      const platformOnChange = vi.fn();
      const namespaceOnChange = vi.fn();
      const runtimeOnChange = vi.fn();

      render(() => (
        <WorkloadsFilter
          {...makeProps({
            search: vi.fn(() => 'foo'),
            setSearch,
            setViewMode,
            setStatusMode,
            setGroupingMode,
            setSortKey,
            setSortDirection,
            hostFilter: {
              value: 'pve1',
              options: [
                { value: '', label: 'All nodes' },
                { value: 'pve1', label: 'pve1' },
              ],
              onChange: hostOnChange,
            },
            platformFilter: {
              value: 'proxmox',
              options: [
                { value: '', label: 'All platforms' },
                { value: 'proxmox', label: 'Proxmox' },
              ],
              onChange: platformOnChange,
            },
            namespaceFilter: {
              value: 'default',
              options: [
                { value: '', label: 'All namespaces' },
                { value: 'default', label: 'default' },
              ],
              onChange: namespaceOnChange,
            },
            containerRuntimeFilter: {
              value: 'docker',
              options: [
                { value: '', label: 'All runtimes' },
                { value: 'docker', label: 'docker' },
              ],
              onChange: runtimeOnChange,
            },
          })}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));

      expect(setSearch).toHaveBeenCalledWith('');
      expect(setSortKey).toHaveBeenCalledWith(DEFAULT_WORKLOADS_SORT_KEY);
      expect(setSortDirection).toHaveBeenCalledWith(DEFAULT_WORKLOADS_SORT_DIRECTION);
      expect(setViewMode).toHaveBeenCalledWith(DEFAULT_WORKLOADS_VIEW_MODE);
      expect(setStatusMode).toHaveBeenCalledWith(DEFAULT_WORKLOADS_STATUS_MODE);
      expect(setGroupingMode).toHaveBeenCalledWith(DEFAULT_WORKLOADS_GROUPING_MODE);
      expect(hostOnChange).toHaveBeenCalledWith('');
      expect(platformOnChange).toHaveBeenCalledWith('');
      expect(namespaceOnChange).toHaveBeenCalledWith('');
      expect(runtimeOnChange).toHaveBeenCalledWith('');
    });

    it('does not reset a suppressed platform-owned viewMode when clearing other filters', () => {
      const setViewMode = vi.fn();
      const setSearch = vi.fn();
      const setSortKey = vi.fn();

      render(() => (
        <WorkloadsFilter
          {...makeProps({
            defaultSortKey: 'name',
            search: vi.fn(() => 'nginx'),
            setSearch,
            setSortKey,
            setViewMode,
            viewMode: vi.fn(() => 'app-container' as const),
            suppressTypeFilter: true,
          })}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));

      expect(setSearch).toHaveBeenCalledWith('');
      expect(setSortKey).toHaveBeenCalledWith('name');
      expect(setViewMode).not.toHaveBeenCalled();
    });
  });

  describe('host filter', () => {
    it('appears in the "+ Filter" menu when hostFilter prop is provided', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            hostFilter: {
              value: '',
              options: [
                { value: '', label: 'All nodes' },
                { value: 'pve1', label: 'pve1' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      openAddFilterMenu();
      expect(screen.getByRole('menuitem', { name: 'Agent' })).toBeInTheDocument();
    });

    it('does not appear when hostFilter prop is absent', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByRole('button', { name: 'Filter' })).not.toBeInTheDocument();
    });

    it('calls onChange when host selection changes via chip popover', () => {
      const onChange = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            hostFilter: {
              value: '',
              options: [
                { value: '', label: 'All nodes' },
                { value: 'pve1', label: 'pve1' },
              ],
              onChange,
            },
          })}
        />
      ));
      pickFromMenu('Agent', 'pve1');
      expect(onChange).toHaveBeenCalledWith('pve1');
    });

    it('uses a custom label when hostFilter.label is provided', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            hostFilter: {
              label: 'K8s cluster',
              value: '',
              options: [{ value: '', label: 'All clusters' }],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      openAddFilterMenu();
      expect(screen.getByRole('menuitem', { name: 'K8s cluster' })).toBeInTheDocument();
    });
  });

  describe('platform filter', () => {
    it('appears in the menu when platformFilter prop is provided', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            platformFilter: {
              value: '',
              options: [
                { value: '', label: 'All platforms' },
                { value: 'proxmox', label: 'Proxmox' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      openAddFilterMenu();
      expect(screen.getByRole('menuitem', { name: 'Platform' })).toBeInTheDocument();
    });

    it('calls onChange when platform selection changes via chip popover', () => {
      const onChange = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            platformFilter: {
              value: '',
              options: [
                { value: '', label: 'All platforms' },
                { value: 'proxmox', label: 'Proxmox' },
              ],
              onChange,
            },
          })}
        />
      ));
      pickFromMenu('Platform', 'Proxmox');
      expect(onChange).toHaveBeenCalledWith('proxmox');
    });
  });

  describe('namespace filter', () => {
    it('appears in the menu when namespaceFilter prop is provided', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            namespaceFilter: {
              value: '',
              options: [
                { value: '', label: 'All namespaces' },
                { value: 'default', label: 'default' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      openAddFilterMenu();
      expect(screen.getByRole('menuitem', { name: 'Namespace' })).toBeInTheDocument();
    });

    it('calls onChange when namespace selection changes via chip popover', () => {
      const onChange = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            namespaceFilter: {
              value: '',
              options: [
                { value: '', label: 'All namespaces' },
                { value: 'default', label: 'default' },
              ],
              onChange,
            },
          })}
        />
      ));
      pickFromMenu('Namespace', 'default');
      expect(onChange).toHaveBeenCalledWith('default');
    });
  });

  describe('container runtime filter', () => {
    it('appears in the menu only when viewMode is a container variant and containerRuntimeFilter is provided', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'container' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All runtimes' },
            { value: 'docker', label: 'docker' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      openAddFilterMenu();
      expect(screen.getByRole('menuitem', { name: 'Runtime' })).toBeInTheDocument();
    });

    it('does not appear when viewMode is not a container variant', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'vm' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All runtimes' },
            { value: 'docker', label: 'docker' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByRole('button', { name: 'Filter' })).not.toBeInTheDocument();
    });

    it('calls onChange when runtime selection changes via chip popover', () => {
      const onChange = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            viewMode: vi.fn(() => 'container' as const),
            containerRuntimeFilter: {
              value: '',
              options: [
                { value: '', label: 'All runtimes' },
                { value: 'docker', label: 'docker' },
              ],
              onChange,
            },
          })}
        />
      ));
      pickFromMenu('Runtime', 'docker');
      expect(onChange).toHaveBeenCalledWith('docker');
    });
  });

  describe('charts toggle', () => {
    it('renders the Charts button when onChartsToggle is provided', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            chartsCollapsed: vi.fn(() => false),
            onChartsToggle: vi.fn(),
          })}
        />
      ));
      expect(screen.getByRole('button', { name: 'Hide charts' })).toBeInTheDocument();
    });

    it('labels the Charts button as a show action when charts are collapsed', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            chartsCollapsed: vi.fn(() => true),
            onChartsToggle: vi.fn(),
          })}
        />
      ));
      expect(screen.getByRole('button', { name: 'Show charts' })).toBeInTheDocument();
    });
  });

  describe('column picker', () => {
    it('renders ColumnPicker when columnVisibility is provided', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            columnVisibility: {
              availableColumns: [{ id: 'cpu', label: 'CPU', toggleable: true }],
              isColumnHidden: () => false,
              onColumnToggle: vi.fn(),
              onColumnReset: vi.fn(),
            },
          })}
        />
      ));
      expect(screen.getByTestId('column-picker')).toBeInTheDocument();
    });

    it('does not render ColumnPicker when columnVisibility is absent', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByTestId('column-picker')).not.toBeInTheDocument();
    });
  });
});
