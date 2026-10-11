// Production page/router/table with synthetic, producer-shaped monitoring data.
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { StandalonePageSurface } from '../src/features/standalone/StandalonePageSurface';
import { WebSocketContext, DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const start = Date.parse('2026-10-11T17:00:00Z');
syncAIRuntimeSettings({ discovery_enabled: false });
const machine: Resource = {
  id: 'fixture-agent', name: 'Silent host', displayName: 'Silent host', type: 'agent',
  platformId: 'fixture-agent', platformType: 'agent', sourceType: 'agent', sources: ['agent'],
  status: 'online', lastSeen: start - 60_000, cpu: { current: 24 },
  identity: { hostname: 'fixture-host' }, agent: { agentId: 'fixture-agent', stale: false },
};
const [state, setState] = createStore({ resources: [machine], pmg: [], connectedInfrastructure: [] });
const [resourceChange, setResourceChange] = createSignal({ version: 1, changedIds: null, changedKeys: null });
const ws = {
  state, activeAlerts: {}, connected: () => true, initialDataReceived: () => true,
  resourceSnapshotReceived: () => true, resourceChange, changedResourceIdsSince: () => null,
  activeAlertsHydrationStatus: () => 'ready', markAgentsTokenRevoked: () => {}, markDockerRuntimesTokenRevoked: () => {},
};
(window as any).__pulseWsStore = ws;
function Fixture() {
  (window as any).__healthFilters = {
    snapshot: () => JSON.stringify(state.resources),
    report: () => {
      setState('resources', 0, 'lastSeen', Date.now());
      setState('resources', 0, 'agent', 'stale', false);
      setResourceChange((v) => ({ ...v, version: v.version + 1 }));
    },
    withdraw: () => {
      setState('resources', 0, 'lastSeen', 0);
      setState('resources', 0, 'agent', 'stale', true);
      setResourceChange((v) => ({ ...v, version: v.version + 1 }));
    },
  };
  return <main class="mx-auto min-h-screen max-w-6xl space-y-3 bg-surface p-3 text-base-content">
    <h1 class="text-base font-semibold">Machines health filter control</h1>
    <p class="text-xs text-muted">Synthetic evidence. No collector or guest command is sent.</p>
    <StandalonePageSurface />
  </main>;
}
render(() => <WebSocketContext.Provider value={ws as any}>
  <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
    <Router><Route path="/*" component={Fixture} /></Router>
  </DarkModeContext.Provider>
</WebSocketContext.Provider>, document.getElementById('root')!);
