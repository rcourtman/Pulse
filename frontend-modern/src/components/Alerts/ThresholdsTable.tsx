import { Show, For } from 'solid-js';
import Toggle from '@/components/shared/Toggle';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { TagInput } from '@/components/shared/TagInput';
import Server from 'lucide-solid/icons/server';
import Monitor from 'lucide-solid/icons/monitor';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Database from 'lucide-solid/icons/database';
import Archive from 'lucide-solid/icons/archive';
import Camera from 'lucide-solid/icons/camera';
import Mail from 'lucide-solid/icons/mail';
import Users from 'lucide-solid/icons/users';
import Boxes from 'lucide-solid/icons/boxes';

// Workaround for eslint false-positive when `For` is used only in JSX
const __ensureForUsage = For;
void __ensureForUsage;

import { ResourceTable } from './ResourceTable';
import { BulkEditDialog } from './BulkEditDialog';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import {
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
} from '@/features/alerts/thresholds/constants';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import { useThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';

export function ThresholdsTable(props: ThresholdsTableProps) {
  const {
    activeTab,
    agentDisksGroupedByAgent,
    agentDisksWithOverrides,
    agentsWithOverrides,
    backupDefaultsRecord,
    backupFactoryConfig,
    backupFactoryDefaultsRecord,
    backupOrphanedPresentation,
    BACKUP_THRESHOLDS_EMPTY_STATE,
    bulkEditColumns,
    bulkEditIds,
    cancelEdit,
    collapseAll,
    CONTAINERS_FILTER_EMPTY_STATE,
    CONTAINER_RUNTIMES_FILTER_EMPTY_STATE,
    dismissHelpBanner,
    dockerContainersGroupedByHost,
    dockerHostGroupMeta,
    dockerHostsWithOverrides,
    dockerIgnoredInput,
    dockerIgnoredPrefixesPresentation,
    dockerServicePresentation,
    editingId,
    editingNote,
    editingThresholds,
    expandAll,
    GUEST_FILTERING_EMPTY_STATE,
    GUEST_THRESHOLDS_EMPTY_STATE,
    GUEST_THRESHOLDS_FILTER_EMPTY_STATE,
    guestFilterPresentation,
    guestGroupHeaderMeta,
    guestsGroupedByNode,
    handleBulkEdit,
    handleDockerIgnoredChange,
    handleResetDockerIgnored,
    handleSaveBulkEdit,
    handleTabClick,
    hasActiveAlert,
    hasSection,
    helpBannerDismissed,
    isBulkEditDialogOpen,
    isCollapsed,
    nodesWithOverrides,
    NODE_THRESHOLDS_FILTER_EMPTY_STATE,
    pbsServersWithOverrides,
    PBS_THRESHOLDS_EMPTY_STATE,
    PBS_THRESHOLDS_FILTER_EMPTY_STATE,
    pmgGlobalDefaults,
    PMG_THRESHOLDS_EMPTY_STATE,
    PMG_THRESHOLDS_FILTER_EMPTY_STATE,
    pmgServersWithOverrides,
    registerSection,
    removeOverride,
    saveEdit,
    searchTerm,
    sectionTitles,
    serviceCriticalInputId,
    serviceGapValidationMessage,
    serviceWarnInputId,
    setEditingNote,
    setEditingThresholds,
    setIsBulkEditDialogOpen,
    setOfflineState,
    setPMGGlobalDefaults,
    setSearchTerm,
    SNAPSHOT_THRESHOLDS_EMPTY_STATE,
    snapshotDefaultsRecord,
    snapshotFactoryConfig,
    snapshotFactoryDefaultsRecord,
    startEditing,
    storageGroupedByNode,
    STORAGE_THRESHOLDS_EMPTY_STATE,
    STORAGE_THRESHOLDS_FILTER_EMPTY_STATE,
    toggleBackup,
    toggleDisabled,
    toggleNodeConnectivity,
    toggleSection,
    toggleSnapshot,
    updateBackupDefaults,
    updateMetricDelay,
    updateSnapshotDefaults,
    AGENT_DISKS_EMPTY_STATE,
    AGENT_DISKS_FILTER_EMPTY_STATE,
    AGENT_THRESHOLDS_FILTER_EMPTY_STATE,
    getAlertThresholdsHelpBanner,
    getAlertThresholdsHelpDismissLabel,
    getAlertThresholdsSearchPlaceholder,
  } = useThresholdsTableState(props);

  return (
    <div class="space-y-4">
      {/* Search Bar */}
      <div class="relative">
        <SearchInput
          value={searchTerm}
          onChange={setSearchTerm}
          placeholder={getAlertThresholdsSearchPlaceholder()}
          class="w-full"
          onBeforeAutoFocus={() => Boolean(editingId())}
          focusOnShortcut
          clearOnEscape
          shortcutHint="Ctrl+F"
        />
      </div>

      {/* Help Banner - Dismissible */}
      <Show when={!helpBannerDismissed()}>
        <div class="rounded-md border border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900 p-3 relative group">
          <button
            type="button"
            onClick={dismissHelpBanner}
            class="absolute top-2 right-2 p-1 rounded-md text-blue-400 hover:text-blue-600 dark:text-blue-500 dark:hover:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900 opacity-0 group-hover:opacity-100 transition-opacity"
            title={getAlertThresholdsHelpDismissLabel()}
            aria-label={getAlertThresholdsHelpDismissLabel()}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          <div class="flex items-start gap-2 pr-6">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="text-sm text-blue-900 dark:text-blue-100">
              <span class="font-medium">{getAlertThresholdsHelpBanner().title}</span> Set any
              threshold to{' '}
              <code class="px-1 py-0.5 bg-blue-100 dark:bg-blue-900 rounded text-xs font-mono">
                {getAlertThresholdsHelpBanner().disableValue}
              </code>{' '}
              to disable alerts for that metric. Click on disabled thresholds showing{' '}
              <span class="italic">{getAlertThresholdsHelpBanner().reenableLabel}</span> to
              re-enable them. Resources with custom settings show a{' '}
              <span class="inline-flex items-center px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded text-xs">
                {getAlertThresholdsHelpBanner().customBadgeLabel}
              </span>{' '}
              badge.{' '}
              <span class="text-blue-600 dark:text-blue-400">
                {getAlertThresholdsHelpBanner().collapseHint}
              </span>
            </div>
          </div>
        </div>
      </Show>

      {/* Tab Navigation */}
      <div class="border-b border-border">
        <nav class="-mb-px flex gap-4 sm:gap-6" aria-label="Tabs">
          <button
            type="button"
            onClick={() => handleTabClick('proxmox')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'proxmox' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Server class="w-4 h-4" />
            <span class="hidden sm:inline">Proxmox / PBS</span>
            <span class="sm:hidden">Proxmox</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('pmg')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'pmg' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Mail class="w-4 h-4" />
            <span class="hidden sm:inline">Mail Gateway</span>
            <span class="sm:hidden">Mail</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('agents')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'agents' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Users class="w-4 h-4" />
            <span>Agents</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('docker')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'docker' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Boxes class="w-4 h-4" />
            <span>Containers</span>
          </button>
        </nav>
      </div>

      {/* Section Controls - Only show on Proxmox tab which has multiple sections */}
      <Show when={activeTab() === 'proxmox'}>
        <div class="flex justify-end gap-2">
          <button
            type="button"
            onClick={expandAll}
            class="text-xs px-2 py-1 hover:text-muted hover:bg-surface-hover rounded transition-colors"
          >
            Expand all
          </button>
          <span class="text-muted">|</span>
          <button
            type="button"
            onClick={collapseAll}
            class="text-xs px-2 py-1 hover:text-muted hover:bg-surface-hover rounded transition-colors"
          >
            Collapse all
          </button>
        </div>
      </Show>

      <div class="space-y-6">
        <Show when={activeTab() === 'proxmox'}>
          <Show when={hasSection('nodes')}>
            <CollapsibleSection
              id="nodes"
              title={sectionTitles.nodes}
              resourceCount={nodesWithOverrides().length}
              collapsed={isCollapsed('nodes')}
              onToggle={() => toggleSection('nodes')}
              icon={<Server class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllNodes()}
              emptyMessage={NODE_THRESHOLDS_FILTER_EMPTY_STATE}
            >
              <div ref={registerSection('nodes')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={nodesWithOverrides()}
                  columns={['CPU %', 'Memory %', 'Disk %', 'Temp °C']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={NODE_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.nodeDefaults}
                  setGlobalDefaults={props.setNodeDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllNodes}
                  onToggleGlobalDisable={() => props.setDisableAllNodes(!props.disableAllNodes())}
                  globalDisableOfflineFlag={props.disableAllNodesOffline}
                  onToggleGlobalDisableOffline={() =>
                    props.setDisableAllNodesOffline(!props.disableAllNodesOffline())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().node}
                  metricDelaySeconds={props.metricTimeThresholds().node ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('node', metric, value)}
                  factoryDefaults={props.factoryNodeDefaults}
                  onResetDefaults={props.resetNodeDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('pbs')}>
            <CollapsibleSection
              id="pbs"
              title={sectionTitles.pbs}
              resourceCount={pbsServersWithOverrides().length}
              collapsed={isCollapsed('pbs')}
              onToggle={() => toggleSection('pbs')}
              icon={<Database class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllPBS()}
              emptyMessage={PBS_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('pbs')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={pbsServersWithOverrides()}
                  columns={['CPU %', 'Memory %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={PBS_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, [
                      'CPU %',
                      'Memory %',
                      'Disk R MB/s',
                      'Disk W MB/s',
                      'Net In MB/s',
                      'Net Out MB/s',
                    ])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.pbsDefaults ?? { cpu: 80, memory: 85 }}
                  setGlobalDefaults={props.setPBSDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllPBS}
                  onToggleGlobalDisable={() => props.setDisableAllPBS(!props.disableAllPBS())}
                  globalDisableOfflineFlag={props.disableAllPBSOffline}
                  onToggleGlobalDisableOffline={() =>
                    props.setDisableAllPBSOffline(!props.disableAllPBSOffline())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().pbs}
                  metricDelaySeconds={props.metricTimeThresholds().pbs ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('pbs', metric, value)}
                  factoryDefaults={props.factoryPBSDefaults}
                  onResetDefaults={props.resetPBSDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('guests')}>
            <CollapsibleSection
              id="guests"
              title={sectionTitles.guests}
              resourceCount={props.allGuests().length}
              collapsed={isCollapsed('guests')}
              onToggle={() => toggleSection('guests')}
              icon={<Monitor class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllGuests()}
              emptyMessage={GUEST_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('guests')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={guestsGroupedByNode()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
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
                  activeAlerts={props.activeAlerts}
                  emptyMessage={GUEST_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  onToggleBackup={toggleBackup}
                  onToggleSnapshot={toggleSnapshot}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, [
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
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.guestDefaults}
                  setGlobalDefaults={props.setGuestDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllGuests}
                  onToggleGlobalDisable={() => props.setDisableAllGuests(!props.disableAllGuests())}
                  globalDisableOfflineFlag={() => props.guestDisableConnectivity()}
                  onToggleGlobalDisableOffline={() =>
                    props.setGuestDisableConnectivity(!props.guestDisableConnectivity())
                  }
                  globalOfflineSeverity={props.guestPoweredOffSeverity()}
                  onSetGlobalOfflineState={(state) => {
                    if (state === 'off') {
                      props.setGuestDisableConnectivity(true);
                    } else {
                      props.setGuestDisableConnectivity(false);
                      props.setGuestPoweredOffSeverity(
                        state === 'critical' ? 'critical' : 'warning',
                      );
                    }
                    props.setHasUnsavedChanges(true);
                  }}
                  onSetOfflineState={setOfflineState}
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().guest}
                  metricDelaySeconds={props.metricTimeThresholds().guest ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('guest', metric, value)}
                  factoryDefaults={props.factoryGuestDefaults}
                  onResetDefaults={props.resetGuestDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={activeTab() === 'proxmox'}>
            <CollapsibleSection
              id="guest-filtering"
              title={sectionTitles.guestFiltering}
              collapsed={isCollapsed('guest-filtering')}
              onToggle={() => toggleSection('guest-filtering')}
              icon={<Monitor class="w-5 h-5" />}
              emptyMessage={GUEST_FILTERING_EMPTY_STATE}
            >
              <div class="grid grid-cols-1 gap-6 p-4 xl:grid-cols-3">
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-base-content">
                      {guestFilterPresentation.ignoredPrefixes.title}
                    </h3>
                    <p class="text-xs text-muted">
                      {guestFilterPresentation.ignoredPrefixes.description}
                    </p>
                  </div>
                  <TagInput
                    tags={props.ignoredGuestPrefixes()}
                    onChange={(tags) => {
                      props.setIgnoredGuestPrefixes(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder={guestFilterPresentation.ignoredPrefixes.placeholder}
                  />
                </Card>
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-base-content">
                      {guestFilterPresentation.tagWhitelist.title}
                    </h3>
                    <p class="text-xs text-muted">
                      {guestFilterPresentation.tagWhitelist.description}
                    </p>
                  </div>
                  <TagInput
                    tags={props.guestTagWhitelist()}
                    onChange={(tags) => {
                      props.setGuestTagWhitelist(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder={guestFilterPresentation.tagWhitelist.placeholder}
                  />
                </Card>
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-base-content">
                      {guestFilterPresentation.tagBlacklist.title}
                    </h3>
                    <p class="text-xs text-muted">
                      {guestFilterPresentation.tagBlacklist.description}
                    </p>
                  </div>
                  <TagInput
                    tags={props.guestTagBlacklist()}
                    onChange={(tags) => {
                      props.setGuestTagBlacklist(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder={guestFilterPresentation.tagBlacklist.placeholder}
                  />
                </Card>
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('backups')}>
            <CollapsibleSection
              id="backups"
              title={sectionTitles.backups}
              collapsed={isCollapsed('backups')}
              onToggle={() => toggleSection('backups')}
              icon={<Archive class="w-5 h-5" />}
              isGloballyDisabled={!props.backupDefaults().enabled}
              emptyMessage={BACKUP_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('backups')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={[
                    {
                      id: 'backups-defaults',
                      name: 'Global Defaults',
                      thresholds: backupDefaultsRecord(),
                      defaults: backupDefaultsRecord(),
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
                  activeAlerts={props.activeAlerts}
                  emptyMessage=""
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={backupDefaultsRecord()}
                  setGlobalDefaults={(value) => {
                    updateBackupDefaults((prev) => {
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
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={() => !props.backupDefaults().enabled}
                  onToggleGlobalDisable={() =>
                    updateBackupDefaults((prev) => ({
                      ...prev,
                      enabled: !prev.enabled,
                    }))
                  }
                  factoryDefaults={backupFactoryDefaultsRecord()}
                  onResetDefaults={() => {
                    if (props.resetBackupDefaults) {
                      props.resetBackupDefaults();
                      props.setHasUnsavedChanges(true);
                    } else {
                      updateBackupDefaults(backupFactoryConfig());
                    }
                  }}
                />
                <Card padding="md" tone="card" class="mt-6">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <h3 class="text-sm font-semibold text-base-content">
                        {backupOrphanedPresentation.title}
                      </h3>
                      <p class="mt-1 text-xs text-muted">
                        {backupOrphanedPresentation.description}
                      </p>
                    </div>
                    <Toggle
                      checked={props.backupDefaults().alertOrphaned ?? true}
                      onToggle={() =>
                        updateBackupDefaults((prev) => ({
                          ...prev,
                          alertOrphaned: !(prev.alertOrphaned ?? true),
                        }))
                      }
                      label={
                        <span class="text-sm font-medium text-base-content">
                          {backupOrphanedPresentation.toggleLabel}
                        </span>
                      }
                      description={
                        <span class="text-xs text-muted">
                          {backupOrphanedPresentation.toggleDescription}
                        </span>
                      }
                      size="sm"
                    />
                  </div>
                  <div class="mt-4">
                    <label class="text-xs font-medium uppercase tracking-wide text-muted">
                      {backupOrphanedPresentation.ignoreVmidsLabel}
                    </label>
                    <p class="mt-1 text-xs text-muted">
                      {backupOrphanedPresentation.ignoreVmidsDescription}
                    </p>
                    <TagInput
                      tags={props.backupDefaults().ignoreVMIDs ?? []}
                      onChange={(tags) => {
                        updateBackupDefaults((prev) => ({ ...prev, ignoreVMIDs: tags }));
                        props.setHasUnsavedChanges(true);
                      }}
                      placeholder={backupOrphanedPresentation.ignoreVmidsPlaceholder}
                    />
                  </div>
                </Card>
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('snapshots')}>
            <CollapsibleSection
              id="snapshots"
              title={sectionTitles.snapshots}
              collapsed={isCollapsed('snapshots')}
              onToggle={() => toggleSection('snapshots')}
              icon={<Camera class="w-5 h-5" />}
              isGloballyDisabled={!props.snapshotDefaults().enabled}
              emptyMessage={SNAPSHOT_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('snapshots')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={[
                    {
                      id: 'snapshots-defaults',
                      name: 'Global Defaults',
                      thresholds: snapshotDefaultsRecord(),
                      defaults: snapshotDefaultsRecord(),
                      editable: true,
                      editScope: 'snapshot',
                    },
                  ]}
                  columns={['Warning Days', 'Critical Days']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage=""
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %', 'Temperature °C'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={snapshotDefaultsRecord()}
                  setGlobalDefaults={(value) => {
                    updateSnapshotDefaults((prev) => {
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
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={() => !props.snapshotDefaults().enabled}
                  onToggleGlobalDisable={() =>
                    updateSnapshotDefaults((prev) => ({
                      ...prev,
                      enabled: !prev.enabled,
                    }))
                  }
                  factoryDefaults={snapshotFactoryDefaultsRecord()}
                  onResetDefaults={() => {
                    if (props.resetSnapshotDefaults) {
                      props.resetSnapshotDefaults();
                      props.setHasUnsavedChanges(true);
                    } else {
                      updateSnapshotDefaults(snapshotFactoryConfig());
                    }
                  }}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('storage')}>
            <CollapsibleSection
              id="storage"
              title={sectionTitles.storage}
              resourceCount={props.storage.length}
              collapsed={isCollapsed('storage')}
              onToggle={() => toggleSection('storage')}
              icon={<HardDrive class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllStorage()}
              emptyMessage={STORAGE_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('storage')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={storageGroupedByNode()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={['Usage %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={STORAGE_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  showOfflineAlertsColumn={false}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={{ usage: props.storageDefault() }}
                  setGlobalDefaults={(value) => {
                    if (typeof value === 'function') {
                      const newValue = value({ usage: props.storageDefault() });
                      props.setStorageDefault(newValue.usage ?? 85);
                    } else {
                      props.setStorageDefault(value.usage ?? 85);
                    }
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllStorage}
                  onToggleGlobalDisable={() =>
                    props.setDisableAllStorage(!props.disableAllStorage())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().storage}
                  metricDelaySeconds={props.metricTimeThresholds().storage ?? {}}
                  onMetricDelayChange={(metric, value) =>
                    updateMetricDelay('storage', metric, value)
                  }
                  factoryDefaults={
                    props.factoryStorageDefault !== undefined
                      ? { usage: props.factoryStorageDefault }
                      : undefined
                  }
                  onResetDefaults={props.resetStorageDefault}
                />
              </div>
            </CollapsibleSection>
          </Show>
        </Show>

        <Show when={activeTab() === 'pmg'}>
          <Show
            when={pmgServersWithOverrides().length > 0}
            fallback={
              <div class="rounded-md border border-border bg-surface p-6 text-sm text-muted">
                {PMG_THRESHOLDS_EMPTY_STATE}
              </div>
            }
          >
            <div ref={registerSection('pmg')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.pmg}
                resources={pmgServersWithOverrides()}
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
                activeAlerts={props.activeAlerts}
                emptyMessage={PMG_THRESHOLDS_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %'])}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={pmgGlobalDefaults()}
                setGlobalDefaults={setPMGGlobalDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllPMG}
                onToggleGlobalDisable={() => props.setDisableAllPMG(!props.disableAllPMG())}
                globalDisableOfflineFlag={props.disableAllPMGOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllPMGOffline(!props.disableAllPMGOffline())
                }
              />
            </div>
          </Show>
        </Show>

        <Show when={activeTab() === 'agents'}>
          <Show when={hasSection('agents')}>
            <div ref={registerSection('agents')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.agents}
                resources={agentsWithOverrides()}
                columns={['CPU %', 'Memory %', 'Disk %', 'Disk Temp °C']}
                activeAlerts={props.activeAlerts}
                emptyMessage={AGENT_THRESHOLDS_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) =>
                  handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
                }
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={props.agentDefaults}
                setGlobalDefaults={props.setAgentDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllAgents}
                onToggleGlobalDisable={() => props.setDisableAllAgents(!props.disableAllAgents())}
                globalDisableOfflineFlag={props.disableAllAgentsOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllAgentsOffline(!props.disableAllAgentsOffline())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().agent}
                metricDelaySeconds={props.metricTimeThresholds().agent ?? {}}
                onMetricDelayChange={(metric, value) => updateMetricDelay('agent', metric, value)}
                factoryDefaults={props.factoryAgentDefaults}
                onResetDefaults={props.resetAgentDefaults}
              />
            </div>
          </Show>

          <Show when={hasSection('agentDisks')}>
            <CollapsibleSection
              id="agentDisks"
              title={sectionTitles.agentDisks}
              resourceCount={agentDisksWithOverrides().length}
              collapsed={isCollapsed('agentDisks')}
              onToggle={() => toggleSection('agentDisks')}
              icon={<HardDrive class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllAgents()}
              emptyMessage={AGENT_DISKS_EMPTY_STATE}
            >
              <div ref={registerSection('agentDisks')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={agentDisksGroupedByAgent()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={['Disk %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={AGENT_DISKS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  showOfflineAlertsColumn={false}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, [
                      'CPU %',
                      'Memory %',
                      'Disk R MB/s',
                      'Disk W MB/s',
                      'Net In MB/s',
                      'Net Out MB/s',
                    ])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={{ disk: props.agentDefaults.disk }}
                  setGlobalDefaults={(value) => {
                    if (typeof value === 'function') {
                      const newValue = value({ disk: props.agentDefaults.disk });
                      props.setAgentDefaults((prev) => ({ ...prev, disk: newValue.disk }));
                    } else {
                      props.setAgentDefaults((prev) => ({ ...prev, disk: value.disk }));
                    }
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                />
              </div>
            </CollapsibleSection>
          </Show>
        </Show>

        <Show when={activeTab() === 'docker'}>
          <Card padding="md" tone="card" class="mb-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">
                  {dockerIgnoredPrefixesPresentation.title}
                </h3>
                <p class="mt-1 text-xs text-muted">
                  {dockerIgnoredPrefixesPresentation.description}
                </p>
              </div>
              <Show when={(props.dockerIgnoredPrefixes().length ?? 0) > 0}>
                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-md border border-transparent px-3 py-1 text-xs font-medium transition hover:bg-surface-alt"
                  onClick={handleResetDockerIgnored}
                >
                  {dockerIgnoredPrefixesPresentation.resetLabel}
                </button>
              </Show>
            </div>
            <textarea
              value={dockerIgnoredInput()}
              onInput={(event) => handleDockerIgnoredChange(event.currentTarget.value)}
              onKeyDown={(event) => {
                // Ensure Enter key works in textarea for creating new lines
                if (event.key === 'Enter') {
                  // Don't prevent default - allow the newline to be inserted
                  event.stopPropagation();
                }
              }}
              placeholder={dockerIgnoredPrefixesPresentation.placeholder}
              rows={4}
              class="mt-4 w-full rounded-md border border-border bg-surface p-3 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
            />
          </Card>

          <Card padding="md" tone="card" class="mb-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">
                  {dockerServicePresentation.title}
                </h3>
                <p class="mt-1 text-xs text-muted">{dockerServicePresentation.description}</p>
              </div>
              <Toggle
                checked={!props.disableAllDockerServices()}
                onToggle={() => {
                  props.setDisableAllDockerServices(!props.disableAllDockerServices());
                  props.setHasUnsavedChanges(true);
                }}
                label={
                  <span class="text-sm font-medium text-base-content">
                    {dockerServicePresentation.toggleLabel}
                  </span>
                }
                description={
                  <span class="text-xs text-muted">
                    {dockerServicePresentation.toggleDescription}
                  </span>
                }
                size="sm"
              />
            </div>

            <div class="mt-4 grid gap-4 sm:grid-cols-2">
              <div>
                <label
                  for={serviceWarnInputId}
                  class="text-xs font-medium uppercase tracking-wide text-muted"
                >
                  {dockerServicePresentation.warningGapLabel}
                </label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  id={serviceWarnInputId}
                  value={props.dockerDefaults.serviceWarnGapPercent}
                  onInput={(event) => {
                    const value = Number(event.currentTarget.value);
                    const normalized = Number.isFinite(value)
                      ? Math.max(0, Math.min(100, value))
                      : 0;
                    props.setDockerDefaults((prev) => ({
                      ...prev,
                      serviceWarnGapPercent: normalized,
                    }));
                    props.setHasUnsavedChanges(true);
                  }}
                  class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                />
                <p class="mt-1 text-xs text-muted">
                  {dockerServicePresentation.warningGapDescription}
                </p>
              </div>
              <div>
                <label
                  for={serviceCriticalInputId}
                  class="text-xs font-medium uppercase tracking-wide text-muted"
                >
                  {dockerServicePresentation.criticalGapLabel}
                </label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  id={serviceCriticalInputId}
                  value={props.dockerDefaults.serviceCriticalGapPercent}
                  onInput={(event) => {
                    const value = Number(event.currentTarget.value);
                    const normalized = Number.isFinite(value)
                      ? Math.max(0, Math.min(100, value))
                      : 0;
                    props.setDockerDefaults((prev) => ({
                      ...prev,
                      serviceCriticalGapPercent: normalized,
                    }));
                    props.setHasUnsavedChanges(true);
                  }}
                  class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                />
                <p class="mt-1 text-xs text-muted">
                  {dockerServicePresentation.criticalGapDescription}
                </p>
              </div>
            </div>
            {serviceGapValidationMessage() && (
              <p class="mt-1.5 text-xs font-medium text-red-600 dark:text-red-400">
                {serviceGapValidationMessage()}
              </p>
            )}
          </Card>

          <Show when={hasSection('dockerHosts')}>
            <div ref={registerSection('dockerHosts')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.dockerHosts}
                resources={dockerHostsWithOverrides()}
                columns={[]}
                activeAlerts={props.activeAlerts}
                emptyMessage={CONTAINER_RUNTIMES_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) =>
                  handleBulkEdit(ids, [
                    'CPU %',
                    'Memory %',
                    'Disk %',
                    'Disk R MB/s',
                    'Disk W MB/s',
                    'Net In MB/s',
                    'Net Out MB/s',
                    'Restart Count',
                    'Restart Window (s)',
                  ])
                }
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDisableFlag={props.disableAllDockerHosts}
                onToggleGlobalDisable={() =>
                  props.setDisableAllDockerHosts(!props.disableAllDockerHosts())
                }
                globalDisableOfflineFlag={props.disableAllDockerHostsOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllDockerHostsOffline(!props.disableAllDockerHostsOffline())
                }
              />
            </div>
          </Show>

          <Show when={hasSection('dockerContainers')}>
            <div ref={registerSection('dockerContainers')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.dockerContainers}
                groupedResources={dockerContainersGroupedByHost()}
                groupHeaderMeta={dockerHostGroupMeta()}
                columns={[
                  'CPU %',
                  'Memory %',
                  'Disk %',
                  'Restart Count',
                  'Restart Window (s)',
                  'Memory Warn %',
                  'Memory Critical %',
                ]}
                activeAlerts={props.activeAlerts}
                emptyMessage={CONTAINERS_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                showOfflineAlertsColumn={false}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) =>
                  handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
                }
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={{
                  cpu: props.dockerDefaults.cpu,
                  memory: props.dockerDefaults.memory,
                  disk: props.dockerDefaults.disk,
                  restartCount: props.dockerDefaults.restartCount,
                  restartWindow: props.dockerDefaults.restartWindow,
                  memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                  memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                }}
                setGlobalDefaults={(value) => {
                  const current = {
                    cpu: props.dockerDefaults.cpu,
                    memory: props.dockerDefaults.memory,
                    disk: props.dockerDefaults.disk,
                    restartCount: props.dockerDefaults.restartCount,
                    restartWindow: props.dockerDefaults.restartWindow,
                    memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                    memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                  };
                  const next =
                    typeof value === 'function' ? value(current) : { ...current, ...value };

                  props.setDockerDefaults((prev) => ({
                    ...prev,
                    cpu: next.cpu ?? prev.cpu,
                    memory: next.memory ?? prev.memory,
                    disk: next.disk ?? prev.disk,
                    restartCount: next.restartCount ?? prev.restartCount,
                    restartWindow: next.restartWindow ?? prev.restartWindow,
                    memoryWarnPct: next.memoryWarnPct ?? prev.memoryWarnPct,
                    memoryCriticalPct: next.memoryCriticalPct ?? prev.memoryCriticalPct,
                  }));
                }}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllDockerContainers}
                onToggleGlobalDisable={() =>
                  props.setDisableAllDockerContainers(!props.disableAllDockerContainers())
                }
                globalDisableOfflineFlag={() => props.dockerDisableConnectivity()}
                onToggleGlobalDisableOffline={() =>
                  props.setDockerDisableConnectivity(!props.dockerDisableConnectivity())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().guest}
                metricDelaySeconds={props.metricTimeThresholds().guest ?? {}}
                onMetricDelayChange={(metric, value) => updateMetricDelay('guest', metric, value)}
                globalOfflineSeverity={props.dockerPoweredOffSeverity()}
                onSetGlobalOfflineState={(state) => {
                  if (state === 'off') {
                    props.setDockerDisableConnectivity(true);
                  } else {
                    props.setDockerDisableConnectivity(false);
                    props.setDockerPoweredOffSeverity(
                      state === 'critical' ? 'critical' : 'warning',
                    );
                  }
                  props.setHasUnsavedChanges(true);
                }}
                onSetOfflineState={setOfflineState}
                factoryDefaults={props.factoryDockerDefaults}
                onResetDefaults={props.resetDockerDefaults}
              />
            </div>
          </Show>
        </Show>
      </div>

      <BulkEditDialog
        isOpen={isBulkEditDialogOpen()}
        onClose={() => setIsBulkEditDialogOpen(false)}
        selectedIds={bulkEditIds()}
        columns={bulkEditColumns()}
        onSave={handleSaveBulkEdit}
      />
    </div>
  );
}
