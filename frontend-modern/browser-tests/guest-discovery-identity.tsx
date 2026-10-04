// Production drawer, clients and CSS; synthetic targets and local HTTP only.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: true });
const makeGuest = (agentId: string, id = 'fixture-pve1-100'): WorkloadGuest =>
  ({
    id,
    vmid: 100,
    name: `Guest on ${agentId}`,
    node: 'pve1',
    instance: id === 'fixture-pve1-100' ? 'fixture' : 'fixture2',
    status: 'running',
    type: 'qemu',
    cpu: 0.1,
    cpus: 2,
    memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, usage: 25 },
    disk: { total: 10 * 1024 ** 3, used: 2 * 1024 ** 3, usage: 20 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: '2026-10-04T06:00:00Z',
    discoveryTarget: { resourceType: 'vm', agentId, resourceId: '100' },
  }) as WorkloadGuest;

function Fixture() {
  const [guest, setGuest] = createSignal(makeGuest('agent-a'));
  const [mounted, setMounted] = createSignal(true);
  (window as any).__guestIdentity = {
    update: (agentId: string, id?: string) => setGuest(makeGuest(agentId, id)),
    ticks: (count: number) => {
      for (let tick = 0; tick < count; tick++) {
        setGuest({ ...guest(), name: `Updated guest ${tick}`, cpu: tick / 1000 });
      }
    },
    mount: (value: boolean) => setMounted(value),
  };
  return (
    <main class="mx-auto max-w-4xl p-3 bg-surface text-base-content">
      <Show when={mounted()}>
        <GuestDrawer
          guest={guest()}
          onClose={() => setMounted(false)}
          customUrl="https://operator.example.test/"
        />
      </Show>
    </main>
  );
}

const dark = new URLSearchParams(location.search).has('dark');
document.documentElement.classList.toggle('dark', dark);
render(
  () => (
    <DarkModeContext.Provider
      value={{ darkMode: () => dark, toggleDarkMode: () => {}, setDarkMode: () => {} }}
    >
      <Router>
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
