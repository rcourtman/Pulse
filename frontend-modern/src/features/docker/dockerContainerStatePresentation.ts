import type { Resource } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

import { dockerContainerRunState, mapDockerContainerStatus } from './dockerPageModel';

// The State column says what `docker ps` would flag beside the status: a
// failing or starting health check, a non-zero exit code, a restart. It reuses
// the row's status-dot verdict so the dot and the words never disagree. Only a
// problem or a stopped container carries a tone; a running one reads plainly.
export type DockerContainerStateTone = 'danger' | 'warning' | 'muted';

export interface DockerContainerStatePresentation {
  label: string;
  title: string;
  tone: DockerContainerStateTone | null;
}

const normalizedToken = (value: unknown): string =>
  (asTrimmedString(value) ?? '').toLowerCase().replace(/[\s_-]/g, '');

const presentationTone = (variant: string): DockerContainerStateTone | null =>
  variant === 'danger' || variant === 'warning' || variant === 'muted' ? variant : null;

const containerStateTitle = (resource: Resource, label: string): string => {
  const state = dockerContainerRunState(resource);
  const health = normalizedToken(resource.docker?.health);
  if (state === 'running') {
    if (health === 'unhealthy') return 'Running, but its health check is failing';
    if (health === 'starting') return 'Running, and its health check has not passed yet';
    if (health === 'healthy') return 'Running, and its health check is passing';
    return 'Running, with no health check';
  }
  return runStateTitle(resource) ?? label;
};

// Docker's own state words, for a surface that names the health check on a
// row of its own (the resource drawer).
const runStateTitle = (resource: Resource): string | undefined => {
  const state = dockerContainerRunState(resource);
  const exitCode = resource.docker?.exitCode;
  if (state === 'exited' && typeof exitCode === 'number') {
    return `Exited with code ${exitCode}`;
  }
  if (state === 'restarting') return 'Docker is restarting it';
  if (state === 'oomkilled') return 'Killed after running out of memory';
  if (state === 'dead') return 'Docker failed to stop or remove it';
  if (state === 'removing') return 'Docker is removing it';
  return undefined;
};

// v5 flagged crash-loopers in the restarts column; a container that restarted
// more than this many times needs an operator's eye even while "running".
export const DOCKER_RESTART_ATTENTION_THRESHOLD = 5;

// flagRestarts: the row shows no Restarts column (narrow layouts), so a
// running container past the restart threshold says so in its State cell.
export const getDockerContainerStatePresentation = (
  resource: Resource,
  options: { flagRestarts?: boolean } = {},
): DockerContainerStatePresentation => {
  const status = mapDockerContainerStatus(resource);
  const tone = presentationTone(status.variant);
  const restarts = resource.docker?.restartCount;
  if (
    options.flagRestarts &&
    tone === null &&
    typeof restarts === 'number' &&
    restarts > DOCKER_RESTART_ATTENTION_THRESHOLD
  ) {
    return {
      label: `${restarts} restarts`,
      title: `Running, but Docker has restarted it ${restarts} times`,
      tone: 'warning',
    };
  }
  return { label: status.label, title: containerStateTitle(resource, status.label), tone };
};

// The container's state without its health check (Running, Exited (139),
// Paused): the drawer shows the health check on its own row.
export const getDockerContainerRunStatePresentation = (
  resource: Resource,
): DockerContainerStatePresentation => {
  const status = mapDockerContainerStatus({
    ...resource,
    docker: { ...resource.docker, health: undefined },
  });
  return {
    label: status.label,
    title: runStateTitle(resource) ?? status.label,
    tone: presentationTone(status.variant),
  };
};

// The health check of a running container. Docker keeps the last result on a
// stopped container (#1724), so a stopped one has none to show.
export const getDockerContainerHealthCheckPresentation = (
  resource: Resource,
): DockerContainerStatePresentation | undefined => {
  const state = dockerContainerRunState(resource);
  if (state && state !== 'running') return undefined;
  const health = normalizedToken(resource.docker?.health);
  if (health === 'unhealthy') {
    return { label: 'Failing', title: 'Its health check is failing', tone: 'danger' };
  }
  if (health === 'starting') {
    return { label: 'Starting', title: 'Its health check has not passed yet', tone: 'warning' };
  }
  if (health === 'healthy') {
    return { label: 'Passing', title: 'Its health check is passing', tone: null };
  }
  return undefined;
};

// Uptime is only the current run: Docker keeps the last value on a stopped
// container, which would read as time spent running.
const UPTIME_STATES = new Set(['running', 'paused']);

export const getDockerContainerUptimeSeconds = (resource: Resource): number | undefined => {
  const state = dockerContainerRunState(resource);
  const seconds = resource.docker?.uptimeSeconds;
  if (!UPTIME_STATES.has(state)) return undefined;
  return typeof seconds === 'number' && Number.isFinite(seconds) && seconds > 0
    ? seconds
    : undefined;
};
