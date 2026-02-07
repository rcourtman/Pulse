import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { formatBytes, formatPercent } from '@/utils/format';
import { buildStorageRecordsV2 } from '@/features/storageBackupsV2/storageAdapters';
import { PLATFORM_BLUEPRINTS } from '@/features/storageBackupsV2/platformBlueprint';
import type { NormalizedHealth, StorageRecordV2 } from '@/features/storageBackupsV2/models';

const HEALTH_CLASS: Record<NormalizedHealth, string> = {
  healthy: 'text-green-700 dark:text-green-300',
  warning: 'text-yellow-700 dark:text-yellow-300',
  critical: 'text-red-700 dark:text-red-300',
  offline: 'text-gray-600 dark:text-gray-300',
  unknown: 'text-gray-500 dark:text-gray-400',
};

const StorageV2: Component = () => {
  const { state } = useWebSocket();
  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal('all');
  const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');

  const records = createMemo<StorageRecordV2[]>(() =>
    buildStorageRecordsV2({ state, resources: state.resources || [] }),
  );

  const sourceOptions = createMemo(() => {
    const values = Array.from(new Set(records().map((record) => record.source.platform)));
    return ['all', ...values];
  });

  const filtered = createMemo(() => {
    const query = search().trim().toLowerCase();
    return records()
      .filter((record) => (sourceFilter() === 'all' ? true : record.source.platform === sourceFilter()))
      .filter((record) => (healthFilter() === 'all' ? true : record.health === healthFilter()))
      .filter((record) => {
        if (!query) return true;
        const haystack = [
          record.name,
          record.category,
          record.location.label,
          record.source.platform,
          ...(record.capabilities || []),
        ]
          .filter(Boolean)
          .join(' ')
          .toLowerCase();
        return haystack.includes(query);
      });
  });

  const summary = createMemo(() => {
    const list = filtered();
    const totals = list.reduce(
      (acc, record) => {
        const total = record.capacity.totalBytes || 0;
        const used = record.capacity.usedBytes || 0;
        acc.total += total;
        acc.used += used;
        acc.byHealth[record.health] = (acc.byHealth[record.health] || 0) + 1;
        return acc;
      },
      {
        total: 0,
        used: 0,
        byHealth: {
          healthy: 0,
          warning: 0,
          critical: 0,
          offline: 0,
          unknown: 0,
        } as Record<NormalizedHealth, number>,
      },
    );
    const usagePercent = totals.total > 0 ? (totals.used / totals.total) * 100 : 0;
    return {
      count: list.length,
      totalBytes: totals.total,
      usedBytes: totals.used,
      usagePercent,
      byHealth: totals.byHealth,
    };
  });

  const nextPlatforms = createMemo(() =>
    PLATFORM_BLUEPRINTS.filter((platform) => platform.stage === 'next').map((platform) => platform.label),
  );

  return (
    <div class="space-y-4">
      <Card padding="md" tone="glass">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Storage V2 Preview</h2>
            <p class="text-xs text-gray-600 dark:text-gray-400">
              Source-agnostic storage view model with capability-first normalization.
            </p>
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            Next platforms: {nextPlatforms().join(', ')}
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="grid gap-3 sm:grid-cols-4">
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Records</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">{summary().count}</div>
          </div>
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Total Capacity</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {formatBytes(summary().totalBytes)}
            </div>
          </div>
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Used</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {formatBytes(summary().usedBytes)}
            </div>
          </div>
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Usage</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {formatPercent(summary().usagePercent)}
            </div>
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="grid gap-2 md:grid-cols-3">
          <input
            type="text"
            value={search()}
            onInput={(event) => setSearch(event.currentTarget.value)}
            placeholder="Search name, location, source, capability..."
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
          />
          <select
            value={sourceFilter()}
            onChange={(event) => setSourceFilter(event.currentTarget.value)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
          >
            <For each={sourceOptions()}>{(option) => <option value={option}>{option}</option>}</For>
          </select>
          <select
            value={healthFilter()}
            onChange={(event) => setHealthFilter(event.currentTarget.value as 'all' | NormalizedHealth)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
          >
            <option value="all">all health states</option>
            <option value="healthy">healthy</option>
            <option value="warning">warning</option>
            <option value="critical">critical</option>
            <option value="offline">offline</option>
            <option value="unknown">unknown</option>
          </select>
        </div>
      </Card>

      <Card padding="none" class="overflow-hidden">
        <Show
          when={filtered().length > 0}
          fallback={
            <div class="p-6 text-sm text-gray-600 dark:text-gray-300">
              No storage records match the current filters.
            </div>
          }
        >
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-400">
                  <th class="px-3 py-2">Name</th>
                  <th class="px-3 py-2">Category</th>
                  <th class="px-3 py-2">Location</th>
                  <th class="px-3 py-2">Platform</th>
                  <th class="px-3 py-2">Capacity</th>
                  <th class="px-3 py-2">Health</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={filtered()}>
                  {(record) => (
                    <tr>
                      <td class="px-3 py-2 text-gray-900 dark:text-gray-100">{record.name}</td>
                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{record.category}</td>
                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{record.location.label}</td>
                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{record.source.platform}</td>
                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                        <Show
                          when={record.capacity.totalBytes && record.capacity.totalBytes > 0}
                          fallback="n/a"
                        >
                          {formatBytes(record.capacity.usedBytes || 0)} /{' '}
                          {formatBytes(record.capacity.totalBytes || 0)}
                        </Show>
                      </td>
                      <td class={`px-3 py-2 font-medium ${HEALTH_CLASS[record.health]}`}>{record.health}</td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </Show>
      </Card>
    </div>
  );
};

export default StorageV2;

