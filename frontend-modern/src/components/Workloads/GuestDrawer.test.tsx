import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import {
  render as solidRender,
  screen,
  fireEvent,
  cleanup,
  waitFor,
  within,
} from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { WorkloadGuest } from '@/types/workloads';
import type { Alert, Memory, Disk, GuestNetworkInterface } from '@/types/api';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { getWorkloadMetadataId } from '@/utils/workloads';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import guestDrawerSource from './GuestDrawer.tsx?raw';
import guestDrawerManageSource from './GuestDrawerManage.tsx?raw';
import guestDrawerOverviewSource from './GuestDrawerOverview.tsx?raw';
import { getGuestDrawerAlertAttention, getGuestDrawerHistoryRangeBounds } from './guestDrawerModel';

// ── Mocks ──────────────────────────────────────────────────────────────

describe('shared drawer History window geometry', () => {
  it('preserves the requested interval and edge observations without manufacturing timestamps', () => {
    const sample = (timestamp: number) => ({ timestamp, value: 0, min: 0, max: 0 });
    const series = [{ points: [sample(800), sample(900)] }, { points: [sample(300)] }];
    expect(getGuestDrawerHistoryRangeBounds(series, { start: 0, end: 1000 })).toEqual({
      startTime: 0,
      endTime: 1000,
    });
    expect(getGuestDrawerHistoryRangeBounds(series, { start: 500, end: 1000 })).toEqual({
      startTime: 300,
      endTime: 1000,
    });
    expect(getGuestDrawerHistoryRangeBounds(series, { start: 1000, end: 0 })).toEqual({
      startTime: 300,
      endTime: 900,
    });
    expect(getGuestDrawerHistoryRangeBounds([], { start: 0, end: 0 })).toBeNull();
  });
});

const chartsApiMocks = vi.hoisted(() => ({
  getMetricsHistory: vi.fn(),
}));

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      getMetricsHistory: chartsApiMocks.getMetricsHistory,
    },
  };
});

vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 90,
}));

vi.mock('@/components/shared/WebInterfaceUrlField', () => ({
  WebInterfaceUrlField: (props: {
    metadataKind: string;
    metadataId: string;
    targetLabel: string;
  }) => (
    <div data-testid="web-interface-url-field">
      <span data-testid="url-kind">{props.metadataKind}</span>
      <span data-testid="url-id">{props.metadataId}</span>
      <span data-testid="url-label">{props.targetLabel}</span>
    </div>
  ),
}));

// After mocks, import the component under test
import { GuestDrawer } from './GuestDrawer';
// Overview loads its charts lazily; warm the module so a cold transform does
// not outlast the tests' waits.
import './GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_POLL_MS } from './guestDrawerHistoryQuery';

const render = (...args: Parameters<typeof solidRender>): ReturnType<typeof solidRender> => {
  const result = solidRender(...args);
  const details = result.container.querySelector<HTMLDetailsElement>(
    '[data-testid="guest-technical-details"]',
  );
  if (details) {
    details.open = true;
    fireEvent(details, new Event('toggle'));
  }
  const manageTab = screen.queryByRole('tab', { name: 'Manage' });
  if (manageTab) fireEvent.click(manageTab);
  return result;
};

// ── Helpers ────────────────────────────────────────────────────────────

/** Build a minimal WorkloadGuest with required fields, overridable via partial. */
function makeGuest(overrides: Partial<WorkloadGuest> = {}): WorkloadGuest {
  return {
    id: 'inst1-node1-100',
    vmid: 100,
    name: 'test-vm',
    node: 'node1',
    instance: 'inst1',
    status: 'running',
    type: 'qemu',
    metricsTarget: { resourceType: 'vm', resourceId: 'inst1:node1:100' },
    cpu: 0.25,
    cpus: 4,
    memory: { total: 4294967296, used: 2147483648, free: 2147483648, usage: 0.5 },
    disk: { total: 10737418240, used: 5368709120, free: 5368709120, usage: 0.5 },
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

function makeGuestWithDiscoveryTarget(overrides: Partial<WorkloadGuest> = {}): WorkloadGuest {
  return makeGuest({
    discoveryTarget: {
      resourceType: 'vm',
      agentId: 'agent-node1',
      resourceId: '100',
    },
    ...overrides,
  });
}

// Filesystems and lifted warning rows render outside Details; the render
// helper opens Details, so read the whole Overview.
const technicalDetails = () => within(screen.getByTestId('guest-drawer-overview'));

const makeHistoryPoints = (base: number) => [
  { timestamp: 1, value: base, min: base, max: base },
  { timestamp: 2, value: base + 5, min: base + 5, max: base + 5 },
  { timestamp: 3, value: base + 10, min: base + 10, max: base + 10 },
];

beforeEach(() => {
  resetAIRuntimeState();
  resetCreateNonSuspendingQueryCacheForTest();
  chartsApiMocks.getMetricsHistory.mockResolvedValue({
    resourceType: 'vm',
    resourceId: 'inst1:node1:100',
    range: '24h',
    start: 1,
    end: 3,
    metrics: {
      cpu: makeHistoryPoints(10),
      memory: makeHistoryPoints(20),
      disk: makeHistoryPoints(30),
      netin: makeHistoryPoints(1000),
      netout: makeHistoryPoints(2000),
      diskread: makeHistoryPoints(3000),
      diskwrite: makeHistoryPoints(4000),
    },
    source: 'store',
  });
});

afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
  vi.useRealTimers();
});

// ── Tests ──────────────────────────────────────────────────────────────

describe('GuestDrawer', () => {
  it('keeps the shared memory provenance policy in the full guest drawer after live replacement', async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue({
      resourceType: 'vm',
      resourceId: 'inst1:node1:100',
      range: '24h',
      start: 1,
      end: 3,
      source: 'store',
      metrics: {},
    });
    const [guest, setGuest] = createSignal(
      makeGuest({
        memory: {
          total: 100,
          used: 0,
          free: 100,
          usage: 0,
          observation: {
            state: 'last-known',
            source: 'guest-agent-meminfo',
            observedAt: '2026-10-04T12:00:00Z',
          },
        },
      }),
    );
    render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    await waitFor(() => expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(1));
    const chart = screen.getAllByTestId('guest-history-group-chart')[0];
    expect(chart.querySelector('[data-history-last-known="memory"]')).toHaveTextContent(
      '0.0%last known',
    );
    expect(chart.querySelector('[data-history-current="memory"]')).toBeNull();
    setGuest({ ...guest(), lastSeen: '2026-10-05T00:00:00Z' });
    expect(chart.querySelector('[data-history-deferred="memory"]')).toHaveTextContent(
      '2026-10-04 12:00:00 UTC',
    );
    setGuest({
      ...guest(),
      memory: {
        ...guest().memory,
        observation: { state: 'current', source: 'status-mem', observedAt: '2026-10-04T13:00:00Z' },
      },
    });
    expect(screen.getAllByTestId('guest-history-group-chart')[0]).toBe(chart);
    expect(chart.querySelector('[data-history-current="memory"]')).toHaveTextContent('0.0%current');
    expect(chart.querySelector('[data-history-deferred="memory"]')).toBeNull();
    expect(chart.querySelector('path[data-history-series]')).toBeNull();
    expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(1);
  });

  it('keeps stored History gaps through live guest recovery without another read', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    const points = makeHistoryPoints(10);
    const payload = {
      resourceType: 'vm',
      resourceId: 'inst1:node1:100',
      range: '24h',
      start: 1,
      end: 3,
      source: 'store',
      metrics: { cpu: points, memory: [points[0], points[2]] },
    };
    chartsApiMocks.getMetricsHistory.mockResolvedValue(payload);
    const [guest, setGuest] = createSignal(
      makeGuest({ lock: 'backup', diskStatusReason: 'prev-vm-locked' }),
    );
    render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = screen.getAllByTestId('guest-history-group-chart')[0];
    expect(chart.querySelectorAll('path[data-history-series]')).toHaveLength(1);
    expect(chart.querySelectorAll('[data-history-observation="memory"]')).toHaveLength(2);
    expect(chart.querySelector('[data-history-gaps]')).toHaveTextContent(
      'Missing observations: Memory.',
    );
    setGuest({
      ...guest(),
      lock: '',
      diskStatusReason: '',
      memory: {
        ...guest().memory,
        usage: 99,
        observation: { state: 'current', source: 'status-mem', observedAt: '2026-10-04T12:00:00Z' },
      },
    });
    expect(screen.getAllByTestId('guest-history-group-chart')[0]).toBe(chart);
    expect(chart.querySelectorAll('[data-history-observation="memory"]')).toHaveLength(2);
    expect(chart).not.toHaveTextContent('99.0%');
    expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(1);
    expect(chartsApiMocks.getMetricsHistory.mock.calls[0][0]).toMatchObject({
      resourceType: 'vm',
      resourceId: 'inst1:node1:100',
      range: '24h',
      maxPoints: 240,
    });
    chartsApiMocks.getMetricsHistory.mockResolvedValue({
      ...payload,
      metrics: { cpu: points, memory: points },
    });
    await vi.advanceTimersByTimeAsync(GUEST_DRAWER_HISTORY_POLL_MS);
    await waitFor(() => expect(chart.querySelector('[data-history-gaps]')).toBeNull());
    expect(chart.querySelectorAll('path[data-history-series]')).toHaveLength(2);
    expect(chart.querySelector('[data-history-observation="memory"]')).toBeNull();
    expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(2);
  });

  it('keeps the accessible filesystem guidance target across read-reason updates', () => {
    const [guest, setGuest] = createSignal(
      makeGuest({
        diskStatusReason: 'agent-not-running',
        disks: [{ mountpoint: '/', total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 }],
      }),
    );
    solidRender(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    const guidance = screen.getByRole('link', { name: 'Filesystem reading guidance' });
    expect(guidance).toHaveAttribute(
      'href',
      '/docs/VM_DISK_MONITORING#a-missing-reading-is-not-an-installation-diagnosis',
    );
    expect(guidance).toHaveClass(
      'inline-flex',
      'min-h-11',
      'sm:min-h-0',
      'text-base-content',
      'focus-visible:outline-2',
      'focus-visible:outline-offset-2',
      'focus-visible:outline-base-content',
    );
    setGuest({ ...guest(), diskStatusReason: 'prev-vm-locked' });
    expect(screen.getByRole('link', { name: 'Filesystem reading guidance' })).toHaveAttribute(
      'href',
      guidance.getAttribute('href'),
    );
    expect(technicalDetails().getByText(/^Using last known disk stats\./)).toHaveTextContent(
      'VM operation lock, such as a backup',
    );
    setGuest({ ...guest(), diskStatusReason: '' });
    expect(screen.queryByRole('link', { name: 'Filesystem reading guidance' })).toBeNull();
  });

  it('keeps the backup precaution visible across drawer tabs without starting a guest check', async () => {
    const [guest, setGuest] = createSignal(
      makeGuest({
        agentVersion: '9.2',
        guestAgentStatus: 'deferred',
        diskStatusReason: 'prev-vm-locked',
        lock: 'backup',
      }),
    );
    render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    const precaution = () => screen.getByTestId('guest-read-precaution');
    expect(precaution()).toBeVisible();
    expect(precaution()).toHaveTextContent(
      'Do not run live diagnostics or restart the guest agent during a backup, freeze/thaw or an unresponsive-guest incident.',
    );
    expect(precaution()).toHaveTextContent('does not prove thaw');
    expect(precaution()).toHaveTextContent('filesystems covered by the backup');
    expect(
      within(precaution()).getByRole('link', { name: 'Backup safety guidance' }),
    ).toHaveAttribute('href', '/docs/VM_DISK_MONITORING');
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    expect(precaution()).toBeVisible();
    setGuest({
      ...guest(),
      lock: '',
      guestAgentStatus: 'expected-unreachable',
      diskStatusReason: 'agent-timeout',
    });
    expect(precaution()).toHaveTextContent('Completion is uncertain');
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(precaution()).toBeVisible();
    setGuest({ ...guest(), guestAgentStatus: 'available', diskStatusReason: '' });
    expect(screen.queryByTestId('guest-read-precaution')).not.toBeInTheDocument();
  });

  it('keeps malformed and future backup times unknown through same-guest replacement', () => {
    const [guest, setGuest] = createSignal(makeGuest({ lastBackup: NaN }));
    render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    const details = technicalDetails();
    expect(details.getByText('Backup time unavailable: invalid timestamp.')).toBeVisible();
    expect(details.queryByText('No completed backup found')).not.toBeInTheDocument();
    expect(details.queryByText('1 hour ago')).not.toBeInTheDocument();
    setGuest({ ...guest(), lastBackup: Date.now() + 86_400_000, backupInProgress: true });
    expect(details.getByText(/timestamp is in the future/)).toBeVisible();
    expect(details.getByText('Running · not completed yet')).toBeVisible();
    setGuest({ ...guest(), lastBackup: Date.now() - 3600_000, backupInProgress: false });
    expect(details.getByText('1 hour ago')).toBeVisible();
    expect(details.queryByText(/unavailable/)).not.toBeInTheDocument();
    setGuest({ ...guest(), lastBackup: 0 });
    expect(details.getByText('No completed backup found')).toBeVisible();
  });

  it('keeps deferred filesystem fallback last-known through History and fresh resumption', async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue({
      resourceType: 'vm',
      resourceId: 'inst1:node1:100',
      range: '24h',
      start: 1,
      end: 3,
      metrics: {},
      source: 'store',
    });
    const [guest, setGuest] = createSignal(
      makeGuest({
        disk: { total: 100, used: 50, usage: 50 },
        diskStatusReason: 'prev-vm-locked',
        lock: 'backup',
      }),
    );
    render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    await waitFor(() =>
      expect(screen.getAllByText('No stored history in this range')).toHaveLength(3),
    );
    const history = screen.getAllByTestId('guest-history-group-chart')[0];
    expect(history.querySelector('[data-history-current="disk"]')).toBeNull();
    expect(history.querySelector('[data-history-last-known="disk"]')).toHaveTextContent(
      'Disk50.0%last known',
    );
    expect(history.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(
      'Guest reads paused while Proxmox reports a VM operation lock',
    );
    setGuest({ ...guest(), lock: '', diskStatusReason: 'prev-agent-busy' });
    expect(history.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(
      'earlier guest request is still in progress',
    );
    expect(history.querySelector('[data-history-current="disk"]')).toBeNull();
    setGuest({ ...guest(), diskStatusReason: '', disk: { total: 100, used: 75, usage: 75 } });
    expect(history.querySelector('[data-history-last-known="disk"]')).toBeNull();
    expect(history.querySelector('[data-history-deferred="disk"]')).toBeNull();
    expect(history.querySelector('[data-history-current="disk"]')).toHaveTextContent(
      'Disk75.0%current',
    );
    // Fresh disk evidence cannot qualify the independent legacy memory value.
    expect(history.querySelector('[data-history-current="memory"]')).toBeNull();
    expect(history.querySelector('[data-history-unknown="memory"]')).toHaveTextContent(
      'freshness unknown',
    );
    expect(history.querySelector('[data-history-deferred="memory"]')).toHaveTextContent(
      'Not a current measurement.',
    );
    expect(history.querySelector('path[data-history-series]')).toBeNull();
    // Same target/range snapshots retain the chart and original History read.
    expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(1);
    for (const [request] of chartsApiMocks.getMetricsHistory.mock.calls) {
      expect(request).toMatchObject({
        resourceType: 'vm',
        resourceId: 'inst1:node1:100',
        range: '24h',
      });
    }
  });

  it("judges a linked agent's disk metric in History by that agent, not the Proxmox reason", async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue({
      resourceType: 'vm',
      resourceId: 'inst1:node1:100',
      range: '24h',
      start: 1,
      end: 3,
      metrics: {},
      source: 'store',
    });
    // Proxmox has no guest filesystems, so the agent's filesystems and disk
    // metric fill in while Proxmox still reports its own agent-not-running.
    const [guest, setGuest] = createSignal(
      makeGuest({
        disk: { total: 100, used: 40, free: 60, usage: 40 },
        disks: [{ total: 100, used: 40, free: 60, usage: 40, mountpoint: '/' }],
        disksFromAgent: true,
        diskStatusReason: 'agent-not-running',
      }),
    );
    render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    await waitFor(() =>
      expect(screen.getAllByText('No stored history in this range')).toHaveLength(3),
    );
    const history = screen.getAllByTestId('guest-history-group-chart')[0];
    expect(history.querySelector('[data-history-current="disk"]')).toHaveTextContent(
      'Disk40.0%current',
    );
    expect(history.querySelector('[data-history-deferred="disk"]')).toBeNull();

    setGuest({ ...guest(), agentStale: true });
    expect(history.querySelector('[data-history-current="disk"]')).toBeNull();
    expect(history.querySelector('[data-history-last-known="disk"]')).toHaveTextContent(
      'Disk40.0%last known',
    );
    expect(history.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(
      'The Pulse Agent in this guest stopped reporting.',
    );

    setGuest({ ...guest(), status: 'stopped' });
    expect(history.querySelector('[data-history-last-known="disk"]')).toBeNull();
    expect(history.querySelector('[data-history-current="disk"]')).toBeNull();
    expect(history.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(
      'Guest filesystem stats unavailable while the VM is stopped.',
    );
  });

  it('keeps canonical resource policy out of Overview and mounts it on demand in Manage', () => {
    expect(guestDrawerOverviewSource).not.toContain('<ResourceOperatorStateSection');
    expect(guestDrawerManageSource).toContain('<ResourceOperatorStateSection');
    expect(guestDrawerSource).toContain("activeTab() === 'manage'");
    expect(guestDrawerManageSource).toContain('data-testid="guest-manage-tab"');
  });

  it('opens with additive context and the active problem before technical inventory', () => {
    const { container } = solidRender(() => (
      <GuestDrawer
        guest={makeGuest({
          osName: 'Debian',
          osVersion: '12',
          ipAddresses: ['192.0.2.25'],
        })}
        alerts={[
          {
            id: 'alert-memory',
            type: 'memory',
            level: 'critical',
            resourceId: 'inst1-node1-100',
            resourceName: 'test-vm',
            node: 'node1',
            instance: 'inst1',
            message: 'Memory usage has remained above 95%',
            value: 96,
            threshold: 95,
            startTime: new Date().toISOString(),
            acknowledged: false,
          },
        ]}
        onClose={vi.fn()}
      />
    ));

    expect(screen.getByText('Needs attention')).toBeInTheDocument();
    expect(screen.getByText('Memory usage has remained above 95%')).toBeInTheDocument();
    expect(screen.getAllByText(/Debian.*12/).length).toBeGreaterThan(0);
    expect(screen.getAllByText('192.0.2.25').length).toBeGreaterThan(0);
    expect(container.querySelector('[data-testid="guest-technical-details"] table')).toBeTruthy();
    expect(screen.queryByTestId('guest-manage-tab')).not.toBeInTheDocument();
  });

  it('explains guest-allocation alerts without translating them from another memory sample', () => {
    solidRender(() => (
      <GuestDrawer
        guest={makeGuest({
          memory: {
            total: 3 * 1024 ** 3,
            used: 2.52 * 1024 ** 3,
            free: 0.48 * 1024 ** 3,
            usage: 84,
          },
        })}
        memoryDisplayBasis="host"
        parentMemoryTotal={15.4 * 1024 ** 3}
        alerts={[
          {
            id: 'alert-memory-host-comparison',
            type: 'memory',
            level: 'warning',
            resourceId: 'inst1-node1-100',
            resourceName: 'test-vm',
            node: 'node1',
            instance: 'inst1',
            message: 'VM memory at 86.6%',
            value: 86.6,
            threshold: 85,
            startTime: new Date().toISOString(),
            acknowledged: false,
          },
        ]}
        onClose={vi.fn()}
      />
    ));

    expect(screen.getByText('VM memory at 86.6%')).toBeInTheDocument();
    expect(
      screen.getByText('Alert percentages use guest allocation. The table row uses host capacity.'),
    ).toBeInTheDocument();
    expect(screen.getByTestId('drawer-attention-section')).not.toHaveTextContent(
      '16% of host capacity',
    );
  });

  it('leads with the live reading when an open alert holds below its trigger', () => {
    solidRender(() => (
      <GuestDrawer
        guest={makeGuest()}
        alerts={[
          {
            id: 'alert-memory-holding',
            type: 'memory',
            level: 'warning',
            resourceId: 'inst1-node1-100',
            resourceName: 'test-vm',
            node: 'node1',
            instance: 'inst1',
            message: 'VM memory at 91.0%',
            value: 91,
            threshold: 90,
            startTime: new Date().toISOString(),
            acknowledged: false,
            metricStatus: {
              phase: 'latched',
              value: 87,
              unit: '%',
              observedAt: new Date().toISOString(),
              trigger: 90,
              recovery: 85,
              recoveryDelaySeconds: 300,
            },
          },
        ]}
        onClose={vi.fn()}
      />
    ));

    expect(screen.getByText('Memory 87% now, back under the 90% alert level')).toBeInTheDocument();
    expect(
      screen.getByText('Stays open until it reaches 85% or lower and stays there for 5 minutes.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('VM memory at 91.0%')).not.toBeInTheDocument();
  });

  it('keeps generic Assistant and context-copy actions out of the drawer header', () => {
    render(() => <GuestDrawer guest={makeGuest({ name: 'homeassistant' })} onClose={vi.fn()} />);

    expect(
      screen.queryByRole('button', { name: 'Ask Pulse Assistant about homeassistant' }),
    ).toBeNull();
    expect(
      screen.queryByRole('button', { name: 'Copy Pulse context for homeassistant' }),
    ).toBeNull();
  });

  it('collapses from the full shared drawer header surface', async () => {
    const onClose = vi.fn();
    render(() => <GuestDrawer guest={makeGuest({ name: 'homeassistant' })} onClose={onClose} />);

    await fireEvent.click(screen.getByRole('button', { name: 'Collapse homeassistant details' }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // ── Tabs ──

  describe('tab switching', () => {
    it('renders Overview and Manage tabs, with the charts on Overview instead of a History tab', () => {
      render(() => <GuestDrawer guest={makeGuestWithDiscoveryTarget()} onClose={vi.fn()} />);
      expect(screen.getByRole('tab', { name: 'Overview' })).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: 'Manage' })).toBeInTheDocument();
      expect(screen.queryByRole('tab', { name: 'History' })).toBeNull();
      expect(screen.queryByRole('tab', { name: 'Discovery' })).toBeNull();
    });

    it('starts on the Overview tab', () => {
      const { container } = solidRender(() => (
        <GuestDrawer guest={makeGuestWithDiscoveryTarget()} onClose={vi.fn()} />
      ));
      const panels = container.querySelectorAll('[class*="overflow-anchor"]');
      expect(panels[0]).not.toHaveClass('hidden');
      expect(panels[1]).toHaveClass('hidden');
    });

    it('renders the metric charts on the Overview tab', async () => {
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);

      await fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));

      await waitFor(() => {
        expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledWith(
          expect.objectContaining({
            resourceType: 'vm',
            resourceId: 'inst1:node1:100',
            range: '24h',
          }),
        );
      });
      const charts = screen.getAllByTestId('guest-history-group-chart');
      expect(charts).toHaveLength(3);
      expect(charts[0].dataset.historyGroup).toBe('utilization');
      expect(charts[1].dataset.historyGroup).toBe('network');
      expect(charts[2].dataset.historyGroup).toBe('disk-io');
      expect(screen.getByText('Utilization')).toBeInTheDocument();
      expect(screen.getByText('Network I/O')).toBeInTheDocument();
      expect(screen.getByText('Disk I/O')).toBeInTheDocument();
      expect(screen.getByTestId('guest-history-range-control')).toBeInTheDocument();
      expect(screen.queryByText('Open related infrastructure')).toBeNull();
    });

    it('adds one min-max band per series whose buckets spread, and none for flat ones', async () => {
      const points = makeHistoryPoints(10);
      chartsApiMocks.getMetricsHistory.mockResolvedValue({
        resourceType: 'vm',
        resourceId: 'inst1:node1:100',
        range: '24h',
        start: 1,
        end: 3,
        source: 'store',
        metrics: {
          cpu: [points[0], { ...points[1], max: 95 }, points[2]],
          memory: makeHistoryPoints(20),
          disk: makeHistoryPoints(30),
        },
      });
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      await fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
      await waitFor(() =>
        expect(
          screen
            .getAllByTestId('guest-history-group-chart')[0]
            .querySelector('path[data-history-band="cpu"]'),
        ).not.toBeNull(),
      );
      const charts = screen.getAllByTestId('guest-history-group-chart');
      expect(charts[0].querySelectorAll('path[data-history-band]')).toHaveLength(1);
      expect(charts[0].querySelectorAll('path[data-history-series]')).toHaveLength(3);
      expect(charts[1].querySelectorAll('path[data-history-band]')).toHaveLength(0);
    });

    it('shows current readings without fabricating history points', async () => {
      chartsApiMocks.getMetricsHistory.mockResolvedValue({
        resourceType: 'vm',
        resourceId: 'inst1:node1:100',
        range: '24h',
        start: 0,
        end: 0,
        metrics: {},
        source: 'store',
      });
      render(() => <GuestDrawer guest={makeGuest({ cpu: 0.25 })} onClose={vi.fn()} />);

      await fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
      await waitFor(() =>
        expect(screen.getAllByTestId('guest-history-group-chart')[0]).toHaveTextContent(
          'No stored history in this range',
        ),
      );

      const utilizationChart = screen.getAllByTestId('guest-history-group-chart')[0];
      expect(utilizationChart).toHaveTextContent('CPU25.0%current');
      expect(utilizationChart.querySelector('path[data-history-series]')).toBeNull();
    });

    it('renders unavailable current disk usage as a dash', async () => {
      chartsApiMocks.getMetricsHistory.mockResolvedValue({
        resourceType: 'vm',
        resourceId: 'inst1:node1:100',
        range: '24h',
        start: 0,
        end: 0,
        metrics: {},
        source: 'store',
      });
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            disk: { total: 100, used: 0, free: 100, usage: -1 },
          })}
          onClose={vi.fn()}
        />
      ));

      await fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
      await waitFor(() => expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalled());

      const utilizationChart = screen.getAllByTestId('guest-history-group-chart')[0];
      expect(utilizationChart).toHaveTextContent('Disk-');
      expect(utilizationChart).not.toHaveTextContent('Disk-100.0%');
    });

    it('updates grouped metric header values on History chart hover', async () => {
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);

      await fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));

      await waitFor(() =>
        expect(screen.getAllByTestId('guest-history-group-chart')[0]).toHaveTextContent('40.0%'),
      );
      const utilizationChart = screen.getAllByTestId('guest-history-group-chart')[0];
      const plot = within(utilizationChart).getByTestId('guest-history-plot');
      plot.getBoundingClientRect = () =>
        ({
          bottom: 100,
          height: 100,
          left: 0,
          right: 360,
          top: 0,
          width: 360,
          x: 0,
          y: 0,
          toJSON: () => ({}),
        }) as DOMRect;

      expect(utilizationChart).toHaveTextContent('20.0%');
      expect(utilizationChart).toHaveTextContent('30.0%');
      expect(utilizationChart).toHaveTextContent('40.0%');

      await fireEvent.mouseMove(plot, { clientX: 180, clientY: 50 });

      expect(
        within(utilizationChart).getByTestId('guest-history-hover-time').textContent,
      ).toBeTruthy();
      expect(within(utilizationChart).queryByTestId('guest-history-tooltip')).toBeNull();
      expect(utilizationChart).toHaveTextContent('CPU');
      expect(utilizationChart).toHaveTextContent('15.0%');
      expect(utilizationChart).toHaveTextContent('Memory');
      expect(utilizationChart).toHaveTextContent('25.0%');
      expect(utilizationChart).toHaveTextContent('Disk');
      expect(utilizationChart).toHaveTextContent('35.0%');

      await fireEvent.pointerLeave(plot);

      expect(within(utilizationChart).getByTestId('guest-history-hover-time')).toHaveTextContent(
        'Latest stored',
      );
      expect(utilizationChart).toHaveTextContent('20.0%');
      expect(utilizationChart).toHaveTextContent('30.0%');
      expect(utilizationChart).toHaveTextContent('40.0%');
    });

    it('switches back to Overview tab', async () => {
      const { container } = render(() => (
        <GuestDrawer guest={makeGuestWithDiscoveryTarget()} onClose={vi.fn()} />
      ));
      await fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
      await fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
      const panels = container.querySelectorAll('[class*="overflow-anchor"]');
      expect(panels[0]).not.toHaveClass('hidden');
      expect(panels[1]).toHaveClass('hidden');
    });
  });

  // ── System card ──

  describe('System details', () => {
    it('displays CPU count', () => {
      render(() => <GuestDrawer guest={makeGuest({ cpus: 8 })} onClose={vi.fn()} />);
      expect(screen.getByText('CPUs')).toBeInTheDocument();
      expect(screen.getByText('8')).toBeInTheDocument();
    });

    it('does not repeat row-level uptime', () => {
      render(() => <GuestDrawer guest={makeGuest({ uptime: 3600 })} onClose={vi.fn()} />);
      expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
    });

    it('hides uptime when 0', () => {
      render(() => <GuestDrawer guest={makeGuest({ uptime: 0 })} onClose={vi.fn()} />);
      expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
    });

    it('does not repeat row-level node placement', () => {
      render(() => <GuestDrawer guest={makeGuest({ node: 'pve-prod-01' })} onClose={vi.fn()} />);
      expect(screen.queryByText('Node')).not.toBeInTheDocument();
      expect(screen.queryByText('pve-prod-01')).not.toBeInTheDocument();
    });
  });

  describe('Nested workload context', () => {
    it('shows nested Docker container count and sample rows in the compact details table', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ name: 'media-lxc', type: 'lxc', workloadType: 'system-container' })}
          nestedWorkloadContext={{
            type: 'app-container',
            label: 'Docker',
            title: 'Nested Docker',
            count: 2,
            href: '/docker/overview?host=media-lxc.mist-stork.ts.net',
            items: [
              { id: 'container-1', name: 'frigate', runtimeLabel: 'Docker', status: 'running' },
              { id: 'container-2', name: 'mosquitto', runtimeLabel: 'Docker', status: 'exited' },
            ],
          }}
          onClose={vi.fn()}
        />
      ));

      const technical = technicalDetails();
      expect(technical.getByText('Nested Docker')).toBeInTheDocument();
      expect(technical.getByText('Containers')).toBeInTheDocument();
      expect(technical.getByText('2')).toBeInTheDocument();
      expect(technical.getByText('frigate')).toBeInTheDocument();
      expect(technical.getByText('mosquitto')).toBeInTheDocument();
    });
  });

  // ── Agent info ──

  describe('Agent info', () => {
    it('shows QEMU agent info for VMs', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: '5.2.0', type: 'qemu' })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Guest agent')).toBeInTheDocument();
      expect(technicalDetails().getByText('QEMU 5.2.0')).toBeInTheDocument();
    });

    it('identifies an API-mapped in-guest Pulse Agent without calling it QEMU', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ agentVersion: '6.2.0', agentKind: 'pulse' })}
          onClose={vi.fn()}
        />
      ));

      expect(screen.getByText('Pulse Agent')).toBeInTheDocument();
      expect(technicalDetails().getByText('Pulse 6.2.0')).toHaveAttribute(
        'title',
        'Pulse Agent 6.2.0',
      );
      expect(screen.queryByText(/QEMU/)).toBeNull();
    });

    it('shows plain agent version for containers', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: '1.0.0', type: 'lxc' })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Guest agent')).toBeInTheDocument();
      expect(technicalDetails().getByText('1.0.0')).toBeInTheDocument();
    });

    it('identifies when a running VM would benefit from a Pulse agent', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: undefined })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Pulse coverage')).toBeInTheDocument();
      expect(technicalDetails().getByText('Agent recommended')).toBeInTheDocument();
    });

    it('shows connected node agent actions when the workload has an explicit action target', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            agentVersion: undefined,
            node: 'delly',
            type: 'lxc',
            workloadType: 'system-container',
            discoveryTarget: {
              resourceType: 'system-container',
              agentId: 'agent-delly',
              resourceId: '124',
            },
          })}
          onClose={vi.fn()}
        />
      ));

      expect(screen.getByText('Pulse coverage')).toBeInTheDocument();
      expect(technicalDetails().getByText('Node agent assigned')).toHaveAttribute(
        'title',
        'Governed actions are assigned to the Pulse Agent on delly. Assignment is not a current connection check.',
      );
      expect(screen.queryByRole('link', { name: 'Add agent for AI actions' })).toBeNull();
    });

    it('hides agent guidance for app containers without an in-guest agent target', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            agentVersion: undefined,
            type: 'docker-container',
            workloadType: 'app-container',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByText('Guest agent')).not.toBeInTheDocument();
      expect(screen.queryByText('Actions')).not.toBeInTheDocument();
    });

    it('hides agent guidance when the parent node is offline', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ agentVersion: undefined, status: 'running' })}
          parentNodeOnline={false}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByRole('link', { name: 'Add agent for AI actions' })).toBeNull();
    });
  });

  // ── Operator identity context (OS + IPs) ──

  describe('Operator identity context', () => {
    it('shows OS name and version', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: 'Ubuntu', osVersion: '22.04' })}
          onClose={vi.fn()}
        />
      ));
      expect(
        within(screen.getByTestId('guest-drawer-facts')).getByText('Ubuntu • 22.04'),
      ).toBeInTheDocument();
    });

    it('shows OS name only when version is missing', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: 'Debian', osVersion: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(
        within(screen.getByTestId('guest-drawer-facts')).getByText('Debian'),
      ).toBeInTheDocument();
    });

    it('shows OS version only when name is missing', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: undefined, osVersion: '11.0' })}
          onClose={vi.fn()}
        />
      ));
      expect(
        within(screen.getByTestId('guest-drawer-facts')).getByText('11.0'),
      ).toBeInTheDocument();
    });

    it('shows IP addresses', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ ipAddresses: ['192.168.1.10', '10.0.0.5'] })}
          onClose={vi.fn()}
        />
      ));
      expect(technicalDetails().getByText('192.168.1.10')).toBeInTheDocument();
      expect(technicalDetails().getByText('10.0.0.5')).toBeInTheDocument();
    });

    it('hides identity rows when no OS info and no IPs', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: undefined, osVersion: undefined, ipAddresses: [] })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByText('Primary IP')).not.toBeInTheDocument();
    });
  });

  // ── Memory card ──

  describe('Memory card', () => {
    it('shows balloon info when different from total', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
        balloon: 2147483648,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      expect(screen.getByText('Memory')).toBeInTheDocument();
      expect(screen.getByText(/Balloon/)).toBeInTheDocument();
    });

    it('shows swap info when swap is present', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
        swapTotal: 2147483648,
        swapUsed: 1073741824,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      expect(screen.getByText(/Swap/)).toBeInTheDocument();
    });

    it('keeps the Memory card showing primary RAM usage even without balloon or swap', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      // Row-level usage is not repeated; capacity and optional balloon/swap
      // facts remain visible in the compact drawer section.
      expect(screen.getByText('Memory')).toBeInTheDocument();
      expect(screen.queryByText(/Balloon/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Swap/)).not.toBeInTheDocument();
    });

    it('hides balloon when it equals total', () => {
      const memory: Memory = {
        total: 4294967296,
        used: 2147483648,
        free: 2147483648,
        usage: 0.5,
        balloon: 4294967296,
      };
      render(() => <GuestDrawer guest={makeGuest({ memory })} onClose={vi.fn()} />);
      expect(screen.queryByText(/Balloon/)).not.toBeInTheDocument();
    });
  });

  // ── Backup card ──

  describe('Backup card', () => {
    beforeEach(() => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-03-02T12:00:00Z'));
    });

    it('shows elapsed hours for a recent backup', () => {
      const now = new Date('2026-03-02T10:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: now })} onClose={vi.fn()} />);
      expect(screen.getByText('Last completed backup')).toBeInTheDocument();
      expect(technicalDetails().getByText('2 hours ago')).toBeInTheDocument();
    });

    it('shows elapsed days for a 1-day-old backup', () => {
      const yesterday = new Date('2026-03-01T12:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: yesterday })} onClose={vi.fn()} />);
      expect(technicalDetails().getByText('1d ago')).toBeInTheDocument();
    });

    it('shows "Xd ago" for older backups', () => {
      const fiveDaysAgo = new Date('2026-02-25T12:00:00Z').getTime();
      render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: fiveDaysAgo })} onClose={vi.fn()} />
      ));
      expect(technicalDetails().getByText('5d ago')).toBeInTheDocument();
    });

    it('applies the caution color for an aging backup', () => {
      const tenDaysAgo = new Date('2026-02-20T12:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: tenDaysAgo })} onClose={vi.fn()} />);
      expect(technicalDetails().getByText('10d ago')).toBeInTheDocument();
      const ageCell = technicalDetails().getByText('10d ago').closest('td');
      expect(ageCell).toHaveClass('text-amber-700');
    });

    it('keeps an old existing backup in the caution color rather than red', () => {
      const fortyDaysAgo = new Date('2026-01-21T12:00:00Z').getTime();
      render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: fortyDaysAgo })} onClose={vi.fn()} />
      ));
      expect(technicalDetails().getByText('40d ago')).toBeInTheDocument();
      const ageCell = technicalDetails().getByText('40d ago').closest('td');
      expect(ageCell).toHaveClass('text-amber-700');
      expect(ageCell).not.toHaveClass('text-rose-700');
    });

    it('applies green color for recent backups', () => {
      const now = new Date('2026-03-02T10:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: now })} onClose={vi.fn()} />);
      const ageCell = technicalDetails().getByText('2 hours ago').closest('td');
      expect(ageCell).toHaveClass('text-emerald-700');
    });

    it.each(['qemu', 'lxc'] as const)(
      'keeps absent completed protection visible while a %s backup runs',
      (type) => {
        solidRender(() => (
          <GuestDrawer
            guest={makeGuest({ type, lastBackup: 0, backupInProgress: true })}
            onClose={vi.fn()}
          />
        ));
        expect(technicalDetails().getByText('Last completed backup')).toBeInTheDocument();
        expect(technicalDetails().getByText('No completed backup found').closest('td')).toHaveClass(
          'text-rose-700',
        );
        expect(technicalDetails().getByText('Backup activity')).toBeInTheDocument();
        expect(
          technicalDetails().getByText('Running · not completed yet').closest('td'),
        ).toHaveClass('text-amber-700');
      },
    );

    it.each([
      ['2026-02-20T12:00:00Z', '10d ago', 'text-amber-700'],
      ['2026-03-02T10:00:00Z', '2 hours ago', 'text-emerald-700'],
    ])('preserves completed backup age and tone during a new backup (%s)', (date, age, tone) => {
      solidRender(() => (
        <GuestDrawer
          guest={makeGuest({ lastBackup: new Date(date).getTime(), backupInProgress: true })}
          onClose={vi.fn()}
        />
      ));
      expect(technicalDetails().getByText(age).closest('td')).toHaveClass(tone);
      expect(technicalDetails().getByText('Running · not completed yet').closest('td')).toHaveClass(
        'text-amber-700',
      );
    });

    it('never treats activity stopping as a completed backup and updates only on completion evidence', () => {
      const [guest, setGuest] = createSignal(makeGuest({ lastBackup: 0 }));
      const { container } = solidRender(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
      const drawer = container.querySelector('[data-testid="guest-technical-details"]');
      expect(drawer).not.toBeNull();
      expect(technicalDetails().queryByText('Backup activity')).not.toBeInTheDocument();
      setGuest({ ...guest(), backupInProgress: true });
      expect(technicalDetails().getByText('Running · not completed yet')).toBeInTheDocument();
      expect(technicalDetails().getByText('No completed backup found')).toBeInTheDocument();
      setGuest({ ...guest(), backupInProgress: false });
      expect(technicalDetails().queryByText('Backup activity')).not.toBeInTheDocument();
      expect(technicalDetails().getByText('No completed backup found')).toBeInTheDocument();
      setGuest({ ...guest(), lastBackup: new Date('2026-03-02T10:00:00Z').getTime() });
      expect(technicalDetails().queryByText('No completed backup found')).not.toBeInTheDocument();
      expect(technicalDetails().getByText('2 hours ago').closest('td')).toHaveClass(
        'text-emerald-700',
      );
      expect(container.querySelector('[data-testid="guest-technical-details"]')).toBe(drawer);
    });

    it('reports missing completed protection rather than hiding it when lastBackup is 0', () => {
      solidRender(() => <GuestDrawer guest={makeGuest({ lastBackup: 0 })} onClose={vi.fn()} />);
      expect(technicalDetails().getByText('No completed backup found')).toBeInTheDocument();
      expect(technicalDetails().queryByText('Backup activity')).not.toBeInTheDocument();
    });
  });

  // ── Facts strip ──

  describe('Facts strip', () => {
    it('states the amounts the chart percentages are of', () => {
      const GiB = 1024 ** 3;
      solidRender(() => (
        <GuestDrawer
          guest={makeGuest({
            cpus: 4,
            memory: {
              total: 4 * GiB,
              used: GiB,
              free: 3 * GiB,
              usage: 25,
              observation: {
                state: 'current',
                source: 'status-mem',
                observedAt: new Date().toISOString(),
              },
            },
            disk: { total: 100 * GiB, used: 25 * GiB, free: 75 * GiB, usage: 25 },
          })}
          onClose={vi.fn()}
        />
      ));
      const facts = screen.getByTestId('guest-drawer-facts');
      expect(facts).toHaveTextContent('4 vCPU');
      expect(facts).toHaveTextContent(/1\.00 GB of 4\.00 GB RAM/);
      expect(facts).toHaveTextContent(/25\.0 GB of 100 GB disk/);
    });

    it('states only the memory size for a retained reading, not a used amount', () => {
      const GiB = 1024 ** 3;
      solidRender(() => (
        <GuestDrawer
          guest={makeGuest({
            memory: {
              total: 4 * GiB,
              used: GiB,
              free: 3 * GiB,
              usage: 25,
              observation: { state: 'last-known', source: 'guest-agent-meminfo' },
            },
          })}
          onClose={vi.fn()}
        />
      ));
      const facts = screen.getByTestId('guest-drawer-facts');
      expect(facts).toHaveTextContent('4.00 GB RAM');
      expect(facts).not.toHaveTextContent(/of 4\.00 GB RAM/);
    });

    it('shows no memory amount when the memory reading is withdrawn', () => {
      solidRender(() => (
        <GuestDrawer
          guest={makeGuest({
            memory: { total: 100, used: 0, free: 100, usage: 0, usageUnavailable: true },
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('guest-drawer-facts')).not.toHaveTextContent('RAM');
    });
  });

  // ── Tags card ──

  describe('Tags card', () => {
    const tagChips = () =>
      Array.from(
        screen.getByTestId('guest-drawer-facts').querySelectorAll('span.rounded-full'),
      ).map((chip) => chip.textContent);

    it('renders tags from an array', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: ['production', 'web'] })} onClose={vi.fn()} />
      ));
      expect(tagChips()).toEqual(['production', 'web']);
    });

    it('renders tags from a comma-separated string', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: 'db,critical' as any })} onClose={vi.fn()} />
      ));
      expect(tagChips()).toEqual(['db', 'critical']);
    });

    it('trims whitespace from tags', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: ' spaced , padded ' as any })} onClose={vi.fn()} />
      ));
      expect(tagChips()).toEqual(['spaced', 'padded']);
    });

    it('renders no tag chips when tags is null', () => {
      render(() => <GuestDrawer guest={makeGuest({ tags: null })} onClose={vi.fn()} />);
      expect(screen.queryByText('Tags')).not.toBeInTheDocument();
      expect(
        screen.queryByTestId('guest-drawer-facts')?.querySelectorAll('span.rounded-full') ?? [],
      ).toHaveLength(0);
    });

    it('renders no tag chips when tags is empty array', () => {
      render(() => <GuestDrawer guest={makeGuest({ tags: [] })} onClose={vi.fn()} />);
      expect(
        screen.queryByTestId('guest-drawer-facts')?.querySelectorAll('span.rounded-full') ?? [],
      ).toHaveLength(0);
    });
  });

  // ── Filesystems card ──

  describe('Filesystems card', () => {
    it('renders compact filesystem usage rows when disks are present', () => {
      const disks: Disk[] = [
        { total: 10737418240, used: 5368709120, free: 5368709120, usage: 0.5, mountpoint: '/' },
      ];
      render(() => <GuestDrawer guest={makeGuest({ disks })} onClose={vi.fn()} />);
      expect(screen.getByText('Filesystems')).toBeInTheDocument();
      expect(technicalDetails().getByText('/')).toBeInTheDocument();
      expect(technicalDetails().getByText(/50% · 5\.00 GB\/10\.0 GB/)).toBeInTheDocument();
      const progress = technicalDetails().getByRole('progressbar', {
        name: 'Filesystem / utilization',
      });
      expect(progress).toHaveAttribute('aria-valuenow', '50');
      expect(progress.querySelector('[data-progress-fill="true"]')).toHaveAttribute('width', '50');
    });

    it('does not render a misleading progress bar when filesystem usage is unavailable', () => {
      const disks: Disk[] = [
        { total: 10737418240, used: 0, usage: -1, mountpoint: '/mnt/config-only' },
      ];
      render(() => <GuestDrawer guest={makeGuest({ disks })} onClose={vi.fn()} />);

      expect(technicalDetails().getByText('— · ?/10.0 GB')).toBeInTheDocument();
      expect(
        technicalDetails().queryByRole('progressbar', {
          name: 'Filesystem /mnt/config-only utilization',
        }),
      ).not.toBeInTheDocument();
    });

    it('hides Filesystems card when disks is empty', () => {
      render(() => <GuestDrawer guest={makeGuest({ disks: [] })} onClose={vi.fn()} />);
      expect(screen.queryByText('Filesystems')).not.toBeInTheDocument();
    });

    it('hides Filesystems card when disks is undefined', () => {
      render(() => <GuestDrawer guest={makeGuest({ disks: undefined })} onClose={vi.fn()} />);
      expect(screen.queryByText('Filesystems')).not.toBeInTheDocument();
    });
  });

  // ── Network card ──

  describe('Network card', () => {
    it('renders network interfaces with name and traffic', () => {
      const networkInterfaces: GuestNetworkInterface[] = [
        {
          name: 'eth0',
          mac: 'aa:bb:cc:dd:ee:ff',
          rxBytes: 1024,
          txBytes: 2048,
          addresses: ['192.168.1.5'],
        },
      ];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('Network')).toBeInTheDocument();
      expect(screen.getByText('eth0')).toBeInTheDocument();
      const interfaceDetails = technicalDetails().getByText(
        /192\.168\.1\.5.*MAC aa:bb:cc:dd:ee:ff/,
      );
      expect(interfaceDetails).toHaveTextContent('RX 1.00 KB / TX 2.00 KB');
    });

    it('displays "interface" as fallback when name is missing', () => {
      const networkInterfaces: GuestNetworkInterface[] = [{ rxBytes: 100, txBytes: 200 }];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('Interface 1')).toBeInTheDocument();
    });

    it('limits displayed interfaces to 4', () => {
      const networkInterfaces: GuestNetworkInterface[] = Array.from({ length: 6 }, (_, i) => ({
        name: `eth${i}`,
        rxBytes: 0,
        txBytes: 0,
      }));
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('eth0')).toBeInTheDocument();
      expect(screen.getByText('eth3')).toBeInTheDocument();
      expect(screen.queryByText('eth4')).not.toBeInTheDocument();
      expect(screen.queryByText('eth5')).not.toBeInTheDocument();
    });

    it('hides Network card when no interfaces', () => {
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces: [] })} onClose={vi.fn()} />);
      expect(screen.queryByText('Network')).not.toBeInTheDocument();
    });

    it('hides traffic line when both rx and tx are 0', () => {
      const networkInterfaces: GuestNetworkInterface[] = [{ name: 'eth0', rxBytes: 0, txBytes: 0 }];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('eth0')).toBeInTheDocument();
      expect(screen.queryByText(/^RX /)).not.toBeInTheDocument();
    });
  });

  // ── In-guest Pulse Agent readings ──

  describe('In-guest Pulse Agent readings', () => {
    // Proxmox read no guest filesystems, so these rows came from the agent.
    const makeAgentGuest = (overrides: Partial<WorkloadGuest> = {}) =>
      makeGuest({
        agentVersion: '6.4.5',
        agentKind: 'pulse',
        disksFromAgent: true,
        diskStatusReason: 'agent-not-running',
        disks: [{ total: 100, used: 75, free: 25, usage: 75, mountpoint: '/' }],
        networkInterfaces: [{ name: 'eth0', addresses: ['192.168.1.5'], rxBytes: 1024 }],
        agentRaid: [
          {
            device: '/dev/md1',
            name: 'data',
            level: 'raid5',
            state: 'clean, degraded, recovering',
            totalDevices: 3,
            activeDevices: 2,
            workingDevices: 3,
            failedDevices: 1,
            spareDevices: 0,
            devices: [{ device: 'sdd1', state: 'faulty', slot: 1 }],
            rebuildPercent: 42,
            rebuildSpeed: '118M/sec',
          },
        ],
        ...overrides,
      });

    it("reads a silent in-guest agent's RAID, network and filesystems as last known", () => {
      render(() => <GuestDrawer guest={makeAgentGuest({ agentStale: true })} onClose={vi.fn()} />);

      expect(technicalDetails().getByText('Pulse 6.4.5 · stopped reporting')).toHaveAttribute(
        'title',
        'Pulse Agent 6.4.5 has stopped reporting. Its readings here are its last report.',
      );
      expect(
        technicalDetails().getByText(
          'Using last known disk stats. The Pulse Agent in this guest stopped reporting.',
        ),
      ).toBeInTheDocument();
      expect(technicalDetails().getByText(/^Last known 75%/)).toBeInTheDocument();
      expect(
        technicalDetails().queryByRole('progressbar', { name: 'Filesystem / utilization' }),
      ).toBeNull();
      expect(
        technicalDetails().getByText(
          'Using last known interface details. The Pulse Agent in this guest stopped reporting.',
        ),
      ).toBeInTheDocument();
      expect(
        technicalDetails()
          .getByText(/^192\.168\.1\.5/)
          .closest('td'),
      ).toHaveClass('text-muted');
      // Proxmox's guest-read reason does not describe rows the agent supplied.
      expect(technicalDetails().queryByText(/qemu-guest-agent/)).toBeNull();

      const array = screen.getByTestId('raid-card-array');
      expect(array).toHaveAttribute('data-raid-reading', 'last-known');
      expect(within(array).getByTestId('raid-card-state')).toHaveAttribute(
        'title',
        'Last known reading, not current: host agent stopped reporting',
      );
      expect(within(array).getByTestId('raid-card-rebuild')).toHaveTextContent(
        'Rebuild was at 42%',
      );
    });

    it('keeps a live in-guest agent current when Proxmox has no guest reads', () => {
      render(() => <GuestDrawer guest={makeAgentGuest()} onClose={vi.fn()} />);

      expect(technicalDetails().getByText('Pulse 6.4.5')).toBeInTheDocument();
      expect(
        technicalDetails().getByRole('progressbar', { name: 'Filesystem / utilization' }),
      ).toHaveAttribute('aria-valuenow', '75');
      expect(technicalDetails().queryByText(/Last known|qemu-guest-agent/)).toBeNull();
      expect(screen.getByTestId('raid-card-array')).toHaveAttribute('data-raid-reading', 'current');
      expect(screen.getByTestId('raid-card-rebuild')).toHaveTextContent('Rebuild: 42% · 118M/sec');
    });

    it("keeps Proxmox's own guest filesystems current under a silent agent", () => {
      render(() => (
        <GuestDrawer
          guest={makeAgentGuest({
            agentStale: true,
            disksFromAgent: undefined,
            diskStatusReason: undefined,
          })}
          onClose={vi.fn()}
        />
      ));

      expect(
        technicalDetails().getByRole('progressbar', { name: 'Filesystem / utilization' }),
      ).toHaveAttribute('aria-valuenow', '75');
      expect(technicalDetails().queryByText(/Using last known disk stats/)).toBeNull();
      expect(screen.getByTestId('raid-card-array')).toHaveAttribute(
        'data-raid-reading',
        'last-known',
      );
    });

    // Proxmox gives vm-stopped only for a guest with allocated disk, so the
    // guest's own stopped status must hold the rows too.
    it.each([
      ['with the Proxmox reason', 'vm-stopped'],
      ['without a Proxmox reason', undefined],
    ])(
      "reads a stopped guest's agent filesystems as unavailable before the lease lapses, %s",
      (_label, diskStatusReason) => {
        render(() => (
          <GuestDrawer
            guest={makeAgentGuest({ status: 'stopped', diskStatusReason })}
            onClose={vi.fn()}
          />
        ));

        expect(
          technicalDetails().getByText(
            'Guest filesystem stats unavailable while the VM is stopped.',
          ),
        ).toBeInTheDocument();
        expect(technicalDetails().getByText(/^Usage unavailable/)).toBeInTheDocument();
        expect(
          technicalDetails().queryByRole('progressbar', { name: 'Filesystem / utilization' }),
        ).toBeNull();
      },
    );

    it('follows the agent between silent and reporting while the drawer stays open', () => {
      const [guest, setGuest] = createSignal(makeAgentGuest());
      render(() => <GuestDrawer guest={guest()} onClose={vi.fn()} />);
      expect(screen.getByTestId('raid-card-array')).toHaveAttribute('data-raid-reading', 'current');

      setGuest({ ...guest(), agentStale: true });
      expect(screen.getByTestId('raid-card-array')).toHaveAttribute(
        'data-raid-reading',
        'last-known',
      );
      expect(technicalDetails().getByText('Pulse 6.4.5 · stopped reporting')).toBeInTheDocument();
      expect(technicalDetails().getByText(/^Last known 75%/)).toBeInTheDocument();

      setGuest({ ...guest(), agentStale: undefined });
      expect(screen.getByTestId('raid-card-array')).toHaveAttribute('data-raid-reading', 'current');
      expect(technicalDetails().queryByText(/stopped reporting|Last known/)).toBeNull();
    });
  });

  // ── WebInterfaceUrlField ──

  describe('WebInterfaceUrlField', () => {
    it('keeps the guest metadata key separate from canonical row identity', () => {
      render(() => <GuestDrawer guest={makeGuest({ id: 'my-guest-id' })} onClose={vi.fn()} />);
      expect(screen.getByTestId('url-id').textContent).toBe(
        getWorkloadMetadataId(makeGuest({ id: 'my-guest-id' })),
      );
    });

    it('passes stable app-container metadataId for Docker-managed workload URLs', () => {
      const guest = makeGuest({
        id: 'app-container-synthetic-hash',
        name: 'grafana',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'docker-main',
        containerId: 'container-runtime-id',
      });

      render(() => <GuestDrawer guest={guest} onClose={vi.fn()} />);

      expect(screen.getByTestId('url-id').textContent).toBe(getWorkloadMetadataId(guest));
      expect(screen.getByTestId('url-id').textContent).toBe(
        'app-container:docker-main:name:grafana',
      );
    });

    it('does not invent a resource identity for a guest with no ID', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ id: '', instance: 'pve', node: 'n1', vmid: 200 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByTestId('url-id')).not.toBeInTheDocument();
    });

    it('labels app-container guests as "container"', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ workloadType: 'app-container' })} onClose={vi.fn()} />
      ));
      expect(screen.getByTestId('url-label').textContent).toBe('container');
    });

    it('labels pod guests as "workload"', () => {
      render(() => <GuestDrawer guest={makeGuest({ workloadType: 'pod' })} onClose={vi.fn()} />);
      expect(screen.getByTestId('url-label').textContent).toBe('workload');
    });

    it('labels VM guests as "workload"', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'qemu', workloadType: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('url-label').textContent).toBe('workload');
    });

    it('labels LXC guests as "workload"', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'lxc', workloadType: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('url-label').textContent).toBe('workload');
    });
  });

  describe('header web interface action', () => {
    it('opens the saved URL from the header without collapsing the drawer', () => {
      const onClose = vi.fn();
      render(() => (
        <GuestDrawer guest={makeGuest()} customUrl="https://auth.home.example" onClose={onClose} />
      ));

      const open = screen.getByRole('link', { name: 'Open web interface for test-vm' });
      expect(open).toHaveAttribute('href', 'https://auth.home.example');
      expect(open).toHaveAttribute('target', '_blank');
      expect(open).toHaveAttribute('rel', 'noopener noreferrer');
      expect(open).toHaveTextContent('Open');
      expect(open.className).toContain('min-h-11');

      fireEvent.click(open);
      expect(onClose).not.toHaveBeenCalled();
    });

    it.each([undefined, '   '])(
      'renders no header action without a saved URL (%j)',
      (customUrl) => {
        render(() => <GuestDrawer guest={makeGuest()} customUrl={customUrl} onClose={vi.fn()} />);

        expect(
          screen.queryByRole('link', { name: 'Open web interface for test-vm' }),
        ).not.toBeInTheDocument();
      },
    );

    it('never turns an unsafe saved URL into a header link', () => {
      const unsafeUrl = 'data:text/html,<script>alert(1)</script>';
      render(() => <GuestDrawer guest={makeGuest()} customUrl={unsafeUrl} onClose={vi.fn()} />);

      expect(
        screen.queryByRole('link', { name: 'Open web interface for test-vm' }),
      ).not.toBeInTheDocument();
      expect(screen.getByRole('img', { name: 'Invalid web interface URL' })).toBeInTheDocument();
    });
  });
});

it('forwards the mounted guest clock without changing the evaluated breach or clear rule', () => {
  const at = '2026-10-10T12:00:00Z';
  const alert = {
    id: 'held-cpu',
    resourceId: 'inst1-node1-100',
    resourceName: 'test-vm',
    node: 'node1',
    instance: 'inst1',
    type: 'cpu',
    message: 'Old breach',
    level: 'warning',
    value: 92,
    threshold: 80,
    startTime: at,
    acknowledged: false,
    metricStatus: {
      phase: 'latched',
      value: 72,
      unit: '%',
      observedAt: at,
      lastBreachAt: '2026-10-10T11:40:00Z',
      trigger: 80,
      recovery: 70,
    },
  } as Alert;
  const now = vi.fn(() => Date.parse(at) + 11 * 60_000);
  const copy = getGuestDrawerAlertAttention(
    alert,
    { guest: makeGuest(), memoryDisplayBasis: 'guest' },
    now,
  );
  expect(copy.message).toBe('Last reading: CPU 72%, 11 mins ago');
  expect(copy.detail).toBe('Stays open until it reaches 70% or lower.');
  expect(copy.title).toContain('Last reading at or above 80%: 92%, 31 mins ago');
  expect(now).toHaveBeenCalledOnce();
});
