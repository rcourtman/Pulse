// Production guest Overview and scoped resource reader; synthetic HTTP only.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { GuestDrawerOverview } from '../src/components/Workloads/GuestDrawerOverview';
import { eventBus } from '../src/stores/events';
import { setOrgID } from '../src/utils/apiClient';
import { dispatchResourceMetadataChanged } from '../src/utils/resourceMetadataEvents';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

const guest = {
  id: 'vm-table-101',
  canonicalResourceId: 'vm-resource-101',
  agentId: 'agent-101',
  name: 'backup-guest',
  vmid: 101,
  type: 'qemu',
  status: 'running',
  cpus: 2,
  node: 'pve-a',
  instance: 'fixture-a',
  cpu: 0,
  disk: { total: 10_000_000_000, used: 3_000_000_000, usage: 30 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: '2026-10-19T05:00:00Z',
  memory: { total: 4_000_000_000, used: 1_000_000_000, free: 3_000_000_000, usage: 25 },
  disks: [],
  agentKind: 'pulse',
} as WorkloadGuest;

function Fixture() {
  const [shown, setShown] = createSignal(true);
  const [open, setOpen] = createSignal(true);
  return (
    <main class="mx-auto max-w-3xl space-y-3 p-3 text-base-content">
      <h1 class="text-lg font-semibold">Guest disk read recovery</h1>
      <nav aria-label="Synthetic evidence controls" class="flex flex-wrap gap-3 text-xs">
        <button
          type="button"
          onClick={() =>
            dispatchResourceMetadataChanged({ metadataKind: 'guest', metadataId: guest.id })
          }
        >
          Refresh collected inventory
        </button>
        <button type="button" onClick={() => setOpen(!open())}>
          {open() ? 'Close guest details' : 'Reopen guest details'}
        </button>
        <button type="button" onClick={() => setShown(!shown())}>
          {shown() ? 'Hide overview' : 'Show overview'}
        </button>
        <button
          type="button"
          onClick={() => {
            setOrgID('other-org');
            eventBus.emit('org_switched', 'other-org');
          }}
        >
          Switch organisation
        </button>
      </nav>
      <Show when={open()}>
        <div class={shown() ? '' : 'hidden'}>
          <GuestDrawerOverview
            guest={guest}
            guestOsSummary="Debian 13"
            agentHeading="Agent"
            agentLabel="Connected"
            agentTitle="Connected"
            hasAgentInfo={true}
            hasFilesystemDetails={false}
            hasNetworkInterfaces={false}
            hasOsInfo={true}
            hasWorkloadActionAgent={false}
            showInGuestAgentInstallCue={false}
            ipAddresses={[]}
            networkInterfaces={[]}
            normalizedTags={[]}
            backupPresentation={null}
            workloadActionAgentTitle=""
            historyEnabled={shown()}
          />
        </div>
      </Show>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
