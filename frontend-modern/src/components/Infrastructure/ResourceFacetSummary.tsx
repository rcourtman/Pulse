import { For, Show, createMemo, type Component } from 'solid-js';
import type {
  ResourceCapability,
  ResourceChange,
  ResourceRelationship,
} from '@/types/resource';

type FacetBadge = {
  label: string;
  title: string;
  className: string;
};

export interface ResourceFacetSummaryCounts {
  capabilities?: number | null;
  relationships?: number | null;
  recentChanges?: number | null;
}

export interface ResourceFacetSummaryProps {
  capabilities?: readonly ResourceCapability[] | null;
  relationships?: readonly ResourceRelationship[] | null;
  recentChanges?: readonly ResourceChange[] | null;
  counts?: ResourceFacetSummaryCounts | null;
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
  counts?: ResourceFacetSummaryCounts | null,
): FacetBadge[] => {
  const badges: FacetBadge[] = [];
  const capabilityCount = counts?.capabilities ?? capabilities?.length ?? 0;
  const relationshipCount = counts?.relationships ?? relationships?.length ?? 0;
  const changeCount = counts?.recentChanges ?? recentChanges?.length ?? 0;

  if (capabilityCount > 0) {
    badges.push({
      label: `Capabilities ${capabilityCount}`,
      title: countLabel(capabilityCount, 'capability'),
      className: `${badgeBase} bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300`,
    });
  }

  if (relationshipCount > 0) {
    badges.push({
      label: `Relationships ${relationshipCount}`,
      title: countLabel(relationshipCount, 'relationship'),
      className: `${badgeBase} bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300`,
    });
  }

  if (changeCount > 0) {
    badges.push({
      label: `Timeline ${changeCount}`,
      title: countLabel(changeCount, 'timeline event'),
      className: `${badgeBase} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`,
    });
  }

  return badges;
};

export const ResourceFacetSummary: Component<ResourceFacetSummaryProps> = (props) => {
  const badges = createMemo(() =>
    buildFacetBadges(props.capabilities, props.relationships, props.recentChanges, props.counts),
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
