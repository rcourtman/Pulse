// Full production drawer/CSS and client; observations and API responses are
// synthetic. No backup, guest-agent or native recovery operation is performed.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { WorkloadGuest } from '../src/types/workloads';
import { guestDiskDeferrals } from '../src/components/Workloads/__fixtures__/guestDiskDeferrals';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });

function Fixture() {
  const [observation, setObservation] = createSignal({
    reason: 'prev-vm-locked',
    usage: 50,
    lock: 'backup',
    kind: 'qemu',
  });
  const guest = createMemo<WorkloadGuest>(() => ({
    id: 'fixture-pve:pve-a:101',
    vmid: 101,
    name: 'backup-guest',
    node: 'pve-a',
    instance: 'fixture-pve',
    status: 'running',
    type: observation().kind,
    cpu: 0.1,
    cpus: 2,
    memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
    disk: {
      total: 10 * 1024 ** 3,
      used: (observation().usage / 100) * 10 * 1024 ** 3,
      usage: observation().usage,
    },
    diskStatusReason: observation().reason,
    lock: observation().lock,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    backupInProgress: Boolean(observation().lock),
    tags: [],
    lastSeen: '2026-10-04T03:00:00Z',
  }));
  (window as unknown as Record<string, unknown>).__guestHistoryProvenance = {
    observe: (next: Partial<ReturnType<typeof observation>>) =>
      setObservation((prev) => ({ ...prev, ...next })),
    deferrals: guestDiskDeferrals,
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest History filesystem provenance</h1>
      <p class="text-xs text-muted">
        Synthetic observations. No backup or guest-agent command is sent.
      </p>
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
