import type { Resource, ResourceCapability } from '@/types/resource';
import { getActionReadinessRefusal } from '@/utils/actionReadiness';
import { asTrimmedString } from '@/utils/stringUtils';

export type DockerContainerLifecycleAction = 'start' | 'stop' | 'restart';

export type DockerContainerLifecycleActionSpec = {
  action: DockerContainerLifecycleAction;
  label: string;
  activeLabel: string;
};

export const DOCKER_CONTAINER_LIFECYCLE_ACTIONS: readonly DockerContainerLifecycleActionSpec[] = [
  { action: 'start', label: 'Start', activeLabel: 'Starting' },
  { action: 'stop', label: 'Stop', activeLabel: 'Stopping' },
  { action: 'restart', label: 'Restart', activeLabel: 'Restarting' },
] as const;

const SUPPORTED_RUNTIMES = new Set(['docker', 'podman']);
const STARTABLE_STATES = new Set(['created', 'exited', 'dead', 'stopped']);
const STALE_SOURCE_STATUSES = new Set(['stale', 'offline', 'missing']);

const normalizeToken = (value: unknown): string => (asTrimmedString(value) ?? '').toLowerCase();

export const isDockerContainerLifecycleResource = (resource: Resource): boolean => {
  if (resource.type !== 'app-container') return false;
  const runtime = normalizeToken(resource.docker?.runtime);
  const hasDockerSource =
    normalizeToken(resource.platformType) === 'docker' ||
    (resource.sources ?? []).some((source) => normalizeToken(source) === 'docker') ||
    (resource.platformScopes ?? []).some((scope) => normalizeToken(scope) === 'docker');
  return hasDockerSource && (runtime === 'docker' || runtime === 'podman');
};

export const dockerContainerLifecycleName = (resource: Resource): string =>
  (asTrimmedString(resource.name) ||
    asTrimmedString(resource.displayName) ||
    asTrimmedString(resource.docker?.displayName) ||
    asTrimmedString(resource.docker?.containerId)) ??
  resource.id;

export const dockerContainerRuntimeLabel = (resource: Resource): string => {
  const runtime = normalizeToken(resource.docker?.runtime);
  if (runtime === 'podman') return 'Podman';
  if (runtime === 'docker') return 'Docker';
  return asTrimmedString(resource.docker?.runtime) ?? 'Container runtime';
};

export const dockerContainerLifecycleCapability = (
  resource: Resource,
  action: DockerContainerLifecycleAction,
): ResourceCapability | undefined => {
  const runtime = normalizeToken(resource.docker?.runtime);
  return resource.capabilities?.find((capability) => {
    if (capability.name !== action) return false;
    const platform = normalizeToken(capability.platform);
    return !platform || !runtime || platform === runtime;
  });
};

const sourceStatusDisabledReason = (
  resource: Resource,
  runtimeLabel: string,
): string | undefined => {
  const dockerStatus = normalizeToken(resource.sourceStatus?.docker?.status);
  if (STALE_SOURCE_STATUSES.has(dockerStatus)) {
    return `${runtimeLabel} inventory is ${dockerStatus}; refresh inventory before running lifecycle actions.`;
  }
  const dockerError = asTrimmedString(resource.sourceStatus?.docker?.error);
  if (dockerError) return `${runtimeLabel} inventory is not healthy: ${dockerError}`;
  if (!resource.lastSeen || resource.lastSeen <= 0) {
    return `${runtimeLabel} inventory has not reported a valid last-seen timestamp.`;
  }
  return undefined;
};

const stateDisabledReason = (
  resource: Resource,
  action: DockerContainerLifecycleAction,
): string | undefined => {
  const state = normalizeToken(resource.docker?.containerState || resource.status);
  if (action === 'start') {
    if (state === 'running') return 'Container is already running.';
    if (STARTABLE_STATES.has(state)) return undefined;
    return state ? `Container state ${state} is not startable.` : 'Container state is unknown.';
  }
  if (state !== 'running') {
    return state ? `Container must be running before ${action}.` : 'Container state is unknown.';
  }
  return undefined;
};

const actionReadinessDisabledReason = (
  resource: Resource,
  action: DockerContainerLifecycleAction,
): string | undefined => getActionReadinessRefusal(resource.actionReadiness, action);

export const getDockerContainerLifecycleDisabledReason = (
  resource: Resource,
  action: DockerContainerLifecycleAction,
): string | undefined => {
  const runtime = normalizeToken(resource.docker?.runtime);
  const runtimeLabel = dockerContainerRuntimeLabel(resource);
  if (runtime && !SUPPORTED_RUNTIMES.has(runtime)) {
    return `${runtimeLabel} is not supported for governed container lifecycle actions.`;
  }
  if (!runtime) return 'Container runtime is not reported.';

  const agentId = asTrimmedString(resource.docker?.agentId);
  if (!agentId) return `No reporting Pulse agent is attached to this ${runtimeLabel} host.`;

  const sourceReason = sourceStatusDisabledReason(resource, runtimeLabel);
  if (sourceReason) return sourceReason;

  const security = resource.docker?.security;
  if (security?.mutatingCommandsBlocked) {
    return (
      asTrimmedString(security.mutatingCommandsBlockedReason) ??
      `${runtimeLabel} host policy blocks mutating container lifecycle commands.`
    );
  }

  const stateReason = stateDisabledReason(resource, action);
  if (stateReason) return stateReason;

  const readinessReason = actionReadinessDisabledReason(resource, action);
  if (readinessReason) return readinessReason;

  if (!dockerContainerLifecycleCapability(resource, action)) {
    return `Pulse does not currently advertise a fresh ${action} command capability for this container.`;
  }

  return undefined;
};
