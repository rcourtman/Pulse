// Synthetic snapshots only; both drawers, History, transport and CSS are production.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';
syncAIRuntimeSettings({ discovery_enabled: false });
function Fixture() {
  const [state, setState] = createSignal('retained');
  const [seen, setSeen] = createSignal(Date.parse('2026-10-04T13:00:00Z'));
  const value = () => (state() === 'current' || state() === 'unavailable' ? 0 : 25);
  const observation = () => ({
    state:
      state() === 'current'
        ? 'current'
        : state() === 'unavailable'
          ? 'unavailable'
          : state() === 'unknown'
            ? 'private-state-sentinel'
            : 'last-known',
    source:
      state() === 'unknown'
        ? 'private-source-sentinel'
        : state() === 'current'
          ? 'status-mem'
          : 'guest-agent-meminfo',
    observedAt: state() === 'unknown' ? '2999-01-01T00:00:00Z' : '2026-10-04T12:00:00Z',
  });
  const reason = () =>
    state() === 'current' ? '' : state() === 'unavailable' ? 'agent-busy' : 'prev-vm-locked';
  const resource = createMemo<Resource>(() => ({
    id: 'fixture-vm',
    type: 'vm',
    name: 'backup-guest',
    displayName: 'Backup guest',
    platformId: 'fixture',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: seen(),
    metricsTarget: { resourceType: 'vm', resourceId: 'fixture:pve:101' },
    cpu: { current: 10 },
    memory: { current: value(), total: 100, used: value(), observation: observation() },
    disk: { current: 50 },
    network: { rxBytes: 0, txBytes: 200 },
    proxmox: {
      vmid: 101,
      node: 'pve',
      instance: 'fixture',
      diskStatusReason: reason(),
      backupInProgress: Boolean(reason()),
    },
  }));
  const guest = createMemo<WorkloadGuest>(() => ({
    id: 'fixture:pve:101',
    vmid: 101,
    node: 'pve',
    instance: 'fixture',
    name: 'Backup guest',
    type: 'qemu',
    status: 'running',
    cpu: 0.1,
    cpus: 2,
    memory: {
      total: 100,
      used: value(),
      free: 100 - value(),
      usage: value(),
      observation: observation(),
    },
    disk: { total: 100, used: 50, usage: 50 },
    diskStatusReason: reason(),
    networkIn: 0,
    networkOut: 200,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lastSeen: new Date(seen()).toISOString(),
    lock: reason() ? 'backup' : '',
  }));
  return (
    <main class="min-h-screen bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Resource History provenance</h1>
      <p class="my-2 text-xs text-muted">
        Synthetic observations. No backup or guest-agent operation is sent.
      </p>
      <div class="mb-3 flex flex-wrap gap-2" aria-label="Fixture observation controls">
        {['retained', 'unavailable', 'unknown', 'current'].map((s) => (
          <button
            class="min-h-11 rounded-sm border border-border px-3 text-xs"
            onClick={() => setState(s)}
          >
            {s}
          </button>
        ))}
        <button
          class="min-h-11 rounded-sm border border-border px-3 text-xs"
          onClick={() => setSeen(Date.parse('2026-10-05T00:00:00Z'))}
        >
          Advance snapshot
        </button>
      </div>
      <section class="mx-auto max-w-5xl" aria-label="Production drawer">
        {new URLSearchParams(location.search).has('guest') ? (
          <GuestDrawer guest={guest()} onClose={() => {}} />
        ) : (
          <ResourceDetailDrawer resource={resource()} presentation="table-row" />
        )}
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
