import { createSignal } from 'solid-js';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { StoragePoolDetail } from '@/components/Storage/StoragePoolDetail';
import { StoragePoolDrawer } from '@/components/Storage/StoragePoolDrawer';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

const historySpy = vi.fn();

// The drawer charts with the shared drawer history component; its own suite
// covers the charts. This stub records what the pool asks it to draw.
vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    enabled?: boolean;
    groups?: Array<{ id: string; series: Array<{ metric: string }> }>;
    currentMetrics?: Record<string, number | undefined>;
    alerts?: Record<string, { severity: string; value?: number } | undefined>;
    onRangeChange?: (range: string) => void;
  }) => {
    historySpy(props);
    return (
      <div
        data-testid="drawer-history"
        data-enabled={String(props.enabled !== false)}
        data-range={props.range}
      >
        {props.target?.resourceType}:{props.target?.resourceId}:
        {props.groups?.flatMap((group) => group.series.map((series) => series.metric)).join(',')}
        <button type="button" onClick={() => props.onRangeChange?.('7d')}>
          Pick 7 days
        </button>
      </div>
    );
  },
}));

vi.mock('@/components/Infrastructure/ResourceOperatorStateSection', () => ({
  ResourceOperatorStateSection: (props: { resourceId: string; resourceType?: string }) => (
    <div data-testid="operator-state">
      {props.resourceType}:{props.resourceId}
    </div>
  ),
}));

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

const zfsPool = (overrides: Record<string, unknown> = {}) => ({
  name: 'tank',
  state: 'ONLINE',
  status: 'Healthy',
  scan: 'none',
  readErrors: 0,
  writeErrors: 0,
  checksumErrors: 0,
  devices: [],
  datasets: [],
  ...overrides,
});

const alert = (overrides: Partial<Alert> = {}): Alert => ({
  id: 'alert-1',
  type: 'usage',
  level: 'warning',
  resourceId: 'storage-1',
  resourceName: 'tank',
  node: 'truenas01',
  instance: 'truenas',
  message: 'Storage usage is 91%',
  value: 91,
  threshold: 85,
  startTime: '2026-10-20T00:00:00Z',
  acknowledged: false,
  ...overrides,
});

const renderOverview = (record: () => StorageRecord, props: Record<string, unknown> = {}) =>
  render(() => <StoragePoolDetail record={record()} physicalDisks={[]} {...props} />);

// A Details row's value, read from the shared detail table.
const detailValue = (label: string) =>
  within(screen.getByTestId('storage-pool-technical-details'))
    .getByText(label, { exact: true })
    .closest('tr')
    ?.lastElementChild?.textContent?.trim();

describe('StoragePoolDetail (pool drawer Overview)', () => {
  it('keeps capacity rows in Details without presenting missing readings as an empty pool', () => {
    const [record, setRecord] = createSignal(makeRecord());
    renderOverview(record);

    const details = screen.getByTestId('storage-pool-technical-details') as HTMLDetailsElement;
    expect(details.tagName).toBe('DETAILS');
    expect(details.open).toBe(false);
    expect(detailValue('Usage')).toBe('40%');

    setRecord(
      makeRecord({
        capacity: { totalBytes: 1000, usedBytes: null, freeBytes: null, usagePercent: null },
      }),
    );
    expect(detailValue('Used')).toBe('n/a');
    expect(detailValue('Free')).toBe('n/a');
    expect(detailValue('Total')).toBe('1000 B');
    expect(detailValue('Usage')).toBe('n/a');

    setRecord(
      makeRecord({
        capacity: { totalBytes: 1000, usedBytes: 0, freeBytes: null, usagePercent: null },
      }),
    );
    expect(detailValue('Used')).toBe('0 B');
    expect(detailValue('Free')).toBe('1000 B');
    expect(detailValue('Usage')).toBe('0%');
  });

  it('charts the canonical metrics target at the drawer default range', async () => {
    historySpy.mockClear();
    renderOverview(() =>
      makeRecord({
        metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
        refs: { resourceId: 'storage-legacy-id', platformEntityId: 'truenas-1' },
      }),
    );

    const chart = await screen.findByTestId('drawer-history');
    expect(chart).toHaveTextContent('storage:pool:tank:usage');
    expect(chart).toHaveAttribute('data-range', '24h');
    expect(historySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ currentMetrics: { usage: 40 } }),
    );
    // No range dropdown of the pool's own: the picker is the charts' toolbar.
    expect(screen.queryByRole('combobox', { name: 'Capacity history range' })).toBeNull();
  });

  it('falls back to legacy storage refs, then the record id, for the chart target', async () => {
    const [record, setRecord] = createSignal(
      makeRecord({ refs: { resourceId: 'pve1:local-zfs', platformEntityId: 'cluster-a' } }),
    );
    renderOverview(record);
    expect(await screen.findByTestId('drawer-history')).toHaveTextContent(
      'storage:pve1:local-zfs:usage',
    );
    setRecord(makeRecord());
    expect(screen.getByTestId('drawer-history')).toHaveTextContent('storage:storage-1:usage');
  });

  it('withholds a stale pool usage as a live reading', async () => {
    historySpy.mockClear();
    renderOverview(() => makeRecord({ freshness: 'stale' }));
    await screen.findByTestId('drawer-history');
    expect(historySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        currentMetrics: {},
        deferredMetrics: {
          usage: expect.objectContaining({ lastKnownValue: 40 }),
        },
      }),
    );
    // The facts strip states the size only.
    expect(screen.getByTestId('storage-pool-drawer-facts')).toHaveTextContent('1000 B total');
    expect(screen.getByTestId('storage-pool-drawer-exceptions')).toHaveTextContent(
      'Retained last-known storage values.',
    );
  });

  it('says there are no active alerts, or lists the pool alerts and draws their trigger', async () => {
    historySpy.mockClear();
    const [alerts, setAlerts] = createSignal<Alert[]>([]);
    render(() => <StoragePoolDetail record={makeRecord()} physicalDisks={[]} alerts={alerts()} />);
    expect(screen.getByTestId('drawer-no-attention')).toHaveTextContent('No active alerts');

    setAlerts([alert()]);
    expect(screen.queryByTestId('drawer-no-attention')).toBeNull();
    const attention = screen.getByTestId('drawer-attention-section');
    expect(attention).toHaveTextContent('Storage usage is 91%');
    await screen.findByTestId('drawer-history');
    expect(historySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ alerts: { usage: { severity: 'warning', value: 85 } } }),
    );
  });

  it('lifts ZFS errors, failing devices, disk errors and hot disks into Worth checking', () => {
    renderOverview(() =>
      makeRecord({
        details: {
          type: 'zfspool',
          parentId: 'host-truenas01',
          zfsPool: zfsPool({
            state: 'DEGRADED',
            readErrors: 2,
            writeErrors: 0,
            checksumErrors: 1,
            scan: 'scrub repaired 0B with 3 errors',
            scanDetails: { function: 'scrub', errors: 3 },
            devices: [
              { name: 'sda', type: 'disk', state: 'ONLINE', readErrors: 0 },
              {
                name: 'ata-WDC_WD40EFRX-68N32N0_WD-WCC7K0000000',
                type: 'disk',
                state: 'FAULTED',
                readErrors: 3,
                writeErrors: 0,
                checksumErrors: 1,
                message: 'too many errors',
              },
            ],
          }),
        },
      }),
    );

    const worth = screen.getByTestId('storage-pool-drawer-exceptions');
    expect(within(worth).getByText('Worth checking')).toBeInTheDocument();
    expect(worth).toHaveTextContent('Degraded');
    expect(worth).toHaveTextContent('Errors: R:2 W:0 C:1');
    expect(worth).toHaveTextContent('scrub repaired 0B with 3 errors');
    expect(worth).toHaveTextContent(
      'ata-WDC_WD40EFRX-68N32N0_WD-WCC7K0000000: Faulted, 3R/0W/1C errors, too many errors',
    );
    // A healthy member is not worth checking; it stays in the device list.
    expect(worth).not.toHaveTextContent('sda');
    const devices = screen.getByTestId('storage-pool-devices');
    expect(within(devices).getByText('sda')).toBeInTheDocument();
    // Lifted pool counters move rather than repeat.
    expect(devices).not.toHaveTextContent('Errors: R:2');
  });

  it('wraps dataset names and mountpoints in the closed Details instead of truncating them', () => {
    renderOverview(() =>
      makeRecord({
        details: {
          type: 'zfspool',
          zfsPool: zfsPool({
            datasets: [
              {
                name: 'tank/apps/a-very-long-dataset-name-that-operators-copy',
                type: 'filesystem',
                mountpoint: '/mnt/tank/apps/a-very-long-dataset-name-that-operators-copy',
                usedBytes: 1_073_741_824,
                availableBytes: 2_147_483_648,
                referencedBytes: 536_870_912,
              },
              { name: 'tank/vm-100-disk-0', type: 'volume', usedBytes: 1024 },
            ],
          }),
        },
      }),
    );

    const datasets = screen.getByTestId('storage-pool-datasets');
    const name = within(datasets).getByText(
      'tank/apps/a-very-long-dataset-name-that-operators-copy',
    );
    expect(name).toHaveClass('whitespace-normal');
    expect(name).not.toHaveClass('truncate');
    const value = within(datasets).getByText(/\/mnt\/tank\/apps\/a-very-long/);
    expect(value).toHaveClass('whitespace-normal');
    expect(value).not.toHaveClass('truncate');
    expect(value).toHaveTextContent('Used 1.00 GB · Available 2.00 GB · Referenced 512 MB');
    expect(within(datasets).getByText(/zvol/)).toBeInTheDocument();
    expect(screen.getByText('Datasets (2)')).toBeInTheDocument();
  });

  it('renders UnRAID topology in Details and linked disk facts in front', () => {
    renderOverview(
      () =>
        makeRecord({
          name: 'Tower Array',
          source: {
            platform: 'unraid',
            family: 'onprem',
            origin: 'resource',
            adapterId: 'resource-storage',
          },
          details: {
            type: 'unraid-array',
            platform: 'unraid',
            topology: 'array',
            arrayState: 'STARTED',
          },
        }),
      {
        physicalDisks: [
          {
            id: 'disk1',
            parentId: 'storage-1',
            type: 'physical_disk',
            name: 'Disk 1',
            displayName: 'Disk 1',
            platformId: 'tower',
            platformType: 'unraid',
            sourceType: 'agent',
            status: 'online',
            lastSeen: Date.now(),
            physicalDisk: {
              devPath: '/dev/sdc',
              model: 'Data Disk',
              storageRole: 'data',
              storageGroup: 'unraid-array',
              storageState: 'online',
              sizeBytes: 6_000_000_000_000,
              temperature: 31,
              spunDown: true,
              readCount: 10,
              writeCount: 20,
              errorCount: 16,
            },
          },
        ] as Resource[],
      },
    );

    expect(detailValue('Kind')).toBe('Array');
    expect(detailValue('Data disks')).toBe('1 disk');
    const disks = screen.getByTestId('storage-pool-linked-disks');
    expect(within(disks).getByText('Physical disks (1)')).toBeInTheDocument();
    expect(within(disks).getByText('/dev/sdc')).toBeInTheDocument();
    expect(within(disks).getByText('Data Disk')).toBeInTheDocument();
    expect(within(disks).getByText('data')).toBeInTheDocument();
    expect(within(disks).getByText('R 10 / W 20')).toBeInTheDocument();
    expect(within(disks).getByText('spun down')).toBeInTheDocument();
    expect(within(disks).getByText('16 errors')).toBeInTheDocument();
    expect(screen.getByTestId('storage-pool-drawer-exceptions')).toHaveTextContent(
      '/dev/sdc: 16 errors',
    );
  });

  it('orders the Overview like the guest and node drawers', async () => {
    renderOverview(() => makeRecord());
    const overview = screen.getByTestId('storage-pool-drawer-overview');
    const order = [
      screen.getByTestId('drawer-no-attention'),
      await screen.findByTestId('drawer-history'),
      screen.getByTestId('storage-pool-drawer-facts'),
      screen.getByTestId('storage-pool-technical-details'),
    ];
    for (let index = 1; index < order.length; index += 1) {
      expect(
        order[index - 1].compareDocumentPosition(order[index]) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
    expect(overview).toContainElement(order[0]);
  });
});

describe('StoragePoolDrawer', () => {
  it('heads the drawer with the pool name and the State cell judgement, and collapses', () => {
    const onClose = vi.fn();
    render(() => (
      <StoragePoolDrawer
        record={makeRecord({
          details: { type: 'zfspool', zfsPool: zfsPool({ state: 'DEGRADED' }) },
        })}
        physicalDisks={[]}
        onClose={onClose}
      />
    ));

    const heading = screen.getByRole('heading', { level: 2, name: 'tank' });
    expect(heading).toBeInTheDocument();
    const dot = screen.getByLabelText('Degraded');
    expect(dot.className).toContain('amber');

    fireEvent.click(screen.getByRole('button', { name: 'Collapse tank details' }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('keeps Overview mounted and pauses its charts while Manage shows operator state', async () => {
    render(() => <StoragePoolDrawer record={makeRecord()} physicalDisks={[]} onClose={() => {}} />);
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('tab', { name: 'History' })).toBeNull();
    expect(await screen.findByTestId('drawer-history')).toHaveAttribute('data-enabled', 'true');
    expect(screen.queryByTestId('operator-state')).toBeNull();

    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(screen.getByTestId('operator-state')).toHaveTextContent('storage:storage-1');
    expect(screen.getByTestId('drawer-history')).toHaveAttribute('data-enabled', 'false');
    expect(screen.getByTestId('storage-pool-drawer-overview')).toBeInTheDocument();
  });

  it('owns the chart range in the drawer and resets it for another pool', async () => {
    const [record, setRecord] = createSignal(makeRecord());
    render(() => <StoragePoolDrawer record={record()} physicalDisks={[]} onClose={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Pick 7 days' }));
    expect(screen.getByTestId('drawer-history')).toHaveAttribute('data-range', '7d');

    setRecord(makeRecord({ id: 'storage-2', name: 'backup' }));
    expect(await screen.findByTestId('drawer-history')).toHaveAttribute('data-range', '24h');
  });
});
