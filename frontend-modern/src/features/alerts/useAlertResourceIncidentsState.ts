import { createSignal, onCleanup } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Incident } from '@/types/api';
import { logger } from '@/utils/logger';
import { getAlertResourceIncidentLoadFailure } from '@/utils/alertIncidentPresentation';

import { INCIDENT_EVENT_TYPES } from './types';

export function useAlertResourceIncidentsState() {
  const [resourceIncidentPanel, setResourceIncidentPanel] = createSignal<{
    resourceId: string;
    resourceName: string;
    // The row that opened the panel. The panel renders inline underneath that
    // row, and several alerts in the history can share one resource, so keying
    // on resourceId alone would open a copy under every one of them.
    rowKey: string;
  } | null>(null);
  const [resourceIncidents, setResourceIncidents] = createSignal<Record<string, Incident[]>>({});
  const [resourceIncidentLoading, setResourceIncidentLoading] = createSignal<
    Record<string, boolean>
  >({});
  const [expandedResourceIncidentIds, setExpandedResourceIncidentIds] = createSignal<Set<string>>(
    new Set(),
  );
  const [resourceIncidentEventFilters, setResourceIncidentEventFilters] = createSignal<Set<string>>(
    new Set(INCIDENT_EVENT_TYPES),
  );

  // Only the latest request for each resource owns its completion writes.
  // Clearing the map invalidates pending reads without cancelling other resources.
  const requests = new Map<string, symbol>();
  let disposed = false;
  onCleanup(() => {
    disposed = true;
    requests.clear();
  });

  const loadResourceIncidents = async (resourceId: string, limit = 10) => {
    if (!resourceId || disposed) return;

    const request = Symbol(resourceId);
    requests.set(resourceId, request);
    const ownsRequest = () => !disposed && requests.get(resourceId) === request;
    setResourceIncidentLoading((prev) => ({ ...prev, [resourceId]: true }));
    try {
      const incidents = await AlertsAPI.getIncidentsForResource(resourceId, limit);
      if (!ownsRequest()) return;
      setResourceIncidents((prev) => ({ ...prev, [resourceId]: incidents }));
    } catch (error) {
      if (!ownsRequest()) return;
      logger.error(getAlertResourceIncidentLoadFailure(), error);
      notificationStore.error(getAlertResourceIncidentLoadFailure());
    } finally {
      if (ownsRequest()) {
        requests.delete(resourceId);
        setResourceIncidentLoading((prev) => ({ ...prev, [resourceId]: false }));
      }
    }
  };

  const openResourceIncidentPanel = async (
    resourceId: string,
    resourceName: string,
    rowKey: string,
  ) => {
    if (!resourceId || disposed) return;

    // Clicking the same row's button again closes the panel, matching how the
    // neighbouring Timeline button toggles its own expansion.
    const current = resourceIncidentPanel();
    if (current && current.rowKey === rowKey) {
      setResourceIncidentPanel(null);
      return;
    }

    setResourceIncidentPanel({ resourceId, resourceName, rowKey });
    setExpandedResourceIncidentIds(new Set<string>());
    if (!(resourceId in resourceIncidents())) {
      await loadResourceIncidents(resourceId);
    }
  };

  const refreshResourceIncidentPanel = async () => {
    const selection = resourceIncidentPanel();
    if (!selection) return;
    await loadResourceIncidents(selection.resourceId);
  };

  const toggleResourceIncidentDetails = (incidentId: string) => {
    setExpandedResourceIncidentIds((prev) => {
      const next = new Set(prev);
      if (next.has(incidentId)) {
        next.delete(incidentId);
      } else {
        next.add(incidentId);
      }
      return next;
    });
  };

  const resetResourceIncidentsState = () => {
    requests.clear();
    setResourceIncidentPanel(null);
    setResourceIncidents({});
    setResourceIncidentLoading({});
    setExpandedResourceIncidentIds(new Set<string>());
    setResourceIncidentEventFilters(new Set(INCIDENT_EVENT_TYPES));
  };

  return {
    resourceIncidentPanel,
    setResourceIncidentPanel,
    resourceIncidents,
    resourceIncidentLoading,
    expandedResourceIncidentIds,
    resourceIncidentEventFilters,
    setResourceIncidentEventFilters,
    openResourceIncidentPanel,
    refreshResourceIncidentPanel,
    toggleResourceIncidentDetails,
    resetResourceIncidentsState,
  };
}
