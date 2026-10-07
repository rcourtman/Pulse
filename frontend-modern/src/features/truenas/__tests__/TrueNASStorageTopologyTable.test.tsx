import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { eventBus } from '@/stores/events';
import type { AlertConfig } from '@/types/alerts';

import {
  TrueNASStorageTopologyTable,
  getTrueNASStorageTopologyIndentClass,
} from '@/features/truenas/TrueNASStorageTopologyTable';
import type { Resource } from '@/types/resource';

const makeStorageResource = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'storage',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'api',
    storage: { topology: 'dataset', platform: 'truenas' },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASStorageTopologyTable', () => {
  it('renders nested dataset depth with distinct row indentation', () => {
    const pool = makeStorageResource({
      id: 'pool-tank',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', path: 'tank' },
    });
    const media = makeStorageResource({
      id: 'dataset-media',
      name: 'tank/media',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const photos = makeStorageResource({
      id: 'dataset-photos',
      name: 'tank/media/photos',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos' },
    });
    const raw = makeStorageResource({
      id: 'dataset-raw',
      name: 'tank/media/photos/raw',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos/raw' },
    });
    const resources = [pool, raw, media, photos];

    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));

    const mediaRow = container.querySelector('[data-truenas-storage-row="dataset:dataset-media"]');
    const photosRow = container.querySelector(
      '[data-truenas-storage-row="dataset:dataset-photos"]',
    );
    const rawRow = container.querySelector('[data-truenas-storage-row="dataset:dataset-raw"]');
    const headers = [...container.querySelectorAll('thead th')];

    expect(headers[0]).toHaveClass('platform-table-name-column', 'platform-table-mobile-w-30');
    expect(headers[1]).toHaveClass('platform-table-mobile-w-15');
    expect(headers[1]).not.toHaveClass('hidden');
    expect(headers[2]).toHaveClass('platform-table-mobile-w-25');
    expect(headers[3]).toHaveClass('platform-table-mobile-w-15');
    expect(headers[3]).not.toHaveClass('hidden');
    expect(headers[5]).toHaveClass('platform-table-phone-hidden');

    expect(mediaRow).toHaveAttribute('data-truenas-storage-depth', '1');
    expect(photosRow).toHaveAttribute('data-truenas-storage-depth', '2');
    expect(rawRow).toHaveAttribute('data-truenas-storage-depth', '3');
    expect(
      mediaRow
        ?.querySelector('[data-truenas-storage-indent-depth="1"]')
        ?.classList.contains('pl-3'),
    ).toBe(true);
    expect(
      photosRow
        ?.querySelector('[data-truenas-storage-indent-depth="2"]')
        ?.classList.contains('pl-6'),
    ).toBe(true);
    expect(
      rawRow?.querySelector('[data-truenas-storage-indent-depth="3"]')?.classList.contains('pl-8'),
    ).toBe(true);
  });

  it('caps deep indentation at the table-safe depth class', () => {
    expect(getTrueNASStorageTopologyIndentClass(0)).toBe('');
    expect(getTrueNASStorageTopologyIndentClass(1)).toBe('pl-3 sm:pl-7');
    expect(getTrueNASStorageTopologyIndentClass(2)).toBe('pl-6 sm:pl-11');
    expect(getTrueNASStorageTopologyIndentClass(3)).toBe('pl-8 sm:pl-16');
    expect(getTrueNASStorageTopologyIndentClass(8)).toBe('pl-8 sm:pl-16');
  });

  it('separates volumes and physical disks with an accessible one-click scope', async () => {
    const pool = makeStorageResource({
      id: 'pool-tank',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', path: 'tank' },
    });
    const dataset = makeStorageResource({
      id: 'dataset-media',
      name: 'tank/media',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const disk = makeStorageResource({
      id: 'disk-sda',
      type: 'physical_disk',
      name: 'sda',
      storage: undefined,
      physicalDisk: { devPath: '/dev/sda', serial: 'SERIAL-A', diskType: 'ssd', wearout: 68 },
    });
    const resources = [pool, dataset, disk];
    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
      />
    ));

    expect(screen.getByRole('group', { name: 'Storage type' })).toBeInTheDocument();
    expect(container.querySelectorAll('[data-truenas-storage-row]')).toHaveLength(3);

    await fireEvent.click(screen.getByRole('button', { name: 'Physical disks, 1' }));
    expect(screen.getByRole('button', { name: 'Physical disks, 1' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(container.querySelectorAll('[data-truenas-storage-row]')).toHaveLength(1);
    expect(container.querySelector('[data-truenas-storage-kind="disk"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-storage-kind="pool"]')).toBeNull();
    expect(screen.getByRole('columnheader', { name: /Endurance/ })).toBeInTheDocument();
    expect(screen.getByText('68% left')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /Temp/ })).toHaveClass('table-cell');
    // The status dot and reason line carry health on phones, so the column
    // stays desktop-only even when it is the disk view's main signal.
    expect(screen.getByRole('columnheader', { name: /Health/ })).toHaveClass(
      'platform-table-phone-hidden',
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Volumes, 2' }));
    expect(container.querySelectorAll('[data-truenas-storage-row]')).toHaveLength(2);
    expect(container.querySelector('[data-truenas-storage-kind="pool"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-storage-kind="dataset"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-storage-kind="disk"]')).toBeNull();
  });

  it('shows health only for exceptions, with the reason in the Health cell', () => {
    const pool = makeStorageResource({
      id: 'pool-archive',
      name: 'archive',
      status: 'warning',
      incidents: [
        {
          code: 'truenas_volume_status',
          severity: 'warning',
          summary: 'Pool archive is DEGRADED: one member of mirror-0 is faulted.',
        },
        {
          code: 'truenas_smart',
          severity: 'warning',
          summary: 'Device /dev/sdc has SMART test failures.',
        },
      ],
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'DEGRADED' },
    });
    const healthy = makeStorageResource({
      id: 'dataset-backups',
      name: 'archive/backups',
      parentId: 'pool-archive',
      storage: { topology: 'dataset', platform: 'truenas' },
    });
    const resources = [pool, healthy];
    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));

    const poolRow = container.querySelector('[data-truenas-storage-resource="pool-archive"]');
    const healthyRow = container.querySelector('[data-truenas-storage-resource="dataset-backups"]');
    const health = poolRow?.querySelector('[data-truenas-storage-health="attention"]');
    const visible = [...(health?.querySelectorAll('[aria-hidden="true"]') ?? [])].map(
      (node) => node.textContent,
    );

    // The reason sits in the Health cell on the row's single line. The name
    // cell carries only the name, per the shared platform-table rhythm.
    expect(health?.closest('td')).toHaveClass('platform-table-phone-hidden');
    expect(visible).toEqual(['Pool archive is DEGRADED: one member of mirror-0 is faulted.', '+1']);
    expect(health?.querySelector('.sr-only')).toHaveTextContent(
      'Attention: Pool archive is DEGRADED: one member of mirror-0 is faulted. Device /dev/sdc has SMART test failures.',
    );
    expect(health).toHaveAttribute(
      'title',
      'Pool archive is DEGRADED: one member of mirror-0 is faulted.\nDevice /dev/sdc has SMART test failures.',
    );
    expect(poolRow?.querySelector('td')).not.toHaveTextContent('DEGRADED');
    expect(healthyRow?.querySelector('[data-truenas-storage-health]')).toBeNull();
    expect(healthyRow).not.toHaveTextContent('Healthy');
  });

  it('flags a disk running hot by its type alert trigger', () => {
    // Disk risk carries no heat, so the table judges it with the alerts
    // store's per-type thresholds (factory here: SATA 55C, NVMe 70C).
    const disk = (id: string, diskType: string, temperature: number) =>
      makeStorageResource({
        id,
        type: 'physical_disk',
        name: id,
        storage: undefined,
        physicalDisk: { devPath: `/dev/${id}`, serial: `serial-${id}`, diskType, temperature },
      });
    const resources = [disk('sda', 'sata', 56), disk('nvme0n1', 'nvme', 63)];
    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));

    const hotRow = container.querySelector('[data-truenas-storage-resource="sda"]');
    expect(hotRow?.querySelector('[data-truenas-storage-health="attention"]')).toHaveAttribute(
      'title',
      'Disk temperature is 56°C, at or above its 55°C alert threshold.',
    );
    const warmRow = container.querySelector('[data-truenas-storage-resource="nvme0n1"]');
    expect(warmRow?.querySelector('[data-truenas-storage-health]')).toBeNull();
    // Phones hide the Health cell, so the status dot carries the heat too.
    expect(hotRow?.querySelector('[title="Warning"]')).not.toBeNull();
    expect(warmRow?.querySelector('[title="Online"]')).not.toBeNull();
    // The hot disk sorts ahead of its cooler sibling.
    expect(
      [...container.querySelectorAll('[data-truenas-storage-resource]')].map((row) =>
        row.getAttribute('data-truenas-storage-resource'),
      ),
    ).toEqual(['sda', 'nvme0n1']);
  });

  it('shows a retained disk temperature as last known and sorts it as no reading', async () => {
    // A TrueNAS disk merged with a host agent's row keeps the agent's last
    // reading after the agent stops reporting, under an unavailable state.
    const disk = (id: string, temperature: number, state: 'available' | 'unavailable') =>
      makeStorageResource({
        id,
        type: 'physical_disk',
        name: id,
        storage: undefined,
        physicalDisk: {
          devPath: `/dev/${id}`,
          serial: `serial-${id}`,
          diskType: 'sata',
          temperature,
          collection: {
            temperature: { state, source: 'agent', reason: 'host agent stopped reporting' },
          },
        },
      });
    const resources = [
      disk('sda', 64, 'unavailable'),
      disk('sdb', 41, 'available'),
      disk('sdc', 38, 'available'),
    ];
    const { container } = render(() => (
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));
    const row = (id: string) => container.querySelector(`[data-truenas-storage-resource="${id}"]`);
    const order = () =>
      [...container.querySelectorAll('[data-truenas-storage-resource]')].map((element) =>
        element.getAttribute('data-truenas-storage-resource'),
      );

    try {
      const retained = row('sda')?.querySelector('[data-temperature-reading="last-known"]');
      expect(retained).toHaveTextContent('64.0°C, last known');
      expect(retained).toHaveAttribute(
        'title',
        'Last known reading, not current: host agent stopped reporting',
      );
      expect(retained).toHaveClass('text-muted');
      // 64C is over the factory SATA trigger, but a retained reading is not heat.
      expect(row('sda')?.querySelector('[data-truenas-storage-health]')).toBeNull();
      expect(row('sdb')?.querySelector('[data-temperature-reading="last-known"]')).toBeNull();
      expect(row('sdb')).toHaveTextContent('41.0°C');

      // Hottest first ranks current readings only; the retained one stays last.
      await fireEvent.click(screen.getByRole('columnheader', { name: /Temp/ }));
      expect(order()).toEqual(['sdb', 'sdc', 'sda']);
      await fireEvent.click(screen.getByRole('columnheader', { name: /Temp/ }));
      expect(order()).toEqual(['sdc', 'sdb', 'sda']);
    } finally {
      window.localStorage.clear();
    }
  });

  it('follows a disk temperature trigger the user raised in Alerts', async () => {
    const getConfig = vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue({
      enabled: true,
      activationState: 'active',
      agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
      diskTempByType: {
        nvme: { trigger: 70, clear: 65 },
        sas: { trigger: 65, clear: 60 },
        sata: { trigger: 60, clear: 55 },
      },
    } as unknown as AlertConfig);
    try {
      const resources = [
        makeStorageResource({
          id: 'sda',
          type: 'physical_disk',
          name: 'sda',
          storage: undefined,
          physicalDisk: {
            devPath: '/dev/sda',
            serial: 'serial-sda',
            diskType: 'sata',
            temperature: 56,
          },
        }),
      ];
      const { container } = render(() => (
        <TrueNASStorageTopologyTable
          resources={resources}
          scope={resources}
          emptyIcon={<span />}
          emptyTitle="No storage"
          emptyDescription="No storage"
        />
      ));
      const health = () =>
        container.querySelector(
          '[data-truenas-storage-resource="sda"] [data-truenas-storage-health]',
        );
      expect(health()).toHaveAttribute('data-truenas-storage-health', 'attention');
      expect(screen.getByRole('button', { name: /Attention/ })).toHaveTextContent('1');
      // With the Attention filter on, the storage-type counts judge heat too.
      await fireEvent.click(screen.getByRole('button', { name: /Attention/ }));
      expect(screen.getByRole('button', { name: 'Physical disks, 1' })).toBeInTheDocument();

      await useAlertsActivation().refreshConfig();
      await waitFor(() =>
        expect(screen.getByRole('button', { name: 'Physical disks, 0' })).toBeInTheDocument(),
      );
      // The status filter counts follow the same thresholds.
      expect(screen.getByRole('button', { name: /Attention/ })).toHaveTextContent('0');
    } finally {
      getConfig.mockRestore();
      eventBus.emit('org_switched', 'default');
    }
  });
});
