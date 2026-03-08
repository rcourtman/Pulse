import { Component, createMemo, For, Show } from 'solid-js';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import type { InteractiveSparklineSeries } from '@/components/shared/InteractiveSparkline';
import type { MetricPoint } from '@/api/charts';
import type {
  ProtectionRollup,
  RecoveryOutcome,
  RecoveryPointsSeriesBucket,
} from '@/types/recovery';
import {
  getRecoveryOutcomeBarClass,
  getRecoveryOutcomeLabel,
  getRecoveryOutcomeTextClass,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type RecoverySummaryTimeRange = '7d' | '30d' | '90d';

const RECOVERY_TIME_RANGES: readonly string[] = ['7d', '30d', '90d'];
const RECOVERY_TIME_RANGE_LABELS: Record<string, string> = {
  '7d': '7d',
  '30d': '30d',
  '90d': '90d',
};

export interface RecoverySummaryProps {
  rollups: () => ProtectionRollup[];
  series: () => RecoveryPointsSeriesBucket[];
  seriesLoaded: () => boolean;
  seriesFailed?: () => boolean;
  summary: () => {
    total: number;
    counts: Record<RecoveryOutcome, number>;
    stale: number;
    neverSucceeded: number;
  };
  timeRange: () => RecoverySummaryTimeRange;
  onTimeRangeChange?: (range: RecoverySummaryTimeRange) => void;
}

// ---------------------------------------------------------------------------
// Freshness buckets
// ---------------------------------------------------------------------------

interface FreshnessBuckets {
  under1h: number;
  under24h: number;
  under7d: number;
  over7d: number;
}

const FRESHNESS_LABELS: { key: keyof FreshnessBuckets; label: string; color: string }[] = [
  { key: 'under1h', label: '<1h', color: 'bg-emerald-500' },
  { key: 'under24h', label: '<24h', color: 'bg-emerald-400' },
  { key: 'under7d', label: '<7d', color: 'bg-amber-400' },
  { key: 'over7d', label: '>7d', color: 'bg-red-500' },
];

// ---------------------------------------------------------------------------
// Issues by provider
// ---------------------------------------------------------------------------

interface ProviderIssue {
  provider: string;
  warning: number;
  failed: number;
  total: number;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export const RecoverySummary: Component<RecoverySummaryProps> = (props) => {
  const summary = () => props.summary();
  const hasRollups = () => summary().total > 0;

  // Card 1: Protection coverage — outcome distribution
  const outcomeSegments = createMemo(() => {
    const s = summary();
    if (s.total <= 0) return [];
    return ['success', 'running', 'warning', 'failed', 'unknown']
      .map((outcome) => ({
        outcome: outcome as RecoveryOutcome,
        label: getRecoveryOutcomeLabel(outcome),
        count: s.counts[outcome as RecoveryOutcome],
        color: getRecoveryOutcomeBarClass(outcome),
        textColor: getRecoveryOutcomeTextClass(outcome),
      }))
      .filter((seg) => seg.count > 0);
  });

  // Card 2: Backup freshness
  const freshnessBuckets = createMemo((): FreshnessBuckets => {
    const now = Date.now();
    const buckets: FreshnessBuckets = { under1h: 0, under24h: 0, under7d: 0, over7d: 0 };
    for (const r of props.rollups()) {
      const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
      if (!successMs || !Number.isFinite(successMs)) {
        buckets.over7d++;
        continue;
      }
      const ageMs = now - successMs;
      if (ageMs < 3_600_000) buckets.under1h++;
      else if (ageMs < 86_400_000) buckets.under24h++;
      else if (ageMs < 604_800_000) buckets.under7d++;
      else buckets.over7d++;
    }
    return buckets;
  });

  const maxFreshnessCount = createMemo(() => {
    const b = freshnessBuckets();
    return Math.max(b.under1h, b.under24h, b.under7d, b.over7d, 1);
  });

  // Card 3: Daily activity sparkline
  const activitySeries = createMemo((): InteractiveSparklineSeries[] => {
    const buckets = props.series();
    if (buckets.length < 2) return [];
    const data: MetricPoint[] = buckets.map((b) => ({
      timestamp: new Date(b.day + 'T00:00:00').getTime(),
      value: b.total,
    }));
    return [{ id: 'total', data, color: '#3b82f6', name: 'Backups' }];
  });

  const hasActivityData = () => activitySeries().length > 0 && activitySeries()[0].data.length >= 2;

  // Card 4: Issues by provider
  const issuesByProvider = createMemo((): ProviderIssue[] => {
    const map = new Map<string, { warning: number; failed: number }>();
    for (const r of props.rollups()) {
      const outcome = normalizeRecoveryOutcome(r.lastOutcome);
      if (outcome !== 'warning' && outcome !== 'failed') continue;
      const providers = (r.providers || []).filter(Boolean);
      if (providers.length === 0) providers.push('unknown');
      for (const provider of providers) {
        const entry = map.get(provider) || { warning: 0, failed: 0 };
        entry[outcome]++;
        map.set(provider, entry);
      }
    }
    return Array.from(map.entries())
      .map(([provider, counts]) => ({
        provider,
        ...counts,
        total: counts.warning + counts.failed,
      }))
      .sort((a, b) => b.total - a.total);
  });

  const maxProviderIssues = createMemo(() => {
    const items = issuesByProvider();
    if (items.length === 0) return 1;
    return Math.max(...items.map((i) => i.total), 1);
  });

  const totalIssues = createMemo(() => summary().counts.warning + summary().counts.failed);

  return (
    <Show when={hasRollups()}>
      <SummaryPanel
        testId="recovery-summary"
        headerLeft={
          <>
            <span class="font-medium text-base-content">{summary().total} protected</span>
            <Show when={summary().counts.success > 0}>
              <span class="text-emerald-600 dark:text-emerald-400">
                {summary().counts.success} healthy
              </span>
            </Show>
            <Show when={totalIssues() > 0}>
              <span class="text-amber-600 dark:text-amber-400">
                {totalIssues()} {totalIssues() === 1 ? 'issue' : 'issues'}
              </span>
            </Show>
          </>
        }
        timeRange={props.timeRange()}
        timeRanges={RECOVERY_TIME_RANGES}
        timeRangeLabels={RECOVERY_TIME_RANGE_LABELS}
        onTimeRangeChange={
          props.onTimeRangeChange
            ? (r) => props.onTimeRangeChange!(r as RecoverySummaryTimeRange)
            : undefined
        }
      >
        {/* Card 1: Protection Coverage */}
        <SummaryMetricCard label="Protection" loaded={true} hasData={hasRollups()}>
          <div class="flex flex-col justify-center h-full gap-2">
            {/* Stacked bar */}
            <div class="h-4 rounded-full overflow-hidden bg-surface-hover flex">
              <For each={outcomeSegments()}>
                {(seg) => (
                  <div
                    class={`h-full transition-all duration-500 ${seg.color}`}
                    style={{ width: `${(seg.count / summary().total) * 100}%` }}
                  />
                )}
              </For>
            </div>
            {/* Legend */}
            <div class="flex flex-wrap gap-x-3 gap-y-0.5">
              <For each={outcomeSegments()}>
                {(seg) => (
                  <div class="flex items-center gap-1 text-[10px]">
                    <div class={`w-2 h-2 rounded-full ${seg.color}`} />
                    <span class={seg.textColor}>
                      {seg.count} {seg.label}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </div>
        </SummaryMetricCard>

        {/* Card 2: Backup Freshness */}
        <SummaryMetricCard label="Freshness" loaded={true} hasData={hasRollups()}>
          <div class="flex items-end justify-between gap-1.5 h-full pt-1 pb-0.5">
            <For each={FRESHNESS_LABELS}>
              {(bucket) => {
                const count = () => freshnessBuckets()[bucket.key];
                const heightPct = () => Math.max((count() / maxFreshnessCount()) * 100, 4);
                return (
                  <div class="flex-1 flex flex-col items-center gap-0.5 min-w-0">
                    <span class="text-[10px] font-medium text-base-content tabular-nums">
                      {count()}
                    </span>
                    <div class="w-full flex items-end" style={{ height: '48px' }}>
                      <div
                        class={`w-full rounded-sm transition-all duration-300 ${bucket.color}`}
                        style={{ height: `${heightPct()}%` }}
                      />
                    </div>
                    <span class="text-[9px] text-muted">{bucket.label}</span>
                  </div>
                );
              }}
            </For>
          </div>
        </SummaryMetricCard>

        {/* Card 3: Daily Activity */}
        <SummaryMetricCard
          label="Daily Activity"
          loaded={props.seriesLoaded()}
          hasData={hasActivityData()}
          emptyMessage={props.seriesFailed?.() ? 'Trend data unavailable' : 'No backup events yet'}
        >
          <InteractiveSparkline
            series={activitySeries()}
            rangeLabel={props.timeRange()}
            yMode="auto"
            formatValue={(v) => `${Math.round(v)}`}
            formatTopLabel={(max) => `${Math.round(max)}`}
          />
        </SummaryMetricCard>

        {/* Card 4: Issues by Provider */}
        <SummaryMetricCard
          label="Issues"
          loaded={true}
          hasData={totalIssues() > 0}
          emptyMessage="No issues"
        >
          <div class="flex flex-col justify-center h-full gap-1.5 overflow-hidden">
            <For each={issuesByProvider().slice(0, 4)}>
              {(item) => (
                <div class="flex items-center gap-2 min-w-0">
                  <span class="text-[10px] text-muted shrink-0 w-16 truncate text-right">
                    {item.provider}
                  </span>
                  <div class="flex-1 h-3 rounded-sm overflow-hidden bg-surface-hover flex">
                    <Show when={item.failed > 0}>
                      <div
                        class="h-full bg-red-500 transition-all duration-300"
                        style={{ width: `${(item.failed / maxProviderIssues()) * 100}%` }}
                      />
                    </Show>
                    <Show when={item.warning > 0}>
                      <div
                        class="h-full bg-amber-400 transition-all duration-300"
                        style={{ width: `${(item.warning / maxProviderIssues()) * 100}%` }}
                      />
                    </Show>
                  </div>
                  <span class="text-[10px] font-medium text-base-content tabular-nums w-4 text-right shrink-0">
                    {item.total}
                  </span>
                </div>
              )}
            </For>
            <Show when={issuesByProvider().length > 4}>
              <div class="text-[9px] text-muted text-center">
                +{issuesByProvider().length - 4} more
              </div>
            </Show>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
