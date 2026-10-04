// Production guest row and full drawer with synthetic observations.
// No collector, backup command, QGA/thaw, installed device or release proof.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { buildSummaryDisclosureControlsId } from '../src/components/shared/summaryInteractionA11y';
import type { WorkloadTableLayoutMode } from '../src/components/Workloads/guestRowModel';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });

function Fixture() {
  const [observation, setObservation] = createSignal({
    lastBackup: 0,
    running: true,
    reason: 'prev-vm-locked',
  });
  const [closeCount, setCloseCount] = createSignal(0);
  const [actionCount, setActionCount] = createSignal(0);
  const [expanded, setExpanded] = createSignal(false);
  const [enabled, setEnabled] = createSignal(true);
  const [layout, setLayout] = createSignal<WorkloadTableLayoutMode>(
    window.innerWidth <= 768 ? 'phone' : 'wide',
  );
  const toggle = () => {
    setActionCount((count) => count + 1);
    setExpanded((value) => !value);
  };
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
  (window as any).__guestRowTouch = {
    update: setObservation,
    closeCount,
    actionCount,
    expanded,
    enabled: setEnabled,
    layout: setLayout,
    identity: () => guest().id,
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest row touch verification</h1>
      <p class="text-xs text-muted">Synthetic observations; no backup command is sent.</p>
      <table class="w-full table-fixed">
        <tbody>
          <GuestRow
            guest={guest()}
            visibleColumnIds={['name', 'disk']}
            onClick={enabled() ? toggle : undefined}
            isExpanded={expanded()}
            workloadTableLayoutMode={layout()}
            customUrl="https://guest.example.invalid/"
          />
        </tbody>
      </table>
      <Show when={expanded()}>
        <section
          id={buildSummaryDisclosureControlsId(guest().id)}
          aria-label="Guest details"
          class="mx-auto max-w-4xl"
        >
          <GuestDrawer
            guest={guest()}
            onClose={() => {
              setCloseCount((count) => count + 1);
              setExpanded(false);
            }}
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
