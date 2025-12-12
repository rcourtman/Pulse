import { Component, Show, createMemo, createSignal, onMount, For } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { AIAPI } from '@/api/ai';
import { formatNumber } from '@/utils/format';
import { logger } from '@/utils/logger';
import { notificationStore } from '@/stores/notifications';
import type { AICostSummary } from '@/types/ai';
import { PROVIDER_NAMES } from '@/types/ai';

const usdFormatter = new Intl.NumberFormat(undefined, {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export const AICostDashboard: Component = () => {
  const [days, setDays] = createSignal(30);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [summary, setSummary] = createSignal<AICostSummary | null>(null);
  let requestSeq = 0;

  const anyPricingKnown = createMemo(() => {
    const data = summary();
    if (!data) return false;
    return data.provider_models.some((pm) => pm.pricing_known);
  });

  const estimatedTotalUSD = createMemo(() => {
    const data = summary();
    if (!data || !anyPricingKnown()) return null;
    return data.totals.estimated_usd ?? 0;
  });

  const formatUSD = (usd: number) => usdFormatter.format(usd);

  const loadSummary = async (rangeDays: number) => {
    const seq = ++requestSeq;
    setLoading(true);
    setLoadError(null);
    try {
      const data = await AIAPI.getCostSummary(rangeDays);
      if (seq !== requestSeq) return;
      setSummary(data);
    } catch (err) {
      if (seq !== requestSeq) return;
      logger.error('[AICostDashboard] Failed to load cost summary:', err);
      notificationStore.error('Failed to load AI cost summary');
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

  const handleRangeClick = (rangeDays: number) => {
    if (loading() || rangeDays === days()) return;
    setDays(rangeDays);
    loadSummary(rangeDays);
  };

  return (
    <Card padding="none" class="overflow-hidden border border-gray-200 dark:border-gray-700" border={false}>
      <div class="bg-gradient-to-r from-emerald-50 to-teal-50 dark:from-emerald-900/20 dark:to-teal-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-emerald-100 dark:bg-emerald-900/40 rounded-lg">
            <svg class="w-5 h-5 text-emerald-600 dark:text-emerald-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 1.343-3 3v1a3 3 0 006 0v-1c0-1.657-1.343-3-3-3zM5 12a7 7 0 0114 0v3a2 2 0 01-2 2H7a2 2 0 01-2-2v-3z" />
            </svg>
          </div>
          <SectionHeader
            title="AI Cost & Usage"
            description="Token usage and estimated spend across providers"
            size="sm"
            class="flex-1"
          />
          <Show when={loading()}>
            <div class="text-xs text-gray-500 dark:text-gray-400">Loading…</div>
          </Show>
          <div class="flex items-center gap-1">
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(1)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 1
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              1d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(7)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 7
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              7d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(30)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 30
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              30d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(90)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 90
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              90d
            </button>
            <button
              type="button"
              disabled={loading()}
              onClick={() => handleRangeClick(365)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 365
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                } ${loading() ? 'opacity-60 cursor-not-allowed' : ''}`}
            >
              1y
            </button>
          </div>
        </div>
      </div>

      <div class="p-6 space-y-4">
        <Show when={!summary() && loading()}>
          <div class="text-sm text-gray-500 dark:text-gray-400">Loading usage…</div>
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
          <div class="text-sm text-gray-500 dark:text-gray-400">{loadError()}</div>
        </Show>

        <Show when={!summary() && !loading() && !loadError()}>
          <div class="text-sm text-gray-500 dark:text-gray-400">No usage data yet.</div>
        </Show>

        <Show when={summary()}>
          {(data) => (
            <>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">Estimated spend</div>
                  <div class="text-lg font-semibold text-gray-900 dark:text-white">
                    <Show
                      when={estimatedTotalUSD() != null}
                      fallback={<span class="text-gray-500 dark:text-gray-400">—</span>}
                    >
                      {formatUSD(estimatedTotalUSD() ?? 0)}
                    </Show>
                  </div>
                </div>
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">Total tokens</div>
                  <div class="text-lg font-semibold text-gray-900 dark:text-white">
                    {formatNumber(data().totals.total_tokens)}
                  </div>
                </div>
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">Model/provider pairs</div>
                  <div class="text-lg font-semibold text-gray-900 dark:text-white">
                    {formatNumber(data().provider_models.length)}
                  </div>
                </div>
              </div>

              <div class="text-xs text-gray-500 dark:text-gray-400">
                USD is an estimate based on public list prices. It may differ from billing.
              </div>

              <div class="overflow-x-auto">
                <table class="min-w-full text-sm">
                  <thead class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    <tr class="border-b border-gray-200 dark:border-gray-700">
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
                        <tr class="border-b border-gray-100 dark:border-gray-800">
                          <td class="py-2 pr-4 font-medium text-gray-900 dark:text-gray-100">
                            {PROVIDER_NAMES[pm.provider as keyof typeof PROVIDER_NAMES] || pm.provider}
                          </td>
                          <td class="py-2 pr-4 text-gray-700 dark:text-gray-300 font-mono text-xs">
                            {pm.model}
                          </td>
                          <td class="py-2 px-2 text-right text-gray-900 dark:text-gray-100">
                            <Show
                              when={pm.pricing_known}
                              fallback={<span class="text-gray-500 dark:text-gray-500">—</span>}
                            >
                              {formatUSD(pm.estimated_usd ?? 0)}
                            </Show>
                          </td>
                          <td class="py-2 px-2 text-right text-gray-700 dark:text-gray-300">
                            {formatNumber(pm.input_tokens)}
                          </td>
                          <td class="py-2 px-2 text-right text-gray-700 dark:text-gray-300">
                            {formatNumber(pm.output_tokens)}
                          </td>
                          <td class="py-2 px-2 text-right text-gray-900 dark:text-gray-100">
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
