import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';

// --- Mocks (must be before component import) ---
const mockIsMobile = vi.fn(() => false);

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({ isMobile: mockIsMobile }),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
  },
}));

vi.mock('@/components/shared/StatusBadge', () => ({
  StatusBadge: (props: any) => (
    <button
      data-testid="status-badge"
      onClick={() => props.onToggle?.()}
      title={props.isEnabled ? props.titleEnabled : props.titleDisabled}
    >
      {props.isEnabled ? (props.labelEnabled ?? 'On') : (props.labelDisabled ?? 'Off')}
    </button>
  ),
}));

vi.mock('@/components/shared/Toggle', () => ({
  TogglePrimitive: (props: any) => (
    <button
      data-testid="toggle"
      role="switch"
      aria-checked={props.checked}
      aria-label={props.ariaLabel}
      disabled={props.disabled}
      title={props.title}
      onClick={() => !props.disabled && props.onToggle?.()}
    >
      {props.checked ? 'ON' : 'OFF'}
    </button>
  ),
}));

vi.mock('@/components/shared/Card', () => ({
  Card: (props: any) => (
    <div data-testid="card" class={props.class}>
      {props.children}
    </div>
  ),
}));

vi.mock('@/components/shared/Table', () => ({
  Table: (props: any) => (
    <table data-testid="table" class={props.class}>
      {props.children}
    </table>
  ),
  TableHeader: (props: any) => <thead>{props.children}</thead>,
  TableBody: (props: any) => <tbody class={props.class}>{props.children}</tbody>,
  TableRow: (props: any) => <tr class={props.class}>{props.children}</tr>,
  TableHead: (props: any) => (
    <th class={props.class} title={props.title}>
      {props.children}
    </th>
  ),
  TableCell: (props: any) => (
    <td class={props.class} colspan={props.colspan}>
      {props.children}
    </td>
  ),
}));

vi.mock('@/components/shared/SectionHeader', () => ({
  SectionHeader: (props: any) => <h3 data-testid="section-header">{props.title}</h3>,
}));

vi.mock('@/components/Dashboard/ThresholdSlider', () => ({
  ThresholdSlider: (props: any) => (
    <input
      data-testid="threshold-slider"
      type="range"
      value={props.value}
      min={props.min}
      max={props.max}
      disabled={props.disabled}
      onChange={(e) => props.onChange?.(Number(e.currentTarget.value))}
    />
  ),
}));

vi.mock('@/components/shared/HelpIcon', () => ({
  HelpIcon: () => <span data-testid="help-icon" />,
}));

vi.mock('lucide-solid/icons/rotate-ccw', () => ({
  default: (props: any) => <svg data-testid="rotate-ccw-icon" class={props.class} />,
}));

import { ResourceTable, type Resource } from './ResourceTable';

// --- Helpers ---
function makeResource(overrides: Partial<Resource> = {}): Resource {
  return {
    id: 'res-1',
    name: 'Test VM',
    thresholds: { cpu: 80, memory: 85 },
    defaults: { cpu: 80, memory: 85 },
    ...overrides,
  };
}

interface DefaultProps {
  title: string;
  resources: Resource[];
  columns: string[];
  onEdit: ReturnType<typeof vi.fn>;
  onSaveEdit: ReturnType<typeof vi.fn>;
  onCancelEdit: ReturnType<typeof vi.fn>;
  onRemoveOverride: ReturnType<typeof vi.fn>;
  editingId: () => string | null;
  editingThresholds: () => Record<string, number | undefined>;
  setEditingThresholds: ReturnType<typeof vi.fn>;
  formatMetricValue: ReturnType<typeof vi.fn>;
  hasActiveAlert: ReturnType<typeof vi.fn>;
  editingNote: () => string;
  setEditingNote: ReturnType<typeof vi.fn>;
}

function makeProps(
  overrides: Partial<DefaultProps> & Record<string, any> = {},
): DefaultProps & Record<string, any> {
  return {
    title: 'Virtual Machines',
    resources: [makeResource()],
    columns: ['CPU %', 'Memory %'],
    onEdit: vi.fn(),
    onSaveEdit: vi.fn(),
    onCancelEdit: vi.fn(),
    onRemoveOverride: vi.fn(),
    editingId: () => null,
    editingThresholds: () => ({}),
    setEditingThresholds: vi.fn(),
    formatMetricValue: vi.fn((metric: string, value: number | undefined) =>
      value !== undefined ? String(value) : '-',
    ),
    hasActiveAlert: vi.fn(() => false),
    editingNote: () => '',
    setEditingNote: vi.fn(),
    ...overrides,
  };
}

// --- Tests ---
describe('ResourceTable', () => {
  beforeEach(() => {
    mockIsMobile.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  describe('rendering basics', () => {
    it('renders the section header with the provided title', () => {
      const props = makeProps({ title: 'Nodes' });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByTestId('section-header')).toHaveTextContent('Nodes');
    });

    it('renders column headers', () => {
      const props = makeProps({ columns: ['CPU %', 'Memory %', 'Disk %'] });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('CPU %')).toBeInTheDocument();
      expect(screen.getByText('Memory %')).toBeInTheDocument();
      expect(screen.getByText('Disk %')).toBeInTheDocument();
    });

    it('renders resource name', () => {
      const props = makeProps({
        resources: [makeResource({ name: 'my-vm-100' })],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('my-vm-100')).toBeInTheDocument();
    });

    it('renders multiple resources', () => {
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM Alpha' }),
          makeResource({ id: 'vm-2', name: 'VM Beta' }),
          makeResource({ id: 'vm-3', name: 'VM Gamma' }),
        ],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('VM Alpha')).toBeInTheDocument();
      expect(screen.getByText('VM Beta')).toBeInTheDocument();
      expect(screen.getByText('VM Gamma')).toBeInTheDocument();
    });

    it('renders Alerts, Resource, and Actions column headers', () => {
      const props = makeProps();
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Alerts')).toBeInTheDocument();
      expect(screen.getByText('Resource')).toBeInTheDocument();
      expect(screen.getByText('Actions')).toBeInTheDocument();
    });
  });

  describe('column header tooltips', () => {
    it('renders tooltips for known columns', () => {
      const props = makeProps({ columns: ['CPU %', 'Memory %'] });
      render(() => <ResourceTable {...props} />);

      const cpuHeader = screen.getByText('CPU %');
      expect(cpuHeader.closest('th')).toHaveAttribute(
        'title',
        'Percent CPU utilization allowed before an alert fires.',
      );

      const memHeader = screen.getByText('Memory %');
      expect(memHeader.closest('th')).toHaveAttribute(
        'title',
        'Percent memory usage threshold for triggering alerts.',
      );
    });
  });

  describe('resource display and editing', () => {
    it('calls onEdit with correct args when edit button is clicked', () => {
      const onEdit = vi.fn();
      const resource = makeResource({
        id: 'vm-1',
        name: 'Test VM',
        thresholds: { cpu: 90 },
        defaults: { cpu: 80 },
        note: 'test note',
      });
      const props = makeProps({ resources: [resource], onEdit });
      render(() => <ResourceTable {...props} />);

      const editButton = screen.getByLabelText('Edit thresholds for Test VM');
      fireEvent.click(editButton);

      expect(onEdit).toHaveBeenCalledWith('vm-1', { cpu: 90 }, { cpu: 80 }, 'test note');
    });

    it('shows cancel button when editing a resource', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'Editing VM' })],
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 90 }),
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByLabelText('Cancel editing')).toBeInTheDocument();
    });

    it('calls onCancelEdit when cancel button is clicked', () => {
      const onCancelEdit = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1' })],
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 90 }),
        onCancelEdit,
      });
      render(() => <ResourceTable {...props} />);

      fireEvent.click(screen.getByLabelText('Cancel editing'));
      expect(onCancelEdit).toHaveBeenCalled();
    });

    it('shows Custom badge for resources with overrides', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'Custom VM', hasOverride: true })],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Custom')).toBeInTheDocument();
    });

    it('calls onRemoveOverride with resource id when revert button is clicked', () => {
      const onRemoveOverride = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'Overridden VM', hasOverride: true })],
        onRemoveOverride,
      });
      render(() => <ResourceTable {...props} />);

      const revertBtn = screen.getByLabelText('Revert to defaults for Overridden VM');
      fireEvent.click(revertBtn);
      expect(onRemoveOverride).toHaveBeenCalledWith('vm-1');
    });

    it('does not show edit button for dockerHost resources (grouped)', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          host1: [makeResource({ id: 'dh-1', name: 'Docker Host', type: 'dockerHost' })],
        },
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.queryByLabelText('Edit thresholds for Docker Host')).not.toBeInTheDocument();
    });

    it('does not show edit button for non-editable flat resources', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'dh-1', name: 'Docker Host', editable: false })],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.queryByLabelText('Edit thresholds for Docker Host')).not.toBeInTheDocument();
      // Non-editable resources show an em-dash placeholder
      expect(screen.getByText('—')).toBeInTheDocument();
    });

    it('renders a note for grouped resources when not editing', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'Noted VM',
              note: 'Threshold lowered for maintenance',
            }),
          ],
        },
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Threshold lowered for maintenance')).toBeInTheDocument();
    });

    it('renders note editor textarea when editing grouped resource', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1' })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 90 }),
        editingNote: () => 'My editing note',
      });
      render(() => <ResourceTable {...props} />);

      const textarea = screen.getByPlaceholderText('Add a note about this override (optional)');
      expect(textarea).toBeInTheDocument();
      expect(textarea).toHaveValue('My editing note');
    });

    it('calls setEditingNote when note textarea input changes', () => {
      const setEditingNote = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1' })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 90 }),
        editingNote: () => '',
        setEditingNote,
      });
      render(() => <ResourceTable {...props} />);

      const textarea = screen.getByPlaceholderText('Add a note about this override (optional)');
      fireEvent.input(textarea, { target: { value: 'Updated note' } });
      expect(setEditingNote).toHaveBeenCalledWith('Updated note');
    });
  });

  describe('metric display values', () => {
    it('shows formatted metric values using formatMetricValue', () => {
      const formatMetricValue = vi.fn((metric: string, value: number | undefined) => {
        if (metric === 'cpu') return '90%';
        if (metric === 'memory') return '85%';
        return String(value);
      });

      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', thresholds: { cpu: 90, memory: 85 } })],
        formatMetricValue,
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('90%')).toBeInTheDocument();
      expect(screen.getByText('85%')).toBeInTheDocument();
    });

    it('shows "Off" text for disabled metrics (value -1) with disabled styling', () => {
      const formatMetricValue = vi.fn((metric: string, value: number | undefined) =>
        value !== undefined ? String(value) : '-',
      );
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', thresholds: { cpu: -1, memory: 85 } })],
        columns: ['CPU %', 'Memory %'],
        formatMetricValue,
      });
      render(() => <ResourceTable {...props} />);

      // The disabled metric (cpu = -1) renders "Off" with italic styling
      const offElements = screen.getAllByText('Off');
      const disabledOff = offElements.find((el) => el.className.includes('italic'));
      expect(disabledOff).toBeTruthy();

      // formatMetricValue should NOT be called for disabled metrics (they show "Off" directly)
      const cpuCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'cpu',
      );
      expect(cpuCalls.length).toBe(0);

      // Memory (85) should still be formatted normally
      const memoryCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'memory',
      );
      expect(memoryCalls.length).toBeGreaterThanOrEqual(1);
    });

    it('shows active alert indicator dot when hasActiveAlert returns true', () => {
      const hasActiveAlert = vi.fn((resourceId: string, metric: string) => metric === 'cpu');
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', thresholds: { cpu: 90, memory: 85 } })],
        hasActiveAlert,
      });
      render(() => <ResourceTable {...props} />);

      // The active alert indicator is a pulsing red dot
      const alertDot = document.querySelector('[title="Active alert"]');
      expect(alertDot).toBeTruthy();
      expect(alertDot!.className).toContain('animate-pulse');
    });
  });

  describe('alert toggle', () => {
    it('renders alert toggle when onToggleDisabled is provided', () => {
      const onToggleDisabled = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', disabled: false })],
        onToggleDisabled,
      });
      render(() => <ResourceTable {...props} />);

      const toggle = screen.getByRole('switch', { name: 'Alerts enabled for this resource' });
      expect(toggle).toBeInTheDocument();
    });

    it('calls onToggleDisabled with resource id when toggle is clicked', () => {
      const onToggleDisabled = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', disabled: false })],
        onToggleDisabled,
      });
      render(() => <ResourceTable {...props} />);

      const toggle = screen.getByRole('switch', { name: 'Alerts enabled for this resource' });
      fireEvent.click(toggle);
      expect(onToggleDisabled).toHaveBeenCalledWith('vm-1');
    });

    it('disables resource toggles when global disable is active', () => {
      const onToggleDisabled = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', disabled: false })],
        onToggleDisabled,
        globalDisableFlag: () => true,
      });
      render(() => <ResourceTable {...props} />);

      const toggle = screen.getByRole('switch', { name: 'Alerts disabled for this resource' });
      expect(toggle).toBeDisabled();
    });

    it('applies opacity to disabled resources', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', disabled: true })],
        onToggleDisabled: vi.fn(),
      });
      render(() => <ResourceTable {...props} />);

      const row = screen.getByText('Test VM').closest('tr');
      expect(row?.className).toContain('opacity-40');
    });
  });

  describe('global defaults row', () => {
    it('renders global defaults row when all three required props are provided', () => {
      const props = makeProps({
        globalDefaults: { cpu: 80, memory: 85 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Global Defaults')).toBeInTheDocument();
    });

    it('shows Custom badge when global defaults differ from factory defaults', () => {
      const props = makeProps({
        resources: [],
        globalDefaults: { cpu: 90, memory: 85 },
        factoryDefaults: { cpu: 80, memory: 85 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Custom')).toBeInTheDocument();
    });

    it('does not show Custom badge when defaults match factory', () => {
      const props = makeProps({
        resources: [],
        globalDefaults: { cpu: 80, memory: 85 },
        factoryDefaults: { cpu: 80, memory: 85 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Global Defaults')).toBeInTheDocument();
      expect(screen.queryByText('Custom')).not.toBeInTheDocument();
    });

    it('renders global toggle when onToggleGlobalDisable is provided', () => {
      const onToggleGlobalDisable = vi.fn();
      const props = makeProps({
        resources: [],
        globalDefaults: { cpu: 80 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
        onToggleGlobalDisable,
        globalDisableFlag: () => false,
      });
      render(() => <ResourceTable {...props} />);

      const globalToggle = screen.getByRole('switch', { name: 'Global alerts toggle' });
      expect(globalToggle).toBeInTheDocument();
    });

    it('calls onToggleGlobalDisable and setHasUnsavedChanges when global toggle is clicked', () => {
      const onToggleGlobalDisable = vi.fn();
      const setHasUnsavedChanges = vi.fn();
      const props = makeProps({
        resources: [],
        globalDefaults: { cpu: 80 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges,
        onToggleGlobalDisable,
        globalDisableFlag: () => false,
      });
      render(() => <ResourceTable {...props} />);

      const globalToggle = screen.getByRole('switch', { name: 'Global alerts toggle' });
      fireEvent.click(globalToggle);
      expect(onToggleGlobalDisable).toHaveBeenCalled();
      expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    });

    it('calls setGlobalDefaults and setHasUnsavedChanges on input change', () => {
      const setGlobalDefaults = vi.fn();
      const setHasUnsavedChanges = vi.fn();
      const props = makeProps({
        resources: [],
        globalDefaults: { cpu: 80 },
        setGlobalDefaults,
        setHasUnsavedChanges,
        columns: ['CPU %'],
      });
      render(() => <ResourceTable {...props} />);

      // Find the global defaults input (only one when no resources and one column)
      const input = screen.getByDisplayValue('80');
      fireEvent.input(input, { target: { value: '95' } });

      expect(setGlobalDefaults).toHaveBeenCalled();
      expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    });

    it('shows reset to factory defaults button when custom and onResetDefaults provided', () => {
      const onResetDefaults = vi.fn();
      const props = makeProps({
        resources: [],
        globalDefaults: { cpu: 95 },
        factoryDefaults: { cpu: 80 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
        onResetDefaults,
      });
      render(() => <ResourceTable {...props} />);

      const resetBtn = screen.getByLabelText('Reset to factory defaults');
      fireEvent.click(resetBtn);
      expect(onResetDefaults).toHaveBeenCalled();
    });
  });

  describe('grouped resources', () => {
    it('renders group headers for grouped resources', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          'node-a': [makeResource({ id: 'vm-1', name: 'VM 1' })],
          'node-b': [makeResource({ id: 'vm-2', name: 'VM 2' })],
        },
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('node-a')).toBeInTheDocument();
      expect(screen.getByText('node-b')).toBeInTheDocument();
      expect(screen.getByText('VM 1')).toBeInTheDocument();
      expect(screen.getByText('VM 2')).toBeInTheDocument();
    });

    it('renders node group headers with display name and cluster name', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          pve1: [makeResource({ id: 'vm-1', name: 'VM 1' })],
        },
        groupHeaderMeta: {
          pve1: {
            type: 'agent',
            displayName: 'PVE Node 1',
            clusterName: 'production',
          },
        },
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('PVE Node 1')).toBeInTheDocument();
      expect(screen.getByText('production')).toBeInTheDocument();
    });

    it('renders group header with link when host is provided', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          pve1: [makeResource({ id: 'vm-1', name: 'VM 1' })],
        },
        groupHeaderMeta: {
          pve1: {
            type: 'agent',
            displayName: 'PVE Node 1',
            host: 'https://pve1.example.com:8006',
          },
        },
      });
      render(() => <ResourceTable {...props} />);

      const link = screen.getByText('PVE Node 1');
      expect(link.tagName).toBe('A');
      expect(link).toHaveAttribute('href', 'https://pve1.example.com:8006');
      expect(link).toHaveAttribute('target', '_blank');
    });
  });

  describe('offline alerts column', () => {
    it('renders Offline Alerts column header when showOfflineAlertsColumn is true', () => {
      const props = makeProps({ showOfflineAlertsColumn: true });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Offline Alerts')).toBeInTheDocument();
    });

    it('does not render Offline Alerts column header when showOfflineAlertsColumn is false', () => {
      const props = makeProps({ showOfflineAlertsColumn: false });
      render(() => <ResourceTable {...props} />);

      expect(screen.queryByText('Offline Alerts')).not.toBeInTheDocument();
    });
  });

  describe('bulk edit', () => {
    it('renders select-all checkbox when onBulkEdit is provided', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM 1' }),
          makeResource({ id: 'vm-2', name: 'VM 2' }),
        ],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByLabelText('Select all resources')).toBeInTheDocument();
    });

    it('renders per-resource checkboxes when onBulkEdit is provided', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM Alpha' }),
          makeResource({ id: 'vm-2', name: 'VM Beta' }),
        ],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByLabelText('Select VM Alpha')).toBeInTheDocument();
      expect(screen.getByLabelText('Select VM Beta')).toBeInTheDocument();
    });

    it('does not render checkboxes when onBulkEdit is not provided', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'VM Alpha' })],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.queryByLabelText('Select all resources')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Select VM Alpha')).not.toBeInTheDocument();
    });

    it('toggles individual resource selection', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM Alpha' }),
          makeResource({ id: 'vm-2', name: 'VM Beta' }),
        ],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      const checkbox = screen.getByLabelText('Select VM Alpha') as HTMLInputElement;
      expect(checkbox.checked).toBe(false);

      fireEvent.change(checkbox, { target: { checked: true } });
      expect(checkbox.checked).toBe(true);
    });

    it('select-all checks all resource checkboxes', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM Alpha' }),
          makeResource({ id: 'vm-2', name: 'VM Beta' }),
        ],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      const selectAll = screen.getByLabelText('Select all resources') as HTMLInputElement;
      fireEvent.change(selectAll, { target: { checked: true } });

      const checkboxAlpha = screen.getByLabelText('Select VM Alpha') as HTMLInputElement;
      const checkboxBeta = screen.getByLabelText('Select VM Beta') as HTMLInputElement;
      expect(checkboxAlpha.checked).toBe(true);
      expect(checkboxBeta.checked).toBe(true);
    });

    it('calls onBulkEdit with selected resource ids when Bulk Edit Settings is clicked', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM Alpha' }),
          makeResource({ id: 'vm-2', name: 'VM Beta' }),
          makeResource({ id: 'vm-3', name: 'VM Gamma' }),
        ],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      // Select two resources
      fireEvent.change(screen.getByLabelText('Select VM Alpha'), { target: { checked: true } });
      fireEvent.change(screen.getByLabelText('Select VM Gamma'), { target: { checked: true } });

      // The floating action bar with "Bulk Edit Settings" should appear
      const bulkEditBtn = screen.getByText('Bulk Edit Settings');
      fireEvent.click(bulkEditBtn);

      expect(onBulkEdit).toHaveBeenCalledTimes(1);
      const calledWith = onBulkEdit.mock.calls[0][0] as string[];
      expect(calledWith).toHaveLength(2);
      expect(calledWith).toContain('vm-1');
      expect(calledWith).toContain('vm-3');
    });

    it('clears selection after bulk edit is triggered', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'VM Alpha' }),
          makeResource({ id: 'vm-2', name: 'VM Beta' }),
        ],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      // Select a resource
      fireEvent.change(screen.getByLabelText('Select VM Alpha'), { target: { checked: true } });

      // Click bulk edit
      fireEvent.click(screen.getByText('Bulk Edit Settings'));

      // After bulk edit, selection should be cleared and the floating bar should disappear
      expect(screen.queryByText('Bulk Edit Settings')).not.toBeInTheDocument();
    });

    it('clears selection when clear button is clicked', () => {
      const onBulkEdit = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'VM Alpha' })],
        onBulkEdit,
      });
      render(() => <ResourceTable {...props} />);

      // Select a resource
      fireEvent.change(screen.getByLabelText('Select VM Alpha'), { target: { checked: true } });
      expect(screen.getByText('Bulk Edit Settings')).toBeInTheDocument();

      // Click clear selection
      fireEvent.click(screen.getByLabelText('Clear selection'));
      expect(screen.queryByText('Bulk Edit Settings')).not.toBeInTheDocument();
    });
  });

  describe('resource type-specific metric support', () => {
    it('only formats supported metrics for storage type (usage only)', () => {
      const formatMetricValue = vi.fn(() => '80');
      const props = makeProps({
        resources: [
          makeResource({ id: 'st-1', name: 'Storage', type: 'storage', thresholds: { usage: 80 } }),
        ],
        columns: ['CPU %', 'Memory %', 'Usage %'],
        formatMetricValue,
      });
      render(() => <ResourceTable {...props} />);

      // Storage only supports 'usage' — cpu and memory should NOT be formatted
      const usageCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'usage',
      );
      expect(usageCalls.length).toBeGreaterThanOrEqual(1);

      const cpuCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'cpu',
      );
      const memoryCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'memory',
      );
      expect(cpuCalls.length).toBe(0);
      expect(memoryCalls.length).toBe(0);
    });

    it('hides disk/network metrics for node type', () => {
      const formatMetricValue = vi.fn(() => '80');
      const props = makeProps({
        resources: [
          makeResource({
            id: 'node-1',
            name: 'Node 1',
            type: 'agent',
            thresholds: { cpu: 80 },
          }),
        ],
        columns: ['CPU %', 'Disk R MB/s', 'Disk W MB/s', 'Net In MB/s', 'Net Out MB/s'],
        formatMetricValue,
      });
      render(() => <ResourceTable {...props} />);

      // formatMetricValue should only be called for 'cpu', not for unsupported network/disk metrics
      const cpuCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'cpu',
      );
      expect(cpuCalls.length).toBeGreaterThanOrEqual(1);

      // Unsupported metrics should not call formatMetricValue
      const unsupportedCalls = formatMetricValue.mock.calls.filter(([metric]: [string]) =>
        ['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric),
      );
      expect(unsupportedCalls.length).toBe(0);
    });

    it('shows only cpu and memory for pbs type', () => {
      const formatMetricValue = vi.fn(() => '80');
      const props = makeProps({
        resources: [
          makeResource({
            id: 'pbs-1',
            name: 'PBS Node',
            type: 'pbs',
            thresholds: { cpu: 80, memory: 85, disk: 90 },
          }),
        ],
        columns: ['CPU %', 'Memory %', 'Disk %'],
        formatMetricValue,
      });
      render(() => <ResourceTable {...props} />);

      // Disk should not call formatMetricValue since PBS doesn't support it
      const diskCalls = formatMetricValue.mock.calls.filter(
        ([metric]: [string]) => metric === 'disk',
      );
      expect(diskCalls.length).toBe(0);
    });
  });

  describe('node type rendering', () => {
    it('prefers displayName for agent resources and action labels', () => {
      const props = makeProps({
        resources: [
          makeResource({
            id: 'node-1',
            name: 'pve-node-1',
            displayName: 'PVE Node 1',
            type: 'agent',
            host: 'https://192.168.0.10:8006',
          }),
        ],
      });
      render(() => <ResourceTable {...props} />);

      const link = screen.getByText('PVE Node 1');
      expect(link.tagName).toBe('A');
      expect(link).toHaveAttribute('href', 'https://192.168.0.10:8006');
      expect(screen.getByLabelText('Edit thresholds for PVE Node 1')).toBeInTheDocument();
    });

    it('renders node name as a link when host URL is provided', () => {
      const props = makeProps({
        resources: [
          makeResource({
            id: 'node-1',
            name: 'pve-node-1',
            type: 'agent',
            host: 'https://192.168.0.10:8006',
          }),
        ],
      });
      render(() => <ResourceTable {...props} />);

      const link = screen.getByText('pve-node-1');
      expect(link.tagName).toBe('A');
      expect(link).toHaveAttribute('href', 'https://192.168.0.10:8006');
    });

    it('renders cluster name badge for node resources', () => {
      const props = makeProps({
        resources: [
          makeResource({
            id: 'node-1',
            name: 'pve-node-1',
            type: 'agent',
            clusterName: 'home-lab',
          }),
        ],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('home-lab')).toBeInTheDocument();
    });

    it('renders node name as plain text when no host URL', () => {
      const props = makeProps({
        resources: [
          makeResource({
            id: 'node-1',
            name: 'pve-node-1',
            type: 'agent',
          }),
        ],
        columns: ['CPU %'],
      });
      render(() => <ResourceTable {...props} />);

      const nameEl = screen.getByText('pve-node-1');
      expect(nameEl).toBeInTheDocument();
      // Without host, name renders as SPAN, not a link
      expect(nameEl.tagName).toBe('SPAN');
    });
  });

  describe('mobile view', () => {
    beforeEach(() => {
      mockIsMobile.mockReturnValue(true);
    });

    it('renders mobile card layout instead of table', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'Mobile VM' })],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.queryByTestId('table')).not.toBeInTheDocument();
      expect(screen.getByText('Mobile VM')).toBeInTheDocument();
    });

    it('shows custom empty message when no resources', () => {
      const props = makeProps({
        resources: [],
        emptyMessage: 'Nothing here',
        globalDefaults: undefined,
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Nothing here')).toBeInTheDocument();
    });

    it('shows default empty message when none specified', () => {
      const props = makeProps({
        resources: [],
        globalDefaults: undefined,
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('No resources available.')).toBeInTheDocument();
    });

    it('renders global defaults card when provided', () => {
      const props = makeProps({
        resources: [makeResource()],
        globalDefaults: { cpu: 80, memory: 85 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Global Defaults')).toBeInTheDocument();
    });

    it('calls onEdit when edit button is clicked', () => {
      const onEdit = vi.fn();
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'Mobile VM' })],
        onEdit,
      });
      render(() => <ResourceTable {...props} />);

      const editBtn = screen.getByLabelText('Edit thresholds for Mobile VM');
      fireEvent.click(editBtn);
      expect(onEdit).toHaveBeenCalled();
    });

    it('renders subtitle when present', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'VM', subtitle: 'Running on pve1' })],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Running on pve1')).toBeInTheDocument();
    });

    it('renders note textarea in editing mode', () => {
      const props = makeProps({
        resources: [makeResource({ id: 'vm-1', name: 'Mobile VM' })],
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 80 }),
        editingNote: () => 'mobile note',
      });
      render(() => <ResourceTable {...props} />);

      const textarea = screen.getByPlaceholderText('Add a note...');
      expect(textarea).toBeInTheDocument();
      expect(textarea).toHaveValue('mobile note');
    });

    it('prefers displayName over name', () => {
      const props = makeProps({
        resources: [
          makeResource({ id: 'vm-1', name: 'vm-100', displayName: 'Production Web Server' }),
        ],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByText('Production Web Server')).toBeInTheDocument();
    });
  });

  describe('delay settings', () => {
    it('shows delay toggle button when showDelayColumn and onMetricDelayChange are provided', () => {
      const props = makeProps({
        showDelayColumn: true,
        onMetricDelayChange: vi.fn(),
        globalDefaults: { cpu: 80 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
      });
      render(() => <ResourceTable {...props} />);

      const delayToggle = screen.getByLabelText('Show alert delay settings');
      expect(delayToggle).toBeInTheDocument();
    });

    it('toggles delay row visibility when clicked', () => {
      const props = makeProps({
        showDelayColumn: true,
        onMetricDelayChange: vi.fn(),
        globalDefaults: { cpu: 80 },
        setGlobalDefaults: vi.fn(),
        setHasUnsavedChanges: vi.fn(),
        globalDelaySeconds: 5,
      });
      render(() => <ResourceTable {...props} />);

      // Initially delay row is hidden
      expect(screen.queryByText('Alert Delay (s)')).not.toBeInTheDocument();

      // Click to show
      fireEvent.click(screen.getByLabelText('Show alert delay settings'));
      expect(screen.getByText('Alert Delay (s)')).toBeInTheDocument();

      // Click to hide
      fireEvent.click(screen.getByLabelText('Hide alert delay settings'));
      expect(screen.queryByText('Alert Delay (s)')).not.toBeInTheDocument();
    });
  });

  describe('editing thresholds in desktop view', () => {
    it('shows number inputs when editing a grouped resource', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1', name: 'VM1' })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 85, memory: 90 }),
        columns: ['CPU %', 'Memory %'],
      });
      render(() => <ResourceTable {...props} />);

      // When editing, number inputs should appear for each metric
      const inputs = document.querySelectorAll('input[type="number"]');
      expect(inputs.length).toBeGreaterThanOrEqual(2);
    });

    it('calls setEditingThresholds when a threshold input changes', () => {
      const setEditingThresholds = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1', name: 'VM1', defaults: { cpu: 80 } })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 85 }),
        setEditingThresholds,
        columns: ['CPU %'],
      });
      render(() => <ResourceTable {...props} />);

      // Find the editing input with the "Set to -1" title (unique to resource editing inputs)
      const editInputs = document.querySelectorAll(
        'input[type="number"][title="Set to -1 to disable alerts for this metric"]',
      );
      expect(editInputs.length).toBeGreaterThanOrEqual(1);
      fireEvent.input(editInputs[0], { target: { value: '95' } });

      expect(setEditingThresholds).toHaveBeenCalledWith(expect.objectContaining({ cpu: 95 }));
    });

    it('calls onSaveEdit on input blur', () => {
      const onSaveEdit = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1', name: 'VM1', defaults: { cpu: 80 } })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 85 }),
        onSaveEdit,
        columns: ['CPU %'],
      });
      render(() => <ResourceTable {...props} />);

      const editInputs = document.querySelectorAll(
        'input[type="number"][title="Set to -1 to disable alerts for this metric"]',
      );
      expect(editInputs.length).toBeGreaterThanOrEqual(1);
      fireEvent.blur(editInputs[0]);

      expect(onSaveEdit).toHaveBeenCalledWith('vm-1');
    });
  });

  describe('offline state button', () => {
    it('renders "Warn" for guest resources with default state', () => {
      const onSetOfflineState = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'VM1',
              type: 'guest',
              disableConnectivity: false,
            }),
          ],
        },
        showOfflineAlertsColumn: true,
        onSetOfflineState,
      });
      render(() => <ResourceTable {...props} />);

      const warnButton = screen.getByText('Warn');
      expect(warnButton).toBeInTheDocument();
      expect(warnButton.title).toContain('warning-level');
    });

    it('renders "Off" for resources with disableConnectivity', () => {
      const onSetOfflineState = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'VM1',
              type: 'guest',
              disableConnectivity: true,
            }),
          ],
        },
        showOfflineAlertsColumn: true,
        onSetOfflineState,
      });
      render(() => <ResourceTable {...props} />);

      const offButton = screen.getByText('Off', { selector: 'button' });
      expect(offButton.title).toContain('Offline alerts disabled');
    });

    it('cycles from warning to critical on click', () => {
      const onSetOfflineState = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'VM1',
              type: 'guest',
              disableConnectivity: false,
            }),
          ],
        },
        showOfflineAlertsColumn: true,
        onSetOfflineState,
      });
      render(() => <ResourceTable {...props} />);

      const warnButton = screen.getByText('Warn');
      fireEvent.click(warnButton);
      expect(onSetOfflineState).toHaveBeenCalledWith('vm-1', 'critical');
    });

    it('does not fire callback when globally disabled', () => {
      const onSetOfflineState = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'VM1',
              type: 'guest',
              disableConnectivity: false,
            }),
          ],
        },
        showOfflineAlertsColumn: true,
        onSetOfflineState,
        globalDisableFlag: () => true,
      });
      render(() => <ResourceTable {...props} />);

      const warnButton = screen.getByText('Warn');
      fireEvent.click(warnButton);
      expect(onSetOfflineState).not.toHaveBeenCalled();
    });
  });

  describe('backup/snapshot toggles', () => {
    it('calls onToggleBackup with resource id when backup badge is clicked', () => {
      const onToggleBackup = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'VM1',
              backup: { enabled: true },
            }),
          ],
        },
        columns: ['Backup'],
        onToggleBackup,
      });
      render(() => <ResourceTable {...props} />);

      const badge = screen.getByTestId('status-badge');
      fireEvent.click(badge);
      expect(onToggleBackup).toHaveBeenCalledWith('vm-1');
    });

    it('calls onToggleSnapshot with resource id when snapshot badge is clicked', () => {
      const onToggleSnapshot = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-2',
              name: 'VM2',
              snapshot: { enabled: false },
            }),
          ],
        },
        columns: ['Snapshot'],
        onToggleSnapshot,
      });
      render(() => <ResourceTable {...props} />);

      const badge = screen.getByTestId('status-badge');
      fireEvent.click(badge);
      expect(onToggleSnapshot).toHaveBeenCalledWith('vm-2');
    });

    it('shows enabled title for backup when enabled', () => {
      const onToggleBackup = vi.fn();
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [
            makeResource({
              id: 'vm-1',
              name: 'VM1',
              backup: { enabled: true },
            }),
          ],
        },
        columns: ['Backup'],
        onToggleBackup,
      });
      render(() => <ResourceTable {...props} />);

      const badge = screen.getByTestId('status-badge');
      expect(badge.title).toContain('Backup alerts enabled');
    });
  });

  describe('slider for SLIDER_METRICS', () => {
    it('renders ThresholdSlider for cpu metric when editing', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1', name: 'VM1' })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ cpu: 80 }),
        columns: ['CPU %'],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.getByTestId('threshold-slider')).toBeInTheDocument();
    });

    it('does not render ThresholdSlider for non-slider metrics when editing', () => {
      const props = makeProps({
        resources: undefined,
        groupedResources: {
          node1: [makeResource({ id: 'vm-1', name: 'VM1', thresholds: { diskRead: 100 } })],
        },
        editingId: () => 'vm-1',
        editingThresholds: () => ({ diskRead: 100 }),
        columns: ['Disk R MB/s'],
      });
      render(() => <ResourceTable {...props} />);

      expect(screen.queryByTestId('threshold-slider')).not.toBeInTheDocument();
    });
  });
});
