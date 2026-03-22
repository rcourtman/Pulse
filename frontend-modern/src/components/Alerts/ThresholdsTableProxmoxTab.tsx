import { Show } from 'solid-js';
import Monitor from 'lucide-solid/icons/monitor';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Database from 'lucide-solid/icons/database';
import Archive from 'lucide-solid/icons/archive';
import Camera from 'lucide-solid/icons/camera';
import Server from 'lucide-solid/icons/server';

import { Card } from '@/components/shared/Card';
import Toggle from '@/components/shared/Toggle';
import { TagInput } from '@/components/shared/TagInput';
import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { ThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';
import {
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
} from '@/features/alerts/thresholds/constants';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';

interface ThresholdsTableTabProps {
  state: ThresholdsTableState;
  tableProps: ThresholdsTableProps;
}

export function ThresholdsTableProxmoxTab(props: ThresholdsTableTabProps) {
  const { state, tableProps } = props;

  return (
    <>
      <Show when={state.hasSection('nodes')}>
        <CollapsibleSection
          id="nodes"
          title={state.sectionTitles.nodes}
          resourceCount={state.nodesWithOverrides().length}
          collapsed={state.isCollapsed('nodes')}
          onToggle={() => state.toggleSection('nodes')}
          icon={<Server class="w-5 h-5" />}
          isGloballyDisabled={tableProps.disableAllNodes()}
          emptyMessage={state.NODE_THRESHOLDS_FILTER_EMPTY_STATE}
        >
          <div ref={state.registerSection('nodes')} class="scroll-mt-24">
            <ResourceTable
              title=""
              resources={state.nodesWithOverrides()}
              columns={['CPU %', 'Memory %', 'Disk %', 'Temp °C']}
              activeAlerts={tableProps.activeAlerts}
              emptyMessage={state.NODE_THRESHOLDS_FILTER_EMPTY_STATE}
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
                state.handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
              }
              formatMetricValue={formatMetricValue}
              hasActiveAlert={state.hasActiveAlert}
              globalDefaults={tableProps.nodeDefaults}
              setGlobalDefaults={tableProps.setNodeDefaults}
              setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
              globalDisableFlag={tableProps.disableAllNodes}
              onToggleGlobalDisable={() =>
                tableProps.setDisableAllNodes(!tableProps.disableAllNodes())
              }
              globalDisableOfflineFlag={tableProps.disableAllNodesOffline}
              onToggleGlobalDisableOffline={() =>
                tableProps.setDisableAllNodesOffline(!tableProps.disableAllNodesOffline())
              }
              showDelayColumn={true}
              globalDelaySeconds={tableProps.timeThresholds().node}
              metricDelaySeconds={tableProps.metricTimeThresholds().node ?? {}}
              onMetricDelayChange={(metric, value) => state.updateMetricDelay('node', metric, value)}
              factoryDefaults={tableProps.factoryNodeDefaults}
              onResetDefaults={tableProps.resetNodeDefaults}
            />
          </div>
        </CollapsibleSection>
      </Show>

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

      <CollapsibleSection
        id="guest-filtering"
        title={state.sectionTitles.guestFiltering}
        collapsed={state.isCollapsed('guest-filtering')}
        onToggle={() => state.toggleSection('guest-filtering')}
        icon={<Monitor class="w-5 h-5" />}
        emptyMessage={state.GUEST_FILTERING_EMPTY_STATE}
      >
        <div class="grid grid-cols-1 gap-6 p-4 xl:grid-cols-3">
          <Card padding="md" tone="card">
            <div class="mb-2">
              <h3 class="text-sm font-semibold text-base-content">
                {state.guestFilterPresentation.ignoredPrefixes.title}
              </h3>
              <p class="text-xs text-muted">
                {state.guestFilterPresentation.ignoredPrefixes.description}
              </p>
            </div>
            <TagInput
              tags={tableProps.ignoredGuestPrefixes()}
              onChange={(tags) => {
                tableProps.setIgnoredGuestPrefixes(tags);
                tableProps.setHasUnsavedChanges(true);
              }}
              placeholder={state.guestFilterPresentation.ignoredPrefixes.placeholder}
            />
          </Card>

          <Card padding="md" tone="card">
            <div class="mb-2">
              <h3 class="text-sm font-semibold text-base-content">
                {state.guestFilterPresentation.tagWhitelist.title}
              </h3>
              <p class="text-xs text-muted">
                {state.guestFilterPresentation.tagWhitelist.description}
              </p>
            </div>
            <TagInput
              tags={tableProps.guestTagWhitelist()}
              onChange={(tags) => {
                tableProps.setGuestTagWhitelist(tags);
                tableProps.setHasUnsavedChanges(true);
              }}
              placeholder={state.guestFilterPresentation.tagWhitelist.placeholder}
            />
          </Card>

          <Card padding="md" tone="card">
            <div class="mb-2">
              <h3 class="text-sm font-semibold text-base-content">
                {state.guestFilterPresentation.tagBlacklist.title}
              </h3>
              <p class="text-xs text-muted">
                {state.guestFilterPresentation.tagBlacklist.description}
              </p>
            </div>
            <TagInput
              tags={tableProps.guestTagBlacklist()}
              onChange={(tags) => {
                tableProps.setGuestTagBlacklist(tags);
                tableProps.setHasUnsavedChanges(true);
              }}
              placeholder={state.guestFilterPresentation.tagBlacklist.placeholder}
            />
          </Card>
        </div>
      </CollapsibleSection>

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
              resources={[
                {
                  id: 'backups-defaults',
                  name: 'Global Defaults',
                  thresholds: state.backupDefaultsRecord(),
                  defaults: state.backupDefaultsRecord(),
                  editable: true,
                  editScope: 'backup',
                },
              ]}
              columns={[
                'Fresh Hours',
                'Stale Hours',
                'Warning Days',
                'Critical Days',
                'Warning Size (GiB)',
                'Critical Size (GiB)',
              ]}
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
              onBulkEdit={(ids) => state.handleBulkEdit(ids, ['Usage %'])}
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
                    warningDays:
                      typeof nextRecord['warning days'] === 'number'
                        ? nextRecord['warning days']
                        : prev.warningDays,
                    criticalDays:
                      typeof nextRecord['critical days'] === 'number'
                        ? nextRecord['critical days']
                        : prev.criticalDays,
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
                <label class="text-xs font-medium uppercase tracking-wide text-muted">
                  {state.backupOrphanedPresentation.ignoreVmidsLabel}
                </label>
                <p class="mt-1 text-xs text-muted">
                  {state.backupOrphanedPresentation.ignoreVmidsDescription}
                </p>
                <TagInput
                  tags={tableProps.backupDefaults().ignoreVMIDs ?? []}
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
                    typeof value === 'function'
                      ? value(currentRecord)
                      : { ...currentRecord, ...value };
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
                  const newValue = value({ usage: tableProps.storageDefault() });
                  tableProps.setStorageDefault(newValue.usage ?? 85);
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
              onMetricDelayChange={(metric, value) =>
                state.updateMetricDelay('storage', metric, value)
              }
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
    </>
  );
}
