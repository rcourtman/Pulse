import { Show } from 'solid-js';

import { ResourceTable } from './ResourceTable';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { ThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';

interface ThresholdsTableTabProps {
  state: ThresholdsTableState;
  tableProps: ThresholdsTableProps;
}

export function ThresholdsTablePMGTab(props: ThresholdsTableTabProps) {
  const { state, tableProps } = props;

  return (
    <Show
      when={state.pmgServersWithOverrides().length > 0}
      fallback={
        <div class="rounded-md border border-border bg-surface p-6 text-sm text-muted">
          {state.PMG_THRESHOLDS_EMPTY_STATE}
        </div>
      }
    >
      <div ref={state.registerSection('pmg')} class="scroll-mt-24">
        <ResourceTable
          title={state.sectionTitles.pmg}
          resources={state.pmgServersWithOverrides()}
          columns={[
            'Queue Warn',
            'Queue Crit',
            'Deferred Warn',
            'Deferred Crit',
            'Hold Warn',
            'Hold Crit',
            'Oldest Warn (min)',
            'Oldest Crit (min)',
            'Spam Warn',
            'Spam Crit',
            'Virus Warn',
            'Virus Crit',
            'Growth Warn %',
            'Growth Warn Min',
            'Growth Crit %',
            'Growth Crit Min',
          ]}
          activeAlerts={tableProps.activeAlerts}
          emptyMessage={state.PMG_THRESHOLDS_FILTER_EMPTY_STATE}
          onEdit={state.startEditing}
          onSaveEdit={state.saveEdit}
          onCancelEdit={state.cancelEdit}
          onRemoveOverride={state.removeOverride}
          onToggleDisabled={state.toggleDisabled}
          onToggleNodeConnectivity={state.toggleNodeConnectivity}
          showOfflineAlertsColumn={true}
          editingId={state.editingId}
          editingThresholds={state.editingThresholds}
          setEditingThresholds={state.setEditingThresholds}
          editingNote={state.editingNote}
          setEditingNote={state.setEditingNote}
          onBulkEdit={(ids) => state.handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %'])}
          formatMetricValue={formatMetricValue}
          hasActiveAlert={state.hasActiveAlert}
          globalDefaults={state.pmgGlobalDefaults()}
          setGlobalDefaults={state.setPMGGlobalDefaults}
          setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
          globalDisableFlag={tableProps.disableAllPMG}
          onToggleGlobalDisable={() => tableProps.setDisableAllPMG(!tableProps.disableAllPMG())}
          globalDisableOfflineFlag={tableProps.disableAllPMGOffline}
          onToggleGlobalDisableOffline={() =>
            tableProps.setDisableAllPMGOffline(!tableProps.disableAllPMGOffline())
          }
        />
      </div>
    </Show>
  );
}
