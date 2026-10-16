// Synthetic workload observations feed the production recovery model and table.
// Loopback HTTP is the only data source; no guest or provider operation exists.
import { createSignal, onMount, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { DarkModeContext } from '../src/contexts/appRuntime';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

function Fixture() {
  const [workloads, setWorkloads] = createSignal<Resource[]>([]);
  const [ready, setReady] = createSignal(false);
  const load = async (fleet = false) => {
    const response = await fetch(`/fixture/workloads${fleet ? '?fleet=1' : ''}`);
    setWorkloads(await response.json());
    setReady(true);
  };
  onMount(() => void load());
  return (
    <main class="min-h-screen space-y-3 bg-base p-3 text-base-content">
      <h1 class="text-lg font-semibold">Proxmox backups</h1>
      <p class="text-xs text-muted">Synthetic observations. No backup or guest operations.</p>
      <div class="flex flex-wrap gap-2 text-xs">
        <button onClick={() => void load()}>Refresh workload observations</button>
        <button onClick={() => void load(true)}>Load fleet observations</button>
      </div>
      <Show when={ready()}>
        <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads()} />
      </Show>
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
