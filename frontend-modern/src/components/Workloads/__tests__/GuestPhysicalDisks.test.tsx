import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { GuestPhysicalDisks } from '../GuestPhysicalDisks';

const queryState = vi.hoisted(() => ({
  resources: [] as Resource[],
  error: undefined as unknown,
  query: '',
  enabled: false,
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options: { query: string; enabled: () => boolean }) => {
    queryState.query = options.query;
    queryState.enabled = options.enabled();
    return { resources: () => queryState.resources, error: () => queryState.error };
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
  beforeEach(() => {
    queryState.resources = [];
    queryState.error = undefined;
    queryState.query = '';
    queryState.enabled = false;
  });
  afterEach(cleanup);

  it('shows the linked guest disk and opens its existing SMART detail', () => {
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
    expect(within(card).getByText('Reallocated Sectors')).toBeInTheDocument();
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

  it('makes a failed child query visible rather than silently implying no disks', () => {
    queryState.error = new Error('unavailable');
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    expect(screen.getByRole('status')).toHaveTextContent('Physical disk details are unavailable.');
  });
});
