// Production rows/drawer and metric History, synthetic selected observations.
// No guest request, thaw, backup, restart or recovery operation is performed.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import {
  getGuestColumnStyle,
  getWorkloadTableLayoutModeForContainer,
} from '../src/components/Workloads/guestRowModel';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { WorkloadGuest } from '../src/types/workloads';
import type { WorkloadMetricHistoryReader } from '../src/components/Workloads/workloadMetricHistoryModel';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
const capacity = 4 * 1024 ** 3;
const observedAt = '2026-09-30T11:00:00Z';
const initial: WorkloadGuest = {
  id: 'fixture-pve1-101',
  vmid: 101,
  name: 'backup-guest',
  instance: 'fixture',
  node: 'pve1',
  type: 'qemu',
  workloadType: 'vm',
  platformScopes: ['proxmox-pve'],
  platformType: 'proxmox-pve',
  status: 'running',
  cpu: 0.1,
  cpus: 2,
  memory: {
    total: capacity,
    used: capacity / 4,
    free: (capacity * 3) / 4,
    usage: 25,
    observation: { state: 'last-known', source: 'guest-agent-meminfo', observedAt },
  },
  disk: { total: capacity, used: capacity / 2, usage: 50 },
  diskStatusReason: 'prev-vm-locked',
  lock: 'backup',
  guestAgentStatus: 'deferred',
  guestAgentExpected: true,
  networkIn: 10,
  networkOut: 20,
  diskRead: 30,
  diskWrite: 40,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lastSeen: '2026-10-01T12:00:00Z',
};
const history: WorkloadMetricHistoryReader = {
  getGuestMetricSeries: (_guest, metric) =>
    metric === 'memory'
      ? [
          {
            id: 'memory',
            label: 'Memory',
            color: '#f59e0b',
            points: [
              { timestamp: Date.parse(observedAt) - 60_000, value: 20 },
              { timestamp: Date.parse(observedAt), value: 25 },
            ],
          },
        ]
      : [],
  getNodeMetricSeries: () => [],
};
function Fixture() {
  const columns = ['name', 'cpu', 'memory', 'disk', 'uptime'];
  const layout = getWorkloadTableLayoutModeForContainer(Math.min(window.innerWidth, 896) - 40);
  const [guest, setGuest] = createSignal(initial);
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const [basis, setBasis] = createSignal<'guest' | 'host'>('guest');
  const [expanded, setExpanded] = createSignal(false);
  (window as any).__memoryRow = {
    update: (next: Partial<WorkloadGuest>) => setGuest({ ...guest(), ...next }),
    reading: () => guest(),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Workloads memory freshness</h1>
      <p class="text-xs text-muted">
        Synthetic observations. No guest checks or recovery commands.
      </p>
      <div class="flex flex-wrap gap-3 text-xs">
        <label class="flex gap-2 items-center">
          Display
          <select
            aria-label="Metric display"
            class="rounded-sm border border-border bg-surface p-1"
            value={mode()}
            onChange={(e) => setMode(e.currentTarget.value as 'bars' | 'sparklines')}
          >
            <option value="bars">Bars</option>
            <option value="sparklines">Sparklines</option>
          </select>
        </label>
        <label class="flex gap-2 items-center">
          Memory basis
          <select
            aria-label="Memory basis"
            class="rounded-sm border border-border bg-surface p-1"
            value={basis()}
            onChange={(e) => setBasis(e.currentTarget.value as 'guest' | 'host')}
          >
            <option value="guest">Guest allocation</option>
            <option value="host">Host capacity</option>
          </select>
        </label>
      </div>
      <section aria-label="Guest row" class="rounded-sm border border-border bg-surface-raised p-2">
        <table class="w-full table-fixed">
          <thead>
            <tr>
              {columns.map((column) => (
                <th
                  class="text-left text-xs"
                  style={getGuestColumnStyle(column, window.innerWidth <= 768, layout, columns)}
                >
                  {column === 'cpu' ? 'CPU' : column[0].toUpperCase() + column.slice(1)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={columns}
              metricDisplayMode={mode()}
              memoryDisplayBasis={basis()}
              parentMemoryTotal={capacity * 2}
              parentNodeName="pve1"
              workloadTableLayoutMode={layout}
              metricHistory={history}
              isExpanded={expanded()}
              onClick={() => setExpanded(!expanded())}
            />
          </tbody>
        </table>
      </section>
      <Show when={expanded()}>
        <section aria-label="Guest details">
          <GuestDrawer guest={guest()} onClose={() => setExpanded(false)} />
        </section>
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
