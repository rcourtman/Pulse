import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  getDockerContainerStatePresentation,
  getDockerContainerUptimeSeconds,
} from '../dockerContainerStatePresentation';

const container = (docker: NonNullable<Resource['docker']>, status = 'online'): Resource =>
  ({
    id: 'app-container-1',
    type: 'app-container',
    name: 'web',
    status,
    docker,
  }) as Resource;

describe('getDockerContainerStatePresentation', () => {
  it('reads a running container plainly and names its health check on hover', () => {
    expect(
      getDockerContainerStatePresentation(
        container({ containerState: 'running', health: 'healthy' }),
      ),
    ).toEqual({
      label: 'Healthy',
      title: 'Running, and its health check is passing',
      tone: null,
    });
    expect(getDockerContainerStatePresentation(container({ containerState: 'running' }))).toEqual({
      label: 'Running',
      title: 'Running, with no health check',
      tone: null,
    });
  });

  it('flags a failing or starting health check on a running container', () => {
    expect(
      getDockerContainerStatePresentation(
        container({ containerState: 'running', health: 'unhealthy' }),
      ),
    ).toEqual({
      label: 'Unhealthy',
      title: 'Running, but its health check is failing',
      tone: 'danger',
    });
    expect(
      getDockerContainerStatePresentation(
        container({ containerState: 'running', health: 'starting' }),
      ),
    ).toEqual({
      label: 'Starting',
      title: 'Running, and its health check has not passed yet',
      tone: 'warning',
    });
  });

  it('shows the exit code of a container that crashed, and mutes a clean stop', () => {
    expect(
      getDockerContainerStatePresentation(container({ containerState: 'exited', exitCode: 139 })),
    ).toEqual({ label: 'Exited (139)', title: 'Exited with code 139', tone: 'danger' });
    expect(
      getDockerContainerStatePresentation(container({ containerState: 'exited', exitCode: 0 })),
    ).toEqual({ label: 'Exited', title: 'Exited with code 0', tone: 'muted' });
    expect(getDockerContainerStatePresentation(container({ containerState: 'paused' }))).toEqual({
      label: 'Paused',
      title: 'Paused',
      tone: 'muted',
    });
  });

  it('says what a dead container means on hover', () => {
    expect(getDockerContainerStatePresentation(container({ containerState: 'dead' }))).toEqual({
      label: 'Dead',
      title: 'Docker failed to stop or remove it',
      tone: 'danger',
    });
  });

  it('marks a restarting container for attention', () => {
    expect(
      getDockerContainerStatePresentation(container({ containerState: 'restarting' })),
    ).toEqual({ label: 'Restarting', title: 'Docker is restarting it', tone: 'warning' });
  });
});

describe('a container whose agent reported no Docker state', () => {
  it('reads the online resource status as running, health check and uptime included', () => {
    expect(
      getDockerContainerStatePresentation(container({ health: 'unhealthy' }, 'online')),
    ).toEqual({
      label: 'Unhealthy',
      title: 'Running, but its health check is failing',
      tone: 'danger',
    });
    expect(getDockerContainerStatePresentation(container({}, 'online'))).toEqual({
      label: 'Running',
      title: 'Running, with no health check',
      tone: null,
    });
    expect(getDockerContainerUptimeSeconds(container({ uptimeSeconds: 120 }, 'online'))).toBe(120);
  });
});

describe('getDockerContainerUptimeSeconds', () => {
  it('reports the current run of a running or paused container', () => {
    expect(
      getDockerContainerUptimeSeconds(container({ containerState: 'running', uptimeSeconds: 8 })),
    ).toBe(8);
    expect(
      getDockerContainerUptimeSeconds(container({ containerState: 'paused', uptimeSeconds: 3600 })),
    ).toBe(3600);
  });

  it('drops the leftover uptime Docker keeps on a stopped container', () => {
    expect(
      getDockerContainerUptimeSeconds(
        container({ containerState: 'exited', exitCode: 1, uptimeSeconds: 184082 }),
      ),
    ).toBeUndefined();
    expect(
      getDockerContainerUptimeSeconds(container({ containerState: 'running', uptimeSeconds: 0 })),
    ).toBeUndefined();
  });
});
