import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, within } from '@solidjs/testing-library';
import { WorkloadsFilter } from '../WorkloadsFilter';
import workloadsFilterSource from '../WorkloadsFilter.tsx?raw';
import {
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
  ColumnPicker: (props: { inline?: boolean }) => (
    <div data-testid="column-picker" data-inline={props.inline ? 'true' : 'false'}>
      ColumnPicker
    </div>
  ),
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

const inlineFilterGroup = (label: string) => screen.getByRole('group', { name: label });

const addFilterSelect = () => screen.getByRole('combobox', { name: 'Filter' });

const addFilterOption = (filterLabel: string, optionLabel: string) =>
  within(addFilterSelect()).getByRole('option', {
    name: `${filterLabel}: ${optionLabel}`,
  }) as HTMLOptionElement;

const pickInlineFilter = (filterLabel: string, optionLabel: string) => {
  fireEvent.click(
    within(inlineFilterGroup(filterLabel)).getByRole('button', { name: optionLabel }),
  );
};

const pickFromMenu = (menuItem: string, optionLabel: string) => {
  fireEvent.change(addFilterSelect(), {
    target: { value: addFilterOption(menuItem, optionLabel).value },
  });
};

const openViewPreferences = () => {
  fireEvent.click(screen.getByRole('button', { name: 'View' }));
  return screen.getByRole('region', { name: 'View preferences' });
};

describe('WorkloadsFilter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    isMobileMock.mockReturnValue(false);
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  describe('rendering', () => {
    it('uses the shared FilterBar status-dot presentation helper', () => {
      expect(workloadsFilterSource).toContain('filterChipStatusDot');
      expect(workloadsFilterSource).not.toContain('const statusDot = (className: string)');
      expect(workloadsFilterSource).not.toContain('h-2 w-2 rounded-full ' + '${className}');
    });

    it('renders the search input', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.getByTestId('search-input')).toBeInTheDocument();
    });

    it('accepts page-owned search copy and status labels', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            searchPlaceholder: 'Search pods by namespace or image',
            statusOptions: [
              { value: 'all', label: 'All' },
              { value: 'running', label: 'Running' },
              { value: 'degraded', label: 'Needs attention' },
              { value: 'stopped', label: 'Not running' },
            ],
          })}
        />
      ));

      expect(screen.getByPlaceholderText('Search pods by namespace or image')).toBeInTheDocument();
      expect(
        within(inlineFilterGroup('Status')).getByRole('button', { name: 'Needs attention' }),
      ).toBeInTheDocument();
      expect(
        within(inlineFilterGroup('Status')).getByRole('button', { name: 'Not running' }),
      ).toBeInTheDocument();
    });

    it('exposes Type and Status as one-click inline controls', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(inlineFilterGroup('Type')).toBeInTheDocument();
      expect(inlineFilterGroup('Status')).toBeInTheDocument();
      expect(screen.queryByRole('combobox', { name: 'Filter' })).not.toBeInTheDocument();
    });

    it('moves persistent layout controls behind one View button', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByRole('button', { name: 'Grouped' })).not.toBeInTheDocument();

      const dialog = within(openViewPreferences());
      expect(dialog.getByRole('button', { name: 'Grouped' })).toBeInTheDocument();
      expect(dialog.getByRole('button', { name: 'List' })).toBeInTheDocument();
    });

    it('lands large-estate totals beside the type and status labels they describe', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            forcedPlatform: 'proxmox-all',
            inventoryStats: () => ({
              total: 578,
              running: 500,
              degraded: 12,
              stopped: 66,
              vms: 253,
              containers: 325,
              appContainers: 0,
              pods: 0,
            }),
          })}
        />
      ));

      expect(
        within(inlineFilterGroup('Type')).getByRole('button', { name: 'All, 578' }),
      ).toHaveTextContent('578');
      expect(
        within(inlineFilterGroup('Type')).getByRole('button', { name: 'VMs, 253' }),
      ).toHaveTextContent('253');
      expect(
        within(inlineFilterGroup('Type')).getByRole('button', { name: 'LXCs, 325' }),
      ).toHaveTextContent('325');
      expect(
        within(inlineFilterGroup('Status')).getByRole('button', { name: 'Degraded, 12' }),
      ).toHaveTextContent('12');
    });

    it('keeps inventory totals optional through the existing View menu', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            inventoryStats: () => ({
              total: 578,
              running: 500,
              degraded: 12,
              stopped: 66,
              vms: 253,
              containers: 325,
              appContainers: 0,
              pods: 0,
            }),
          })}
        />
      ));

      const dialog = within(openViewPreferences());
      const visibility = dialog.getByRole('group', { name: 'Inventory totals visibility' });
      fireEvent.click(within(visibility).getByRole('button', { name: 'Hide' }));

      expect(
        within(inlineFilterGroup('Type')).getByRole('button', { name: 'All' }),
      ).not.toHaveTextContent('578');
      expect(window.localStorage.getItem('platformEstateOverviewVisible')).toBe('false');
    });

    it('supports one page-owned visibility signal for filters and adjacent headers', () => {
      const setInventoryCountsVisible = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            inventoryStats: () => ({
              total: 578,
              running: 500,
              degraded: 12,
              stopped: 66,
              vms: 253,
              containers: 325,
              appContainers: 0,
              pods: 0,
            }),
            inventoryCountsVisible: () => false,
            setInventoryCountsVisible,
          })}
        />
      ));

      expect(
        within(inlineFilterGroup('Type')).getByRole('button', { name: 'All' }),
      ).not.toHaveTextContent('578');

      const dialog = within(openViewPreferences());
      fireEvent.click(
        within(dialog.getByRole('group', { name: 'Inventory totals visibility' })).getByRole(
          'button',
          { name: 'Show' },
        ),
      );
      expect(setInventoryCountsVisible).toHaveBeenCalledWith(true);
      expect(window.localStorage.getItem('platformEstateOverviewVisible')).toBeNull();
    });

    it('offers a guest/host memory percentage basis when the owning page enables it', () => {
      const setMemoryDisplayBasis = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            memoryDisplayBasis: () => 'guest',
            setMemoryDisplayBasis,
          })}
        />
      ));

      expect(
        screen.queryByRole('group', { name: 'Memory percentage basis' }),
      ).not.toBeInTheDocument();
      const group = within(openViewPreferences()).getByRole('group', {
        name: 'Memory percentage basis',
      });
      fireEvent.click(within(group).getByRole('button', { name: 'Host' }));
      expect(setMemoryDisplayBasis).toHaveBeenCalledWith('host');
    });

    it('keeps metric presentation in View and hides the range for bars', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            metricDisplayMode: () => 'bars',
            setMetricDisplayMode: vi.fn(),
            metricHistoryRange: () => '1h',
            setMetricHistoryRange: vi.fn(),
          })}
        />
      ));

      expect(screen.queryByRole('button', { name: 'Bars' })).not.toBeInTheDocument();
      expect(screen.queryByRole('group', { name: 'Sparkline range' })).not.toBeInTheDocument();
      expect(within(openViewPreferences()).getByRole('button', { name: 'Bars' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
    });

    it('keeps the frequently changed range inline while Trends is active', () => {
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            metricDisplayMode: () => 'sparklines',
            setMetricDisplayMode: vi.fn(),
            metricHistoryRange: () => '1h',
            setMetricHistoryRange: vi.fn(),
          })}
        />
      ));

      expect(screen.getByRole('group', { name: 'Sparkline range' })).toBeInTheDocument();
      expect(screen.getByText('Trend range')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Trends' })).not.toBeInTheDocument();
      expect(within(openViewPreferences()).getByRole('button', { name: 'Trends' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
    });

    it('does not reserve an empty leading-action slot in the default mobile filter rail', () => {
      isMobileMock.mockReturnValue(true);
      render(() => (
        <WorkloadsFilter
          {...makeProps({
            pinnedSelectionActive: () => false,
            onClearPinnedSelection: vi.fn(),
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

      fireEvent.click(screen.getByRole('button', { name: 'Filters' }));

      const actions = screen.getByRole('group', { name: 'Filter actions' });
      const actionCluster = actions.querySelector('[data-filter-action-cluster]');
      expect(actionCluster).toContainElement(
        within(actions).getByRole('combobox', { name: 'Filter' }),
      );
      expect(
        screen.queryByRole('button', { name: 'Clear pinned selection' }),
      ).not.toBeInTheDocument();
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
      expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument();
    });
  });

  describe('type filter', () => {
    it('calls setViewMode when a different inline type is selected', () => {
      const setViewMode = vi.fn();
      render(() => <WorkloadsFilter {...makeProps({ setViewMode })} />);
      pickInlineFilter('Type', 'VMs');
      expect(setViewMode).toHaveBeenCalledWith('vm');
    });

    it.each(['proxmox-pve', 'proxmox-all', 'proxmox'])(
      'limits the %s platform scope to Proxmox workload types',
      (forcedPlatform) => {
        render(() => <WorkloadsFilter {...makeProps({ forcedPlatform })} />);

        const typeFilter = within(inlineFilterGroup('Type'));
        expect(typeFilter.getByRole('button', { name: 'All' })).toBeInTheDocument();
        expect(typeFilter.getByRole('button', { name: 'VMs' })).toBeInTheDocument();
        expect(typeFilter.getByRole('button', { name: 'LXCs' })).toBeInTheDocument();
        expect(typeFilter.queryByRole('button', { name: 'Containers' })).not.toBeInTheDocument();
        expect(typeFilter.queryByRole('button', { name: 'Pods' })).not.toBeInTheDocument();
      },
    );

    it.each([undefined, '', 'all', 'kubernetes'])(
      'keeps global workload types for the %s platform scope',
      (forcedPlatform) => {
        render(() => <WorkloadsFilter {...makeProps({ forcedPlatform })} />);

        const typeFilter = within(inlineFilterGroup('Type'));
        expect(typeFilter.getByRole('button', { name: 'Containers' })).toBeInTheDocument();
        expect(typeFilter.getByRole('button', { name: 'Pods' })).toBeInTheDocument();
        expect(typeFilter.queryByRole('button', { name: 'LXCs' })).not.toBeInTheDocument();
      },
    );
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
      fireEvent.click(within(openViewPreferences()).getByRole('button', { name: 'List' }));
      expect(setGroupingMode).toHaveBeenCalledWith('flat');
    });

    it('calls setGroupingMode("grouped") when the Grouped view-option is clicked', () => {
      const setGroupingMode = vi.fn();
      render(() => (
        <WorkloadsFilter
          {...makeProps({ groupingMode: vi.fn(() => 'flat' as const), setGroupingMode })}
        />
      ));
      fireEvent.click(within(openViewPreferences()).getByRole('button', { name: 'Grouped' }));
      expect(setGroupingMode).toHaveBeenCalledWith('grouped');
    });
  });

  describe('clear filters', () => {
    it('does not render when all filters are at their defaults', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument();
    });

    it('renders when search is non-empty', () => {
      render(() => <WorkloadsFilter {...makeProps({ search: vi.fn(() => 'foo') })} />);
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
    });

    it('renders when viewMode is not the default', () => {
      render(() => <WorkloadsFilter {...makeProps({ viewMode: vi.fn(() => 'vm' as const) })} />);
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
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
      expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument();
    });

    it('renders when statusMode is not the default', () => {
      render(() => (
        <WorkloadsFilter {...makeProps({ statusMode: vi.fn(() => 'running' as const) })} />
      ));
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
    });

    it('does not render when only groupingMode is "flat" (view option, not a filter)', () => {
      render(() => (
        <WorkloadsFilter {...makeProps({ groupingMode: vi.fn(() => 'flat' as const) })} />
      ));
      expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument();
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
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
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
                { value: 'podman', label: 'podman' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument();
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
                { value: 'podman', label: 'podman' },
              ],
              onChange: runtimeOnChange,
            },
          })}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));

      expect(setSearch).toHaveBeenCalledWith('');
      expect(setSortKey).toHaveBeenCalledWith(DEFAULT_WORKLOADS_SORT_KEY);
      expect(setSortDirection).toHaveBeenCalledWith(DEFAULT_WORKLOADS_SORT_DIRECTION);
      expect(setViewMode).toHaveBeenCalledWith(DEFAULT_WORKLOADS_VIEW_MODE);
      expect(setStatusMode).toHaveBeenCalledWith(DEFAULT_WORKLOADS_STATUS_MODE);
      expect(setGroupingMode).not.toHaveBeenCalled();
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

      fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));

      expect(setSearch).toHaveBeenCalledWith('');
      expect(setSortKey).toHaveBeenCalledWith('name');
      expect(setViewMode).not.toHaveBeenCalled();
    });
  });

  describe('host filter', () => {
    it('appears in the direct Filter selector when hostFilter prop is provided', () => {
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
      expect(addFilterOption('Agent', 'pve1')).toBeInTheDocument();
    });

    it('does not appear when hostFilter prop is absent', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByRole('combobox', { name: 'Filter' })).not.toBeInTheDocument();
    });

    it('calls onChange when host selection changes through the direct selector', () => {
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
              options: [
                { value: '', label: 'All clusters' },
                { value: 'prod', label: 'prod' },
              ],
              onChange: vi.fn(),
            },
          })}
        />
      ));
      expect(addFilterOption('K8s cluster', 'prod')).toBeInTheDocument();
    });
  });

  describe('platform filter', () => {
    it('appears in the direct Filter selector when platformFilter prop is provided', () => {
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
      expect(addFilterOption('Platform', 'Proxmox')).toBeInTheDocument();
    });

    it('calls onChange when platform selection changes through the direct selector', () => {
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
    it('appears in the direct Filter selector when namespaceFilter prop is provided', () => {
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
      expect(addFilterOption('Namespace', 'default')).toBeInTheDocument();
    });

    it('calls onChange when namespace selection changes through the direct selector', () => {
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
    it('renders inline runtime chips when viewMode is a container variant and containerRuntimeFilter is provided', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'container' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All runtimes' },
            { value: 'docker', label: 'docker' },
            { value: 'podman', label: 'podman' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      const runtimeGroup = inlineFilterGroup('Runtime');
      expect(within(runtimeGroup).getByRole('button', { name: 'Docker' })).toBeInTheDocument();
      expect(within(runtimeGroup).getByRole('button', { name: 'Podman' })).toBeInTheDocument();
    });

    it('does not render when viewMode is not a container variant', () => {
      const props = makeProps({
        viewMode: vi.fn(() => 'vm' as const),
        containerRuntimeFilter: {
          value: '',
          options: [
            { value: '', label: 'All runtimes' },
            { value: 'docker', label: 'docker' },
            { value: 'podman', label: 'podman' },
          ],
          onChange: vi.fn(),
        },
      });
      render(() => <WorkloadsFilter {...props} />);
      expect(screen.queryByRole('group', { name: 'Runtime' })).not.toBeInTheDocument();
    });

    it('calls onChange when a runtime chip is clicked', () => {
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
                { value: 'podman', label: 'podman' },
              ],
              onChange,
            },
          })}
        />
      ));
      pickInlineFilter('Runtime', 'Docker');
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
      expect(screen.queryByRole('button', { name: 'Hide charts' })).not.toBeInTheDocument();
      expect(
        within(openViewPreferences()).getByRole('button', { name: 'Hide charts' }),
      ).toBeInTheDocument();
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
      expect(
        within(openViewPreferences()).getByRole('button', { name: 'Show charts' }),
      ).toBeInTheDocument();
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
      expect(screen.queryByTestId('column-picker')).not.toBeInTheDocument();
      expect(within(openViewPreferences()).getByTestId('column-picker')).toHaveAttribute(
        'data-inline',
        'true',
      );
    });

    it('does not render ColumnPicker when columnVisibility is absent', () => {
      render(() => <WorkloadsFilter {...makeProps()} />);
      expect(screen.queryByTestId('column-picker')).not.toBeInTheDocument();
    });
  });
});
