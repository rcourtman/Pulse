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

  it('falls back to normalized Proxmox resource facets for node name and version', () => {
    const node = nodeFromResource({
      ...createNodeResource({}),
      proxmox: { node: 'pve-node-2', instance: 'cluster-a' },
      agent: { osName: 'Proxmox VE', osVersion: '9.1.9' },
      network: { rxBytes: 1024, txBytes: 2048 },
      diskIO: { readRate: 4096, writeRate: 8192 },
    } as Resource);

    expect(node?.name).toBe('pve-node-2');
    expect(node?.pveVersion).toBe('9.1.9');
    expect(node?.networkIn).toBe(1024);
    expect(node?.networkOut).toBe(2048);
    expect(node?.diskRead).toBe(4096);
    expect(node?.diskWrite).toBe(8192);
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

  it('uses PMG host and guest URLs from canonical platform data when available', () => {
    const instance = pmgInstanceFromResource(
      createServiceResource('pmg', {
        pmg: {
          hostname: 'pmg-service.local',
          hostUrl: 'https://pmg-service.local:8443',
          guestUrl: 'https://mail.example.com',
          instanceId: 'pmg-instance-1',
        },
      }),
    );

    expect(instance?.host).toBe('https://pmg-service.local:8443');
    expect(instance?.guestURL).toBe('https://mail.example.com');
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

  it('preserves canonical platform scopes across realtime resource merges', () => {
    const existing = {
      id: 'docker-container-frigate-141',
      type: 'app-container',
      name: 'frigate',
      displayName: 'frigate',
      platformId: 'frigate',
      platformType: 'docker',
      platformScopes: ['proxmox-pve', 'docker'],
      sourceType: 'api',
      sources: ['docker'],
      status: 'online',
      lastSeen: Date.now() - 1_000,
      platformData: {
        sources: ['docker'],
        docker: {
          hostSourceId: 'proxmox-lxc-docker:pve-a:node-a:141',
          containerId: 'frigate',
          runtime: 'docker',
        },
      },
    } as Resource;

    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          ...existing,
          lastSeen: Date.now(),
          platformScopes: undefined,
        } as Resource,
      ],
      [existing],
    );

    expect(resource.platformScopes).toEqual(['proxmox-pve', 'docker']);
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

  it('canonicalizes native TrueNAS share resources with the TrueNAS facet intact', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'truenas-share-media',
          type: 'network-share',
          name: 'Media',
          displayName: 'SMB Media',
          platformId: 'truenas-main:smb:media',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'online',
          lastSeen: Date.now(),
          truenas: {
            hostname: 'truenas-main.local',
            share: {
              id: 'smb-media',
              name: 'Media',
              protocol: 'SMB',
              path: '/mnt/tank/media',
              dataset: 'tank/media',
              enabled: true,
              readOnly: false,
            },
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.platformType).toBe('truenas');
    expect(resource.sourceType).toBe('api');
    expect(resource.truenas?.share).toMatchObject({
      id: 'smb-media',
      protocol: 'SMB',
      dataset: 'tank/media',
      path: '/mnt/tank/media',
    });
    expect((resource.platformData as Record<string, unknown>).sources).toEqual(['truenas']);
    expect((resource.platformData as Record<string, unknown>).truenas).toMatchObject({
      hostname: 'truenas-main.local',
      share: { id: 'smb-media' },
    });
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

  it('coalesces split realtime Proxmox and agent host records before rendering', () => {
    const proxmoxOnly = {
      id: 'agent-proxmox-delly',
      type: 'agent',
      name: 'delly',
      displayName: 'delly',
      platformId: 'delly',
      platformType: 'proxmox-pve',
      sourceType: 'api',
      sources: ['proxmox'],
      status: 'online',
      lastSeen: Date.now() - 1_000,
      canonicalIdentity: {
        displayName: 'delly',
        hostname: 'delly',
        platformId: 'delly',
        primaryId: 'node:homelab-delly',
      },
      proxmox: {
        nodeName: 'delly',
        clusterName: 'homelab',
        pveVersion: '9.1.9',
      },
      platformData: {
        sources: ['proxmox'],
        proxmox: {
          nodeName: 'delly',
          clusterName: 'homelab',
          pveVersion: '9.1.9',
        },
      },
    } as Resource;
    const agentOnly = {
      id: 'agent-runtime-delly',
      type: 'agent',
      name: 'delly',
      displayName: 'delly',
      platformId: 'delly',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      status: 'online',
      lastSeen: Date.now(),
      canonicalIdentity: {
        displayName: 'delly',
        hostname: 'delly',
        platformId: 'delly',
        primaryId: 'agent:delly-runtime',
      },
      agent: {
        hostname: 'delly',
        osName: 'Debian GNU/Linux',
        osVersion: '13',
      },
      platformData: {
        sources: ['agent'],
        agent: {
          hostname: 'delly',
          osName: 'Debian GNU/Linux',
          osVersion: '13',
        },
      },
    } as Resource;

    for (const incoming of [
      [proxmoxOnly, agentOnly],
      [agentOnly, proxmoxOnly],
    ]) {
      const [resource] = mergeCanonicalResourceSnapshot(incoming, []);

      expect(mergeCanonicalResourceSnapshot(incoming, [])).toHaveLength(1);
      expect(resource.id).toBe('agent-runtime-delly');
      expect(resource.platformType).toBe('proxmox-pve');
      expect(resource.sourceType).toBe('hybrid');
      expect(new Set(resource.sources ?? [])).toEqual(new Set(['agent', 'proxmox']));
      expect(resource.proxmox).toMatchObject({ nodeName: 'delly', clusterName: 'homelab' });
      expect(resource.agent).toMatchObject({ hostname: 'delly', osName: 'Debian GNU/Linux' });
      expect((resource.platformData as Record<string, unknown>).proxmox).toMatchObject({
        nodeName: 'delly',
      });
      expect((resource.platformData as Record<string, unknown>).agent).toMatchObject({
        hostname: 'delly',
      });
    }
  });

  it('does not coalesce same-name agent-only records without a platform source bridge', () => {
    const resources = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'agent-runtime-1',
          type: 'agent',
          name: 'shared-hostname',
          displayName: 'shared-hostname',
          platformId: 'shared-hostname',
          platformType: 'agent',
          sourceType: 'agent',
          sources: ['agent'],
          status: 'online',
          lastSeen: Date.now(),
        } as Resource,
        {
          id: 'agent-runtime-2',
          type: 'agent',
          name: 'shared-hostname',
          displayName: 'shared-hostname',
          platformId: 'shared-hostname',
          platformType: 'agent',
          sourceType: 'agent',
          sources: ['agent'],
          status: 'online',
          lastSeen: Date.now(),
        } as Resource,
      ],
      [],
    );

    expect(resources.map((resource) => resource.id)).toEqual([
      'agent-runtime-1',
      'agent-runtime-2',
    ]);
  });

  it('uses authoritative top-level sources for realtime storage platform canonicalization', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'storage:tower-array',
          type: 'storage',
          name: 'Tower Array',
          displayName: 'Tower Array',
          platformId: 'tower-array',
          platformType: 'proxmox-pve',
          sourceType: 'agent',
          sources: ['agent'],
          status: 'degraded',
          lastSeen: Date.now(),
          storage: {
            platform: 'unraid',
            type: 'unraid-array',
            topology: 'array',
            postureSummary: 'Unraid array is running without parity protection',
          },
          platformData: {
            platform: 'unraid',
            type: 'unraid-array',
            topology: 'array',
            postureSummary: 'Unraid array is running without parity protection',
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.platformType).toBe('agent');
    expect(resource.sourceType).toBe('agent');
    expect((resource.platformData as Record<string, unknown>).sources).toEqual(['agent']);
    expect((resource.platformData as Record<string, unknown>).storage).toMatchObject({
      platform: 'unraid',
      type: 'unraid-array',
      topology: 'array',
    });
    expect((resource.platformData as Record<string, unknown>).proxmox).toBeUndefined();
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

  it('preserves discovery readiness when merging realtime resource snapshots', () => {
    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'system-container-homeassistant',
          type: 'system-container',
          name: 'homeassistant',
          displayName: 'homeassistant',
          platformId: '101',
          platformType: 'proxmox-pve',
          sourceType: 'api',
          status: 'online',
          lastSeen: Date.now(),
          discoveryReadiness: {
            state: 'missing',
            reason: 'Discovery has not run for this resource.',
            resourceType: 'system-container',
            targetId: 'agent-delly',
            resourceId: '101',
            generatedAt: '2026-06-04T15:00:00Z',
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.discoveryReadiness).toMatchObject({
      state: 'missing',
      targetId: 'agent-delly',
      resourceId: '101',
    });
  });
});
