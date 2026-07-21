import { createRoot } from 'solid-js';
import { renderHook } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

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
});
