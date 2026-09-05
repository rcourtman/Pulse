// Browser fixture: real backup component and router; synthetic HTTP polling.
// Deliberately not a substitute for the full application/WebSocket path.
import { render } from 'solid-js/web';
import { createSignal, onMount, onCleanup } from 'solid-js';
import { Router, Route } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

function Fixture() {
  const [workloads, setWorkloads] = createSignal<Resource[]>([]);
  onMount(() => {
    const poll = async () => setWorkloads(await (await fetch('/fixture/workloads')).json());
    void poll();
    const timer = setInterval(() => void poll(), 1000);
    onCleanup(() => clearInterval(timer));
  });
  return (
    <main style={{ padding: '24px', height: '100vh', overflow: 'auto' }}>
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads()} />
    </main>
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
