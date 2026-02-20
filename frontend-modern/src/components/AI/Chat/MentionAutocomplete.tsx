import { createSignal, createEffect, For, Show, onCleanup } from 'solid-js';
import type { MentionResource } from './mentionResources';

interface MentionAutocompleteProps {
  query: string;
  resources: MentionResource[];
  position: { top: number; left: number };
  onSelect: (resource: MentionResource) => void;
  onClose: () => void;
  visible: boolean;
}

export function MentionAutocomplete(props: MentionAutocompleteProps) {
  const [selectedIndex, setSelectedIndex] = createSignal(0);

  // Filter resources based on query
  const filteredResources = () => {
    const q = props.query.toLowerCase();
    if (!q) return props.resources.slice(0, 10); // Show first 10 if no query

    return props.resources
      .filter(r => r.name.toLowerCase().includes(q))
      .slice(0, 10); // Limit to 10 results
  };

  // Reset selection when query changes
  createEffect(() => {
    props.query; // Track query
    setSelectedIndex(0);
  });

  // Handle keyboard navigation
  const handleKeyDown = (e: KeyboardEvent) => {
    if (!props.visible) return;

    const resources = filteredResources();

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex(i => Math.min(i + 1, resources.length - 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex(i => Math.max(i - 1, 0));
        break;
      case 'Enter':
      case 'Tab':
        e.preventDefault();
        if (resources[selectedIndex()]) {
          props.onSelect(resources[selectedIndex()]);
        }
        break;
      case 'Escape':
        e.preventDefault();
        props.onClose();
        break;
    }
  };

  // Register keyboard listener when visible
  createEffect(() => {
    if (props.visible) {
      document.addEventListener('keydown', handleKeyDown);
      onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
    }
  });

  // Get icon for resource type
  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'vm':
        return (
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
          </svg>
        );
      case 'container':
        return (
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
          </svg>
        );
      case 'docker':
        return (
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
            <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.185.185 0 00-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.185.185 0 00-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.185.185 0 00-.185.185v1.887c0 .102.082.185.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.185.185 0 00-.185.185v1.887c0 .102.083.185.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.185.185 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.185.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 00.185-.185V9.006a.185.185 0 00-.185-.186h-2.119a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.185v1.888c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.376 11.376 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z" />
          </svg>
        );
      case 'node':
        return (
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
          </svg>
        );
      case 'host':
        return (
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
          </svg>
        );
      default:
        return (
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
          </svg>
        );
    }
  };

  // Get status color
  const getStatusColor = (status?: string) => {
    switch (status?.toLowerCase()) {
      case 'running':
      case 'online':
      case 'healthy':
      case 'up':
        return 'bg-green-500';
      case 'stopped':
      case 'offline':
      case 'error':
      case 'failed':
        return 'bg-red-500';
      case 'paused':
      case 'degraded':
      case 'warning':
        return 'bg-yellow-500';
      default:
        return 'bg-gray-400';
    }
  };

  return (
    <Show when={props.visible && filteredResources().length > 0}>
      <div
        class="absolute z-50 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg shadow-lg overflow-hidden min-w-[280px] max-w-[400px]"
        style={{
          bottom: `${props.position.top}px`,
          left: `${props.position.left}px`,
        }}
      >
        <div class="px-3 py-2 border-b border-slate-200 dark:border-slate-700 text-xs font-medium text-slate-500 dark:text-slate-400">
          Resources
        </div>
        <div class="max-h-[240px] overflow-y-auto">
          <For each={filteredResources()}>
            {(resource, index) => (
              <button
                type="button"
                class={`w-full px-3 py-2 flex items-center gap-3 text-left hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors ${
                  index() === selectedIndex() ? 'bg-slate-100 dark:bg-slate-700' : ''
                }`}
                onClick={() => props.onSelect(resource)}
                onMouseEnter={() => setSelectedIndex(index())}
              >
                <span class="text-slate-500 dark:text-slate-400">
                  {getTypeIcon(resource.type)}
                </span>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-slate-900 dark:text-slate-100 truncate">
                      {resource.name}
                    </span>
                    <Show when={resource.status}>
                      <span class={`w-2 h-2 rounded-full ${getStatusColor(resource.status)}`} />
                    </Show>
                  </div>
                  <div class="text-xs text-slate-500 dark:text-slate-400">
                    {resource.type}
                    <Show when={resource.node}>
                      {' · '}{resource.node}
                    </Show>
                  </div>
                </div>
              </button>
            )}
          </For>
        </div>
        <div class="px-3 py-1.5 border-t border-slate-200 dark:border-slate-700 text-xs text-slate-400 dark:text-slate-500 flex items-center gap-2">
          <span class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 rounded text-[10px]">↑↓</span>
          navigate
          <span class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 rounded text-[10px]">↵</span>
          select
          <span class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 rounded text-[10px]">esc</span>
          close
        </div>
      </div>
    </Show>
  );
}
