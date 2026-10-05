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

const containerStateTitle = (resource: Resource, label: string): string => {
  const state = dockerContainerRunState(resource);
  const health = normalizedToken(resource.docker?.health);
  const exitCode = resource.docker?.exitCode;
  if (state === 'running') {
    if (health === 'unhealthy') return 'Running, but its health check is failing';
    if (health === 'starting') return 'Running, and its health check has not passed yet';
    if (health === 'healthy') return 'Running, and its health check is passing';
    return 'Running, with no health check';
  }
  if (state === 'exited' && typeof exitCode === 'number') {
    return `Exited with code ${exitCode}`;
  }
  if (state === 'restarting') return 'Docker is restarting it';
  if (state === 'oomkilled') return 'Killed after running out of memory';
  if (state === 'dead') return 'Docker failed to stop or remove it';
  if (state === 'removing') return 'Docker is removing it';
  return label;
};

export const getDockerContainerStatePresentation = (
  resource: Resource,
): DockerContainerStatePresentation => {
  const status = mapDockerContainerStatus(resource);
  const tone =
    status.variant === 'danger' || status.variant === 'warning' || status.variant === 'muted'
      ? status.variant
      : null;
  return { label: status.label, title: containerStateTitle(resource, status.label), tone };
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
