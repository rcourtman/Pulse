import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { guestOverrideIdCandidates } from '@/features/alerts/guestOverrideIdentity';
import {
  getCanonicalWorkloadId,
  resolveDiscoveryTargetForWorkload,
  resolveWorkloadType,
} from '@/utils/workloads';
import type { AlertThresholdScope } from '@/utils/metricThresholds';

const firstTrimmed = (values: Array<string | null | undefined>): string => {
  for (const value of values) {
    const trimmed = (value || '').trim();
    if (trimmed) return trimmed;
  }
  return '';
};

const dedupeTrimmed = (values: Array<string | null | undefined>): string[] => {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const value of values) {
    const trimmed = (value || '').trim();
    if (!trimmed) continue;
    const key = trimmed.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(trimmed);
  }
  return result;
};

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

export const getWorkloadAlertThresholdScope = (guest: WorkloadGuest): AlertThresholdScope => {
  const type = resolveWorkloadType(guest);
  return type === 'app-container' ? 'docker' : 'guest';
};

export const getWorkloadAlertResourceIdCandidates = (guest: WorkloadGuest): string[] => {
  const type = resolveWorkloadType(guest);
  if (type === 'vm' || type === 'system-container') {
    return guestOverrideIdCandidates(guest);
  }

  if (type === 'app-container') {
    const discoveryTarget = resolveDiscoveryTargetForWorkload(guest);
    const hostCandidates = dedupeTrimmed([
      discoveryTarget?.agentId,
      guest.dockerHostId,
      guest.contextLabel,
      guest.node,
      guest.instance,
    ]);
    const idSegments = guest.id.split(/[:/]/).filter(Boolean);
    const shortId = idSegments[idSegments.length - 1] || guest.id;
    const containerIds = dedupeTrimmed([
      discoveryTarget?.resourceId,
      guest.containerId,
      shortId,
      guest.id,
    ]);
    const dockerOverrideIds = hostCandidates.flatMap((hostId) =>
      containerIds.map((containerId) => `docker:${hostId}/${containerId}`),
    );
    return dedupeTrimmed([...dockerOverrideIds, guest.id]);
  }

  return dedupeTrimmed([getCanonicalWorkloadId(guest), guest.id]);
};

export const getWorkloadContainerHostId = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type !== 'app-container') return '';
  return firstTrimmed([guest.dockerHostId, guest.contextLabel, guest.node, guest.instance]);
};

export const workloadHostScopeId = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type === 'pod') return '';
  if (type === 'app-container') return getWorkloadContainerHostId(guest);
  return workloadNodeScopeId(guest);
};

export const getWorkloadHostLabel = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type === 'app-container') {
    return firstTrimmed([guest.contextLabel, guest.node, guest.instance, guest.dockerHostId]);
  }
  return (guest.node || '').trim();
};

export const getWorkloadHostHintCandidates = (guest: WorkloadGuest): string[] => {
  const type = resolveWorkloadType(guest);
  if (type === 'pod') return [];
  if (type === 'app-container') {
    return dedupeTrimmed([guest.dockerHostId, guest.contextLabel, guest.node, guest.instance]);
  }
  return dedupeTrimmed([guest.node, guest.instance, guest.contextLabel]);
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
