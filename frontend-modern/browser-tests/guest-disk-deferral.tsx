// Production row, disk tooltip, list and disclosed Overview with synthetic
// same-VM observations. No collector, QGA, native backup or release proof.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawerOverview } from '../src/components/Workloads/GuestDrawerOverview';
import { DiskList } from '../src/components/Workloads/DiskList';
import type { VM } from '../src/types/api';
import { DarkModeContext } from '../src/contexts/appRuntime';
import '../src/index.css';

function Fixture() {
  const [observation, setObservation] = createSignal({
    reason: 'prev-vm-locked',
    data: true,
    usage: 50,
  });
  const [expanded, setExpanded] = createSignal(false);
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const guest = (): VM => ({
    id: 'fixture-pve:pve-a:101',
    vmid: 101,
    name: 'backup-guest',
    node: 'pve-a',
    instance: 'fixture-pve',
    status: 'running',
    type: 'qemu',
    cpu: 0.1,
    cpus: 2,
    memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
    disk: {
      total: 10 * 1024 ** 3,
      used: observation().data ? (observation().usage / 100) * 10 * 1024 ** 3 : 0,
      usage: observation().data ? observation().usage : -1,
    },
    disks: observation().data
      ? [
          {
            mountpoint: '/data',
            type: 'ext4',
            total: 10 * 1024 ** 3,
            used: (observation().usage / 100) * 10 * 1024 ** 3,
            usage: observation().usage,
          },
        ]
      : [],
    diskStatusReason: observation().reason,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: observation().reason.endsWith('vm-locked') ? 'backup' : '',
    backupInProgress: observation().reason.endsWith('vm-locked'),
    lastSeen: '2026-10-03T14:00:00Z',
  });
  (window as any).__guestDiskEvidence = {
    update: setObservation,
    mode: setMode,
    identity: () => guest().id,
  };
  return (
    <main class="min-h-screen space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest disk observation</h1>
      <table class="w-full table-fixed">
        <tbody>
          <GuestRow
            guest={guest()}
            visibleColumnIds={['name', 'disk']}
            onClick={() => setExpanded(!expanded())}
            isExpanded={expanded()}
            metricDisplayMode={mode()}
            workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
          />
        </tbody>
      </table>
      <Show when={expanded()}>
        <section aria-label="Guest Overview">
          <GuestDrawerOverview
            guest={guest()}
            guestOsSummary="Linux"
            agentHeading="Guest agent"
            agentLabel=""
            agentTitle=""
            hasAgentInfo={false}
            hasFilesystemDetails={guest().disks!.length > 0}
            hasNetworkInterfaces={false}
            hasOsInfo={false}
            hasWorkloadActionAgent={false}
            showInGuestAgentInstallCue={false}
            ipAddresses={[]}
            networkInterfaces={[]}
            normalizedTags={[]}
            backupPresentation={null}
            workloadActionAgentTitle=""
          />
        </section>
      </Show>
      <section aria-label="Guest disk list" class="max-w-xl">
        <h2 class="text-sm font-medium">Filesystems</h2>
        <DiskList disks={guest().disks!} diskStatusReason={guest().diskStatusReason} />
      </section>
    </main>
  );
}

render(
  () => (
    <Router>
      <Route
        path="*"
        component={() => (
          <DarkModeContext.Provider
            value={() => document.documentElement.classList.contains('dark')}
          >
            <Fixture />
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
