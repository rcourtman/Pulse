import { type Component, For, Show, createMemo } from 'solid-js';
import type { ResourceChange } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';
import { buildInfrastructureResourceHref } from '@/routing/resourceLinks';
import {
  formatResourceChangeHeadline,
  formatResourceChangeKind,
} from '@/utils/resourceChangePresentation';

interface ResourceChangeSummaryProps {
  changes?: ResourceChange[] | null;
  title: string;
  subtitle?: string;
  emptyText?: string;
  buildResourceHref?: (resourceId: string) => string | null | undefined;
  maxChanges?: number;
  compact?: boolean;
  class?: string;
}

const sortRecentChanges = (changes: ResourceChange[]): ResourceChange[] =>
  [...changes].sort((left, right) => {
    const observedDelta =
      new Date(right.observedAt).getTime() - new Date(left.observedAt).getTime();
    if (observedDelta !== 0) {
      return observedDelta;
    }

    return right.id.localeCompare(left.id);
  });

export const ResourceChangeSummary: Component<ResourceChangeSummaryProps> = (props) => {
  const sortedChanges = createMemo(() => sortRecentChanges(props.changes ?? []));
  const maxChanges = () => props.maxChanges ?? sortedChanges().length;
  const visibleChanges = createMemo(() => sortedChanges().slice(0, maxChanges()));
  const hasChanges = () => visibleChanges().length > 0;
  const compact = () => props.compact ?? false;
  const buildResourceHref = props.buildResourceHref ?? buildInfrastructureResourceHref;
  const itemPadding = () => (compact() ? 'p-2.5' : 'p-3');
  const itemText = () => (compact() ? 'text-[11px]' : 'text-xs');
  const headlineText = () => (compact() ? 'text-[11px]' : 'text-sm');
  const badgeText = () => (compact() ? 'text-[10px]' : 'text-[11px]');
  const gapSize = () => (compact() ? 'gap-2' : 'gap-3');

  const renderRelatedResource = (resourceId: string) => {
    const href = buildResourceHref(resourceId);
    const className =
      'rounded-full border border-border-subtle bg-surface px-2 py-0.5 font-medium text-muted transition-colors hover:border-border hover:text-base-content';

    return href ? (
      <a
        href={href}
        class={className}
        aria-label={`Open related resource ${resourceId} in Infrastructure`}
      >
        {resourceId}
      </a>
    ) : (
      <span class={className}>{resourceId}</span>
    );
  };

  return (
    <section class={props.class}>
      <div class="flex items-start justify-between gap-2">
        <div class="min-w-0">
          <h3 class={`font-semibold text-base-content ${headlineText()}`}>{props.title}</h3>
          <Show when={props.subtitle}>
            <p class="text-xs text-muted">{props.subtitle}</p>
          </Show>
        </div>
      </div>

      <Show
        when={hasChanges()}
        fallback={
          <p class={`mt-3 ${itemText()} text-muted`}>
            {props.emptyText ?? 'No canonical changes were recorded in the last 24 hours.'}
          </p>
        }
      >
        <ul class="mt-3 space-y-2">
          <For each={visibleChanges()}>
            {(change) => {
              const headline = formatResourceChangeHeadline(change);
              const resourceHref = buildResourceHref(change.resourceId);
              const relatedResources = (change.relatedResources ?? []).slice(0, 3);

              return (
                <li class={`rounded-md border border-border-subtle bg-base ${itemPadding()}`}>
                  <div class={`flex flex-wrap items-start justify-between ${gapSize()}`}>
                    <div class="min-w-0">
                      <p class={headlineText()}>
                        {headline}
                      </p>
                      <p class={`mt-1 ${itemText()} text-muted`}>
                        {resourceHref ? (
                          <a
                            href={resourceHref}
                            class="font-mono hover:text-base-content"
                            aria-label={`Open resource ${change.resourceId} in Infrastructure`}
                          >
                            {change.resourceId}
                          </a>
                        ) : (
                          <span class="font-mono">{change.resourceId}</span>
                        )}
                        <span class="mx-1.5">·</span>
                        {formatRelativeTime(change.observedAt, {
                          compact: true,
                          emptyText: 'just now',
                        })}
                      </p>
                      <Show when={change.actor}>
                        <p class={`mt-1 ${itemText()} text-muted`}>By {change.actor}</p>
                      </Show>
                      <Show when={change.reason}>
                        <p class={`mt-1 ${itemText()} text-muted`}>{change.reason}</p>
                      </Show>
                    </div>

                    <div class="flex flex-wrap items-center gap-1.5">
                      <span
                        class={`rounded-full border border-border-subtle bg-surface px-2 py-0.5 ${badgeText()} font-medium text-muted`}
                      >
                        {formatResourceChangeKind(change.kind)}
                      </span>
                      <span
                        class={`rounded-full border border-border-subtle bg-surface px-2 py-0.5 ${badgeText()} font-medium text-muted`}
                      >
                        {change.sourceType}
                      </span>
                      <Show when={change.sourceAdapter}>
                        <span
                          class={`rounded-full border border-border-subtle bg-surface px-2 py-0.5 ${badgeText()} font-medium text-muted`}
                        >
                          {change.sourceAdapter}
                        </span>
                      </Show>
                    </div>
                  </div>

                  <Show when={(change.relatedResources ?? []).length > 0}>
                    <div class={`mt-2 flex flex-wrap items-center gap-1.5 ${itemText()} text-muted`}>
                      <span>Related:</span>
                      <For each={relatedResources}>{renderRelatedResource}</For>
                    </div>
                  </Show>
                </li>
              );
            }}
          </For>
        </ul>
      </Show>
    </section>
  );
};

export default ResourceChangeSummary;
