import { For, Show, createMemo, type Component } from 'solid-js';
import type {
  ResourceCapability,
  ResourceChange,
  ResourceChangeKind,
  ResourceFacetCounts,
  ResourceRelationship,
} from '@/types/resource';

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
  class?: string;
  testId?: string;
}

const badgeBase =
  'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap';

const countLabel = (count: number, singular: string, plural = `${singular}s`) =>
  `${count} ${count === 1 ? singular : plural}`;

const recentChangeKindOrder: ResourceChangeKind[] = [
  'state_transition',
  'restart',
  'config_update',
  'metric_anomaly',
  'relationship_change',
  'capability_change',
];

const recentChangeKindLabels: Record<
  ResourceChangeKind,
  { label: string; plural: string; className: string }
> = {
  state_transition: {
    label: 'State transition',
    plural: 'State transitions',
    className: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
  },
  restart: {
    label: 'Restart',
    plural: 'Restarts',
    className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  },
  config_update: {
    label: 'Config update',
    plural: 'Config updates',
    className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  },
  metric_anomaly: {
    label: 'Anomaly',
    plural: 'Anomalies',
    className: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
  },
  relationship_change: {
    label: 'Relationship change',
    plural: 'Relationship changes',
    className: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300',
  },
  capability_change: {
    label: 'Capability change',
    plural: 'Capability changes',
    className: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300',
  },
};

const buildFacetBadges = (
  capabilities?: readonly ResourceCapability[] | null,
  relationships?: readonly ResourceRelationship[] | null,
  recentChanges?: readonly ResourceChange[] | null,
  counts?: ResourceFacetCounts | null,
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

  const kindCounts = counts?.recentChangeKinds;
  if (kindCounts) {
    for (const kind of recentChangeKindOrder) {
      const count = kindCounts[kind];
      if (!count || count <= 0) continue;
      const kindLabel = recentChangeKindLabels[kind];
      badges.push({
        label: `${kindLabel.label} ${count}`,
        title: countLabel(count, kindLabel.label.toLowerCase(), kindLabel.plural.toLowerCase()),
        className: `${badgeBase} ${kindLabel.className}`,
      });
    }
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
