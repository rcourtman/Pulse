// Synthetic state: this qualifies browser interaction, not backend delivery.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { AlertHistoryMobileList } from '@/features/alerts/AlertHistoryMobileList';
import type { AlertHistoryState } from '@/features/alerts/useAlertHistoryState';
import '@/index.css';
const stub = (callback = (..._args: any[]) => {}) => callback;
function createState() {
  const alert = {
    id: 'alert-1',
    source: 'alert',
    resourceId: 'node-1',
    resourceName: 'pve-production-01',
    resourceType: 'node',
    title: 'Backup',
    description: 'Backup failed after the target became unavailable.',
    severity: 'warning',
    status: 'resolved',
    duration: '14m',
    startTime: '2026-08-04T14:30:00.000Z',
    node: 'pve-production-01',
    nodeDisplayName: 'Production cluster',
    rawAlertType: 'backup_failed',
  };
  const [expandedIncidents, setExpandedIncidents] = createSignal(new Set<string>());
  const [resourceIncidentPanel, setResourceIncidentPanel] = createSignal<{
    resourceId: string;
    resourceName: string;
    rowKey: string;
  } | null>(null);
  const toggleIncidentTimeline = stub((rowKey: string) => {
    setExpandedIncidents((current) => {
      const next = new Set(current);
      if (next.has(rowKey)) next.delete(rowKey);
      else next.add(rowKey);
      return next;
    });
  });
  const openResourceIncidentPanel = stub(
    (resourceId: string, resourceName: string, rowKey: string) => {
      setResourceIncidentPanel({ resourceId, resourceName, rowKey });
    },
  );
  const closeResourceIncidentPanel = stub(() => setResourceIncidentPanel(null));

  const [groupedAlerts, setGroupedAlerts] = createSignal([
    {
      label: 'Today (August 4th)',
      fullLabel: 'Today, August 4th 2026',
      alerts: [alert],
    },
  ]);

  const incident = {
    id: 'incident-1',
    alertIdentifier: alert.id,
    alertType: 'backup_failed',
    level: 'warning',
    resourceId: 'node-1',
    resourceName: alert.resourceName,
    status: 'resolved',
    openedAt: alert.startTime,
    acknowledged: false,
    message: 'Backup destination unavailable.',
    events: [
      {
        id: 'event-1',
        type: 'alert_fired',
        timestamp: alert.startTime,
        summary: 'Backup destination unavailable.',
      },
    ],
  };
  const state = {
    groupedAlerts,
    getIncidentRowKey: () => 'alert-1-row',
    expandedIncidents,
    incidentLoading: () => ({}),
    incidentErrors: () => ({}),
    incidentTimelines: () => ({ 'alert-1-row': incident }),
    historyIncidentEventFilters: () => new Set(['alert_fired']),
    setHistoryIncidentEventFilters: stub(),
    incidentNoteDrafts: () => ({}),
    setIncidentNoteDraft: stub(),
    incidentNoteSaving: () => new Set<string>(),
    saveIncidentNote: stub(),
    loadIncidentTimeline: stub(),
    toggleIncidentTimeline,
    openResourceIncidentPanel,
    resourceIncidentPanel,
    resourceIncidents: () => ({ 'node-1': [incident] }),
    expandedResourceIncidentIds: () => new Set(['incident-1']),
    toggleResourceIncidentExpanded: stub(),
    resourceIncidentLoading: () => ({}),
    resourceIncidentEventFilters: () => new Set(['alert_fired']),
    setResourceIncidentEventFilters: stub(),
    refreshResourceIncidentPanel: stub(),
    setResourceIncidentPanel: closeResourceIncidentPanel,
    getResource: () => undefined,
    formatAlertRowTime: () => '14:30',
    formatAlertRowTimestamp: () => 'Tuesday, 4 August 2026 at 14:30:00',
  } as unknown as AlertHistoryState;

  return {
    closeResourceIncidentPanel,
    openResourceIncidentPanel,
    state,
    setGroupedAlerts,
    toggleIncidentTimeline,
  };
}

const fixture = createState();
Object.assign(window, { removeHistoryRows: () => fixture.setGroupedAlerts([]) });
render(() => <AlertHistoryMobileList state={fixture.state} />, document.getElementById('root')!);
