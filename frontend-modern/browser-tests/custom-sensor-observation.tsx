// Production canonical delta merger and full machine drawer; stored fixtures only.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
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
const initial: Resource = {
  id: 'sensor-machine',
  name: 'Hybrid machine',
  displayName: 'Hybrid machine',
  type: 'agent',
  platformId: 'fixture',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  sources: ['agent', 'proxmox'],
  status: 'online',
  lastSeen: Date.parse(observedAt),
  identity: {},
  agent: {
    agentId: 'fixture-agent',
    hostname: 'machine.invalid',
    stale: false,
    agentVersion: 'fixture',
    cpuCount: 2,
    sensors: { custom: [] },
  },
};
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
  (window as any).__customSensors = {
    report: (stale = false) =>
      update(
        { agent: { ...snapshot()[0].agent, stale, sensors: { custom: structuredClone(metrics) } } },
        ['agent'],
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
        <h1 class="text-base font-semibold">Machine custom metrics</h1>
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
            <Fixture />
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
