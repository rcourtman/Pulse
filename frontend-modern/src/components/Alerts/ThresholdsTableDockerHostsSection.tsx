import { Show } from 'solid-js';
import Server from 'lucide-solid/icons/server';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerHostsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('dockerHosts')}>
      <CollapsibleSection
        id="dockerHosts"
        title={state.sectionTitles.dockerHosts}
        resourceCount={state.dockerHostsWithOverrides().length}
        collapsed={state.isCollapsed('dockerHosts')}
        onToggle={() => state.toggleSection('dockerHosts')}
        icon={<Server class="h-5 w-5" />}
        isGloballyDisabled={tableProps.disableAllDockerHosts()}
        emptyMessage={state.CONTAINER_RUNTIMES_FILTER_EMPTY_STATE}
      >
        <div ref={state.registerSection('dockerHosts')} class="scroll-mt-24">
          <p class="mb-3 text-sm text-muted">
            Offline Alerts only controls runtime-disconnection alerts, not container or machine
            alerts. To mute existing containers, turn off their Alerts switches in Containers and
            save. Newly added containers need their own setting. There is no single-runtime mute for
            all its containers.
          </p>
          <ResourceTable
            title=""
            onConfigureResourceIntent={tableProps.onConfigureResourceIntent}
            resources={state.dockerHostsWithOverrides()}
            columns={[]}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.CONTAINER_RUNTIMES_FILTER_EMPTY_STATE}
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            showAlertsColumn={false}
            onToggleNodeConnectivity={state.toggleNodeConnectivity}
            showOfflineAlertsColumn={true}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDisableFlag={tableProps.disableAllDockerHosts}
            globalDisableOfflineFlag={tableProps.disableAllDockerHostsOffline}
            onToggleGlobalDisableOffline={() =>
              tableProps.setDisableAllDockerHostsOffline(!tableProps.disableAllDockerHostsOffline())
            }
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
