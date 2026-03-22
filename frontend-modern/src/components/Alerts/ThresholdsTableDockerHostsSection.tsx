import { Show } from 'solid-js';

import { ResourceTable } from './ResourceTable';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerHostsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
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
  );
}
