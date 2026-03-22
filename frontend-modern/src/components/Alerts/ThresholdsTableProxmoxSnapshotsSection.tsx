import { Show } from 'solid-js';
import Camera from 'lucide-solid/icons/camera';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxSnapshotsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('snapshots')}>
      <CollapsibleSection
        id="snapshots"
        title={state.sectionTitles.snapshots}
        collapsed={state.isCollapsed('snapshots')}
        onToggle={() => state.toggleSection('snapshots')}
        icon={<Camera class="w-5 h-5" />}
        isGloballyDisabled={!tableProps.snapshotDefaults().enabled}
        emptyMessage={state.SNAPSHOT_THRESHOLDS_EMPTY_STATE}
      >
        <div ref={state.registerSection('snapshots')} class="scroll-mt-24">
          <ResourceTable
            title=""
            resources={[
              {
                id: 'snapshots-defaults',
                name: 'Global Defaults',
                thresholds: state.snapshotDefaultsRecord(),
                defaults: state.snapshotDefaultsRecord(),
                editable: true,
                editScope: 'snapshot',
              },
            ]}
            columns={['Warning Days', 'Critical Days']}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage=""
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            showOfflineAlertsColumn={true}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) => state.handleBulkEdit(ids, ['Usage %', 'Temperature °C'])}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={state.snapshotDefaultsRecord()}
            setGlobalDefaults={(value) => {
              state.updateSnapshotDefaults((prev) => {
                const currentRecord = {
                  'warning days': prev.warningDays ?? 0,
                  'critical days': prev.criticalDays ?? 0,
                  'warning size (gib)': prev.warningSizeGiB ?? 0,
                  'critical size (gib)': prev.criticalSizeGiB ?? 0,
                };
                const nextRecord =
                  typeof value === 'function' ? value(currentRecord) : { ...currentRecord, ...value };
                return {
                  ...prev,
                  warningDays:
                    typeof nextRecord['warning days'] === 'number'
                      ? nextRecord['warning days']
                      : prev.warningDays,
                  criticalDays:
                    typeof nextRecord['critical days'] === 'number'
                      ? nextRecord['critical days']
                      : prev.criticalDays,
                  warningSizeGiB:
                    typeof nextRecord['warning size (gib)'] === 'number'
                      ? nextRecord['warning size (gib)']
                      : prev.warningSizeGiB,
                  criticalSizeGiB:
                    typeof nextRecord['critical size (gib)'] === 'number'
                      ? nextRecord['critical size (gib)']
                      : prev.criticalSizeGiB,
                };
              });
            }}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={() => !tableProps.snapshotDefaults().enabled}
            onToggleGlobalDisable={() =>
              state.updateSnapshotDefaults((prev) => ({
                ...prev,
                enabled: !prev.enabled,
              }))
            }
            factoryDefaults={state.snapshotFactoryDefaultsRecord()}
            onResetDefaults={() => {
              if (tableProps.resetSnapshotDefaults) {
                tableProps.resetSnapshotDefaults();
                tableProps.setHasUnsavedChanges(true);
              } else {
                state.updateSnapshotDefaults(state.snapshotFactoryConfig());
              }
            }}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
