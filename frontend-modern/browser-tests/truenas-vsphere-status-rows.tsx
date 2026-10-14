// Synthetic resources through the production TrueNAS Apps and Shares tables and
// the vSphere Activity table with app CSS. No real appliance, vCenter or
// installed/released acceptance claim.
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { TrueNASAppsTable } from '../src/features/truenas/TrueNASAppsTable';
import { TrueNASNetworkSharesTable } from '../src/features/truenas/TrueNASNetworkSharesTable';
import { TrueNASServicesTable } from '../src/features/truenas/TrueNASServicesTable';
import { buildTrueNASServiceRows } from '../src/features/truenas/truenasPageModel';
import { VsphereActivityTable } from '../src/features/vmware/VsphereActivityTable';
import { buildVmwareActivityRows } from '../src/features/vmware/vmwarePageModel';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import type { Resource } from '../src/types/resource';
import { markSystemSettingsLoadedWithDefaults } from '../src/stores/systemSettings';
import '../src/index.css';

markSystemSettingsLoadedWithDefaults();

const [wsState] = createStore({
  resources: [] as Resource[],
  lastUpdate: 0,
  nodes: [],
  pbs: [],
  pmg: [],
  vms: [],
  containers: [],
});
const [connected] = createSignal(false);
const ws = {
  state: wsState,
  connected,
  initialDataReceived: () => true,
  resourceChange: () => ({ version: 0, changedIds: null, changedKeys: null }),
  activeAlerts: () => ({}),
  shutdown: () => {},
};
window.__pulseWsStore = ws as any;

const now = '2026-10-03T12:00:00Z';
const truenas = (resource: Partial<Resource>): Resource =>
  ({
    status: 'online',
    lastSeen: now,
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'api',
    parentName: 'nas-primary',
    ...resource,
  }) as Resource;

const system = truenas({
  id: 'system-primary',
  type: 'agent',
  name: 'nas-primary',
  displayName: 'nas-primary',
  truenas: {
    hostname: 'nas-primary',
    version: 'TrueNAS-SCALE-24.10.2',
    services: [
      { id: '1', service: 'smb', enabled: true, state: 'RUNNING' },
      { id: '2', service: 'smartd', enabled: true, state: 'STOPPED' },
    ],
  },
});
// statusFromApp / statusFromNetworkShare: CRASHED -> warning, disabled -> offline.
const apps = [
  truenas({
    id: 'app-plex',
    type: 'app-container',
    name: 'plex',
    status: 'warning',
    truenas: { app: { id: 'plex', name: 'plex', state: 'CRASHED', containerCount: 1 } },
  }),
  truenas({
    id: 'app-immich',
    type: 'app-container',
    name: 'immich',
    status: 'online',
    truenas: { app: { id: 'immich', name: 'immich', state: 'RUNNING', containerCount: 4 } },
  }),
];
const shares = [
  truenas({
    id: 'share-archive',
    type: 'network-share',
    name: 'archive',
    status: 'offline',
    truenas: {
      share: {
        id: '1',
        name: 'archive',
        protocol: 'smb',
        path: '/mnt/tank/archive',
        enabled: false,
      },
    },
  }),
  truenas({
    id: 'share-media',
    type: 'network-share',
    name: 'media',
    status: 'warning',
    truenas: {
      share: {
        id: '2',
        name: 'media',
        protocol: 'nfs',
        path: '/mnt/tank/media',
        enabled: true,
        locked: true,
      },
    },
  }),
];
const vm = {
  id: 'vm-201',
  type: 'vm',
  name: 'warehouse-api-01',
  displayName: 'warehouse-api-01',
  status: 'online',
  platformType: 'vmware-vsphere',
  platformScopes: ['vmware-vsphere'],
  sourceType: 'api',
  vmware: { entityType: 'vm', managedObjectId: 'vm-201', connectionName: 'lab-vcenter' },
  recentChanges: [
    {
      id: 'activity-task-reconfigure',
      observedAt: now,
      occurredAt: now,
      resourceId: 'vm-201',
      kind: 'activity',
      sourceType: 'platform_event',
      sourceAdapter: 'vmware_adapter',
      confidence: 'high',
      reason: 'Reconfigure virtual machine (error)',
      metadata: {
        activity_type: 'vmware_task',
        activity_native_id: 'task-901',
        activity_title: 'Reconfigure virtual machine',
        activity_state: 'error',
        activity_message: 'Permission denied while reconfiguring VM',
        vmwareManagedObjectId: 'vm-201',
        vmwareEntityType: 'vm',
      },
    },
  ],
} as unknown as Resource;

function Fixture() {
  const tableProps = {
    emptyIcon: <span />,
    emptyTitle: 'No fixture rows',
    emptyDescription: 'No fixture rows',
    showToolbar: false,
  };
  return (
    <WebSocketContext.Provider value={ws as any}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
          <h1 class="text-lg font-semibold">TrueNAS and vSphere status rows</h1>
          <p data-fixture-loaded>fixture ready</p>
          <section aria-label="App table">
            <TrueNASAppsTable apps={apps} scope={[system, ...apps]} {...tableProps} />
          </section>
          <section aria-label="Share table">
            <TrueNASNetworkSharesTable
              shares={shares}
              scope={[system, ...shares]}
              {...tableProps}
            />
          </section>
          <section aria-label="Service table">
            <TrueNASServicesTable services={buildTrueNASServiceRows([system])} {...tableProps} />
          </section>
          <section aria-label="Activity table">
            <VsphereActivityTable
              singleVcenter
              activity={buildVmwareActivityRows([vm])}
              {...tableProps}
            />
          </section>
        </main>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
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
