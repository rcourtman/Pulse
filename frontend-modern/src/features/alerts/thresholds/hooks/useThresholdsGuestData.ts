import { createMemo } from 'solid-js';

import {
  getGuestOverrideIdentity,
  guestDiskAlertResourceId,
  guestDiskOverrideIdCandidates,
  guestDiskOverrideStorageId,
  guestOverrideIdCandidates,
} from '@/features/alerts/guestOverrideIdentity';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';
import type { Disk } from '@/types/api';

import type { GroupHeaderMeta, Resource as TableResource } from '../tableTypes';
import { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  buildNodeHeaderMeta,
  createOverridesMap,
  findOverrideByCandidates,
  hasThresholdDiff,
} from '../thresholdsResourceModel';

export function useThresholdsGuestData(inputs: ThresholdsDataInputs) {
  const { props, editingId, searchTerm } = inputs;

  const guestsGroupedByNode = createMemo<Record<string, TableResource[]>>((prev = {}) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());

    const guests = (props.allGuests() ?? []).map((guest) => {
      const guestIdentity = getGuestOverrideIdentity(guest);
      const vmid = guestIdentity?.vmid;
      const node = guestIdentity?.node ?? '';
      const instance = guestIdentity?.instance ?? guest.platformId ?? '';
      const override = findOverrideByCandidates(overridesMap, guestOverrideIdCandidates(guest));
      const overrideSeverity = override?.poweredOffSeverity;
      const hasCustomThresholds = hasThresholdDiff(
        override,
        props.guestDefaults as Record<string, number | undefined>,
      );
      const hasOverride =
        hasCustomThresholds ||
        Boolean(override?.disabled) ||
        Boolean(override?.disableConnectivity) ||
        overrideSeverity !== undefined;

      return {
        id: guest.id,
        name: getAlertResourceDisplayLabel(guest),
        displayName: getAlertResourceDisplayLabel(guest),
        rawName: guest.name,
        type: 'guest' as const,
        resourceType: guest.type === 'vm' ? 'VM' : 'Container',
        vmid,
        node,
        instance,
        status: guest.status,
        hasOverride,
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.guestDefaults,
        backup: override?.backup || props.backupDefaults(),
        snapshot: override?.snapshot || props.snapshotDefaults(),
        poweredOffSeverity: overrideSeverity,
      };
    });

    const filteredGuests = search
      ? guests.filter(
          (guest) =>
            guest.name.toLowerCase().includes(search) ||
            guest.vmid?.toString().includes(search) ||
            guest.node?.toLowerCase().includes(search),
        )
      : guests;

    const grouped: Record<string, TableResource[]> = {};
    filteredGuests.forEach((guest) => {
      const groupKey = guest.instance || guest.node || 'Unknown';
      if (!grouped[groupKey]) {
        grouped[groupKey] = [];
      }
      grouped[groupKey].push(guest);
    });

    Object.keys(grouped).forEach((node) => {
      grouped[node].sort((a, b) => {
        if (a.vmid && b.vmid) return a.vmid - b.vmid;
        return a.name.localeCompare(b.name);
      });
    });

    return grouped;
  }, {});

  const guestsFlat = createMemo<TableResource[]>(() =>
    Object.values(guestsGroupedByNode() ?? {}).flat(),
  );

  const guestDisksWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const seen = new Set<string>();
    const guestDisks: TableResource[] = [];

    (props.allGuests() ?? []).forEach((guest) => {
      const disks = guest.proxmox?.disks ?? [];
      const guestName = getAlertResourceDisplayLabel(guest);
      const guestIdentity = getGuestOverrideIdentity(guest);

      disks.forEach((disk: Disk) => {
        // The unified resources payload omits zero-valued numerics, so
        // total/used/usage may all be absent; usage < 0 is the poller's
        // "unknown" sentinel and stays excluded.
        const total = disk?.total ?? 0;
        const used = disk?.used ?? 0;
        if (!disk || total <= 0 || (typeof disk.usage === 'number' && disk.usage < 0)) return;
        const usagePercent =
          typeof disk.usage === 'number' && Number.isFinite(disk.usage)
            ? disk.usage
            : (used / total) * 100;

        const candidates = guestDiskOverrideIdCandidates(guest, disk.mountpoint, disk.device);
        const storageId = guestDiskOverrideStorageId(guest, disk.mountpoint, disk.device);
        const alertResourceId = guestDiskAlertResourceId(guest, disk.mountpoint, disk.device);
        if (!storageId || !alertResourceId) return;

        const override = findOverrideByCandidates(overridesMap, candidates);
        const label = disk.mountpoint?.trim() || disk.device?.trim() || 'disk';
        const hasCustomThresholds = hasThresholdDiff(override, {
          disk: props.guestDefaults.disk,
        });
        candidates.forEach((candidate) => seen.add(candidate));
        seen.add(storageId);

        guestDisks.push({
          id: alertResourceId,
          overrideIdCandidates: candidates,
          overrideStorageId: storageId,
          name: label,
          displayName: label,
          rawName: disk.device || label,
          type: 'guestDisk' as const,
          resourceType: 'Guest Filesystem',
          host: guest.id,
          node: guestName,
          groupKey: guestIdentity
            ? `${guestName} · ${guestIdentity.instance}/${guestIdentity.vmid}`
            : guestName,
          // Unified payloads carry the filesystem under `filesystem`; the
          // legacy state payload uses `type`.
          instance: disk.filesystem || disk.type || '',
          vmid: guestIdentity?.vmid,
          status: guest.status,
          hasOverride: hasCustomThresholds || Boolean(override?.disabled),
          disabled: override?.disabled || false,
          thresholds: override?.thresholds || {},
          defaults: { disk: props.guestDefaults.disk },
          subtitle: `${(used / 1024 / 1024 / 1024).toFixed(1)} / ${(total / 1024 / 1024 / 1024).toFixed(1)} GB · ${usagePercent.toFixed(1)}%`,
        } satisfies TableResource);
      });
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'guestDisk' && !seen.has(override.id))
      .forEach((override) => {
        const name = override.name || override.id;
        guestDisks.push({
          id: override.id,
          overrideIdCandidates: [override.id],
          overrideStorageId: override.id,
          name,
          displayName: name,
          rawName: name,
          type: 'guestDisk' as const,
          resourceType: 'Guest Filesystem',
          node: override.node || 'Unknown Guest',
          instance: override.instance || '',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          thresholds: override.thresholds || {},
          defaults: { disk: props.guestDefaults.disk },
        });
      });

    return search
      ? guestDisks.filter(
          (disk) =>
            disk.name.toLowerCase().includes(search) ||
            disk.node?.toLowerCase().includes(search) ||
            disk.rawName?.toLowerCase().includes(search),
        )
      : guestDisks;
  }, []);

  const guestDisksGroupedByGuest = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    guestDisksWithOverrides().forEach((disk) => {
      const guestName =
        (typeof disk.groupKey === 'string' && disk.groupKey.trim()) ||
        disk.node?.trim() ||
        'Unknown Guest';
      if (!grouped[guestName]) grouped[guestName] = [];
      grouped[guestName].push(disk);
    });
    Object.values(grouped).forEach((disks) => disks.sort((a, b) => a.name.localeCompare(b.name)));
    return grouped;
  });

  const guestGroupHeaderMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.nodes ?? []).forEach((node) => {
      const { headerMeta, keys } = buildNodeHeaderMeta(node);
      keys.forEach((key: string) => {
        meta[key] = headerMeta;
      });
    });
    return meta;
  });

  return {
    guestDisksGroupedByGuest,
    guestDisksWithOverrides,
    guestsGroupedByNode,
    guestsFlat,
    guestGroupHeaderMeta,
  };
}
