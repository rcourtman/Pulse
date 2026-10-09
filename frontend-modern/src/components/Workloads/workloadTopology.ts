import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { guestOverrideIdCandidates } from '@/features/alerts/guestOverrideIdentity';
import {
  getCanonicalWorkloadId,
  getWorkloadPlatformScopes,
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

// UI scope, not a collector/metric ID. Encode each component before joining:
// `lab-east` / `pve1` and `lab` / `east-pve1` are different parents.
export const workloadNodeScopeId = (guest: Pick<WorkloadGuest, 'instance' | 'node'>): string => {
  const instance = (guest.instance || '').trim();
  const node = (guest.node || '').trim();
  return instance || node ? `node|${encodeURIComponent(instance)}|${encodeURIComponent(node)}` : '';
};

export const readWorkloadNodeScope = (scope: string): { instance: string; node: string } | null => {
  const parts = scope.split('|');
  if (parts.length !== 3 || parts[0] !== 'node') return null;
  try {
    const result = { instance: decodeURIComponent(parts[1]), node: decodeURIComponent(parts[2]) };
    return workloadNodeScopeId(result) === scope ? result : null;
  } catch {
    return null;
  }
};

// Old bookmarks remain usable only when their lossy alias resolves uniquely.
// Never use this alias for grouping or parent attribution.
export const legacyWorkloadNodeScopeId = (scope: string): string => {
  const node = readWorkloadNodeScope(scope);
  return node ? `${node.instance}-${node.node}` : scope;
};

export const resolveWorkloadHostScope = (guests: WorkloadGuest[], scope: string): string | null => {
  const exact = guests.find((guest) => workloadHostScopeId(guest) === scope);
  if (exact) return scope;
  const candidates = new Set(
    guests
      .map(workloadHostScopeId)
      .filter((candidate) => legacyWorkloadNodeScopeId(candidate) === scope),
  );
  return candidates.size === 1 ? [...candidates][0] : null;
};

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

/**
 * Tags the alert engine reads for this workload's guest thresholds. It reads
 * Pulse control tags such as pulse-relaxed only from Proxmox VMs and LXCs, so
 * a vSphere VM or a container keeps its thresholds whatever it is tagged.
 * Rows without platform scopes are the legacy Proxmox shape.
 */
export const getWorkloadAlertPolicyTags = (guest: WorkloadGuest): string[] => {
  const type = resolveWorkloadType(guest);
  if (type !== 'vm' && type !== 'system-container') return [];
  const platformScopes = getWorkloadPlatformScopes(guest);
  if (platformScopes.length > 0 && !platformScopes.includes('proxmox-pve')) return [];
  const tags = guest.tags;
  if (Array.isArray(tags)) return tags;
  // Legacy rows carry the raw tag string: Proxmox separates with ';',
  // older Pulse payloads with ','.
  return typeof tags === 'string' ? tags.split(/[;,]/) : [];
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
  const map: Record<string, Node> = Object.create(null);
  const ambiguous = new Set<string>();
  const add = (key: string, node: Node) => {
    if (!key || ambiguous.has(key)) return;
    if (map[key] && map[key] !== node) {
      delete map[key];
      ambiguous.add(key);
    } else {
      map[key] = node;
    }
  };
  nodes.forEach((node) => {
    add(node.id, node);
    add(workloadNodeScopeId({ instance: node.instance, node: node.name }), node);
  });
  return map;
};

export const buildGuestParentNodeMap = (
  guests: WorkloadGuest[],
  nodeMap: Record<string, Node>,
): Record<string, Node | undefined> => {
  const mapping: Record<string, Node | undefined> = Object.create(null);

  guests.forEach((guest) => {
    const canonicalGuestId = getCanonicalWorkloadId(guest);

    const type = resolveWorkloadType(guest);
    if (type !== 'vm' && type !== 'system-container') return;
    // A vSphere VM's matching labels are not evidence of a Proxmox parent.
    const scopes = getWorkloadPlatformScopes(guest);
    if (scopes.length > 0 && !scopes.includes('proxmox-pve')) return;
    const parent = nodeMap[workloadNodeScopeId(guest)];
    if (parent) {
      mapping[canonicalGuestId] = parent;
      return;
    }

    // Legacy IDs may supply an otherwise missing parent, but must not
    // contradict source-owned instance/node fields or resolve an ID collision.
    if (guest.id) {
      const lastDash = guest.id.lastIndexOf('-');
      if (
        lastDash > 0 &&
        Number.isSafeInteger(guest.vmid) &&
        guest.vmid > 0 &&
        guest.id.slice(lastDash + 1) === String(guest.vmid)
      ) {
        const nodeId = guest.id.slice(0, lastDash);
        const candidate = nodeMap[nodeId];
        if (
          candidate &&
          (!guest.instance?.trim() || guest.instance.trim() === candidate.instance?.trim()) &&
          (!guest.node?.trim() || guest.node.trim() === candidate.name?.trim())
        ) {
          mapping[canonicalGuestId] = candidate;
        }
      }
    }
  });

  return mapping;
};

export const buildGuestParentNodeMapFromNodes = (
  guests: WorkloadGuest[],
  nodes: Node[],
): Record<string, Node | undefined> => buildGuestParentNodeMap(guests, buildNodeByInstance(nodes));

// The existing charts API consumes native node IDs, not UI scope tokens. If a
// native ID is shared, omit its first-match filter rather than choosing another
// node. The bounded summary then keeps its existing per-resource chart keys.
export const getWorkloadHistoryNodeId = (scope: string, nodes?: Node[]): string | undefined => {
  const tuple = readWorkloadNodeScope(scope);
  if (!tuple) return scope || undefined;
  if (!nodes) return legacyWorkloadNodeScopeId(scope);
  const matches = nodes.filter(
    (node) => workloadNodeScopeId({ instance: node.instance, node: node.name }) === scope,
  );
  if (matches.length !== 1 || !matches[0].id) return undefined;
  const id = matches[0].id;
  return nodes.filter((node) => node.id === id).length === 1 ? id : undefined;
};
