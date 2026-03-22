import { Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import Toggle from '@/components/shared/Toggle';
import { ResourceTable } from './ResourceTable';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { ThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';

interface ThresholdsTableTabProps {
  state: ThresholdsTableState;
  tableProps: ThresholdsTableProps;
}

export function ThresholdsTableDockerTab(props: ThresholdsTableTabProps) {
  const { state, tableProps } = props;

  return (
    <>
      <Card padding="md" tone="card" class="mb-6">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-base-content">
              {state.dockerIgnoredPrefixesPresentation.title}
            </h3>
            <p class="mt-1 text-xs text-muted">
              {state.dockerIgnoredPrefixesPresentation.description}
            </p>
          </div>
          <Show when={(tableProps.dockerIgnoredPrefixes().length ?? 0) > 0}>
            <button
              type="button"
              class="inline-flex items-center justify-center rounded-md border border-transparent px-3 py-1 text-xs font-medium transition hover:bg-surface-alt"
              onClick={state.handleResetDockerIgnored}
            >
              {state.dockerIgnoredPrefixesPresentation.resetLabel}
            </button>
          </Show>
        </div>
        <textarea
          value={state.dockerIgnoredInput()}
          onInput={(event) => state.handleDockerIgnoredChange(event.currentTarget.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.stopPropagation();
            }
          }}
          placeholder={state.dockerIgnoredPrefixesPresentation.placeholder}
          rows={4}
          class="mt-4 w-full rounded-md border border-border bg-surface p-3 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
        />
      </Card>

      <Card padding="md" tone="card" class="mb-6">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-base-content">
              {state.dockerServicePresentation.title}
            </h3>
            <p class="mt-1 text-xs text-muted">{state.dockerServicePresentation.description}</p>
          </div>
          <Toggle
            checked={!tableProps.disableAllDockerServices()}
            onToggle={() => {
              tableProps.setDisableAllDockerServices(!tableProps.disableAllDockerServices());
              tableProps.setHasUnsavedChanges(true);
            }}
            label={
              <span class="text-sm font-medium text-base-content">
                {state.dockerServicePresentation.toggleLabel}
              </span>
            }
            description={
              <span class="text-xs text-muted">
                {state.dockerServicePresentation.toggleDescription}
              </span>
            }
            size="sm"
          />
        </div>

        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <label
              for={state.serviceWarnInputId}
              class="text-xs font-medium uppercase tracking-wide text-muted"
            >
              {state.dockerServicePresentation.warningGapLabel}
            </label>
            <input
              type="number"
              min="0"
              max="100"
              id={state.serviceWarnInputId}
              value={tableProps.dockerDefaults.serviceWarnGapPercent}
              onInput={(event) => {
                const value = Number(event.currentTarget.value);
                const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
                tableProps.setDockerDefaults((prev) => ({
                  ...prev,
                  serviceWarnGapPercent: normalized,
                }));
                tableProps.setHasUnsavedChanges(true);
              }}
              class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
            />
            <p class="mt-1 text-xs text-muted">
              {state.dockerServicePresentation.warningGapDescription}
            </p>
          </div>
          <div>
            <label
              for={state.serviceCriticalInputId}
              class="text-xs font-medium uppercase tracking-wide text-muted"
            >
              {state.dockerServicePresentation.criticalGapLabel}
            </label>
            <input
              type="number"
              min="0"
              max="100"
              id={state.serviceCriticalInputId}
              value={tableProps.dockerDefaults.serviceCriticalGapPercent}
              onInput={(event) => {
                const value = Number(event.currentTarget.value);
                const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
                tableProps.setDockerDefaults((prev) => ({
                  ...prev,
                  serviceCriticalGapPercent: normalized,
                }));
                tableProps.setHasUnsavedChanges(true);
              }}
              class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
            />
            <p class="mt-1 text-xs text-muted">
              {state.dockerServicePresentation.criticalGapDescription}
            </p>
          </div>
        </div>
        {state.serviceGapValidationMessage() && (
          <p class="mt-1.5 text-xs font-medium text-red-600 dark:text-red-400">
            {state.serviceGapValidationMessage()}
          </p>
        )}
      </Card>

      <Show when={state.hasSection('dockerHosts')}>
        <div ref={state.registerSection('dockerHosts')} class="scroll-mt-24">
          <ResourceTable
            title={state.sectionTitles.dockerHosts}
            resources={state.dockerHostsWithOverrides()}
            columns={[]}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.CONTAINER_RUNTIMES_FILTER_EMPTY_STATE}
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
            hasActiveAlert={state.hasActiveAlert}
            globalDisableFlag={tableProps.disableAllDockerHosts}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllDockerHosts(!tableProps.disableAllDockerHosts())
            }
            globalDisableOfflineFlag={tableProps.disableAllDockerHostsOffline}
            onToggleGlobalDisableOffline={() =>
              tableProps.setDisableAllDockerHostsOffline(!tableProps.disableAllDockerHostsOffline())
            }
          />
        </div>
      </Show>

      <Show when={state.hasSection('dockerContainers')}>
        <div ref={state.registerSection('dockerContainers')} class="scroll-mt-24">
          <ResourceTable
            title={state.sectionTitles.dockerContainers}
            groupedResources={state.dockerContainersGroupedByHost()}
            groupHeaderMeta={state.dockerHostGroupMeta()}
            columns={[
              'CPU %',
              'Memory %',
              'Disk %',
              'Restart Count',
              'Restart Window (s)',
              'Memory Warn %',
              'Memory Critical %',
            ]}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.CONTAINERS_FILTER_EMPTY_STATE}
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
            onBulkEdit={(ids) => state.handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={{
              cpu: tableProps.dockerDefaults.cpu,
              memory: tableProps.dockerDefaults.memory,
              disk: tableProps.dockerDefaults.disk,
              restartCount: tableProps.dockerDefaults.restartCount,
              restartWindow: tableProps.dockerDefaults.restartWindow,
              memoryWarnPct: tableProps.dockerDefaults.memoryWarnPct,
              memoryCriticalPct: tableProps.dockerDefaults.memoryCriticalPct,
            }}
            setGlobalDefaults={(value) => {
              const current = {
                cpu: tableProps.dockerDefaults.cpu,
                memory: tableProps.dockerDefaults.memory,
                disk: tableProps.dockerDefaults.disk,
                restartCount: tableProps.dockerDefaults.restartCount,
                restartWindow: tableProps.dockerDefaults.restartWindow,
                memoryWarnPct: tableProps.dockerDefaults.memoryWarnPct,
                memoryCriticalPct: tableProps.dockerDefaults.memoryCriticalPct,
              };
              const next = typeof value === 'function' ? value(current) : { ...current, ...value };

              tableProps.setDockerDefaults((prev) => ({
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
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={tableProps.disableAllDockerContainers}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllDockerContainers(!tableProps.disableAllDockerContainers())
            }
            globalDisableOfflineFlag={() => tableProps.dockerDisableConnectivity()}
            onToggleGlobalDisableOffline={() =>
              tableProps.setDockerDisableConnectivity(!tableProps.dockerDisableConnectivity())
            }
            showDelayColumn={true}
            globalDelaySeconds={tableProps.timeThresholds().guest}
            metricDelaySeconds={tableProps.metricTimeThresholds().guest ?? {}}
            onMetricDelayChange={(metric, value) => state.updateMetricDelay('guest', metric, value)}
            globalOfflineSeverity={tableProps.dockerPoweredOffSeverity()}
            onSetGlobalOfflineState={(nextState) => {
              if (nextState === 'off') {
                tableProps.setDockerDisableConnectivity(true);
              } else {
                tableProps.setDockerDisableConnectivity(false);
                tableProps.setDockerPoweredOffSeverity(
                  nextState === 'critical' ? 'critical' : 'warning',
                );
              }
              tableProps.setHasUnsavedChanges(true);
            }}
            onSetOfflineState={state.setOfflineState}
            factoryDefaults={tableProps.factoryDockerDefaults}
            onResetDefaults={tableProps.resetDockerDefaults}
          />
        </div>
      </Show>
    </>
  );
}
