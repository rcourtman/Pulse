import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { DiskList } from '@/components/Storage/DiskList';

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
  }) =>
    render(() => {
      const [selectedDiskId, setSelectedDiskId] = createSignal<string | null>(null);
      return (
        <DiskList
          disks={props.disks}
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
