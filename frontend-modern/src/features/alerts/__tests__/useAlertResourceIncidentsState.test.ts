import { createRoot } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AlertsAPI } from '@/api/alerts';
import type { Incident } from '@/types/api';
import { useAlertResourceIncidentsState } from '../useAlertResourceIncidentsState';

vi.mock('@/stores/notifications', () => ({ notificationStore: { error: vi.fn() } }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

describe('resource incident read failures', () => {
  afterEach(() => vi.restoreAllMocks());

  it('keeps failed reads distinct from empty evidence, including a cached refresh', async () => {
    let dispose!: () => void;
    const state = createRoot((cleanup) => {
      dispose = cleanup;
      return useAlertResourceIncidentsState();
    });
    const incident = { id: 'retained-occurrence', events: [] } as unknown as Incident;
    const read = vi.spyOn(AlertsAPI, 'getIncidentsForResource');
    try {
      read.mockRejectedValueOnce(new Error('canonical history unavailable'));
      await state.openResourceIncidentPanel('resource-1', 'Resource', 'row-1');
      expect(state.resourceIncidentError()['resource-1']).toBe(true);
      expect(state.resourceIncidents()['resource-1']).toBeUndefined();
      expect(state.resourceIncidentLoading()['resource-1']).toBe(false);

      read.mockResolvedValueOnce([incident]);
      await state.refreshResourceIncidentPanel();
      expect(state.resourceIncidentError()['resource-1']).toBe(false);
      expect(state.resourceIncidents()['resource-1']).toEqual([incident]);

      read.mockRejectedValueOnce(new Error('canonical history unavailable'));
      await state.refreshResourceIncidentPanel();
      expect(state.resourceIncidentError()['resource-1']).toBe(true);
      expect(state.resourceIncidents()['resource-1']).toEqual([incident]);

      read.mockResolvedValueOnce([]);
      await state.refreshResourceIncidentPanel();
      expect(state.resourceIncidentError()['resource-1']).toBe(false);
      expect(state.resourceIncidents()['resource-1']).toEqual([]);
      state.resetResourceIncidentsState();
      expect(state.resourceIncidentError()).toEqual({});
    } finally {
      dispose();
    }
  });
});
