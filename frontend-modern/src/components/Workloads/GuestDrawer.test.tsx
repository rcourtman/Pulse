import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor, within } from '@solidjs/testing-library';
import { Suspense } from 'solid-js';
import type { WorkloadGuest } from '@/types/workloads';
import type { Memory, Disk, GuestNetworkInterface } from '@/types/api';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { getDiscoveryProvenanceTitle } from '@/utils/discoveryPresentation';

// ── Mocks ──────────────────────────────────────────────────────────────

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

vi.mock('./DiskList', () => ({
  DiskList: (props: { disks: Disk[] }) => (
    <div data-testid="disk-list">DiskList({props.disks.length} disks)</div>
  ),
}));

vi.mock('../Discovery/DiscoveryTab', () => ({
  DiscoveryTab: (props: {
    resourceType: string;
    agentId?: string;
    resourceId: string;
    hostname: string;
    showManualRunAction?: boolean;
  }) => (
    <div data-testid="discovery-tab">
      <span data-testid="disc-resource-type">{props.resourceType}</span>
      <span data-testid="disc-agent-id">{props.agentId}</span>
      <span data-testid="disc-resource-id">{props.resourceId}</span>
      <span data-testid="disc-manual-run-action">{String(props.showManualRunAction)}</span>
    </div>
  ),
}));

const discoveryApiMocks = vi.hoisted(() => ({
  getDiscovery: vi.fn(
    async (..._args: unknown[]): Promise<import('@/types/discovery').ResourceDiscovery | null> =>
      null,
  ),
}));

vi.mock('@/api/discovery', async () => {
  const actual = await vi.importActual<typeof import('@/api/discovery')>('@/api/discovery');
  return {
    ...actual,
    getDiscovery: discoveryApiMocks.getDiscovery,
  };
});

vi.mock('@/components/shared/WebInterfaceUrlField', () => ({
  WebInterfaceUrlField: (props: {
    metadataKind: string;
    metadataId: string;
    targetLabel: string;
    suggestedUrl?: string;
    suggestedUrlReasonText?: string;
    suggestedUrlDiagnostic?: string;
  }) => (
    <div data-testid="web-interface-url-field">
      <span data-testid="url-kind">{props.metadataKind}</span>
      <span data-testid="url-id">{props.metadataId}</span>
      <span data-testid="url-label">{props.targetLabel}</span>
      <span data-testid="url-suggested">{props.suggestedUrl ?? ''}</span>
      <span data-testid="url-suggested-reason">{props.suggestedUrlReasonText ?? ''}</span>
      <span data-testid="url-suggested-diagnostic">{props.suggestedUrlDiagnostic ?? ''}</span>
    </div>
  ),
}));

// After mocks, import the component under test
import { GuestDrawer } from './GuestDrawer';

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

const makeHistoryPoints = (base: number) => [
  { timestamp: 1, value: base, min: base, max: base },
  { timestamp: 2, value: base + 5, min: base + 5, max: base + 5 },
  { timestamp: 3, value: base + 10, min: base + 10, max: base + 10 },
];

beforeEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  discoveryApiMocks.getDiscovery.mockResolvedValue(null);
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
  vi.clearAllMocks();
  vi.useRealTimers();
});

// ── Tests ──────────────────────────────────────────────────────────────

describe('GuestDrawer', () => {
  // ── Tabs ──

  describe('tab switching', () => {
    it('renders Overview and Discovery tabs', () => {
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      expect(screen.getByText('Overview')).toBeInTheDocument();
      expect(screen.getByText('History')).toBeInTheDocument();
      expect(screen.getByText('Discovery')).toBeInTheDocument();
    });

    it('hides Discovery when the workload has no canonical discovery target', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            id: 'app-container:truenas-main:nextcloud',
            type: 'app-container',
            workloadType: 'app-container',
            platformType: 'truenas',
            dockerHostId: '',
            node: 'truenas-main',
            instance: 'truenas-main',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Overview')).toBeInTheDocument();
      expect(screen.getByText('History')).toBeInTheDocument();
      expect(screen.queryByText('Discovery')).toBeNull();
      expect(screen.queryByTestId('discovery-tab')).toBeNull();
    });

    it('starts on the Overview tab (discovery content hidden)', () => {
      const { container } = render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      const panels = container.querySelectorAll('[style*="overflow-anchor"]');
      expect(panels[0]).not.toHaveClass('hidden');
      expect(panels[1]).toHaveClass('hidden');
    });

    it('renders the Identified Service overview card when a populated discovery record exists', async () => {
      discoveryApiMocks.getDiscovery.mockResolvedValueOnce({
        id: 'vm:node1:100',
        resource_type: 'vm',
        resource_id: '100',
        target_id: 'node1',
        service_name: 'Homepage Dashboard',
        service_type: 'homepage',
        service_version: '0.9.0',
        category: 'web_server',
        confidence: 0.95,
        cli_access: 'docker exec -it homepage /bin/sh',
        ports: [],
        facts: [],
        config_paths: ['/opt/homepage/config'],
        data_paths: [],
        log_paths: [],
        updated_at: '2026-05-18T09:49:19.049058+01:00',
        suggested_url: 'http://192.0.2.10:3000',
        suggested_url_source_code: 'web_port_inference',
        suggested_url_source_detail: 'detected 3000/tcp',
      } as unknown as import('@/types/discovery').ResourceDiscovery);

      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);

      await waitFor(() => {
        expect(screen.getByText('Identified Service')).toBeInTheDocument();
      });
      expect(screen.getByText('Homepage Dashboard')).toBeInTheDocument();
      expect(screen.getByText('web_server')).toBeInTheDocument();
      expect(screen.getByText('0.9.0')).toBeInTheDocument();
      expect(screen.getByText('95%')).toBeInTheDocument();
      expect(screen.getByLabelText(getDiscoveryProvenanceTitle())).toBeInTheDocument();
      expect(screen.getAllByText('http://192.0.2.10:3000').length).toBeGreaterThan(0);
      expect(screen.getByText('docker exec -it homepage /bin/sh')).toBeInTheDocument();
      expect(screen.getByTestId('url-suggested')).toHaveTextContent('http://192.0.2.10:3000');
      expect(screen.getByTestId('url-suggested-reason')).toHaveTextContent('Detected 3000/tcp');
    });

    it('hides the Identified Service card when the discovery record is null or empty', async () => {
      discoveryApiMocks.getDiscovery.mockResolvedValueOnce(null);
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      await waitFor(() => {
        expect(discoveryApiMocks.getDiscovery).toHaveBeenCalled();
      });
      expect(screen.queryByText('Identified Service')).toBeNull();
    });

    it('hides low-signal placeholder discovery from the Overview card and URL field', async () => {
      discoveryApiMocks.getDiscovery.mockResolvedValueOnce({
        id: 'system-container:pve4:152',
        resource_type: 'system-container',
        resource_id: '152',
        target_id: 'pve4',
        service_name: '',
        service_type: 'service',
        service_version: '',
        category: 'unknown',
        confidence: 0,
        cli_access: 'pct exec 152 -- /bin/bash',
        ports: [],
        facts: [
          {
            category: 'service',
            key: 'status',
            value: 'online',
            source: 'metadata',
            confidence: 1,
            discovered_at: '2026-05-19T00:00:00Z',
          },
          {
            category: 'config',
            key: 'config_availability',
            value: 'missing_node_config',
            source: 'all_commands',
            confidence: 1,
            discovered_at: '2026-05-19T00:00:00Z',
          },
        ],
        config_paths: [],
        data_paths: [],
        log_paths: [],
        updated_at: '2026-05-18T09:49:19.049058+01:00',
        suggested_url_diagnostic: 'no host or IP candidate available',
      } as unknown as import('@/types/discovery').ResourceDiscovery);

      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);

      await waitFor(() => {
        expect(discoveryApiMocks.getDiscovery).toHaveBeenCalled();
      });
      expect(screen.queryByText('Identified Service')).toBeNull();
      expect(screen.getByTestId('url-suggested-diagnostic')).toHaveTextContent('');
    });

    it('keeps the passive discovery lookup out of the parent Suspense fallback', () => {
      discoveryApiMocks.getDiscovery.mockImplementationOnce(
        () => new Promise<import('@/types/discovery').ResourceDiscovery | null>(() => undefined),
      );

      render(() => (
        <Suspense fallback={<div data-testid="parent-suspense-fallback">Loading page</div>}>
          <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />
        </Suspense>
      ));

      expect(screen.queryByTestId('parent-suspense-fallback')).toBeNull();
      expect(screen.getByText('Overview')).toBeInTheDocument();
      expect(screen.getByText('History')).toBeInTheDocument();
      expect(screen.getByText('Discovery')).toBeInTheDocument();
    });

    it('switches to Discovery tab on click', async () => {
      const { container } = render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      await fireEvent.click(screen.getByText('Discovery'));
      const panels = container.querySelectorAll('[style*="overflow-anchor"]');
      expect(panels[0]).toHaveClass('hidden');
      expect(panels[1]).not.toHaveClass('hidden');
    });

    it('renders persistent metric charts on the History tab', async () => {
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      expect(screen.queryByTestId('guest-history-group-chart')).toBeNull();

      await fireEvent.click(screen.getByText('History'));

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

    it('updates grouped metric header values on History chart hover', async () => {
      render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);

      await fireEvent.click(screen.getByText('History'));

      await waitFor(() => {
        expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalled();
      });
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

      expect(within(utilizationChart).queryByTestId('guest-history-hover-time')).toBeNull();
      expect(utilizationChart).toHaveTextContent('20.0%');
      expect(utilizationChart).toHaveTextContent('30.0%');
      expect(utilizationChart).toHaveTextContent('40.0%');
    });

    it('switches back to Overview tab', async () => {
      const { container } = render(() => <GuestDrawer guest={makeGuest()} onClose={vi.fn()} />);
      await fireEvent.click(screen.getByText('Discovery'));
      await fireEvent.click(screen.getByText('Overview'));
      const panels = container.querySelectorAll('[style*="overflow-anchor"]');
      expect(panels[0]).not.toHaveClass('hidden');
      expect(panels[1]).toHaveClass('hidden');
    });
  });

  // ── System card ──

  describe('System card', () => {
    it('displays CPU count', () => {
      render(() => <GuestDrawer guest={makeGuest({ cpus: 8 })} onClose={vi.fn()} />);
      expect(screen.getByText('CPUs')).toBeInTheDocument();
      expect(screen.getByText('8')).toBeInTheDocument();
    });

    it('displays uptime when > 0', () => {
      render(() => <GuestDrawer guest={makeGuest({ uptime: 3600 })} onClose={vi.fn()} />);
      expect(screen.getByText('Uptime')).toBeInTheDocument();
    });

    it('hides uptime when 0', () => {
      render(() => <GuestDrawer guest={makeGuest({ uptime: 0 })} onClose={vi.fn()} />);
      expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
    });

    it('displays node name', () => {
      render(() => <GuestDrawer guest={makeGuest({ node: 'pve-prod-01' })} onClose={vi.fn()} />);
      expect(screen.getByText('Node')).toBeInTheDocument();
      // Node name appears in both overview and discovery mock; use getAllByText
      const nodes = screen.getAllByText('pve-prod-01');
      expect(nodes.length).toBeGreaterThanOrEqual(1);
    });
  });

  // ── Agent info ──

  describe('Agent info', () => {
    it('shows QEMU agent info for VMs', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: '5.2.0', type: 'qemu' })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Agent')).toBeInTheDocument();
      expect(screen.getByText('QEMU 5.2.0')).toBeInTheDocument();
    });

    it('shows plain agent version for containers', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: '1.0.0', type: 'lxc' })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Agent')).toBeInTheDocument();
      expect(screen.getByText('1.0.0')).toBeInTheDocument();
    });

    it('hides agent section when no agentVersion', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ agentVersion: undefined })} onClose={vi.fn()} />
      ));
      expect(screen.queryByText('Agent')).not.toBeInTheDocument();
    });
  });

  // ── Guest Info card (OS + IPs) ──

  describe('Guest Info card', () => {
    it('shows OS name and version', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: 'Ubuntu', osVersion: '22.04' })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Guest Info')).toBeInTheDocument();
      expect(screen.getByText('Ubuntu')).toBeInTheDocument();
      expect(screen.getByText('22.04')).toBeInTheDocument();
    });

    it('shows OS name only when version is missing', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: 'Debian', osVersion: undefined })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Debian')).toBeInTheDocument();
    });

    it('shows OS version only when name is missing', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: undefined, osVersion: '11.0' })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Guest Info')).toBeInTheDocument();
      expect(screen.getByText('11.0')).toBeInTheDocument();
    });

    it('shows IP addresses', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ ipAddresses: ['192.168.1.10', '10.0.0.5'] })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('192.168.1.10')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.5')).toBeInTheDocument();
    });

    it('hides Guest Info card when no OS info and no IPs', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ osName: undefined, osVersion: undefined, ipAddresses: [] })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByText('Guest Info')).not.toBeInTheDocument();
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
      // The Memory card now always surfaces primary RAM usage (Usage / Total /
      // Free) to match the node drawer's memory card; balloon and swap remain
      // optional rows. See commit "show RAM usage in guest drawer Memory card".
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

    it('shows "Today" for a backup from today', () => {
      const now = new Date('2026-03-02T10:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: now })} onClose={vi.fn()} />);
      expect(screen.getByText('Backup')).toBeInTheDocument();
      expect(screen.getByText('Today')).toBeInTheDocument();
    });

    it('shows "Yesterday" for a 1-day-old backup', () => {
      const yesterday = new Date('2026-03-01T12:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: yesterday })} onClose={vi.fn()} />);
      expect(screen.getByText('Yesterday')).toBeInTheDocument();
    });

    it('shows "Xd ago" for older backups', () => {
      const fiveDaysAgo = new Date('2026-02-25T12:00:00Z').getTime();
      render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: fiveDaysAgo })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('5d ago')).toBeInTheDocument();
    });

    it('applies warning color for backups older than 7 days', () => {
      const tenDaysAgo = new Date('2026-02-20T12:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: tenDaysAgo })} onClose={vi.fn()} />);
      expect(screen.getByText('10d ago')).toBeInTheDocument();
      const ageEl = screen.getByText('10d ago');
      expect(ageEl.className).toContain('amber');
    });

    it('applies critical color for backups older than 30 days', () => {
      const fortyDaysAgo = new Date('2026-01-21T12:00:00Z').getTime();
      render(() => (
        <GuestDrawer guest={makeGuest({ lastBackup: fortyDaysAgo })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('40d ago')).toBeInTheDocument();
      const ageEl = screen.getByText('40d ago');
      expect(ageEl.className).toContain('red');
    });

    it('applies green color for recent backups', () => {
      const now = new Date('2026-03-02T10:00:00Z').getTime();
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: now })} onClose={vi.fn()} />);
      const ageEl = screen.getByText('Today');
      expect(ageEl.className).toContain('green');
    });

    it('hides Backup card when lastBackup is 0 (falsy)', () => {
      render(() => <GuestDrawer guest={makeGuest({ lastBackup: 0 })} onClose={vi.fn()} />);
      expect(screen.queryByText('Backup')).not.toBeInTheDocument();
    });
  });

  // ── Tags card ──

  describe('Tags card', () => {
    it('renders tags from an array', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: ['production', 'web'] })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('Tags')).toBeInTheDocument();
      expect(screen.getByText('production')).toBeInTheDocument();
      expect(screen.getByText('web')).toBeInTheDocument();
    });

    it('renders tags from a comma-separated string', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: 'db,critical' as any })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('db')).toBeInTheDocument();
      expect(screen.getByText('critical')).toBeInTheDocument();
    });

    it('trims whitespace from tags', () => {
      render(() => (
        <GuestDrawer guest={makeGuest({ tags: ' spaced , padded ' as any })} onClose={vi.fn()} />
      ));
      expect(screen.getByText('spaced')).toBeInTheDocument();
      expect(screen.getByText('padded')).toBeInTheDocument();
    });

    it('hides Tags card when tags is null', () => {
      render(() => <GuestDrawer guest={makeGuest({ tags: null })} onClose={vi.fn()} />);
      expect(screen.queryByText('Tags')).not.toBeInTheDocument();
    });

    it('hides Tags card when tags is empty array', () => {
      render(() => <GuestDrawer guest={makeGuest({ tags: [] })} onClose={vi.fn()} />);
      expect(screen.queryByText('Tags')).not.toBeInTheDocument();
    });
  });

  // ── Filesystems card ──

  describe('Filesystems card', () => {
    it('renders DiskList when disks are present', () => {
      const disks: Disk[] = [
        { total: 10737418240, used: 5368709120, free: 5368709120, usage: 0.5, mountpoint: '/' },
      ];
      render(() => <GuestDrawer guest={makeGuest({ disks })} onClose={vi.fn()} />);
      expect(screen.getByText('Filesystems')).toBeInTheDocument();
      expect(screen.getByTestId('disk-list')).toBeInTheDocument();
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
      expect(screen.getByText('aa:bb:cc:dd:ee:ff')).toBeInTheDocument();
      expect(screen.getByText('192.168.1.5')).toBeInTheDocument();
      expect(screen.getByText(/^RX /)).toBeInTheDocument();
      expect(screen.getByText(/^TX /)).toBeInTheDocument();
    });

    it('displays "interface" as fallback when name is missing', () => {
      const networkInterfaces: GuestNetworkInterface[] = [{ rxBytes: 100, txBytes: 200 }];
      render(() => <GuestDrawer guest={makeGuest({ networkInterfaces })} onClose={vi.fn()} />);
      expect(screen.getByText('interface')).toBeInTheDocument();
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

  // ── WebInterfaceUrlField ──

  describe('WebInterfaceUrlField', () => {
    it('passes canonical metadataId using shared workload identity', () => {
      render(() => <GuestDrawer guest={makeGuest({ id: 'my-guest-id' })} onClose={vi.fn()} />);
      expect(screen.getByTestId('url-id').textContent).toBe(
        getCanonicalWorkloadId(makeGuest({ id: 'my-guest-id' })),
      );
    });

    it('builds canonical id from instance:node:vmid when id is empty', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ id: '', instance: 'pve', node: 'n1', vmid: 200 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('url-id').textContent).toBe(
        getCanonicalWorkloadId(makeGuest({ id: '', instance: 'pve', node: 'n1', vmid: 200 })),
      );
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

  // ── Discovery tab integration ──

  describe('DiscoveryTab integration', () => {
    it('passes correct resourceType and agentId for VM', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'qemu', node: 'pve1', vmid: 101 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('vm');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('pve1');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('101');
      expect(screen.getByTestId('disc-manual-run-action').textContent).toBe('true');
    });

    it('passes correct resourceType and agentId for app-container', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            workloadType: 'app-container',
            type: 'app-container',
            platformType: 'docker',
            dockerHostId: 'dh-1',
            containerId: 'container-abc-runtime-id',
            id: 'app-container-synthetic-hash',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('app-container');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('dh-1');
      // Discovery routes by the Docker-native containerId, not the canonical
      // synthetic workload id, so the agent can resolve `docker exec <id>`.
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('container-abc-runtime-id');
    });

    it('does not render DiscoveryTab for TrueNAS app-containers without explicit discovery support', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            id: 'app-container:truenas-main:nextcloud',
            workloadType: 'app-container',
            type: 'app-container',
            platformType: 'truenas',
            dockerHostId: '',
            node: 'truenas-main',
            instance: 'truenas-main',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.queryByTestId('discovery-tab')).toBeNull();
    });

    it('honors explicit canonical discovery targets for API-backed app-containers', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            id: 'app-container:truenas-main:nextcloud',
            workloadType: 'app-container',
            type: 'app-container',
            platformType: 'truenas',
            dockerHostId: '',
            discoveryTarget: {
              resourceType: 'app-container',
              agentId: 'truenas-helper',
              resourceId: 'nextcloud',
            },
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByText('Discovery')).toBeInTheDocument();
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('app-container');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('truenas-helper');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('nextcloud');
    });

    it('passes canonical pod resourceType for pod workloads', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({
            workloadType: 'pod',
            kubernetesAgentId: 'k8s-agent-1',
            id: 'k8s:ctx:pod:my-pod',
          })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('pod');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('k8s-agent-1');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('my-pod');
    });

    it('passes system-container as resourceType for LXC', () => {
      render(() => (
        <GuestDrawer
          guest={makeGuest({ type: 'lxc', node: 'pve2', vmid: 200 })}
          onClose={vi.fn()}
        />
      ));
      expect(screen.getByTestId('disc-resource-type').textContent).toBe('system-container');
      expect(screen.getByTestId('disc-agent-id').textContent).toBe('pve2');
      expect(screen.getByTestId('disc-resource-id').textContent).toBe('200');
    });
  });
});
