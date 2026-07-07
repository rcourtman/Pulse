import { For, Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { ResourceTable } from './ResourceTable';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

const DISK_TEMP_TYPE_FIELDS: readonly { key: string; label: string }[] = [
  { key: 'nvme', label: 'NVMe' },
  { key: 'sas', label: 'SAS' },
  { key: 'sata', label: 'SATA' },
];

export function ThresholdsTableAgentsResourcesSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('agents')}>
      <div ref={state.registerSection('agents')} class="scroll-mt-24">
        <ResourceTable
          title={state.sectionTitles.agents}
          resources={state.agentsWithOverrides()}
          columns={['CPU %', 'Memory %', 'Disk %', 'Disk Temp °C']}
          activeAlerts={tableProps.activeAlerts}
          emptyMessage={state.AGENT_THRESHOLDS_FILTER_EMPTY_STATE}
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
          globalDefaults={tableProps.agentDefaults}
          setGlobalDefaults={tableProps.setAgentDefaults}
          setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
          globalDisableFlag={tableProps.disableAllAgents}
          onToggleGlobalDisable={() =>
            tableProps.setDisableAllAgents(!tableProps.disableAllAgents())
          }
          globalDisableOfflineFlag={tableProps.disableAllAgentsOffline}
          onToggleGlobalDisableOffline={() =>
            tableProps.setDisableAllAgentsOffline(!tableProps.disableAllAgentsOffline())
          }
          showDelayColumn={true}
          globalDelaySeconds={tableProps.timeThresholds().agent}
          metricDelaySeconds={tableProps.metricTimeThresholds().agent ?? {}}
          onMetricDelayChange={(metric, value) => state.updateMetricDelay('agent', metric, value)}
          factoryDefaults={tableProps.factoryAgentDefaults}
          onResetDefaults={tableProps.resetAgentDefaults}
        />

        <Card padding="md" tone="card" class="mt-4">
          <h3 class="text-sm font-semibold text-base-content">Disk temperature by type</h3>
          <p class="mt-1 text-xs text-muted">
            Alert trigger in °C for each disk type. Warning colors start 5°C below the trigger.
            Setting a Disk Temp override on a host above replaces these for all of that host's
            disks.
          </p>
          <div class="mt-4 grid gap-4 sm:grid-cols-3">
            <For each={DISK_TEMP_TYPE_FIELDS}>
              {(field) => (
                <div>
                  <label
                    for={`disk-temp-by-type-${field.key}`}
                    class="text-xs font-medium uppercase tracking-wide text-muted"
                  >
                    {field.label} °C
                  </label>
                  <input
                    type="number"
                    min="1"
                    max="100"
                    id={`disk-temp-by-type-${field.key}`}
                    value={tableProps.diskTempByType[field.key] ?? ''}
                    onInput={(event) => {
                      const value = Number(event.currentTarget.value);
                      if (!Number.isFinite(value) || value <= 0) return;
                      const normalized = Math.max(1, Math.min(100, Math.round(value)));
                      tableProps.setDiskTempByType((prev) => ({
                        ...prev,
                        [field.key]: normalized,
                      }));
                      tableProps.setHasUnsavedChanges(true);
                    }}
                    class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                  />
                </div>
              )}
            </For>
          </div>
        </Card>
      </div>
    </Show>
  );
}
