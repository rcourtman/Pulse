import { For, Show, createMemo, type Component } from 'solid-js';
import type { ResourceCorrelation } from '@/types/aiIntelligence';
import { formatRelativeTime } from '@/utils/format';
import {
  formatResourceCorrelationEndpoint,
  formatResourceCorrelationHeadline,
  formatResourceCorrelationPattern,
  formatResourceCorrelationSummary,
} from '@/utils/resourceCorrelationPresentation';

interface ResourceGraphSummaryProps {
  correlations?: ResourceCorrelation[] | null;
  dependencies?: string[] | null;
  dependents?: string[] | null;
  title?: string;
  summaryText?: string;
  buildResourceHref: (resourceId: string) => string | null | undefined;
  showLastSeen?: boolean;
  class?: string;
  maxCorrelations?: number;
}

const formatPluralCount = (count: number, singular: string, plural: string): string =>
  `${count} ${count === 1 ? singular : plural}`;

const formatSummaryParts = (parts: Array<string | null | undefined>): string =>
  parts.filter((part): part is string => Boolean(part && part.trim())).join(' · ');

const sortResourceCorrelations = (correlations: ResourceCorrelation[]): ResourceCorrelation[] =>
  [...correlations].sort((left, right) => {
    const confidenceDiff = (right.confidence || 0) - (left.confidence || 0);
    if (confidenceDiff !== 0) return confidenceDiff;
    const leftTime = Date.parse(left.last_seen || '');
    const rightTime = Date.parse(right.last_seen || '');
    return (Number.isFinite(rightTime) ? rightTime : 0) - (Number.isFinite(leftTime) ? leftTime : 0);
  });

export const ResourceGraphSummary: Component<ResourceGraphSummaryProps> = (props) => {
  const className = () => props.class?.trim() ?? '';
  const correlations = createMemo(() => sortResourceCorrelations(props.correlations ?? []));
  const dependencies = () => props.dependencies ?? [];
  const dependents = () => props.dependents ?? [];
  const maxCorrelations = () => props.maxCorrelations ?? 3;
  const hasContent = () =>
    dependencies().length > 0 || dependents().length > 0 || correlations().length > 0;

  const summaryText = () =>
    props.summaryText?.trim() ||
    formatSummaryParts([
      dependencies().length > 0
        ? formatPluralCount(dependencies().length, 'dependency', 'dependencies')
        : null,
      dependents().length > 0
        ? formatPluralCount(dependents().length, 'dependent', 'dependents')
        : null,
      correlations().length > 0
        ? formatPluralCount(correlations().length, 'correlation', 'correlations')
        : null,
    ]);

  return (
    <Show when={hasContent()}>
      <div class={`rounded-md border border-border-subtle bg-base p-4 ${className()}`.trim()}>
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div>
            <h3 class="text-sm font-semibold text-base-content">{props.title ?? 'Graph context'}</h3>
            <Show when={summaryText()}>
              {(summary) => <p class="mt-1 text-xs text-muted">{summary()}</p>}
            </Show>
          </div>
        </div>

        <Show when={dependencies().length > 0}>
          <div class="mt-3">
            <div class="text-[9px] uppercase tracking-wide text-muted">Depends on</div>
            <div class="mt-1 flex flex-wrap gap-1">
              <For each={dependencies().slice(0, maxCorrelations())}>
                {(dependency) => {
                  const href = props.buildResourceHref(dependency);
                  return href ? (
                    <a
                      class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px] text-blue-700 hover:underline dark:text-blue-300"
                      href={href}
                      aria-label={`Open dependency resource ${dependency} in Infrastructure`}
                    >
                      {dependency}
                    </a>
                  ) : (
                    <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                      {dependency}
                    </span>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>

        <Show when={dependents().length > 0}>
          <div class="mt-3">
            <div class="text-[9px] uppercase tracking-wide text-muted">Used by</div>
            <div class="mt-1 flex flex-wrap gap-1">
              <For each={dependents().slice(0, maxCorrelations())}>
                {(dependent) => {
                  const href = props.buildResourceHref(dependent);
                  return href ? (
                    <a
                      class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px] text-blue-700 hover:underline dark:text-blue-300"
                      href={href}
                      aria-label={`Open dependent resource ${dependent} in Infrastructure`}
                    >
                      {dependent}
                    </a>
                  ) : (
                    <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                      {dependent}
                    </span>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>

        <Show when={correlations().length > 0}>
          <div class="mt-3">
            <div class="text-[9px] uppercase tracking-wide text-muted">Correlations</div>
            <div class="mt-1 space-y-1.5">
              <For each={correlations().slice(0, maxCorrelations())}>
                {(correlation) => {
                  const sourceHref = props.buildResourceHref(correlation.source_id);
                  const targetHref = props.buildResourceHref(correlation.target_id);
                  const headline = formatResourceCorrelationHeadline(correlation);
                  const summary = formatResourceCorrelationSummary(correlation);
                  const sourceLabel = formatResourceCorrelationEndpoint(correlation, 'source');
                  const targetLabel = formatResourceCorrelationEndpoint(correlation, 'target');
                  const patternLabel = formatResourceCorrelationPattern(correlation);
                  const lastSeenLabel = props.showLastSeen && correlation.last_seen
                    ? formatRelativeTime(correlation.last_seen, {
                        compact: true,
                        emptyText: 'just now',
                      })
                    : '';
                  return (
                    <div class="rounded bg-surface px-2 py-1" title={headline}>
                      <div class="flex flex-wrap items-center gap-1 text-[10px] text-base-content">
                        {sourceHref ? (
                          <a
                            class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px] text-blue-700 hover:underline dark:text-blue-300"
                            href={sourceHref}
                            aria-label={`Open source resource ${sourceLabel} in Infrastructure`}
                          >
                            {sourceLabel}
                          </a>
                        ) : (
                          <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                            {sourceLabel}
                          </span>
                        )}
                        <span class="text-muted">→</span>
                        {targetHref ? (
                          <a
                            class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px] text-blue-700 hover:underline dark:text-blue-300"
                            href={targetHref}
                            aria-label={`Open target resource ${targetLabel} in Infrastructure`}
                          >
                            {targetLabel}
                          </a>
                        ) : (
                          <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                            {targetLabel}
                          </span>
                        )}
                        <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[9px] uppercase tracking-wide text-muted">
                          {patternLabel}
                        </span>
                      </div>
                      <div class="mt-0.5 text-[10px] text-muted">
                        {summary}
                        <Show when={lastSeenLabel}>
                          <>{' '}· last seen {lastSeenLabel}</>
                        </Show>
                      </div>
                      <Show when={correlation.description}>
                        {(description) => (
                          <div class="mt-0.5 text-[10px] text-muted">{description()}</div>
                        )}
                      </Show>
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>
      </div>
    </Show>
  );
};

export default ResourceGraphSummary;
