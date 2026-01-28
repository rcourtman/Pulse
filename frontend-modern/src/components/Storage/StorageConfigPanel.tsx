import { Component, Show, For, createEffect, createMemo, createSignal } from 'solid-js';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { Card } from '@/components/shared/Card';
import { MonitoringAPI } from '@/api/monitoring';
import type { StorageConfigEntry } from '@/types/api';

interface StorageConfigPanelProps {
  nodeFilter?: string | null;
  searchTerm?: string;
}

export const StorageConfigPanel: Component<StorageConfigPanelProps> = (props) => {
  const [items, setItems] = createSignal<StorageConfigEntry[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [onlyNodeRelevant, setOnlyNodeRelevant] = usePersistentSignal<boolean>('storageConfigOnlyNode', false);

  const fetchConfig = async () => {
    setLoading(true);
    setError(null);
    try {
      const storages = await MonitoringAPI.getStorageConfig({
        node: props.nodeFilter ?? undefined,
      });
      setItems(storages);
    } catch (err) {
      setError((err as Error).message || 'Failed to load storage config');
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    fetchConfig();
  });

  const filtered = createMemo(() => {
    const term = (props.searchTerm || '').trim().toLowerCase();
    return items().filter((item) => {
      if (onlyNodeRelevant() && props.nodeFilter) {
        if (!item.nodes || item.nodes.length === 0) {
          return false;
        }
        const match = item.nodes.some((node) => node.toLowerCase() === props.nodeFilter!.toLowerCase());
        if (!match) {
          return false;
        }
      }
      if (!term) {
        return true;
      }
      const haystack = [
        item.id,
        item.name,
        item.instance,
        item.type,
        item.content,
        item.path,
        (item.nodes || []).join(','),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  });

  const formatNodes = (nodes?: string[]) => {
    if (!nodes || nodes.length === 0) {
      return 'all nodes';
    }
    return nodes.join(', ');
  };

  return (
    <Card padding="sm" class="mb-4">
      <div class="flex items-center justify-between mb-2">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Storage Configuration</h3>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Cluster storage.cfg entries (enabled/active, nodes, path)
          </p>
        </div>
        <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <Show when={props.nodeFilter}>
            <button
              type="button"
              onClick={() => setOnlyNodeRelevant((prev) => !prev)}
              class={`inline-flex items-center gap-1 px-2 py-0.5 rounded border text-[10px] transition-colors ${
                onlyNodeRelevant()
                  ? 'border-blue-300 text-blue-700 bg-blue-50 dark:border-blue-700 dark:text-blue-300 dark:bg-blue-900/20'
                  : 'border-gray-200 text-gray-600 dark:border-gray-700 dark:text-gray-300'
              }`}
            >
              {onlyNodeRelevant() ? 'Only selected node' : 'All nodes'}
            </button>
          </Show>
          <Show when={loading()}>
            <span class="inline-flex items-center gap-1">
              <span class="h-1.5 w-1.5 rounded-full bg-blue-400 animate-pulse" />
              Loading
            </span>
          </Show>
        </div>
      </div>

      <Show when={error()}>
        <div class="text-xs text-red-600 dark:text-red-400 mb-2">{error()}</div>
      </Show>

      <Show when={!loading() && filtered().length === 0}>
        <div class="text-xs text-gray-500 dark:text-gray-400">No storage configuration entries found.</div>
      </Show>

      <Show when={filtered().length > 0}>
        <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
          <style>{`
            .overflow-x-auto::-webkit-scrollbar { display: none; }
          `}</style>
          <table class="w-full text-xs">
            <thead>
              <tr class="text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                <th class="py-1.5 text-left font-medium uppercase tracking-wide">Storage</th>
                <th class="py-1.5 text-left font-medium uppercase tracking-wide">Instance</th>
                <th class="py-1.5 text-left font-medium uppercase tracking-wide">Nodes</th>
                <th class="py-1.5 text-left font-medium uppercase tracking-wide">Path</th>
                <th class="py-1.5 text-left font-medium uppercase tracking-wide">Flags</th>
              </tr>
            </thead>
            <tbody>
              <For each={filtered()}>
                {(item) => (
                  <tr class="border-b border-gray-100 dark:border-gray-800/70">
                    <td class="py-2 pr-3">
                      <div class="flex items-center gap-2">
                        <div class="font-medium text-gray-900 dark:text-gray-100">{item.name}</div>
                        <Show when={!item.nodes || item.nodes.length === 0}>
                          <span class="px-1.5 py-0.5 rounded text-[10px] bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                            global
                          </span>
                        </Show>
                      </div>
                      <div class="text-[10px] text-gray-500 dark:text-gray-400">{item.type || 'unknown'} / {item.content || 'any'}</div>
                    </td>
                    <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">
                      {item.instance || '-'}
                    </td>
                    <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">
                      {formatNodes(item.nodes)}
                    </td>
                    <td class="py-2 pr-3 font-mono text-[10px] text-gray-600 dark:text-gray-300">
                      {item.path || '-'}
                    </td>
                    <td class="py-2 text-gray-700 dark:text-gray-300">
                      <div class="flex flex-wrap gap-1">
                        <span class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                          item.enabled ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                        }`}>
                          {item.enabled ? 'enabled' : 'disabled'}
                        </span>
                        <span class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                          item.active ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                        }`}>
                          {item.active ? 'active' : 'inactive'}
                        </span>
                        <Show when={item.shared}>
                          <span class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300">
                            shared
                          </span>
                        </Show>
                      </div>
                    </td>
                  </tr>
                )}
              </For>
            </tbody>
          </table>
        </div>
      </Show>
    </Card>
  );
};
