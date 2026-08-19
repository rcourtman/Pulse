import { A } from '@solidjs/router';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import EyeIcon from 'lucide-solid/icons/eye';
import EyeOffIcon from 'lucide-solid/icons/eye-off';
import ServerOffIcon from 'lucide-solid/icons/server-off';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import { For, Show, createMemo } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { TableCard } from '@/components/shared/TableCard';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import type { Resource } from '@/types/resource';
import {
  PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY,
  buildPlatformEstateOverviewModel,
  deserializePlatformEstateOverviewVisibility,
  formatPlatformEstateMetricValue,
  type PlatformEstateOverviewPlatform,
} from './platformEstateOverviewModel';

export function PlatformEstateOverview(props: {
  platform: PlatformEstateOverviewPlatform;
  resources: readonly Resource[];
}) {
  const [visible, setVisible] = usePersistentSignal(PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY, true, {
    deserialize: deserializePlatformEstateOverviewVisibility,
  });
  const model = createMemo(() => buildPlatformEstateOverviewModel(props.platform, props.resources));

  return (
    <Show
      when={visible()}
      fallback={
        <div class="flex justify-end" data-platform-estate-overview-hidden>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            class="gap-2"
            onClick={() => setVisible(true)}
          >
            <EyeIcon class="h-3.5 w-3.5" aria-hidden="true" />
            Show estate overview
          </Button>
        </div>
      }
    >
      <section
        aria-labelledby={`${props.platform}-estate-overview-title`}
        data-platform-estate-overview={props.platform}
        class="space-y-2"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="min-w-0">
            <h2
              id={`${props.platform}-estate-overview-title`}
              class="text-xs font-semibold uppercase tracking-wider text-muted"
            >
              Estate at a glance
            </h2>
            <p class="mt-0.5 text-xs text-muted">
              Live inventory and the most important operational signals.
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            class="shrink-0 gap-2"
            onClick={() => setVisible(false)}
          >
            <EyeOffIcon class="h-3.5 w-3.5" aria-hidden="true" />
            Hide
          </Button>
        </div>

        <TableCard class="overflow-hidden">
          <div class="grid grid-cols-2 lg:grid-cols-4" data-platform-estate-metrics>
            <For each={model().metrics}>
              {(item, index) => (
                <div
                  class={`min-w-0 px-3 py-3 ${index() % 2 === 0 ? 'border-r border-border' : ''} ${
                    index() < 2 ? 'border-b border-border lg:border-b-0' : ''
                  } ${index() < 3 ? 'lg:border-r lg:border-border' : 'lg:border-r-0'}`}
                  data-platform-estate-metric={item.id}
                >
                  <div class="text-lg font-semibold tabular-nums text-base-content">
                    {formatPlatformEstateMetricValue(item.value)}
                  </div>
                  <div class="mt-0.5 truncate text-[11px] text-muted" title={item.label}>
                    {item.label}
                  </div>
                </div>
              )}
            </For>
          </div>
        </TableCard>

        <Show when={model().spotlights.length > 0}>
          <div class="pt-1">
            <div class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted">
              Operational spotlights
            </div>
            <div class="grid gap-2 lg:grid-cols-3" data-platform-estate-spotlights>
              <For each={model().spotlights}>
                {(spotlight) => (
                  <A
                    href={spotlight.href}
                    class="group flex min-w-0 items-center gap-2 rounded-md border border-border bg-surface px-3 py-2 text-left transition-colors hover:border-blue-400 hover:bg-surface-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
                    data-platform-estate-spotlight={spotlight.resourceId}
                  >
                    <span
                      class={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
                        spotlight.tone === 'danger'
                          ? 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-300'
                          : 'bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300'
                      }`}
                    >
                      <Show
                        when={spotlight.tone === 'danger'}
                        fallback={<TriangleAlertIcon class="h-4 w-4" aria-hidden="true" />}
                      >
                        <ServerOffIcon class="h-4 w-4" aria-hidden="true" />
                      </Show>
                    </span>
                    <span class="min-w-0 flex-1">
                      <span class="block truncate text-xs font-medium text-base-content">
                        {spotlight.label}
                      </span>
                      <span class="mt-0.5 block truncate text-[11px] text-muted">
                        {spotlight.meta}
                      </span>
                    </span>
                    <ChevronRightIcon
                      class="h-4 w-4 shrink-0 text-muted transition-transform group-hover:translate-x-0.5"
                      aria-hidden="true"
                    />
                  </A>
                )}
              </For>
            </div>
          </div>
        </Show>
      </section>
    </Show>
  );
}

export default PlatformEstateOverview;
