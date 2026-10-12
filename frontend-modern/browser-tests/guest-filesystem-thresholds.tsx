// Real alert row projection and desktop/mobile editors with synthetic observations.
// No guest command, backup, alert delivery or installed acceptance is exercised.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ResourceTable } from '../src/components/Alerts/ResourceTable';
import { useThresholdsGuestData } from '../src/features/alerts/thresholds/hooks/useThresholdsGuestData';
import type { ThresholdsTableProps } from '../src/features/alerts/thresholds/types';
import type { Resource } from '../src/types/resource';
import { guestDiskDeferrals } from '../src/components/Workloads/__fixtures__/guestDiskDeferrals';
import '../src/index.css';

function Fixture() {
  const [observation, setObservation] = createSignal({
    reason: 'prev-vm-locked',
    power: 'running',
    type: 'vm' as Resource['type'],
    data: true,
    usage: 90,
  });
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<
    Record<string, number | undefined>
  >({});
  const [editingNote, setEditingNote] = createSignal('');
  const [actions, setActions] = createSignal<string[]>([]);
  const guest = (instance: string, name: string, control = false): Resource => ({
    id: `${instance}:pve-a:101`,
    name,
    type: control ? 'vm' : observation().type,
    status: 'online',
    platformId: instance,
    platformType: 'proxmox-pve',
    platformScopes: ['proxmox-pve'],
    sourceType: 'api',
    lastSeen: 1,
    proxmox: {
      instance,
      node: 'pve-a',
      vmid: 101,
      runtimeStatus: control ? 'running' : observation().power,
      diskStatusReason: control ? '' : observation().reason,
      disks:
        control || observation().data
          ? [
              {
                mountpoint: '/data',
                device: '/dev/vda1',
                type: 'ext4',
                total: 10 * 1024 ** 3,
                used: (control ? 0.3 : observation().usage / 100) * 10 * 1024 ** 3,
                usage: control ? 30 : observation().usage,
              },
            ]
          : [],
    },
  });
  const data = useThresholdsGuestData({
    props: {
      allGuests: () => [
        guest('fixture-a', 'backup-guest'),
        guest('fixture-b', 'fresh-control', true),
      ],
      overrides: () => [
        {
          id: 'guest-disk:guest:fixture-a:101/disk:data-dev-vda1',
          name: '/data',
          node: 'backup-guest',
          type: 'guestDisk',
          thresholds: { disk: 95 },
        },
      ],
      nodes: [],
      guestDefaults: { disk: 85 },
      backupDefaults: () => ({ enabled: false }),
      snapshotDefaults: () => ({ enabled: false }),
    } as unknown as ThresholdsTableProps,
    editingId,
    searchTerm: () => '',
  });
  (window as any).__guestFilesystemThresholds = {
    apply: (next: Partial<ReturnType<typeof observation>>) =>
      setObservation((previous) => ({ ...previous, ...next })),
    rows: data.guestDisksWithOverrides,
    actions,
    deferrals: guestDiskDeferrals,
  };
  return (
    <main class="min-h-screen bg-surface p-3 text-base-content">
      <h1 class="mb-2 text-base font-semibold">Guest filesystem alert settings</h1>
      <p class="mb-3 text-xs text-muted">Synthetic observations; no guest or backup operation.</p>
      <ResourceTable
        title="Guest Filesystems"
        groupedResources={data.guestDisksGroupedByGuest()}
        columns={['Disk %']}
        editingId={editingId}
        editingThresholds={editingThresholds}
        setEditingThresholds={setEditingThresholds}
        editingNote={editingNote}
        setEditingNote={setEditingNote}
        formatMetricValue={(_metric, value) => (value === undefined ? '—' : `${value}%`)}
        hasActiveAlert={() => false}
        onEdit={(id, thresholds, _defaults, note) => {
          setEditingId(id);
          setEditingThresholds(thresholds);
          setEditingNote(note ?? '');
          setActions((previous) => [...previous, `edit:${id}`]);
        }}
        onSaveEdit={(id) => {
          setActions((previous) => [...previous, `save:${id}`]);
          setEditingId(null);
        }}
        onCancelEdit={() => {
          setActions((previous) => [...previous, 'cancel']);
          setEditingId(null);
        }}
        onRemoveOverride={(id) => setActions((previous) => [...previous, `remove:${id}`])}
      />
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
