import { Show } from 'solid-js';
import Thermometer from 'lucide-solid/icons/thermometer';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

const AGENT_SENSOR_THRESHOLD_COLUMNS = ['Temp °C'];
const AGENT_SENSOR_COLUMN_TOOLTIPS = {
  'Temp °C': 'Sensor temperature that raises an alert. Off until you set a limit.',
};

// Native lm-sensors temperatures, one row per sensor under its machine. A
// sensor alerts only on its own limit: there is no global default, so the
// section has no defaults row and every unconfigured sensor reads Off. The
// limit cell is the only switch, so there is no separate Alerts toggle.
export function ThresholdsTableAgentSensorsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('agentSensors')}>
      <CollapsibleSection
        id="agentSensors"
        title={state.sectionTitles.agentSensors}
        defaultsSummary="Off until a sensor has its own limit"
        resourceCount={state.agentSensorsWithOverrides().length}
        collapsed={state.isCollapsed('agentSensors')}
        onToggle={() => state.toggleSection('agentSensors')}
        icon={<Thermometer class="h-5 w-5" />}
        isGloballyDisabled={tableProps.disableAllAgents()}
        emptyMessage={state.AGENT_SENSORS_EMPTY_STATE}
      >
        <div ref={state.registerSection('agentSensors')} class="scroll-mt-24">
          <ResourceTable
            title=""
            onConfigureResourceIntent={tableProps.onConfigureResourceIntent}
            groupedResources={state.agentSensorsGroupedByAgent()}
            groupHeaderMeta={state.agentGroupHeaderMeta()}
            columns={AGENT_SENSOR_THRESHOLD_COLUMNS}
            columnTooltips={AGENT_SENSOR_COLUMN_TOOLTIPS}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.AGENT_SENSORS_FILTER_EMPTY_STATE}
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            showAlertsColumn={false}
            showOfflineAlertsColumn={false}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) => state.handleBulkEdit(ids, AGENT_SENSOR_THRESHOLD_COLUMNS)}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
