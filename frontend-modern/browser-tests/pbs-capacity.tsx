// Production table/router/CSS and canonical resource adapter. Synthetic read
// states only: no provider, native datastore, backup or thaw operation.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import type { Resource, ResourcePBSDatastore } from '../src/types/resource';
import { canonicalizeRealtimeResource } from '../src/utils/resourceStateAdapters';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import '../src/index.css';

const states = [
  'Healthy',
  'Failure',
  'Unknown',
  'Offline',
  'Primary offline',
  'Primary warning',
  'False empty',
  'False full',
  'Rounded up',
  'Rounded down',
  'Overflow ratio',
  'Zero',
  'Full',
  'Error',
  'No stores',
];
const normal = {
  name: 'main',
  total: 10000,
  used: 4000,
  available: 6000,
  usagePercent: 40,
  status: 'available',
  deduplicationFactor: 2,
};

function Fixture() {
  const [state, setState] = createSignal('Healthy');
  Object.assign(window, { __pbsCapacityFixture: { setState } });
  const detailsMode = new URLSearchParams(location.search).has('details');
  const servers = createMemo((): Resource[] => {
    let store: ResourcePBSDatastore = { ...normal };
    if (state() === 'Failure')
      store = { ...store, total: 0, used: 0, available: 0, usagePercent: 0, status: 'unavailable' };
    if (state() === 'Unknown')
      store = { ...store, total: 0, used: 0, available: 0, usagePercent: 0 };
    if (state() === 'Zero') store = { ...store, used: 0, usagePercent: 0, available: 10000 };
    if (state() === 'Full' || state() === 'Primary warning')
      store = { ...store, used: 9500, usagePercent: 95, available: 500 };
    if (state() === 'False empty')
      store = { ...store, used: 9500, usagePercent: 0, available: 500 };
    if (state() === 'False full')
      store = { ...store, used: 1000, usagePercent: 99, available: 9000 };
    if (state() === 'Rounded up')
      store = { ...store, used: 7460, usagePercent: 75, available: 2540 };
    if (state() === 'Rounded down')
      store = { ...store, used: 9010, usagePercent: 90, available: 990 };
    if (state() === 'Overflow ratio')
      store = { ...store, total: Number.MIN_VALUE, used: Number.MAX_VALUE, usagePercent: 40 };
    if (state() === 'Error') store = { ...store, error: 'PRIVATE_PROVIDER_ERROR_SENTINEL' };
    if (detailsMode) store.name = `main/${'capacity-'.repeat(14)}<literal-name>`;
    const pbs = {
      instanceId: 'pbs-main',
      connectionHealth: state() === 'Offline' ? 'offline' : 'healthy',
      version: '3.2.1',
      datastores:
        state() === 'No stores'
          ? []
          : detailsMode
            ? [
                store,
                {
                  ...normal,
                  name: 'secondary',
                  total: 0,
                  used: 0,
                  available: 0,
                  status: 'unknown',
                },
              ]
            : [store],
    };
    const resource: Resource = {
      id: 'pbs-main',
      type: 'pbs',
      name: detailsMode ? `pbs-main-${'connection-'.repeat(10)}` : 'pbs-main',
      displayName: detailsMode ? `pbs-main-${'connection-'.repeat(10)}` : 'pbs-main',
      platformId: 'pbs-main',
      platformType: 'proxmox-pbs',
      sourceType: 'api',
      status:
        state() === 'Primary offline'
          ? 'offline'
          : state() === 'Primary warning'
            ? 'warning'
            : 'online',
      lastSeen: Date.parse('2026-10-04T12:00:00Z'),
      cpu: { current: 12 },
      memory: { current: 40, total: 10000, used: 4000 },
      pbs,
    };
    if (new URLSearchParams(location.search).get('transport') === 'canonical') {
      resource.pbs = undefined;
      resource.platformData = { sources: ['pbs'], pbs };
      return [canonicalizeRealtimeResource(resource)];
    }
    return [resource];
  });
  return (
    <main class="p-3 sm:p-6">
      <div class="mb-4 flex flex-wrap gap-2" aria-label="Synthetic observation controls">
        {states.map((value) => (
          <button class="rounded border px-2 py-1" type="button" onClick={() => setState(value)}>
            {value}
          </button>
        ))}
      </div>
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[]} servers={servers()} />
    </main>
  );
}

render(
  () => (
    <WebSocketContext.Provider
      value={{ state: { pmg: [] }, activeAlerts: {}, connected: () => false } as any}
    >
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <Router>
          <Route path="/*" component={Fixture} />
        </Router>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
