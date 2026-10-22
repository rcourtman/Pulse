import { createSignal, onCleanup } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { DarkModeContext } from '../src/contexts/appRuntime';
import type { Resource } from '../src/types/resource';
import '../src/index.css';
const guest = (id: string, instance: string, node: string, vmid = 100): Resource => ({
  id,
  name: id,
  type: 'vm',
  platformId: instance,
  status: 'online',
  sourceType: 'api',
  proxmox: { vmid, instance, node, nodeDisplayName: 'Shared' },
});
const initial = [
  guest('east-vm', 'east', 'pve-a'),
  guest('east-no-backup', 'east', 'pve-a', 101),
  guest('west-vm', 'west', 'pve-a'),
  guest('east-other-node', 'east', 'pve-b'),
  guest('case-distinct-vm', 'East', 'pve-a'),
];
function Fixture() {
  const [workloads, setWorkloads] = createSignal(initial);
  const replace = (event: Event) => {
    const mode = (event as CustomEvent).detail;
    setWorkloads(
      mode === 'remove'
        ? initial.filter(
            (value) => value.proxmox?.instance !== 'east' || value.proxmox?.node !== 'pve-a',
          )
        : initial.map((value) =>
            value.proxmox?.instance === 'east' && value.proxmox?.node === 'pve-a'
              ? { ...value, proxmox: { ...value.proxmox, nodeDisplayName: 'Renamed' } }
              : value,
          ),
    );
  };
  document.addEventListener('fixture-node-observation', replace);
  onCleanup(() => document.removeEventListener('fixture-node-observation', replace));
  return (
    <main class="min-h-screen space-y-3 bg-base p-3 text-base-content">
      <h1 class="text-lg font-semibold">Proxmox backups</h1>
      <p class="text-xs text-muted">
        Synthetic collected observations. No backup or guest operations.
      </p>
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads()} />
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
