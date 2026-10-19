// Real router/table/API reader/styles; all HTTP evidence is synthetic.
import { ErrorBoundary, Show, createSignal, onCleanup, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { eventBus } from '../src/stores/events';
import { setOrgID } from '../src/utils/apiClient';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const guest = {
  id: 'ct-112',
  type: 'system-container',
  name: 'backup-guest',
  platformId: 'pve-a',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.parse('2026-10-03T12:00:00Z'),
  proxmox: { vmid: 112, node: 'pve-a', instance: 'pve-a' },
} as Resource;
const server = {
  id: 'pbs-main',
  type: 'pbs',
  name: 'pbs-main',
  platformId: 'pbs-main',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.parse('2026-10-03T12:00:00Z'),
  pbs: {
    instanceId: 'pbs-main',
    connectionHealth: 'healthy',
    datastores: [{ name: 'main', total: 10_000, used: 4_000, available: 6_000 }],
  },
} as Resource;

function Fixture() {
  const [visible, setVisible] = createSignal(true);
  onMount(() => {
    Object.assign(window, {
      __switchBackupOrg: (orgID: string) => {
        setOrgID(orgID);
        eventBus.emit('org_switched', orgID);
      },
    });
    onCleanup(() => Reflect.deleteProperty(window, '__switchBackupOrg'));
  });
  return (
    <main class="p-3 sm:p-6">
      <button
        class="mb-3 min-h-11 rounded-md border border-border px-3 text-sm"
        type="button"
        onClick={() => setVisible(!visible())}
      >
        Fixture: {visible() ? 'hide' : 'show'} backup view
      </button>
      <Show when={visible()}>
        <ErrorBoundary fallback={<div role="alert">Backup view failed</div>}>
          <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} servers={[server]} />
        </ErrorBoundary>
      </Show>
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
