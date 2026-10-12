// Production guest row, drawer, alert presentation and CSS; synthetic observations only.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import {
  getGuestColumnStyle,
  getWorkloadTableLayoutModeForContainer,
} from '../src/components/Workloads/guestRowModel';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Alert } from '../src/types/api';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
const capacity = 4 * 1024 ** 3;
const alert: Alert = {
  id: 'fixture-memory-alert',
  resourceId: 'fixture:pve:101',
  resourceName: 'memory-guest',
  node: 'pve',
  instance: 'fixture',
  type: 'memory',
  level: 'warning',
  message: 'VM memory at 86.6%',
  value: 86.6,
  threshold: 85,
  startTime: new Date(Date.now() - 3600000).toISOString(),
  acknowledged: false,
};
const initial: WorkloadGuest = {
  id: 'fixture:pve:101',
  vmid: 101,
  name: 'memory-guest',
  instance: 'fixture',
  node: 'pve',
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
    observation: {
      state: 'last-known',
      source: 'guest-agent-meminfo',
      observedAt: new Date(Date.now() - 1800000).toISOString(),
    },
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
  lastSeen: new Date().toISOString(),
};

function Fixture() {
  const [guest, setGuest] = createSignal(initial);
  const [alerts, setAlerts] = createSignal([alert]);
  const [basis, setBasis] = createSignal<'guest' | 'host'>('host');
  const [hostTotal, setHostTotal] = createSignal(capacity * 2);
  const [expanded, setExpanded] = createSignal(false);
  const columns = ['name', 'cpu', 'memory', 'disk', 'uptime'];
  const layout = getWorkloadTableLayoutModeForContainer(Math.min(window.innerWidth, 896) - 40);
  Object.assign(window, {
    __guestAlertMemory: {
      apply: (next: Partial<WorkloadGuest>, nextHostTotal?: number) => {
        setGuest((previous) => ({ ...previous, ...next }));
        if (nextHostTotal !== undefined) setHostTotal(nextHostTotal);
      },
      alerts: setAlerts,
      guest,
      alert,
    },
  });
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest alert memory basis</h1>
      <p class="text-xs text-muted">Synthetic observations. No guest or recovery operation.</p>
      <label class="flex gap-2 items-center text-xs">
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
      <section aria-label="Production guest table">
        <table class="w-full table-fixed" style="border-collapse: separate; border-spacing: 0 4px">
          <colgroup>
            {columns.map((column) => (
              <col style={getGuestColumnStyle(column, window.innerWidth < 768, layout, columns)} />
            ))}
          </colgroup>
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={columns}
              memoryDisplayBasis={basis()}
              parentMemoryTotal={hostTotal()}
              parentNodeName="pve"
              workloadTableLayoutMode={layout}
              isExpanded={expanded()}
              onClick={() => setExpanded(!expanded())}
            />
          </tbody>
        </table>
      </section>
      <Show when={expanded()}>
        <section aria-label="Guest details">
          <GuestDrawer
            guest={guest()}
            memoryDisplayBasis={basis()}
            parentMemoryTotal={hostTotal()}
            alerts={alerts()}
            onClose={() => setExpanded(false)}
          />
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
