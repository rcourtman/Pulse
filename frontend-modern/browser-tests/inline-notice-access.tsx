// The canonical producer, both production drawers, recovery panels and Docs
// renderer. Observations and GET replies are synthetic; no guest operation.
import { createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { RolesPanel } from '../src/components/Settings/RolesPanel';
import { UserAssignmentsPanel } from '../src/components/Settings/UserAssignmentsPanel';
import { InlineNotice, type InlineNoticeTone } from '../src/components/shared/InlineNotice';
import Docs from '../src/pages/Docs';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const resource: Resource = {
  id: 'fixture:pve1:101', type: 'vm', name: 'safety-guest', displayName: 'safety-guest',
  platformId: 'fixture', platformType: 'proxmox-pve', platformScopes: ['proxmox-pve'],
  sourceType: 'api', sources: ['proxmox'], status: 'online', lastSeen: 1791885600000,
  cpu: { current: 10 }, memory: { current: 25, total: 4 * 1024 ** 3, used: 1024 ** 3 },
  disk: { current: 50, used: 5 * 1024 ** 3, total: 10 * 1024 ** 3 },
  proxmox: {
    vmid: 101, nodeName: 'pve1', instance: 'fixture', runtimeStatus: 'running',
    guestAgentStatus: 'deferred', diskStatusReason: 'prev-vm-locked', lock: 'backup',
    disks: [{ mountpoint: '/data', filesystem: 'ext4', total: 10 * 1024 ** 3, used: 5 * 1024 ** 3 }],
  },
};

function GuestFixture() {
  const [snapshot, setSnapshot] = createSignal<Resource[]>([resource]);
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0];
  const [open, setOpen] = createSignal(false);
  (window as any).__noticeAccess = {
    evidence: () => ({ canonical: snapshot()[0], projected: guest() }),
    reason: (reason: string) => setSnapshot((previous) => [{
      ...previous[0], proxmox: { ...previous[0].proxmox, diskStatusReason: reason, lock: reason.includes('locked') ? 'backup' : '' },
    }]),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1>Safety notice action access</h1>
      <p class="text-xs text-muted">Synthetic observations; no guest checks or backup actions.</p>
      <Show when={guest()}>
        <table class="w-full table-fixed"><tbody>
          <GuestRow guest={guest()} visibleColumnIds={['name', 'disk']} isExpanded={open()}
            workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
            onClick={() => setOpen(!open())} />
        </tbody></table>
        <Show when={open()}>
          <section aria-label="Workload drawer"><GuestDrawer guest={guest()} onClose={() => setOpen(false)} /></section>
          <section aria-label="Canonical drawer"><ResourceDetailDrawer resource={snapshot()[0]} presentation="table-row" /></section>
        </Show>
      </Show>
    </main>
  );
}

function RecoveryFixture() {
  return <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
    <h1>Notice recovery controls</h1>
    <p class="text-xs text-muted">Synthetic read failures; no access settings are changed.</p>
    <section aria-label="Roles recovery"><RolesPanel /></section>
    <section aria-label="User access recovery"><UserAssignmentsPanel /></section>
    <For each={['warning', 'danger', 'info', 'success'] as InlineNoticeTone[]}>
      {(tone) => <InlineNotice tone={tone} data-testid={`tone-${tone}`} actionHref="/docs/VM_DISK_MONITORING"
        actionLabel="Read the safety guidance before changing a guest's monitoring configuration">
        Wrapping and focus contrast control.
      </InlineNotice>}
    </For>
  </main>;
}

render(() => <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
  <Router>
    <Route path="/docs/*docPath" component={Docs} />
    <Route path="/recovery-fixture" component={RecoveryFixture} />
    <Route path="*" component={GuestFixture} />
  </Router>
</DarkModeContext.Provider>, document.getElementById('root')!);
