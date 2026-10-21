// Navigation from a websocket-only cache into the real REST-backed consumers.
// Every input is synthetic stored inventory; this fixture sends no guest command.
import { createSignal, Show } from 'solid-js';
import { createStore, unwrap } from 'solid-js/store';
import { WebSocketContext } from '../src/contexts/appRuntime';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { GuestPhysicalDisks } from '../src/components/Workloads/GuestPhysicalDisks';
import { DockerContainersTable } from '../src/features/docker/DockerContainersTable';
import { markSystemSettingsLoadedWithDefaults } from '../src/stores/systemSettings';
import { dispatchResourceMetadataChanged } from '../src/utils/resourceMetadataEvents';
import type { Resource } from '../src/types/resource';

const observedAt = '2026-10-20T12:00:00Z';
const seed: Resource[] = [
  {
    id: 'app-container-web',
    name: 'edge-web',
    displayName: 'edge-web',
    type: 'app-container',
    platformId: 'docker',
    platformType: 'docker',
    sourceType: 'agent',
    sources: ['docker'],
    status: 'running',
    lastSeen: Date.parse(observedAt),
    capabilities: [
      { name: 'update', type: 'common', platform: 'docker', minimumApprovalLevel: 'admin' },
    ],
    docker: {
      agentId: 'agent-edge',
      hostSourceId: 'agent-edge',
      containerId: 'native-web',
      runtime: 'docker',
      containerState: 'running',
      image: 'nginx:latest',
      updateStatus: { updateAvailable: true, lastChecked: observedAt },
    },
  },
  {
    id: 'vm-resource-101',
    name: 'qualified-memory',
    displayName: 'qualified-memory',
    type: 'vm',
    platformId: 'fixture',
    platformType: 'proxmox-pve',
    platformScopes: ['proxmox-pve'],
    sourceType: 'hybrid',
    sources: ['proxmox', 'agent'],
    status: 'online',
    lastSeen: Date.parse(observedAt),
    memory: {
      current: 75,
      used: 3 * 1024 ** 3,
      total: 4 * 1024 ** 3,
      observation: { state: 'last-known', source: 'guest-agent-meminfo', observedAt },
    },
    proxmox: { vmid: 101, instance: 'fixture', nodeName: 'pve-a', runtimeStatus: 'running' },
  },
];

const SeedScreen = (props: { next: () => void }) => {
  const rows = useUnifiedResources({
    query: '',
    cacheKey: 'all-resources',
    initialHydration: 'prefer-ws-then-rest',
  });
  (window as any).__composition = {
    evidence: () => JSON.parse(JSON.stringify(unwrap(rows.resources()))),
  };
  return (
    <section aria-label="Websocket seed">
      <p data-seed-count>{rows.resources().length} websocket-only resources</p>
      <button class="min-h-11 rounded border border-border px-3" onClick={props.next}>
        Open REST-backed resources
      </button>
    </section>
  );
};

const CanonicalScreen = () => {
  const rows = useUnifiedResources({
    query: 'type=app-container,vm',
    cacheKey: 'composition-scoped-resources',
    initialHydration: 'prefer-ws-then-rest',
  });
  const [view, setView] = createSignal('Container details');
  const ofType = (type: Resource['type']) => rows.resources().filter((row) => row.type === type);
  (window as any).__composition = {
    evidence: () => JSON.parse(JSON.stringify(unwrap(rows.resources()))),
    refresh: () => rows.refetch(),
    rereadDisk: () =>
      dispatchResourceMetadataChanged({ metadataKind: 'guest', metadataId: 'vm-resource-101' }),
  };
  return (
    <>
      <nav class="flex flex-wrap gap-2" aria-label="Fixture consumer views">
        {['Container details', 'Container table', 'Memory details', 'Disk inventory'].map(
          (name) => (
            <button
              type="button"
              class="min-h-11 rounded border border-border px-3"
              aria-pressed={view() === name}
              onClick={() => setView(name)}
            >
              {name}
            </button>
          ),
        )}
      </nav>
      <Show when={view() === 'Container details' && ofType('app-container')[0]}>
        <section aria-label="Container details consumer">
          <ResourceDetailDrawer resource={ofType('app-container')[0]} presentation="table-row" />
        </section>
      </Show>
      <Show when={view() === 'Container table'}>
        <section aria-label="Container table consumer">
          <DockerContainersTable
            resources={ofType('app-container')}
            emptyIcon={<span />}
            emptyTitle="No containers"
            emptyDescription="No synthetic containers"
            showToolbar={false}
          />
        </section>
      </Show>
      <Show when={view() === 'Memory details' && ofType('vm')[0]}>
        <section aria-label="Memory details consumer">
          <ResourceDetailDrawer resource={ofType('vm')[0]} presentation="table-row" />
        </section>
      </Show>
      <Show when={view() === 'Disk inventory'}>
        <section aria-label="Disk inventory consumer">
          <GuestPhysicalDisks parentId="vm-resource-101" />
        </section>
      </Show>
    </>
  );
};

export const ResourceCompositionFixture = () => {
  markSystemSettingsLoadedWithDefaults();
  const [state] = createStore({
    resources: structuredClone(seed),
    lastUpdate: 1,
    nodes: [],
    pbs: [],
    pmg: [],
    vms: [],
    containers: [],
  });
  const ws = {
    state,
    connected: () => true,
    initialDataReceived: () => true,
    resourceChange: () => ({ version: state.lastUpdate, changedIds: null, changedKeys: null }),
    activeAlerts: {},
    shutdown: () => {},
  };
  window.__pulseWsStore = ws as any;
  const [scoped, setScoped] = createSignal(false);
  return (
    <WebSocketContext.Provider value={ws as any}>
      <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
        <h1 class="text-base font-semibold">Canonical resource composition</h1>
        <p class="text-xs text-muted">
          Synthetic stored reports. No agent, guest, update or backup operation is sent.
        </p>
        <Show when={scoped()} fallback={<SeedScreen next={() => setScoped(true)} />}>
          <CanonicalScreen />
        </Show>
      </main>
    </WebSocketContext.Provider>
  );
};
