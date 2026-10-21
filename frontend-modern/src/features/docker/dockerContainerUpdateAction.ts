import type { DockerContainerUpdateStatus } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { UpdateButtonProps } from '@/components/shared/containerUpdateBadgeModel';
import { asTrimmedString } from '@/utils/stringUtils';
import { dockerContainerRuntimeLabel } from './dockerContainerLifecycleActions';

const STALE_SOURCE_STATUSES = new Set(['stale', 'offline', 'missing']);

// Why Pulse offers no update for a container whose server row does not
// advertise the update capability. The server records a readiness refusal
// only for capabilities it advertised, so a host that stopped reporting
// leaves neither, and the button used to read that silence as available.
const updateNotOfferedReason = (resource: Resource): string | undefined => {
  if (resource.capabilities?.some((capability) => capability.name === 'update')) return undefined;
  const status = (asTrimmedString(resource.sourceStatus?.docker?.status) ?? '').toLowerCase();
  if (STALE_SOURCE_STATUSES.has(status)) {
    return `${dockerContainerRuntimeLabel(resource)} inventory for this host is ${status}.`;
  }
  return 'Pulse does not currently offer an update action for this container.';
};

const updateLastCheckedMillis = (lastChecked: string | undefined): number => {
  if (!lastChecked) return 0;
  const parsed = Date.parse(lastChecked);
  return Number.isFinite(parsed) ? parsed : 0;
};

/** The container's reported image update state, as the update controls read it. */
export const getDockerContainerUpdateStatus = (
  resource: Resource,
): DockerContainerUpdateStatus | undefined => {
  const update = resource.docker?.updateStatus;
  if (!update) return undefined;

  const error = asTrimmedString(update.error);
  if (typeof update.updateAvailable !== 'boolean' && !error) return undefined;

  return {
    updateAvailable: update.updateAvailable === true,
    currentDigest: asTrimmedString(update.currentDigest) || undefined,
    latestDigest: asTrimmedString(update.latestDigest) || undefined,
    lastChecked: updateLastCheckedMillis(update.lastChecked),
    error: error || undefined,
  };
};

/**
 * Everything the shared `UpdateButton` needs for one container, or nothing
 * when the container cannot be addressed for an update. The containers table
 * and the container drawer both build their update control from this, so the
 * two offer the same action with the same readiness. The host resource, where
 * the caller has it, names the host and its agent's command access in the
 * explanation shown when Pulse cannot apply the update.
 */
export const getDockerContainerUpdateAction = (
  resource: Resource,
  host?: Resource,
): UpdateButtonProps | undefined => {
  const updateStatus = getDockerContainerUpdateStatus(resource);
  const agentId = asTrimmedString(resource.docker?.hostSourceId);
  const containerId = asTrimmedString(resource.docker?.containerId);
  if (!updateStatus || !agentId || !containerId) return undefined;

  return {
    agentId,
    containerId,
    containerName:
      asTrimmedString(resource.name) ||
      asTrimmedString(resource.displayName) ||
      asTrimmedString(resource.docker?.displayName) ||
      containerId,
    updateStatus,
    resourceId: resource.id,
    actionReadiness: resource.actionReadiness,
    updateNotOfferedReason: updateNotOfferedReason(resource),
    hostName: asTrimmedString(host?.displayName) || asTrimmedString(host?.name) || undefined,
    // The agent facet omits commandsEnabled when false, so its presence is
    // what makes an absent flag mean "installed without command access".
    hostCommandsEnabled: host?.agent ? host.agent.commandsEnabled === true : undefined,
  };
};
