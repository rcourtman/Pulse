import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal, type ComponentProps, type JSX } from 'solid-js';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import type { WorkloadGuest } from '@/types/workloads';
import type { Resource } from '@/types/resource';

const query = vi.hoisted(() => ({ resources: [] as Resource[] }));
vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    resources: () => query.resources,
    error: () => undefined,
    loading: () => false,
    refetch: vi.fn(),
  }),
}));
vi.mock('@/components/shared/HistoryChart', () => ({
  HistoryChartHoverGroup: (props: { children: JSX.Element }) => <div>{props.children}</div>,
  HistoryChart: (props: { label: string; resourceId: string }) => (
    <div data-testid="history-chart" data-resource-id={props.resourceId}>
      {props.label}
    </div>
  ),
}));
// Resolve the real lazy leaves before assertions; do not make their first
// transform compete with the default findBy deadline in the complete suite.
beforeAll(async () => {
  await import('../GuestPhysicalDisks');
  await import('@/components/Storage/DiskDetail');
});
afterEach(cleanup);
const guest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'fixture:pve-a:101',
  canonicalResourceId: 'vm-resource-101',
  vmid: 101,
  name: 'hardware-guest',
  node: 'pve-a',
  instance: 'fixture',
  type: 'qemu',
  status: 'running',
  cpu: 0,
  cpus: 2,
  memory: { total: 1024, used: 0, free: 1024, usage: 0 },
  disk: { total: 1024, used: 0, free: 1024, usage: 0 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 100,
  template: false,
  lock: '',
  tags: [],
  lastBackup: 0,
  lastSeen: '2026-10-20T12:00:00Z',
  agentKind: 'pulse',
  agentVersion: 'fixture-version',
  agentStale: false,
  agentRaid: [
    {
      device: '/dev/md0',
      name: 'data',
      level: 'raid1',
      state: 'clean',
      totalDevices: 2,
      activeDevices: 2,
      workingDevices: 2,
      failedDevices: 0,
      spareDevices: 0,
      rebuildPercent: 37,
      rebuildSpeed: '100M/sec',
      devices: [
        { device: '/dev/sda', slot: 0, state: 'active sync' },
        { device: '/dev/sdb', slot: 1, state: 'faulty' },
      ],
    },
  ],
  ...overrides,
});
const disk = (): Resource => ({
  id: 'disk-101-a',
  parentId: 'vm-resource-101',
  type: 'physical_disk',
  name: 'Guest SATA',
  displayName: 'Guest SATA',
  platformId: 'fixture',
  platformType: 'proxmox-pve',
  sourceType: 'agent',
  sources: ['agent'],
  status: 'online',
  lastSeen: Date.now(),
  health: { verdict: 'ok', reasons: [] },
  physicalDisk: {
    devPath: '/dev/sda',
    model: 'Guest SATA',
    serial: 'FIXTURE-101-A',
    diskType: 'sata',
    sizeBytes: 1_000_000_000,
    health: 'PASSED',
    temperature: 45,
    smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 0 },
    collection: {
      temperature: { state: 'available', source: 'smartctl' },
      io: { state: 'available', source: 'host_agent' },
    },
  },
});
const props = (value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> => ({
  guest: value,
  guestOsSummary: '',
  agentHeading: 'Pulse Agent',
  agentLabel: 'fixture-version',
  agentTitle: 'Previously observed version',
  hasAgentInfo: true,
  hasFilesystemDetails: false,
  hasNetworkInterfaces: false,
  hasOsInfo: false,
  hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false,
  ipAddresses: [],
  networkInterfaces: [],
  normalizedTags: [],
  backupPresentation: null,
  workloadActionAgentTitle: '',
});
async function expandDisk() {
  const row = await screen.findByTestId('guest-physical-disk');
  (row as HTMLDetailsElement).open = true;
  fireEvent(row, new Event('toggle'));
  await screen.findByTestId('disk-detail-health');
  return row;
}

describe('native power cannot renew in-guest hardware evidence', () => {
  it.each(['paused', 'suspended', 'stopped', 'offline'])(
    'qualifies RAID at %s power with an unexpired agent lease',
    async (status) => {
      query.resources = [disk()];
      const [value, setValue] = createSignal(guest());
      render(() => <GuestDrawerOverview {...props(value())} />);
      const array = screen.getByTestId('raid-card-array');
      expect(array).toHaveAttribute('data-raid-reading', 'current');
      setValue({ ...value(), status });
      expect(array).toHaveAttribute('data-raid-reading', 'last-known');
      expect(within(array).getByText('clean (last known)')).toBeVisible();
      expect(within(array).getByText('Rebuild was at 37%')).toBeVisible();
      expect(within(array).getByText('/dev/sdb · faulty')).toBeVisible();
      expect(array).not.toHaveTextContent('100M/sec');
      expect(array.querySelector('.text-emerald-600')).toBeNull();
      expect(
        screen.getByText(/RAID states are last known, not current\. Proxmox reports this VM/),
      ).toBeVisible();
      expect(value().agentStale).toBe(false);
    },
  );

  it.each(['paused', 'suspended', 'stopped', 'offline'])(
    'qualifies open SMART at %s power without altering the disk source',
    async (status) => {
      query.resources = [disk()];
      const original = JSON.stringify(query.resources);
      const [value, setValue] = createSignal(guest());
      render(() => <GuestDrawerOverview {...props(value())} />);
      const row = await expandDisk();
      expect(within(row).getAllByText('Healthy')).toHaveLength(2);
      const chart = within(row).getAllByTestId('history-chart')[0];
      setValue({ ...value(), status });
      expect(within(row).queryAllByText('Healthy')).toHaveLength(0);
      expect(within(row).getAllByText('Last known')).toHaveLength(2);
      expect(within(row).getByTestId('disk-detail-health')).toHaveTextContent(
        `Proxmox reports this VM ${status === 'offline' ? 'stopped' : status}.`,
      );
      expect(within(row).getByText('Last known temperature')).toBeVisible();
      expect(row.querySelector('[data-temperature-reading]')).toHaveAttribute(
        'data-temperature-reading',
        'last-known',
      );
      expect(row.querySelector('[data-temperature-reading]')).toHaveAttribute(
        'title',
        expect.stringContaining('Proxmox reports this VM'),
      );
      expect(within(row).queryByText('Live I/O (30m)')).not.toBeInTheDocument();
      expect(within(row).getAllByTestId('history-chart')[0]).toBe(chart);
      expect(row.querySelector('.text-green-600')).toBeNull();
      expect(JSON.stringify(query.resources)).toBe(original);
      expect((row as HTMLDetailsElement).open).toBe(true);
      // Clear the parent qualifier without resetting the disk owner or History.
      // This is not independent thaw or filesystem-liveness evidence.
      setValue(guest());
      expect(within(row).getAllByText('Healthy')).toHaveLength(2);
      expect(within(row).queryByText('Last known temperature')).not.toBeInTheDocument();
      expect(row.querySelector('[data-temperature-reading]')).toHaveAttribute(
        'data-temperature-reading',
        'current',
      );
    },
  );

  it.each(['stopped', 'offline'])(
    'qualifies agent SMART and RAID when an LXC becomes %s without expiring its lease',
    async (status) => {
      query.resources = [disk()];
      const original = JSON.stringify(query.resources);
      const [value, setValue] = createSignal(guest({ type: 'lxc' }));
      render(() => <GuestDrawerOverview {...props(value())} />);
      const row = await expandDisk();
      const chart = within(row).getAllByTestId('history-chart')[0];
      setValue({ ...value(), status });
      expect(within(row).queryAllByText('Healthy')).toHaveLength(0);
      expect(within(row).getByTestId('disk-detail-health')).toHaveTextContent(
        'Proxmox reports this container stopped.',
      );
      expect(within(row).getByText('Last known temperature')).toBeVisible();
      const array = screen.getByTestId('raid-card-array');
      expect(array).toHaveAttribute('data-raid-reading', 'last-known');
      expect(array).toHaveTextContent('/dev/sdb · faulty');
      expect(array).toHaveTextContent('Rebuild was at 37%');
      expect(array).not.toHaveTextContent('100M/sec');
      expect(within(row).getAllByTestId('history-chart')[0]).toBe(chart);
      expect((row as HTMLDetailsElement).open).toBe(true);
      expect(JSON.stringify(query.resources)).toBe(original);
      expect(value().agentStale).toBe(false);
      setValue({ ...value(), status: 'running' });
      expect(within(row).getAllByText('Healthy')).toHaveLength(2);
      expect(array).toHaveAttribute('data-raid-reading', 'current');
    },
  );

  it('preserves an independent API disk on a stopped LXC while retaining agent RAID', async () => {
    const source = disk();
    source.sourceType = 'api';
    source.sources = ['proxmox'];
    query.resources = [source];
    render(() => <GuestDrawerOverview {...props(guest({ type: 'lxc', status: 'stopped' }))} />);
    const row = await screen.findByTestId('guest-physical-disk');
    expect(within(row).getByText('Healthy')).toBeVisible();
    expect(screen.getByTestId('raid-card-array')).toHaveAttribute(
      'data-raid-reading',
      'last-known',
    );
  });

  it('keeps reported disk risks and a separately stale disk qualified after running resumes', async () => {
    const source = disk();
    source.physicalDisk!.smart!.pendingSectors = 8;
    source.health = { verdict: 'stale', reasons: [{ code: 'telemetry_stale' }] };
    query.resources = [source];
    const [value, setValue] = createSignal(guest({ status: 'paused' }));
    render(() => <GuestDrawerOverview {...props(value())} />);
    const row = await expandDisk();
    expect(within(row).getAllByText('Last known risk')).toHaveLength(2);
    expect(row).toHaveTextContent('Last reported: Needs Attention');
    setValue(guest());
    await waitFor(() => expect(within(row).getAllByText('Last known risk')).toHaveLength(2));
    expect(within(row).getByText('Last known temperature')).toBeVisible();
    expect(within(row).getByTestId('disk-detail-health')).toHaveTextContent(
      'Disk source stopped reporting.',
    );
  });

  it.each([
    { type: 'lxc', status: 'paused' },
    {
      type: 'vm',
      platformType: 'vmware-vsphere',
      platformScopes: ['vmware-vsphere'],
      status: 'paused',
    },
    { status: 'unknown-private-state' },
  ] as Partial<WorkloadGuest>[])(
    'does not invent a Proxmox power reason from another or unknown native state %j',
    async (overrides) => {
      query.resources = [disk()];
      render(() => <GuestDrawerOverview {...props(guest(overrides))} />);
      const row = await screen.findByTestId('guest-physical-disk');
      expect(within(row).getByText('Healthy')).toBeVisible();
      expect(screen.getByTestId('raid-card-array')).toHaveAttribute('data-raid-reading', 'current');
      expect(screen.queryByText(/Proxmox reports this VM/)).not.toBeInTheDocument();
    },
  );

  it('does not downgrade an explicitly independent API disk because the guest agent stopped', async () => {
    const source = disk();
    source.sourceType = 'api';
    source.sources = ['proxmox'];
    query.resources = [source];
    render(() => <GuestDrawerOverview {...props(guest({ agentStale: true }))} />);
    const row = await screen.findByTestId('guest-physical-disk');
    expect(within(row).getByText('Healthy')).toBeVisible();
    expect(screen.getByTestId('raid-card-array')).toHaveAttribute(
      'data-raid-reading',
      'last-known',
    );
  });
});
