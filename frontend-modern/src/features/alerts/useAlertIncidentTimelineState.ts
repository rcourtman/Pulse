import { createSignal } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Incident } from '@/types/api';
import {
  getAlertResourceIncidentNoteSaveFailure,
  getAlertResourceIncidentNoteSavedLabel,
  getAlertResourceIncidentTimelineFailure,
} from '@/utils/alertIncidentPresentation';
import { logger } from '@/utils/logger';

import { INCIDENT_EVENT_TYPES } from './types';

const createIncidentEventFilterSet = () => new Set<string>(INCIDENT_EVENT_TYPES);

export function useAlertIncidentTimelineState() {
  const [incidentTimelines, setIncidentTimelines] = createSignal<Record<string, Incident | null>>(
    {},
  );
  const [incidentLoading, setIncidentLoading] = createSignal<Record<string, boolean>>({});
  const [incidentErrors, setIncidentErrors] = createSignal<Record<string, boolean>>({});
  const [expandedIncidents, setExpandedIncidents] = createSignal<Set<string>>(new Set());
  const [incidentNoteDrafts, setIncidentNoteDrafts] = createSignal<Record<string, string>>({});
  const [incidentNoteSaving, setIncidentNoteSaving] = createSignal<Set<string>>(new Set());
  const [eventFilters, setEventFilters] = createSignal<Set<string>>(createIncidentEventFilterSet());

  const resetState = () => {
    setIncidentTimelines({});
    setIncidentLoading({});
    setIncidentErrors({});
    setExpandedIncidents(new Set());
    setIncidentNoteDrafts({});
    setIncidentNoteSaving(new Set());
    setEventFilters(createIncidentEventFilterSet());
  };

  const loadIncidentTimeline = async (key: string, alertIdentifier: string, startedAt?: string) => {
    setIncidentLoading((prev) => ({ ...prev, [key]: true }));
    try {
      const timeline = await AlertsAPI.getIncidentTimeline(alertIdentifier, startedAt);
      setIncidentTimelines((prev) => ({ ...prev, [key]: timeline }));
      setIncidentErrors((prev) => ({ ...prev, [key]: false }));
    } catch (error) {
      logger.error(getAlertResourceIncidentTimelineFailure(), error);
      notificationStore.error(getAlertResourceIncidentTimelineFailure());
      setIncidentErrors((prev) => ({ ...prev, [key]: true }));
    } finally {
      setIncidentLoading((prev) => ({ ...prev, [key]: false }));
    }
  };

  const toggleIncidentTimeline = async (key: string, alertIdentifier: string, startedAt?: string) => {
    const next = new Set(expandedIncidents());
    if (next.has(key)) {
      next.delete(key);
      setExpandedIncidents(next);
      return;
    }

    next.add(key);
    setExpandedIncidents(next);

    if (!(key in incidentTimelines())) {
      await loadIncidentTimeline(key, alertIdentifier, startedAt);
    }
  };

  const setIncidentNoteDraft = (key: string, value: string) => {
    setIncidentNoteDrafts((prev) => ({ ...prev, [key]: value }));
  };

  const saveIncidentNote = async (key: string, alertIdentifier: string, startedAt?: string) => {
    const note = (incidentNoteDrafts()[key] || '').trim();
    if (!note) {
      return;
    }

    setIncidentNoteSaving((prev) => {
      const next = new Set(prev);
      next.add(key);
      return next;
    });

    try {
      const incidentId = incidentTimelines()[key]?.id;
      await AlertsAPI.addIncidentNote({ alertIdentifier, incidentId, note });
      setIncidentNoteDraft(key, '');
      await loadIncidentTimeline(key, alertIdentifier, startedAt);
      notificationStore.success(getAlertResourceIncidentNoteSavedLabel());
    } catch (error) {
      logger.error(getAlertResourceIncidentNoteSaveFailure(), error);
      notificationStore.error(getAlertResourceIncidentNoteSaveFailure());
    } finally {
      setIncidentNoteSaving((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  };

  return {
    incidentTimelines,
    incidentLoading,
    incidentErrors,
    expandedIncidents,
    incidentNoteDrafts,
    incidentNoteSaving,
    eventFilters,
    setEventFilters,
    resetState,
    loadIncidentTimeline,
    toggleIncidentTimeline,
    setIncidentNoteDraft,
    saveIncidentNote,
  };
}
