// Actual canonical resource API -> useWorkloads -> row/disclosure/CSS.
// Only synthetic observations are supplied; this sends no guest commands.
import { createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import {
  GUEST_COLUMNS,
  getGuestColumnWidthStyle,
  getWorkloadTableLayoutModeForContainer,
  getWorkloadVisibleColumnsForLayout,
} from '../src/components/Workloads/guestRowModel';
import { DarkModeContext } from '../src/contexts/appRuntime';
import type { WorkloadMetricHistoryReader } from '../src/components/Workloads/workloadMetricHistoryModel';
import '../src/index.css';

function Fixture() {
  const state = useWorkloads(() => true);
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const [expanded, setExpanded] = createSignal(false);
  const [actions, setActions] = createSignal(0);
  const primary = () => state.workloads().find((guest) => guest.vmid === 101)!;
  const control = () => state.workloads().find((guest) => guest.vmid === 102)!;
  const layout = getWorkloadTableLayoutModeForContainer(window.innerWidth - 24);
  const requested = new Set(['name', 'cpu', 'memory', 'disk', 'uptime']);
  const columns = getWorkloadVisibleColumnsForLayout(
    GUEST_COLUMNS.filter((column) => requested.has(column.id)),
    layout,
  );
  const columnIds = columns.map((column) => column.id);
  const history: WorkloadMetricHistoryReader = {
    hasData: () => true,
    getGuestMetricSeries: () => [
      {
        label: 'Recorded',
        color: '#64748b',
        points: [
          { timestamp: Date.parse('2026-10-12T20:00:00Z'), value: 25 },
          { timestamp: Date.parse('2026-10-12T20:01:00Z'), value: 55 },
        ],
      },
    ],
    getNodeMetricSeries: () => [],
  };
  (window as any).__readingStatusAccess = {
    refetch: state.refetch,
    mode: setMode,
    actions,
    evidence: () => ({ id: primary().id, memory: primary().memory, disk: primary().disk }),
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Workload reading access</h1>
      <p class="text-xs text-muted">Synthetic resource observations; no guest commands.</p>
      <Show when={primary() && control()}>
        <div class="table-scroll-shell overflow-x-auto">
          <table class="w-full platform-table workload-table table-fixed">
            <colgroup>
              <For each={columnIds}>
                {(id) => (
                  <col
                    style={getGuestColumnWidthStyle(
                      id,
                      window.innerWidth <= 768,
                      layout,
                      columnIds,
                    )}
                  />
                )}
              </For>
            </colgroup>
            <thead>
              <tr>
                <For each={columns}>{(column) => <th scope="col">{column.label}</th>}</For>
              </tr>
            </thead>
            <tbody>
              <GuestRow
                guest={primary()}
                visibleColumnIds={columnIds}
                workloadTableLayoutMode={layout}
                metricDisplayMode={mode()}
                metricHistory={history}
                isExpanded={expanded()}
                onClick={() => {
                  setActions((value) => value + 1);
                  setExpanded((value) => !value);
                }}
              />
              <GuestRow
                guest={control()}
                visibleColumnIds={columnIds}
                workloadTableLayoutMode={layout}
                metricDisplayMode={mode()}
                metricHistory={history}
              />
            </tbody>
          </table>
        </div>
        <Show when={expanded()}>
          <section aria-label="Guest details">
            <GuestDrawer guest={primary()} onClose={() => setExpanded(false)} />
          </section>
        </Show>
      </Show>
    </main>
  );
}
render(
  () => (
    <Router>
      <Route
        path="*"
        component={() => (
          <DarkModeContext.Provider
            value={() => document.documentElement.classList.contains('dark')}
          >
            <Fixture />
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
