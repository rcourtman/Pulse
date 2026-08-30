import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import {
  TrueNASStorageTopologyTable,
  getTrueNASStorageTopologyIndentClass,
} from '@/features/truenas/TrueNASStorageTopologyTable';
import type { Resource } from '@/types/resource';

const makeStorageResource = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'storage',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'api',
    storage: { topology: 'dataset', platform: 'truenas' },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASStorageTopologyTable', () => {
  it('renders nested dataset depth with distinct row indentation', () => {
    const pool = makeStorageResource({
      id: 'pool-tank',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', path: 'tank' },
    });
    const media = makeStorageResource({
      id: 'dataset-media',
      name: 'tank/media',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const photos = makeStorageResource({
      id: 'dataset-photos',
      name: 'tank/media/photos',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos' },
    });
    const raw = makeStorageResource({
      id: 'dataset-raw',
      name: 'tank/media/photos/raw',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos/raw' },
    });
    const resources = [pool, raw, media, photos];

    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));

    const mediaRow = container.querySelector('[data-truenas-storage-row="dataset:dataset-media"]');
    const photosRow = container.querySelector(
      '[data-truenas-storage-row="dataset:dataset-photos"]',
    );
    const rawRow = container.querySelector('[data-truenas-storage-row="dataset:dataset-raw"]');
    const headers = [...container.querySelectorAll('thead th')];

    expect(headers[0]).toHaveClass('platform-table-name-column', 'platform-table-mobile-w-30');
    expect(headers[1]).toHaveClass('platform-table-mobile-w-15');
    expect(headers[1]).not.toHaveClass('hidden');
    expect(headers[2]).toHaveClass('platform-table-mobile-w-25');
    expect(headers[3]).toHaveClass('platform-table-mobile-w-15');
    expect(headers[3]).not.toHaveClass('hidden');
    expect(headers[5]).toHaveClass('platform-table-phone-hidden');

    expect(mediaRow).toHaveAttribute('data-truenas-storage-depth', '1');
    expect(photosRow).toHaveAttribute('data-truenas-storage-depth', '2');
    expect(rawRow).toHaveAttribute('data-truenas-storage-depth', '3');
    expect(
      mediaRow
        ?.querySelector('[data-truenas-storage-indent-depth="1"]')
        ?.classList.contains('pl-3'),
    ).toBe(true);
    expect(
      photosRow
        ?.querySelector('[data-truenas-storage-indent-depth="2"]')
        ?.classList.contains('pl-6'),
    ).toBe(true);
    expect(
      rawRow?.querySelector('[data-truenas-storage-indent-depth="3"]')?.classList.contains('pl-8'),
    ).toBe(true);
  });

  it('caps deep indentation at the table-safe depth class', () => {
    expect(getTrueNASStorageTopologyIndentClass(0)).toBe('');
    expect(getTrueNASStorageTopologyIndentClass(1)).toBe('pl-3 sm:pl-7');
    expect(getTrueNASStorageTopologyIndentClass(2)).toBe('pl-6 sm:pl-11');
    expect(getTrueNASStorageTopologyIndentClass(3)).toBe('pl-8 sm:pl-16');
    expect(getTrueNASStorageTopologyIndentClass(8)).toBe('pl-8 sm:pl-16');
  });

  it('separates volumes and physical disks with an accessible one-click scope', async () => {
    const pool = makeStorageResource({
      id: 'pool-tank',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', path: 'tank' },
    });
    const dataset = makeStorageResource({
      id: 'dataset-media',
      name: 'tank/media',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const disk = makeStorageResource({
      id: 'disk-sda',
      type: 'physical_disk',
      name: 'sda',
      storage: undefined,
      physicalDisk: { devPath: '/dev/sda', serial: 'SERIAL-A', diskType: 'ssd', wearout: 68 },
    });
    const resources = [pool, dataset, disk];
    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
      />
    ));

    expect(screen.getByRole('group', { name: 'Storage type' })).toBeInTheDocument();
    expect(container.querySelectorAll('[data-truenas-storage-row]')).toHaveLength(3);

    await fireEvent.click(screen.getByRole('button', { name: 'Physical disks, 1' }));
    expect(screen.getByRole('button', { name: 'Physical disks, 1' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(container.querySelectorAll('[data-truenas-storage-row]')).toHaveLength(1);
    expect(container.querySelector('[data-truenas-storage-kind="disk"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-storage-kind="pool"]')).toBeNull();
    expect(screen.getByRole('columnheader', { name: /Endurance/ })).toBeInTheDocument();
    expect(screen.getByText('68% left')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /Temp/ })).toHaveClass('table-cell');
    expect(screen.getByRole('columnheader', { name: /Health/ })).toHaveClass('table-cell');

    await fireEvent.click(screen.getByRole('button', { name: 'Volumes, 2' }));
    expect(container.querySelectorAll('[data-truenas-storage-row]')).toHaveLength(2);
    expect(container.querySelector('[data-truenas-storage-kind="pool"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-storage-kind="dataset"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-storage-kind="disk"]')).toBeNull();
  });
});
