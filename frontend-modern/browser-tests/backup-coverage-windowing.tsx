// Safe synthetic HTTP observations through the real inventory/posture readers.
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const workloads: Resource[] = Array.from({ length: 200 }, (_, index) => ({
  id: `fixture-${index}`,
  name: `workload-${String(index).padStart(3, '0')}`,
  type: 'vm',
  platformId: 'fixture-pve',
  status: 'online',
  sourceType: 'api',
  sources: ['proxmox'],
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 100 + index, node: 'pve-a', instance: 'fixture-pve' },
}));
function Fixture() {
  return (
    <main class="min-h-screen space-y-3 bg-base p-3 text-base-content">
      <h1 class="text-lg font-semibold">Proxmox backups</h1>
      <p class="text-xs text-muted">Synthetic inventory. No guest or backup operations.</p>
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads} />
    </main>
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
