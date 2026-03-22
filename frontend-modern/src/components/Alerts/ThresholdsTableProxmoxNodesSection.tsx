import { Show } from 'solid-js';
import Server from 'lucide-solid/icons/server';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxNodesSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
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
  );
}
