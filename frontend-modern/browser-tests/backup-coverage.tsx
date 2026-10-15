// Synthetic owning snapshots through the production mapper, row and full drawer.
// These controls never query a guest or change a monitoring integration.
import { createSignal, For, Show } from 'solid-js';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { renderInAppRuntime } from './fixtureRuntime';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const common: Resource = {
  id: 'coverage:fixture',
  name: 'Integration workload',
  type: 'vm',
  status: 'online',
  sourceType: 'api',
  sources: ['vmware'],
  platformScopes: ['vmware-vsphere'],
  cpu: { current: 10 },
  memory: { current: 25, used: 256 * 1024 ** 2, total: 1024 ** 3 },
};
const pve: Resource = {
  ...common,
  sources: ['proxmox'],
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 101, nodeName: 'pve-a', runtimeStatus: 'running' },
};
const completed = '2026-10-15T19:00:00Z';
const cases: Record<string, Resource> = {
  vsphere: common,
  docker: { ...common, type: 'app-container', sources: ['docker'], platformScopes: ['docker'] },
  pod: { ...common, type: 'pod', sources: ['kubernetes'], platformScopes: ['kubernetes'] },
  incus: { ...common, type: 'system-container', sources: ['incus'], platformScopes: ['incus'] },
  'pve-missing': pve,
  'pve-fresh': { ...pve, proxmox: { ...pve.proxmox, lastBackup: completed } },
  'pve-running': { ...pve, proxmox: { ...pve.proxmox, backupInProgress: true } },
  'pve-deferred': {
    ...pve,
    proxmox: {
      ...pve.proxmox,
      lastBackup: completed,
      lock: 'backup',
      diskStatusReason: 'prev-vm-locked',
      guestAgentStatus: 'deferred',
    },
  },
};

function Fixture() {
  const [selection, setSelection] = createSignal('vsphere');
  const state = useWorkloads(() => true, {
    resourceSnapshot: () => [cases[selection()]],
  });
  const guest = () => state.workloads()[0];
  const [expanded, setExpanded] = createSignal(false);
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Workloads</h1>
      <p class="text-xs text-muted">Synthetic observations. No guest operations.</p>
      <label class="block text-sm">
        Monitoring integration{' '}
        <select
          aria-label="Monitoring integration"
          class="min-h-11 rounded-sm border border-border bg-surface px-2"
          value={selection()}
          onChange={(event) => setSelection(event.currentTarget.value)}
        >
          <For each={Object.keys(cases)}>{(name) => <option value={name}>{name}</option>}</For>
        </select>
      </label>
      <Show when={guest()}>
        {(value) => (
          <table class="w-full table-fixed">
            <colgroup>
              <col style={{ width: '70%' }} />
              <col style={{ width: '30%' }} />
            </colgroup>
            <thead>
              <tr>
                <th scope="col">Name</th>
                <th scope="col">Backup</th>
              </tr>
            </thead>
            <tbody>
              <GuestRow
                guest={value()}
                visibleColumnIds={['name', 'backup']}
                workloadTableLayoutMode={
                  matchMedia('(max-width: 639px)').matches ? 'phone' : 'wide'
                }
                isExpanded={expanded()}
                onClick={() => setExpanded(!expanded())}
              />
              <Show when={expanded()}>
                <tr>
                  <td colspan="2" class="bg-surface-alt p-2">
                    <GuestDrawer guest={value()} onClose={() => setExpanded(false)} />
                  </td>
                </tr>
              </Show>
            </tbody>
          </table>
        )}
      </Show>
    </main>
  );
}
renderInAppRuntime(Fixture);
