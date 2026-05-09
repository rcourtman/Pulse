import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import {
  Activity,
  Archive,
  Cpu,
  Database,
  Download,
  Mail,
  Search,
  Server,
  ServerCog,
} from 'lucide-solid';
import type {
  InfrastructureSourcePickerItemId,
  InfrastructureSourcePickerItemPresentation,
  InfrastructureSourcePickerRouteStep,
} from '@/utils/infrastructureOnboardingPresentation';
import {
  getInfrastructureGovernanceBadgeLabel,
  getInfrastructureSourcePickerItems,
} from '@/utils/infrastructureOnboardingPresentation';

interface InfrastructureSourcePickerProps {
  onSelectStep: (step: InfrastructureSourcePickerRouteStep) => void;
  onDetectApiPlatform?: () => void;
}

const readinessBadgeClass =
  'inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200';

const searchInputClass =
  'h-10 w-full rounded-md border border-border bg-surface py-2 pl-9 pr-3 text-sm text-base-content outline-none transition-colors placeholder:text-muted focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20';

// Primary-path cards present the two onboarding journeys (API connection vs
// agent install) up front so users pick a path before scanning the per-
// platform card grid. The grid below stays as a direct alternative for users
// who already know which platform they want.
const primaryPathCardClass =
  'group flex h-full items-start gap-3 rounded-md border border-blue-200 bg-blue-50 p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/40 dark:hover:bg-blue-900';

const CARD_ICON: Record<InfrastructureSourcePickerItemId, Component<{ class?: string }>> = {
  vmware: ServerCog,
  pve: Server,
  truenas: Database,
  unraid: Database,
  pbs: Archive,
  pmg: Mail,
  'linux-host': Cpu,
  docker: Server,
  kubernetes: ServerCog,
  availability: Activity,
};

// Popular platforms shown by default. The rest are hidden behind 'Show more
// platforms' to keep the picker scannable at a glance and let users scroll
// to fewer cards before deciding. Search bypasses this split — searching
// for any platform surfaces it regardless of popular status.
const POPULAR_PICKER_ITEM_IDS: ReadonlySet<InfrastructureSourcePickerItemId> = new Set([
  'pve',
  'truenas',
  'unraid',
  'linux-host',
  'availability',
]);

const itemMatchesQuery = (item: InfrastructureSourcePickerItemPresentation, query: string) => {
  if (!query) return true;
  const searchableText = [
    item.label,
    item.catalogDescription,
    item.bestFor,
    item.coverage,
    item.id,
    item.connectionType,
    ...item.searchAliases,
  ]
    .join(' ')
    .toLowerCase();
  return query
    .split(/\s+/)
    .filter(Boolean)
    .every((term) => searchableText.includes(term));
};

export const InfrastructureSourcePicker: Component<InfrastructureSourcePickerProps> = (props) => {
  const [query, setQuery] = createSignal('');
  const [showAllPlatforms, setShowAllPlatforms] = createSignal(false);
  const items = () => getInfrastructureSourcePickerItems();
  const normalizedQuery = createMemo(() => query().trim().toLowerCase());
  const matchedItems = createMemo(() =>
    items().filter((item) => itemMatchesQuery(item, normalizedQuery())),
  );
  // When the user is searching, show every match regardless of popular
  // status. Otherwise gate the long tail behind a 'Show more platforms'
  // disclosure so the default scan is short.
  const visibleItems = createMemo(() => {
    if (normalizedQuery() || showAllPlatforms()) return matchedItems();
    return matchedItems().filter((item) => POPULAR_PICKER_ITEM_IDS.has(item.id));
  });
  const hiddenCount = createMemo(() =>
    normalizedQuery() || showAllPlatforms()
      ? 0
      : matchedItems().filter((item) => !POPULAR_PICKER_ITEM_IDS.has(item.id)).length,
  );
  const heading = createMemo(() =>
    normalizedQuery() ? 'Matching choices' : 'Or pick a specific platform',
  );

  return (
    <div class="space-y-4 p-4">
      <Show when={!normalizedQuery()}>
        <section class="space-y-2">
          <h3 class="text-sm font-semibold text-base-content">Choose how Pulse should connect</h3>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Show when={props.onDetectApiPlatform}>
              <button type="button" onClick={props.onDetectApiPlatform} class={primaryPathCardClass}>
                <div
                  aria-hidden="true"
                  class="flex h-10 w-10 flex-none items-center justify-center rounded-md border border-blue-200 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200"
                >
                  <Search class="h-5 w-5" />
                </div>
                <div class="min-w-0 flex-1 space-y-1">
                  <div class="text-sm font-semibold text-base-content">Detect API platform</div>
                  <p class="text-xs leading-5 text-muted">
                    Paste a hostname, IP, or URL. Pulse identifies the platform (Proxmox, TrueNAS,
                    VMware, PBS, PMG) and opens the right credential form.
                  </p>
                </div>
              </button>
            </Show>
            <button
              type="button"
              onClick={() => props.onSelectStep('linux-host')}
              class={primaryPathCardClass}
            >
              <div
                aria-hidden="true"
                class="flex h-10 w-10 flex-none items-center justify-center rounded-md border border-blue-200 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200"
              >
                <Download class="h-5 w-5" />
              </div>
              <div class="min-w-0 flex-1 space-y-1">
                <div class="text-sm font-semibold text-base-content">Install Pulse Agent</div>
                <p class="text-xs leading-5 text-muted">
                  Run an installer on a host (Linux, macOS, Windows, Unraid). Pulse classifies the
                  host profile from its OS and starts collecting telemetry.
                </p>
              </div>
            </button>
          </div>
        </section>
      </Show>

      <div class="border-b border-border pb-4">
        <label class="relative block">
          <span class="sr-only">Search infrastructure type</span>
          <Search
            aria-hidden="true"
            class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted"
          />
          <input
            type="search"
            value={query()}
            onInput={(event) => setQuery(event.currentTarget.value)}
            class={searchInputClass}
            placeholder="Search platforms, hosts, services..."
          />
        </label>
      </div>

      <section class="space-y-2">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-base-content">{heading()}</h3>
          <Show when={normalizedQuery()}>
            <div class="text-xs text-muted">
              {visibleItems().length} match{visibleItems().length === 1 ? '' : 'es'}
            </div>
          </Show>
        </div>

        <Show
          when={visibleItems().length > 0}
          fallback={
            <div class="rounded-md border border-dashed border-border bg-surface-alt p-4 text-sm text-muted">
              No matching source yet. Try a platform, host type, service, or API endpoint address.
            </div>
          }
        >
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            <For each={visibleItems()}>
              {(item) => {
                const Icon = CARD_ICON[item.id];
                const governanceBadge = getInfrastructureGovernanceBadgeLabel(
                  item.governanceState,
                  item.readinessStage,
                );
                return (
                  <button
                    type="button"
                    onClick={() => props.onSelectStep(item.routeStep)}
                    class="group flex h-full min-h-[82px] items-start gap-3 rounded-md border border-border bg-surface p-3 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
                  >
                    <div
                      aria-hidden="true"
                      class="flex h-9 w-9 flex-none items-center justify-center rounded-md border border-border bg-surface-alt text-base-content"
                    >
                      <Icon class="h-4 w-4" />
                    </div>
                    <div class="min-w-0 flex-1 space-y-2">
                      <div class="flex flex-wrap items-center gap-2">
                        <div class="text-sm font-semibold text-base-content">{item.label}</div>
                        <Show when={governanceBadge}>
                          <span class={readinessBadgeClass}>{governanceBadge}</span>
                        </Show>
                      </div>
                      <p class="text-xs leading-5 text-muted">{item.catalogDescription}</p>
                    </div>
                  </button>
                );
              }}
            </For>
          </div>
          <Show when={hiddenCount() > 0}>
            <button
              type="button"
              onClick={() => setShowAllPlatforms(true)}
              class="mt-1 inline-flex items-center text-xs font-medium text-blue-700 hover:text-blue-900 hover:underline dark:text-blue-300 dark:hover:text-blue-100"
            >
              Show {hiddenCount()} more platform{hiddenCount() === 1 ? '' : 's'}
            </button>
          </Show>
        </Show>
      </section>
    </div>
  );
};

export default InfrastructureSourcePicker;
