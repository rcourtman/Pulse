import { For, Show, createMemo, type Component } from 'solid-js';
import type { ResourceChange, ResourceFacetCounts } from '@/types/resource';
import {
  RESOURCE_CHANGE_KIND_ORDER,
  RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER,
  RESOURCE_CHANGE_SOURCE_TYPE_ORDER,
  getResourceChangeKindPresentation,
  getResourceChangeSourceAdapterPresentation,
  getResourceChangeSourceTypePresentation,
} from '@/utils/resourceChangePresentation';

type FacetBadge = {
  label: string;
  title: string;
  className: string;
};

export interface ResourceFacetSummaryProps {
  recentChanges?: readonly ResourceChange[] | null;
  counts?: ResourceFacetCounts | null;
  showTimeline?: boolean;
  class?: string;
  testId?: string;
}

const badgeBase =
  'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap';

const countLabel = (count: number, singular: string, plural = `${singular}s`) =>
  `${count} ${count === 1 ? singular : plural}`;

const buildFacetBadges = (
  recentChanges?: readonly ResourceChange[] | null,
  counts?: ResourceFacetCounts | null,
  visibility: { showTimeline: boolean } = { showTimeline: true },
): FacetBadge[] => {
  const badges: FacetBadge[] = [];
  const changeCount = counts?.recentChanges ?? recentChanges?.length ?? 0;

  if (visibility.showTimeline && changeCount > 0) {
    badges.push({
      label: `Timeline ${changeCount}`,
      title: countLabel(changeCount, 'timeline event'),
      className: `${badgeBase} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`,
    });
  }

  const kindCounts = visibility.showTimeline ? counts?.recentChangeKinds : null;
  if (kindCounts) {
    for (const kind of RESOURCE_CHANGE_KIND_ORDER) {
      const count = kindCounts[kind];
      if (!count || count <= 0) continue;
      const kindLabel = getResourceChangeKindPresentation(kind);
      badges.push({
        label: `${kindLabel.label} ${count}`,
        title: countLabel(count, kindLabel.label.toLowerCase(), kindLabel.plural.toLowerCase()),
        className: `${badgeBase} ${kindLabel.className}`,
      });
    }
  }

  const sourceTypeCounts = visibility.showTimeline ? counts?.recentChangeSourceTypes : null;
  if (sourceTypeCounts) {
    for (const sourceType of RESOURCE_CHANGE_SOURCE_TYPE_ORDER) {
      const count = sourceTypeCounts[sourceType];
      if (!count || count <= 0) continue;
      const sourceTypeLabel = getResourceChangeSourceTypePresentation(sourceType);
      badges.push({
        label: `${sourceTypeLabel.label} ${count}`,
        title: countLabel(
          count,
          sourceTypeLabel.label.toLowerCase(),
          sourceTypeLabel.plural.toLowerCase(),
        ),
        className: `${badgeBase} ${sourceTypeLabel.className}`,
      });
    }
  }

  const sourceAdapterCounts = visibility.showTimeline ? counts?.recentChangeSourceAdapters : null;
  if (sourceAdapterCounts) {
    for (const sourceAdapter of RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER) {
      const count = sourceAdapterCounts[sourceAdapter];
      if (!count || count <= 0) continue;
      const sourceAdapterLabel = getResourceChangeSourceAdapterPresentation(sourceAdapter);
      badges.push({
        label: `${sourceAdapterLabel.label} ${count}`,
        title: countLabel(
          count,
          sourceAdapterLabel.label.toLowerCase(),
          sourceAdapterLabel.plural.toLowerCase(),
        ),
        className: `${badgeBase} ${sourceAdapterLabel.className}`,
      });
    }
  }

  return badges;
};

export const ResourceFacetSummary: Component<ResourceFacetSummaryProps> = (props) => {
  const badges = createMemo(() =>
    buildFacetBadges(props.recentChanges, props.counts, {
      showTimeline: props.showTimeline ?? true,
    }),
  );

  return (
    <Show when={badges().length > 0}>
      <div
        data-testid={props.testId ?? 'resource-facet-summary'}
        class={`flex flex-wrap gap-1 ${props.class ?? ''}`}
      >
        <For each={badges()}>
          {(badge) => (
            <span class={badge.className} title={badge.title}>
              {badge.label}
            </span>
          )}
        </For>
      </div>
    </Show>
  );
};

export default ResourceFacetSummary;
