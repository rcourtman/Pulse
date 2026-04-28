import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import type { ResourceBadge } from '@/utils/resourceBadgePresentation';
import type { UnifiedResourceTableLayoutMode } from './unifiedResourceTableStateModel';

interface UnifiedResourceSourceBadgeCellProps {
  unifiedBadges: ResourceBadge[];
  platformBadge: ResourceBadge | null;
  sourceBadge: ResourceBadge | null;
  titleBadges?: ResourceBadge[];
  layoutMode: UnifiedResourceTableLayoutMode;
}

const getVisibleSourceBadgeLimit = (layoutMode: UnifiedResourceTableLayoutMode): number =>
  layoutMode === 'wide' ? 3 : 2;

export const UnifiedResourceSourceBadgeCell: Component<UnifiedResourceSourceBadgeCellProps> = (
  props,
) => {
  const badges = createMemo(() => {
    if (props.unifiedBadges.length > 0) return props.unifiedBadges;
    return [props.platformBadge, props.platformBadge ? null : props.sourceBadge].filter(
      (badge): badge is ResourceBadge => Boolean(badge),
    );
  });
  const titleBadges = createMemo(() =>
    props.titleBadges && props.titleBadges.length > 0 ? props.titleBadges : badges(),
  );
  const visibleBadges = createMemo(() =>
    badges().slice(0, getVisibleSourceBadgeLimit(props.layoutMode)),
  );
  const hiddenBadges = createMemo(() => badges().slice(visibleBadges().length));
  const hiddenBadgeCount = createMemo(() => Math.max(0, badges().length - visibleBadges().length));
  const hiddenBadgeLabel = createMemo(() =>
    hiddenBadgeCount() === 1 ? `+${hiddenBadges()[0]?.label ?? ''}` : `+${hiddenBadgeCount()}`,
  );
  const title = createMemo(() =>
    titleBadges()
      .map((badge) => badge.title ?? badge.label)
      .join(', '),
  );

  return (
    <div
      class="flex min-w-0 max-w-full items-center justify-center gap-1 overflow-hidden"
      aria-label={title() ? `System: ${title()}` : undefined}
      title={title()}
    >
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
          aria-label={`Additional systems: ${hiddenBadges()
            .map((badge) => badge.title ?? badge.label)
            .join(', ')}`}
          title={title()}
        >
          <span class="min-w-0 truncate">{hiddenBadgeLabel()}</span>
        </span>
      </Show>
    </div>
  );
};
