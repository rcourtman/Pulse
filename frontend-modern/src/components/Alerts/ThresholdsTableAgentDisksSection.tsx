import { Show } from 'solid-js';
import HardDrive from 'lucide-solid/icons/hard-drive';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableAgentDisksSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
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
            groupHeaderMeta={state.agentGroupHeaderMeta()}
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
  );
}
