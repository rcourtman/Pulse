import { Show } from 'solid-js';

import { ResourceTable } from './ResourceTable';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

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
      </div>
    </Show>
  );
}
