// Real router, inventory readers, production tables and CSS. Synthetic data.
import { createSignal, onCleanup, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { identityGuest } from '../src/features/proxmox/__fixtures__/backupIdentity';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

function Fixture() {
  const [workloads, setWorkloads] = createSignal<Resource[]>([
    identityGuest('west-100', 'west', 'pve-10'),
    identityGuest('east-100', 'east', 'pve-1'),
  ]);
  onMount(() => {
    Object.assign(window, { __replaceBackupIdentityWorkloads: setWorkloads });
    onCleanup(() => Reflect.deleteProperty(window, '__replaceBackupIdentityWorkloads'));
  });
  return (
    <main class="p-3 sm:p-6">
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
