// Presentation only: selected synthetic resources, real table, drawer and CSS.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DockerContainersTable } from '../src/features/docker/DockerContainersTable';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
const container = (name: string, host: string, cpu: number, engine = 'docker'): Resource => ({
  id: name,
  type: 'app-container',
  name,
  platformId: name,
  platformType: 'docker',
  sourceType: 'agent',
  status: 'running',
  cpu: { current: cpu },
  memory: { current: 10 },
  docker: { hostname: host, runtime: engine, runtimeVersion: 'fixture', containerState: 'running' },
});
const containers = [
  container('small-positive', 'edge-a', 0.26),
  container('measured-zero', 'edge-a', 0),
  container('tiny-positive', 'edge-b', 0.04, 'podman'),
];
const hosts: Resource[] = [
  {
    id: 'host-a',
    type: 'docker-host',
    name: 'edge-a',
    platformId: 'host-a',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'offline',
    docker: { hostname: 'edge-a', runtime: 'docker', runtimeVersion: 'fixture' },
    health: {
      verdict: 'stale',
      reasons: [{ code: 'telemetry_stale', detail: '9m' }],
    },
  },
  {
    id: 'host-b',
    type: 'docker-host',
    name: 'edge-b',
    platformId: 'host-b',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    docker: { hostname: 'edge-b', runtime: 'podman', runtimeVersion: 'fixture' },
  },
];
function Fixture() {
  const [withHosts, setWithHosts] = createSignal(true);
  return (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <main class="min-h-screen space-y-4 bg-surface p-3 text-base-content">
        <h1 class="text-lg font-semibold">Composed container presentation</h1>
        <p class="text-xs text-muted">Synthetic metrics. No container or notification action.</p>
        <button
          type="button"
          class="min-h-11 rounded border border-border px-3 text-xs"
          onClick={() => setWithHosts(!withHosts())}
        >
          {withHosts() ? 'Remove host context' : 'Restore host context'}
        </button>
        <DockerContainersTable
          resources={containers}
          hosts={withHosts() ? hosts : undefined}
          emptyIcon={<span />}
          emptyTitle="No containers"
          emptyDescription="No containers"
        />
      </main>
    </DarkModeContext.Provider>
  );
}
render(
  () => (
    <Router>
      <Route path="*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
