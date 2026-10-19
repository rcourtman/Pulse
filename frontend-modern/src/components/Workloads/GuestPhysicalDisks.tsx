import { For, Show, Suspense, createSignal, lazy, onCleanup, type Component } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
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
import { eventBus } from '@/stores/events';

// SMART history and chart controls are only needed after a disk row is opened.
// Keep their Storage detail module out of the initial Workloads surface.
const LazyDiskDetail = lazy(() =>
  import('@/components/Storage/DiskDetail').then(({ DiskDetail }) => ({ default: DiskDetail })),
);

const GuestPhysicalDiskRow: Component<{
  disk: Resource;
  alertResourceIds: string[];
  historyEnabled: boolean;
}> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  const data = () =>
    extractPhysicalDiskPresentationData(
      props.disk,
      getDiskTemperatureThresholds,
      props.alertResourceIds,
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
            <LazyDiskDetail
              disk={props.disk}
              nodes={[]}
              alertResourceIds={props.alertResourceIds}
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
    reportBackgroundErrors: true,
    readTimeoutMs: 20_000,
    enabled: () => parentId.length > 0,
    realtimeEnabled: () => parentId.length > 0 && props.historyEnabled !== false,
  });
  const [retrying, setRetrying] = createSignal(false);
  let retryGeneration = 0;
  // The same guest id may exist in another organisation. Its read must not
  // inherit an unanswered retry's busy state, or its eventual settlement.
  const resetRetry = () => {
    retryGeneration += 1;
    setRetrying(false);
  };
  const unsubscribeOrgSwitch = eventBus.on('org_switched', resetRetry);
  onCleanup(() => {
    unsubscribeOrgSwitch();
    retryGeneration += 1;
  });
  const retry = async () => {
    if (retrying() || source.loading()) return;
    const generation = ++retryGeneration;
    setRetrying(true);
    try {
      // This only rereads Pulse's scoped inventory; it never probes the guest.
      await source.refetch();
    } catch {
      // The query owns its failure state. Provider text may contain private data.
    } finally {
      if (generation === retryGeneration) setRetrying(false);
    }
  };
  const disks = () =>
    // A retained successful snapshot is not a current successful read. Remove
    // its rows and expanded SMART/history consumer until this query recovers.
    source.error()
      ? []
      : source
          .resources()
          .filter(
            (resource) => resource.type === 'physical_disk' && resource.parentId === parentId,
          );

  return (
    <>
      <Show when={source.error()}>
        <div class="space-y-2 rounded-md border border-border p-3 text-xs text-muted">
          <p role="status">
            Physical disk details are unavailable. Previously loaded disk readings are hidden until
            a read succeeds. Reads stop waiting after 20 seconds. Retry reads Pulse's collected
            inventory, not a guest probe.
          </p>
          <Button
            size="sm"
            class="min-h-11 min-w-11 sm:min-h-0"
            aria-label="Retry physical disk details"
            disabled={retrying() || source.loading()}
            onClick={() => void retry()}
          >
            {retrying() || source.loading() ? 'Retrying…' : 'Retry'}
          </Button>
        </div>
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
