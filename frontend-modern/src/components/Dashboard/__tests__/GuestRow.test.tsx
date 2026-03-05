import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import type { WorkloadGuest } from '@/types/workloads';
import type { Memory, Disk } from '@/types/api';

// ── Hoisted mocks ──────────────────────────────────────────────────────

const { isMobileMock } = vi.hoisted(() => {
  const isMobileMock = vi.fn(() => false);
  return { isMobileMock };
});

// ── Module mocks ───────────────────────────────────────────────────────

const mockNavigate = vi.fn();

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

vi.mock('@/components/Dashboard/EnhancedCPUBar', () => ({
  EnhancedCPUBar: (props: { usage: number; cores?: number }) => (
    <div data-testid="cpu-bar" data-usage={props.usage} data-cores={props.cores} />
  ),
}));

vi.mock('../StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="disk-bar" />,
}));

vi.mock('../StackedMemoryBar', () => ({
  StackedMemoryBar: (props: { used: number; total: number }) => (
    <div data-testid="memory-bar" data-used={props.used} data-total={props.total} />
  ),
}));

vi.mock('../TagBadges', () => ({
  TagBadges: (props: { tags: string[] }) => (
    <div data-testid="tag-badges" data-count={props.tags.length} />
  ),
}));

vi.mock('@/components/shared/ContainerUpdateBadge', () => ({
  UpdateButton: () => <div data-testid="update-button" />,
}));

vi.mock('@/components/shared/workloadTypeBadges', () => ({
  getWorkloadTypeBadge: (type: string, opts?: { label?: string; title?: string }) => ({
    className: `badge-${type}`,
    label: opts?.label || type.toUpperCase(),
    title: opts?.title || type,
  }),
}));

vi.mock('../infrastructureLink', () => ({
  buildInfrastructureHrefForWorkload: () => '/infrastructure/node1',
}));

vi.mock('../workloadSelectors', () => ({
  getWorkloadDockerHostId: (guest: WorkloadGuest) => guest.dockerHostId || '',
}));

// After mocks, import
import { GuestRow, GUEST_COLUMNS, VIEW_MODE_COLUMNS, type WorkloadIOEmphasis } from '../GuestRow';

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

    it('renders memory bar', () => {
      renderGuestRow({ guest: makeGuest() });
      expect(screen.getByTestId('memory-bar')).toBeTruthy();
    });

    it('renders disk bar when disk usage is available', () => {
      renderGuestRow({ guest: makeGuest() });
      expect(screen.getByTestId('disk-bar')).toBeTruthy();
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
    it('shows vmid when displayId is not set', () => {
      renderGuestRow({
        guest: makeGuest({ vmid: 123, displayId: undefined }),
        visibleColumnIds: ['name', 'vmid'],
      });
      expect(screen.getByText('123')).toBeTruthy();
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
        guest: makeGuest({ tags: ['prod', 'web'] }),
        visibleColumnIds: ['name', 'tags'],
      });
      const badges = screen.getByTestId('tag-badges');
      expect(badges.dataset.count).toBe('2');
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

    it('applies box-shadow for alert accent on unacknowledged alerts', () => {
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
      const style = tr?.getAttribute('style') ?? '';
      expect(style).toContain('box-shadow');
      expect(style).toContain('#ef4444');
    });

    it('applies grey accent for acknowledged-only alerts', () => {
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
      const style = tr?.getAttribute('style') ?? '';
      expect(style).toContain('#9ca3af');
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

    it('calls onHoverChange with canonical guestId on mouseenter', () => {
      const onHoverChange = vi.fn();
      const { container } = renderGuestRow({
        guest: makeGuest({ id: 'inst1-node1-100' }),
        onHoverChange,
      });
      const tr = container.querySelector('tr')!;
      const expectedId = tr.dataset.guestId!;
      fireEvent.mouseEnter(tr);
      expect(onHoverChange).toHaveBeenCalledWith(expectedId);
    });

    it('calls onHoverChange with null on mouseleave', () => {
      const onHoverChange = vi.fn();
      const { container } = renderGuestRow({
        guest: makeGuest(),
        onHoverChange,
      });
      const tr = container.querySelector('tr')!;
      fireEvent.mouseLeave(tr);
      expect(onHoverChange).toHaveBeenCalledWith(null);
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
    it('renders node name as a link button', () => {
      renderGuestRow({
        guest: makeGuest({ node: 'pve1' }),
        visibleColumnIds: ['name', 'node'],
      });
      const nodeButton = screen.getByText('pve1');
      expect(nodeButton.tagName).toBe('BUTTON');
    });

    it('navigates on node click', () => {
      renderGuestRow({
        guest: makeGuest({ node: 'pve1' }),
        visibleColumnIds: ['name', 'node'],
      });
      const nodeButton = screen.getByText('pve1');
      fireEvent.click(nodeButton);
      expect(mockNavigate).toHaveBeenCalledWith('/infrastructure/node1');
    });
  });

  describe('app-container workload type', () => {
    it('shows image column for app-container guests', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          image: 'nginx:latest',
        }),
        visibleColumnIds: ['name', 'image'],
      });
      // getShortImageName truncates the image
      expect(screen.getByText('nginx:latest')).toBeTruthy();
    });

    it('shows update button for app-container guests', () => {
      renderGuestRow({
        guest: makeGuest({
          type: 'app-container',
          workloadType: 'app-container',
          dockerHostId: 'host-1',
        }),
        visibleColumnIds: ['name', 'update'],
      });
      expect(screen.getByTestId('update-button')).toBeTruthy();
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
      expect(firstTd?.className).toContain('pl-3');
    });

    it('uses default indent class when isGroupedView is false', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isGroupedView: false,
      });
      const firstTd = container.querySelector('td');
      expect(firstTd?.className).toContain('pl-2');
    });
  });

  describe('custom URL in link column', () => {
    it('renders external link when customUrl is set', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        customUrl: 'https://example.com',
        visibleColumnIds: ['name', 'link'],
      });
      const link = container.querySelector('a[href="https://example.com"]');
      expect(link).toBeTruthy();
      expect(link?.getAttribute('target')).toBe('_blank');
    });

    it('renders infrastructure link button when no customUrl', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        visibleColumnIds: ['name', 'link'],
      });
      // Should have a button instead of an anchor
      const buttons = container.querySelectorAll('td:last-child button');
      expect(buttons.length).toBeGreaterThan(0);
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
      });
      const chevronWrapper = container.querySelector('.rotate-90');
      expect(chevronWrapper).toBeTruthy();
    });

    it('does not rotate chevron when collapsed', () => {
      const { container } = renderGuestRow({
        guest: makeGuest(),
        isExpanded: false,
      });
      const chevronWrapper = container.querySelector('.rotate-90');
      expect(chevronWrapper).toBeNull();
    });
  });
});

describe('GUEST_COLUMNS', () => {
  it('has the expected number of columns', () => {
    // name, type, info, vmid, cpu, memory, disk, ip, uptime, node,
    // image, namespace, context, backup, tags, update, os, netIo, diskIo, link
    expect(GUEST_COLUMNS.length).toBe(20);
  });

  it('has name as the first column', () => {
    expect(GUEST_COLUMNS[0].id).toBe('name');
  });

  it('has link as the last column', () => {
    expect(GUEST_COLUMNS[GUEST_COLUMNS.length - 1].id).toBe('link');
  });

  it('marks toggleable columns correctly', () => {
    const toggleable = GUEST_COLUMNS.filter((c) => c.toggleable);
    const toggleableIds = toggleable.map((c) => c.id);
    expect(toggleableIds).toContain('ip');
    expect(toggleableIds).toContain('uptime');
    expect(toggleableIds).toContain('node');
    expect(toggleableIds).toContain('backup');
    expect(toggleableIds).toContain('tags');
    expect(toggleableIds).toContain('os');
    expect(toggleableIds).toContain('netIo');
    expect(toggleableIds).toContain('diskIo');
  });

  it('non-toggleable columns include core metrics', () => {
    const nonToggleable = GUEST_COLUMNS.filter((c) => !c.toggleable);
    const ids = nonToggleable.map((c) => c.id);
    expect(ids).toContain('name');
    expect(ids).toContain('cpu');
    expect(ids).toContain('memory');
    expect(ids).toContain('disk');
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

  it('all mode does not include vmid (uses info instead)', () => {
    expect(VIEW_MODE_COLUMNS.all!.has('vmid')).toBe(false);
  });

  it('vm mode includes vmid but not info', () => {
    expect(VIEW_MODE_COLUMNS.vm!.has('vmid')).toBe(true);
    expect(VIEW_MODE_COLUMNS.vm!.has('info')).toBe(false);
  });

  it('app-container mode includes image and context', () => {
    expect(VIEW_MODE_COLUMNS['app-container']!.has('image')).toBe(true);
    expect(VIEW_MODE_COLUMNS['app-container']!.has('context')).toBe(true);
  });

  it('app-container mode does not include disk or netIo', () => {
    expect(VIEW_MODE_COLUMNS['app-container']!.has('disk')).toBe(false);
    expect(VIEW_MODE_COLUMNS['app-container']!.has('netIo')).toBe(false);
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

  it('shows dash for disk I/O when guest is stopped', () => {
    renderGuestRow({
      guest: makeGuest({ status: 'stopped', diskRead: 0, diskWrite: 0 }),
      visibleColumnIds: ['name', 'diskIo'],
    });
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
  });
});

describe('OCI container handling', () => {
  it('renders OCI badge for OCI containers', () => {
    renderGuestRow({
      guest: makeGuest({
        type: 'oci-container',
        workloadType: 'system-container',
        osTemplate: 'oci:docker.io/library/alpine:3.18',
      }),
      visibleColumnIds: ['name', 'type'],
    });
    // Should render the OCI type badge
    expect(screen.getByText('OCI-CONTAINER')).toBeTruthy();
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
    // getShortImageName('library/nginx:latest') returns 'library/nginx:latest' (last 2 parts)
    expect(screen.getByText('library/nginx:latest')).toBeTruthy();
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
  it('node button click does not trigger row onClick', () => {
    const rowClick = vi.fn();
    renderGuestRow({
      guest: makeGuest({ node: 'pve1' }),
      onClick: rowClick,
      visibleColumnIds: ['name', 'node'],
    });
    const nodeButton = screen.getByText('pve1');
    fireEvent.click(nodeButton);
    // Node click calls stopPropagation, so row onClick should not fire
    expect(rowClick).not.toHaveBeenCalled();
  });

  it('link button click does not trigger row onClick', () => {
    const rowClick = vi.fn();
    const { container } = renderGuestRow({
      guest: makeGuest(),
      onClick: rowClick,
      visibleColumnIds: ['name', 'link'],
    });
    const linkButton = container.querySelector('td:last-child button')!;
    fireEvent.click(linkButton);
    expect(rowClick).not.toHaveBeenCalled();
  });
});

describe('app-container update button visibility', () => {
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
        type: 'app-container',
        workloadType: 'app-container',
        dockerHostId: 'host-1',
      }),
      visibleColumnIds: ['name', 'update'],
    });
    expect(screen.getByTestId('update-button')).toBeTruthy();
  });
});
