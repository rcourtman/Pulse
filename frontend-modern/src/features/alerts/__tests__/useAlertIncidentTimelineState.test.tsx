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

  it('keeps recurring alert timelines and note targets isolated when loads finish out of order', async () => {
    const oldStart = '2026-03-01T00:00:00Z';
    const newStart = '2026-03-02T00:00:00Z';
    type Timeline = Awaited<ReturnType<typeof AlertsAPI.getIncidentTimeline>>;
    const oldIncident = { id: 'incident-old', events: [] } as unknown as Timeline;
    const newIncident = { id: 'incident-new', events: [] } as unknown as Timeline;
    let finishOld!: (timeline: Timeline) => void;
    vi.mocked(AlertsAPI.getIncidentTimeline)
      .mockImplementationOnce(
        () =>
          new Promise<Timeline>((resolve) => {
            finishOld = resolve;
          }),
      )
      .mockResolvedValueOnce(newIncident)
      .mockResolvedValueOnce(oldIncident);
    vi.mocked(AlertsAPI.addIncidentNote).mockResolvedValue(undefined as any);

    const { result } = renderHook(() => useAlertIncidentTimelineState());
    const oldLoad = result.toggleIncidentTimeline('old-row', 'alert-1', oldStart);
    await result.toggleIncidentTimeline('new-row', 'alert-1', newStart);
    expect(result.incidentLoading()['old-row']).toBe(true);
    expect(result.incidentLoading()['new-row']).toBe(false);
    finishOld(oldIncident);
    await oldLoad;

    expect(AlertsAPI.getIncidentTimeline).toHaveBeenNthCalledWith(1, 'alert-1', oldStart);
    expect(AlertsAPI.getIncidentTimeline).toHaveBeenNthCalledWith(2, 'alert-1', newStart);
    expect(result.incidentTimelines()['old-row']).toEqual(oldIncident);
    expect(result.incidentTimelines()['new-row']).toEqual(newIncident);
    result.setIncidentNoteDraft('old-row', '  historical note  ');
    result.setIncidentNoteDraft('new-row', 'current draft');
    await result.saveIncidentNote('old-row', 'alert-1', oldStart);

    expect(AlertsAPI.addIncidentNote).toHaveBeenCalledExactlyOnceWith({
      alertIdentifier: 'alert-1',
      incidentId: 'incident-old',
      note: 'historical note',
    });
    expect(AlertsAPI.getIncidentTimeline).toHaveBeenNthCalledWith(3, 'alert-1', oldStart);
    expect(result.incidentNoteDrafts()['old-row']).toBe('');
    expect(result.incidentNoteDrafts()['new-row']).toBe('current draft');
    expect(result.incidentTimelines()['new-row']).toEqual(newIncident);
    expect(result.incidentNoteSaving().size).toBe(0);

    await result.toggleIncidentTimeline('old-row', 'alert-1', oldStart);
    expect(result.expandedIncidents().has('new-row')).toBe(true);
    await result.toggleIncidentTimeline('old-row', 'alert-1', oldStart);
    expect(AlertsAPI.getIncidentTimeline).toHaveBeenCalledTimes(3);
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
    expect(AlertsAPI.getIncidentTimeline).toHaveBeenCalledWith('alert-1', '2026-03-01T00:00:00Z');
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
