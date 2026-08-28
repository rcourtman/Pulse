import XIcon from 'lucide-solid/icons/x';
import { Match, Switch } from 'solid-js';

import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { ActionIconButton } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';

import { AlertResourceIncidentsPanel } from './AlertResourceIncidentsPanel';
import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryAlert = ReturnType<AlertHistoryState['groupedAlerts']>[number]['alerts'][number];

export type MobileAlertHistoryInvestigation = {
  kind: 'timeline' | 'resource';
  alert: AlertHistoryAlert;
  rowKey: string;
};

interface MobileAlertHistoryInvestigationDialogProps {
  investigation: MobileAlertHistoryInvestigation;
  state: AlertHistoryState;
  onClose: () => void;
}

export function MobileAlertHistoryInvestigationDialog(
  props: MobileAlertHistoryInvestigationDialogProps,
) {
  const dialogTitle = () =>
    props.investigation.kind === 'timeline' ? 'Incident timeline' : 'Resource incidents';
  const dialogLabel = () => `${dialogTitle()} for ${props.investigation.alert.resourceName}`;
  const rowKey = () => props.investigation.rowKey;
  const alert = () => props.investigation.alert;

  return (
    <Dialog
      isOpen
      onClose={props.onClose}
      layout="drawer-right"
      panelClass="max-w-full sm:max-w-xl"
      ariaLabel={dialogLabel()}
    >
      <div class="flex h-full min-h-0 flex-col" data-testid="mobile-alert-investigation">
        <header class="flex shrink-0 items-start justify-between gap-3 border-b border-border bg-surface px-4 py-3">
          <div class="min-w-0">
            <h2 class="text-base font-semibold text-base-content">{dialogTitle()}</h2>
            <p class="truncate text-xs text-muted" title={alert().resourceName}>
              {alert().resourceName}
            </p>
          </div>
          <ActionIconButton
            type="button"
            onClick={props.onClose}
            label={`Close ${dialogTitle().toLowerCase()}`}
            title="Close"
            tone="muted"
            size="md"
          >
            <XIcon class="h-5 w-5" aria-hidden="true" />
          </ActionIconButton>
        </header>

        <div
          class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4"
          data-testid="mobile-alert-investigation-scroll"
        >
          <Switch>
            <Match when={props.investigation.kind === 'timeline'}>
              <IncidentTimelinePanel
                loading={() => props.state.incidentLoading()[rowKey()]}
                error={() => props.state.incidentErrors()[rowKey()]}
                timeline={() => props.state.incidentTimelines()[rowKey()]}
                filters={props.state.historyIncidentEventFilters}
                setFilters={props.state.setHistoryIncidentEventFilters}
                filterVariant="compact"
                eventCardVariant="surface"
                noteDraft={() => props.state.incidentNoteDrafts()[rowKey()] || ''}
                onNoteDraftChange={(value) => props.state.setIncidentNoteDraft(rowKey(), value)}
                noteSaving={() => props.state.incidentNoteSaving().has(rowKey())}
                onSaveNote={() => {
                  void props.state.saveIncidentNote(rowKey(), alert().id, alert().startTime);
                }}
                onRetry={() => {
                  void props.state.loadIncidentTimeline(rowKey(), alert().id, alert().startTime);
                }}
              />
            </Match>
            <Match when={props.investigation.kind === 'resource'}>
              <AlertResourceIncidentsPanel
                state={props.state}
                onClose={props.onClose}
                showCloseAction={false}
              />
            </Match>
          </Switch>
        </div>
      </div>
    </Dialog>
  );
}
