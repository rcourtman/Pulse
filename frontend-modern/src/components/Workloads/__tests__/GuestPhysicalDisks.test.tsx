import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { GuestPhysicalDisks } from '../GuestPhysicalDisks';

const queryState = vi.hoisted(() => ({
  resources: [] as Resource[],
  // Set to a signal accessor to drive live resource updates.
  accessor: undefined as (() => Resource[]) | undefined,
  error: undefined as unknown,
  query: '',
  enabled: false,
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options: { query: string; enabled: () => boolean }) => {
    queryState.query = options.query;
    queryState.enabled = options.enabled();
    return {
      resources: () => (queryState.accessor ? queryState.accessor() : queryState.resources),
      error: () => queryState.error,
    };
  },
}));

const disk: Resource = {
  id: 'disk-101-a',
  parentId: 'vm-resource-101',
  type: 'physical_disk',
  name: 'disk-101-a',
  displayName: 'Guest SATA',
  platformId: 'cluster-a',
  platformType: 'proxmox-pve',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  physicalDisk: {
    devPath: '/dev/sda',
    model: 'Guest SATA',
    diskType: 'sata',
    sizeBytes: 1_000_000_000,
    health: 'PASSED',
    temperature: 45,
    smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 2 },
  },
};

describe('GuestPhysicalDisks', () => {
  // The first expansion lazy-loads the Storage disk detail; a cold transform of
  // that module can outlast findByText's default wait on a loaded machine.
  beforeAll(async () => {
    await import('@/components/Storage/DiskDetail');
  }, 30_000);

  beforeEach(() => {
    queryState.resources = [];
    queryState.accessor = undefined;
    queryState.error = undefined;
    queryState.query = '';
    queryState.enabled = false;
  });
  afterEach(cleanup);

  it('shows the linked guest disk and loads its existing SMART detail on expansion', async () => {
    queryState.resources = [disk, { ...disk, id: 'host-disk', parentId: 'pve-node-1' }];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    expect(queryState.query).toBe('type=physical_disk&parent=vm-resource-101');
    expect(queryState.enabled).toBe(true);
    const card = screen.getByTestId('guest-physical-disks');
    expect(within(card).getByText('Physical Disks & SMART (1)')).toBeInTheDocument();
    expect(within(card).getByText('Guest SATA')).toBeInTheDocument();
    expect(within(card).getByText('Healthy')).toBeInTheDocument();
    expect(within(card).getByText('45°C')).toBeInTheDocument();
    expect(within(card).queryByText('Reallocated Sectors')).toBeNull();

    const row = within(card).getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));
    expect(await within(card).findByText('Reallocated Sectors')).toBeInTheDocument();
    expect(within(card).getByText('Pending Sectors')).toBeInTheDocument();
    expect(within(card).getByText('CRC Errors')).toBeInTheDocument();
    expect(within(card).getAllByTestId('guest-physical-disk')).toHaveLength(1);
  });

  it('does not add an empty storage card to an uninstrumented guest', () => {
    render(() => <GuestPhysicalDisks parentId="vm-resource-102" />);
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
  });

  it('encodes the parent value and never widens an empty parent to all disks', () => {
    render(() => <GuestPhysicalDisks parentId="vm&101" />);
    expect(queryState.query).toBe('type=physical_disk&parent=vm%26101');
    cleanup();
    render(() => <GuestPhysicalDisks parentId=" " />);
    expect(queryState.enabled).toBe(false);
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
  });

  it('marks a retained disk temperature as last known', () => {
    queryState.resources = [
      disk,
      {
        ...disk,
        id: 'disk-101-b',
        displayName: 'Guest NVMe',
        physicalDisk: {
          ...disk.physicalDisk!,
          devPath: '/dev/nvme0',
          model: 'Guest NVMe',
          temperature: 61,
          collection: {
            temperature: {
              state: 'unavailable',
              source: 'host_agent',
              reason: 'host agent stopped reporting',
            },
          },
        },
      },
    ];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);

    const [currentRow, retainedRow] = screen.getAllByTestId('guest-physical-disk');
    const current = within(currentRow).getByText('45°C');
    expect(current).toHaveAttribute('data-temperature-reading', 'current');
    expect(current).not.toHaveAttribute('title');

    const retained = retainedRow.querySelector('[data-temperature-reading="last-known"]');
    expect(retained).not.toBeNull();
    expect(retained).toHaveTextContent('61°C, last known');
    expect(retained).toHaveClass('underline', 'decoration-dotted', 'text-muted');
    expect(retained).toHaveAttribute(
      'title',
      'Last known reading, not current: host agent stopped reporting',
    );
  });

  it('follows a disk between current and last known as its collection state changes', () => {
    const withReading = (
      temperature: number,
      collection?: NonNullable<Resource['physicalDisk']>['collection'],
    ): Resource => ({
      ...disk,
      physicalDisk: { ...disk.physicalDisk!, temperature, collection },
    });
    const [resources, setResources] = createSignal<Resource[]>([disk]);
    queryState.accessor = resources;
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const reading = () =>
      screen.getByTestId('guest-physical-disk').querySelector('[data-temperature-reading]');

    expect(reading()).toHaveAttribute('data-temperature-reading', 'current');

    setResources([
      withReading(61, {
        temperature: {
          state: 'unavailable',
          source: 'host_agent',
          reason: 'host agent stopped reporting',
        },
      }),
    ]);
    expect(reading()).toHaveAttribute('data-temperature-reading', 'last-known');
    expect(reading()).toHaveTextContent('61°C, last known');
    expect(reading()).toHaveClass('decoration-dotted');

    setResources([withReading(47, { temperature: { state: 'available', source: 'smartctl' } })]);
    expect(reading()).toHaveAttribute('data-temperature-reading', 'current');
    expect(reading()).toHaveTextContent('47°C');
    expect(reading()).not.toHaveTextContent('last known');
    expect(reading()).not.toHaveAttribute('title');
    expect(reading()).not.toHaveClass('decoration-dotted');

    setResources([withReading(0)]);
    expect(reading()).toBeNull();
  });

  it('makes a failed child query visible rather than silently implying no disks', () => {
    queryState.error = new Error('unavailable');
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    expect(screen.getByRole('status')).toHaveTextContent('Physical disk details are unavailable.');
  });
});
