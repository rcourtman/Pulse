import type { Component } from 'solid-js';
import { Show, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { DockerFilter } from './DockerFilter';
import { DockerSummaryStatsBar } from './DockerSummaryStats';
import { DockerUnifiedTable } from './DockerUnifiedTable';
import { useWebSocket } from '@/App';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';

interface DockerHostsProps {
  hosts: DockerHost[];
  activeAlerts?: Record<string, unknown> | any;
}

type StatsFilter = { type: 'host-status' | 'container-state' | 'service-health'; value: string } | null;

export const DockerHosts: Component<DockerHostsProps> = (props) => {
  const { initialDataReceived, reconnecting, connected } = useWebSocket();

  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => {
      const aName = a.displayName || a.hostname || a.id || '';
      const bName = b.displayName || b.hostname || b.id || '';
      return aName.localeCompare(bName);
    });
  });

  const isLoading = createMemo(() => {
    if (typeof initialDataReceived === 'function') {
      const hostCount = Array.isArray(props.hosts) ? props.hosts.length : 0;
      return !initialDataReceived() && hostCount === 0;
    }
    return false;
  });

  const [search, setSearch] = createSignal('');
  const debouncedSearch = useDebouncedValue(search, 250);

  const [statsFilter, setStatsFilter] = createSignal<StatsFilter>(null);

  let searchInputRef: HTMLInputElement | undefined;

  const focusSearchInput = () => {
    queueMicrotask(() => searchInputRef?.focus());
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    const target = event.target as HTMLElement;

    if (event.key === 'Escape' && statsFilter()) {
      event.preventDefault();
      setStatsFilter(null);
      return;
    }

    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
      return;
    }

    if (event.ctrlKey || event.metaKey || event.altKey) {
      return;
    }

    if (event.key.length === 1 && searchInputRef) {
      event.preventDefault();
      focusSearchInput();
      setSearch((prev) => prev + event.key);
    }
  };

  onMount(() => document.addEventListener('keydown', handleKeyDown));
  onCleanup(() => document.removeEventListener('keydown', handleKeyDown));

  const handleStatsFilterChange = (filter: StatsFilter) => {
    if (!filter) {
      setStatsFilter(null);
      return;
    }

    setStatsFilter((current) => {
      if (current && current.type === filter.type && current.value === filter.value) {
        return null;
      }
      return filter;
    });
  };

  return (
    <div class="space-y-0">
      <Show when={isLoading()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg class="h-12 w-12 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title={reconnecting() ? 'Reconnecting to Docker agents...' : 'Loading Docker data...'}
            description={
              reconnecting()
                ? 'Re-establishing metrics from the monitoring service.'
                : connected()
                  ? 'Waiting for the first Docker update.'
                  : 'Connecting to the monitoring service.'
            }
          />
        </Card>
      </Show>

      <Show when={!isLoading()}>
        <Show
          when={sortedHosts().length === 0}
          fallback={
            <>
              <DockerFilter
                search={search}
                setSearch={setSearch}
                onReset={() => {
                  setSearch('');
                  setStatsFilter(null);
                }}
                searchInputRef={(el) => {
                  searchInputRef = el;
                }}
              />

              <Card padding="lg">
                <DockerSummaryStatsBar
                  hosts={sortedHosts()}
                  onFilterChange={handleStatsFilterChange}
                  activeFilter={statsFilter()}
                />
              </Card>

              <DockerUnifiedTable
                hosts={sortedHosts()}
                searchTerm={debouncedSearch()}
                statsFilter={statsFilter()}
              />
            </>
          }
        >
          <Card padding="lg">
            <EmptyState
              icon={
                <svg class="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
              }
              title="No Docker hosts configured"
              description="Deploy the Pulse Docker agent on at least one Docker host to light up this tab. As soon as an agent reports in, container metrics appear automatically."
            />
          </Card>
        </Show>
      </Show>
    </div>
  );
};
