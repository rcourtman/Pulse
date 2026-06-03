import type { WorkloadGuest } from '@/types/workloads';
import { isGuestRunning } from '@/utils/status';
import { resolveWorkloadType } from '@/utils/workloads';

const isOciSystemContainer = (guest: WorkloadGuest): boolean =>
  guest.type === 'oci-container' || (guest as { isOci?: boolean }).isOci === true;

export const hasInGuestPulseAgent = (guest: WorkloadGuest): boolean =>
  (guest.agentVersion || '').trim().length > 0;

export const isInGuestPulseAgentEligibleWorkload = (guest: WorkloadGuest): boolean => {
  if ((guest as { template?: boolean }).template === true) return false;

  const workloadType = resolveWorkloadType(guest);
  if (workloadType === 'vm') return true;
  if (workloadType === 'system-container') return !isOciSystemContainer(guest);
  return false;
};

export const shouldShowInGuestAgentInstallCue = (
  guest: WorkloadGuest,
  parentNodeOnline = true,
): boolean =>
  isInGuestPulseAgentEligibleWorkload(guest) &&
  !hasInGuestPulseAgent(guest) &&
  isGuestRunning(guest, parentNodeOnline);

export const IN_GUEST_AGENT_INSTALL_SUMMARY_LABEL = 'Install agent';
export const IN_GUEST_AGENT_INSTALL_ACTION_LABEL = 'Add agent for AI actions';
export const IN_GUEST_AGENT_INSTALL_TITLE =
  'Install Pulse Agent inside this guest to unlock deep telemetry and AI actions.';
