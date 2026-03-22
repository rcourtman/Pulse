import { renderHook } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';

import { INCIDENT_EVENT_TYPES } from '../types';
import { useAlertIncidentTimelineState } from '../useAlertIncidentTimelineState';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    addIncidentNote: vi.fn(),
    getIncidentTimeline: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('useAlertIncidentTimelineState', () => {
  beforeEach(() => {
    vi.mocked(AlertsAPI.getIncidentTimeline).mockReset();
    vi.mocked(AlertsAPI.addIncidentNote).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
  });

  it('owns shared incident timeline load, note-save, and reset behavior for alert surfaces', async () => {
    vi.mocked(AlertsAPI.getIncidentTimeline).mockResolvedValue({
      id: 'incident-1',
      events: [],
    } as any);
    vi.mocked(AlertsAPI.addIncidentNote).mockResolvedValue(undefined as any);

    const { result } = renderHook(() => useAlertIncidentTimelineState());

    expect(Array.from(result.eventFilters())).toEqual(INCIDENT_EVENT_TYPES);

    await result.toggleIncidentTimeline('row-1', 'alert-1', '2026-03-01T00:00:00Z');

    expect(result.expandedIncidents().has('row-1')).toBe(true);
    expect(AlertsAPI.getIncidentTimeline).toHaveBeenCalledWith(
      'alert-1',
      '2026-03-01T00:00:00Z',
    );
    expect(result.incidentTimelines()['row-1']).toMatchObject({ id: 'incident-1' });

    result.setIncidentNoteDraft('row-1', 'operator note');
    await result.saveIncidentNote('row-1', 'alert-1', '2026-03-01T00:00:00Z');

    expect(AlertsAPI.addIncidentNote).toHaveBeenCalledWith({
      alertIdentifier: 'alert-1',
      incidentId: 'incident-1',
      note: 'operator note',
    });
    expect(notificationStore.success).toHaveBeenCalledWith('Incident note saved');
    expect(result.incidentNoteDrafts()['row-1']).toBe('');

    result.setEventFilters(new Set(['note']));
    result.resetState();

    expect(result.incidentTimelines()).toEqual({});
    expect(result.expandedIncidents().size).toBe(0);
    expect(Array.from(result.eventFilters())).toEqual(INCIDENT_EVENT_TYPES);
  });
});
