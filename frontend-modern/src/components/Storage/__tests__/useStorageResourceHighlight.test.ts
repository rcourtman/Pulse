import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import { useStorageResourceHighlight } from '@/components/Storage/useStorageResourceHighlight';

const getResource = vi.hoisted(() => vi.fn());
vi.mock('@/api/resources', () => ({ ResourceAPI: { getResource } }));
afterEach(() => getResource.mockReset());

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-ceph',
    name: 'ceph-store',
    category: 'pool',
    health: 'healthy',
    location: { label: 'cluster-main', scope: 'cluster' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: ['capacity'],
    source: {
      platform: 'proxmox-pve',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    observedAt: Date.now(),
    refs: { platformEntityId: '' },
    ...overrides,
  }) as StorageRecord;

// Mounts the hook with live search, records, active and expanded state, and
// records every pool the hook opens.
const mountHighlight = (search: string, records: StorageRecord[], active = true) => {
  const [locationPathname] = createSignal('/proxmox/storage');
  const [locationSearch, setLocationSearch] = createSignal(search);
  const [recordList, setRecords] = createSignal<StorageRecord[]>(records);
  const [isActive, setActive] = createSignal(active);
  const [expandedPoolId, setExpandedPoolIdSignal] = createSignal<string | null>(null);
  const navigate = vi.fn();
  const opened: Array<string | null> = [];
  const { result } = renderHook(() =>
    useStorageResourceHighlight({
      locationPathname,
      locationSearch,
      navigate,
      records: recordList,
      expandedPoolId,
      setExpandedPoolId: (value) => {
        const next = typeof value === 'function' ? value(expandedPoolId()) : value;
        opened.push(next);
        setExpandedPoolIdSignal(next);
      },
      active: isActive,
    }),
  );
  return {
    result,
    opened,
    navigate,
    setLocationSearch,
    setActive,
    setRecords,
    setExpandedPoolIdSignal,
  };
};

describe('useStorageResourceHighlight', () => {
  it('highlights a deep-linked pool and opens it', () => {
    const { result, opened, navigate } = mountHighlight('?resource=storage-ceph', [makeRecord()]);
    expect(result()).toBe('storage-ceph');
    expect(opened).toEqual(['storage-ceph']);
    expect(navigate).not.toHaveBeenCalled();
  });

  it('does nothing when the registry cannot resolve the reference', async () => {
    getResource.mockRejectedValue(new Error('not found'));
    const { result, opened, navigate } = mountHighlight('?resource=missing', [makeRecord()]);
    await waitFor(() => expect(getResource).toHaveBeenCalledWith('missing', expect.anything()));
    expect(result()).toBeNull();
    expect(opened).toEqual([]);
    expect(navigate).not.toHaveBeenCalled();
  });

  it('rewrites whitespace-padded resource deep-links to the canonical storage URL', () => {
    const { result, opened, navigate } = mountHighlight(
      '?resource=%20storage-ceph%20&from=overview',
      [makeRecord()],
    );
    expect(navigate).toHaveBeenCalledWith('/proxmox/storage?resource=storage-ceph&from=overview', {
      replace: true,
    });
    expect(result()).toBe('storage-ceph');
    expect(opened).toEqual(['storage-ceph']);
  });

  it('opens any deep-linked pool, not only Ceph, so the page can reveal it', () => {
    const { result, opened } = mountHighlight('?resource=storage-zfs', [
      makeRecord({ id: 'storage-zfs', name: 'local-zfs' }),
    ]);
    expect(result()).toBe('storage-zfs');
    expect(opened).toEqual(['storage-zfs']);
    expect(getResource).not.toHaveBeenCalled();
  });

  it('matches the metrics id alerts use for a pool', () => {
    const { opened } = mountHighlight('?resource=cluster-pve1-local', [
      makeRecord({
        id: 'storage-local',
        name: 'local',
        metricsTarget: { resourceType: 'storage', resourceId: 'cluster-pve1-local' },
      } as Partial<StorageRecord>),
    ]);
    expect(opened).toEqual(['storage-local']);
  });

  it('does not open a pool by a name several pools share', () => {
    // Every Proxmox node can carry a `local-zfs`.
    const { result, opened } = mountHighlight('?resource=local-zfs', [
      makeRecord({ id: 'storage-pve1-zfs', name: 'local-zfs' }),
      makeRecord({ id: 'storage-pve2-zfs', name: 'local-zfs' }),
    ]);
    expect(result()).toBeNull();
    expect(opened).toEqual([]);
    // The registry resolves ids, not names, so no lookup is made.
    expect(getResource).not.toHaveBeenCalled();
  });

  it('opens the pool the resource registry resolves an alert source id to', async () => {
    getResource.mockResolvedValue({ id: 'storage-2b0d' });
    const { result, opened } = mountHighlight('?resource=mock-cluster-1-pve1-local', [
      makeRecord({ id: 'storage-2b0d', name: 'west-a-iso-library' }),
    ]);
    await waitFor(() => expect(opened).toEqual(['storage-2b0d']));
    expect(getResource).toHaveBeenCalledWith('mock-cluster-1-pve1-local', expect.anything());
    expect(result()).toBe('storage-2b0d');
  });

  it('ignores another tab’s link while Storage is hidden, then applies it when shown', async () => {
    // The Proxmox page keeps Storage mounted behind its other tabs, whose
    // guest links also use ?resource=.
    getResource.mockResolvedValue({ id: 'storage-2b0d' });
    const records = [makeRecord({ id: 'storage-2b0d', name: 'west-a-iso-library' })];
    const hidden = mountHighlight('?resource=mock-cluster-1-pve2-121', records, false);
    expect(getResource).not.toHaveBeenCalled();
    expect(hidden.opened).toEqual([]);

    hidden.setLocationSearch('?resource=mock-cluster-1-pve1-local');
    hidden.setActive(true);
    await waitFor(() => expect(hidden.opened).toEqual(['storage-2b0d']));
  });

  it('retries a refused reference on a later visit, not on every update', async () => {
    getResource.mockRejectedValueOnce(new Error('not yet registered'));
    const records = [makeRecord({ id: 'storage-new', name: 'new-pool' })];
    const view = mountHighlight('?resource=pve1-new-pool', records);
    await waitFor(() => expect(getResource).toHaveBeenCalledTimes(1));
    expect(view.opened).toEqual([]);

    getResource.mockResolvedValue({ id: 'storage-new' });
    view.setLocationSearch('');
    view.setLocationSearch('?resource=pve1-new-pool');
    await waitFor(() => expect(view.opened).toEqual(['storage-new']));
    expect(getResource).toHaveBeenCalledTimes(2);
  });

  it('keeps the pool the operator opened while the lookup was running', async () => {
    let resolveLookup: (value: { id: string }) => void = () => undefined;
    getResource.mockReturnValue(
      new Promise<{ id: string }>((resolve) => {
        resolveLookup = resolve;
      }),
    );
    const records = [
      makeRecord({ id: 'storage-linked', name: 'linked-pool' }),
      makeRecord({ id: 'storage-chosen', name: 'chosen-pool' }),
    ];
    const view = mountHighlight('?resource=pve1-linked-pool', records);
    await waitFor(() => expect(getResource).toHaveBeenCalled());

    view.setExpandedPoolIdSignal('storage-chosen');
    resolveLookup({ id: 'storage-linked' });
    await Promise.resolve();
    await Promise.resolve();
    expect(view.opened).toEqual([]);
    expect(view.result()).toBeNull();
  });
  it('waits for a resolved pool to be listed instead of asking again', async () => {
    getResource.mockResolvedValue({ id: 'storage-late' });
    const view = mountHighlight('?resource=pve1-late-pool', [makeRecord()]);
    await waitFor(() => expect(getResource).toHaveBeenCalledTimes(1));
    await Promise.resolve();
    await Promise.resolve();
    view.setRecords([makeRecord(), makeRecord({ id: 'other', name: 'other-pool' })]);
    expect(getResource).toHaveBeenCalledTimes(1);

    view.setRecords([makeRecord(), makeRecord({ id: 'storage-late', name: 'late-pool' })]);
    await waitFor(() => expect(view.opened).toEqual(['storage-late']));
    expect(getResource).toHaveBeenCalledTimes(1);
  });

  it('retries a refused reference after visiting another pool', async () => {
    getResource.mockRejectedValueOnce(new Error('not yet registered'));
    const records = [
      makeRecord({ id: 'storage-new', name: 'new-pool' }),
      makeRecord({ id: 'storage-other', name: 'other-pool' }),
    ];
    const view = mountHighlight('?resource=pve1-new-pool', records);
    await waitFor(() => expect(getResource).toHaveBeenCalledTimes(1));

    view.setLocationSearch('?resource=storage-other');
    expect(view.opened).toEqual(['storage-other']);
    getResource.mockResolvedValue({ id: 'storage-new' });
    view.setLocationSearch('?resource=pve1-new-pool');
    await waitFor(() => expect(view.opened).toEqual(['storage-other', 'storage-new']));
    expect(getResource).toHaveBeenCalledTimes(2);
  });

  it('keeps the operator’s pool when the linked record arrives during the lookup', async () => {
    getResource.mockReturnValue(new Promise(() => undefined));
    const chosen = makeRecord({ id: 'storage-chosen', name: 'chosen-pool' });
    const view = mountHighlight('?resource=storage-linked', [chosen]);
    await waitFor(() => expect(getResource).toHaveBeenCalled());

    view.setExpandedPoolIdSignal('storage-chosen');
    view.setRecords([chosen, makeRecord({ id: 'storage-linked', name: 'linked-pool' })]);
    expect(view.opened).toEqual([]);
    expect(view.result()).toBeNull();
  });
  it('reopens a pool when its link is followed again after an unresolved one', async () => {
    getResource.mockRejectedValue(new Error('not found'));
    const records = [
      makeRecord({ id: 'storage-a', name: 'pool-a' }),
      makeRecord({ id: 'storage-c', name: 'pool-c' }),
    ];
    const view = mountHighlight('?resource=storage-a', records);
    expect(view.opened).toEqual(['storage-a']);
    view.setExpandedPoolIdSignal('storage-c');

    view.setLocationSearch('?resource=pve1-missing');
    await waitFor(() => expect(getResource).toHaveBeenCalledTimes(1));
    view.setLocationSearch('?resource=storage-a');
    expect(view.opened).toEqual(['storage-a', 'storage-a']);
  });

  it('does not reopen the linked pool when Storage is shown again on the same link', () => {
    const view = mountHighlight('?resource=storage-a', [makeRecord({ id: 'storage-a' })]);
    expect(view.opened).toEqual(['storage-a']);
    view.setActive(false);
    view.setActive(true);
    expect(view.opened).toEqual(['storage-a']);
  });
  it('reopens a pool when its link returns after another tab’s link', () => {
    const records = [
      makeRecord({ id: 'storage-a', name: 'pool-a' }),
      makeRecord({ id: 'storage-c', name: 'pool-c' }),
    ];
    const view = mountHighlight('?resource=storage-a', records);
    view.setExpandedPoolIdSignal('storage-c');
    view.setActive(false);
    view.setLocationSearch('?resource=mock-cluster-1-pve2-121');
    view.setLocationSearch('?resource=storage-a');
    view.setActive(true);
    expect(view.opened).toEqual(['storage-a', 'storage-a']);
    expect(getResource).not.toHaveBeenCalled();
  });
  it('keeps the operator’s choice when they reopen and close a pool during the lookup', async () => {
    let resolveLookup: (value: { id: string }) => void = () => undefined;
    getResource.mockReturnValue(
      new Promise<{ id: string }>((resolve) => {
        resolveLookup = resolve;
      }),
    );
    const records = [
      makeRecord({ id: 'storage-linked', name: 'linked-pool' }),
      makeRecord({ id: 'storage-chosen', name: 'chosen-pool' }),
    ];
    const view = mountHighlight('?resource=pve1-linked-pool', records);
    await waitFor(() => expect(getResource).toHaveBeenCalled());

    view.setExpandedPoolIdSignal('storage-chosen');
    view.setExpandedPoolIdSignal(null);
    resolveLookup({ id: 'storage-linked' });
    await Promise.resolve();
    await Promise.resolve();
    expect(view.opened).toEqual([]);
  });
});
