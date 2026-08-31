import { Show } from 'solid-js';
import Archive from 'lucide-solid/icons/archive';

import { Card } from '@/components/shared/Card';
import Toggle from '@/components/shared/Toggle';
import { TagInput } from '@/components/shared/TagInput';
import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import {
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
} from '@/features/alerts/thresholds/constants';
import {
  formatMetricValue,
  reconcileWarningCriticalEdit,
} from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxBackupsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('backups')}>
      <CollapsibleSection
        id="backups"
        title={state.sectionTitles.backups}
        collapsed={state.isCollapsed('backups')}
        onToggle={() => state.toggleSection('backups')}
        icon={<Archive class="w-5 h-5" />}
        isGloballyDisabled={!tableProps.backupDefaults().enabled}
        emptyMessage={state.BACKUP_THRESHOLDS_EMPTY_STATE}
      >
        <div ref={state.registerSection('backups')} class="scroll-mt-24">
          <ResourceTable
            title=""
            columns={['Fresh Hours', 'Stale Hours', 'Warning Days', 'Critical Days']}
            activeAlerts={tableProps.activeAlerts}
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={state.backupDefaultsRecord()}
            setGlobalDefaults={(value) => {
              state.updateBackupDefaults((prev) => {
                const currentRecord = {
                  'fresh hours': prev.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
                  'stale hours': prev.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
                  'warning days': prev.warningDays ?? 0,
                  'critical days': prev.criticalDays ?? 0,
                };
                const nextRecord =
                  typeof value === 'function'
                    ? value(currentRecord)
                    : { ...currentRecord, ...value };
                const days = reconcileWarningCriticalEdit(
                  {
                    warning: currentRecord['warning days'],
                    critical: currentRecord['critical days'],
                  },
                  {
                    warning:
                      typeof nextRecord['warning days'] === 'number'
                        ? nextRecord['warning days']
                        : currentRecord['warning days'],
                    critical:
                      typeof nextRecord['critical days'] === 'number'
                        ? nextRecord['critical days']
                        : currentRecord['critical days'],
                  },
                );
                return {
                  ...prev,
                  freshHours:
                    typeof nextRecord['fresh hours'] === 'number'
                      ? nextRecord['fresh hours']
                      : prev.freshHours,
                  staleHours:
                    typeof nextRecord['stale hours'] === 'number'
                      ? nextRecord['stale hours']
                      : prev.staleHours,
                  warningDays: days.warning,
                  criticalDays: days.critical,
                };
              });
            }}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={() => !tableProps.backupDefaults().enabled}
            onToggleGlobalDisable={() =>
              state.updateBackupDefaults((prev) => ({
                ...prev,
                enabled: !prev.enabled,
              }))
            }
            factoryDefaults={state.backupFactoryDefaultsRecord()}
            onResetDefaults={() => {
              if (tableProps.resetBackupDefaults) {
                tableProps.resetBackupDefaults();
                tableProps.setHasUnsavedChanges(true);
              } else {
                state.updateBackupDefaults(state.backupFactoryConfig());
              }
            }}
          />

          <Card padding="md" tone="card" class="mt-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">
                  {state.backupOrphanedPresentation.title}
                </h3>
                <p class="mt-1 text-xs text-muted">
                  {state.backupOrphanedPresentation.description}
                </p>
              </div>
              <Toggle
                checked={tableProps.backupDefaults().alertOrphaned ?? true}
                onToggle={() =>
                  state.updateBackupDefaults((prev) => ({
                    ...prev,
                    alertOrphaned: !(prev.alertOrphaned ?? true),
                  }))
                }
                label={
                  <span class="text-sm font-medium text-base-content">
                    {state.backupOrphanedPresentation.toggleLabel}
                  </span>
                }
                description={
                  <span class="text-xs text-muted">
                    {state.backupOrphanedPresentation.toggleDescription}
                  </span>
                }
                size="sm"
              />
            </div>
            <div class="mt-4">
              <span class="text-xs font-medium uppercase tracking-wide text-muted">
                {state.backupOrphanedPresentation.ignoreVmidsLabel}
              </span>
              <p class="mt-1 text-xs text-muted">
                {state.backupOrphanedPresentation.ignoreVmidsDescription}
              </p>
              <TagInput
                tags={tableProps.backupDefaults().ignoreVMIDs ?? []}
                label={state.backupOrphanedPresentation.ignoreVmidsLabel}
                onChange={(tags) => {
                  state.updateBackupDefaults((prev) => ({ ...prev, ignoreVMIDs: tags }));
                  tableProps.setHasUnsavedChanges(true);
                }}
                placeholder={state.backupOrphanedPresentation.ignoreVmidsPlaceholder}
              />
            </div>
          </Card>
        </div>
      </CollapsibleSection>
    </Show>
  );
}
