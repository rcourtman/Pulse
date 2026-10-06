// Actual production row, full drawer, router, documentation renderer and CSS.
// Observations are synthetic. This sends no guest or backup command.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import Docs from '../src/pages/Docs';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
function Fixture() {
  const [reading, setReading] = createSignal({
    reason: 'agent-not-running',
    retained: false,
    os: 'Windows 11',
  });
  const [expanded, setExpanded] = createSignal(false);
  const guest = (): WorkloadGuest => ({
    id: 'fixture-pve:pve-a:101',
    vmid: 101,
    name: 'reading-guest',
    node: 'pve-a',
    instance: 'fixture-pve',
    status: 'running',
    type: 'qemu',
    cpu: 0.1,
    cpus: 2,
    memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
    disk: { total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
    disks: reading().retained
      ? [
          {
            mountpoint: '/data',
            total: 10 * 1024 ** 3,
            used: 5 * 1024 ** 3,
            usage: 50,
            type: 'ext4',
          },
        ]
      : [],
    diskStatusReason:
      reading().retained && reading().reason ? `prev-${reading().reason}` : reading().reason,
    osName: reading().os,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lastSeen: '2026-10-06T03:16:33Z',
  });
  (window as any).__readingHelp = { update: setReading };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1>Guest reading help</h1>
      <p class="text-xs text-muted">Synthetic observations; no guest command is sent.</p>
      <table class="w-full table-fixed">
        <tbody>
          <GuestRow
            guest={guest()}
            visibleColumnIds={['name', 'disk']}
            isExpanded={expanded()}
            onClick={() => setExpanded(!expanded())}
            workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
          />
        </tbody>
      </table>
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
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="/docs/*docPath" component={Docs} />
        <Route path="/*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
