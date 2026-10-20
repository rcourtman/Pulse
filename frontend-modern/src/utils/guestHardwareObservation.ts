import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadPlatformScopes, resolveWorkloadType } from '@/utils/workloads';

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
  return guest.agentStale ? 'The Pulse Agent in this guest stopped reporting.' : undefined;
};
