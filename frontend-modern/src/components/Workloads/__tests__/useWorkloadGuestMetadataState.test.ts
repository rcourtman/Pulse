import { createRoot } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useWorkloadGuestMetadataState } from '../useWorkloadGuestMetadataState';

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
});
