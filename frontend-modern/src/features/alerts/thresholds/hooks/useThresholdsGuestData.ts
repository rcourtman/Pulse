import { createMemo } from 'solid-js';

import {
  getGuestOverrideIdentity,
  guestOverrideIdCandidates,
} from '@/features/alerts/guestOverrideIdentity';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';

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
    guestsGroupedByNode,
    guestsFlat,
    guestGroupHeaderMeta,
  };
}
