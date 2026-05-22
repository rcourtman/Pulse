import { cleanup, render } from '@solidjs/testing-library';
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

    expect(mediaRow).toHaveAttribute('data-truenas-storage-depth', '1');
    expect(photosRow).toHaveAttribute('data-truenas-storage-depth', '2');
    expect(rawRow).toHaveAttribute('data-truenas-storage-depth', '3');
    expect(
      mediaRow
        ?.querySelector('[data-truenas-storage-indent-depth="1"]')
        ?.classList.contains('pl-5'),
    ).toBe(true);
    expect(
      photosRow
        ?.querySelector('[data-truenas-storage-indent-depth="2"]')
        ?.classList.contains('pl-9'),
    ).toBe(true);
    expect(
      rawRow?.querySelector('[data-truenas-storage-indent-depth="3"]')?.classList.contains('pl-12'),
    ).toBe(true);
  });

  it('caps deep indentation at the table-safe depth class', () => {
    expect(getTrueNASStorageTopologyIndentClass(0)).toBe('');
    expect(getTrueNASStorageTopologyIndentClass(1)).toBe('pl-5 sm:pl-7');
    expect(getTrueNASStorageTopologyIndentClass(2)).toBe('pl-9 sm:pl-11');
    expect(getTrueNASStorageTopologyIndentClass(3)).toBe('pl-12 sm:pl-16');
    expect(getTrueNASStorageTopologyIndentClass(8)).toBe('pl-12 sm:pl-16');
  });
});
