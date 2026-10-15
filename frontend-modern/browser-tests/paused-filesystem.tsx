import { render } from 'solid-js/web';
import { createSignal, Show, onMount } from 'solid-js';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext } from '@/contexts/appRuntime';
import { useWorkloads } from '@/hooks/useWorkloads';
import { GuestRow } from '@/components/Workloads/GuestRow';
import { GuestDrawer } from '@/components/Workloads/GuestDrawer';
import { ResourceTable } from '@/components/Alerts/ResourceTable';
import { useThresholdsGuestData } from '@/features/alerts/thresholds/hooks/useThresholdsGuestData';
import type { Resource } from '@/types/resource';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import '@/index.css';

// Safe synthetic snapshots exercise the production mapper and consuming surfaces.
// This fixture performs no guest, backup or settings operation.
function Fixture() {
  const [resources, setResources] = createSignal<Resource[]>([]);
  const [open, setOpen] = createSignal(false);
  const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
  const workload = useWorkloads(() => true, { resourceSnapshot: resources });
  const guest = () => workload.workloads()[0];
  const props = {
    allGuests: resources,
    overrides: () => [
      {
        id: 'guest-disk:guest:site-a:105/disk:data-dev-vda1',
        type: 'guestDisk',
        name: '/data',
        node: 'backup-guest',
        thresholds: { disk: 95 },
      },
    ],
    nodes: [],
    guestDefaults: { disk: 85 },
    backupDefaults: () => ({ enabled: false }),
    snapshotDefaults: () => ({ enabled: false }),
  } as unknown as ThresholdsTableProps;
  const thresholds = useThresholdsGuestData({ props, editingId: () => null, searchTerm: () => '' });
  const loadSnapshot = async () => {
    const response = await fetch('/api/proof/resources');
    setResources(await response.json());
    await Promise.resolve();
    await Promise.resolve();
  };
  Object.assign(window, { loadSnapshot, setDisplayMode: setMode });
  onMount(loadSnapshot);
  const noop = () => {};
  return (
    <main class="mx-auto max-w-6xl p-3 space-y-4 bg-surface text-base-content pulse-wide-data-surface">
      <h1 class="text-lg font-semibold">Workloads</h1>
      <Show when={guest()}>
        {(value) => (
          <>
            <table class="w-full table-fixed border border-border">
              <colgroup>
                <col style={{ width: '42%' }} />
                <col style={{ width: '38%' }} />
                <col style={{ width: '20%' }} />
              </colgroup>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Disk</th>
                  <th>Backup</th>
                </tr>
              </thead>
              <tbody>
                <GuestRow
                  guest={value()}
                  visibleColumnIds={['name', 'disk', 'backup']}
                  metricDisplayMode={mode()}
                  isExpanded={open()}
                  workloadTableLayoutMode={
                    matchMedia('(max-width: 639px)').matches ? 'phone' : 'wide'
                  }
                  onClick={() => setOpen((v) => !v)}
                />
                <Show when={open()}>
                  <tr>
                    <td colspan="3" class="p-2 bg-surface-alt">
                      <GuestDrawer guest={value()} onClose={() => setOpen(false)} />
                    </td>
                  </tr>
                </Show>
              </tbody>
            </table>
          </>
        )}
      </Show>
      <section
        aria-label="Guest filesystem alert settings"
        class="border border-border rounded-lg p-2"
      >
        <ResourceTable
          title="Guest filesystems"
          resources={thresholds.guestDisksWithOverrides()}
          columns={['disk']}
          onEdit={noop}
          onSaveEdit={noop}
          onCancelEdit={noop}
          onRemoveOverride={noop}
          editingId={() => null}
          editingThresholds={() => ({})}
          setEditingThresholds={noop}
          formatMetricValue={(_, v) => `${v}%`}
          hasActiveAlert={() => false}
          editingNote={() => ''}
          setEditingNote={noop}
        />
      </section>
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('app')!,
);
