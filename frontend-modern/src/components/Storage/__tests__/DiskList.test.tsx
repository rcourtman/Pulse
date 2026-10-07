import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AlertsAPI } from '@/api/alerts';
import type { AlertConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';
import { DiskList } from '@/components/Storage/DiskList';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { eventBus } from '@/stores/events';

vi.mock('@/components/Storage/DiskDetail', () => ({
  DiskDetail: (props: { disk: Resource }) => <div data-testid="disk-detail">{props.disk.id}</div>,
}));

const buildNode = (id: string, name: string): Resource =>
  ({
    id,
    type: 'agent',
    name,
    displayName: name,
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { proxmox: { instance: 'cluster-main' } },
  }) as unknown as Resource;

const buildDisk = (
  id: string,
  nodeName: string,
  overrides: Partial<Resource['physicalDisk']> = {},
): Resource =>
  ({
    id,
    type: 'physical_disk',
    name: id,
    displayName: id,
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    parentId: `node-${nodeName}`,
    lastSeen: Date.now(),
    metricsTarget: { resourceType: 'disk', resourceId: `agent-${nodeName}:${id}` },
    identity: { hostname: nodeName },
    canonicalIdentity: { hostname: nodeName },
    platformData: {
      proxmox: { nodeName, instance: 'cluster-main' },
    },
    physicalDisk: {
      devPath: `/dev/${id}`,
      model: `Disk ${id}`,
      serial: `SERIAL-${id}`,
      diskType: 'sata',
      sizeBytes: 2_000_000_000_000,
      health: 'PASSED',
      temperature: 41,
      storageRole: 'parity',
      storageGroup: 'Tower Array',
      ...overrides,
    },
  }) as unknown as Resource;

describe('DiskList', () => {
  const renderDiskList = (props: {
    disks: Resource[];
    nodes: Resource[];
    selectedNode: string | null;
    searchTerm: string;
    getDiskAlertResourceIds?: (disk: Resource) => string[];
  }) =>
    render(() => {
      const [selectedDiskId, setSelectedDiskId] = createSignal<string | null>(null);
      return (
        <DiskList
          disks={props.disks}
          getDiskAlertResourceIds={props.getDiskAlertResourceIds}
          nodes={props.nodes}
          selectedNode={props.selectedNode}
          searchTerm={props.searchTerm}
          selectedDiskId={selectedDiskId()}
          onSelectedDiskChange={setSelectedDiskId}
        />
      );
    });

  afterEach(() => {
    cleanup();
  });

  it('shows a retained temperature as last known, not as a live threshold reading', async () => {
    const withTemperature = (collection: NonNullable<Resource['physicalDisk']>['collection']) =>
      buildDisk('sda', 'tower', { diskType: 'sata', temperature: 72, collection });
    const [disks, setDisks] = createSignal([
      withTemperature({ temperature: { state: 'available', source: 'host_agent' } }),
    ]);
    const view = render(() => (
      <DiskList
        disks={disks()}
        nodes={[]}
        selectedNode={null}
        searchTerm=""
        selectedDiskId={null}
        onSelectedDiskChange={() => {}}
      />
    ));
    const reading = () =>
      view.container.querySelector<HTMLElement>(
        '[data-row-id="sda"] td[data-storage-column="temp"] [data-temperature-reading]',
      )!;

    expect(reading()).toHaveAttribute('data-temperature-reading', 'current');
    expect(reading()).toHaveClass('text-red-600');
    expect(reading()).not.toHaveAttribute('title');
    expect(reading()).toHaveTextContent(/^72°C$/);

    // The same row, retained after its host agent stopped reporting.
    setDisks([
      withTemperature({
        temperature: {
          state: 'unavailable',
          source: 'host_agent',
          reason: 'host agent stopped reporting',
        },
      }),
    ]);
    await waitFor(() =>
      expect(reading()).toHaveAttribute('data-temperature-reading', 'last-known'),
    );
    expect(reading()).toHaveClass('text-muted');
    expect(reading().className).not.toMatch(/text-(red|amber|green)-/);
    expect(reading()).toHaveAttribute(
      'title',
      'Last known reading, not current: host agent stopped reporting',
    );
    expect(reading()).toHaveTextContent('72°C, last known');

    // A source that predates collection state still reads as current.
    setDisks([withTemperature(undefined)]);
    await waitFor(() => expect(reading()).toHaveAttribute('data-temperature-reading', 'current'));
    expect(reading()).toHaveClass('text-red-600');
  });

  it('refreshes keyed disk rows without losing the expanded detail or keyboard focus', async () => {
    const initial = buildDisk('sda', 'tower', { diskType: 'ssd', wearout: 96 });
    const [disks, setDisks] = createSignal([initial]);
    const [selectedDiskId, setSelectedDiskId] = createSignal<string | null>('sda');
    const view = render(() => (
      <DiskList
        disks={disks()}
        nodes={[]}
        selectedNode={null}
        searchTerm=""
        selectedDiskId={selectedDiskId()}
        onSelectedDiskChange={setSelectedDiskId}
      />
    ));
    const row = view.container.querySelector('[data-row-id="sda"]')!;
    const detail = screen.getByTestId('disk-detail');
    const disclosure = within(row as HTMLElement).getByRole('button');
    disclosure.focus();
    const initialControls = disclosure.getAttribute('aria-controls');

    setDisks([
      {
        ...buildDisk('sda', 'archive', {
          model: 'Replacement SSD',
          devPath: '/dev/sdz',
          diskType: 'ssd',
          wearout: 4,
          sizeBytes: 4_000_000_000_000,
          temperature: 63,
          storageRole: 'cache_pool',
          storageGroup: 'Archive Pool',
          health: 'FAILED',
          risk: {
            level: 'critical',
            reasons: [{ code: 'smart-failed', severity: 'critical', summary: 'SMART failed.' }],
          },
        }),
        metricsTarget: { resourceType: 'disk', resourceId: 'disk:archive:sdz' },
      },
    ]);

    await waitFor(() =>
      expect(within(row as HTMLElement).getByText('Replace Now')).toBeInTheDocument(),
    );
    for (const text of [
      'Replacement SSD',
      '/dev/sdz',
      'archive',
      'Cache Pool',
      'Archive Pool',
      '4%',
      '63°C',
    ]) {
      expect(within(row as HTMLElement).getByText(text)).toBeInTheDocument();
    }
    // The reason is not a second truncating string in the 32px row: it rides
    // on the verdict as its title and is spelled out in the disk drawer.
    expect(within(row as HTMLElement).queryByText('SMART failed.')).not.toBeInTheDocument();
    expect(within(row as HTMLElement).getByText('Replace Now')).toHaveAttribute(
      'title',
      'SMART failed.',
    );
    expect(within(row as HTMLElement).getByText('3.64 TB')).toBeInTheDocument();
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument();
    expect(screen.queryByText('96%')).not.toBeInTheDocument();
    expect(view.container.querySelector('[data-row-id="sda"]')).toBe(row);
    expect(screen.getByTestId('disk-detail')).toBe(detail);
    expect(document.activeElement).toBe(disclosure);
    expect(disclosure.getAttribute('aria-label')).toContain('Replacement SSD');
    expect(row).toHaveAttribute('data-summary-series-id', 'disk:archive:sdz');
    expect(disclosure.getAttribute('aria-controls')).not.toBe(initialControls);
    expect(
      view.container.querySelector('[data-inline-detail-for="disk:archive:sdz"]'),
    ).not.toBeNull();
    expect(document.getElementById(disclosure.getAttribute('aria-controls')!)).not.toBeNull();
  });

  it('removes obsolete readings and fault styling when a keyed snapshot stops reporting them', async () => {
    const [disks, setDisks] = createSignal([
      buildDisk('sda', 'tower', {
        health: 'FAILED',
        diskType: 'ssd',
        wearout: 4,
        temperature: 63,
        risk: {
          level: 'critical',
          reasons: [{ code: 'smart-failed', severity: 'critical', summary: 'SMART failed.' }],
        },
      }),
    ]);
    const view = render(() => (
      <DiskList
        disks={disks()}
        nodes={[]}
        selectedNode={null}
        searchTerm=""
        selectedDiskId={null}
        onSelectedDiskChange={() => {}}
      />
    ));
    const row = view.container.querySelector('[data-row-id="sda"]')!;
    expect(within(row as HTMLElement).getByText('Replace Now')).toHaveClass('text-red-700');
    setDisks([
      buildDisk('sda', 'tower', {
        health: 'UNKNOWN',
        diskType: '',
        wearout: -1,
        temperature: 0,
        sizeBytes: 0,
        storageRole: '',
        storageGroup: '',
        model: '',
        devPath: '',
      }),
    ]);
    await waitFor(() =>
      expect(within(row as HTMLElement).getByText('Unknown')).toBeInTheDocument(),
    );
    for (const text of ['Replace Now', 'SMART failed.', '4%', '63°C', 'Parity', 'Tower Array']) {
      expect(within(row as HTMLElement).queryByText(text)).not.toBeInTheDocument();
    }
    expect(within(row as HTMLElement).getByText('Unknown')).not.toHaveClass('text-red-700');
    expect(within(row as HTMLElement).getByText('sda')).toBeInTheDocument();
    for (const column of ['temp', 'life', 'size', 'device', 'role', 'parent']) {
      expect(row.querySelector(`td[data-storage-column="${column}"]`)).toHaveTextContent('—');
    }
    expect(view.container.querySelector('[data-row-id="sda"]')).toBe(row);
  });

  it('keeps the attention filter and row health consistent during live updates and recovery', async () => {
    const [disks, setDisks] = createStore({ items: [buildDisk('sda', 'tower')] });
    const [healthFilter, setHealthFilter] = createSignal<'all' | 'attention'>('all');
    const view = render(() => (
      <DiskList
        disks={disks.items}
        nodes={[]}
        selectedNode={null}
        searchTerm=""
        healthFilter={healthFilter()}
        selectedDiskId={null}
        onSelectedDiskChange={() => {}}
      />
    ));
    const row = view.container.querySelector('[data-row-id="sda"]');
    setDisks(
      'items',
      reconcile([buildDisk('sda', 'tower', { smart: { pendingSectors: 2 }, temperature: 52 })]),
    );
    await waitFor(() => expect(screen.getByText('Needs Attention')).toBeInTheDocument());
    expect(view.container.querySelector('[data-row-id="sda"]')).toBe(row);
    setHealthFilter('attention');
    expect(screen.getByText('Needs Attention')).toBeInTheDocument();
    setDisks('items', reconcile([buildDisk('sda', 'tower')]));
    await waitFor(() => expect(screen.getByText('No disks need attention')).toBeInTheDocument());
    setHealthFilter('all');
    await waitFor(() => expect(screen.getByText('Healthy')).toBeInTheDocument());
    expect(screen.queryByText('52°C')).not.toBeInTheDocument();
  });

  it('renders physical disks in a single-line operational grid', () => {
    renderDiskList({
      disks: [
        buildDisk('sda', 'tower', {
          risk: {
            level: 'warning',
            reasons: [
              {
                code: 'pending-sectors',
                severity: 'warning',
                summary: 'Pending sectors detected.',
              },
            ],
          },
        }),
      ],
      nodes: [buildNode('node-tower', 'tower')],
      selectedNode: null,
      searchTerm: '',
    });

    expect(screen.getByRole('columnheader', { name: 'Disk' })).toBeInTheDocument();
    expect(screen.getByRole('table')).toHaveAttribute('data-storage-table', 'physical-disks');
    expect(screen.getByRole('table')).toHaveAttribute('data-storage-layout', 'compact');
    expect(screen.getByRole('columnheader', { name: 'Device' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Device' })).toHaveAttribute(
      'data-storage-column',
      'device',
    );
    expect(screen.getByRole('columnheader', { name: 'Host' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'SSD life remaining' })).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Source' })).not.toBeInTheDocument();
    expect(screen.getByText('Disk sda')).toBeInTheDocument();
    expect(screen.getByText('/dev/sda')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('Parity')).toBeInTheDocument();
    expect(screen.getByText('Tower Array')).toBeInTheDocument();
    expect(screen.getByText('Needs Attention')).toBeInTheDocument();
    // The reason stays one hover away on the verdict rather than as a second
    // truncating string in the row; the drawer spells it out.
    expect(screen.queryByText('Pending sectors detected.')).not.toBeInTheDocument();
    expect(screen.getByText('Needs Attention')).toHaveAttribute(
      'title',
      'Pending sectors detected.',
    );
  });

  it('says a disk that is only hot is running hot, not due for replacement', () => {
    const view = renderDiskList({
      disks: [
        buildDisk('sda', 'pve3', {
          model: 'Crucial MX500 2TB',
          diskType: 'sata',
          wearout: 95,
          temperature: 56,
        }),
        buildDisk('sdb', 'pve3', {
          temperature: 72,
          smart: { pendingSectors: 2 },
          risk: {
            level: 'critical',
            reasons: [
              {
                code: 'pending_sectors',
                severity: 'critical',
                summary: 'Pending sectors detected (2)',
              },
            ],
          },
        }),
        buildDisk('nvme0n1', 'pve3', {
          model: 'Samsung 990 PRO',
          diskType: 'nvme',
          temperature: 63,
        }),
      ],
      nodes: [buildNode('node-pve3', 'pve3')],
      selectedNode: null,
      searchTerm: '',
    });

    const rows = Array.from(view.container.querySelectorAll('[data-row-id]'));
    // The disk that needs replacing sorts above the one that needs cooling.
    expect(rows.map((row) => row.getAttribute('data-row-id'))).toEqual(['sdb', 'sda', 'nvme0n1']);
    const hotRow = within(rows[1] as HTMLElement);
    expect(hotRow.queryByText('Replace Now')).not.toBeInTheDocument();
    expect(hotRow.getByText('Running Hot')).toHaveClass(
      'platform-table-label-full',
      'text-red-700',
    );
    expect(hotRow.getByText('Running Hot')).toHaveAttribute(
      'title',
      'Disk temperature is 56°C, at or above its 55°C alert threshold.',
    );
    expect(hotRow.getByText('Hot')).toHaveClass('platform-table-label-compact', 'text-red-700');
    // The Temp cell beside it reads red too: the same SATA alert trigger.
    expect(hotRow.getByText('56°C')).toHaveClass('text-red-600');
    const failingRow = within(rows[0] as HTMLElement);
    expect(failingRow.getByText('Replace Now')).toHaveAttribute(
      'title',
      'Pending sectors detected (2)',
    );
    // An NVMe at 63C is under its 70C trigger and its 65C warning colour.
    const nvmeRow = within(rows[2] as HTMLElement);
    expect(nvmeRow.getByText('Healthy')).toBeInTheDocument();
    expect(nvmeRow.getByText('63°C')).toHaveClass('text-green-600');
  });

  it('follows a disk from warm to hot and back to healthy', async () => {
    const [disks, setDisks] = createStore({
      items: [buildDisk('sda', 'pve3', { temperature: 52 })],
    });
    const [healthFilter, setHealthFilter] = createSignal<'all' | 'critical'>('all');
    const view = render(() => (
      <DiskList
        disks={disks.items}
        nodes={[]}
        selectedNode={null}
        searchTerm=""
        healthFilter={healthFilter()}
        selectedDiskId={null}
        onSelectedDiskChange={() => {}}
      />
    ));
    const row = () => view.container.querySelector('[data-row-id="sda"]') as HTMLElement | null;
    // Warm: the Temp cell warns in amber, the verdict stays Healthy.
    expect(within(row()!).getByText('52°C')).toHaveClass('text-amber-600');
    expect(within(row()!).getByText('Healthy')).toBeInTheDocument();
    setHealthFilter('critical');
    await waitFor(() => expect(row()).toBeNull());

    setDisks('items', reconcile([buildDisk('sda', 'pve3', { temperature: 56 })]));
    await waitFor(() => expect(row()).not.toBeNull());
    expect(within(row()!).getByText('Running Hot')).toHaveClass('text-red-700');

    setHealthFilter('all');
    setDisks('items', reconcile([buildDisk('sda', 'pve3', { temperature: 41 })]));
    await waitFor(() => expect(within(row()!).getByText('Healthy')).toBeInTheDocument());
    expect(within(row()!).queryByText('Running Hot')).not.toBeInTheDocument();
  });

  it('judges heat by the NVMe trigger the user raised in Alerts', async () => {
    const getConfig = vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue({
      enabled: true,
      activationState: 'active',
      agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
      diskTempByType: {
        nvme: { trigger: 75, clear: 70 },
        sas: { trigger: 65, clear: 60 },
        sata: { trigger: 55, clear: 50 },
      },
    } as unknown as AlertConfig);
    try {
      const view = renderDiskList({
        disks: [buildDisk('nvme0n1', 'pve3', { diskType: 'nvme', temperature: 72 })],
        nodes: [],
        selectedNode: null,
        searchTerm: '',
      });
      const row = () =>
        within(view.container.querySelector('[data-row-id="nvme0n1"]') as HTMLElement);
      // Factory thresholds until the user's configuration loads: 72C passes
      // the 70C NVMe trigger.
      expect(row().getByText('Running Hot')).toBeInTheDocument();
      expect(row().getByText('72°C')).toHaveClass('text-red-600');

      await useAlertsActivation().refreshConfig();
      await waitFor(() => expect(row().getByText('Healthy')).toBeInTheDocument());
      expect(row().getByText('72°C')).toHaveClass('text-amber-600');
    } finally {
      getConfig.mockRestore();
      eventBus.emit('org_switched', 'default');
    }
  });

  it('judges each disk by the Disk Temp override of the machine that reports it', async () => {
    const getConfig = vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue({
      enabled: true,
      activationState: 'active',
      agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
      diskTempByType: { nvme: { trigger: 70, clear: 65 } },
      overrides: {
        'host-pve3': { diskTemperature: { trigger: 80, clear: 75 } },
        'host-pve5': { disabled: true },
      },
    } as unknown as AlertConfig);
    try {
      const view = renderDiskList({
        disks: [
          buildDisk('nvme-pve3', 'pve3', { diskType: 'nvme', temperature: 72 }),
          buildDisk('nvme-pve4', 'pve4', { diskType: 'nvme', temperature: 72 }),
          buildDisk('nvme-pve5', 'pve5', { diskType: 'nvme', temperature: 72 }),
        ],
        nodes: [],
        selectedNode: null,
        searchTerm: '',
        getDiskAlertResourceIds: (disk) => [`host-${disk.parentId?.replace('node-', '')}`],
      });
      const row = (id: string) =>
        within(view.container.querySelector(`[data-row-id="${id}"]`) as HTMLElement);

      await useAlertsActivation().refreshConfig();
      // pve3's 80C override puts 72C under its warning band: healthy and green.
      await waitFor(() => expect(row('nvme-pve3').getByText('Healthy')).toBeInTheDocument());
      expect(row('nvme-pve3').getByText('72°C')).toHaveClass('text-green-600');
      // pve4 has no override, so the 70C NVMe trigger judges its disk.
      expect(row('nvme-pve4').getByText('Running Hot')).toBeInTheDocument();
      expect(row('nvme-pve4').getByText('72°C')).toHaveClass('text-red-600');
      // pve5's alerts are switched off, so its disk is never judged hot.
      expect(row('nvme-pve5').queryByText('Running Hot')).not.toBeInTheDocument();
      expect(row('nvme-pve5').getByText('Healthy')).toBeInTheDocument();
    } finally {
      getConfig.mockRestore();
      eventBus.emit('org_switched', 'default');
    }
  });

  it('renders SSD life and falls back to the Proxmox usage string for Belongs', () => {
    renderDiskList({
      disks: [
        buildDisk('sda', 'tower', {
          diskType: 'ssd',
          wearout: 96,
          storageGroup: '',
          used: 'ZFS',
          storageRole: '',
        }),
      ],
      nodes: [buildNode('node-tower', 'tower')],
      selectedNode: null,
      searchTerm: '',
    });

    expect(screen.getByText('96%')).toBeInTheDocument();
    expect(screen.getByText('ZFS')).toBeInTheDocument();
  });

  it('filters disks by search term and supports row expansion', () => {
    renderDiskList({
      disks: [buildDisk('sda', 'tower'), buildDisk('sdb', 'tower', { model: 'Cache SSD' })],
      nodes: [buildNode('node-tower', 'tower')],
      selectedNode: null,
      searchTerm: 'cache',
    });

    expect(screen.queryByText('Disk sda')).not.toBeInTheDocument();
    expect(screen.getByText('Cache SSD')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Cache SSD'));
    expect(screen.getByTestId('disk-detail')).toHaveTextContent('sdb');
  });

  it('renders canonical Settings Infrastructure guidance for empty physical disks', () => {
    renderDiskList({
      disks: [],
      nodes: [buildNode('node-tower', 'tower')],
      selectedNode: null,
      searchTerm: '',
    });

    expect(screen.getByText('Physical disk monitoring requirements:')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Enable "Monitor physical disk health (SMART)" in Settings → Infrastructure for the Proxmox node',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Settings → Infrastructure → Proxmox/)).not.toBeInTheDocument();

    cleanup();
    renderDiskList({
      disks: [],
      nodes: [],
      selectedNode: null,
      searchTerm: '',
    });

    expect(
      screen.getByText(
        'No Proxmox nodes configured. Add Proxmox VE in Settings → Infrastructure to monitor physical disks.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Add a Proxmox VE cluster in Settings/)).not.toBeInTheDocument();
  });

  it('keeps api-backed TrueNAS disks on the canonical physical-disk surface even without hardware ids', () => {
    renderDiskList({
      disks: [
        {
          ...buildDisk('sda', 'truenas-main', {
            model: 'Seagate IronWolf',
            serial: '',
            wwn: '',
          }),
          platformType: 'truenas',
          metricsTarget: { resourceType: 'disk', resourceId: 'disk:truenas-main:sda' },
          sourceType: 'api',
          identity: { hostname: 'truenas-main' },
          canonicalIdentity: { hostname: 'truenas-main' },
          platformData: {
            sources: ['truenas'],
            physicalDisk: {
              serial: '',
              wwn: '',
            },
          },
        } as Resource,
      ],
      nodes: [
        {
          ...buildNode('truenas-main', 'truenas-main'),
          platformType: 'truenas',
        } as Resource,
      ],
      selectedNode: null,
      searchTerm: '',
    });

    expect(screen.getByText('truenas-main')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Seagate IronWolf'));
    expect(screen.getByTestId('disk-detail')).toHaveTextContent('sda');
  });
});
