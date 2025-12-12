import { Component, Show, createSignal, onMount, For } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { AIAPI } from '@/api/ai';
import { formatNumber } from '@/utils/format';
import { logger } from '@/utils/logger';
import { notificationStore } from '@/stores/notifications';
import type { AICostSummary } from '@/types/ai';
import { PROVIDER_NAMES } from '@/types/ai';

export const AICostDashboard: Component = () => {
  const [days, setDays] = createSignal(30);
  const [loading, setLoading] = createSignal(false);
  const [summary, setSummary] = createSignal<AICostSummary | null>(null);

  const loadSummary = async (rangeDays: number) => {
    setLoading(true);
    try {
      const data = await AIAPI.getCostSummary(rangeDays);
      setSummary(data);
    } catch (err) {
      logger.error('[AICostDashboard] Failed to load cost summary:', err);
      notificationStore.error('Failed to load AI cost summary');
      setSummary(null);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadSummary(days());
  });

  const handleRangeClick = (rangeDays: number) => {
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
          <div class="flex items-center gap-1">
            <button
              type="button"
              onClick={() => handleRangeClick(7)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 7
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
            >
              7d
            </button>
            <button
              type="button"
              onClick={() => handleRangeClick(30)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 30
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
            >
              30d
            </button>
            <button
              type="button"
              onClick={() => handleRangeClick(90)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 90
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
            >
              90d
            </button>
            <button
              type="button"
              onClick={() => handleRangeClick(365)}
              class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${days() === 365
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
            >
              1y
            </button>
          </div>
        </div>
      </div>

      <div class="p-6 space-y-4">
        <Show when={loading()}>
          <div class="text-sm text-gray-500 dark:text-gray-400">Loading usage…</div>
        </Show>

        <Show when={!loading() && summary() == null}>
          <div class="text-sm text-gray-500 dark:text-gray-400">No usage data yet.</div>
        </Show>

        <Show when={!loading() && summary()}>
          {(data) => (
            <>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">Input tokens</div>
                  <div class="text-lg font-semibold text-gray-900 dark:text-white">
                    {formatNumber(data().totals.input_tokens)}
                  </div>
                </div>
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">Output tokens</div>
                  <div class="text-lg font-semibold text-gray-900 dark:text-white">
                    {formatNumber(data().totals.output_tokens)}
                  </div>
                </div>
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">Total tokens</div>
                  <div class="text-lg font-semibold text-gray-900 dark:text-white">
                    {formatNumber(data().totals.total_tokens)}
                  </div>
                </div>
              </div>

              <div class="overflow-x-auto">
                <table class="min-w-full text-sm">
                  <thead class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    <tr class="border-b border-gray-200 dark:border-gray-700">
                      <th class="text-left py-2 pr-4">Provider</th>
                      <th class="text-left py-2 pr-4">Model</th>
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
