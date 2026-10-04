// Synthetic observations through the actual GuestDrawer, its state model and
// production CSS. This fixture neither starts a backup nor establishes thaw.
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
  const [observation, setObservation] = createSignal({
    lastBackup: 0,
    running: false,
    reason: '',
  });
  const [closeCount, setCloseCount] = createSignal(0);
  const type = new URLSearchParams(window.location.search).get('kind') === 'lxc' ? 'lxc' : 'qemu';
  const guest = (): WorkloadGuest => ({
    id: 'fixture-pve:pve-a:101',
    vmid: 101,
    name: 'backup-guest',
    node: 'pve-a',
    instance: 'fixture-pve',
    status: 'running',
    type,
    cpu: 0.1,
    cpus: 2,
    memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
    disk: { total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
    disks: [
      { mountpoint: '/data', type: 'ext4', total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
    ],
    diskStatusReason: observation().reason,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: observation().lastBackup,
    backupInProgress: observation().running,
    tags: [],
    lock: observation().running ? 'backup' : '',
    lastSeen: '2026-10-03T12:00:00Z',
  });
  (window as any).__backupProtection = {
    update: setObservation,
    closeCount,
    identity: () => guest().id,
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest backup protection</h1>
      <p class="text-xs text-muted">Synthetic observations; no backup command is sent.</p>
      <section aria-label="Guest details" class="mx-auto max-w-4xl">
        <GuestDrawer guest={guest()} onClose={() => setCloseCount((count) => count + 1)} />
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
