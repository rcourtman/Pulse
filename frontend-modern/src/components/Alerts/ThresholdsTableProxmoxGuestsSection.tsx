import { Show } from 'solid-js';
import Monitor from 'lucide-solid/icons/monitor';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxGuestsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('guests')}>
      <CollapsibleSection
        id="guests"
        title={state.sectionTitles.guests}
        resourceCount={tableProps.allGuests().length}
        collapsed={state.isCollapsed('guests')}
        onToggle={() => state.toggleSection('guests')}
        icon={<Monitor class="w-5 h-5" />}
        isGloballyDisabled={tableProps.disableAllGuests()}
        emptyMessage={state.GUEST_THRESHOLDS_EMPTY_STATE}
      >
        <div ref={state.registerSection('guests')} class="scroll-mt-24">
          <ResourceTable
            title=""
            groupedResources={state.guestsGroupedByNode()}
            groupHeaderMeta={state.guestGroupHeaderMeta()}
            columns={[
              'CPU %',
              'Memory %',
              'Disk %',
              'Backup',
              'Snapshot',
              'Disk R MB/s',
              'Disk W MB/s',
              'Net In MB/s',
              'Net Out MB/s',
            ]}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.GUEST_THRESHOLDS_FILTER_EMPTY_STATE}
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            onToggleDisabled={state.toggleDisabled}
            onToggleNodeConnectivity={state.toggleNodeConnectivity}
            onToggleBackup={state.toggleBackup}
            onToggleSnapshot={state.toggleSnapshot}
            showOfflineAlertsColumn={true}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) =>
              state.handleBulkEdit(ids, [
                'CPU %',
                'Memory %',
                'Disk %',
                'Disk R MB/s',
                'Disk W MB/s',
                'Net In MB/s',
                'Net Out MB/s',
              ])
            }
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={tableProps.guestDefaults}
            setGlobalDefaults={tableProps.setGuestDefaults}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={tableProps.disableAllGuests}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllGuests(!tableProps.disableAllGuests())
            }
            globalDisableOfflineFlag={() => tableProps.guestDisableConnectivity()}
            onToggleGlobalDisableOffline={() =>
              tableProps.setGuestDisableConnectivity(!tableProps.guestDisableConnectivity())
            }
            globalOfflineSeverity={tableProps.guestPoweredOffSeverity()}
            onSetGlobalOfflineState={(nextState) => {
              if (nextState === 'off') {
                tableProps.setGuestDisableConnectivity(true);
              } else {
                tableProps.setGuestDisableConnectivity(false);
                tableProps.setGuestPoweredOffSeverity(
                  nextState === 'critical' ? 'critical' : 'warning',
                );
              }
              tableProps.setHasUnsavedChanges(true);
            }}
            onSetOfflineState={state.setOfflineState}
            showDelayColumn={true}
            globalDelaySeconds={tableProps.timeThresholds().guest}
            metricDelaySeconds={tableProps.metricTimeThresholds().guest ?? {}}
            onMetricDelayChange={(metric, value) => state.updateMetricDelay('guest', metric, value)}
            factoryDefaults={tableProps.factoryGuestDefaults}
            onResetDefaults={tableProps.resetGuestDefaults}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
