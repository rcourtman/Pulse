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
