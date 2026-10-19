// The real Backups consumer and CSS, with synthetic collected HTTP observations.
import { ErrorBoundary, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const guest = (id: string, instance: string): Resource =>
  ({
    id,
    name: id,
    type: 'vm',
    platformId: instance,
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: Date.parse('2026-10-19T12:00:00Z'),
    proxmox: { vmid: 100, nodeName: 'shared-node', instance },
  }) as Resource;
const east = guest('east-guest', 'east');
const west = guest('west-guest', 'west');

function Fixture() {
  const [ambiguous, setAmbiguous] = createSignal(true);
  return (
    <main class="p-3 sm:p-6">
      <button
        class="mb-3 min-h-11 rounded-md border border-border px-3 text-sm"
        type="button"
        onClick={() => setAmbiguous(!ambiguous())}
      >
        Fixture: {ambiguous() ? 'resolve' : 'restore'} ambiguity
      </button>
      <ErrorBoundary fallback={<div role="alert">Backup view failed</div>}>
        <ProxmoxBackupsTable emptyIcon={<span />} workloads={ambiguous() ? [east, west] : [east]} />
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
