// Production canonical delta merger and full machine drawer; stored fixtures only.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { ResourceCompositionFixture } from './resource-composition-observation';
import { mergeCanonicalResourceDeltaSnapshot } from '../src/utils/resourceStateAdapters';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import type { HostCustomSensorMetric } from '../src/types/api';
import '../src/index.css';
const observedAt = '2026-10-20T12:00:00Z';
const metrics: HostCustomSensorMetric[] = [
  { id: 'service', name: 'Checkout', kind: 'boolean', value: 1, status: 'ok', observedAt },
  {
    id: 'backup',
    name: 'Last file backup',
    kind: 'timestamp',
    value: 7200,
    status: 'critical',
    observedAt,
    eventAt: '2026-10-20T10:00:00Z',
  },
  { id: 'queue', name: 'Queue depth', value: 0, unit: 'items', status: 'warning', observedAt },
  {
    id: 'failed',
    name: 'Database health',
    value: 1,
    kind: 'boolean',
    status: 'error',
    stale: true,
    error: 'Synthetic read timed out',
    observedAt,
  },
  {
    id: 'missing',
    name: 'Missing reading',
    status: 'error',
    error: 'Synthetic no result',
    observedAt,
  },
];
const guest: Resource = {
  id: 'sensor-machine',
  name: 'Hybrid machine',
  displayName: 'Hybrid machine',
  type: 'vm',
  platformId: 'fixture',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'hybrid',
  sources: ['agent', 'proxmox'],
  status: 'online',
  lastSeen: Date.parse(observedAt),
  identity: {},
  proxmox: {
    runtimeStatus: 'running',
    networkInterfaces: [{ name: 'host-config', addresses: ['192.0.2.99/24'] }],
  },
  agent: {
    agentId: 'fixture-agent',
    hostname: 'machine.invalid',
    stale: false,
    agentVersion: 'fixture',
    cpuCount: 2,
    sensors: { custom: [] },
  },
};
// Reuse the production drawer/merger fixture for the API host aggregate path.
const rootDiskOnly = new URLSearchParams(window.location.search).get('kind') === 'root-disk';
const compositionOnly = new URLSearchParams(window.location.search).get('kind') === 'composition';
const initial: Resource = rootDiskOnly
  ? {
      id: 'root-disk-host',
      name: 'API host',
      displayName: 'API host',
      type: 'agent',
      platformId: 'fixture',
      platformType: 'proxmox-pve',
      platformScopes: ['proxmox-pve'],
      sourceType: 'api',
      sources: ['proxmox'],
      status: 'online',
      lastSeen: Date.parse(observedAt),
      platformData: { proxmox: { nodeName: 'API host' } },
    }
  : guest;
syncAIRuntimeSettings({ discovery_enabled: false });
function Fixture() {
  const [snapshot, setSnapshot] = createSignal<Resource[]>([structuredClone(initial)]);
  const update = (patch: Partial<Resource>, keys: (keyof Resource)[]) =>
    setSnapshot((previous) =>
      mergeCanonicalResourceDeltaSnapshot(
        [{ ...previous[0], ...patch }],
        previous,
        new Set([initial.id]),
        new Map([[initial.id, keys]]),
      ),
    );
  (window as any).__resourceAgent = {
    rootDisk: (disk?: Resource['disk']) => update({ disk }, ['disk']),
    report: (stale = false) =>
      update(
        {
          agent: {
            ...snapshot()[0].agent,
            stale,
            disks: [{ mountpoint: '/data', total: 10 * 1024 ** 3, used: 9 * 1024 ** 3 }],
            networkInterfaces: [
              { name: 'eth0', addresses: ['192.0.2.11/24'], mac: '02:00:00:00:00:11' },
            ],
            raid: [
              {
                device: '/dev/md0',
                name: 'data',
                level: 'raid1',
                state: 'clean, degraded, recovering',
                totalDevices: 2,
                activeDevices: 1,
                workingDevices: 1,
                failedDevices: 1,
                spareDevices: 0,
                rebuildPercent: 37,
                rebuildSpeed: '100M/sec',
              },
            ],
            sensors: {
              custom: structuredClone(metrics),
              temperatureCelsius: { cpu: 42 },
              smart: [{ device: '/dev/sda', temperature: 36 }],
            },
          },
        },
        ['agent'],
      ),
    power: (runtimeStatus: string) =>
      update({ proxmox: { ...snapshot()[0].proxmox, runtimeStatus } }, ['proxmox']),
    kind: (type: Resource['type']) => update({ type }, ['type']),
    scope: (scope: string) => update({ platformScopes: [scope] }, ['platformScopes']),
    noAgentNetwork: () =>
      update(
        {
          agent: { ...snapshot()[0].agent, networkInterfaces: [] },
          // The existing API-only host card requires its actual host context;
          // a guest facet alone is not proof that this branch was consumed.
          platformData: { sources: ['agent', 'proxmox'], proxmox: { nodeName: 'fixture-node' } },
        },
        ['agent', 'platformData'],
      ),
    stale: (stale: boolean) => update({ agent: { ...snapshot()[0].agent, stale } }, ['agent']),
    platform: () =>
      update({ status: 'online', lastSeen: Date.parse('2026-10-20T12:05:00Z') }, [
        'status',
        'lastSeen',
      ]),
    remove: () => update({ agent: { ...snapshot()[0].agent, sensors: { custom: [] } } }, ['agent']),
    evidence: () => structuredClone(snapshot()[0]),
  };
  const ws = {
    activeAlerts: {},
    state: {
      get resources() {
        return snapshot();
      },
    },
  };
  return (
    <WebSocketContext.Provider value={ws as any}>
      <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
        <h1 class="text-base font-semibold">
          {rootDiskOnly ? 'Canonical root disk observations' : 'Canonical guest agent observations'}
        </h1>
        <p class="text-xs text-muted">
          Synthetic stored reports. No agent, service or backup checks are sent.
        </p>
        <ResourceDetailDrawer
          resource={snapshot()[0]}
          presentation="table-row"
          initialShowHostDetails
        />
      </main>
    </WebSocketContext.Provider>
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
            {compositionOnly ? <ResourceCompositionFixture /> : <Fixture />}
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
