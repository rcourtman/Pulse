import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { AvailabilityFleetView } from '../src/features/standalone/AvailabilityFleetView';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

// Synthetic replacement snapshots; the drawers, API client, fleet and CSS are production code.
syncAIRuntimeSettings({ discovery_enabled: false });
const pbs = (current = false, other = false, missing = false): Resource => ({
  id: other ? 'pbs-b' : 'pbs-a',
  name: other ? 'PBS B' : 'PBS A',
  displayName: other ? 'PBS B' : current ? 'PBS A current' : 'PBS A',
  type: 'pbs',
  platformId: other ? 'pbs-b' : 'pbs-a',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: current ? 'offline' : 'online',
  lastSeen: Date.now(),
  cpu: { current: current ? 77 : other ? 25 : 12 },
  metricsTarget: missing
    ? undefined
    : {
        resourceType: 'agent',
        resourceId: other ? 'history-b' : current ? 'history-a-current' : 'history-a',
      },
});
const check = (other = false, down = false): Resource => ({
  id: other ? 'check-b' : 'check-a',
  name: other ? 'Check B' : 'Check A',
  displayName: other ? 'Check B' : 'Check A',
  type: 'network-endpoint',
  platformId: other ? 'check-b' : 'check-a',
  platformType: 'availability',
  sourceType: 'api',
  status: down ? 'offline' : 'online',
  lastSeen: Date.now(),
  availability: {
    targetId: other ? 'check-b' : 'check-a',
    address: other ? 'check-b.invalid' : 'check-a.invalid',
    protocol: 'https',
    path: '/health',
    enabled: true,
    available: !down,
    latencyMillis: down ? undefined : 12,
    lastChecked: new Date().toISOString(),
    pollIntervalSeconds: 60,
  },
});
const [resource, setResource] = createSignal(pbs());
const [checks, setChecks] = createSignal([check(), check(true)]);
const controls = [
  ['Update PBS A', () => setResource(pbs(true))],
  ['Withdraw target', () => setResource(pbs(true, false, true))],
  ['Restore target', () => setResource(pbs(true))],
  ['Select PBS B', () => setResource(pbs(false, true))],
  ['Fail check A', () => setChecks([check(false, true), check(true)])],
  ['Remove check B', () => setChecks([check(false, true)])],
  ['Restore check B', () => setChecks([check(false, true), check(true)])],
] as const;
render(
  () => (
    <WebSocketContext.Provider value={{ state: { pmg: [] }, connected: () => true } as any}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
          <h1>Detail snapshot verification</h1>
          <p>Synthetic PBS and availability snapshots; no appliance or release acceptance.</p>
          <nav aria-label="Snapshot controls" class="flex flex-wrap gap-2">
            {controls.map(([name, action]) => (
              <button class="min-h-11 rounded border border-border px-3" onClick={action}>
                {name}
              </button>
            ))}
          </nav>
          <section aria-label="PBS detail verification" class="rounded border border-border p-3">
            <ResourceDetailDrawer resource={resource()} presentation="table-row" />
          </section>
          <AvailabilityFleetView
            resources={checks()}
            historyByTarget={new Map()}
            historyLoading={false}
          />
        </main>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
