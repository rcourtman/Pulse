import { Show } from 'solid-js';
import HardDrive from 'lucide-solid/icons/hard-drive';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxStorageSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('storage')}>
      <CollapsibleSection
        id="storage"
        title={state.sectionTitles.storage}
        resourceCount={tableProps.storage.length}
        collapsed={state.isCollapsed('storage')}
        onToggle={() => state.toggleSection('storage')}
        icon={<HardDrive class="w-5 h-5" />}
        isGloballyDisabled={tableProps.disableAllStorage()}
        emptyMessage={state.STORAGE_THRESHOLDS_EMPTY_STATE}
      >
        <div ref={state.registerSection('storage')} class="scroll-mt-24">
          <ResourceTable
            title=""
            groupedResources={state.storageGroupedByNode()}
            groupHeaderMeta={state.guestGroupHeaderMeta()}
            columns={['Usage %']}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.STORAGE_THRESHOLDS_FILTER_EMPTY_STATE}
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            onToggleDisabled={state.toggleDisabled}
            showOfflineAlertsColumn={false}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) => state.handleBulkEdit(ids, ['Usage %'])}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={{ usage: tableProps.storageDefault() }}
            setGlobalDefaults={(value) => {
              if (typeof value === 'function') {
                const nextValue = value({ usage: tableProps.storageDefault() });
                tableProps.setStorageDefault(nextValue.usage ?? 85);
              } else {
                tableProps.setStorageDefault(value.usage ?? 85);
              }
            }}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={tableProps.disableAllStorage}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllStorage(!tableProps.disableAllStorage())
            }
            showDelayColumn={true}
            globalDelaySeconds={tableProps.timeThresholds().storage}
            metricDelaySeconds={tableProps.metricTimeThresholds().storage ?? {}}
            onMetricDelayChange={(metric, value) => state.updateMetricDelay('storage', metric, value)}
            factoryDefaults={
              tableProps.factoryStorageDefault !== undefined
                ? { usage: tableProps.factoryStorageDefault }
                : undefined
            }
            onResetDefaults={tableProps.resetStorageDefault}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
