import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
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
      searchTerm: "",
    });

    expect(screen.getByRole('columnheader', { name: 'Disk' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Source' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Host' })).toBeInTheDocument();
    expect(screen.getByText('Disk sda')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('PVE')).toBeInTheDocument();
    expect(screen.getByText('Parity')).toBeInTheDocument();
    expect(screen.getByText('Tower Array')).toBeInTheDocument();
    expect(screen.getByText('Needs Attention')).toBeInTheDocument();
    expect(screen.getByText('Pending sectors detected.')).toBeInTheDocument();
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

  it('keeps api-backed TrueNAS disks on the canonical physical-disk surface even without hardware ids', () => {
    renderDiskList({
      disks: [
          ({
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
          }) as Resource,
      ],
      nodes: [
          ({
            ...buildNode('truenas-main', 'truenas-main'),
            platformType: 'truenas',
          }) as Resource,
      ],
      selectedNode: null,
      searchTerm: '',
    });

    expect(screen.getByText('TrueNAS')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Seagate IronWolf'));
    expect(screen.getByTestId('disk-detail')).toHaveTextContent('sda');
  });
});
