import type { WorkloadGuest } from '@/types/workloads';
import {
  getWorkloadPlatformScopes,
  resolveWorkloadType,
  resolveWorkloadTypeFromString,
} from '@/utils/workloads';

type GuestNetworkEvidence = Pick<
  WorkloadGuest,
  | 'type'
  | 'workloadType'
  | 'platformType'
  | 'platformScopes'
  | 'status'
  | 'agentStale'
  | 'networkInterfacesFromAgent'
>;

// Interface identity and cumulative counters survive optional-enrichment and
// power-only updates. Keep them useful without presenting them as a current
// connection. A reporting lease, online resource or recent poll cannot renew
// in-guest evidence while Proxmox reports the VM is not executing.
export const getGuestNetworkLastKnownReason = (guest: GuestNetworkEvidence): string | null => {
  const scopes = getWorkloadPlatformScopes(guest);
  const proxmoxVM =
    resolveWorkloadType(guest) === 'vm' &&
    (scopes.includes('proxmox-pve') || (scopes.length === 0 && guest.type === 'qemu'));
  const status = guest.status?.trim().toLowerCase();
  if (proxmoxVM && ['paused', 'suspended', 'stopped', 'offline'].includes(status ?? '')) {
    const power = status === 'offline' ? 'stopped' : status;
    return `Using last known interface details. Proxmox reports this VM ${power}. Retained addresses and counters do not establish a current connection.`;
  }
  const proxmoxContainer =
    (resolveWorkloadTypeFromString(guest.workloadType) ??
      resolveWorkloadTypeFromString(guest.type)) === 'system-container' &&
    (scopes.includes('proxmox-pve') || (scopes.length === 0 && guest.type === 'lxc'));
  // Only an explicitly selected in-container agent list is execution evidence.
  // Host-derived LXC configuration can legitimately survive a stopped guest.
  if (
    proxmoxContainer &&
    guest.networkInterfacesFromAgent === true &&
    ['stopped', 'offline'].includes(status ?? '')
  ) {
    return 'Using last known interface details. Proxmox reports this container stopped. Retained addresses and counters do not establish a current connection.';
  }
  // A stale linked agent cannot invalidate independently supplied platform
  // interfaces (including host-derived LXC configuration). Preserve the
  // existing legacy interpretation only when the producer is unspecified.
  return guest.agentStale && guest.networkInterfacesFromAgent !== false
    ? 'Using last known interface details. The Pulse Agent in this guest stopped reporting.'
    : null;
};
