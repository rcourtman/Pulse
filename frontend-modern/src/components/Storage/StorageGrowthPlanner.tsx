import { Component, For, Show } from 'solid-js';
import type {
  StorageGrowthPlannerPool,
  StorageGrowthPlannerPresentation,
} from '@/features/storageBackups/storageGrowthPlannerPresentation';

type StorageGrowthPlannerProps = {
  planner: StorageGrowthPlannerPresentation;
  loaded: boolean;
  fetchFailed: boolean;
  activeSeriesId?: string | null;
  onFocusPool?: (seriesId: string) => void;
  onHoverPool?: (seriesId: string | null) => void;
};

const STORAGE_GROWTH_PLANNER_FRAME_CLASS = 'rounded-md border border-border bg-surface shadow-sm';
const STORAGE_GROWTH_PLANNER_HEADER_CLASS =
  'flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-3 py-2';
const STORAGE_GROWTH_PLANNER_LIST_CLASS = 'grid gap-2 p-2 sm:grid-cols-2 xl:grid-cols-3';
const STORAGE_GROWTH_PLANNER_ITEM_CLASS =
  'min-w-0 rounded border border-border bg-surface-alt px-2.5 py-2 text-left transition-colors hover:border-sky-300 hover:bg-surface focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40';
const STORAGE_GROWTH_PLANNER_ACTIVE_ITEM_CLASS =
  'border-sky-400 bg-sky-50 ring-2 ring-inset ring-sky-500/20 dark:bg-sky-950/30';
const STORAGE_GROWTH_PLANNER_EMPTY_CLASS = 'px-3 py-4 text-sm text-muted';
const STORAGE_GROWTH_PLANNER_SKELETON_CLASS =
  'h-[74px] animate-pulse rounded border border-border bg-surface-alt';

const StorageGrowthPlannerItem: Component<{
  pool: StorageGrowthPlannerPool;
  active: boolean;
  onFocus?: (seriesId: string) => void;
  onHover?: (seriesId: string | null) => void;
}> = (props) => (
  <button
    type="button"
    class={`${STORAGE_GROWTH_PLANNER_ITEM_CLASS} ${
      props.active ? STORAGE_GROWTH_PLANNER_ACTIVE_ITEM_CLASS : ''
    }`.trim()}
    title={props.pool.runwayTitle}
    data-testid={`storage-growth-planner-pool-${props.pool.seriesId}`}
    data-summary-series-id={props.pool.seriesId}
    data-storage-growth-priority={props.pool.priority}
    onClick={() => props.onFocus?.(props.pool.seriesId)}
    onPointerEnter={(event) => {
      if (event.pointerType !== 'touch') {
        props.onHover?.(props.pool.seriesId);
      }
    }}
    onPointerLeave={(event) => {
      if (event.pointerType !== 'touch') {
        props.onHover?.(null);
      }
    }}
    onFocus={() => props.onHover?.(props.pool.seriesId)}
    onBlur={() => props.onHover?.(null)}
  >
    <div class="flex min-w-0 items-start justify-between gap-2">
      <div class="min-w-0">
        <div class="truncate text-[12px] font-semibold text-base-content">{props.pool.name}</div>
        <div class="truncate text-[11px] text-muted">{props.pool.hostLabel}</div>
      </div>
      <span
        class={`shrink-0 rounded-full border px-1.5 py-0.5 text-[10px] font-semibold ${props.pool.priorityToneClass}`}
      >
        {props.pool.priorityLabel}
      </span>
    </div>
    <div class="mt-2 grid grid-cols-3 gap-2 text-[11px]">
      <div class="min-w-0">
        <div class="text-muted">Growth</div>
        <div class={`truncate font-mono font-semibold ${props.pool.growthToneClass}`}>
          {props.pool.growthLabel}
        </div>
      </div>
      <div class="min-w-0">
        <div class="text-muted">Runway</div>
        <div class="truncate font-semibold text-base-content">{props.pool.runwayLabel}</div>
      </div>
      <div class="min-w-0">
        <div class="text-muted">{props.pool.usageLabel}</div>
        <div class="truncate text-base-content">{props.pool.freeLabel}</div>
      </div>
    </div>
  </button>
);

export const StorageGrowthPlanner: Component<StorageGrowthPlannerProps> = (props) => {
  const headerSummary = () => {
    if (props.fetchFailed) {
      return 'History unavailable';
    }
    return `${props.planner.growingPoolCount}/${props.planner.trackedPoolCount} growing`;
  };

  return (
    <section
      class={STORAGE_GROWTH_PLANNER_FRAME_CLASS}
      data-testid="storage-growth-planner"
      aria-label="Storage growth planner"
    >
      <div class={STORAGE_GROWTH_PLANNER_HEADER_CLASS}>
        <div class="min-w-0">
          <h2 class="text-sm font-semibold text-base-content">Growth Planner</h2>
          <p class="text-[11px] text-muted">
            {props.planner.rangeLabel} runway from storage history
          </p>
        </div>
        <div class="text-[11px] font-medium text-muted">{headerSummary()}</div>
      </div>
      <Show
        when={props.loaded}
        fallback={
          <div class={STORAGE_GROWTH_PLANNER_LIST_CLASS}>
            <div class={STORAGE_GROWTH_PLANNER_SKELETON_CLASS} />
            <div class={STORAGE_GROWTH_PLANNER_SKELETON_CLASS} />
            <div class={STORAGE_GROWTH_PLANNER_SKELETON_CLASS} />
          </div>
        }
      >
        <Show
          when={!props.fetchFailed && props.planner.topPools.length > 0}
          fallback={
            <div class={STORAGE_GROWTH_PLANNER_EMPTY_CLASS}>
              <div class="font-medium text-base-content">{props.planner.emptyTitle}</div>
              <div class="mt-0.5">
                {props.fetchFailed
                  ? 'Trend history could not be loaded.'
                  : props.planner.emptyMessage}
              </div>
            </div>
          }
        >
          <div class={STORAGE_GROWTH_PLANNER_LIST_CLASS}>
            <For each={props.planner.topPools}>
              {(pool) => (
                <StorageGrowthPlannerItem
                  pool={pool}
                  active={props.activeSeriesId === pool.seriesId}
                  onFocus={props.onFocusPool}
                  onHover={props.onHoverPool}
                />
              )}
            </For>
          </div>
        </Show>
      </Show>
    </section>
  );
};

export default StorageGrowthPlanner;
