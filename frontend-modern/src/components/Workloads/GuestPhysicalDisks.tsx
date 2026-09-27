import { For, Show, Suspense, createSignal, lazy, type Component } from 'solid-js';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  extractPhysicalDiskPresentationData,
  getPhysicalDiskHealthStatus,
} from '@/features/storageBackups/diskPresentation';
import type { Resource } from '@/types/resource';
import { formatBytes } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';

// SMART history and chart controls are only needed after a disk row is opened.
// Keep their Storage detail module out of the initial Workloads surface.
const LazyDiskDetail = lazy(() =>
  import('@/components/Storage/DiskDetail').then(({ DiskDetail }) => ({ default: DiskDetail })),
);

const GuestPhysicalDiskRow: Component<{ disk: Resource }> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const data = () => extractPhysicalDiskPresentationData(props.disk);
  const health = () => getPhysicalDiskHealthStatus(data());
  const label = () => data().model || data().devPath || props.disk.name;

  return (
    <details
      class="rounded border border-border p-2"
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
        <Show when={data().temperature > 0}>
          <span class="text-muted">{formatTemperature(data().temperature)}</span>
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
            <LazyDiskDetail disk={props.disk} nodes={[]} />
          </Suspense>
        </div>
      </Show>
    </details>
  );
};

export const GuestPhysicalDisks: Component<{ parentId: string }> = (props) => {
  // The Proxmox Overview snapshot deliberately excludes physical disks. Query
  // only the open guest's direct children rather than hydrating every disk in
  // a large estate (or mistaking the VMID-based table key for the resource ID).
  const parentId = props.parentId.trim();
  const source = useUnifiedResources({
    query: `type=physical_disk&parent=${encodeURIComponent(parentId)}`,
    cacheKey: `guest-physical-disks:${parentId}`,
    enabled: () => parentId.length > 0,
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
            <For each={disks()}>{(disk) => <GuestPhysicalDiskRow disk={disk} />}</For>
          </div>
        </InfoCardFrame>
      </Show>
    </>
  );
};
