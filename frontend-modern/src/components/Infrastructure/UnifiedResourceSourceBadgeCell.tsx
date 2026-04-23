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

const getSourceBadgeDisplayLabel = (
  label: string,
  layoutMode: UnifiedResourceTableLayoutMode,
): string => (layoutMode === 'wide' ? label : compactSourceBadgeLabel(label));

const getCompactSourceDotClasses = (classes: string): string => {
  const toneClasses = classes
    .split(/\s+/)
    .filter((className) => className.startsWith('bg-') || className.startsWith('dark:bg-'))
    .join(' ');

  return toneClasses || 'bg-muted';
};

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
    hiddenBadgeCount() === 1
      ? `+${compactSourceBadgeLabel(hiddenBadges()[0]?.label ?? '')}`
      : `+${hiddenBadgeCount()}`,
  );
  const title = createMemo(() =>
    badges()
      .map((badge) => badge.title ?? badge.label)
      .join(', '),
  );
  const shouldUseCompactSourceList = createMemo(
    () => (props.layoutMode === 'mobile' || props.layoutMode === 'tablet') && badges().length > 0,
  );

  return (
    <div
      class="flex min-w-0 max-w-full items-center justify-center gap-1 overflow-hidden"
      aria-label={title() ? `Sources: ${title()}` : undefined}
      title={title()}
    >
      <Show
        when={shouldUseCompactSourceList()}
        fallback={
          <>
            <For each={visibleBadges()}>
              {(badge) => (
                <span
                  class={`${badge.classes} min-w-0 max-w-full overflow-hidden px-1`}
                  title={badge.title}
                >
                  <span class="min-w-0 truncate">
                    {getSourceBadgeDisplayLabel(badge.label, props.layoutMode)}
                  </span>
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
        <For each={visibleBadges()}>
          {(badge) => (
            <span
              class="inline-flex shrink-0 items-center gap-0.5 overflow-hidden rounded"
              title={badge.title}
            >
              <span
                class={`${getCompactSourceDotClasses(badge.classes)} h-1.5 w-1.5 shrink-0 rounded-full`}
                aria-hidden="true"
              />
              <span class="whitespace-nowrap text-[10px] font-medium leading-none text-base-content">
                {compactSourceBadgeLabel(badge.label)}
              </span>
            </span>
          )}
        </For>
        <Show when={hiddenBadgeCount() > 0}>
          <span
            class="shrink-0 whitespace-nowrap text-[10px] font-medium leading-none text-muted"
            aria-label={`Additional sources: ${hiddenBadges()
              .map((badge) => badge.title ?? badge.label)
              .join(', ')}`}
            title={title()}
          >
            {hiddenBadgeLabel()}
          </span>
        </Show>
      </Show>
    </div>
  );
};
