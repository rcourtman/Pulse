// Real backup consumer and HTTP readers. Every fleet observation is synthetic.
import { Show, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const workloads: Resource[] = Array.from({ length: 2001 }, (_, index) => ({
  id: `vm:${String(index).padStart(4, '0')}`,
  name: index === 0 || index === 2000 ? `edge-backup-reader-${index}` : `fleet-${index}`,
  type: 'vm',
  platformId: 'synthetic-pve',
  status: 'online',
  sourceType: 'api',
  sources: ['proxmox'],
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid: index + 100, node: 'pve-a', instance: 'synthetic-pve' },
}));
function Fixture() {
  const [visible, setVisible] = createSignal(true);
  return (
    <main class="min-h-screen space-y-3 bg-base p-3 text-base-content">
      <h1 class="text-lg font-semibold">Proxmox backups</h1>
      <p class="text-xs text-muted">Synthetic 2,001-guest fleet. No guest operations.</p>
      <button
        type="button"
        class="min-h-11 rounded-md border border-border px-3 text-sm"
        onClick={() => setVisible(!visible())}
      >
        Fixture: {visible() ? 'hide' : 'show'} backup view
      </button>
      <Show when={visible()}>
        <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads} />
      </Show>
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
