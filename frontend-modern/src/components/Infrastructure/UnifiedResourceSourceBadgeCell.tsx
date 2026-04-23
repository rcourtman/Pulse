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
  layoutMode === 'wide' ? 3 : 1;

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
  const hiddenBadgeCount = createMemo(() => Math.max(0, badges().length - visibleBadges().length));
  const title = createMemo(() =>
    badges()
      .map((badge) => badge.title ?? badge.label)
      .join(', '),
  );

  return (
    <div class="flex min-w-0 max-w-full items-center justify-center gap-1 overflow-hidden">
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
          title={title()}
        >
          +{hiddenBadgeCount()}
        </span>
      </Show>
    </div>
  );
};
