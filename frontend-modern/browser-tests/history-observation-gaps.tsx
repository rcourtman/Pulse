// Real production drawer/CSS. The browser supplies synthetic stored readings;
// no collector, guest-agent, backup, thaw or installation operation is run.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });

function Fixture() {
  const [recovered, setRecovered] = createSignal(false);
  const guest = (): WorkloadGuest => ({
    id: 'fixture:pve:101',
    vmid: 101,
    name: 'backup-guest',
    node: 'pve',
    instance: 'fixture',
    status: 'running',
    type: 'qemu',
    cpu: 0.1,
    cpus: 2,
    memory: {
      total: 1024 ** 3,
      used: 256 * 1024 ** 2,
      free: 768 * 1024 ** 2,
      usage: 25,
      observation: {
        state: recovered() ? 'current' : 'last-known',
        source: 'status-mem',
        observedAt: '2026-10-04T12:01:00Z',
      },
    },
    disk: { total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
    diskStatusReason: recovered() ? '' : 'prev-vm-locked',
    lock: recovered() ? '' : 'backup',
    guestAgentStatus: recovered() ? 'available' : 'deferred',
    backupInProgress: !recovered(),
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lastSeen: '2026-10-04T12:04:00Z',
  });
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Drawer History missing observations</h1>
      <p class="text-xs text-muted">
        Synthetic readings. No backup or guest-agent command is sent.
      </p>
      <button
        class="rounded-sm border border-border p-2 text-xs"
        onClick={() => setRecovered(true)}
      >
        Simulate live recovery
      </button>
      <section class="mx-auto max-w-5xl" aria-label="Guest details">
        <GuestDrawer guest={guest()} onClose={() => {}} />
      </section>
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
