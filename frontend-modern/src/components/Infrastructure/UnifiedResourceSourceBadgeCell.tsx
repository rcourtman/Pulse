import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import type { ResourceBadge } from '@/utils/resourceBadgePresentation';
import type { UnifiedResourceTableLayoutMode } from './unifiedResourceTableStateModel';

interface UnifiedResourceSourceBadgeCellProps {
  unifiedBadges: ResourceBadge[];
  platformBadge: ResourceBadge | null;
  sourceBadge: ResourceBadge | null;
  layoutMode: UnifiedResourceTableLayoutMode;
}

const getVisibleSourceBadgeLimit = (layoutMode: UnifiedResourceTableLayoutMode): number =>
  layoutMode === 'wide' ? 3 : 2;

const compactSourceBadgeLabel = (label: string): string =>
  label === 'Containers' ? 'Cont' : label;

export const UnifiedResourceSourceBadgeCell: Component<UnifiedResourceSourceBadgeCellProps> = (
  props,
) => {
  const badges = createMemo(() => {
    if (props.unifiedBadges.length > 0) return props.unifiedBadges;
    return [props.platformBadge, props.sourceBadge].filter((badge): badge is ResourceBadge =>
      Boolean(badge),
    );
  });
  const visibleBadges = createMemo(() =>
    badges().slice(0, getVisibleSourceBadgeLimit(props.layoutMode)),
  );
  const hiddenBadges = createMemo(() => badges().slice(visibleBadges().length));
  const hiddenBadgeCount = createMemo(() => Math.max(0, badges().length - visibleBadges().length));
  const hiddenBadgeLabel = createMemo(() =>
    hiddenBadgeCount() === 1 ? `+${hiddenBadges()[0]?.label ?? ''}` : `+${hiddenBadgeCount()}`,
  );
  const title = createMemo(() =>
    badges()
      .map((badge) => badge.title ?? badge.label)
      .join(', '),
  );
  const summaryBadge = createMemo(() => {
    const allBadges = badges();
    if (props.layoutMode === 'wide' || allBadges.length === 0) return null;

    const badge = allBadges[0];
    if (!badge) return null;

    const visibleLabels = allBadges
      .slice(0, 2)
      .map((badge) => compactSourceBadgeLabel(badge.label));
    const hiddenCount = allBadges.length - visibleLabels.length;
    return {
      badge,
      label: `${visibleLabels.join('+')}${hiddenCount > 0 ? `+${hiddenCount}` : ''}`,
    };
  });

  return (
    <div class="flex min-w-0 max-w-full items-center justify-center gap-1 overflow-hidden">
      <Show
        when={summaryBadge()}
        fallback={
          <>
            <For each={visibleBadges()}>
              {(badge) => (
                <span
                  class={`${badge.classes} min-w-0 max-w-full overflow-hidden px-1`}
                  title={badge.title}
                >
                  <span class="min-w-0 truncate">{badge.label}</span>
                </span>
              )}
            </For>
            <Show when={hiddenBadgeCount() > 0}>
              <span
                class="inline-flex min-w-0 max-w-full items-center overflow-hidden rounded bg-surface-alt px-1 py-0.5 text-[10px] font-medium text-muted"
                aria-label={`Additional sources: ${hiddenBadges()
                  .map((badge) => badge.title ?? badge.label)
                  .join(', ')}`}
                title={title()}
              >
                <span class="min-w-0 truncate">{hiddenBadgeLabel()}</span>
              </span>
            </Show>
          </>
        }
      >
        {(summary) => (
          <span
            class={`${summary().badge.classes} w-full min-w-0 max-w-full justify-center overflow-hidden px-1.5`}
            aria-label={`Sources: ${title()}`}
            title={title()}
          >
            <span class="min-w-0 truncate">{summary().label}</span>
          </span>
        )}
      </Show>
    </div>
  );
};
