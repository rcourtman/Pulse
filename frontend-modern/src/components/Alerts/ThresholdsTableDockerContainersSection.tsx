import { Show } from 'solid-js';

import { ResourceTable } from './ResourceTable';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerContainersSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
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
  );
}
