// Real Backups consumer, production HTTP readers and shipped CSS. No guest operations.
import { ErrorBoundary } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const guest = {
  id: 'east-100',
  name: 'east-guest',
  type: 'vm',
  platformId: 'east',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.parse('2026-10-19T12:00:00Z'),
  proxmox: { vmid: 100, nodeName: 'pve-1', instance: 'east' },
} as Resource;
function Fixture() {
  return (
    <main class="p-3 sm:p-6">
      <ErrorBoundary fallback={<div role="alert">Backup view failed</div>}>
        <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />
      </ErrorBoundary>
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
