import { Component, Show, For, createSignal, createMemo } from 'solid-js';
import type { ChatContextItem } from './types';

interface ContextPickerProps {
  isOpen: boolean;
  onClose: () => void;
  resources: ChatContextItem[];
  selectedIds: string[];
  onSelect: (resource: ChatContextItem) => void;
}

export const ContextPicker: Component<ContextPickerProps> = (props) => {
  const [search, setSearch] = createSignal('');

  const filteredResources = createMemo(() => {
    const term = search().toLowerCase();
    if (!term) return props.resources;
    return props.resources.filter(
      (r) =>
        r.name.toLowerCase().includes(term) ||
        r.type.toLowerCase().includes(term) ||
        (r.node && r.node.toLowerCase().includes(term))
    );
  });

  const isSelected = (id: string) => props.selectedIds.includes(id);

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'vm':
        return { bg: 'bg-blue-100 dark:bg-blue-900/40', text: 'text-blue-600 dark:text-blue-400', label: 'VM' };
      case 'container':
        return { bg: 'bg-green-100 dark:bg-green-900/40', text: 'text-green-600 dark:text-green-400', label: 'CT' };
      case 'node':
        return { bg: 'bg-orange-100 dark:bg-orange-900/40', text: 'text-orange-600 dark:text-orange-400', label: 'N' };
      case 'host':
        return { bg: 'bg-purple-100 dark:bg-purple-900/40', text: 'text-purple-600 dark:text-purple-400', label: 'H' };
      default:
        return { bg: 'bg-gray-100 dark:bg-gray-700', text: 'text-gray-600 dark:text-gray-400', label: '?' };
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running':
      case 'online':
        return 'bg-green-500';
      case 'stopped':
      case 'offline':
        return 'bg-gray-400';
      default:
        return 'bg-yellow-500';
    }
  };

  return (
    <Show when={props.isOpen}>
      <div class="absolute bottom-full left-0 mb-1 w-72 max-h-80 bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 overflow-hidden z-50">
        {/* Search input */}
        <div class="p-2 border-b border-gray-200 dark:border-gray-700">
          <div class="relative">
            <svg
              class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              type="text"
              value={search()}
              onInput={(e) => setSearch(e.currentTarget.value)}
              placeholder="Search VMs, containers, hosts..."
              class="w-full pl-9 pr-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-purple-500 focus:border-transparent"
              autofocus
            />
          </div>
        </div>

        {/* Resource list */}
        <div class="max-h-56 overflow-y-auto">
          <Show
            when={filteredResources().length > 0}
            fallback={
              <div class="p-6 text-center text-xs text-gray-500 dark:text-gray-400">
                No resources found
              </div>
            }
          >
            <For each={filteredResources()}>
              {(resource) => {
                const typeInfo = getTypeIcon(resource.type);
                const selected = isSelected(resource.id);

                return (
                  <button
                    type="button"
                    onClick={() => !selected && props.onSelect(resource)}
                    disabled={selected}
                    class={`w-full px-3 py-2.5 text-left flex items-center gap-2.5 text-xs transition-colors ${
                      selected
                        ? 'bg-purple-50 dark:bg-purple-900/20 text-gray-400 dark:text-gray-500 cursor-default'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-700/50 text-gray-700 dark:text-gray-300'
                    }`}
                  >
                    {/* Type icon */}
                    <span
                      class={`flex-shrink-0 w-6 h-6 rounded-lg flex items-center justify-center text-[9px] font-bold uppercase ${typeInfo.bg} ${typeInfo.text}`}
                    >
                      {typeInfo.label}
                    </span>

                    {/* Name and details */}
                    <div class="flex-1 min-w-0">
                      <div class="font-medium truncate">{resource.name}</div>
                      <Show when={resource.node}>
                        <div class="text-[10px] text-gray-400">{resource.node}</div>
                      </Show>
                    </div>

                    {/* Status indicator */}
                    <span
                      class={`flex-shrink-0 w-2 h-2 rounded-full ${getStatusColor(resource.status)}`}
                    />

                    {/* Selected checkmark */}
                    <Show when={selected}>
                      <svg class="w-4 h-4 text-purple-500" fill="currentColor" viewBox="0 0 20 20">
                        <path
                          fill-rule="evenodd"
                          d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                          clip-rule="evenodd"
                        />
                      </svg>
                    </Show>
                  </button>
                );
              }}
            </For>
          </Show>
        </div>

        {/* Close button */}
        <div class="p-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
          <button
            type="button"
            onClick={() => {
              props.onClose();
              setSearch('');
            }}
            class="w-full px-2 py-1.5 text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </Show>
  );
};
