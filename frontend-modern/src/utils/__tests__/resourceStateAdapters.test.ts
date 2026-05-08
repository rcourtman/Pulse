import { describe, expect, it } from 'vitest';

import {
  mergeCanonicalResourceSnapshot,
  nodeFromResource,
  pbsInstanceFromResource,
  pmgInstanceFromResource,
} from '../resourceStateAdapters';
import type { Resource } from '@/types/resource';

const createNodeResource = (platformData: Record<string, unknown>): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'pve-node-1',
    displayName: 'PVE Node 1',
    platformId: 'pve-node-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    disk: { current: 30, total: 2048, used: 512 },
    platformData,
  }) as Resource;

const createServiceResource = (
  type: 'pbs' | 'pmg',
  platformData: Record<string, unknown>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: `${type}-1`,
    type,
    name: `${type}-name`,
    displayName: `${type.toUpperCase()} Display`,
    platformId: '',
    platformType: type === 'pbs' ? 'proxmox-pbs' : 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    disk: { current: 30, total: 2048, used: 512 },
    platformData,
    ...overrides,
  }) as Resource;

describe('resourceStateAdapters nodeFromResource', () => {
  it('maps canonical linkedAgentId', () => {
    const node = nodeFromResource(
      createNodeResource({
        linkedAgentId: 'agent-canonical',
        proxmox: { nodeName: 'pve-node-1' },
      }),
    );

    expect(node?.linkedAgentId).toBe('agent-canonical');
  });

  it('falls back to the actionable agent identity when linkedAgentId is absent', () => {
    const node = nodeFromResource(
      createNodeResource({
        proxmox: { nodeName: 'pve-node-1' },
        agent: { agentId: 'agent-from-facet' },
      }),
    );

    expect(node?.linkedAgentId).toBe('agent-from-facet');
  });

  it('uses typed canonical identity for node labels when proxmox nodeName is absent', () => {
    const node = nodeFromResource({
      ...createNodeResource({
        proxmox: {},
      } as Record<string, unknown>),
      name: '',
      displayName: '',
      platformId: '',
      canonicalIdentity: {
        displayName: 'Tower',
        hostname: 'tower.local',
        platformId: 'pve-canonical',
      },
    } as Resource);

    expect(node?.name).toBe('tower.local');
    expect(node?.displayName).toBe('Tower');
    expect(node?.host).toBe('tower.local');
    expect(node?.instance).toBe('pve-canonical');
    expect(node?.clusterName).toBeUndefined();
  });

  it('keeps node operator labels on local identity when governed summaries exist', () => {
    const node = nodeFromResource({
      ...createNodeResource({
        proxmox: {},
      } as Record<string, unknown>),
      name: '',
      displayName: 'Tower',
      platformId: 'tower-id',
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['hostname'] },
      },
      canonicalIdentity: {
        displayName: 'Tower',
        hostname: 'tower.local',
        platformId: 'tower-id',
      },
    } as Resource);

    expect(node?.name).toBe('tower.local');
    expect(node?.displayName).toBe('Tower');
    expect(node?.host).toBe('tower.local');
  });

  it('projects the canonical cluster name through the shared helper', () => {
    const node = nodeFromResource(
      createNodeResource({
        proxmox: {
          nodeName: 'pve-node-1',
          clusterName: 'cluster-a',
        },
      }),
    );

    expect(node?.clusterName).toBe('cluster-a');
  });

  it('maps PBS display and host identity through shared resource helpers', () => {
    const instance = pbsInstanceFromResource(
      createServiceResource('pbs', {
        pbs: { hostname: 'pbs-service.local', instanceId: 'pbs-instance-1' },
      }),
    );

    expect(instance?.name).toBe('PBS Display');
    expect(instance?.host).toBe('https://pbs-service.local:8007');
  });

  it('keeps PBS operator labels on local identity when governed summaries exist', () => {
    const instance = pbsInstanceFromResource(
      createServiceResource(
        'pbs',
        {
          pbs: { hostname: 'pbs-service.local', instanceId: 'pbs-instance-1' },
        },
        {
          displayName: 'PBS Main',
          policy: {
            sensitivity: 'restricted',
            routing: { scope: 'local-only', redact: ['hostname'] },
          },
        },
      ),
    );

    expect(instance?.name).toBe('PBS Main');
  });

  it('maps PMG identity through shared hostname fallback when displayName is absent', () => {
    const instance = pmgInstanceFromResource(
      createServiceResource(
        'pmg',
        {
          pmg: { hostname: 'pmg-service.local', instanceId: 'pmg-instance-1' },
        },
        { displayName: '' as unknown as Resource['displayName'] },
      ),
    );

    expect(instance?.name).toBe('pmg-service.local');
    expect(instance?.host).toBe('https://pmg-service.local:8006');
  });

  it('keeps PMG operator labels on local identity when governed summaries exist', () => {
    const instance = pmgInstanceFromResource(
      createServiceResource(
        'pmg',
        {
          pmg: { hostname: 'pmg-service.local', instanceId: 'pmg-instance-1' },
        },
        {
          displayName: 'PMG Main',
          policy: {
            sensitivity: 'restricted',
            routing: { scope: 'local-only', redact: ['hostname'] },
          },
        },
      ),
    );

    expect(instance?.name).toBe('PMG Main');
  });

  it('canonicalizes thin realtime resource platform data without inventing standalone clusters', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'docker-host-1',
          type: 'docker-host',
          name: 'Ops Services 01',
          displayName: 'Ops Services 01',
          platformId: 'ops-services-01',
          platformType: 'docker',
          sourceType: 'api',
          status: 'online',
          lastSeen: Date.now(),
          platformData: {
            hostname: 'ops-services-01',
            hostSourceId: 'ops-services-01',
            runtime: 'docker',
          },
        } as Resource,
        {
          id: 'pbs-1',
          type: 'pbs',
          name: 'backup-vault',
          displayName: 'backup-vault',
          platformId: 'pbs-1',
          platformType: 'proxmox-pbs',
          sourceType: 'api',
          status: 'online',
          lastSeen: Date.now(),
          platformData: {
            host: '198.51.100.10',
            version: '3.2.1',
            connectionHealth: 'healthy',
            numDatastores: 2,
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.clusterId).toBeUndefined();
    const pbs = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'pbs-1',
          type: 'pbs',
          name: 'backup-vault',
          displayName: 'backup-vault',
          platformId: 'pbs-1',
          platformType: 'proxmox-pbs',
          sourceType: 'api',
          status: 'online',
          lastSeen: Date.now(),
          platformData: {
            host: '198.51.100.10',
            version: '3.2.1',
            connectionHealth: 'healthy',
            numDatastores: 2,
          },
        } as Resource,
      ],
      [],
    )[0];

    expect((pbs.platformData as Record<string, unknown>)?.pbs).toMatchObject({
      hostname: '198.51.100.10',
      version: '3.2.1',
      connectionHealth: 'healthy',
      datastoreCount: 2,
    });
  });

  it('canonicalizes agentless availability realtime resources as availability endpoints', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'network-endpoint-1',
          type: 'network-endpoint',
          name: 'MQTT power meter',
          displayName: 'MQTT power meter',
          platformId: 'mock-availability-mqtt-meter',
          platformType: 'generic',
          sourceType: 'api',
          status: 'online',
          lastSeen: Date.now(),
          platformData: {
            availability: {
              targetId: 'mock-availability-mqtt-meter',
              protocol: 'tcp',
              address: 'power-meter-01.lab.local',
              port: 1883,
            },
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.platformType).toBe('availability');
    expect(resource.sourceType).toBe('api');
    expect(resource.availability).toMatchObject({
      targetId: 'mock-availability-mqtt-meter',
      protocol: 'tcp',
      address: 'power-meter-01.lab.local',
      port: 1883,
    });
    expect((resource.platformData as Record<string, unknown>).sources).toEqual(['availability']);
  });

  it('canonicalizes top-level Proxmox and agent facets as one hybrid PVE resource', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'agent-pi',
          type: 'agent',
          name: 'pi',
          displayName: 'pi',
          platformId: 'pi',
          platformType: 'agent',
          sourceType: 'agent',
          status: 'online',
          lastSeen: Date.now(),
          proxmox: {
            nodeName: 'pi',
            instance: 'pi',
          },
          agent: {
            hostname: 'pi',
            platform: 'debian',
            osName: 'Debian GNU/Linux',
            osVersion: '12',
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.platformType).toBe('proxmox-pve');
    expect(resource.sourceType).toBe('hybrid');
    expect(resource.proxmox).toMatchObject({ nodeName: 'pi', instance: 'pi' });
    expect(resource.agent).toMatchObject({ hostname: 'pi', osName: 'Debian GNU/Linux' });
    expect((resource.platformData as Record<string, unknown>).sources).toEqual([
      'proxmox',
      'agent',
    ]);
    expect((resource.platformData as Record<string, unknown>).proxmox).toMatchObject({
      nodeName: 'pi',
    });
    expect((resource.platformData as Record<string, unknown>).agent).toMatchObject({
      hostname: 'pi',
    });
  });

  it('replaces stale platform source facets with the current snapshot sources', () => {
    const existing = {
      id: 'agent-tower',
      type: 'agent',
      name: 'Tower',
      displayName: 'Tower',
      platformId: 'tower',
      platformType: 'proxmox-pve',
      sourceType: 'hybrid',
      status: 'degraded',
      lastSeen: Date.now(),
      proxmox: {
        nodeName: 'Tower',
      },
      agent: {
        hostname: 'Tower',
        hostProfile: 'unraid',
        osName: 'Unraid',
      },
      platformData: {
        sources: ['proxmox', 'docker', 'agent'],
        proxmox: {
          nodeName: 'Tower',
        },
        docker: {
          runtime: 'docker',
        },
        agent: {
          hostname: 'Tower',
          hostProfile: 'unraid',
          osName: 'Unraid',
        },
      },
    } as Resource;

    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'agent-tower',
          type: 'agent',
          name: 'Tower',
          displayName: 'Tower',
          platformId: 'tower',
          platformType: 'agent',
          sourceType: 'hybrid',
          status: 'degraded',
          lastSeen: Date.now(),
          agent: {
            hostname: 'Tower',
            hostProfile: 'unraid',
            osName: 'Unraid',
          },
          docker: {
            runtime: 'docker',
          },
        } as Resource,
      ],
      [existing],
    );

    expect(resource.platformType).toBe('docker');
    expect(resource.sourceType).toBe('hybrid');
    expect(resource.proxmox).toBeUndefined();
    expect(resource.agent).toMatchObject({ hostProfile: 'unraid', osName: 'Unraid' });
    expect((resource.platformData as Record<string, unknown>).sources).toEqual(['docker', 'agent']);
    expect((resource.platformData as Record<string, unknown>).proxmox).toBeUndefined();
  });

  it('does not synthesize a Proxmox facet from flat agent disk telemetry', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'agent-tower',
          type: 'agent',
          name: 'Tower',
          displayName: 'Tower',
          platformId: 'tower',
          platformType: 'agent',
          sourceType: 'hybrid',
          status: 'degraded',
          lastSeen: Date.now(),
          agent: {
            hostname: 'Tower',
            hostProfile: 'unraid',
            osName: 'Unraid',
          },
          docker: {
            runtime: 'docker',
          },
          platformData: {
            osName: 'Unraid',
            osVersion: '7.2.2',
            platform: 'linux',
            disks: [
              {
                device: 'rootfs',
                mountpoint: '/',
                filesystem: 'rootfs',
              },
            ],
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.platformType).toBe('docker');
    expect((resource.platformData as Record<string, unknown>).sources).toEqual(['docker', 'agent']);
    expect((resource.platformData as Record<string, unknown>).proxmox).toBeUndefined();
  });

  it('preserves richer existing resource details when realtime updates are thinner', () => {
    const existing: Resource = {
      id: 'node-1',
      type: 'agent',
      name: 'West Production A',
      displayName: 'West Production A',
      platformId: 'west-production-a',
      platformType: 'proxmox-pve',
      sourceType: 'hybrid',
      status: 'online',
      lastSeen: Date.now(),
      cpu: { current: 10 },
      diskIO: { readRate: 1_250_000, writeRate: 640_000 },
      platformData: {
        proxmox: {
          clusterName: 'Core Fabric',
        },
      },
    } as Resource;

    const [merged] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'node-1',
          type: 'agent',
          name: 'West Production A',
          displayName: 'West Production A',
          platformId: 'west-production-a',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          status: 'online',
          lastSeen: Date.now(),
          cpu: { current: 42 },
          platformData: {},
        } as Resource,
      ],
      [existing],
    );

    expect(merged.cpu?.current).toBe(42);
    expect(merged.diskIO).toEqual({
      readRate: 1_250_000,
      writeRate: 640_000,
    });
    expect(merged.clusterId).toBe('Core Fabric');
  });

  it('preserves canonical health context from realtime resources', () => {
    const [agent] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'agent:tower',
          type: 'agent',
          name: 'Tower',
          displayName: 'Tower',
          platformId: 'agent-1',
          platformType: 'agent',
          sourceType: 'agent',
          status: 'degraded',
          lastSeen: Date.now(),
          incidentSummary: 'Unraid array is running without parity protection',
          agent: {
            osName: 'Unraid',
            storagePostureSummary: 'Unraid array is running without parity protection',
            unraid: {
              postureSummary: 'Unraid array is running without parity protection',
              risk: {
                level: 'warning',
                reasons: [
                  {
                    code: 'unraid_no_parity',
                    severity: 'warning',
                    summary: 'Unraid array is running without parity protection',
                  },
                ],
              },
            },
          },
        } as Resource,
      ],
      [],
    );

    expect(agent.incidentSummary).toBe('Unraid array is running without parity protection');
    expect(agent.agent?.storagePostureSummary).toBe(
      'Unraid array is running without parity protection',
    );
    expect(agent.agent?.unraid?.risk?.reasons?.[0]?.code).toBe('unraid_no_parity');
  });
});
