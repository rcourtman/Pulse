import { createRoot, createSignal } from 'solid-js';
import { renderHook } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { useWorkloadGuestMetadataState } from '../useWorkloadGuestMetadataState';
import {
  RESOURCE_METADATA_CHANGED_EVENT,
  type ResourceMetadataChangedDetail,
} from '@/utils/resourceMetadataEvents';

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: {
    getAllMetadata: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('@/utils/apiClient', () => ({
  getOrgID: () => 'default',
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: vi.fn(() => () => undefined),
  },
}));

describe('useWorkloadGuestMetadataState', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('keeps an empty stable app-container metadata record when clearing a URL', () => {
    createRoot((dispose) => {
      const state = useWorkloadGuestMetadataState();

      state.handleCustomUrlUpdate('app-container:docker-main:name:grafana', '');

      expect(state.guestMetadata()['app-container:docker-main:name:grafana']).toEqual({
        id: 'app-container:docker-main:name:grafana',
        customUrl: '',
      });

      dispose();
    });
  });

  it('does not create empty records for non-stable metadata ids', () => {
    createRoot((dispose) => {
      const state = useWorkloadGuestMetadataState();

      state.handleCustomUrlUpdate('app-container:docker-main:runtime-id', '');

      expect(state.guestMetadata()['app-container:docker-main:runtime-id']).toBeUndefined();

      dispose();
    });
  });

  it('applies resource-metadata-changed events under the dispatched metadata id', () => {
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());

    window.dispatchEvent(
      new CustomEvent<ResourceMetadataChangedDetail>(RESOURCE_METADATA_CHANGED_EVENT, {
        detail: {
          metadataKind: 'guest',
          metadataId: 'pve1:node1:104',
          customUrl: 'https://svc.example.lan',
        },
      }),
    );

    expect(result.guestMetadata()['pve1:node1:104']?.customUrl).toBe('https://svc.example.lan');
    cleanup();
  });

  it('ignores agent metadata events', () => {
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());

    window.dispatchEvent(
      new CustomEvent<ResourceMetadataChangedDetail>(RESOURCE_METADATA_CHANGED_EVENT, {
        detail: {
          metadataKind: 'agent',
          metadataId: 'machine-1',
          customUrl: 'https://machine.example.lan',
        },
      }),
    );

    expect(result.guestMetadata()['machine-1']).toBeUndefined();
    cleanup();
  });
  it('reloads migrated metadata keys without losing the record contents', async () => {
    const original = {
      id: 'cluster-a:node-1:101',
      customUrl: 'https://guest.example',
      notes: ['Operator note'],
      tags: ['production'],
    };
    const migrated = { ...original, id: 'cluster-a:node-2:101' };
    vi.mocked(GuestMetadataAPI.getAllMetadata)
      .mockReset()
      .mockResolvedValueOnce({ [original.id]: original })
      .mockResolvedValueOnce({ [migrated.id]: migrated });
    const [ids, setIds] = createSignal<ReadonlySet<string>>(new Set([original.id]));
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState(ids));
    await Promise.resolve();
    expect(result.guestMetadata()[original.id]).toEqual(original);
    setIds(new Set([migrated.id]));
    await Promise.resolve();
    expect(GuestMetadataAPI.getAllMetadata).toHaveBeenCalledTimes(2);
    expect(result.guestMetadata()[migrated.id]).toEqual(migrated);
    expect(result.guestMetadata()[original.id]).toBeUndefined();
    cleanup();
  });

  it('does not let a stale bulk read overwrite a confirmed metadata edit', async () => {
    let finish!: (metadata: Record<string, GuestMetadata>) => void;
    vi.mocked(GuestMetadataAPI.getAllMetadata)
      .mockReset()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve;
          }),
      );
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
    result.handleCustomUrlUpdate('cluster-a:node-1:101', 'https://new.example');
    finish({
      'cluster-a:node-1:101': { id: 'cluster-a:node-1:101', customUrl: 'https://old.example' },
    });
    await Promise.resolve();
    expect(result.guestMetadata()['cluster-a:node-1:101']?.customUrl).toBe('https://new.example');
    cleanup();
  });

  it('accepts only the latest metadata-coordinate refresh', async () => {
    let finishOld!: (metadata: Record<string, GuestMetadata>) => void;
    vi.mocked(GuestMetadataAPI.getAllMetadata)
      .mockReset()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishOld = resolve;
          }),
      )
      .mockResolvedValueOnce({
        'cluster-a:node-2:101': { id: 'cluster-a:node-2:101', customUrl: 'https://new.example' },
      });
    const [ids, setIds] = createSignal<ReadonlySet<string>>(new Set(['cluster-a:node-1:101']));
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState(ids));
    setIds(new Set(['cluster-a:node-2:101']));
    await Promise.resolve();
    finishOld({
      'cluster-a:node-1:101': { id: 'cluster-a:node-1:101', customUrl: 'https://old.example' },
    });
    await Promise.resolve();
    expect(result.guestMetadata()['cluster-a:node-2:101']?.customUrl).toBe('https://new.example');
    expect(result.guestMetadata()['cluster-a:node-1:101']).toBeUndefined();
    cleanup();
  });
});
