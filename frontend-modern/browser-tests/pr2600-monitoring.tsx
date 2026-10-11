// Production page/table and Debug derived state, with synthetic local evidence.
import { Show, createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { StandalonePageSurface } from '../src/features/standalone/StandalonePageSurface';
import { buildStandalonePostureSummary } from '../src/features/standalone/standalonePageModel';
import { useRelativeTimeNow } from '../src/utils/relativeTimeClock';
import { ResourceDetailDrawerDebugTab } from '../src/components/Infrastructure/ResourceDetailDrawerDebugTab';
import { useResourceDetailDrawerDerivedState } from '../src/components/Infrastructure/useResourceDetailDrawerDerivedState';
import type { UseResourceDetailDrawerStateResult } from '../src/components/Infrastructure/useResourceDetailDrawerState';
import type { PlatformData } from '../src/components/Infrastructure/resourceDetailMappers';
import { WebSocketContext, DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const start = Date.parse('2026-10-11T17:00:00Z');
const surface = new URLSearchParams(location.search).get('surface') ?? 'machines';
syncAIRuntimeSettings({ discovery_enabled: false });
const machine: Resource = {
  id: 'fixture-agent',
  name: 'Silent host',
  displayName: 'Silent host',
  type: 'agent',
  platformId: 'fixture-agent',
  platformType: 'agent',
  sourceType: 'agent',
  sources: ['agent'],
  status: 'online',
  lastSeen: start - 60_000,
  cpu: { current: 24 },
  identity: { hostname: 'fixture-host' },
  agent: { agentId: 'fixture-agent', stale: false },
};
const [wsState, setWsState] = createStore({
  resources: [machine],
  pmg: [],
  connectedInfrastructure: [],
});
const [resourceChange, setResourceChange] = createSignal({
  version: 1,
  changedIds: null,
  changedKeys: null,
});
const ws = {
  state: wsState,
  activeAlerts: {},
  connected: () => true,
  initialDataReceived: () => true,
  resourceSnapshotReceived: () => true,
  resourceChange,
  changedResourceIdsSince: () => null,
  activeAlertsHydrationStatus: () => 'ready',
  markAgentsTokenRevoked: () => {},
  markDockerRuntimesTokenRevoked: () => {},
};
(window as any).__pulseWsStore = ws;
const [debugResource, setDebugResource] = createStore<Resource & { platformData: PlatformData }>({
  id: 'fixture:pve-a:101',
  name: 'Held guest',
  displayName: 'Held guest',
  type: 'vm',
  platformId: '101',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: start - 60_000,
  platformData: {
    proxmox: { vmid: 101, node: 'pve-a', diskStatusReason: 'prev-agent-timeout' },
    agent: { id: 'fixture-agent' },
    sourceStatus: {
      proxmox: { status: 'deferred', lastSeen: start - 60_000, error: 'Guest read deferred' },
      agent: { status: 'stale', lastSeen: new Date(start - 120_000).toISOString() },
    },
  },
});

function DebugFixture() {
  const derived = useResourceDetailDrawerDerivedState({
    resource: debugResource,
    debugEnabled: () => true,
    discoveryFeatureEnabled: () => false,
    resourceIntelligence: () => null,
  });
  const sections = derived.sourceSections();
  (window as any).__pr2600.sectionsRetained = () => derived.sourceSections() === sections;
  const drawer = {
    ...derived,
    copied: () => false,
    handleCopyJson: () => {},
  } as unknown as UseResourceDetailDrawerStateResult;
  return <ResourceDetailDrawerDebugTab resource={debugResource} drawer={drawer} />;
}
function Fixture() {
  const now = useRelativeTimeNow();
  const [mounted, setMounted] = createSignal(true);
  (window as any).__pr2600 = {
    snapshot: () => JSON.stringify({ machine: wsState.resources[0], debugResource }),
    remount: () => {
      setMounted(false);
      setMounted(true);
    },
    report: () => {
      setWsState('resources', 0, 'lastSeen', Date.now());
      setResourceChange((value) => ({ ...value, version: value.version + 1 }));
    },
    updateStatus: () =>
      setDebugResource('platformData', 'sourceStatus', {
        proxmox: { status: 'available', lastSeen: Date.now() },
        agent: {
          status: 'error',
          lastSeen: new Date(Date.now()).toISOString(),
          error: 'Agent unavailable',
        },
      }),
    withdrawStatus: () =>
      setDebugResource(
        'platformData',
        'sourceStatus',
        reconcile({
          proxmox: { status: 'deferred', lastSeen: 'invalid', error: 'No usable timestamp' },
        }),
      ),
  };
  return (
    <main class="mx-auto min-h-screen max-w-6xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Retained monitoring evidence</h1>
      <p class="text-xs text-muted">Synthetic local data. No collector or guest command is sent.</p>
      <Show when={surface === 'machines'}>
        <output data-testid="fixture-posture-summary" class="text-xs text-muted">
          Classification control:{' '}
          {buildStandalonePostureSummary(wsState.resources, now()).attention} need attention
        </output>
      </Show>
      <Show when={mounted()}>
        <Show when={surface === 'machines'} fallback={<DebugFixture />}>
          <StandalonePageSurface />
        </Show>
      </Show>
    </main>
  );
}
render(
  () => (
    <WebSocketContext.Provider value={ws as any}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <Router>
          <Route path="/*" component={Fixture} />
        </Router>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
