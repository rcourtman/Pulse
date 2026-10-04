// Production table/router/CSS and canonical resource adapter. Synthetic read
// states only: no provider, native datastore, backup or thaw operation.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import type { Resource, ResourcePBSDatastore } from '../src/types/resource';
import { canonicalizeRealtimeResource } from '../src/utils/resourceStateAdapters';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import '../src/index.css';

const states = ['Healthy', 'Failure', 'Unknown', 'Offline', 'Zero', 'Full', 'Error', 'No stores'];
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
  const servers = createMemo((): Resource[] => {
    let store: ResourcePBSDatastore = { ...normal };
    if (state() === 'Failure')
      store = { ...store, total: 0, used: 0, available: 0, usagePercent: 0, status: 'unavailable' };
    if (state() === 'Unknown')
      store = { ...store, total: 0, used: 0, available: 0, usagePercent: 0 };
    if (state() === 'Zero') store = { ...store, used: 0, usagePercent: 0, available: 10000 };
    if (state() === 'Full') store = { ...store, used: 9500, usagePercent: 95, available: 500 };
    if (state() === 'Error') store = { ...store, error: 'PRIVATE_PROVIDER_ERROR_SENTINEL' };
    const pbs = {
      instanceId: 'pbs-main',
      connectionHealth: state() === 'Offline' ? 'offline' : 'healthy',
      version: '3.2.1',
      datastores: state() === 'No stores' ? [] : [store],
    };
    const resource: Resource = {
      id: 'pbs-main',
      type: 'pbs',
      name: 'pbs-main',
      platformId: 'pbs-main',
      platformType: 'proxmox-pbs',
      sourceType: 'api',
      status: 'online',
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
    <Router>
      <Route path="/*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
