// Production API readers, recovery mapper, both tables and shipped Docs.
// All observations are supplied by loopback HTTP fixtures, never a guest.
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { DarkModeContext } from '../src/contexts/appRuntime';
import Docs from '../src/pages/Docs';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const guest: Resource = {
  id: 'fixture-vm',
  name: 'recovery-vm',
  type: 'vm',
  platformId: 'fixture-pve',
  status: 'online',
  sourceType: 'api',
  sources: ['proxmox'],
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 101, node: 'pve-a', instance: 'fixture-pve' },
};
function Fixture() {
  return (
    <main class="min-h-screen space-y-3 bg-base p-3 text-base-content">
      <h1 class="text-lg font-semibold">Proxmox backups</h1>
      <p class="text-xs text-muted">Synthetic observations. No backup or guest operations.</p>
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />
      <a href="/docs/PBS#backups-are-visible-but-coverage-says-unprotected">PBS protection help</a>
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="/docs/*docPath" component={Docs} />
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
