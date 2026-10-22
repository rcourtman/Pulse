import { For, Show, Suspense, createMemo, createSignal, lazy, type Component } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import { getPhysicalDiskAlerts } from '@/components/Storage/physicalDiskAlertOwner';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { useWebSocket } from '@/contexts/appRuntime';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  extractPhysicalDiskPresentationData,
  getPhysicalDiskHealthStatus,
} from '@/features/storageBackups/diskPresentation';
import {
  PHYSICAL_DISK_TEMPERATURE_LAST_KNOWN_CLASS,
  getPhysicalDiskTemperaturePresentation,
} from '@/features/storageBackups/diskTemperaturePresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource } from '@/types/resource';
import { formatBytes } from '@/utils/format';

import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from './guestDrawerModel';

// An opened disk shows the Storage disk drawer's Overview, so a disk reads the
// same here as on the Storage tab. Load it only once a disk row is opened, to
// keep it out of the initial Workloads surface.
const LazyDiskDrawerOverview = lazy(() =>
  import('@/components/Storage/DiskDrawerOverview').then(({ DiskDrawerOverview }) => ({
    default: DiskDrawerOverview,
  })),
);

// The disks' reporting machine is the open guest, not a loaded resource here:
// alerts come from each disk's own identity, its Proxmox disk key and, when
// the guest runs an agent, that agent's SMART temperature key.
const NO_LOADED_DISK_OWNERS: ReadonlyMap<string, Resource> = new Map();

const GuestPhysicalDiskRow: Component<{
  disk: Resource;
  alertResourceIds: string[];
  reportingAgentId?: string;
  historyEnabled: boolean;
}> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const { activeAlerts } = useWebSocket();
  const { detectionEnabled, getDiskTemperatureThresholds } = useAlertsActivation();
  const data = createMemo(() =>
    extractPhysicalDiskPresentationData(
      props.disk,
      getDiskTemperatureThresholds,
      props.alertResourceIds,
    ),
  );
  const alerts = () =>
    getPhysicalDiskAlerts(
      props.disk,
      NO_LOADED_DISK_OWNERS,
      activeAlerts,
      detectionEnabled(),
      props.reportingAgentId,
    );
  const health = () => getPhysicalDiskHealthStatus(data());
  const temperature = () => getPhysicalDiskTemperaturePresentation(data());
  const label = () => data().model || data().devPath || props.disk.name;

  return (
    <details
      class="rounded-sm border border-border p-2"
      onToggle={(event) => setExpanded(event.currentTarget.open)}
      data-testid="guest-physical-disk"
    >
      <summary class="flex cursor-pointer flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        <span class="font-semibold text-base-content">{label()}</span>
        <Show when={data().devPath}>
          <span class="font-mono text-muted">{data().devPath}</span>
        </Show>
        <span class={health().tone} title={health().summary}>
          {health().label}
        </span>
        <Show when={temperature()}>
          {(reading) => (
            <span
              class={reading().current ? 'text-muted' : PHYSICAL_DISK_TEMPERATURE_LAST_KNOWN_CLASS}
              title={reading().title}
              data-temperature-reading={reading().current ? 'current' : 'last-known'}
            >
              {reading().label}
              <Show when={!reading().current}>
                <span class="sr-only">, last known</span>
              </Show>
            </span>
          )}
        </Show>
        <Show when={data().size > 0}>
          <span class="text-muted">{formatBytes(data().size)}</span>
        </Show>
      </summary>
      <Show when={expanded()}>
        <div class="mt-2 border-t border-border pt-2">
          <Suspense
            fallback={
              <p class="text-xs text-muted" role="status">
                Loading SMART details…
              </p>
            }
          >
            <LazyDiskDrawerOverview
              disk={props.disk}
              diskData={data()}
              alerts={alerts()}
              historyRange={historyRange()}
              onHistoryRangeChange={setHistoryRange}
              historyEnabled={props.historyEnabled}
            />
          </Suspense>
        </div>
      </Show>
    </details>
  );
};

export const GuestPhysicalDisks: Component<{
  parentId: string;
  /**
   * Alert override keys judging these disks' heat, in the order the backend
   * reads them: the guest's own agent reports them, so its Disk Temp override
   * applies, then the guest's canonical ID and override keys.
   */
  alertResourceIds?: string[];
  /** The guest's own agent, which reports these disks' SMART temperatures. */
  reportingAgentId?: string;
  /**
   * False while the Overview holding these disks is hidden: an expanded disk's
   * charts and the list's live refresh pause until it is shown again.
   */
  historyEnabled?: boolean;
}> = (props) => {
  // The Proxmox Overview snapshot deliberately excludes physical disks. Query
  // only the open guest's direct children rather than hydrating every disk in
  // a large estate (or mistaking the VMID-based table key for the resource ID).
  const parentId = props.parentId.trim();
  const source = useUnifiedResources({
    query: `type=physical_disk&parent=${encodeURIComponent(parentId)}`,
    cacheKey: `guest-physical-disks:${parentId}`,
    enabled: () => parentId.length > 0,
    realtimeEnabled: () => parentId.length > 0 && props.historyEnabled !== false,
  });
  const disks = () =>
    source
      .resources()
      .filter((resource) => resource.type === 'physical_disk' && resource.parentId === parentId);

  return (
    <>
      <Show when={source.error()}>
        <p class="text-xs text-muted" role="status">
          Physical disk details are unavailable.
        </p>
      </Show>
      <Show when={disks().length > 0}>
        <InfoCardFrame data-testid="guest-physical-disks">
          <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
            Physical Disks &amp; SMART ({disks().length})
          </h3>
          <div class="space-y-2">
            <For each={disks()}>
              {(disk) => (
                <GuestPhysicalDiskRow
                  disk={disk}
                  alertResourceIds={props.alertResourceIds ?? []}
                  reportingAgentId={props.reportingAgentId}
                  historyEnabled={props.historyEnabled !== false}
                />
              )}
            </For>
          </div>
        </InfoCardFrame>
      </Show>
    </>
  );
};
