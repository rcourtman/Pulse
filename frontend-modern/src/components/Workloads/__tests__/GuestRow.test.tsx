import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { WorkloadGuest } from '@/types/workloads';
import type { Memory, Disk } from '@/types/api';

// ── Hoisted mocks ──────────────────────────────────────────────────────

const { isMobileMock } = vi.hoisted(() => {
  const isMobileMock = vi.fn(() => false);
  return { isMobileMock };
});

// ── Module mocks ───────────────────────────────────────────────────────

const mockNavigate = vi.fn();
const updateButtonSpy = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: isMobileMock,
  }),
}));

vi.mock('@/hooks/useTooltip', () => ({
  useTooltip: () => ({
    onMouseEnter: vi.fn(),
    onMouseLeave: vi.fn(),
    show: () => false,
    pos: () => ({ x: 0, y: 0 }),
  }),
}));

vi.mock('@/components/shared/TooltipPortal', () => ({
  TooltipPortal: () => null,
}));

vi.mock('@/hooks/useAnomalies', () => ({
  useAnomalyForMetric: () => () => null,
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => 'active',
    config: () => null,
    isLoading: () => false,
    activeAlerts: () => ({}),
    lastError: () => null,
    isPastObservationWindow: () => true,
    getBackupThresholds: () => ({ staleHours: 48, criticalHours: 168 }),
    getTemperatureThreshold: () => null,
    getMetricThresholds: () => ({ warning: 70, critical: 85 }),
    refreshConfig: vi.fn(),
    refreshActiveAlerts: vi.fn(),
    activate: vi.fn(),
    deactivate: vi.fn(),
    snooze: vi.fn(),
  }),
}));

vi.mock('@/components/shared/StatusDot', () => ({
  StatusDot: (props: { variant: string; title: string }) => (
    <span data-testid="status-dot" data-variant={props.variant} title={props.title} />
  ),
}));

vi.mock('@/components/Workloads/EnhancedCPUBar', () => ({
  EnhancedCPUBar: (props: { usage: number; cores?: number }) => (
    <div data-testid="cpu-bar" data-usage={props.usage} data-cores={props.cores} />
  ),
}));

vi.mock('../StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="disk-bar" />,
}));

vi.mock('../StackedMemoryBar', () => ({
  StackedMemoryBar: (props: {
    used: number;
    total: number;
    unavailable?: boolean;
    comparisonTotalLabel?: string;
    tooltipTitle?: string;
  }) => (
    <div
      data-testid="memory-bar"
      data-used={props.used}
      data-total={props.total}
      data-unavailable={props.unavailable}
      data-comparison-total-label={props.comparisonTotalLabel}
      data-tooltip-title={props.tooltipTitle}
    />
  ),
}));

vi.mock('@/components/shared/TagBadges', () => ({
  TagBadges: (props: { tags: string[]; sourceInstance?: string }) => (
    <div
      data-testid="tag-badges"
      data-count={props.tags.length}
      data-source-instance={props.sourceInstance}
    />
  ),
}));

vi.mock('@/components/shared/ContainerUpdateBadge', () => ({
  UpdateButton: (props: { agentId: string; containerId: string; containerName: string }) => {
    updateButtonSpy(props);
    return <div data-testid="update-button" />;
  },
}));

vi.mock('@/components/shared/workloadTypeBadges', () => ({
  getWorkloadTypeBadge: (type: string, opts?: { label?: string; title?: string }) => ({
    className: `badge-${type}`,
    label: opts?.label || type.toUpperCase(),
    title: opts?.title || type,
  }),
}));

vi.mock('../workloadTopology', () => ({
  getWorkloadAlertResourceIdCandidates: (guest: WorkloadGuest) => [guest.id],
  getWorkloadAlertThresholdScope: (guest: WorkloadGuest) =>
    guest.workloadType === 'app-container' ? 'docker' : 'guest',
  getWorkloadDockerHostId: (guest: WorkloadGuest) => guest.dockerHostId || '',
}));

// After mocks, import
import { GuestRow } from '../GuestRow';
import {
  GUEST_COLUMNS,
  VIEW_MODE_COLUMNS,
  getGuestColumnStyle,
  getGuestColumnWidthStyle,
  getWorkloadTableLayoutMode,
  getWorkloadTableLayoutModeForContainer,
  getWorkloadVisibleColumnsForLayout,
  resolveWorkloadColumnViewMode,
  type WorkloadIOEmphasis,
} from '../guestRowModel';

// ── Helpers ────────────────────────────────────────────────────────────

function makeMemory(overrides: Partial<Memory> = {}): Memory {
  return {
    total: 4294967296,
    used: 2147483648,
    free: 2147483648,
    usage: 50,
    ...overrides,
  };
}

function makeDisk(overrides: Partial<Disk> = {}): Disk {
  return {
    total: 10737418240,
    used: 5368709120,
    free: 5368709120,
    usage: 50,
    ...overrides,
  };
}

function makeGuest(overrides: Partial<WorkloadGuest> = {}): WorkloadGuest {
  return {
    id: 'inst1-node1-100',
    vmid: 100,
    name: 'test-vm',
    node: 'node1',
    instance: 'inst1',
    status: 'running',
    type: 'qemu',
    cpu: 0.25,
    cpus: 4,
    memory: makeMemory(),
    disk: makeDisk(),
    networkIn: 1000,
    networkOut: 2000,
    diskRead: 500,
    diskWrite: 600,
    uptime: 86400,
    template: false,
    lastBackup: 0,
    tags: null,
    lock: '',
    lastSeen: new Date().toISOString(),
    ...overrides,
  } as WorkloadGuest;
}

/** Wrap GuestRow in a <table> since it renders <tr> */
function renderGuestRow(props: Parameters<typeof GuestRow>[0]) {
  return render(() => (
    <table>
      <tbody>
        <GuestRow {...props} />
      </tbody>
    </table>
  ));
}

// ── Setup / Teardown ───────────────────────────────────────────────────

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.clearAllMocks();
  isMobileMock.mockReturnValue(false);
});

// ── Tests ──────────────────────────────────────────────────────────────

describe('GuestRow', () => {
  describe('rendering basics', () => {
    it('renders guest name', () => {
      renderGuestRow({ guest: makeGuest({ name: 'my-webserver' }) });
      expect(screen.getByText('my-webserver')).toBeTruthy();
    });

    it('renders a subtle nested workload cue when the row has drawer-only containers', () => {
      renderGuestRow({
        guest: makeGuest({ name: 'frigate', type: 'lxc', workloadType: 'system-container' }),
        nestedWorkloadContext: {
          label: 'Docker',
          count: 2,
        },
      });

      const cue = screen.getByTestId('nested-workload-cue');
      expect(cue).toHaveAttribute(
        'aria-label',
        '2 nested Docker containers. Open row for details.',
      );
      expect(cue).toHaveTextContent('2');
      expect(screen.queryByText('Nested Docker')).toBeNull();
    });

    it('does not repeat the in-guest agent install action in every row', () => {
      renderGuestRow({
        guest: makeGuest({ agentVersion: undefined, status: 'running' }),
        visibleColumnIds: ['name'],
      });

      expect(screen.queryByRole('link', { name: 'Add Pulse Agent for AI actions' })).toBeNull();
    });

    it('renders the status dot', () => {
      renderGuestRow({ guest: makeGuest({ status: 'running' }) });
      const dot = screen.getByTestId('status-dot');
      expect(dot).toBeTruthy();
    });

    it('renders CPU bar with correct usage percentage', () => {
      renderGuestRow({ guest: makeGuest({ cpu: 0.75 }) });
      const cpuBar = screen.getByTestId('cpu-bar');
      expect(cpuBar.dataset.usage).toBe('75');
    });

    it('renders unknown canonical telemetry as unavailable instead of reported zero', () => {
      renderGuestRow({
        guest: makeGuest({
          cpu: 0,
          networkIn: 0,
          networkOut: 0,
          diskRead: 0,
          diskWrite: 0,
          uptime: 0,
          telemetryAvailability: {
            cpu: false,
            memory: false,
            disk: false,
            networkIO: false,
            diskIO: false,
            uptime: false,
          },
        }),
        visibleColumnIds: ['name', 'cpu', 'memory', 'netIo', 'diskIo', 'uptime'],
      });

      expect(screen.queryByTestId('cpu-bar')).toBeNull();
      expect(screen.getByTestId('memory-bar')).toHaveAttribute('data-unavailable', 'true');
      expect(screen.queryByText('0 B/s')).toBeNull();
      expect(screen.queryByText('0s')).toBeNull();
      expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(4);
    });

    it('keeps API-reported zero telemetry visible as a real value', () => {
      renderGuestRow({
        guest: makeGuest({
          cpu: 0,
          networkIn: 0,
          networkOut: 0,
          diskRead: 0,
          diskWrite: 0,
          uptime: 0,
          telemetryAvailability: {
            cpu: true,
            memory: true,
            disk: true,
            networkIO: true,
            diskIO: true,
            uptime: true,
          },
        }),
        visibleColumnIds: ['name', 'cpu', 'netIo', 'diskIo', 'uptime'],
      });

      expect(screen.getByTestId('cpu-bar')).toHaveAttribute('data-usage', '0');
      expect(screen.getAllByText('0 B/s')).toHaveLength(4);
      expect(screen.getByText('0s')).toBeTruthy();
    });

    it.each([1, 4, 8])(
      'does not renormalize authoritative guest CPU by %i allocated cores',
      (cpus) => {
        renderGuestRow({ guest: makeGuest({ cpu: 0.0058, cpus }) });
        const cpuBar = screen.getByTestId('cpu-bar');
        expect(Number(cpuBar.dataset.usage)).toBeCloseTo(0.58);
        expect(cpuBar.dataset.cores).toBe(String(cpus));
      },
    );

    it('renders memory bar', () => {
      renderGuestRow({ guest: makeGuest() });
      expect(screen.getByTestId('memory-bar')).toBeTruthy();
    });

    it('renders raw-byte memory history against the parent host total when requested', () => {
      const getGuestMetricSeries = vi.fn(
        (_guest: WorkloadGuest, _metric: string, _options?: unknown) => [
          {
            id: 'memory',
            label: 'Host memory share',
            color: '#f59e0b',
            points: [
              { timestamp: 1, value: 10 },
              { timestamp: 2, value: 12.5 },
            ],
          },
        ],
      );
      renderGuestRow({
        guest: makeGuest({ memory: makeMemory({ used: 2 * 1024 ** 3, total: 4 * 1024 ** 3 }) }),
        visibleColumnIds: ['name', 'memory'],
        memoryDisplayBasis: 'host',
        parentMemoryTotal: 16 * 1024 ** 3,
        parentNodeName: 'pve-01',
        metricDisplayMode: 'sparklines',
        metricHistory: {
          getGuestMetricSeries,
          getNodeMetricSeries: () => [],
        },
      });

      expect(screen.getByTestId('metric-mini-sparkline')).toHaveAttribute(
        'title',
        'test-vm host memory share history',
      );
      expect(screen.getByText('13%')).toBeTruthy();
      expect(screen.queryByTestId('memory-bar')).toBeNull();
      expect(getGuestMetricSeries).toHaveBeenCalledWith(expect.any(Object), 'memory', {
        memoryDisplayBasis: 'host',
        parentMemoryTotal: 16 * 1024 ** 3,
      });
    });

    it('marks host-relative memory unavailable when the parent total is missing', () => {
      renderGuestRow({
        guest: makeGuest(),
        visibleColumnIds: ['name', 'memory'],
        memoryDisplayBasis: 'host',
      });

      expect(screen.getByTestId('memory-bar')).toHaveAttribute('data-unavailable', 'true');
    });

    it('does not inflate a sub-one-percent host memory share', () => {
      renderGuestRow({
        guest: makeGuest({
          memory: makeMemory({ used: 128 * 1024 ** 2, total: 4 * 1024 ** 3 }),
        }),
        visibleColumnIds: ['name', 'memory'],
        memoryDisplayBasis: 'host',
        parentMemoryTotal: 16 * 1024 ** 3,
        metricDisplayMode: 'sparklines',
      });

      expect(screen.getByText('1%')).toBeTruthy();
      expect(screen.queryByText('78%')).toBeNull();
    });

    it('renders disk bar when disk usage is available', () => {
      renderGuestRow({ guest: makeGuest() });
      expect(screen.getByTestId('disk-bar')).toBeTruthy();
    });

    it('renders metric sparklines instead of bars when the display mode is sparklines', () => {
      renderGuestRow({
        guest: makeGuest({ name: 'spark-vm' }),
        visibleColumnIds: ['name', 'cpu', 'memory', 'disk'],
        metricDisplayMode: 'sparklines',
        metricHistory: {
          getGuestMetricSeries: (_guest, metric) => [
            {
              id: metric,
              label: metric,
              color: '#8b5cf6',
              points: [
                { timestamp: 1, value: 10 },
                { timestamp: 2, value: 25 },
              ],
            },
          ],
          getNodeMetricSeries: () => [],
        },
      });

      expect(screen.getAllByTestId('metric-mini-sparkline')).toHaveLength(3);
      expect(screen.queryByTestId('cpu-bar')).toBeNull();
      expect(screen.queryByTestId('memory-bar')).toBeNull();
      expect(screen.queryByTestId('disk-bar')).toBeNull();
    });

    it('keeps bars at rest and synchronizes the row history lens cursor', () => {
      renderGuestRow({
        guest: makeGuest({ name: 'layered-vm' }),
        visibleColumnIds: ['name', 'cpu', 'memory', 'disk'],
        metricDisplayMode: 'bars',
        metricHistory: {
          getGuestMetricSeries: (_guest, metric) => [
            {
              id: metric,
              label: metric,
              color: '#8b5cf6',
              points: [
                { timestamp: 1, value: 10 },
                { timestamp: 2, value: 25 },
              ],
            },
          ],
          getNodeMetricSeries: () => [],
        },
      });

      expect(screen.getByTestId('cpu-bar')).toBeInTheDocument();
      expect(screen.getByTestId('memory-bar')).toBeInTheDocument();
      expect(screen.getByTestId('disk-bar')).toBeInTheDocument();
      expect(screen.queryByTestId('metric-mini-sparkline')).not.toBeInTheDocument();

      const row = screen.getByText('layered-vm').closest('tr')!;
      fireEvent.pointerEnter(row, { pointerType: 'mouse' });

      expect(row).toHaveAttribute('data-history-lens-active', 'true');
      expect(screen.queryByTestId('cpu-bar')).not.toBeInTheDocument();
      expect(screen.queryByTestId('memory-bar')).not.toBeInTheDocument();
      expect(screen.queryByTestId('disk-bar')).not.toBeInTheDocument();

      const lensCharts = screen.getAllByTestId('metric-mini-sparkline');
      expect(lensCharts).toHaveLength(3);
      expect(lensCharts[0].parentElement).toHaveClass('motion-reduce:animate-none');

      const firstSvg = lensCharts[0].querySelector('svg') as SVGSVGElement;
      firstSvg.getBoundingClientRect = () =>
        ({
          bottom: 18,
          height: 18,
          left: 0,
          right: 96,
          top: 0,
          width: 96,
          x: 0,
          y: 0,
          toJSON: () => ({}),
        }) as DOMRect;

      fireEvent.mouseMove(firstSvg, { clientX: 48, clientY: 8 });
      expect(lensCharts.every((chart) => chart.querySelector('[data-metric-history-cursor]'))).toBe(
        true,
      );

      fireEvent.mouseLeave(firstSvg);
      expect(
        lensCharts.every((chart) => chart.querySelector('[data-metric-history-cursor]') === null),
      ).toBe(true);

      fireEvent.pointerLeave(row, { pointerType: 'mouse' });
      expect(row).not.toHaveAttribute('data-history-lens-active');
      expect(screen.getByTestId('cpu-bar')).toBeInTheDocument();
      expect(screen.getByTestId('memory-bar')).toBeInTheDocument();
      expect(screen.getByTestId('disk-bar')).toBeInTheDocument();
      expect(screen.queryByTestId('metric-mini-sparkline')).not.toBeInTheDocument();
    });

    it('keeps the live bars visible until warmed history is ready', () => {
      const [historyReady, setHistoryReady] = createSignal(false);
      renderGuestRow({
        guest: makeGuest({ name: 'warming-vm' }),
        visibleColumnIds: ['name', 'cpu', 'memory', 'disk'],
        metricDisplayMode: 'bars',
        metricHistory: {
          hasGuestHistory: historyReady,
          getGuestMetricSeries: (_guest, metric) => [
            {
              id: metric,
              label: metric,
              color: '#8b5cf6',
              points: [
                { timestamp: 1, value: 10 },
                { timestamp: 2, value: 25 },
              ],
            },
          ],
          getNodeMetricSeries: () => [],
        },
      });

      const row = screen.getByText('warming-vm').closest('tr')!;
      fireEvent.pointerEnter(row, { pointerType: 'mouse' });

      expect(row).toHaveAttribute('data-history-lens-active', 'true');
      expect(row).toHaveAttribute('data-history-lens-pending', 'true');
      expect(screen.getByTestId('cpu-bar')).toBeInTheDocument();
      expect(screen.getByTestId('memory-bar')).toBeInTheDocument();
      expect(screen.getByTestId('disk-bar')).toBeInTheDocument();
      expect(screen.queryByTestId('metric-mini-sparkline')).not.toBeInTheDocument();

      setHistoryReady(true);

      expect(row).not.toHaveAttribute('data-history-lens-pending');
      expect(screen.getAllByTestId('metric-mini-sparkline')).toHaveLength(3);
      expect(screen.queryByTestId('cpu-bar')).not.toBeInTheDocument();
      expect(screen.queryByTestId('memory-bar')).not.toBeInTheDocument();
      expect(screen.queryByTestId('disk-bar')).not.toBeInTheDocument();
    });

    it('keeps Net I/O and Disk I/O live labels out of sparkline table cells', () => {
      renderGuestRow({
        guest: makeGuest({ name: 'spark-io-vm', networkIn: 1024, networkOut: 2048 }),
        visibleColumnIds: ['name', 'netIo', 'diskIo'],
        metricDisplayMode: 'sparklines',
        metricHistory: {
          getGuestMetricSeries: (_guest, metric) => [
            {
              id: metric,
              label: metric,
              color: '#8b5cf6',
              points: [
                { timestamp: 1, value: 10 },
                { timestamp: 2, value: 25 },
              ],
            },
          ],
          getNodeMetricSeries: () => [],
        },
      });

      const sparklines = screen.getAllByTestId('metric-mini-sparkline');
      expect(sparklines).toHaveLength(2);
      expect(sparklines.map((sparkline) => sparkline.dataset.valueLabelMode)).toEqual([
        'tooltip',
        'tooltip',
      ]);
      expect(screen.queryByText('1.00 KB/s / 2.00 KB/s')).toBeNull();
    });

    it('shows dash when disk data is unavailable', () => {
      renderGuestRow({
        guest: makeGuest({ disk: { total: 0, used: 0, free: 0, usage: 0 } }),
      });
      expect(screen.queryByTestId('disk-bar')).toBeNull();
      // Fallback is a hyphen "-"
      expect(screen.getByText('-')).toBeTruthy();
    });

    it('renders data-guest-id attribute', () => {
      const { container } = renderGuestRow({ guest: makeGuest() });
      const tr = container.querySelector('tr');
      expect(tr?.dataset.guestId).toBeTruthy();
    });
  });

  describe('displayId logic', () => {
    it('centers the shared info identifier beside the workload type', () => {
      const { container } = renderGuestRow({
        guest: makeGuest({ vmid: 123, displayId: undefined }),
        visibleColumnIds: ['name', 'info'],
      });

      expect(container.querySelector('[data-workload-col="info"]')).toHaveClass('text-center');
    });

    it('shows vmid when displayId is not set', () => {
      const { container } = renderGuestRow({
        guest: makeGuest({ vmid: 123, displayId: undefined }),
        visibleColumnIds: ['name', 'vmid'],
      });
      expect(screen.getByText('123')).toBeTruthy();
      expect(container.querySelector('[data-workload-col="vmid"]')).toHaveClass('text-center');
    });

    it('shows displayId when set', () => {
      renderGuestRow({
        guest: makeGuest({ vmid: 100, displayId: 'custom-id' }),
        visibleColumnIds: ['name', 'vmid'],
      });
      expect(screen.getByText('custom-id')).toBeTruthy();
    });

    it('shows dash for vmid column when no id available', () => {
      renderGuestRow({
        guest: makeGuest({ vmid: 0, displayId: '' }),
        visibleColumnIds: ['name', 'vmid'],
      });
      // The fallback dash renders
      const cells = screen.getAllByText('—');
      expect(cells.length).toBeGreaterThan(0);
    });
  });

  describe('column visibility', () => {
    it('shows all columns when visibleColumnIds is undefined', () => {
      renderGuestRow({ guest: makeGuest() });
      // CPU, memory, disk bars should all render
      expect(screen.getByTestId('cpu-bar')).toBeTruthy();
      expect(screen.getByTestId('memory-bar')).toBeTruthy();
      expect(screen.getByTestId('disk-bar')).toBeTruthy();
    });

    it('hides CPU when not in visibleColumnIds', () => {
      renderGuestRow({
        guest: makeGuest(),
        visibleColumnIds: ['name', 'memory', 'disk'],
      });
      expect(screen.queryByTestId('cpu-bar')).toBeNull();
      expect(screen.getByTestId('memory-bar')).toBeTruthy();
    });

    it('hides memory when not in visibleColumnIds', () => {
      renderGuestRow({
        guest: makeGuest(),
        visibleColumnIds: ['name', 'cpu', 'disk'],
      });
      expect(screen.getByTestId('cpu-bar')).toBeTruthy();
      expect(screen.queryByTestId('memory-bar')).toBeNull();
    });

    it('hides tags when not in visibleColumnIds', () => {
      renderGuestRow({
        guest: makeGuest({ tags: ['prod', 'web'] }),
        visibleColumnIds: ['name', 'cpu'],
      });
      expect(screen.queryByTestId('tag-badges')).toBeNull();
    });

    it('shows tags when in visibleColumnIds', () => {
      renderGuestRow({
        guest: makeGuest({ instance: 'pve-a', tags: ['prod', 'web'] }),
        visibleColumnIds: ['name', 'tags'],
      });
      const badges = screen.getByTestId('tag-badges');
      expect(badges.dataset.count).toBe('2');
      expect(badges.dataset.sourceInstance).toBe('pve-a');
    });
  });

  describe('lock label', () => {
    it('shows lock label when guest is locked', () => {
      renderGuestRow({ guest: makeGuest({ lock: 'migrate' }) });
      expect(screen.getByText(/Lock:.*migrate/)).toBeTruthy();
    });

    it('does not show lock label when not locked', () => {
      renderGuestRow({ guest: makeGuest({ lock: '' }) });
      expect(screen.queryByText('Lock:')).toBeNull();
    });
  });

  describe('row classes and alert styling', () => {
    it('applies opacity-60 when guest is not running', () => {
      const { container } = renderGuestRow({
        guest: makeGuest({ status: 'stopped' }),
      });
      const tr = container.querySelector('tr');
      expect(tr?.className).toContain('opacity-60');
    });

    it('does not apply opacity when guest is running', () => {
      const { container } = renderGuestRow({
        guest: makeGuest({ status: 'running' }),
      });
      const tr = container.querySelector('tr');
      expect(tr?.className).not.toContain('opacity-60');
    });

    it('applies expanded styling when isExpanded is true', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isExpanded: true,
      });
      const tr = container.querySelector('tr');
      expect(tr?.className).toContain('bg-blue-50');
    });

    it('routes summary-linked emphasis through the shared active-row marker instead of lane-local fills', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isSummaryHighlighted: true,
      });
      const tr = container.querySelector('tr');
      expect(tr?.getAttribute('data-summary-row-active')).toBe('true');
      expect(tr?.className).not.toContain('bg-sky-50');
      expect(tr?.className).not.toContain('ring-sky-400/25');
    });

    it('applies critical alert background for unacknowledged critical alerts', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        alertStyles: {
          rowClass: '',
          indicatorClass: '',
          badgeClass: '',
          hasAlert: true,
          alertCount: 1,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
        },
      });
      const tr = container.querySelector('tr');
      expect(tr?.className).toContain('bg-red-50');
    });

    it('applies warning alert background for unacknowledged warning alerts', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        alertStyles: {
          rowClass: '',
          indicatorClass: '',
          badgeClass: '',
          hasAlert: true,
          alertCount: 1,
          severity: 'warning',
          hasUnacknowledgedAlert: true,
        },
      });
      const tr = container.querySelector('tr');
      expect(tr?.className).toContain('bg-yellow-50');
    });

    it('marks unacknowledged critical alerts with the canonical alert accent tone', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        alertStyles: {
          rowClass: '',
          indicatorClass: '',
          badgeClass: '',
          hasAlert: true,
          alertCount: 1,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
        },
      });
      const tr = container.querySelector('tr');
      expect(tr).toHaveAttribute('data-workload-alert-accent', 'critical');
      expect(tr?.getAttribute('style')).toBeNull();
    });

    it('marks acknowledged-only alerts with the canonical alert accent tone', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        alertStyles: {
          rowClass: '',
          indicatorClass: '',
          badgeClass: '',
          hasAlert: true,
          alertCount: 1,
          severity: null,
          hasAcknowledgedOnlyAlert: true,
        },
      });
      const tr = container.querySelector('tr');
      expect(tr).toHaveAttribute('data-workload-alert-accent', 'acknowledged');
      expect(tr?.getAttribute('style')).toBeNull();
    });
  });

  describe('click and hover handlers', () => {
    it('calls onClick when row is clicked', () => {
      const onClick = vi.fn();
      const { container } = renderGuestRow({
        guest: makeGuest(),
        onClick,
      });
      const tr = container.querySelector('tr')!;
      fireEvent.click(tr);
      expect(onClick).toHaveBeenCalledOnce();
    });

    it('calls onHoverChange with canonical guestId on fine-pointer preview', () => {
      const onHoverChange = vi.fn();
      const { container } = renderGuestRow({
        guest: makeGuest({ id: 'inst1-node1-100' }),
        onHoverChange,
      });
      const tr = container.querySelector('tr')!;
      const expectedId = tr.dataset.guestId!;
      fireEvent.pointerEnter(tr, { pointerType: 'mouse' });
      expect(onHoverChange).toHaveBeenCalledWith(expectedId);
    });

    it('calls onHoverChange with null when preview leaves the row', () => {
      const onHoverChange = vi.fn();
      const { container } = renderGuestRow({
        guest: makeGuest(),
        onHoverChange,
      });
      const tr = container.querySelector('tr')!;
      fireEvent.pointerLeave(tr, { pointerType: 'mouse' });
      expect(onHoverChange).toHaveBeenCalledWith(null);
    });

    it('activates and clears the history lens through keyboard focus', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
      });
      const tr = container.querySelector('tr')!;

      fireEvent.focusIn(tr);
      expect(tr).toHaveAttribute('data-history-lens-active', 'true');

      fireEvent.focusOut(tr, { relatedTarget: document.body });
      expect(tr).not.toHaveAttribute('data-history-lens-active');
    });

    it('does not activate the history lens for touch pointer entry', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
      });
      const tr = container.querySelector('tr')!;

      const touchPointerEnter = new MouseEvent('pointerover', { bubbles: true });
      Object.defineProperty(touchPointerEnter, 'pointerType', { value: 'touch' });
      fireEvent(tr, touchPointerEnter);
      expect(tr).not.toHaveAttribute('data-history-lens-active');
    });

    it('keeps the original bars and hover callbacks inactive in details mode', () => {
      const onHoverChange = vi.fn();
      const { container } = renderGuestRow({
        guest: makeGuest(),
        metricHoverMode: 'details',
        onHoverChange,
      });
      const tr = container.querySelector('tr')!;

      fireEvent.pointerEnter(tr, { pointerType: 'mouse' });

      expect(tr).not.toHaveAttribute('data-history-lens-active');
      expect(screen.getByTestId('cpu-bar')).toBeInTheDocument();
      expect(screen.getByTestId('memory-bar')).toBeInTheDocument();
      expect(screen.getByTestId('disk-bar')).toBeInTheDocument();
      expect(screen.queryByTestId('metric-mini-sparkline')).not.toBeInTheDocument();
      expect(onHoverChange).not.toHaveBeenCalled();
    });

    it('toggles the row from the shared disclosure button keyboard path', () => {
      const onClick = vi.fn();
      const onHoverChange = vi.fn();
      const guest = makeGuest();
      renderGuestRow({
        guest,
        onClick,
        onHoverChange,
      });
      const toggleButton = screen.getByRole('button', {
        name: `Expand ${guest.name}`,
      });

      fireEvent.focusIn(toggleButton);
      expect(onHoverChange).toHaveBeenCalled();

      fireEvent.click(toggleButton);
      expect(onClick).toHaveBeenCalledOnce();
    });
  });

  describe('uptime display', () => {
    it('shows uptime for running guests', () => {
      renderGuestRow({
        guest: makeGuest({ status: 'running', uptime: 86400 }),
        visibleColumnIds: ['name', 'uptime'],
      });
      // formatUptime(86400) = "1d 0h"
      expect(screen.getByText('1d 0h')).toBeTruthy();
    });

    it('shows dash for stopped guests', () => {
      renderGuestRow({
        guest: makeGuest({ status: 'stopped', uptime: 0 }),
        visibleColumnIds: ['name', 'uptime'],
      });
      expect(screen.getAllByText('—').length).toBeGreaterThan(0);
    });
  });

  describe('node column', () => {
    it('renders node name as plain text in the platform-first layout', () => {
      renderGuestRow({
        guest: makeGuest({ node: 'pve1' }),
        visibleColumnIds: ['name', 'node'],
      });
      const nodeLabel = screen.getByText('pve1');
      // The legacy cross-jump to /infrastructure?source=...&query=<node> was
      // dropped; the node name is now non-interactive context inside the
      // owning platform page.
      expect(nodeLabel.tagName).toBe('SPAN');
    });

    it('does not navigate when the node label is clicked', () => {
      renderGuestRow({
        guest: makeGuest({ node: 'pve1' }),
        visibleColumnIds: ['name', 'node'],
      });
      const nodeLabel = screen.getByText('pve1');
      fireEvent.click(nodeLabel);
      expect(mockNavigate).not.toHaveBeenCalled();
    });
  });

  describe('app-container workload type', () => {
    it('shows image column for app-container guests', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'ghcr.io/library/nginx:latest',
        }),
        visibleColumnIds: ['name', 'image'],
      });
      const image = screen.getByText('nginx:latest');
      expect(image).toBeTruthy();
      expect(image.getAttribute('title')).toBe('ghcr.io/library/nginx:latest');
      expect(image.closest('td')?.querySelector('div')?.className).toContain('justify-start');
    });

    it('shows update button for app-container guests', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          platformType: 'docker',
          dockerHostId: 'host-1',
        }),
        visibleColumnIds: ['name', 'update'],
      });
      expect(screen.getByTestId('update-button')).toBeTruthy();
    });

    it('keeps the image column scoped to the image name', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'nginx:latest',
          containerRuntime: 'docker',
        }),
        visibleColumnIds: ['name', 'image'],
      });
      expect(screen.getByText('nginx:latest')).toBeTruthy();
      expect(screen.queryByText('Docker')).toBeNull();
    });

    it('renders Docker runtime chip in the runtime column for Docker-managed app-containers', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'nginx:latest',
          containerRuntime: 'docker',
        }),
        visibleColumnIds: ['name', 'runtime', 'image'],
      });
      const runtimeBadge = screen.getByText('Docker');
      expect(runtimeBadge).toBeTruthy();
      expect(runtimeBadge.className).toContain('bg-sky-100');
    });

    it('renders Podman runtime chip in the runtime column for Podman-managed app-containers', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'nginx:latest',
          containerRuntime: 'podman',
        }),
        visibleColumnIds: ['name', 'runtime', 'image'],
      });
      const runtimeBadge = screen.getByText('Podman');
      expect(runtimeBadge).toBeTruthy();
      expect(runtimeBadge.className).toContain('bg-violet-100');
    });

    it('does not render a runtime chip when runtime and platform are both unknown', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'nginx:latest',
        }),
        visibleColumnIds: ['name', 'runtime', 'image'],
      });
      expect(screen.queryByText('Docker')).toBeNull();
      expect(screen.queryByText('Podman')).toBeNull();
    });

    it('does not treat the owning platform as container runtime metadata', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'nginx:latest',
          platformType: 'truenas',
        }),
        visibleColumnIds: ['name', 'runtime', 'image'],
      });
      expect(screen.queryByText('TrueNAS')).toBeNull();
    });
  });

  describe('pod workload type', () => {
    it('shows namespace column for pod guests', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'pod',
          workloadType: 'pod',
          namespace: 'default',
        }),
        visibleColumnIds: ['name', 'namespace'],
      });
      expect(screen.getByText('default')).toBeTruthy();
    });

    it('shows context column for pod guests', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'production-cluster',
        }),
        visibleColumnIds: ['name', 'context'],
      });
      expect(screen.getByText('production-cluster')).toBeTruthy();
    });
  });

  describe('grouped view indentation', () => {
    it('uses grouped indent class when isGroupedView is true', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isGroupedView: true,
      });
      const firstTd = container.querySelector('td');
      expect(firstTd?.className).toContain('pl-1');
      expect(firstTd?.className).toContain('sm:pl-5');
    });

    it('uses default indent class when isGroupedView is false', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isGroupedView: false,
      });
      const firstTd = container.querySelector('td');
      expect(firstTd?.className).toContain('pl-1');
      expect(firstTd?.className).toContain('sm:pl-3');
    });

    it('visually removes the redundant disclosure control in compact layouts', () => {
      const guest = makeGuest();
      renderGuestRow({
        guest,
        isGroupedView: true,
        workloadTableLayoutMode: 'phone',
        onClick: vi.fn(),
      });
      const disclosureButton = screen.getByRole('button', {
        name: `Expand ${guest.name}`,
      });

      expect(disclosureButton.className).toContain('sr-only');
      expect(disclosureButton.className).toContain('focus:not-sr-only');
      expect(disclosureButton.className).toContain('sm:not-sr-only');
    });

    it('keeps the visible disclosure control in desktop layouts', () => {
      const guest = makeGuest();
      renderGuestRow({ guest, onClick: vi.fn() });
      const disclosureButton = screen.getByRole('button', {
        name: `Expand ${guest.name}`,
      });

      expect(disclosureButton.className).not.toContain('sr-only');
    });
  });

  describe('custom URL on the name cell', () => {
    it('renders an external link beside the workload name when customUrl is set', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        customUrl: 'https://example.com',
        visibleColumnIds: ['name', 'link'],
      });
      const link = container.querySelector('a[href="https://example.com"]');
      expect(link).toBeTruthy();
      expect(link?.getAttribute('target')).toBe('_blank');
      expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
      expect(link?.getAttribute('aria-label')).toBe('Open web interface for test-vm');
      expect(link?.textContent).toBe('');
      expect(link?.closest('td')?.textContent).toContain('test-vm');
      expect(screen.getByText('test-vm').closest('a')).toBeNull();
      expect(container.querySelector('td[data-workload-col="link"]')).toBeNull();
    });

    it.each(['narrow', 'phone'] as const)(
      'prioritizes the workload name over the adjacent web link in %s rows',
      (workloadTableLayoutMode) => {
        const { container } = renderGuestRow({
          guest: makeGuest(),
          customUrl: 'https://example.com',
          visibleColumnIds: ['name'],
          workloadTableLayoutMode,
        });

        expect(container.querySelector('a[href="https://example.com"]')?.parentElement).toHaveClass(
          '[&>a]:hidden',
        );
      },
    );

    it('does not render a trailing infrastructure fallback link when no customUrl is set', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        visibleColumnIds: ['name', 'link'],
      });
      expect(container.querySelector('a[href="/infrastructure/node1"]')).toBeNull();
      expect(container.querySelector('td[data-workload-col="link"]')).toBeNull();
    });
  });

  describe('disk usage edge cases', () => {
    it('shows dash when disk usage is -1 (unsupported)', () => {
      renderGuestRow({
        guest: makeGuest({
          disk: { total: 1000, used: 0, free: 1000, usage: -1 },
        }),
        visibleColumnIds: ['name', 'disk'],
      });
      expect(screen.queryByTestId('disk-bar')).toBeNull();
      // Fallback is a hyphen "-"
      expect(screen.getByText('-')).toBeTruthy();
    });

    it('shows disk bar when disk data is valid', () => {
      renderGuestRow({
        guest: makeGuest({
          disk: { total: 1000, used: 500, free: 500, usage: 50 },
        }),
        visibleColumnIds: ['name', 'disk'],
      });
      expect(screen.getByTestId('disk-bar')).toBeTruthy();
    });
  });

  describe('memory balloon and swap tooltip', () => {
    it('sets title attribute when balloon differs from total', () => {
      renderGuestRow({
        guest: makeGuest({
          memory: makeMemory({ balloon: 2147483648, total: 4294967296 }),
        }),
        visibleColumnIds: ['name', 'memory'],
      });
      const memoryContainer = screen.getByTestId('memory-bar')?.parentElement;
      const titleAttr = memoryContainer?.getAttribute('title');
      expect(titleAttr).toContain('Balloon');
    });

    it('includes swap info in title when swap is present', () => {
      renderGuestRow({
        guest: makeGuest({
          memory: makeMemory({
            swapTotal: 1073741824,
            swapUsed: 536870912,
          }),
        }),
        visibleColumnIds: ['name', 'memory'],
      });
      const memoryContainer = screen.getByTestId('memory-bar')?.parentElement;
      const titleAttr = memoryContainer?.getAttribute('title');
      expect(titleAttr).toContain('Swap');
    });

    it('has no extra title when no balloon or swap', () => {
      renderGuestRow({
        guest: makeGuest({
          memory: makeMemory({ balloon: undefined, swapTotal: undefined }),
        }),
        visibleColumnIds: ['name', 'memory'],
      });
      const memoryContainer = screen.getByTestId('memory-bar')?.parentElement;
      // No title set or undefined
      const titleAttr = memoryContainer?.getAttribute('title');
      expect(titleAttr).toBeFalsy();
    });
  });

  describe('expand chevron rotation', () => {
    it('rotates chevron when expanded', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isExpanded: true,
        onClick: vi.fn(),
      });
      const chevronWrapper = container.querySelector('.rotate-90');
      expect(chevronWrapper).toBeTruthy();
    });

    it('does not rotate chevron when collapsed', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isExpanded: false,
        onClick: vi.fn(),
      });
      const chevronWrapper = container.querySelector('.rotate-90');
      expect(chevronWrapper).toBeNull();
    });
  });
});

describe('GUEST_COLUMNS', () => {
  it('has the expected number of columns', () => {
    // name, runtime, type, info, vmid, cpu, memory, disk, ip, uptime, node,
    // image, namespace, context, aiContext, backup, tags, os, netIo, diskIo, update
    expect(GUEST_COLUMNS.length).toBe(22);
  });

  it('has name as the first column', () => {
    expect(GUEST_COLUMNS[0].id).toBe('name');
  });

  it('does not expose a trailing link column', () => {
    expect(GUEST_COLUMNS.map((column) => column.id)).not.toContain('link');
    expect(GUEST_COLUMNS[GUEST_COLUMNS.length - 1].id).toBe('update');
  });

  it('keeps Docker I/O and update headers aligned with the rendered row cells', () => {
    const columnIds = GUEST_COLUMNS.map((column) => column.id);
    expect(columnIds.slice(0, 3)).toEqual(['name', 'availability', 'runtime']);
    const dockerTailStart = columnIds.indexOf('netIo');
    expect(columnIds.slice(dockerTailStart)).toEqual(['netIo', 'diskIo', 'update']);
  });

  it('sorts the mixed Info/ID column by its rendered value', () => {
    expect(GUEST_COLUMNS.find((column) => column.id === 'info')?.sortKey).toBe('info');
    expect(GUEST_COLUMNS.find((column) => column.id === 'vmid')?.sortKey).toBe('vmid');
  });

  it('marks toggleable columns correctly', () => {
    const toggleable = GUEST_COLUMNS.filter((c) => c.toggleable);
    const toggleableIds = toggleable.map((c) => c.id);
    expect(toggleableIds).toContain('type');
    expect(toggleableIds).toContain('runtime');
    expect(toggleableIds).toContain('ip');
    expect(toggleableIds).toContain('uptime');
    expect(toggleableIds).toContain('node');
    expect(toggleableIds).toContain('aiContext');
    expect(toggleableIds).toContain('backup');
    expect(toggleableIds).toContain('tags');
    expect(toggleableIds).toContain('os');
    expect(toggleableIds).toContain('netIo');
    expect(toggleableIds).toContain('diskIo');
  });

  it('keeps the Type and I/O column width contract aligned with the desktop table layout', () => {
    const nameColumn = GUEST_COLUMNS.find((column) => column.id === 'name');
    const runtimeColumn = GUEST_COLUMNS.find((column) => column.id === 'runtime');
    const typeColumn = GUEST_COLUMNS.find((column) => column.id === 'type');
    const aiContextColumn = GUEST_COLUMNS.find((column) => column.id === 'aiContext');
    const netIoColumn = GUEST_COLUMNS.find((column) => column.id === 'netIo');
    const diskIoColumn = GUEST_COLUMNS.find((column) => column.id === 'diskIo');
    const updateColumn = GUEST_COLUMNS.find((column) => column.id === 'update');

    expect(nameColumn?.width).toBe('200px');
    expect(nameColumn?.minWidth).toBe('180px');
    expect(nameColumn?.maxWidth).toBe('220px');
    expect(runtimeColumn?.width).toBe('104px');
    expect(runtimeColumn?.minWidth).toBe('96px');
    expect(runtimeColumn?.maxWidth).toBe('112px');
    expect(typeColumn?.width).toBe('60px');
    expect(aiContextColumn?.defaultHidden).toBe(true);
    expect(aiContextColumn?.width).toBe('92px');
    expect(netIoColumn?.width).toBe('170px');
    expect(netIoColumn?.minWidth).toBe('170px');
    expect(diskIoColumn?.width).toBe('170px');
    expect(diskIoColumn?.minWidth).toBe('170px');
    expect(updateColumn?.width).toBe('86px');
  });

  it('derives mobile overrides from the canonical guest column model', () => {
    expect(getGuestColumnStyle('name', true)).toEqual({
      width: '30%',
      'max-width': '30%',
    });
    expect(getGuestColumnStyle('cpu', true)).toEqual({
      width: '11.3235%',
      'max-width': '11.3235%',
    });
    expect(getGuestColumnStyle('availability', true)).toEqual({
      width: '7.2059%',
      'max-width': '7.2059%',
    });
    expect(getGuestColumnStyle('type', true)).toEqual({
      width: '9.2647%',
      'max-width': '9.2647%',
    });
    expect(getGuestColumnWidthStyle('name', true)).toEqual({ width: '30%' });
    expect(getGuestColumnWidthStyle('diskIo', true)).toEqual({ width: '170px' });
  });

  it('derives normalized tablet and compact widths from the visible workload columns', () => {
    const allModeColumns = GUEST_COLUMNS.filter((column) => VIEW_MODE_COLUMNS.all!.has(column.id));
    const narrowColumns = getWorkloadVisibleColumnsForLayout(allModeColumns, 'narrow');
    const phoneColumns = getWorkloadVisibleColumnsForLayout(allModeColumns, 'phone');
    const mobileColumns = getWorkloadVisibleColumnsForLayout(allModeColumns, 'mobile');
    const tabletColumns = getWorkloadVisibleColumnsForLayout(allModeColumns, 'tablet');
    const compactColumns = getWorkloadVisibleColumnsForLayout(allModeColumns, 'compact');

    expect(narrowColumns.map((column) => column.id)).toEqual([
      'name',
      'cpu',
      'memory',
      'disk',
      'uptime',
    ]);
    expect(
      getGuestColumnWidthStyle(
        'name',
        true,
        'narrow',
        narrowColumns.map((column) => column.id),
      ),
    ).toEqual({ width: '40%' });
    expect(
      getGuestColumnWidthStyle(
        'cpu',
        true,
        'narrow',
        narrowColumns.map((column) => column.id),
      ),
    ).toEqual({ width: '16%' });
    expect(phoneColumns.map((column) => column.id)).toEqual([
      'name',
      'availability',
      'cpu',
      'memory',
      'disk',
      'uptime',
    ]);
    expect(mobileColumns.map((column) => column.id)).toEqual([
      'name',
      'availability',
      'type',
      'info',
      'cpu',
      'memory',
      'disk',
      'uptime',
    ]);
    expect(tabletColumns.map((column) => column.id)).toEqual([
      'name',
      'availability',
      'type',
      'info',
      'cpu',
      'memory',
      'disk',
      'uptime',
    ]);
    expect(compactColumns.map((column) => column.id)).toEqual([
      'name',
      'availability',
      'type',
      'info',
      'cpu',
      'memory',
      'disk',
      'uptime',
      'aiContext',
      'backup',
    ]);

    expect(
      getGuestColumnWidthStyle(
        'name',
        false,
        'tablet',
        tabletColumns.map((column) => column.id),
      ),
    ).toEqual({ width: '30.9278%' });
    expect(
      getGuestColumnWidthStyle(
        'name',
        false,
        'compact',
        compactColumns.map((column) => column.id),
      ),
    ).toEqual({ width: '25.2427%' });
  });

  it('normalizes compact widths for workload view modes with different column sets', () => {
    const podColumns = GUEST_COLUMNS.filter((column) => VIEW_MODE_COLUMNS.pod!.has(column.id));
    const compactPodColumns = getWorkloadVisibleColumnsForLayout(podColumns, 'compact');
    const compactPodColumnIds = compactPodColumns.map((column) => column.id);

    expect(compactPodColumnIds).toEqual([
      'name',
      'cpu',
      'memory',
      'image',
      'namespace',
      'context',
      'aiContext',
    ]);
    expect(getGuestColumnWidthStyle('name', false, 'compact', compactPodColumnIds)).toEqual({
      width: '25%',
    });
  });

  it('gives Docker update status enough compact width for check-result labels', () => {
    const compactDockerRuntimeColumnIds = [
      'name',
      'runtime',
      'cpu',
      'memory',
      'uptime',
      'image',
      'context',
      'update',
    ];

    expect(
      getGuestColumnWidthStyle('update', false, 'compact', compactDockerRuntimeColumnIds),
    ).toEqual({
      width: '8.9286%',
    });
  });

  it('maps workload table layout modes to viewport width stages', () => {
    expect(getWorkloadTableLayoutMode(359)).toBe('narrow');
    expect(getWorkloadTableLayoutMode(360)).toBe('phone');
    expect(getWorkloadTableLayoutMode(479)).toBe('phone');
    expect(getWorkloadTableLayoutMode(480)).toBe('mobile');
    expect(getWorkloadTableLayoutMode(767)).toBe('mobile');
    expect(getWorkloadTableLayoutMode(768)).toBe('tablet');
    expect(getWorkloadTableLayoutMode(899)).toBe('tablet');
    expect(getWorkloadTableLayoutMode(900)).toBe('compact');
    // Wide waits for a shell that can hold the full column set without
    // horizontal scroll (see WORKLOAD_TABLE_WIDE_LAYOUT_WIDTH).
    expect(getWorkloadTableLayoutMode(1440)).toBe('compact');
    expect(getWorkloadTableLayoutMode(1535)).toBe('compact');
    expect(getWorkloadTableLayoutMode(1536)).toBe('wide');
  });

  it('maps workload table layout modes to the actual table container', () => {
    expect(getWorkloadTableLayoutModeForContainer(359)).toBe('narrow');
    expect(getWorkloadTableLayoutModeForContainer(360)).toBe('phone');
    expect(getWorkloadTableLayoutModeForContainer(439)).toBe('phone');
    expect(getWorkloadTableLayoutModeForContainer(440)).toBe('mobile');
    expect(getWorkloadTableLayoutModeForContainer(719)).toBe('mobile');
    expect(getWorkloadTableLayoutModeForContainer(720)).toBe('tablet');
    expect(getWorkloadTableLayoutModeForContainer(899)).toBe('tablet');
    expect(getWorkloadTableLayoutModeForContainer(900)).toBe('compact');
    expect(getWorkloadTableLayoutModeForContainer(1439)).toBe('compact');
    expect(getWorkloadTableLayoutModeForContainer(1440)).toBe('wide');
  });

  it('keeps CPU and memory fixed while allowing disk to be platform-scoped', () => {
    const nonToggleable = GUEST_COLUMNS.filter((c) => !c.toggleable);
    const ids = nonToggleable.map((c) => c.id);
    expect(ids).toContain('name');
    expect(ids).toContain('cpu');
    expect(ids).toContain('memory');
    expect(ids).not.toContain('disk');
    expect(ids).not.toContain('type');
    expect(GUEST_COLUMNS.find((c) => c.id === 'disk')?.toggleable).toBe(true);
  });
});

describe('VIEW_MODE_COLUMNS', () => {
  it('defines column sets for all 5 view modes', () => {
    expect(VIEW_MODE_COLUMNS.all).toBeInstanceOf(Set);
    expect(VIEW_MODE_COLUMNS.vm).toBeInstanceOf(Set);
    expect(VIEW_MODE_COLUMNS['system-container']).toBeInstanceOf(Set);
    expect(VIEW_MODE_COLUMNS['app-container']).toBeInstanceOf(Set);
    expect(VIEW_MODE_COLUMNS.pod).toBeInstanceOf(Set);
  });

  it('all mode includes info column (merged identifier)', () => {
    expect(VIEW_MODE_COLUMNS.all!.has('info')).toBe(true);
  });

  it('all view modes expose the hidden AI Context column', () => {
    for (const [, cols] of Object.entries(VIEW_MODE_COLUMNS)) {
      if (cols) expect(cols.has('aiContext')).toBe(true);
    }
  });

  it('all mode does not include vmid (uses info instead)', () => {
    expect(VIEW_MODE_COLUMNS.all!.has('vmid')).toBe(false);
  });

  it('vm mode includes vmid but not info', () => {
    expect(VIEW_MODE_COLUMNS.vm!.has('vmid')).toBe(true);
    expect(VIEW_MODE_COLUMNS.vm!.has('info')).toBe(false);
  });

  it('app-container mode includes image and context', () => {
    expect(VIEW_MODE_COLUMNS['app-container']!.has('runtime')).toBe(true);
    expect(VIEW_MODE_COLUMNS['app-container']!.has('image')).toBe(true);
    expect(VIEW_MODE_COLUMNS['app-container']!.has('context')).toBe(true);
  });

  it('app-container mode keeps capacity and I/O metrics available', () => {
    expect(VIEW_MODE_COLUMNS['app-container']!.has('disk')).toBe(true);
    expect(VIEW_MODE_COLUMNS['app-container']!.has('netIo')).toBe(true);
    expect(VIEW_MODE_COLUMNS['app-container']!.has('diskIo')).toBe(true);
  });

  it('pod mode includes namespace and image', () => {
    expect(VIEW_MODE_COLUMNS.pod!.has('namespace')).toBe(true);
    expect(VIEW_MODE_COLUMNS.pod!.has('image')).toBe(true);
  });

  it('pod mode is minimal (no disk, uptime, tags, backup)', () => {
    expect(VIEW_MODE_COLUMNS.pod!.has('disk')).toBe(false);
    expect(VIEW_MODE_COLUMNS.pod!.has('uptime')).toBe(false);
    expect(VIEW_MODE_COLUMNS.pod!.has('tags')).toBe(false);
    expect(VIEW_MODE_COLUMNS.pod!.has('backup')).toBe(false);
  });

  it('all view modes include name', () => {
    for (const [, cols] of Object.entries(VIEW_MODE_COLUMNS)) {
      if (cols) expect(cols.has('name')).toBe(true);
    }
  });

  it('all view modes include cpu and memory', () => {
    for (const [, cols] of Object.entries(VIEW_MODE_COLUMNS)) {
      if (cols) {
        expect(cols.has('cpu')).toBe(true);
        expect(cols.has('memory')).toBe(true);
      }
    }
  });

  it('narrows the combined container column profile when a subtype is excluded', () => {
    expect(resolveWorkloadColumnViewMode('container', ['app-container'])).toBe('system-container');
    expect(resolveWorkloadColumnViewMode('container', ['system-container'])).toBe('app-container');
    expect(resolveWorkloadColumnViewMode('container')).toBe('container');
    expect(resolveWorkloadColumnViewMode('container', ['system-container', 'app-container'])).toBe(
      'container',
    );
    expect(resolveWorkloadColumnViewMode('vm', ['app-container'])).toBe('vm');
  });
});

describe('getOutlierEmphasis (via I/O column rendering)', () => {
  // We test the outlier emphasis logic indirectly through the GuestRow rendering
  // by checking class names applied to net I/O and disk I/O values

  const makeIOStats = (overrides: Partial<WorkloadIOEmphasis['network']> = {}) => ({
    median: 100,
    mad: 20,
    max: 1000,
    p97: 800,
    p99: 950,
    count: 50,
    ...overrides,
  });

  it('applies muted class for normal I/O values', () => {
    const { container } = renderGuestRow({
      guest: makeGuest({ networkIn: 100, networkOut: 50, status: 'running' }),
      visibleColumnIds: ['name', 'netIo'],
      ioEmphasis: {
        network: makeIOStats(),
        diskIO: makeIOStats(),
      },
    });
    // Net I/O grid has 4-column layout; the value spans (2nd and 4th) should be text-muted
    const ioGrid = container.querySelector('.tabular-nums');
    expect(ioGrid).toBeTruthy();
    const valueSpans = ioGrid!.querySelectorAll('span.text-muted');
    // The two value spans (in/out) should have text-muted
    expect(valueSpans.length).toBe(2);
  });

  it('applies emphasis class for extreme outlier values', () => {
    // Value that exceeds p99 and has high modified-Z
    const { container } = renderGuestRow({
      guest: makeGuest({ networkIn: 960, networkOut: 960, status: 'running' }),
      visibleColumnIds: ['name', 'netIo'],
      ioEmphasis: {
        network: makeIOStats({ median: 100, mad: 20, p99: 950, max: 1000 }),
        diskIO: makeIOStats(),
      },
    });
    // Net I/O grid: the value spans should get emphasis styling (font-semibold)
    const ioGrid = container.querySelector('.tabular-nums');
    expect(ioGrid).toBeTruthy();
    const emphasized = ioGrid!.querySelectorAll('.font-semibold, .font-medium');
    expect(emphasized.length).toBeGreaterThan(0);
  });

  it('shows dash for net I/O when guest is stopped', () => {
    renderGuestRow({
      guest: makeGuest({ status: 'stopped', networkIn: 0, networkOut: 0 }),
      visibleColumnIds: ['name', 'netIo'],
    });
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });

  it('renders valid zero I/O rates for a running guest on an online parent', () => {
    renderGuestRow({
      guest: makeGuest({
        status: 'running',
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
      }),
      parentNodeOnline: true,
      visibleColumnIds: ['name', 'netIo', 'diskIo'],
    });

    expect(screen.getAllByText('0 B/s')).toHaveLength(4);
    expect(screen.queryByText('—')).toBeNull();
  });

  it('shows dash for disk I/O when guest is stopped', () => {
    renderGuestRow({
      guest: makeGuest({ status: 'stopped', diskRead: 0, diskWrite: 0 }),
      visibleColumnIds: ['name', 'diskIo'],
    });
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });
});

describe('OCI container handling', () => {
  it('renders system-container badge with OCI-specific tooltip for OCI containers', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'oci-container',
        workloadType: 'system-container',
        osTemplate: 'oci:docker.io/library/alpine:3.18',
      }),
      visibleColumnIds: ['name', 'type'],
    });
    const badge = screen.getByText('SYSTEM-CONTAINER');
    expect(badge).toBeTruthy();
    expect(badge.getAttribute('title')).toBe('OCI Container • docker.io/library/alpine:3.18');
  });
});

describe('context column for PVE workloads', () => {
  it('shows cluster name badge for PVE workloads with clusterName', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        node: 'pve1',
        contextLabel: 'pve1',
        clusterName: 'prod-cluster',
      }),
      visibleColumnIds: ['name', 'context'],
    });
    expect(screen.getByText('prod-cluster')).toBeTruthy();
  });
});

describe('backup column', () => {
  it('keeps a fresh backup age accessible without drawing it in the row', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-26T12:00:00Z'));

    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        lastBackup: Date.parse('2026-05-26T07:00:00Z'),
      }),
      visibleColumnIds: ['name', 'backup'],
    });

    // A healthy backup is carried by the shield colour alone. The age stays in
    // the aria-label and the tooltip so nothing is lost.
    expect(screen.queryByText('5h')).toBeNull();
    expect(
      container.querySelector('[aria-label="Backup status: fresh, last backup 5 hours ago"]'),
    ).toBeTruthy();
  });

  it('draws the compact age for supported guests whose backup is stale', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-26T12:00:00Z'));

    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        lastBackup: Date.parse('2026-05-24T12:00:00Z'),
      }),
      visibleColumnIds: ['name', 'backup'],
    });

    const badge = container.querySelector('[aria-label^="Backup status: stale"]');
    expect(badge).toBeTruthy();
    expect(badge?.textContent?.trim()).toMatch(/\d/);
  });

  it('keeps an overdue backup amber because a backup still exists', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-26T12:00:00Z'));

    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        lastBackup: Date.parse('2026-05-21T12:00:00Z'),
      }),
      visibleColumnIds: ['name', 'backup'],
    });

    const badge = container.querySelector('[aria-label^="Backup status: overdue"]');
    expect(badge).toBeTruthy();
    expect(badge?.classList.contains('text-yellow-700')).toBe(true);
    expect(badge?.className).not.toContain('text-red');
  });

  it('reserves red for supported guests without a backup', () => {
    const { container } = renderGuestRow({
      guest: makeGuest({ type: 'qemu', workloadType: 'vm', lastBackup: 0 }),
      visibleColumnIds: ['name', 'backup'],
    });
    expect(screen.getByText('None')).toBeTruthy();
    const badge = container.querySelector('[aria-label="Backup status: no backup found"]');
    expect(badge).toBeTruthy();
    expect(badge?.classList.contains('text-red-700')).toBe(true);
  });

  it('keeps positive backup coverage visible when the backup column is hidden', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-26T12:00:00Z'));

    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        lastBackup: Date.parse('2026-05-26T07:00:00Z'),
      }),
      visibleColumnIds: ['name'],
    });

    const indicator = container.querySelector('[aria-label="Last backup: 5 hours ago"]');
    expect(indicator).toBeTruthy();
    expect(indicator?.classList.contains('text-green-600')).toBe(true);
  });

  it('shows active backup work in blue when the backup column is hidden', () => {
    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        lastBackup: 0,
        backupInProgress: true,
      }),
      visibleColumnIds: ['name'],
    });

    const indicator = container.querySelector(
      '[aria-label="Backup running now · no completed backup found"]',
    );
    expect(indicator).toBeTruthy();
    expect(indicator?.classList.contains('text-blue-600')).toBe(true);
  });

  it('does not infer a backup failure for a vSphere VM when the column is hidden', () => {
    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'vm',
        workloadType: 'vm',
        platformType: 'vmware-vsphere',
        platformScopes: ['vmware-vsphere'],
        lastBackup: 0,
      }),
      visibleColumnIds: ['name'],
    });

    expect(container.querySelector('[title="No backup found"]')).toBeNull();
    expect(container.querySelector('[aria-label^="Backup status:"]')).toBeNull();
  });

  it('renders backup status as unavailable for a vSphere VM when the column is visible', () => {
    const { container } = renderGuestRow({
      guest: makeGuest({
        type: 'vm',
        workloadType: 'vm',
        platformType: 'vmware-vsphere',
        platformScopes: ['vmware-vsphere'],
        lastBackup: 0,
      }),
      visibleColumnIds: ['name', 'backup'],
    });

    expect(container.querySelector('[aria-label^="Backup status:"]')).toBeNull();
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });

  it('shows dash for app-container workloads (no backup support)', () => {
    renderGuestRow({
      guest: makeGuest({ type: 'app-container', workloadType: 'app-container' }),
      visibleColumnIds: ['name', 'backup'],
    });
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });

  it('shows dash for template guests', () => {
    renderGuestRow({
      guest: makeGuest({ type: 'qemu', template: true }),
      visibleColumnIds: ['name', 'backup'],
    });
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });
});

describe('info merged column', () => {
  it('shows VMID for VM workloads in info column', () => {
    renderGuestRow({
      guest: makeGuest({ type: 'qemu', workloadType: 'vm', vmid: 200, displayId: '200' }),
      visibleColumnIds: ['name', 'info'],
    });
    expect(screen.getByText('200')).toBeTruthy();
  });

  it('shows short image name for app-container workloads in info column', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'app-container',
        workloadType: 'app-container',
        image: 'library/nginx:latest',
      }),
      visibleColumnIds: ['name', 'info'],
    });
    expect(screen.getByText('nginx:latest')).toBeTruthy();
  });

  it('shows namespace for pod workloads in info column', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'pod',
        workloadType: 'pod',
        namespace: 'kube-system',
      }),
      visibleColumnIds: ['name', 'info'],
    });
    expect(screen.getByText('kube-system')).toBeTruthy();
  });

  it('shows dash when no info value is available', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'app-container',
        workloadType: 'app-container',
        image: '',
      }),
      visibleColumnIds: ['name', 'info'],
    });
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });
});

describe('event propagation', () => {
  it('name custom URL click does not trigger row onClick', () => {
    const rowClick = vi.fn();
    const { container } = renderGuestRow({
      guest: makeGuest(),
      customUrl: 'https://example.com',
      onClick: rowClick,
      visibleColumnIds: ['name'],
    });
    const link = container.querySelector('a[href="https://example.com"]')!;
    fireEvent.click(link);
    expect(rowClick).not.toHaveBeenCalled();
  });
});

describe('app-container update button visibility', () => {
  afterEach(() => {
    updateButtonSpy.mockReset();
  });

  it('does not show update button when dockerHostId is missing', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'app-container',
        workloadType: 'app-container',
        dockerHostId: '',
      }),
      visibleColumnIds: ['name', 'update'],
    });
    expect(screen.queryByTestId('update-button')).toBeNull();
  });

  it('shows update button when dockerHostId is present', () => {
    renderGuestRow({
      guest: makeGuest({
        id: 'app-container:docker-main:grafana',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'host-1',
        containerId: 'docker-grafana',
        name: 'grafana',
      }),
      visibleColumnIds: ['name', 'update'],
    });
    expect(screen.getByTestId('update-button')).toBeTruthy();
    expect(updateButtonSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        agentId: 'host-1',
        containerId: 'docker-grafana',
        containerName: 'grafana',
      }),
    );
  });

  it('does not show Docker update button for TrueNAS app containers', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        containerRuntime: 'docker',
        dockerHostId: 'truenas-main',
      }),
      visibleColumnIds: ['name', 'update'],
    });
    expect(screen.queryByTestId('update-button')).toBeNull();
  });
});
