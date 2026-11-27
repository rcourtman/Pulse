import type { Resource } from '@/types/resource';
import { hasFailedAttachedAvailabilityCheck } from '@/utils/availabilityProbePresentation';
import {
  dockerComposeMembership,
  getDockerStaleNotice,
  mapDockerContainerStatus,
  type DockerStaleNotice,
} from './dockerPageModel';

export interface DockerComposeApplication {
  project: string;
  hostId: string;
  host?: Resource;
  containers: Resource[];
}

export function buildDockerComposeApplications(hosts: Resource[], containers: Resource[]) {
  const hostBySourceId = new Map(hosts.map((host) => [host.docker?.hostSourceId, host]));
  const projects = new Map<string, DockerComposeApplication>();
  for (const container of containers) {
    const { project } = dockerComposeMembership(container.docker);
    const hostId = container.docker?.hostSourceId;
    if (!hostId || !project) continue;
    const key = JSON.stringify([hostId, project]);
    let app = projects.get(key);
    if (!app) {
      app = { project, hostId, host: hostBySourceId.get(hostId), containers: [] };
      projects.set(key, app);
    }
    app.containers.push(container);
  }
  return [...projects.values()].sort(
    (left, right) =>
      left.project.localeCompare(right.project) || left.hostId.localeCompare(right.hostId),
  );
}

export function getDockerComposeApplicationStaleness(
  app: DockerComposeApplication,
): DockerStaleNotice | null {
  const hostNotice = getDockerStaleNotice(app.host);
  if (hostNotice) return hostNotice;
  for (const container of app.containers) {
    const notice = getDockerStaleNotice(container);
    if (notice) return notice;
  }
  return null;
}

export function composeAttentionCount(app: DockerComposeApplication) {
  return app.containers.filter((container) => {
    const { variant } = mapDockerContainerStatus(container);
    return (
      variant === 'danger' ||
      variant === 'warning' ||
      (container.incidentCount ?? 0) > 0 ||
      hasFailedAttachedAvailabilityCheck(container)
    );
  }).length;
}

export function composeGuestLocation(host?: Resource) {
  if (!host || (host.type !== 'vm' && host.type !== 'system-container')) return undefined;
  if (host.platformType !== 'proxmox-pve' && !host.platformScopes?.includes('proxmox-pve'))
    return undefined;
  const subject = host.type === 'vm' ? 'Proxmox VM' : 'Proxmox LXC';
  const guest = [subject, host.proxmox?.vmid, host.displayName || host.name]
    .filter(Boolean)
    .join(' ');
  const node = host.proxmox?.nodeName || host.proxmox?.node || host.parentName;
  return node ? `${guest} on ${node}` : guest;
}

export interface ComposeActivity {
  id: string;
  time: number;
  container: string;
  message: string;
  lines?: string[];
}

export function composeActivity(containers: Resource[]): ComposeActivity[] {
  const events: ComposeActivity[] = [];
  for (const container of containers) {
    const docker = container.docker;
    if (!docker) continue;
    const record = (stamp: string | undefined, message: string, lines?: string[]) => {
      const time = Date.parse(stamp ?? '');
      if (Number.isFinite(time) && time > 0)
        events.push({
          id: JSON.stringify([container.id, stamp, message]),
          time,
          container: container.name,
          message,
          lines,
        });
    };
    record(docker.startedAt, 'Started');
    record(docker.finishedAt, 'Exited');
    for (const incident of container.incidents ?? []) {
      record(incident.startedAt, incident.summary || incident.code);
    }
    if (docker.failureLog?.lines.length) {
      record(docker.failureLog.capturedAt, 'Failure log captured', docker.failureLog.lines);
    }
  }
  return events.sort((a, b) => b.time - a.time || a.container.localeCompare(b.container));
}
