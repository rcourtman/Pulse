// Synthetic REST observations through the real hook and the production
// Kubernetes Pods, Deployments and Controllers tables with app CSS. No real
// websocket, cluster or installed/released acceptance claim.
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import { KubernetesPodsTable } from '../src/features/kubernetes/KubernetesPodsTable';
import { KubernetesDeploymentsTable } from '../src/features/kubernetes/KubernetesDeploymentsTable';
import { KubernetesControllersTable } from '../src/features/kubernetes/KubernetesControllersTable';
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
  resourceChange: () => ({ version: wsState.lastUpdate, changedIds: null, changedKeys: null }),
  activeAlerts: () => ({}),
  shutdown: () => {},
};
window.__pulseWsStore = ws as any;

function Fixture() {
  const rows = useUnifiedResources({ query: '', cacheKey: 'kubernetes-status-rows-browser' });
  const ofType = (...types: string[]) => rows.resources().filter((row) => types.includes(row.type));
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
          <h1 class="text-lg font-semibold">Kubernetes status rows</h1>
          <p data-fixture-loaded>{rows.resources().length} REST-mapped rows</p>
          <section aria-label="Pod table">
            <KubernetesPodsTable resources={ofType('pod')} {...tableProps} />
          </section>
          <section aria-label="Deployment table">
            <KubernetesDeploymentsTable resources={ofType('k8s-deployment')} {...tableProps} />
          </section>
          <section aria-label="Controller table">
            <KubernetesControllersTable resources={ofType('k8s-replicaset')} {...tableProps} />
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
