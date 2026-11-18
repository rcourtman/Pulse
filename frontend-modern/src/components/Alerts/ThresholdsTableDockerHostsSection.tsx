import { For, Show, createMemo } from 'solid-js';
import Server from 'lucide-solid/icons/server';

import { buildThresholdsDockerHostLabels } from '@/features/alerts/thresholds/thresholdsResourceModel';
import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerHostsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  const runtimes = createMemo(() => {
    const hosts = tableProps.containerRuntimes ?? [];
    const labelFor = buildThresholdsDockerHostLabels(hosts);
    const labels = new Map<string, string>();
    for (const host of hosts) {
      const id = host.docker?.hostSourceId;
      if (id) labels.set(id, labelFor(host).groupKey);
    }
    for (const id of tableProps.dockerMutedContainerHosts?.() ?? []) {
      if (!labels.has(id)) labels.set(id, id);
    }
    return [...labels].sort((left, right) => left[1].localeCompare(right[1]));
  });
  const setMuted = (id: string, mute: boolean) => {
    const previous = tableProps.dockerMutedContainerHosts?.() ?? [];
    const next = new Set(previous);
    if (mute) next.add(id);
    else next.delete(id);
    tableProps.setDockerMutedContainerHosts?.([...next]);
    tableProps.setHasUnsavedChanges(true);
  };

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
            Offline Alerts controls runtime disconnection. You can also mute container alerts on an
            individual runtime. Machine alerts, Swarm services and endpoint checks keep their own
            settings.
          </p>
          <Show when={runtimes().length && tableProps.setDockerMutedContainerHosts}>
            <fieldset class="mb-4 space-y-1">
              <legend class="mb-1 text-sm font-semibold">Mute current and future containers</legend>
              <For each={runtimes()}>
                {([id, label]) => (
                  <label class="flex min-h-9 items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={tableProps.dockerMutedContainerHosts?.().includes(id) ?? false}
                      onChange={(event) => setMuted(id, event.currentTarget.checked)}
                    />
                    <span>{label}</span>
                    <Show when={label !== id}>
                      <span class="break-all text-xs text-muted">{id}</span>
                    </Show>
                  </label>
                )}
              </For>
            </fieldset>
          </Show>
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
