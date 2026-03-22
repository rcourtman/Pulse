import { Show } from 'solid-js';
import HardDrive from 'lucide-solid/icons/hard-drive';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { ThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';

interface ThresholdsTableTabProps {
  state: ThresholdsTableState;
  tableProps: ThresholdsTableProps;
}

export function ThresholdsTableAgentsTab(props: ThresholdsTableTabProps) {
  const { state, tableProps } = props;

  return (
    <>
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

      <Show when={state.hasSection('agentDisks')}>
        <CollapsibleSection
          id="agentDisks"
          title={state.sectionTitles.agentDisks}
          resourceCount={state.agentDisksWithOverrides().length}
          collapsed={state.isCollapsed('agentDisks')}
          onToggle={() => state.toggleSection('agentDisks')}
          icon={<HardDrive class="w-5 h-5" />}
          isGloballyDisabled={tableProps.disableAllAgents()}
          emptyMessage={state.AGENT_DISKS_EMPTY_STATE}
        >
          <div ref={state.registerSection('agentDisks')} class="scroll-mt-24">
            <ResourceTable
              title=""
              groupedResources={state.agentDisksGroupedByAgent()}
              groupHeaderMeta={state.guestGroupHeaderMeta()}
              columns={['Disk %']}
              activeAlerts={tableProps.activeAlerts}
              emptyMessage={state.AGENT_DISKS_FILTER_EMPTY_STATE}
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
              globalDefaults={{ disk: tableProps.agentDefaults.disk }}
              setGlobalDefaults={(value) => {
                if (typeof value === 'function') {
                  const nextValue = value({ disk: tableProps.agentDefaults.disk });
                  tableProps.setAgentDefaults((prev) => ({ ...prev, disk: nextValue.disk }));
                } else {
                  tableProps.setAgentDefaults((prev) => ({ ...prev, disk: value.disk }));
                }
              }}
              setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            />
          </div>
        </CollapsibleSection>
      </Show>
    </>
  );
}
