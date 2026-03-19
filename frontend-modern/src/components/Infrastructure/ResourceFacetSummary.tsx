import { For, Show, createMemo, type Component } from 'solid-js';
import type {
  ResourceCapability,
  ResourceChange,
  ResourceFacetCounts,
  ResourceRelationship,
} from '@/types/resource';
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
  capabilities?: readonly ResourceCapability[] | null;
  relationships?: readonly ResourceRelationship[] | null;
  recentChanges?: readonly ResourceChange[] | null;
  counts?: ResourceFacetCounts | null;
  showCapabilities?: boolean;
  showRelationships?: boolean;
  showTimeline?: boolean;
  class?: string;
  testId?: string;
}

const badgeBase =
  'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap';

const countLabel = (count: number, singular: string, plural = `${singular}s`) =>
  `${count} ${count === 1 ? singular : plural}`;

const buildFacetBadges = (
  capabilities?: readonly ResourceCapability[] | null,
  relationships?: readonly ResourceRelationship[] | null,
  recentChanges?: readonly ResourceChange[] | null,
  counts?: ResourceFacetCounts | null,
  visibility: { showCapabilities: boolean; showRelationships: boolean; showTimeline: boolean } = {
    showCapabilities: true,
    showRelationships: true,
    showTimeline: true,
  },
): FacetBadge[] => {
  const badges: FacetBadge[] = [];
  const capabilityCount = counts?.capabilities ?? capabilities?.length ?? 0;
  const relationshipCount = counts?.relationships ?? relationships?.length ?? 0;
  const changeCount = counts?.recentChanges ?? recentChanges?.length ?? 0;

  if (visibility.showCapabilities && capabilityCount > 0) {
    badges.push({
      label: `Capabilities ${capabilityCount}`,
      title: countLabel(capabilityCount, 'capability'),
      className: `${badgeBase} bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300`,
    });
  }

  if (visibility.showRelationships && relationshipCount > 0) {
    badges.push({
      label: `Relationships ${relationshipCount}`,
      title: countLabel(relationshipCount, 'relationship'),
      className: `${badgeBase} bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300`,
    });
  }

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
    buildFacetBadges(props.capabilities, props.relationships, props.recentChanges, props.counts, {
      showCapabilities: props.showCapabilities ?? true,
      showRelationships: props.showRelationships ?? true,
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
