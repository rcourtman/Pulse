// Production row, full drawer, column sizing and CSS with synthetic observations.
// This is presentation/input evidence, not collection, native thaw or delivery.
import { createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { buildSummaryDisclosureControlsId } from '../src/components/shared/summaryInteractionA11y';
import {
  GUEST_COLUMNS,
  getGuestColumnWidthStyle,
  getWorkloadTableLayoutModeForContainer,
  getWorkloadVisibleColumnsForLayout,
} from '../src/components/Workloads/guestRowModel';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { WorkloadGuest } from '../src/types/workloads';
import type { WorkloadMetricHistoryReader } from '../src/components/Workloads/workloadMetricHistoryModel';
import { guestDiskDeferrals } from '../src/components/Workloads/__fixtures__/guestDiskDeferrals';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });

function Fixture() {
  const [observation, setObservation] = createSignal({
    reason: 'prev-vm-locked',
    data: true,
    usage: 50,
    lock: 'backup',
    kind: 'qemu',
  });
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const [expanded, setExpanded] = createSignal(false);
  const [actionCount, setActionCount] = createSignal(0);
  const layout = getWorkloadTableLayoutModeForContainer(window.innerWidth - 24);
  const requested = new Set(['name', 'cpu', 'memory', 'disk', 'uptime', 'backup']);
  const columns = getWorkloadVisibleColumnsForLayout(
    GUEST_COLUMNS.filter((column) => requested.has(column.id)),
    layout,
  );
  const columnIds = columns.map((column) => column.id);
  const toggle = () => {
    setActionCount((count) => count + 1);
    setExpanded((value) => !value);
  };
  const disk = () =>
    observation().data
      ? {
          total: 10 * 1024 ** 3,
          used: (observation().usage / 100) * 10 * 1024 ** 3,
          usage: observation().usage,
        }
      : { total: 0, used: 0, usage: -1 };
  const guest = (): WorkloadGuest => ({
    id: 'fixture-pve:pve-a:101',
    vmid: 101,
    name: 'backup-guest',
    node: 'pve-a',
    instance: 'fixture-pve',
    status: 'running',
    type: observation().kind,
    cpu: 0.1,
    cpus: 2,
    memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
    disk: disk(),
    disks: observation().data ? [{ ...disk(), mountpoint: '/data', type: 'ext4' }] : [],
    diskStatusReason: observation().reason,
    lock: observation().lock,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    backupInProgress: Boolean(observation().lock),
    tags: [],
    lastSeen: '2026-10-04T01:00:00Z',
  });
  const control = (): WorkloadGuest => ({
    ...guest(),
    id: 'fixture-pve:pve-a:102',
    vmid: 102,
    name: 'fresh-control',
    diskStatusReason: '',
    lock: '',
    backupInProgress: false,
  });
  const history: WorkloadMetricHistoryReader = {
    getGuestMetricSeries: (_guest, metric) => [
      {
        id: metric,
        label: metric,
        color: '#10b981',
        points: [
          { timestamp: 1791075540000, value: 25 },
          { timestamp: 1791075600000, value: 50 },
        ],
      },
    ],
    getNodeMetricSeries: () => [],
  };
  (window as any).__guestDiskProvenance = {
    apply: (next: Partial<ReturnType<typeof observation>>) =>
      setObservation((previous) => ({ ...previous, ...next })),
    mode: setMode,
    expanded,
    actionCount,
    layout,
    columnIds,
    deferrals: guestDiskDeferrals,
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Guest filesystem read provenance</h1>
      <p class="text-xs text-muted">
        Synthetic observations. No backup or guest-agent command is sent.
      </p>
      <div class="flex gap-2">
        <button
          class="rounded-sm border border-border p-2 text-xs"
          onClick={() => setMode((value) => (value === 'bars' ? 'sparklines' : 'bars'))}
        >
          Switch display mode
        </button>
        <button
          class="rounded-sm border border-border p-2 text-xs"
          onClick={() =>
            setObservation((previous) => ({
              ...previous,
              reason: '',
              lock: '',
              data: true,
              usage: 75,
            }))
          }
        >
          Mark observation fresh
        </button>
      </div>
      <div class="table-scroll-shell overflow-x-auto">
        <table class="w-full platform-table workload-table table-fixed">
          <colgroup>
            <For each={columnIds}>
              {(id) => (
                <col
                  style={getGuestColumnWidthStyle(id, window.innerWidth <= 768, layout, columnIds)}
                />
              )}
            </For>
          </colgroup>
          <thead>
            <tr>
              <For each={columns}>{(column) => <th scope="col">{column.label}</th>}</For>
            </tr>
          </thead>
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={columnIds}
              workloadTableLayoutMode={layout}
              metricDisplayMode={mode()}
              metricHistory={history}
              onClick={toggle}
              isExpanded={expanded()}
            />
            <GuestRow
              guest={control()}
              visibleColumnIds={columnIds}
              workloadTableLayoutMode={layout}
              metricDisplayMode={mode()}
              metricHistory={history}
            />
          </tbody>
        </table>
      </div>
      <Show when={expanded()}>
        <section
          id={buildSummaryDisclosureControlsId(guest().id)}
          aria-label="Guest details"
          class="mx-auto max-w-4xl"
        >
          <GuestDrawer guest={guest()} onClose={() => setExpanded(false)} />
        </section>
      </Show>
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
