import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getDockerContainerUpdateAction,
  getDockerContainerUpdateStatus,
} from '../dockerContainerUpdateAction';

const container = (docker: Resource['docker']): Resource =>
  ({
    id: 'app-container:edge-web',
    type: 'app-container',
    name: 'edge-web',
    status: 'online',
    actionReadiness: [{ name: 'update', available: true }],
    docker,
  }) as unknown as Resource;

describe('docker container update action', () => {
  it('addresses the update by host and container, with the resource and its readiness', () => {
    const action = getDockerContainerUpdateAction(
      container({
        hostSourceId: ' agent-edge ',
        containerId: 'abc',
        updateStatus: { updateAvailable: true, lastChecked: '2026-10-14T10:00:00Z' },
      }),
    );
    expect(action).toMatchObject({
      agentId: 'agent-edge',
      containerId: 'abc',
      containerName: 'edge-web',
      resourceId: 'app-container:edge-web',
      actionReadiness: [{ name: 'update', available: true }],
      updateStatus: { updateAvailable: true, lastChecked: Date.parse('2026-10-14T10:00:00Z') },
    });
  });

  it("names the host and reads its agent's command access", () => {
    const resource = container({
      hostSourceId: 'agent-tower',
      containerId: 'abc',
      updateStatus: { updateAvailable: true },
    });
    const host = (agent?: Record<string, unknown>) =>
      ({ id: 'agent-tower', type: 'agent', name: 'tower', displayName: 'Tower', agent }) as never;

    // The facet omits commandsEnabled when false: present facet, absent flag.
    expect(getDockerContainerUpdateAction(resource, host({ agentId: 'a' }))).toMatchObject({
      hostName: 'Tower',
      hostCommandsEnabled: false,
    });
    expect(
      getDockerContainerUpdateAction(resource, host({ commandsEnabled: true }))
        ?.hostCommandsEnabled,
    ).toBe(true);
    // A Docker-only host reports no agent facet, so command access is unknown.
    expect(getDockerContainerUpdateAction(resource, host())?.hostCommandsEnabled).toBeUndefined();
    expect(getDockerContainerUpdateAction(resource)?.hostName).toBeUndefined();
  });

  it('says why no update is offered when the server does not advertise one', () => {
    const update = {
      hostSourceId: 'h',
      containerId: 'abc',
      runtime: 'docker',
      updateStatus: { updateAvailable: true },
    };
    const withCapability = {
      ...container(update),
      capabilities: [{ name: 'update', type: 'common' }],
    } as unknown as Resource;
    expect(getDockerContainerUpdateAction(withCapability)?.updateNotOfferedReason).toBeUndefined();

    // A host that stopped reporting: no capability and no readiness refusal.
    const offline = {
      ...container(update),
      sourceStatus: { docker: { status: 'offline' } },
    } as unknown as Resource;
    expect(getDockerContainerUpdateAction(offline)?.updateNotOfferedReason).toBe(
      'Docker inventory for this host is offline.',
    );
    expect(getDockerContainerUpdateAction(container(update))?.updateNotOfferedReason).toBe(
      'Pulse does not currently offer an update action for this container.',
    );
  });

  it('offers nothing a container cannot be updated through', () => {
    const update = { updateAvailable: true };
    expect(
      getDockerContainerUpdateAction(container({ containerId: 'abc', updateStatus: update })),
    ).toBeUndefined();
    expect(
      getDockerContainerUpdateAction(container({ hostSourceId: 'h', updateStatus: update })),
    ).toBeUndefined();
    expect(
      getDockerContainerUpdateAction(container({ hostSourceId: 'h', containerId: 'abc' })),
    ).toBeUndefined();
  });

  it('reads an unchecked status as no status, and keeps a check error', () => {
    expect(
      getDockerContainerUpdateStatus(container({ updateStatus: {} as never })),
    ).toBeUndefined();
    expect(
      getDockerContainerUpdateStatus(
        container({ updateStatus: { error: ' registry 401 ' } as never }),
      ),
    ).toMatchObject({ updateAvailable: false, error: 'registry 401' });
  });
});
