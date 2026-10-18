import type { DockerContainerUpdateStatus } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { UpdateButtonProps } from '@/components/shared/containerUpdateBadgeModel';
import { asTrimmedString } from '@/utils/stringUtils';

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
 * two offer the same action with the same readiness.
 */
export const getDockerContainerUpdateAction = (
  resource: Resource,
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
  };
};
