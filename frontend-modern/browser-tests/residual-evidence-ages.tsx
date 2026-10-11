// Actual current panels, state, API adapters and CSS with synthetic local inputs.
// No native guest, permission grant, token creation or profile save is performed.
import { createSignal, Show, Suspense } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { ResourceChangeSummary } from '../src/components/Infrastructure/ResourceChangeSummary';
import { ResourceOperatorStateSection } from '../src/components/Infrastructure/ResourceOperatorStateSection';
import { APITokenManager } from '../src/components/Settings/APITokenManager';
import { AgentProfilesPanel } from '../src/components/Settings/AgentProfilesPanel';
import { SuggestProfileModal } from '../src/components/Settings/SuggestProfileModal';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import { loadRuntimeCapabilities } from '../src/stores/license';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const start = Date.parse('2026-10-11T11:00:00Z');
const iso = (offset: number) => new Date(start + offset).toISOString();
const surface = new URLSearchParams(location.search).get('surface') ?? 'resource';
syncAIRuntimeSettings({ discovery_enabled: false });
const change = {
  id: 'fixture-change',
  resourceId: 'fixture:pve-a:101',
  kind: 'config_update' as const,
  observedAt: iso(-60_000),
  occurredAt: iso(-120_000),
  reason: 'Retained retention change',
  sourceType: 'platform_event' as const,
  sourceAdapter: 'proxmox_adapter' as const,
  confidence: 'high' as const,
};
const initial: Resource = {
  id: 'fixture:pve-a:101',
  name: 'Held evidence guest',
  displayName: 'Held evidence guest',
  platformId: '101',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  type: 'vm',
  status: 'online',
  lastSeen: start - 60_000,
  recentChanges: [change],
};
const agent: Resource = {
  id: 'fixture-agent',
  name: 'Held profile host',
  displayName: 'Held profile host',
  type: 'agent',
  platformId: 'fixture-agent',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: start - 60_000,
  identity: { hostname: 'fixture-host' },
  agent: { agentId: 'fixture-agent' },
};
const [wsState, setWsState] = createStore({
  resources: [agent],
  pmg: [],
  connectedInfrastructure: [
    {
      id: 'fixture-host',
      name: 'Held profile host',
      displayName: 'Held profile host',
      hostname: 'fixture-host',
      status: 'active',
      lastSeen: start - 60_000,
      surfaces: [{ kind: 'agent', controlId: 'fixture-agent', status: 'active' }],
    },
  ],
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
if (surface === 'profiles') void loadRuntimeCapabilities();

function Fixture() {
  const [resource, setResource] = createSignal(initial);
  const [mounted, setMounted] = createSignal(true);
  const [refreshing, setRefreshing] = createSignal(false);
  const [showDrafts, setShowDrafts] = createSignal(true);
  let accepted: unknown = null;
  (window as any).__residualAges = {
    snapshot: () => JSON.stringify({ resource: resource(), agent: wsState.resources[0], accepted }),
    unmount: () => setMounted(false),
    remount: () => setMounted(true),
    refreshTokens: () => setRefreshing(true),
    replaceChange: () =>
      setResource({
        ...initial,
        recentChanges: [
          { ...change, observedAt: new Date(Date.now()).toISOString(), occurredAt: undefined },
        ],
      }),
    removeChange: () => setResource({ ...initial, recentChanges: [] }),
    replaceAgent: () => {
      setWsState('resources', [{ ...agent, lastSeen: Date.now() }]);
      setResourceChange((value) => ({ ...value, version: value.version + 1 }));
    },
    accepted: () => accepted,
  };
  return (
    <main class="mx-auto min-h-screen max-w-6xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Held change and administration ages</h1>
      <p class="text-xs text-muted">Synthetic local evidence. No production operation is sent.</p>
      <Suspense fallback={<p>Loading fixture panels</p>}>
        <Show when={mounted()}>
          <Show when={surface === 'resource'}>
            <section data-testid="held-summary">
              <ResourceChangeSummary
                title="Recent changes"
                changes={resource().recentChanges}
                buildResourceHref={() => '#fixture-resource'}
              />
            </section>
            <ResourceDetailDrawer resource={resource()} presentation="table-row" />
          </Show>
          <Show when={surface === 'operator'}>
            <ResourceOperatorStateSection resourceId="held-policy" />
          </Show>
          <Show when={surface === 'tokens'}>
            <APITokenManager canManage refreshing={refreshing()} />
          </Show>
          <Show when={surface === 'profiles'}>
            <AgentProfilesPanel />
          </Show>
          <Show when={surface === 'drafts' && showDrafts()}>
            <SuggestProfileModal
              onClose={() => setShowDrafts(false)}
              onSuggestionAccepted={(value) => {
                accepted = value;
              }}
            />
          </Show>
        </Show>
      </Suspense>
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
