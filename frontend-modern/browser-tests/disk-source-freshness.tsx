// Synthetic surrounding state; production HTTP owner, consumers and CSS.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { DiskList } from '../src/components/Storage/DiskList';
import { GuestDrawerOverview } from '../src/components/Workloads/GuestDrawerOverview';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import { dispatchResourceMetadataChanged } from '../src/utils/resourceMetadataEvents';
import type { WorkloadGuest } from '../src/types/workloads';
import type { StorageHealthFilter } from '../src/features/storageBackups/models';
import '../src/index.css';

const guest = {
  id: 'guest-101',
  canonicalResourceId: 'guest-101',
  agentId: 'agent-101',
  name: 'backup-guest',
  type: 'qemu',
  status: 'running',
  node: 'pve-a',
  agentKind: 'pulse',
  memory: { total: 4e9, used: 1e9, free: 3e9, usage: 25 },
  disks: [],
  tags: [],
} as unknown as WorkloadGuest;

function Fixture() {
  const [view, setView] = createSignal('storage');
  const [selected, setSelected] = createSignal<string | null>(null);
  const [health, setHealth] = createSignal<StorageHealthFilter>('all');
  const source = useUnifiedResources({
    query: 'type=physical_disk&parent=guest-101',
    cacheKey: 'guest-physical-disks:guest-101',
    reportBackgroundErrors: true,
    readTimeoutMs: 20_000,
  });
  return (
    <main class="mx-auto max-w-5xl space-y-3 p-3 text-base-content">
      <h1 class="text-lg font-semibold">Disk source freshness</h1>
      <nav aria-label="Synthetic fixture controls" class="flex flex-wrap gap-3 text-xs">
        <button
          type="button"
          onClick={() =>
            dispatchResourceMetadataChanged({ metadataKind: 'guest', metadataId: guest.id })
          }
        >
          Refresh collected inventory
        </button>
        <button type="button" onClick={() => setView('storage')}>
          Storage disks
        </button>
        <button type="button" onClick={() => setView('guest')}>
          Guest overview
        </button>
        <button type="button" onClick={() => setHealth(health() === 'all' ? 'healthy' : 'all')}>
          Toggle healthy filter
        </button>
      </nav>
      <Show
        when={view() === 'storage'}
        fallback={
          <GuestDrawerOverview
            guest={guest}
            guestOsSummary="Debian 13"
            agentHeading="Pulse Agent"
            agentLabel="Observed"
            agentTitle="Synthetic agent context"
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
          />
        }
      >
        <section aria-label="Storage disk inventory">
          <DiskList
            disks={source.resources()}
            nodes={[]}
            selectedNode={null}
            searchTerm=""
            healthFilter={health()}
            selectedDiskId={selected()}
            onSelectedDiskChange={setSelected}
          />
        </section>
      </Show>
    </main>
  );
}
render(() => <Fixture />, document.getElementById('root')!);
