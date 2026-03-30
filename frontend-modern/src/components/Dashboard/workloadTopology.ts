import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import {
  getCanonicalWorkloadId,
  resolveDiscoveryTargetForWorkload,
  resolveWorkloadType,
} from '@/utils/workloads';

export const workloadNodeScopeId = (guest: WorkloadGuest): string =>
  `${(guest.instance || '').trim()}-${(guest.node || '').trim()}`;

export const getKubernetesContextKey = (guest: WorkloadGuest): string => {
  const candidates = [guest.contextLabel, guest.instance, guest.node];
  for (const value of candidates) {
    const trimmed = (value || '').trim();
    if (trimmed.length > 0) {
      return trimmed;
    }
  }
  return '';
};

export const getWorkloadDockerHostId = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type !== 'app-container') return '';
  return (guest.dockerHostId || '').trim();
};

export const getWorkloadContainerHostId = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type !== 'app-container') return '';
  return (guest.dockerHostId || guest.node || guest.instance || '').trim();
};

export const getDiscoveryHostIdForWorkload = (guest: WorkloadGuest): string => {
  return resolveDiscoveryTargetForWorkload(guest)?.agentId || '';
};

export const getDiscoveryResourceIdForWorkload = (guest: WorkloadGuest): string => {
  return resolveDiscoveryTargetForWorkload(guest)?.resourceId || '';
};

export const buildNodeByInstance = (nodes: Node[]): Record<string, Node> => {
  const map: Record<string, Node> = {};
  nodes.forEach((node) => {
    map[node.id] = node;
    const instanceNameKey = `${node.instance}-${node.name}`;
    if (!map[instanceNameKey]) {
      map[instanceNameKey] = node;
    }
  });
  return map;
};

export const buildGuestParentNodeMap = (
  guests: WorkloadGuest[],
  nodeMap: Record<string, Node>,
): Record<string, Node | undefined> => {
  const mapping: Record<string, Node | undefined> = {};

  guests.forEach((guest) => {
    const canonicalGuestId = getCanonicalWorkloadId(guest);

    if (guest.id) {
      const lastDash = guest.id.lastIndexOf('-');
      if (lastDash > 0) {
        const nodeId = guest.id.slice(0, lastDash);
        if (nodeMap[nodeId]) {
          mapping[canonicalGuestId] = nodeMap[nodeId];
          return;
        }
      }
    }

    const compositeKey = `${guest.instance}-${guest.node}`;
    if (nodeMap[compositeKey]) {
      mapping[canonicalGuestId] = nodeMap[compositeKey];
    }
  });

  return mapping;
};
