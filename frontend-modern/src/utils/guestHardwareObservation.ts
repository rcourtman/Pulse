import type { WorkloadGuest } from '@/types/workloads';
import {
  getWorkloadPlatformScopes,
  resolveWorkloadType,
  resolveWorkloadTypeFromString,
} from '@/utils/workloads';

type GuestHardwareEvidence = Pick<
  WorkloadGuest,
  'type' | 'workloadType' | 'platformType' | 'platformScopes' | 'status' | 'agentStale'
>;

// A native power update can precede expiry of the in-guest agent's lease and
// its separately queried disks. Qualify that retained report without rewriting
// collection state, health or timestamps, or treating QGA state as Pulse state.
export const getGuestHardwareLastKnownReason = (
  guest: GuestHardwareEvidence,
): string | undefined => {
  const scopes = getWorkloadPlatformScopes(guest);
  const proxmoxVM =
    resolveWorkloadType(guest) === 'vm' &&
    (scopes.includes('proxmox-pve') || (scopes.length === 0 && guest.type === 'qemu'));
  const status = guest.status?.trim().toLowerCase();
  if (proxmoxVM && ['paused', 'suspended', 'stopped', 'offline'].includes(status ?? '')) {
    return `Proxmox reports this VM ${status === 'offline' ? 'stopped' : status}.`;
  }
  // LXC configuration can remain visible on its host while the container is
  // stopped, but RAID/SMART from its own agent cannot be a current report.
  const proxmoxContainer =
    (resolveWorkloadTypeFromString(guest.workloadType) ??
      resolveWorkloadTypeFromString(guest.type)) === 'system-container' &&
    (scopes.includes('proxmox-pve') || (scopes.length === 0 && guest.type === 'lxc'));
  if (proxmoxContainer && ['stopped', 'offline'].includes(status ?? '')) {
    return 'Proxmox reports this container stopped.';
  }
  return guest.agentStale ? 'The Pulse Agent in this guest stopped reporting.' : undefined;
};
