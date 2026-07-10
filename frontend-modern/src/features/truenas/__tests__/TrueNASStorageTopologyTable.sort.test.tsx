import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { TrueNASStorageTopologyTable } from '@/features/truenas/TrueNASStorageTopologyTable';
import type { Resource } from '@/types/resource';

// The capacity bar observes its container size; jsdom has no ResizeObserver,
// and this suite only asserts row order.
vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: (props: { type: string }) => (
    <div data-testid={`responsive-${props.type}-metric`} />
  ),
}));

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

// Two pools with root datasets so both levels of the topology are sortable:
// the pools against each other and the datasets within each pool's subtree.
const FIXTURE: Resource[] = [
  makeStorageResource({
    id: 'pool-alpha',
    name: 'alpha',
    displayName: 'alpha',
    disk: { current: 50 } as Resource['disk'],
    storage: { topology: 'pool', platform: 'truenas', path: 'alpha' },
  }),
  makeStorageResource({
    id: 'dataset-zeta',
    name: 'alpha/zeta',
    displayName: 'alpha/zeta',
    disk: { current: 10 } as Resource['disk'],
    storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/alpha/zeta' },
  }),
  makeStorageResource({
    id: 'dataset-mike',
    name: 'alpha/mike',
    displayName: 'alpha/mike',
    disk: { current: 80 } as Resource['disk'],
    storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/alpha/mike' },
  }),
  // No disk metric: must sink to the bottom of its sibling group when the
  // user sorts on Usage / Size.
  makeStorageResource({
    id: 'dataset-bravo',
    name: 'alpha/bravo',
    displayName: 'alpha/bravo',
    storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/alpha/bravo' },
  }),
  makeStorageResource({
    id: 'pool-zulu',
    name: 'zulu',
    displayName: 'zulu',
    disk: { current: 70 } as Resource['disk'],
    storage: { topology: 'pool', platform: 'truenas', path: 'zulu' },
  }),
  makeStorageResource({
    id: 'dataset-data',
    name: 'zulu/data',
    displayName: 'zulu/data',
    disk: { current: 30 } as Resource['disk'],
    storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/zulu/data' },
  }),
];

const renderTable = () =>
  render(() => (
    <TrueNASStorageTopologyTable
      resources={FIXTURE}
      scope={FIXTURE}
      emptyIcon={<span />}
      emptyTitle="No storage"
      emptyDescription="No storage"
      showToolbar={false}
    />
  ));

const visibleRowOrder = (container: HTMLElement): string[] =>
  Array.from(container.querySelectorAll('tr[data-truenas-storage-row]')).map(
    (row) => row.getAttribute('data-truenas-storage-row') ?? '',
  );

const headerFor = (label: string): HTMLElement => {
  const header = screen
    .getAllByRole('columnheader')
    .find((th) => th.textContent?.trim().startsWith(label));
  if (!header) throw new Error(`No column header labelled ${label}`);
  return header;
};

afterEach(() => {
  window.localStorage.clear();
  cleanup();
});

describe('TrueNASStorageTopologyTable user sorting', () => {
  it('sorts sibling groups without tearing subtrees from their pools', () => {
    const { container } = renderTable();

    // Built-in order: pools alphabetically, each with its datasets below it.
    expect(visibleRowOrder(container)).toEqual([
      'pool:pool-alpha',
      'dataset:dataset-bravo',
      'dataset:dataset-mike',
      'dataset:dataset-zeta',
      'pool:pool-zulu',
      'dataset:dataset-data',
    ]);

    fireEvent.click(headerFor('Resource'));
    expect(headerFor('Resource')).toHaveAttribute('aria-sort', 'ascending');
    expect(visibleRowOrder(container)).toEqual([
      'pool:pool-alpha',
      'dataset:dataset-bravo',
      'dataset:dataset-mike',
      'dataset:dataset-zeta',
      'pool:pool-zulu',
      'dataset:dataset-data',
    ]);

    // Descending re-orders the pools AND the datasets within each pool, but
    // every dataset stays directly below its own pool.
    fireEvent.click(headerFor('Resource'));
    expect(headerFor('Resource')).toHaveAttribute('aria-sort', 'descending');
    expect(visibleRowOrder(container)).toEqual([
      'pool:pool-zulu',
      'dataset:dataset-data',
      'pool:pool-alpha',
      'dataset:dataset-zeta',
      'dataset:dataset-mike',
      'dataset:dataset-bravo',
    ]);

    // Third click clears back to the built-in topology order.
    fireEvent.click(headerFor('Resource'));
    expect(headerFor('Resource')).not.toHaveAttribute('aria-sort');
    expect(visibleRowOrder(container)).toEqual([
      'pool:pool-alpha',
      'dataset:dataset-bravo',
      'dataset:dataset-mike',
      'dataset:dataset-zeta',
      'pool:pool-zulu',
      'dataset:dataset-data',
    ]);
  });

  it('sorts Usage / Size descending first with missing values last in each subtree', () => {
    const { container } = renderTable();

    fireEvent.click(headerFor('Usage / Size'));
    expect(headerFor('Usage / Size')).toHaveAttribute('aria-sort', 'descending');
    // Pools order on their own usage (zulu 70 > alpha 50); alpha's datasets
    // order 80 > 10 with the metric-less bravo sinking to the bottom of its
    // sibling group.
    expect(visibleRowOrder(container)).toEqual([
      'pool:pool-zulu',
      'dataset:dataset-data',
      'pool:pool-alpha',
      'dataset:dataset-mike',
      'dataset:dataset-zeta',
      'dataset:dataset-bravo',
    ]);
  });

  it('persists the chosen sort across a remount', () => {
    const first = renderTable();
    fireEvent.click(headerFor('Resource'));
    fireEvent.click(headerFor('Resource'));
    expect(window.localStorage.getItem('truenasStorageSortKey')).toBe('resource');
    expect(window.localStorage.getItem('truenasStorageSortDirection')).toBe('desc');
    expect(visibleRowOrder(first.container)[0]).toBe('pool:pool-zulu');

    cleanup();

    const second = renderTable();
    expect(headerFor('Resource')).toHaveAttribute('aria-sort', 'descending');
    expect(visibleRowOrder(second.container)[0]).toBe('pool:pool-zulu');
  });
});
