// Synthetic REST observations through the real hook, tables, detail disclosures
// and CSS. No real websocket, appliance or installed/released acceptance claim.
import { createSignal, Show } from 'solid-js';
import { createStore, unwrap } from 'solid-js/store';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import { ProxmoxMailGatewayTable } from '../src/features/proxmox/ProxmoxMailGatewayTable';
import { DockerContainersTable } from '../src/features/docker/DockerContainersTable';
import { DockerImagesTable } from '../src/features/docker/DockerImagesTable';
import { DockerServicesTable } from '../src/features/docker/DockerServicesTable';
import { DockerTasksTable } from '../src/features/docker/DockerTasksTable';
import { StackedDiskBar } from '../src/components/Workloads/StackedDiskBar';
import { StoragePoolsTable } from '../src/components/Storage/StoragePoolsTable';
import { Table, TableBody, TableRow, TableCell } from '../src/components/shared/Table';
import { getDockerContainerLifecycleDisabledReason } from '../src/features/docker/dockerContainerLifecycleActions';
import { buildStorageRecords } from '../src/features/storageBackups/storageAdapters';
import { groupStorageRecords } from '../src/features/storageBackups/storageModelCore';
import { EMPTY_STORAGE_ALERT_STATE } from '../src/features/storageBackups/storageAlertState';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const [wsState, setWsState] = createStore({
  resources: [] as Resource[],
  lastUpdate: 0,
  nodes: [],
  pbs: [],
  pmg: [],
  vms: [],
  containers: [],
});
const [connected, setConnected] = createSignal(false);
const ws = {
  state: wsState,
  connected,
  initialDataReceived: () => true,
  resourceChange: () => ({ version: wsState.lastUpdate, changedIds: null, changedKeys: null }),
  activeAlerts: () => ({}),
  shutdown: () => {},
};
window.__pulseWsStore = ws as any;

function Fixture() {
  const [view, setView] = createSignal('Mail');
  const [expandedPool, setExpandedPool] = createSignal<string | null>(null);
  const rows = useUnifiedResources({ query: '', cacheKey: 'operator-rows-browser' });
  const ofType = (type: string) => rows.resources().filter((row) => row.type === type);
  const container = () => ofType('app-container')[0];
  const records = () =>
    buildStorageRecords({ state: wsState as any, resources: ofType('storage') });
  const grouped = () => groupStorageRecords(records(), 'none');
  (window as any).__operatorRows = {
    rows: () => JSON.parse(JSON.stringify(unwrap(rows.resources()))),
    refresh: () => rows.refetch(),
    seedWebsocket: () => {
      setWsState('resources', JSON.parse(JSON.stringify(unwrap(rows.resources()))));
      setWsState('lastUpdate', Date.now());
      setConnected(true);
    },
  };
  const nativeProps = {
    emptyIcon: <span />,
    emptyTitle: 'No fixture rows',
    emptyDescription: 'No fixture rows',
    showToolbar: false,
  };
  return (
    <WebSocketContext.Provider value={ws as any}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
          <h1 class="text-lg font-semibold">Transport facets and operator rows</h1>
          <nav aria-label="Fixture views" class="flex flex-wrap gap-2">
            {['Mail', 'Storage', 'Containers', 'Images', 'Swarm', 'Disk'].map((name) => (
              <button
                type="button"
                class="min-h-11 rounded border border-border px-3"
                onClick={() => setView(name)}
              >
                {name}
              </button>
            ))}
          </nav>
          <p data-fixture-loaded>{rows.resources().length} REST-mapped rows</p>
          <Show when={view() === 'Mail'}>
            <section aria-label="Mail table">
              <ProxmoxMailGatewayTable
                resources={ofType('pmg')}
                emptyTitle="No mail"
                emptyDescription="No mail"
              />
            </section>
          </Show>
          <Show when={view() === 'Storage'}>
            <section aria-label="Storage table">
              <StoragePoolsTable
                groupedRecords={grouped()}
                groupBy="none"
                expandedGroups={new Set(['All'])}
                toggleGroup={() => {}}
                expandedPoolId={expandedPool()}
                setExpandedPoolId={setExpandedPool}
                storageGrowthBySeriesId={new Map()}
                storageGrowthColumnLabel="Growth"
                physicalDisks={[]}
                nodeOnlineByLabel={new Map()}
                highlightedRecordId={null}
                getRecordAlertState={() => EMPTY_STORAGE_ALERT_STATE}
                isLoading={false}
              />
            </section>
          </Show>
          <Show when={view() === 'Containers'}>
            <section aria-label="Container table">
              <p data-lifecycle-refusal>
                {container() && getDockerContainerLifecycleDisabledReason(container(), 'restart')}
              </p>
              <DockerContainersTable resources={ofType('app-container')} {...nativeProps} />
            </section>
          </Show>
          <Show when={view() === 'Images'}>
            <section aria-label="Image table">
              <DockerImagesTable resources={ofType('docker-image')} {...nativeProps} />
            </section>
          </Show>
          <Show when={view() === 'Swarm'}>
            <section aria-label="Swarm services">
              <DockerServicesTable resources={ofType('docker-service')} {...nativeProps} />
            </section>
            <section aria-label="Swarm tasks">
              <DockerTasksTable resources={ofType('docker-task')} {...nativeProps} />
            </section>
          </Show>
          <Show when={view() === 'Disk'}>
            <section aria-label="Multi-disk Bars">
              <Table>
                <TableBody>
                  <TableRow>
                    <TableCell>pve-edge</TableCell>
                    <TableCell>
                      <StackedDiskBar
                        mode="vertical-bars"
                        disks={[
                          {
                            device: '/dev/sda',
                            mountpoint: '/',
                            usage: 24,
                            total: 100,
                            used: 24,
                            free: 76,
                            type: 'ext4',
                          },
                          {
                            device: '/dev/sdb',
                            mountpoint: '/data',
                            usage: 92,
                            total: 100,
                            used: 92,
                            free: 8,
                            type: 'zfs',
                          },
                        ]}
                      />
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </section>
          </Show>
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
