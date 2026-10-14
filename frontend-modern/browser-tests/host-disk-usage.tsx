// Canonical Resources HTTP -> real platform tables -> real host drawers/CSS.
// Synthetic source observations only; no disk, guest or recovery operation.
import { Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import { ProxmoxNodesTable } from '../src/features/proxmox/ProxmoxNodesTable';
import { DockerHostsTable } from '../src/features/docker/DockerHostsTable';
import { DarkModeContext, WebSocketContext, type WebSocketStore } from '../src/contexts/appRuntime';
import '../src/index.css';

function Fixture() {
  const state = useUnifiedResources({
    query: 'type=agent,docker-host',
    realtimeEnabled: () => false,
  });
  const kind = new URLSearchParams(window.location.search).get('kind');
  (window as any).__hostDiskUsage = { refetch: state.refetch, resources: state.resources };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Host filesystem readings</h1>
      <p class="text-xs text-muted">Synthetic resource snapshots; no host operations.</p>
      <Show
        when={kind === 'node'}
        fallback={
          <DockerHostsTable
            resources={state.resources().filter((value) => value.type === 'docker-host')}
            emptyIcon={<span />}
            emptyTitle="No Docker hosts"
            emptyDescription="Waiting for synthetic observations"
            showToolbar={false}
          />
        }
      >
        <ProxmoxNodesTable
          nodes={state.resources().filter((value) => value.type === 'agent')}
          guests={[]}
          emptyIcon={<span />}
          emptyTitle="No nodes"
          emptyDescription="Waiting for synthetic observations"
        />
      </Show>
    </main>
  );
}
render(
  () => (
    <WebSocketContext.Provider
      value={
        {
          // Browser-supplied, synthetic alert observations. This exercises the
          // same consuming surface without a live agent or induced alert.
          activeAlerts:
            (window as Window & { __hostDiskAlerts?: WebSocketStore['activeAlerts'] })
              .__hostDiskAlerts ?? {},
          state: { pmg: [] },
          connected: () => false,
        } as unknown as WebSocketStore
      }
    >
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <Router>
          <Route path="*" component={Fixture} />
        </Router>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
