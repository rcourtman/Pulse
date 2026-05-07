import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { Activity, Archive, Cpu, Database, Mail, Search, Server, ServerCog } from 'lucide-solid';
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
  onDetectFromAddress?: () => void;
}

const detectButtonClass =
  'inline-flex items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover';

const readinessBadgeClass =
  'inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200';

const searchInputClass =
  'h-10 w-full rounded-md border border-border bg-surface py-2 pl-9 pr-3 text-sm text-base-content outline-none transition-colors placeholder:text-muted focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20';

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
  const items = () => getInfrastructureSourcePickerItems();
  const normalizedQuery = createMemo(() => query().trim().toLowerCase());
  const visibleItems = createMemo(() =>
    items().filter((item) => itemMatchesQuery(item, normalizedQuery())),
  );
  const heading = createMemo(() => (normalizedQuery() ? 'Matching choices' : 'Common choices'));

  return (
    <div class="space-y-4 p-4">
      <div class="flex flex-col gap-2 border-b border-border pb-4 sm:flex-row">
        <label class="relative flex-1">
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
            placeholder="Search Unraid, TrueNAS, Proxmox, Docker..."
          />
        </label>
        <Show when={props.onDetectFromAddress}>
          <button
            type="button"
            onClick={props.onDetectFromAddress}
            class={`${detectButtonClass} sm:w-auto`}
          >
            <Search class="mr-2 h-4 w-4" />
            Detect from address
          </button>
        </Show>
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
              No matching source yet. Try a platform name, a host type, or use detect from address.
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
        </Show>
      </section>
    </div>
  );
};

export default InfrastructureSourcePicker;
