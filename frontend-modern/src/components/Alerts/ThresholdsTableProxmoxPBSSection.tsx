import { Show } from 'solid-js';
import Database from 'lucide-solid/icons/database';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxPBSSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('pbs')}>
      <CollapsibleSection
        id="pbs"
        title={state.sectionTitles.pbs}
        resourceCount={state.pbsServersWithOverrides().length}
        collapsed={state.isCollapsed('pbs')}
        onToggle={() => state.toggleSection('pbs')}
        icon={<Database class="w-5 h-5" />}
        isGloballyDisabled={tableProps.disableAllPBS()}
        emptyMessage={state.PBS_THRESHOLDS_EMPTY_STATE}
      >
        <div ref={state.registerSection('pbs')} class="scroll-mt-24">
          <ResourceTable
            title=""
            resources={state.pbsServersWithOverrides()}
            columns={['CPU %', 'Memory %']}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.PBS_THRESHOLDS_FILTER_EMPTY_STATE}
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
            onBulkEdit={(ids) =>
              state.handleBulkEdit(ids, [
                'CPU %',
                'Memory %',
                'Disk R MB/s',
                'Disk W MB/s',
                'Net In MB/s',
                'Net Out MB/s',
              ])
            }
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={tableProps.pbsDefaults ?? { cpu: 80, memory: 85 }}
            setGlobalDefaults={tableProps.setPBSDefaults}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={tableProps.disableAllPBS}
            onToggleGlobalDisable={() => tableProps.setDisableAllPBS(!tableProps.disableAllPBS())}
            globalDisableOfflineFlag={tableProps.disableAllPBSOffline}
            onToggleGlobalDisableOffline={() =>
              tableProps.setDisableAllPBSOffline(!tableProps.disableAllPBSOffline())
            }
            showDelayColumn={true}
            globalDelaySeconds={tableProps.timeThresholds().pbs}
            metricDelaySeconds={tableProps.metricTimeThresholds().pbs ?? {}}
            onMetricDelayChange={(metric, value) => state.updateMetricDelay('pbs', metric, value)}
            factoryDefaults={tableProps.factoryPBSDefaults}
            onResetDefaults={tableProps.resetPBSDefaults}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
