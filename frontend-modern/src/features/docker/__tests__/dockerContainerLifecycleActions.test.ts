import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  dockerContainerLifecycleCapability,
  getDockerContainerLifecycleDisabledReason,
} from '../dockerContainerLifecycleActions';

const resource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'app-container:docker-host:web',
  name: 'web',
  displayName: 'web',
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'running',
  type: 'app-container',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    agentId: 'agent-1',
    containerId: 'abc123',
    containerState: 'running',
  },
  capabilities: [
    {
      name: 'stop',
      type: 'common',
      platform: 'docker',
      minimumApprovalLevel: 'admin',
    },
    {
      name: 'restart',
      type: 'common',
      platform: 'docker',
      minimumApprovalLevel: 'admin',
    },
  ],
  ...overrides,
});

describe('dockerContainerLifecycleActions', () => {
  it('enables advertised runtime-matched capabilities', () => {
    const running = resource();

    expect(dockerContainerLifecycleCapability(running, 'restart')?.name).toBe('restart');
    expect(getDockerContainerLifecycleDisabledReason(running, 'restart')).toBeUndefined();
    expect(getDockerContainerLifecycleDisabledReason(running, 'stop')).toBeUndefined();
    expect(getDockerContainerLifecycleDisabledReason(running, 'start')).toBe(
      'Container is already running.',
    );
  });

  it('enables start for stopped containers and explains non-running stop/restart states', () => {
    const stopped = resource({
      status: 'offline',
      docker: {
        runtime: 'podman',
        agentId: 'agent-1',
        containerId: 'abc123',
        containerState: 'exited',
      },
      capabilities: [
        {
          name: 'start',
          type: 'common',
          platform: 'podman',
          minimumApprovalLevel: 'admin',
        },
      ],
    });

    expect(getDockerContainerLifecycleDisabledReason(stopped, 'start')).toBeUndefined();
    expect(getDockerContainerLifecycleDisabledReason(stopped, 'restart')).toBe(
      'Container must be running before restart.',
    );
  });

  it('returns clear disabled reasons for missing agent, stale inventory, policy block, unsupported runtime, and missing capability', () => {
    expect(
      getDockerContainerLifecycleDisabledReason(
        resource({ docker: { runtime: 'docker', containerState: 'running' } }),
        'restart',
      ),
    ).toBe('No reporting Pulse agent is attached to this Docker host.');

    expect(
      getDockerContainerLifecycleDisabledReason(
        resource({ sourceStatus: { docker: { status: 'stale' } } }),
        'restart',
      ),
    ).toBe('Docker inventory is stale; refresh inventory before running lifecycle actions.');

    expect(
      getDockerContainerLifecycleDisabledReason(
        resource({
          docker: {
            runtime: 'docker',
            agentId: 'agent-1',
            containerId: 'abc123',
            containerState: 'running',
            security: {
              mutatingCommandsBlocked: true,
              mutatingCommandsBlockedReason: 'daemon authorization plugin blocks mutation',
            },
          },
        }),
        'restart',
      ),
    ).toBe('daemon authorization plugin blocks mutation');

    expect(
      getDockerContainerLifecycleDisabledReason(
        resource({ docker: { runtime: 'containerd', agentId: 'agent-1' } }),
        'restart',
      ),
    ).toBe('containerd is not supported for governed container lifecycle actions.');

    expect(
      getDockerContainerLifecycleDisabledReason(resource({ capabilities: [] }), 'restart'),
    ).toBe(
      'Pulse does not currently advertise a fresh restart command capability for this container.',
    );
  });
});
