import { createSignal } from 'solid-js';
import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { StoragePoolDetail } from '../StoragePoolDetail';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';
import type { Resource } from '@/types/resource';
import type { State } from '@/types/api';

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: () => <div>History fixture</div>,
}));

const pool = (owner: string | undefined): Resource =>
  ({
    id: 'pool',
    name: 'tank',
    type: 'storage',
    platformType: 'truenas',
    parentId: owner,
    parentName: 'Identical label',
    storage: {
      type: 'zfs-pool',
      isZfs: true,
      zfsPool: {
        name: 'tank',
        state: 'ONLINE',
        readErrors: 0,
        writeErrors: 0,
        checksumErrors: 0,
        devices: [{ name: 'sda', type: 'disk', state: 'ONLINE' }],
      },
    },
  }) as Resource;

const disk = (id: string, owner: string, model: string, errors = 0): Resource =>
  ({
    id,
    name: model,
    type: 'physical_disk',
    platformType: 'truenas',
    parentId: owner,
    parentName: 'Identical label',
    physicalDisk: { devPath: '/dev/sda', model, errorCount: errors, temperature: errors ? 82 : 34 },
  }) as Resource;

describe('Storage pool ownership through live snapshots', () => {
  it('updates mounted ownership, local warnings and removal without leaking peer disks', () => {
    const [resource, setResource] = createSignal(pool('host-a'));
    const [disks, setDisks] = createSignal([
      disk('a', 'host-a', 'Local disk'),
      disk('b', 'host-b', 'Peer disk', 99),
    ]);
    const record = () => buildStorageRecords({ state: {} as State, resources: [resource()] })[0];
    render(() => <StoragePoolDetail record={record()} physicalDisks={disks()} />);

    expect(screen.getByText('Physical disks (1)')).toBeVisible();
    expect(screen.getByText('Local disk')).toBeVisible();
    expect(screen.queryByText('Peer disk')).not.toBeInTheDocument();
    expect(screen.queryByText('99 errors')).not.toBeInTheDocument();

    setResource(pool('host-b'));
    expect(screen.queryByText('Local disk')).not.toBeInTheDocument();
    expect(screen.getByText('Peer disk')).toBeVisible();
    expect(screen.getByText('99 errors')).toBeVisible();
    // The peer disk's errors and heat are now this pool's to check.
    expect(screen.getByTestId('storage-pool-drawer-exceptions')).toHaveTextContent(
      '/dev/sda: 99 errors',
    );
    expect(screen.getByText('82°C')).toBeVisible();
    expect(screen.getByText('Peer disk')).toBeVisible();

    setResource(pool(undefined));
    expect(screen.queryByText(/Physical disks/)).not.toBeInTheDocument();
    expect(screen.getByText('ZFS pool')).toBeVisible();
    expect(screen.queryByTestId('storage-pool-drawer-exceptions')).toBeNull();
    setResource(pool('host-a'));
    setDisks([disk('a', 'host-a', 'Local disk', 4)]);
    expect(screen.getByText('Local disk')).toBeVisible();
    expect(screen.getByText('4 errors')).toBeVisible();
    setDisks([]);
    expect(screen.queryByText(/Physical disks/)).not.toBeInTheDocument();
    expect(screen.queryByText('4 errors')).not.toBeInTheDocument();
  });
});
