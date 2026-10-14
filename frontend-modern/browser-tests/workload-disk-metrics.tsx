// Synthetic REST observations -> production mapper, selection, row and drawer.
// The fixture controls only selection/display, never a guest or backup operation.
import { createMemo, createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import {
  filterWorkloads,
  createWorkloadSortComparator,
} from '../src/components/Workloads/workloadSelectors';
import { getGuestDrawerCurrentMetrics } from '../src/components/Workloads/guestDrawerModel';
import { DarkModeContext } from '../src/contexts/appRuntime';
import '../src/index.css';

function Fixture() {
  const state = useWorkloads(() => true);
  const [search, setSearch] = createSignal('');
  const [direction, setDirection] = createSignal<'asc' | 'desc'>('desc');
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const [selected, setSelected] = createSignal<string | null>(null);
  const phone = matchMedia('(max-width: 639px)').matches;
  const ordered = createMemo(() =>
    [
      ...filterWorkloads({
        guests: state.workloads(),
        searchTerm: search(),
        viewMode: 'all',
        statusMode: 'all',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      }),
    ].sort(createWorkloadSortComparator('disk', direction())!),
  );
  const guest = () => state.workloads().find((row) => row.id === selected());
  (window as any).__diskMetrics = {
    refresh: state.refetch,
    evidence: () =>
      state
        .workloads()
        .map((row) => ({
          id: row.id,
          disk: row.disk,
          available: row.telemetryAvailability?.disk,
          current: getGuestDrawerCurrentMetrics(row).disk,
        })),
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Workload disk readings</h1>
      <p class="text-xs text-muted">
        Synthetic observations; fixture selection controls. No guest operations.
      </p>
      <div class="flex flex-wrap gap-2 text-sm">
        <label>
          Search{' '}
          <input
            aria-label="Search fixtures"
            value={search()}
            onInput={(e) => setSearch(e.currentTarget.value)}
            class="w-32 rounded-sm border border-border bg-surface px-2 py-2"
          />
        </label>
        <label>
          Disk order{' '}
          <select
            aria-label="Disk order"
            value={direction()}
            onChange={(e) => setDirection(e.currentTarget.value as 'asc' | 'desc')}
            class="rounded-sm border border-border bg-surface px-2 py-2"
          >
            <option value="asc">Ascending</option>
            <option value="desc">Descending</option>
          </select>
        </label>
        <label>
          Metrics{' '}
          <select
            aria-label="Metrics"
            value={mode()}
            onChange={(e) => setMode(e.currentTarget.value as 'bars' | 'sparklines')}
            class="rounded-sm border border-border bg-surface px-2 py-2"
          >
            <option value="bars">Bars</option>
            <option value="sparklines">Sparklines</option>
          </select>
        </label>
      </div>
      <section aria-label="Workload readings" class="rounded-sm border border-border">
        <table class="workload-table platform-table w-full table-fixed">
          <thead>
            <tr>
              <th class="p-2 text-left">Name</th>
              <th class="p-2 text-center">Disk</th>
            </tr>
          </thead>
          <tbody>
            <For each={ordered()}>
              {(row) => (
                <GuestRow
                  guest={row}
                  visibleColumnIds={['name', 'disk']}
                  metricDisplayMode={mode()}
                  workloadTableLayoutMode={phone ? 'phone' : 'wide'}
                  onClick={() => setSelected(row.id)}
                />
              )}
            </For>
          </tbody>
        </table>
      </section>
      <Show when={guest()}>
        {(_row) => (
          <section aria-label="Selected guest" class="rounded-sm border border-border p-3">
            <GuestDrawer guest={guest()!} onClose={() => setSelected(null)} />
          </section>
        )}
      </Show>
    </main>
  );
}

render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
