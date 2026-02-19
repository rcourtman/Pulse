import { Component, Show, createMemo, createSignal, onMount, For } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { AIAPI } from '@/api/ai';
import { formatNumber } from '@/utils/format';
import { logger } from '@/utils/logger';
import { notificationStore } from '@/stores/notifications';
import type { AICostSummary, AISettings } from '@/types/ai';
import { PROVIDER_NAMES } from '@/types/ai';

const usdFormatter = new Intl.NumberFormat(undefined, {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const TinySparkline: Component<{
  values: number[];
  width?: number;
  height?: number;
  stroke?: string;
}> = (props) => {
  const width = () => props.width ?? 160;
  const height = () => props.height ?? 28;
  const stroke = () => props.stroke ?? '#22c55e';

  const pathD = createMemo(() => {
    const values = props.values;
    const w = width();
    const h = height();
    if (!values || values.length === 0) return '';

    const max = Math.max(...values, 0);
    const safeMax = max <= 0 ? 1 : max;

    // For single point, draw a horizontal line across the middle
    if (values.length === 1) {
      const y = h - (Math.max(0, values[0]) / safeMax) * h;
      // Draw a short horizontal line to make the single point visible
      return `M0,${y.toFixed(2)} L${w.toFixed(2)},${y.toFixed(2)}`;
    }

    const xStep = w / (values.length - 1);
    let d = '';
    values.forEach((v, idx) => {
      const x = idx * xStep;
      const y = h - (Math.max(0, v) / safeMax) * h;
      d += `${idx === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)} `;
    });
    return d.trim();
  });

  return (
    <svg width={width()} height={height()} viewBox={`0 0 ${width()} ${height()}`} class="block">
      <path d={pathD()} fill="none" stroke={stroke()} stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  );
};

export const AICostDashboard: Component = () => {
  const [days, setDays] = createSignal(30);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [summary, setSummary] = createSignal<AICostSummary | null>(null);
  const [aiSettings, setAISettings] = createSignal<AISettings | null>(null);
  let requestSeq = 0;

  const anyPricingKnown = createMemo(() => {
    const data = summary();
    if (!data) return false;
    return data.provider_models.some((pm) => pm.pricing_known);
  });

  const unpricedProviderModels = createMemo(() => {
    const data = summary();
    if (!data) return [];
    return (data.provider_models ?? []).filter((pm) => !pm.pricing_known && (pm.total_tokens ?? 0) > 0);
  });

  const estimatedTotalUSD = createMemo(() => {
    const data = summary();
    if (!data || !anyPricingKnown()) return null;
    return data.totals.estimated_usd ?? 0;
  });

  const useCaseMap = createMemo(() => {
    const data = summary();
    const map = new Map<string, { tokens: number; usd: number; pricingKnown: boolean }>();
    if (!data) return map;
    for (const uc of data.use_cases ?? []) {
      map.set(uc.use_case, {
        tokens: uc.total_tokens,
        usd: uc.estimated_usd ?? 0,
        pricingKnown: uc.pricing_known,
      });
    }
    return map;
  });

  const dailyTotals = createMemo(() => summary()?.daily_totals ?? []);
  const dailyTokenValues = createMemo(() => dailyTotals().map((d) => d.total_tokens));
  const dailyUSDValues = createMemo(() => dailyTotals().map((d) => d.estimated_usd ?? 0));

  const lastDailyTokens = createMemo(() => {
    const values = dailyTokenValues();
    if (values.length === 0) return null;
    return values[values.length - 1];
  });

  const lastDailyUSD = createMemo(() => {
    const values = dailyUSDValues();
    if (values.length === 0) return null;
    return values[values.length - 1];
  });

  const formatUSD = (usd: number) => usdFormatter.format(usd);

  const loadSummary = async (rangeDays: number) => {
    const seq = ++requestSeq;
    const isInitialLoad = summary() === null;
    setLoading(true);
    setLoadError(null);
    try {
      const data = await AIAPI.getCostSummary(rangeDays);
      if (seq !== requestSeq) return;
      setSummary(data);
    } catch (err) {
      if (seq !== requestSeq) return;
      logger.error('[AICostDashboard] Failed to load cost summary:', err);
      // Only show notification on refresh failures, not initial load
      if (!isInitialLoad) {
        notificationStore.error('Failed to refresh Pulse Assistant usage summary');
      }
      const message =
        err instanceof Error && err.message ? err.message : 'Failed to load usage data';
      setLoadError(message);
    } finally {
      if (seq === requestSeq) setLoading(false);
    }
  };

  onMount(() => {
    loadSummary(days());
  });

  const loadBudgetSettings = async () => {
    try {
      const s = await AIAPI.getSettings();
      setAISettings(s);
    } catch (err) {
      logger.debug('[AICostDashboard] Failed to load AI settings for budget:', err);
      setAISettings(null);
    }
  };

  onMount(() => {
    loadBudgetSettings();
  });

  const parsedBudgetUSD30d = createMemo(() => {
    const s = aiSettings();
    const n = s?.cost_budget_usd_30d;
    if (typeof n !== 'number' || !Number.isFinite(n) || n <= 0) return null;
    return n;
  });

  const budgetForRange = createMemo(() => {
    const budget30d = parsedBudgetUSD30d();
    if (budget30d == null) return null;
    const rangeDays = days();
    return (budget30d * rangeDays) / 30;
  });

  const isOverBudget = createMemo(() => {
    const budget = budgetForRange();
    const usd = estimatedTotalUSD();
    if (budget == null || usd == null) return false;
    return usd > budget;
  });

  const resetHistory = async () => {
    if (!confirm('Reset Pulse Assistant usage history? A backup will be created in the Pulse config directory.')) return;
    try {
      const result = await AIAPI.resetCostHistory();
      if (result.backup_file) {
        notificationStore.success(`Pulse Assistant usage history reset (backup: ${result.backup_file})`);
      } else {
        notificationStore.success('Pulse Assistant usage history reset');
      }
      await loadSummary(days());
    } catch (err) {
      logger.error('[AICostDashboard] Failed to reset AI cost history:', err);
      notificationStore.error('Failed to reset Pulse Assistant usage history');
    }
  };

  const downloadExport = async (format: 'csv' | 'json') => {
    try {
      const resp = await AIAPI.exportCostHistory(days(), format);
      if (!resp.ok) {
        throw new Error(`Export failed (${resp.status})`);
      }
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `pulse-ai-usage-${new Date().toISOString().split('T')[0]}-${days()}d.${format}`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      logger.error('[AICostDashboard] Failed to export cost history:', err);
      notificationStore.error('Failed to export Pulse Assistant usage history');
    }
  };

  const handleRangeClick = (rangeDays: number) => {
    if (loading() || rangeDays === days()) return;
    setDays(rangeDays);
    loadSummary(rangeDays);
  };

  return (
    <Card padding="none" class="overflow-hidden border border-slate-200 dark:border-slate-700" border={false}>
      <div class="bg-blue-50 dark:bg-blue-900/20 px-6 py-4 border-b border-slate-200 dark:border-slate-700">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-md">
            <svg class="w-5 h-5 text-blue-600 dark:text-blue-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 1.343-3 3v1a3 3 0 006 0v-1c0-1.657-1.343-3-3-3zM5 12a7 7 0 0114 0v3a2 2 0 01-2 2H7a2 2 0 01-2-2v-3z" />
            </svg>
          </div>
          <SectionHeader
            title="Pulse Cost & Usage"
            description="Token usage and estimated spend across providers"
            size="sm"
            class="flex-1"
          />
          <Show when={loading()}>
            <div class="text-xs text-slate-500 dark:text-slate-400">Loading…</div>
          </Show>
          <div class="flex items-center gap-1">
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(1)}
              class={`min-h-10 sm:min-h-9 min-w-10 px-2.5 py-2 text-sm border rounded transition-colors ${days() === 1
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-slate-300 dark:border-slate-600 hover:bg-slate-100 dark:hover:bg-slate-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              1d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(7)}
              class={`min-h-10 sm:min-h-9 min-w-10 px-2.5 py-2 text-sm border rounded transition-colors ${days() === 7
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-slate-300 dark:border-slate-600 hover:bg-slate-100 dark:hover:bg-slate-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              7d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(30)}
              class={`min-h-10 sm:min-h-9 min-w-10 px-2.5 py-2 text-sm border rounded transition-colors ${days() === 30
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-slate-300 dark:border-slate-600 hover:bg-slate-100 dark:hover:bg-slate-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              30d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(90)}
              class={`min-h-10 sm:min-h-9 min-w-10 px-2.5 py-2 text-sm border rounded transition-colors ${days() === 90
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-slate-300 dark:border-slate-600 hover:bg-slate-100 dark:hover:bg-slate-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              90d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(365)}
              class={`min-h-10 sm:min-h-9 min-w-10 px-2.5 py-2 text-sm border rounded transition-colors ${days() === 365
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-slate-300 dark:border-slate-600 hover:bg-slate-100 dark:hover:bg-slate-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              1y
            </button>
          </div>
        </div>
      </div>

      <div class="p-6 space-y-4">
        <Show when={!summary() && loading()}>
          <div class="text-sm text-slate-500 dark:text-slate-400">Loading usage…</div>
        </Show>

        <Show when={summary()?.truncated}>
          <div class="text-xs px-3 py-2 rounded border border-blue-200 dark:border-blue-800/60 bg-blue-50 dark:bg-blue-900/20 text-blue-900 dark:text-blue-100">
            Showing the last {summary()?.effective_days} days due to a {summary()?.retention_days}-day retention window.
          </div>
        </Show>

        <Show when={isOverBudget()}>
          <div class="text-xs px-3 py-2 rounded border border-red-200 dark:border-red-800/60 bg-red-50 dark:bg-red-900/20 text-red-900 dark:text-red-100">
            Estimated spend ({formatUSD(estimatedTotalUSD() ?? 0)}) is above your budget ({formatUSD(budgetForRange() ?? 0)}).
          </div>
        </Show>

        <Show when={loadError() && summary()}>
          <div class="flex items-center justify-between gap-3 text-xs px-3 py-2 rounded border border-amber-200 dark:border-amber-800/60 bg-amber-50 dark:bg-amber-900/20 text-amber-900 dark:text-amber-100">
            <div class="truncate">
              Couldn’t refresh. Showing last loaded data. {loadError()}
            </div>
            <button
              type="button"
              disabled={loading()}
              onClick={() => loadSummary(days())}
              class={`shrink-0 px-2 py-1 rounded border border-amber-300 dark:border-amber-700 hover:bg-amber-100 dark:hover:bg-amber-900/40 ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              Retry
            </button>
          </div>
        </Show>

        <Show when={!summary() && !loading() && loadError()}>
          <div class="text-sm text-slate-500 dark:text-slate-400">{loadError()}</div>
        </Show>

        <Show when={!summary() && !loading() && !loadError()}>
          <div class="text-sm text-slate-500 dark:text-slate-400">No usage data yet.</div>
        </Show>

        <Show when={summary()}>
          {(data) => (
            <>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="text-xs text-slate-500 dark:text-slate-400">Estimated spend</div>
                  <div class="text-lg font-semibold text-slate-900 dark:text-white">
                    <Show
                      when={estimatedTotalUSD() != null}
                      fallback={<span class="text-slate-500 dark:text-slate-400">—</span>}
                    >
                      {formatUSD(estimatedTotalUSD() ?? 0)}
                    </Show>
                  </div>
                </div>
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="text-xs text-slate-500 dark:text-slate-400">Total tokens</div>
                  <div class="text-lg font-semibold text-slate-900 dark:text-white">
                    {formatNumber(data().totals.total_tokens)}
                  </div>
                </div>
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="text-xs text-slate-500 dark:text-slate-400">Model/provider pairs</div>
                  <div class="text-lg font-semibold text-slate-900 dark:text-white">
                    {formatNumber(data().provider_models.length)}
                  </div>
                </div>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="text-xs text-slate-500 dark:text-slate-400">Chat</div>
                  <div class="text-sm font-semibold text-slate-900 dark:text-white">
                    {formatNumber(useCaseMap().get('chat')?.tokens ?? 0)} tokens
                  </div>
                  <div class="text-xs text-slate-500 dark:text-slate-400">
                    <Show
                      when={useCaseMap().get('chat')?.pricingKnown}
                      fallback={<span>—</span>}
                    >
                      {formatUSD(useCaseMap().get('chat')?.usd ?? 0)}
                    </Show>
                  </div>
                </div>
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="text-xs text-slate-500 dark:text-slate-400">Patrol</div>
                  <div class="text-sm font-semibold text-slate-900 dark:text-white">
                    {formatNumber(useCaseMap().get('patrol')?.tokens ?? 0)} tokens
                  </div>
                  <div class="text-xs text-slate-500 dark:text-slate-400">
                    <Show
                      when={useCaseMap().get('patrol')?.pricingKnown}
                      fallback={<span>—</span>}
                    >
                      {formatUSD(useCaseMap().get('patrol')?.usd ?? 0)}
                    </Show>
                  </div>
                </div>
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="text-xs text-slate-500 dark:text-slate-400">Budget alert (USD per 30d)</div>
                  <div class="text-sm font-semibold text-slate-900 dark:text-white mt-1">
                    <Show when={parsedBudgetUSD30d() != null} fallback={<span class="text-slate-500 dark:text-slate-400">—</span>}>
                      {formatUSD(parsedBudgetUSD30d() ?? 0)}
                    </Show>
                  </div>
                  <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                    Set in Pulse Assistant settings. Pro-rated for {days()}d:{' '}
                    <Show when={budgetForRange() != null} fallback={<span>—</span>}>
                      {formatUSD(budgetForRange() ?? 0)}
                    </Show>
                  </div>
                </div>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="flex items-center justify-between">
                    <div class="text-xs text-slate-500 dark:text-slate-400">Daily estimated USD</div>
                    <div class="text-xs text-slate-500 dark:text-slate-400">
                      <Show when={anyPricingKnown() && lastDailyUSD() != null} fallback={<span>—</span>}>
                        {formatUSD(lastDailyUSD() ?? 0)}
                      </Show>
                    </div>
                  </div>
                  <div class="mt-2">
                    <Show
                      when={anyPricingKnown() && dailyUSDValues().length >= 2}
                      fallback={<div class="text-xs text-slate-500 dark:text-slate-400">No daily USD trend yet.</div>}
                    >
                      <TinySparkline values={dailyUSDValues()} stroke="#10b981" />
                    </Show>
                  </div>
                </div>
                <div class="p-3 rounded-md bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                  <div class="flex items-center justify-between">
                    <div class="text-xs text-slate-500 dark:text-slate-400">Daily total tokens</div>
                    <div class="text-xs text-slate-500 dark:text-slate-400">
                      <Show when={lastDailyTokens() != null} fallback={<span>—</span>}>
                        {formatNumber(lastDailyTokens() ?? 0)}
                      </Show>
                    </div>
                  </div>
                  <div class="mt-2">
                    <Show
                      when={dailyTokenValues().length >= 2}
                      fallback={<div class="text-xs text-slate-500 dark:text-slate-400">No daily token trend yet.</div>}
                    >
                      <TinySparkline values={dailyTokenValues()} stroke="#3b82f6" />
                    </Show>
                  </div>
                </div>
              </div>

              <div class="text-xs text-slate-500 dark:text-slate-400">
                USD is an estimate based on public list prices. It may differ from billing.
                <Show when={unpricedProviderModels().length > 0}>
                  <span class="ml-2">
                    Estimated spend is partial. Pricing is unknown for{' '}
                    {unpricedProviderModels()
                      .slice(0, 6)
                      .map(
                        (pm) =>
                          `${PROVIDER_NAMES[pm.provider as keyof typeof PROVIDER_NAMES] || pm.provider}/${pm.model}`,
                      )
                      .join(', ')}
                    <Show when={unpricedProviderModels().length > 6}>
                      <span> (+{unpricedProviderModels().length - 6} more)</span>
                    </Show>
                    .
                  </span>
                </Show>
                <Show when={summary()?.pricing_as_of}>
                  <span class="ml-2">Prices as of {summary()?.pricing_as_of}.</span>
                </Show>
              </div>

              <div class="flex items-center justify-between gap-3">
                <div class="text-xs text-slate-500 dark:text-slate-400">
                  History retention: {data().retention_days} days
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    disabled={loading()}
                    onClick={() => downloadExport('csv')}
                    class={`min-h-10 sm:min-h-9 px-2.5 py-2 text-sm rounded border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
                  >
                    Export CSV
                  </button>
                  <button
                    type="button"
                    disabled={loading()}
                    onClick={() => downloadExport('json')}
                    class={`min-h-10 sm:min-h-9 px-2.5 py-2 text-sm rounded border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
                  >
                    Export JSON
                  </button>
                  <button
                    type="button"
                    disabled={loading()}
                    onClick={resetHistory}
                    class={`min-h-10 sm:min-h-9 px-2.5 py-2 text-sm rounded border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
                  >
                    Reset history
                  </button>
                </div>
              </div>

              <Show when={(data().targets?.length ?? 0) > 0}>
                <div class="overflow-x-auto">
                  <table class="min-w-full text-sm">
                    <thead class="text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                      <tr class="border-b border-slate-200 dark:border-slate-700">
                        <th class="text-left py-2 pr-4">Top targets</th>
                        <th class="text-right py-2 px-2">Est. USD</th>
                        <th class="text-right py-2 px-2">Calls</th>
                        <th class="text-right py-2 px-2">Tokens</th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={data().targets}>
                        {(t) => (
                          <tr class="border-b border-slate-100 dark:border-slate-800">
                            <td class="py-2 pr-4 text-slate-700 dark:text-slate-300 font-mono text-xs">
                              {t.target_type}:{t.target_id}
                            </td>
                            <td class="py-2 px-2 text-right text-slate-900 dark:text-slate-100">
                              <Show
                                when={t.pricing_known}
                                fallback={<span class="text-slate-500 dark:text-slate-500">—</span>}
                              >
                                {formatUSD(t.estimated_usd ?? 0)}
                              </Show>
                            </td>
                            <td class="py-2 px-2 text-right text-slate-700 dark:text-slate-300">
                              {formatNumber(t.calls)}
                            </td>
                            <td class="py-2 px-2 text-right text-slate-900 dark:text-slate-100">
                              {formatNumber(t.total_tokens)}
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>

              <div class="overflow-x-auto">
                <table class="min-w-full text-sm">
                  <thead class="text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                    <tr class="border-b border-slate-200 dark:border-slate-700">
                      <th class="text-left py-2 pr-4">Provider</th>
                      <th class="text-left py-2 pr-4">Model</th>
                      <th class="text-right py-2 px-2">Est. USD</th>
                      <th class="text-right py-2 px-2">Input</th>
                      <th class="text-right py-2 px-2">Output</th>
                      <th class="text-right py-2 px-2">Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={data().provider_models}>
                      {(pm) => (
                        <tr class="border-b border-slate-100 dark:border-slate-800">
                          <td class="py-2 pr-4 font-medium text-slate-900 dark:text-slate-100">
                            {PROVIDER_NAMES[pm.provider as keyof typeof PROVIDER_NAMES] || pm.provider}
                          </td>
                          <td class="py-2 pr-4 text-slate-700 dark:text-slate-300 font-mono text-xs">
                            {pm.model}
                          </td>
                          <td class="py-2 px-2 text-right text-slate-900 dark:text-slate-100">
                            <Show
                              when={pm.pricing_known}
                              fallback={<span class="text-slate-500 dark:text-slate-500">—</span>}
                            >
                              {formatUSD(pm.estimated_usd ?? 0)}
                            </Show>
                          </td>
                          <td class="py-2 px-2 text-right text-slate-700 dark:text-slate-300">
                            {formatNumber(pm.input_tokens)}
                          </td>
                          <td class="py-2 px-2 text-right text-slate-700 dark:text-slate-300">
                            {formatNumber(pm.output_tokens)}
                          </td>
                          <td class="py-2 px-2 text-right text-slate-900 dark:text-slate-100">
                            {formatNumber(pm.total_tokens)}
                          </td>
                        </tr>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </>
          )}
        </Show>
      </div>
    </Card>
  );
};

export default AICostDashboard;
