import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  DOCKER_TAB_SPECS,
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
