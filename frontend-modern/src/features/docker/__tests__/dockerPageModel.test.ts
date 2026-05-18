import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  DOCKER_TAB_SPECS,
  buildDockerContainerDefaultHiddenColumnIds,
  buildDockerPageModel,
  buildVisibleDockerTabSpecs,
  hasDockerSwarmEvidence,
} from '../dockerPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('dockerPageModel', () => {
  it('declares the Docker section set with hosts, containers, and Swarm services', () => {
    expect(DOCKER_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'containers',
      'services',
    ]);
  });

  it('buckets Docker hosts, containers, and Swarm services from canonical resources', () => {
    const model = buildDockerPageModel([
      makeResource({ id: 'docker-host-1', type: 'agent' }),
      makeResource({
        id: 'ctr-1',
        type: 'app-container',
        platformType: 'docker',
      }),
      makeResource({ id: 'svc-1', type: 'docker-service' }),
      makeResource({
        id: 'pve-node-1',
        type: 'agent',
        platformType: 'proxmox-pve',
      }),
    ]);

    expect(model.hosts.map((r) => r.id)).toEqual(['docker-host-1']);
    expect(model.containers.map((r) => r.id)).toEqual(['ctr-1']);
    expect(model.services.map((r) => r.id)).toEqual(['svc-1']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['ctr-1', 'docker-host-1', 'svc-1'].sort(),
    );
  });

  it('excludes non-Docker hosts that share the agent type', () => {
    const model = buildDockerPageModel([
      makeResource({
        id: 'truenas-host',
        type: 'agent',
        platformType: 'truenas',
      }),
    ]);
    expect(model.hosts).toEqual([]);
    expect(model.resources).toEqual([]);
  });

  it('shows Docker subtabs only when canonical resource evidence exists', () => {
    expect(
      buildVisibleDockerTabSpecs(
        buildDockerPageModel([makeResource({ id: 'docker-host-1', type: 'agent' })]),
      ).map((tab) => tab.id),
    ).toEqual(['overview']);

    expect(
      buildVisibleDockerTabSpecs(
        buildDockerPageModel([
          makeResource({ id: 'docker-host-1', type: 'agent' }),
          makeResource({ id: 'ctr-1', type: 'app-container' }),
        ]),
      ).map((tab) => tab.id),
    ).toEqual(['overview', 'containers']);

    expect(
      buildVisibleDockerTabSpecs(
        buildDockerPageModel([
          makeResource({ id: 'docker-host-1', type: 'agent' }),
          makeResource({ id: 'svc-1', type: 'docker-service' }),
        ]),
      ).map((tab) => tab.id),
    ).toEqual(['overview', 'services']);
  });

  it('hides Docker container I/O columns by default when the snapshot has no I/O telemetry', () => {
    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({ id: 'ctr-1', type: 'app-container' }),
      ]),
    ).toEqual(['disk', 'tags', 'netIo', 'diskIo']);
  });

  it('keeps Docker container I/O columns default-visible once telemetry exists', () => {
    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({
          id: 'ctr-1',
          type: 'app-container',
          network: { rxBytes: 0, txBytes: 0 },
          diskIO: { readRate: 0, writeRate: 0 },
        }),
      ]),
    ).toEqual(['disk', 'tags']);
  });

  it('decides Docker container network and disk I/O defaults independently', () => {
    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({
          id: 'ctr-1',
          type: 'app-container',
          network: { rxBytes: 128, txBytes: 64 },
        }),
      ]),
    ).toEqual(['disk', 'tags', 'diskIo']);

    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({
          id: 'ctr-2',
          type: 'app-container',
          diskIO: { readRate: 128, writeRate: 64 },
        }),
      ]),
    ).toEqual(['disk', 'tags', 'netIo']);
  });

  it('does not treat standalone inactive Docker Swarm metadata as Swarm evidence', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'docker-host-1',
          type: 'agent',
          docker: {
            swarm: {
              nodeRole: 'worker',
              localState: 'inactive',
              scope: 'node',
            },
          },
        }),
      ),
    ).toBe(false);

    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'docker-host-1',
          type: 'agent',
          docker: {
            swarm: {
              nodeId: 'node-1',
              nodeRole: 'manager',
              localState: 'active',
            },
          },
        }),
      ),
    ).toBe(true);
  });
});
