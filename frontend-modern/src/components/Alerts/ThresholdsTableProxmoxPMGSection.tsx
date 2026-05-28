import { Show } from 'solid-js';
import Mail from 'lucide-solid/icons/mail';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxPMGSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('pmg')}>
      <CollapsibleSection
        id="pmg"
        title={state.sectionTitles.pmg}
        resourceCount={state.pmgServersWithOverrides().length}
        collapsed={state.isCollapsed('pmg')}
        onToggle={() => state.toggleSection('pmg')}
        icon={<Mail class="w-5 h-5" />}
        isGloballyDisabled={tableProps.disableAllPMG()}
        emptyMessage={state.PMG_THRESHOLDS_EMPTY_STATE}
      >
        <div id="threshold-section-pmg" ref={state.registerSection('pmg')} class="scroll-mt-24">
          <ResourceTable
            title=""
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
      </CollapsibleSection>
    </Show>
  );
}
