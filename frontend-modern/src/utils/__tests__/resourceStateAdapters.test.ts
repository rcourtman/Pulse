import { describe, expect, it } from 'vitest';

import {
  buildFastResourceStorePatchOps,
  getFastResourceMergePatchKeys,
  mergeCanonicalResourceDeltaSnapshot,
  mergeCanonicalResourceSnapshot,
  nodeFromResource,
  pbsInstanceFromResource,
  pmgInstanceFromResource,
  unionResourceChangedKeys,
} from '../resourceStateAdapters';
import { getAvailabilityProbePresentation } from '../availabilityProbePresentation';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';

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
  it('keeps selected memory provenance independent of a different raw platform reading', () => {
    const observation = {
      state: 'last-known',
      source: 'guest-agent-meminfo',
      observedAt: '2026-10-04T14:00:00Z',
    };
    const raw = { state: 'current', source: 'status-mem', observedAt: '2026-10-04T17:00:00Z' };
    const resource = {
      ...createNodeResource({}),
      memory: { current: 25, total: 1024, used: 256, observation },
      proxmox: { memory: { total: 1024, used: 768, usage: 75, observation: raw } },
    } as Resource;
    expect(nodeFromResource(resource)?.memory).toMatchObject({ usage: 25, observation });
    expect(
      nodeFromResource({ ...resource, memory: { current: 25, total: 1024, used: 256 } })?.memory,
    ).not.toHaveProperty('observation');
    expect(nodeFromResource({ ...resource, memory: undefined })?.memory).toMatchObject({
      usage: 75,
      observation: raw,
    });
    expect(
      nodeFromResource({
        ...resource,
        memory: {
          ...resource.memory!,
          observation: { state: 'current', source: 'agent', observedAt: '2026-10-04T16:00:00Z' },
        },
      })?.memory.observation?.source,
    ).toBe('agent');
  });

  it('keeps Proxmox metric coordinates without inventing an agent from discovery routing', () => {
    const resource = {
      ...createNodeResource({ proxmox: { nodeName: 'pve-node-1' } }),
      id: 'agent-canonical-id',
      metricsTarget: { resourceType: 'node', resourceId: 'cluster-pve-node-1' },
      discoveryTarget: { resourceType: 'agent', resourceId: 'pve-node-1', agentId: 'pve-node-1' },
    } as Resource;
    const node = nodeFromResource(resource);
    expect(node?.metricsTarget).toEqual(resource.metricsTarget);
    expect(node?.linkedAgentId).toBeUndefined();
    const linked = nodeFromResource({ ...resource, agent: { agentId: 'real-agent' } });
    expect(linked?.linkedAgentId).toBe('real-agent');
  });

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

  it('maps the PVE web interface URL from proxmox metadata', () => {
    const node = nodeFromResource(
      createNodeResource({
        proxmox: {
          nodeName: 'pve-node-1',
          guestUrl: 'https://pve.example.com:8006',
          host: 'https://192.168.0.5:8006',
        },
      }),
    );

    expect(node?.guestURL).toBe('https://pve.example.com:8006');
    expect(node?.host).toBe('pve-node-1');
  });

  it('falls back to the PVE API connection URL when no guest URL override exists', () => {
    const node = nodeFromResource(
      createNodeResource({
        proxmox: {
          nodeName: 'pve-node-1',
          host: 'https://192.168.0.5:8006',
        },
      }),
    );

    expect(node?.guestURL).toBe('https://192.168.0.5:8006');
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

  it('uses Proxmox network inventory unless a linked agent has richer interface data', () => {
    const proxmoxOnly = nodeFromResource({
      ...createNodeResource({}),
      proxmox: {
        nodeName: 'pve-node-2',
        networkInterfaces: [
          { name: 'eno1', addresses: [] },
          { name: 'vmbr0', addresses: ['192.168.10.21/24'] },
        ],
      },
    } as Resource);
    expect(proxmoxOnly?.networkInterfaces).toEqual([
      { name: 'eno1', addresses: [] },
      { name: 'vmbr0', addresses: ['192.168.10.21/24'] },
    ]);

    const linkedAgent = nodeFromResource({
      ...createNodeResource({}),
      proxmox: {
        nodeName: 'pve-node-2',
        networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.21/24'] }],
      },
      agent: {
        networkInterfaces: [{ name: 'eth0', addresses: ['192.168.10.22'] }],
      },
    } as Resource);
    expect(linkedAgent?.networkInterfaces).toEqual([
      { name: 'eth0', addresses: ['192.168.10.22'] },
    ]);
  });

  it("reads a Proxmox node's CPU low and record from the poller's temperature details", () => {
    // The unified resource API publishes the scalar maximum as
    // proxmox.temperature and the full reading as proxmox.temperatureDetails.
    const proxmox = {
      nodeName: 'pve-node-2',
      temperature: 58,
      temperatureDetails: {
        available: true,
        hasCPU: true,
        cpuPackage: 52,
        cpuMax: 58,
        cpuMin: 41,
        cpuMaxRecord: 76,
        minRecorded: '2026-10-06T08:00:00Z',
        maxRecorded: '2026-10-06T12:00:00Z',
        cores: [{ core: 0, temp: 58 }],
        lastUpdate: '2026-10-06T15:00:00Z',
      },
    };
    const node = nodeFromResource({
      ...createNodeResource({ proxmox }),
      temperature: 58,
      proxmox,
    } as unknown as Resource);

    expect(node?.temperature).toMatchObject({
      available: true,
      cpuPackage: 52,
      cpuMax: 58,
      cpuMin: 41,
      cpuMaxRecord: 76,
      lastUpdate: '2026-10-06T15:00:00Z',
    });
    expect(node?.temperature?.cores).toEqual([{ core: 0, temp: 58 }]);

    // Without details there is no history: the current reading must not be
    // reported as the node's low or record.
    const bare = nodeFromResource({
      ...createNodeResource({ proxmox: { nodeName: 'pve-node-2', temperature: 58 } }),
      temperature: 58,
    } as unknown as Resource);
    expect(bare?.temperature?.cpuPackage).toBe(58);
    expect(bare?.temperature?.cpuMin).toBeUndefined();
    expect(bare?.temperature?.cpuMaxRecord).toBeUndefined();

    // Details without a CPU reading do not displace a valid scalar CPU reading.
    const sensorsOnly = nodeFromResource({
      ...createNodeResource({
        proxmox: {
          nodeName: 'pve-node-2',
          temperatureDetails: {
            available: true,
            hasCPU: false,
            nvme: [{ device: 'nvme0', temp: 44 }],
          },
        },
      }),
      temperature: 58,
    } as unknown as Resource);
    expect(sensorsOnly?.temperature?.cpuPackage).toBe(58);
    expect(sensorsOnly?.temperature?.available).toBe(true);

    // An unusable earlier record still falls back to the scalar reading.
    const legacyUnavailable = nodeFromResource({
      ...createNodeResource({
        temperature: { available: false },
        proxmox: { nodeName: 'pve-node-2', temperature: { available: false, cpuPackage: 58 } },
      }),
      temperature: 58,
    } as unknown as Resource);
    expect(legacyUnavailable?.temperature?.cpuPackage).toBe(58);
    expect(legacyUnavailable?.temperature?.available).toBe(true);

    // A node with a host agent keeps reading the agent facet first.
    const agentLinked = nodeFromResource({
      ...createNodeResource({ proxmox, agent: { agentId: 'pve-agent', temperature: 61 } }),
      temperature: 61,
      proxmox,
    } as unknown as Resource);
    expect(agentLinked?.temperature?.cpuPackage).toBe(61);
    expect(agentLinked?.temperature?.cpuMin).toBeUndefined();
  });

  it('preserves linked-agent GPU telemetry for the Proxmox node drawer', () => {
    const gpu = {
      id: '0',
      name: 'NVIDIA RTX A6000',
      temperatureCelsius: 63,
      utilizationPercent: 42,
      memoryUsedBytes: 8 * 1024 * 1024 * 1024,
      memoryTotalBytes: 48 * 1024 * 1024 * 1024,
    };
    const node = nodeFromResource({
      ...createNodeResource({}),
      proxmox: { nodeName: 'pve-node-2' },
      agent: { agentId: 'pve-agent', sensors: { gpu: [gpu] } },
    } as Resource);

    expect(node?.sensors?.gpu).toEqual([gpu]);
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

  it('preserves canonical identity succession across thin realtime merges', () => {
    const existing = {
      ...createNodeResource({ proxmox: { nodeName: 'pve-node-1' } }),
      canonicalIdentity: {
        primaryId: 'agent:connection-current',
        aliases: ['agent-current', 'node-1'],
        supersededIds: ['agent-retired-before-refresh'],
      },
    } as Resource;

    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          ...existing,
          lastSeen: Date.now(),
          canonicalIdentity: {
            primaryId: 'agent:connection-current',
            aliases: ['agent-current'],
            supersededIds: ['agent-retired-in-refresh'],
          },
        } as Resource,
      ],
      [existing],
    );

    expect(resource.canonicalIdentity).toMatchObject({
      primaryId: 'agent:connection-current',
      aliases: ['agent-current', 'node-1'],
      supersededIds: ['agent-retired-in-refresh', 'agent-retired-before-refresh'],
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

  it('preserves plural attached availability facets across thin realtime merges', () => {
    const availabilityChecks = [
      {
        targetId: 'ops-api',
        linkedResourceId: 'docker-host-1',
        protocol: 'tcp',
        address: '192.0.2.18',
        port: 8007,
        available: true,
        correlationState: 'attached' as const,
      },
      {
        targetId: 'ops-web',
        linkedResourceId: 'docker-host-1',
        protocol: 'https',
        address: 'ops.example.test',
        path: '/health',
        available: true,
        correlationState: 'attached' as const,
      },
    ];
    const existing = {
      id: 'docker-host-1',
      type: 'docker-host',
      name: 'Operations Docker',
      displayName: 'Operations Docker',
      platformId: 'docker-host-1',
      platformType: 'docker',
      sourceType: 'api',
      sources: ['docker', 'availability'],
      status: 'online',
      lastSeen: Date.now() - 1_000,
      availability: availabilityChecks[0],
      availabilityChecks,
    } as Resource;

    const [resource] = mergeCanonicalResourceSnapshot(
      [
        {
          ...existing,
          lastSeen: Date.now(),
          availability: undefined,
          availabilityChecks: undefined,
        } as Resource,
      ],
      [existing],
    );

    expect(resource.availability).toEqual(availabilityChecks[0]);
    expect(resource.availabilityChecks).toEqual(availabilityChecks);
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

  it('keeps same-hostname standalone Proxmox provider rows separate across realtime merges', () => {
    const providerRow = (
      id: string,
      displayName: string,
      instance: string,
      host: string,
      agentId?: string,
    ): Resource =>
      ({
        id,
        type: 'agent',
        name: displayName,
        displayName,
        platformId: 'pve',
        platformType: 'proxmox-pve',
        sourceType: agentId ? 'hybrid' : 'api',
        sources: agentId ? ['proxmox', 'agent'] : ['proxmox'],
        status: 'online',
        lastSeen: Date.now(),
        canonicalIdentity: {
          displayName,
          hostname: 'pve',
          platformId: 'pve',
        },
        identity: {
          hostnames: ['pve'],
          ...(agentId ? { machineId: `machine-${agentId}` } : {}),
        },
        proxmox: {
          sourceId: id,
          nodeIdentity: id,
          nodeName: 'pve',
          nodeDisplayName: displayName,
          instance,
          host,
        },
        ...(agentId
          ? {
              agent: {
                agentId,
                hostname: 'pve',
                machineId: `machine-${agentId}`,
              },
            }
          : {}),
      }) as Resource;

    // Mirrors #1753 after one same-name site's agent has authenticated while
    // the other is still represented by its independent provider poll. The
    // browser must not undo the server's provider-scoped split on a later
    // realtime reconciliation.
    const staging = providerRow(
      'staging-pve',
      'Tripper Staging',
      'hema-staging',
      'https://pve.hemastaging.hot:8006',
      'host-staging',
    );
    const production = providerRow(
      'production-pve',
      'VV Staging',
      'hema-production',
      'https://pve.hemaproduction.hot:8006',
    );

    const full = mergeCanonicalResourceSnapshot([staging, production], []);
    expect(full).toHaveLength(2);
    expect(full.map((resource) => resource.displayName).sort()).toEqual([
      'Tripper Staging',
      'VV Staging',
    ]);

    const delta = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(staging), structuredClone(production)],
      full,
      new Set(['production-pve']),
    );
    expect(delta).toHaveLength(2);
    expect(delta.map((resource) => resource.proxmox?.instance).sort()).toEqual([
      'hema-production',
      'hema-staging',
    ]);

    const duplicateEndpoint = providerRow(
      'duplicate-pve',
      'Duplicate connection',
      'duplicate-instance',
      'pve.hemastaging.hot:8006',
    );
    expect(mergeCanonicalResourceSnapshot([staging, duplicateEndpoint], [])).toHaveLength(1);
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

  it('does not synthesize Docker platform state from an empty machine-agent Docker facet', () => {
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
          status: 'online',
          lastSeen: Date.now(),
          agent: {
            hostname: 'Tower',
          },
          docker: {},
          platformData: {
            agent: {
              hostname: 'Tower',
            },
            docker: {},
          },
        } as Resource,
      ],
      [],
    );

    expect(resource.platformType).toBe('agent');
    expect(resource.sourceType).toBe('agent');
    expect(resource.docker).toBeUndefined();
    expect((resource.platformData as Record<string, unknown>).sources).toEqual(['agent']);
    expect((resource.platformData as Record<string, unknown>).docker).toBeUndefined();
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

describe('resourceStateAdapters unavailable memory contract', () => {
  const unavailableProxmoxMemory = {
    nodeName: 'n1',
    memory: {
      total: 8192,
      used: 0,
      free: 0,
      usage: 0,
      usageUnavailable: true,
    },
  } as unknown as Resource['proxmox'];

  it('clears a previous metric when a snapshot explicitly reports unavailable memory', () => {
    const previous = createNodeResource({});
    const incoming = { ...previous, proxmox: unavailableProxmoxMemory };
    delete incoming.memory; // JSON omitempty, not an explicit undefined property.
    const [merged] = mergeCanonicalResourceSnapshot([incoming], [previous]);
    expect(nodeFromResource(merged)?.memory.usageUnavailable).toBe(true);
    expect(merged.memory).toBeUndefined();
  });

  it('clears withdrawn memory through both delta paths and emits the store clear', () => {
    const previous = {
      ...createNodeResource({}),
      type: 'vm',
      proxmox: unavailableProxmoxMemory,
    } as Resource;
    const incoming = { ...previous };
    delete incoming.memory;
    for (const keys of [undefined, new Map([[previous.id, ['memory']]])]) {
      const [merged] = mergeCanonicalResourceDeltaSnapshot(
        [incoming],
        [previous],
        new Set([previous.id]),
        keys,
      );
      expect(merged.memory).toBeUndefined();
      expect(buildFastResourceStorePatchOps(merged, ['memory'])).toEqual([
        { key: 'memory', value: undefined, mode: 'set' },
      ]);
      expect(buildFastResourceStorePatchOps(merged, ['proxmox'])).toContainEqual({
        key: 'memory',
        value: undefined,
        mode: 'set',
      });
    }
  });

  it('retains richer memory on partial omission without unavailable evidence', () => {
    const previous = createNodeResource({});
    const incoming = { ...previous };
    delete incoming.memory;
    const [merged] = mergeCanonicalResourceSnapshot([incoming], [previous]);
    expect(merged.memory).toEqual(previous.memory);
  });

  it('accepts a newly trusted metric after an unavailable snapshot', () => {
    const previous = { ...createNodeResource({}), proxmox: unavailableProxmoxMemory };
    delete previous.memory;
    const incoming = { ...previous, memory: { current: 0, total: 8192, used: 0 } };
    const [merged] = mergeCanonicalResourceSnapshot([incoming], [previous]);
    expect(nodeFromResource(merged)?.memory).toMatchObject({ usage: 0, usageUnavailable: false });
  });

  it('preserves explicit unavailable usage when no trusted metric exists', () => {
    const node = nodeFromResource({
      ...createNodeResource({}),
      memory: undefined,
      proxmox: unavailableProxmoxMemory,
    });

    expect(node?.memory).toMatchObject({
      total: 8192,
      used: 0,
      usage: 0,
      usageUnavailable: true,
    });
  });

  it('lets a trusted merged metric override unavailable raw evidence', () => {
    const node = nodeFromResource({
      ...createNodeResource({}),
      memory: { current: 50, total: 8192, used: 4096, free: 4096 },
      proxmox: unavailableProxmoxMemory,
    });

    expect(node?.memory).toMatchObject({
      total: 8192,
      used: 4096,
      usage: 50,
      usageUnavailable: false,
    });
  });
});

// REST rows and the websocket baseline the store rebuilds from merge-patch
// deltas carry the availability summary as one whole check record whose
// failure, location and certificate fields are omitempty, so a field missing
// from a facet that is present means "no longer true", not "unchanged".
describe('resourceStateAdapters availability summary facet', () => {
  const failingHttpsCheck: ResourceAvailabilityMeta = {
    targetId: 'shop-https',
    name: 'Shop HTTPS',
    targetKind: 'service',
    address: 'shop.example.test',
    protocol: 'https',
    port: 443,
    path: '/health',
    probeOutcome: 'failed',
    transportOutcome: 'reachable',
    applicationOutcome: 'failed',
    applicationStatusCode: 503,
    applicationFailureCode: 'http_status',
    aggregateState: 'degraded',
    disagreement: true,
    expectedLocations: 2,
    reportingLocations: 2,
    enabled: true,
    available: false,
    lastChecked: '2026-10-07T12:00:00Z',
    lastSuccess: '2026-10-07T11:55:00Z',
    consecutiveFailures: 3,
    lastError: 'HTTP 503 Service Unavailable',
    failureThreshold: 3,
    pollIntervalSeconds: 30,
    timeoutMillis: 5000,
    correlationState: 'standalone',
  };
  const recoveredHttpsCheck: ResourceAvailabilityMeta = {
    targetId: 'shop-https',
    name: 'Shop HTTPS',
    targetKind: 'service',
    address: 'shop.example.test',
    protocol: 'https',
    port: 443,
    path: '/health',
    probeOutcome: 'reachable',
    transportOutcome: 'reachable',
    applicationOutcome: 'passed',
    applicationStatusCode: 200,
    aggregateState: 'healthy',
    expectedLocations: 2,
    reportingLocations: 2,
    enabled: true,
    available: true,
    lastChecked: '2026-10-07T12:01:00Z',
    lastSuccess: '2026-10-07T12:01:00Z',
    latencyMillis: 41,
    failureThreshold: 3,
    pollIntervalSeconds: 30,
    timeoutMillis: 5000,
    correlationState: 'standalone',
  };
  const endpointRow = (availability: ResourceAvailabilityMeta): Resource =>
    ({
      id: 'network-endpoint-shop',
      type: 'network-endpoint',
      name: 'Shop HTTPS',
      displayName: 'Shop HTTPS',
      platformId: 'shop-https',
      platformType: 'availability',
      sourceType: 'api',
      sources: ['availability'],
      status: availability.available ? 'online' : 'offline',
      lastSeen: Date.parse(availability.lastChecked ?? '2026-10-07T12:00:00Z'),
      availability,
    }) as Resource;

  it('drops a recovered check’s failure fields on snapshot and delta merges', () => {
    const [previous] = mergeCanonicalResourceSnapshot([endpointRow(failingHttpsCheck)], []);
    const incoming = endpointRow(recoveredHttpsCheck);

    const [snapshot] = mergeCanonicalResourceSnapshot([structuredClone(incoming)], [previous]);
    const [delta] = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(incoming)],
      [previous],
      new Set([incoming.id]),
      new Map([[incoming.id, ['status', 'lastSeen', 'availability']]]),
    );

    for (const merged of [snapshot, delta]) {
      expect(merged.availability).toEqual(recoveredHttpsCheck);
      expect((merged.platformData as Record<string, unknown> | undefined)?.availability).toEqual(
        recoveredHttpsCheck,
      );
      const presentation = getAvailabilityProbePresentation(
        merged,
        new Date('2026-10-07T12:01:10Z'),
      );
      expect(presentation?.detailLabel).not.toContain('failures');
      expect(presentation?.detailLabel).not.toContain('503');
    }
  });

  it('replaces a machine summary that switches to another attached check', () => {
    const passingTcpCheck: ResourceAvailabilityMeta = {
      targetId: 'db-1-postgres',
      linkedResourceId: 'agent-db-1',
      name: 'db-1 Postgres',
      targetKind: 'machine',
      address: '192.0.2.40',
      protocol: 'tcp',
      port: 5432,
      probeOutcome: 'reachable',
      transportOutcome: 'reachable',
      aggregateState: 'healthy',
      expectedLocations: 2,
      reportingLocations: 2,
      enabled: true,
      available: true,
      lastChecked: '2026-10-07T12:00:00Z',
      lastSuccess: '2026-10-07T12:00:00Z',
      latencyMillis: 3,
      failureThreshold: 3,
      pollIntervalSeconds: 30,
      timeoutMillis: 2000,
      correlationState: 'attached',
    };
    const failingIcmpCheck: ResourceAvailabilityMeta = {
      targetId: 'db-1-uptime-ping',
      linkedResourceId: 'agent-db-1',
      name: 'db-1 ping',
      targetKind: 'machine',
      address: '192.0.2.40',
      protocol: 'icmp',
      probeOutcome: 'failed',
      transportOutcome: 'unreachable',
      enabled: true,
      available: false,
      lastChecked: '2026-10-07T12:01:00Z',
      consecutiveFailures: 3,
      lastError: 'ping timed out',
      failureThreshold: 3,
      pollIntervalSeconds: 30,
      timeoutMillis: 2000,
      correlationState: 'attached',
    };
    // The backend orders checks by target id and, while both pass, keeps the
    // first as the summary; a confirmed ping outage outranks it.
    const passingIcmpCheck: ResourceAvailabilityMeta = {
      targetId: 'db-1-uptime-ping',
      linkedResourceId: 'agent-db-1',
      name: 'db-1 ping',
      targetKind: 'machine',
      address: '192.0.2.40',
      protocol: 'icmp',
      probeOutcome: 'reachable',
      transportOutcome: 'reachable',
      enabled: true,
      available: true,
      lastChecked: '2026-10-07T12:00:00Z',
      lastSuccess: '2026-10-07T12:00:00Z',
      latencyMillis: 1,
      failureThreshold: 3,
      pollIntervalSeconds: 30,
      timeoutMillis: 2000,
      correlationState: 'attached',
    };
    const machineRow = (checks: ResourceAvailabilityMeta[], summary: ResourceAvailabilityMeta) =>
      ({
        id: 'agent-db-1',
        type: 'agent',
        name: 'db-1',
        displayName: 'db-1',
        platformId: 'db-1',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent', 'availability'],
        status: 'online',
        lastSeen: Date.parse('2026-10-07T12:01:00Z'),
        agent: { agentId: 'agent-db-1', hostname: 'db-1' },
        availability: summary,
        availabilityChecks: checks,
      }) as Resource;

    const [previous] = mergeCanonicalResourceSnapshot(
      [machineRow([passingTcpCheck, passingIcmpCheck], passingTcpCheck)],
      [],
    );
    const incoming = machineRow([passingTcpCheck, failingIcmpCheck], failingIcmpCheck);

    const [snapshot] = mergeCanonicalResourceSnapshot([structuredClone(incoming)], [previous]);
    const [delta] = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(incoming)],
      [previous],
      new Set([incoming.id]),
      new Map([[incoming.id, ['availability']]]),
    );

    for (const merged of [snapshot, delta]) {
      expect(merged.availability).toEqual(failingIcmpCheck);
      expect((merged.platformData as Record<string, unknown> | undefined)?.availability).toEqual(
        failingIcmpCheck,
      );
      expect(merged.availabilityChecks).toEqual([passingTcpCheck, failingIcmpCheck]);
    }
  });

  it('drops the plural check mirror when the availability source is withdrawn', () => {
    const check: ResourceAvailabilityMeta = {
      targetId: 'db-1-icmp',
      linkedResourceId: 'agent-db-1',
      address: '192.0.2.40',
      protocol: 'icmp',
      enabled: true,
      available: true,
      lastChecked: '2026-10-07T12:00:00Z',
      correlationState: 'attached',
    };
    // REST hydration mirrors both facets into platformData.
    const restRow = {
      id: 'agent-db-1',
      type: 'agent',
      name: 'db-1',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent', 'availability'],
      status: 'online',
      lastSeen: Date.parse('2026-10-07T12:00:00Z'),
      availability: check,
      availabilityChecks: [check],
      platformData: {
        sources: ['agent', 'availability'],
        agent: { agentId: 'agent-db-1', hostname: 'db-1' },
        availability: check,
        availabilityChecks: [check],
      },
    } as unknown as Resource;
    // The check was deleted: the realtime row no longer lists the source.
    const realtimeRow = {
      id: 'agent-db-1',
      type: 'agent',
      name: 'db-1',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      status: 'online',
      lastSeen: Date.parse('2026-10-07T12:01:00Z'),
      platformData: {
        sources: ['agent'],
        agent: { agentId: 'agent-db-1', hostname: 'db-1' },
      },
    } as unknown as Resource;

    const [merged] = mergeCanonicalResourceSnapshot([realtimeRow], [restRow]);
    // Hydration merges the realtime projection over the REST cache once more.
    const [rehydrated] = mergeCanonicalResourceSnapshot([merged], [restRow]);

    for (const row of [merged, rehydrated]) {
      expect(row.availability).toBeUndefined();
      expect(row.availabilityChecks).toBeUndefined();
      expect((row.platformData as Record<string, unknown>).availability).toBeUndefined();
      expect((row.platformData as Record<string, unknown>).availabilityChecks).toBeUndefined();
    }
  });
});

describe('incremental canonical resource snapshots', () => {
  it('clears a node sensor setup verdict when the next snapshot sends false', () => {
    // Facets merge field by field and an omitted field is read as a partial
    // snapshot, so the registry sends every Proxmox node an explicit verdict.
    const node = (sensorSetupOutdated?: boolean) =>
      ({
        id: 'agent-pve3',
        type: 'agent',
        name: 'pve3',
        status: 'online',
        sources: ['proxmox'],
        proxmox: { nodeName: 'pve3', sensorSetupOutdated },
      }) as unknown as Resource;

    const [cleared] = mergeCanonicalResourceSnapshot([node(false)], [node(true)]);
    expect(cleared?.proxmox?.sensorSetupOutdated).toBe(false);

    const [omitted] = mergeCanonicalResourceSnapshot(
      [{ ...node(), proxmox: { nodeName: 'pve3' } } as unknown as Resource],
      [node(true)],
    );
    expect(omitted?.proxmox?.sensorSetupOutdated).toBe(true);
  });

  it('preserves untouched row identity while refreshing changed resources', () => {
    const unchanged = {
      id: 'vm-unchanged',
      type: 'vm',
      name: 'vm-unchanged',
      status: 'running',
      cpu: { current: 10 },
    } as Resource;
    const changed = {
      id: 'vm-changed',
      type: 'vm',
      name: 'vm-changed',
      status: 'running',
      cpu: { current: 20 },
    } as Resource;
    const incoming = [
      structuredClone(unchanged),
      { ...structuredClone(changed), cpu: { current: 75 } },
    ];

    const result = mergeCanonicalResourceDeltaSnapshot(
      incoming,
      [unchanged, changed],
      new Set(['vm-changed']),
    );

    expect(result[0]).toBe(unchanged);
    expect(result[1]).not.toBe(changed);
    expect(result[1]?.cpu?.current).toBe(75);
  });

  it('preserves agent identity when no member of its host-merge group changed', () => {
    const agent = {
      id: 'agent-a',
      type: 'agent',
      name: 'host-a',
      status: 'online',
      sources: ['agent'],
    } as unknown as Resource;
    const vm = {
      id: 'vm-1',
      type: 'vm',
      name: 'vm-1',
      status: 'running',
      cpu: { current: 5 },
    } as Resource;
    const incoming = [structuredClone(agent), { ...structuredClone(vm), cpu: { current: 60 } }];

    const result = mergeCanonicalResourceDeltaSnapshot(incoming, [agent, vm], new Set(['vm-1']));

    expect(result[0]).toBe(agent);
    expect(result[1]?.cpu?.current).toBe(60);
  });

  it('refreshes an agent when its own id is in the delta', () => {
    const agent = {
      id: 'agent-a',
      type: 'agent',
      name: 'host-a',
      status: 'online',
      cpu: { current: 10 },
      sources: ['agent'],
    } as unknown as Resource;
    const incoming = [{ ...structuredClone(agent), cpu: { current: 90 } }];

    const result = mergeCanonicalResourceDeltaSnapshot(incoming, [agent], new Set(['agent-a']));

    expect(result[0]).not.toBe(agent);
    expect(result[0]?.cpu?.current).toBe(90);
  });

  it('refreshes agent groups when a changed id is absent from the incoming snapshot', () => {
    // A flagged id that no longer appears can be a removal or a partner id an
    // earlier coalesce folded away; either can alter a group without flagging
    // its surviving member, so such ticks refresh every agent group.
    const agent = {
      id: 'agent-a',
      type: 'agent',
      name: 'host-a',
      status: 'online',
      sources: ['agent'],
    } as unknown as Resource;
    const incoming = [structuredClone(agent)];

    const result = mergeCanonicalResourceDeltaSnapshot(incoming, [agent], new Set(['gone-id']));

    expect(result[0]).not.toBe(agent);
    expect(result[0]?.id).toBe('agent-a');
  });

  it('still coalesces a dirty host-merge group into one merged agent', () => {
    const agentSide = {
      id: 'agent-a',
      type: 'agent',
      name: 'host-a',
      status: 'online',
      sources: ['agent'],
    } as unknown as Resource;
    const platformSide = {
      id: 'pve-a',
      type: 'agent',
      name: 'host-a',
      status: 'online',
      sources: ['proxmox-pve'],
    } as unknown as Resource;
    const incoming = [structuredClone(agentSide), structuredClone(platformSide)];

    const result = mergeCanonicalResourceDeltaSnapshot(incoming, [], new Set(['pve-a']));

    expect(result).toHaveLength(1);
    expect(result[0]?.sources).toEqual(expect.arrayContaining(['agent', 'proxmox-pve']));
  });
});

describe('fast merge path for metrics-only delta patches', () => {
  // Mirrors the wire shape of a PVE guest metrics tick captured from the mock
  // estate: top-level metric subtrees, the proxmox facet mirror, and the four
  // platformData metric mirror leaves.
  const createPveGuestRaw = (): Resource =>
    ({
      id: 'vm-fast-1',
      type: 'vm',
      name: 'vm-fast-1',
      displayName: 'VM Fast 1',
      platformId: 'pve-node-1',
      platformType: 'proxmox-pve',
      sourceType: 'api',
      status: 'running',
      lastSeen: 1700000000000,
      uptime: 1000,
      cpu: { current: 10 },
      memory: { current: 20, total: 1024, used: 256, free: 768 },
      disk: { current: 30, total: 2048, used: 512, free: 1536 },
      network: { rxBytes: 100, txBytes: 200 },
      diskIO: { readRate: 5, writeRate: 7 },
      tags: ['prod'],
      canonicalIdentity: { canonicalId: 'vm-fast-1', aliases: ['vm/100'] },
      proxmox: {
        vmid: 100,
        nodeName: 'pve-node-1',
        memory: { free: 768, usage: 20, used: 256 },
        uptime: 1000,
      },
      platformData: {
        sources: ['proxmox-pve'],
        instance: 'pve-node-1',
        vmid: 100,
        diskRead: 5,
        diskWrite: 7,
        networkIn: 100,
        networkOut: 200,
        proxmox: { vmid: 100, nodeName: 'pve-node-1' },
      },
    }) as unknown as Resource;

  const METRICS_PATCH_KEYS = [
    'cpu',
    'uptime',
    'lastSeen',
    'memory',
    'network',
    'diskIO',
    'proxmox',
    'platformData.diskRead',
    'platformData.diskWrite',
    'platformData.networkIn',
    'platformData.networkOut',
  ] as const;

  const applyMetricsTick = (raw: Resource): Resource => {
    const next = structuredClone(raw) as unknown as Record<string, unknown>;
    next.cpu = { current: 55 };
    next.uptime = 1002;
    next.lastSeen = 1700000002000;
    next.memory = { current: 40, total: 1024, used: 400, free: 624 };
    next.network = { rxBytes: 300, txBytes: 400 };
    next.diskIO = { readRate: 9, writeRate: 11 };
    next.proxmox = {
      ...(next.proxmox as Record<string, unknown>),
      memory: { free: 624, usage: 40, used: 400 },
      uptime: 1002,
    };
    next.platformData = {
      ...(next.platformData as Record<string, unknown>),
      diskRead: 9,
      diskWrite: 11,
      networkIn: 300,
      networkOut: 400,
    };
    return next as unknown as Resource;
  };

  const seedDisplayRows = (raw: Resource[]): Resource[] =>
    mergeCanonicalResourceDeltaSnapshot(
      raw.map((row) => structuredClone(row)),
      [],
      new Set(raw.map((row) => row.id)),
    );

  it('clears omitted guest read-state fields on a new native outcome in full and fast deltas', () => {
    const raw = createPveGuestRaw();
    raw.proxmox = {
      ...raw.proxmox,
      runtimeStatus: 'running',
      guestAgentStatus: 'deferred',
      diskStatusReason: 'prev-vm-locked',
      guestAgentExpected: true,
      lock: 'backup',
    };
    const previous = seedDisplayRows([raw]);
    const incoming = structuredClone(raw);
    incoming.proxmox = {
      vmid: 100,
      nodeName: 'pve-node-1',
      runtimeStatus: 'running',
      guestAgentStatus: 'available',
    };
    const changed = new Set([raw.id]);
    const keys = new Map([[raw.id, ['proxmox']]]);
    expect(getFastResourceMergePatchKeys(keys, raw.id, previous[0])).toEqual(['proxmox']);
    const full = mergeCanonicalResourceSnapshot([incoming], previous);
    const delta = mergeCanonicalResourceDeltaSnapshot([incoming], previous, changed);
    const fast = mergeCanonicalResourceDeltaSnapshot([incoming], previous, changed, keys);
    expect(fast[0]).toEqual(delta[0]);
    expect(fast[0]).toEqual(full[0]);
    for (const [merged] of [full, delta, fast]) {
      expect(merged.proxmox?.guestAgentStatus).toBe('available');
      expect(merged.proxmox?.diskStatusReason).toBeUndefined();
      expect(merged.proxmox?.lock).toBeUndefined();
      expect(merged.proxmox?.guestAgentExpected).toBeUndefined();
      expect(merged.proxmox?.uptime).toBe(1000); // unrelated richer facet is retained
      expect(buildFastResourceStorePatchOps(merged, ['proxmox'])).toContainEqual({
        key: 'proxmox',
        value: merged.proxmox,
        mode: 'reconcile',
      });
    }
  });

  it('applies the same read-state clearing to the compatibility provider facet', () => {
    const raw = createPveGuestRaw();
    raw.proxmox = {
      ...raw.proxmox,
      runtimeStatus: 'running',
      guestAgentStatus: 'deferred',
      diskStatusReason: 'prev-vm-locked',
      guestAgentExpected: true,
      lock: 'backup',
    };
    raw.platformData = { ...raw.platformData, proxmox: raw.proxmox };
    const previous = seedDisplayRows([raw]);
    const incoming = structuredClone(raw);
    incoming.proxmox = {
      vmid: 100,
      nodeName: 'pve-node-1',
      runtimeStatus: 'running',
      guestAgentStatus: 'available',
    };
    incoming.platformData = { ...incoming.platformData, proxmox: incoming.proxmox };
    const [merged] = mergeCanonicalResourceSnapshot([incoming], previous);
    expect(merged.platformData?.proxmox).toEqual(merged.proxmox);
    expect(merged.proxmox?.diskStatusReason).toBeUndefined();
    expect(merged.proxmox?.lock).toBeUndefined();
  });

  it('preserves read-state evidence on partial facet omission, including the fast path', () => {
    const raw = createPveGuestRaw();
    raw.proxmox = {
      ...raw.proxmox,
      runtimeStatus: 'running',
      guestAgentStatus: 'deferred',
      diskStatusReason: 'prev-agent-timeout',
      guestAgentExpected: true,
      lock: 'backup',
    };
    const previous = seedDisplayRows([raw]);
    for (const proxmox of [
      undefined,
      { vmid: 100, uptime: 1001 },
      { guestAgentStatus: 'available' },
      { vmid: 0, runtimeStatus: 'running', guestAgentStatus: 'available' },
      { vmid: 1.5, runtimeStatus: 'running', guestAgentStatus: 'available' },
      { vmid: Infinity, runtimeStatus: 'running', guestAgentStatus: 'available' },
      { vmid: 100, runtimeStatus: ' ', guestAgentStatus: 'available' },
      { vmid: 100, runtimeStatus: 'running', guestAgentStatus: ' ' },
    ]) {
      const incoming = { ...raw, proxmox } as Resource;
      const full = mergeCanonicalResourceSnapshot([incoming], previous);
      const fast = mergeCanonicalResourceDeltaSnapshot(
        [incoming],
        previous,
        new Set([raw.id]),
        new Map([[raw.id, ['proxmox']]]),
      );
      for (const [merged] of [full, fast]) {
        expect(merged.proxmox).toMatchObject({
          diskStatusReason: 'prev-agent-timeout',
          guestAgentExpected: true,
          lock: 'backup',
        });
      }
    }
  });

  it('keeps explicit lock and deferral evidence on a new outcome, including false expected', () => {
    const raw = createPveGuestRaw();
    raw.proxmox = {
      ...raw.proxmox,
      runtimeStatus: 'running',
      guestAgentStatus: 'available',
    };
    const previous = seedDisplayRows([raw]);
    const incoming = structuredClone(raw);
    incoming.proxmox = {
      ...raw.proxmox,
      guestAgentStatus: 'deferred',
      diskStatusReason: 'agent-busy',
      guestAgentExpected: false,
      lock: 'migrate',
    };
    const [merged] = mergeCanonicalResourceDeltaSnapshot(
      [incoming],
      previous,
      new Set([raw.id]),
      new Map([[raw.id, ['proxmox']]]),
    );
    expect(merged.proxmox).toMatchObject({
      guestAgentStatus: 'deferred',
      diskStatusReason: 'agent-busy',
      guestAgentExpected: false,
      lock: 'migrate',
    });
  });

  it('produces the same merged row as the full path for a metrics-only patch', () => {
    const raw = createPveGuestRaw();
    const display = seedDisplayRows([raw]);
    const patched = applyMetricsTick(raw);
    const changedIds = new Set(['vm-fast-1']);
    const changedKeys = new Map([['vm-fast-1', [...METRICS_PATCH_KEYS]]]);

    const fast = mergeCanonicalResourceDeltaSnapshot([patched], display, changedIds, changedKeys);
    const slow = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(patched)],
      display,
      changedIds,
    );

    expect(fast[0]).toEqual(slow[0]);
    expect(fast[0]?.cpu?.current).toBe(55);
    expect(fast[0]?.uptime).toBe(1002);
    expect((fast[0]?.platformData as Record<string, unknown>)?.diskRead).toBe(9);
    expect((fast[0]?.proxmox as Record<string, unknown>)?.uptime).toBe(1002);
  });

  it('keeps unpatched subtree identity on the fast path', () => {
    const raw = createPveGuestRaw();
    const display = seedDisplayRows([raw]);
    const patched = applyMetricsTick(raw);
    const changedKeys = new Map([['vm-fast-1', [...METRICS_PATCH_KEYS]]]);

    const fast = mergeCanonicalResourceDeltaSnapshot(
      [patched],
      display,
      new Set(['vm-fast-1']),
      changedKeys,
    );

    expect(fast[0]).not.toBe(display[0]);
    expect(fast[0]?.canonicalIdentity).toBe(display[0]?.canonicalIdentity);
    expect(fast[0]?.disk).toBe(display[0]?.disk);
    expect(fast[0]?.tags).toBe(display[0]?.tags);
    // platformData is rebuilt for the patched leaves but its untouched nested
    // records keep identity so downstream reconciles short-circuit on them.
    expect((fast[0]?.platformData as Record<string, unknown>)?.proxmox).toBe(
      (display[0]?.platformData as Record<string, unknown>)?.proxmox,
    );
    // The fast row must not adopt the raw row's subtrees: reconcile mutates
    // adopted objects in place and would corrupt the raw delta baseline.
    expect(fast[0]?.cpu).not.toBe(patched.cpu);
    expect(fast[0]?.proxmox).not.toBe(patched.proxmox);
  });

  it('falls back to the full merge when a patch touches an ineligible key', () => {
    const raw = createPveGuestRaw();
    const display = seedDisplayRows([raw]);
    const patched = {
      ...applyMetricsTick(raw),
      tags: ['prod', 'new-tag'],
    } as unknown as Resource;
    const changedKeys = new Map([['vm-fast-1', [...METRICS_PATCH_KEYS, 'tags']]]);

    expect(getFastResourceMergePatchKeys(changedKeys, 'vm-fast-1', display[0])).toBeNull();

    const result = mergeCanonicalResourceDeltaSnapshot(
      [patched],
      display,
      new Set(['vm-fast-1']),
      changedKeys,
    );
    expect(result[0]?.tags).toEqual(['prod', 'new-tag']);
    expect(result[0]?.cpu?.current).toBe(55);
  });

  it('falls back to the full merge for platformData leaves outside the metric mirrors', () => {
    const display = seedDisplayRows([createPveGuestRaw()]);
    const changedKeys = new Map([['vm-fast-1', ['cpu', 'platformData.instance']]]);

    expect(getFastResourceMergePatchKeys(changedKeys, 'vm-fast-1', display[0])).toBeNull();
  });

  it('treats unknown change shapes and agent rows as ineligible', () => {
    const display = seedDisplayRows([createPveGuestRaw()]);
    expect(
      getFastResourceMergePatchKeys(new Map([['vm-fast-1', null]]), 'vm-fast-1', display[0]),
    ).toBeNull();
    expect(getFastResourceMergePatchKeys(undefined, 'vm-fast-1', display[0])).toBeNull();
    expect(
      getFastResourceMergePatchKeys(new Map([['vm-fast-1', ['cpu']]]), 'vm-fast-1', undefined),
    ).toBeNull();

    const agent = { ...createPveGuestRaw(), type: 'agent' } as unknown as Resource;
    expect(
      getFastResourceMergePatchKeys(new Map([['vm-fast-1', ['cpu']]]), 'vm-fast-1', agent),
    ).toBeNull();
  });

  it('keeps the existing value when a fast key was deleted from the raw row', () => {
    const raw = createPveGuestRaw();
    const display = seedDisplayRows([raw]);
    const patched = structuredClone(raw) as unknown as Record<string, unknown>;
    delete patched.diskIO;
    const changedKeys = new Map([['vm-fast-1', ['diskIO']]]);

    const fast = mergeCanonicalResourceDeltaSnapshot(
      [patched as unknown as Resource],
      display,
      new Set(['vm-fast-1']),
      changedKeys,
    );
    const slow = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(patched) as unknown as Resource],
      display,
      new Set(['vm-fast-1']),
    );

    expect(fast[0]?.diskIO).toEqual(display[0]?.diskIO);
    expect(fast[0]).toEqual(slow[0]);
  });

  it('unions changed keys across ticks with null contamination', () => {
    expect(unionResourceChangedKeys(['cpu'], ['memory', 'cpu'])).toEqual(['cpu', 'memory']);
    expect(unionResourceChangedKeys(['cpu'], null)).toBeNull();
    expect(unionResourceChangedKeys(null, ['cpu'])).toBeNull();
    expect(unionResourceChangedKeys(undefined, ['cpu'])).toEqual(['cpu']);
    expect(unionResourceChangedKeys(['cpu'], undefined)).toEqual(['cpu']);
  });

  it('builds per-key store patch ops for a fast row', () => {
    const raw = createPveGuestRaw();
    const display = seedDisplayRows([raw]);
    const patched = applyMetricsTick(raw);
    const changedKeys = new Map([['vm-fast-1', [...METRICS_PATCH_KEYS]]]);
    const fast = mergeCanonicalResourceDeltaSnapshot(
      [patched],
      display,
      new Set(['vm-fast-1']),
      changedKeys,
    );

    const ops = buildFastResourceStorePatchOps(fast[0]!, [...METRICS_PATCH_KEYS]);
    const byTarget = new Map(ops.map((op) => [op.leaf ? `${op.key}.${op.leaf}` : op.key, op]));

    expect(byTarget.get('cpu')?.mode).toBe('reconcile');
    expect(byTarget.get('uptime')).toEqual({ key: 'uptime', value: 1002, mode: 'set' });
    expect(byTarget.get('platformData.diskRead')).toEqual({
      key: 'platformData',
      leaf: 'diskRead',
      value: 9,
      mode: 'set',
    });
    expect(byTarget.get('proxmox')?.mode).toBe('reconcile');
    // No op may reference platformData wholesale; only leaf writes are allowed
    // so unpatched platformData subtrees never get touched in the store.
    expect(ops.every((op) => op.key !== 'platformData' || op.leaf !== undefined)).toBe(true);
  });
});
