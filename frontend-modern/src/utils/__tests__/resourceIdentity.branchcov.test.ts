import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import type { Agent, Node } from '@/types/api';
import {
  getInfrastructureDiscoveryHostname,
  getInfrastructureMetadataId,
  getPreferredResourceDisplayName,
  getPreferredResourceIP,
  getPreferredWorkloadsAgentHint,
  getPrimaryResourceIdentity,
  getResourceIdentityAliases,
  resolveGuestUrlWithIdentity,
} from '@/utils/resourceIdentity';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource-1',
    type: 'agent',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'resource-1',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('getPrimaryResourceIdentity (branch coverage)', () => {
  it('falls through a whitespace-only canonical primary id to lower-priority sources', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          canonicalIdentity: { primaryId: '   ' },
          metricsTarget: { resourceType: 'docker-host', resourceId: 'docker-host-1' },
        }),
      ),
    ).toBe('docker-host:docker-host-1');
  });

  it('skips the metrics identity when the metrics resource type is whitespace', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          metricsTarget: {
            resourceType: '   ',
            resourceId: 'x',
          } as unknown as Resource['metricsTarget'],
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-1',
            resourceId: 'agent-1',
          },
        }),
      ),
    ).toBe('agent:agent-1');
  });

  it('prefixes the kubernetes cluster id for k8s-cluster, k8s-node, and pod resources', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          type: 'k8s-cluster',
          kubernetes: { clusterName: 'prod-eu' },
        }),
      ),
    ).toBe('k8s:prod-eu');

    expect(
      getPrimaryResourceIdentity(
        makeResource({
          type: 'k8s-node',
          kubernetes: { context: 'worker-ctx' },
        }),
      ),
    ).toBe('k8s:worker-ctx');

    expect(
      getPrimaryResourceIdentity(
        makeResource({
          type: 'pod',
          kubernetes: { clusterName: 'prod-eu' },
        }),
      ),
    ).toBe('k8s:prod-eu');
  });

  it('falls back to the pbs and pmg instance ids under platformData', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          type: 'vm',
          platformData: { pbs: { instanceId: 'pbs-inst-1' } },
        }),
      ),
    ).toBe('pbs:pbs-inst-1');

    expect(
      getPrimaryResourceIdentity(
        makeResource({
          type: 'vm',
          platformData: { pmg: { instanceId: 'pmg-inst-1' } },
        }),
      ),
    ).toBe('pmg:pmg-inst-1');
  });

  it('falls back to the raw resource id when no identity source resolves', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          id: 'lonely-vm',
          type: 'vm',
        }),
      ),
    ).toBe('lonely-vm');
  });

  it('does not emit a docker-host prefix when the docker runtime id is missing', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          id: 'docker-host-bare',
          type: 'docker-host',
        }),
      ),
    ).toBe('docker-host-bare');
  });
});

describe('getPreferredWorkloadsAgentHint (branch coverage)', () => {
  it('returns undefined for resource types outside the docker/truenas/agent families', () => {
    expect(getPreferredWorkloadsAgentHint(makeResource({ type: 'vm' }))).toBeUndefined();
    expect(getPreferredWorkloadsAgentHint(makeResource({ type: 'k8s-cluster' }))).toBeUndefined();
  });

  it('falls back to the shared hostname when the docker hostname is absent', () => {
    expect(
      getPreferredWorkloadsAgentHint(
        makeResource({
          type: 'docker-host',
          name: 'dock-01',
          platformData: { docker: {} },
        }),
      ),
    ).toBe('dock-01');
  });

  it('falls back to the shared hostname when the truenas hostname is absent', () => {
    expect(
      getPreferredWorkloadsAgentHint(
        makeResource({
          type: 'agent',
          platformType: 'truenas',
          name: 'nas-box',
          platformData: { truenas: {} },
        }),
      ),
    ).toBe('nas-box');
  });

  it('falls back to the shared hostname when the proxmox node name is absent', () => {
    expect(
      getPreferredWorkloadsAgentHint(
        makeResource({
          type: 'agent',
          platformType: 'agent',
          name: 'pve-host',
          platformData: { proxmox: {} },
        }),
      ),
    ).toBe('pve-host');
  });
});

describe('getPreferredResourceIP (branch coverage)', () => {
  it('prefers a routable IPv6 address over an unusable link-local IPv6', () => {
    const resource = makeResource({
      identity: { ips: ['fe80::1', '2001:db8::1'] },
    });
    expect(getPreferredResourceIP(resource)).toBe('2001:db8::1');
  });

  it('treats a loopback IPv4 as the only (unusable) candidate and still returns it', () => {
    const resource = makeResource({ identity: { ips: ['127.0.0.1'] } });
    expect(getPreferredResourceIP(resource)).toBe('127.0.0.1');
  });

  it('scores a public IPv4 above a carrier-grade NAT address', () => {
    const resource = makeResource({ identity: { ips: ['100.64.0.1', '8.8.8.8'] } });
    expect(getPreferredResourceIP(resource)).toBe('8.8.8.8');
  });

  it('returns a non-IP string untouched when it is the only candidate', () => {
    const resource = makeResource({ identity: { ips: ['host-name'] } });
    expect(getPreferredResourceIP(resource)).toBe('host-name');
  });

  it('normalizes bracketed, CIDR-suffixed, and zone-tagged interface addresses', () => {
    const bracketed = makeResource({
      agent: {
        networkInterfaces: [{ name: 'eth0', addresses: ['[192.168.0.5]'] }],
      },
    });
    expect(getPreferredResourceIP(bracketed)).toBe('192.168.0.5');

    const cidr = makeResource({
      agent: {
        networkInterfaces: [{ name: 'eth0', addresses: ['10.0.0.9/24'] }],
      },
    });
    expect(getPreferredResourceIP(cidr)).toBe('10.0.0.9');

    const zoned = makeResource({
      agent: {
        networkInterfaces: [{ name: 'eth0', addresses: ['2001:db8::1%eth0'] }],
      },
    });
    expect(getPreferredResourceIP(zoned)).toBe('2001:db8::1');
  });

  it('skips empty and whitespace-only interface addresses', () => {
    const resource = makeResource({
      agent: {
        networkInterfaces: [{ name: 'eth0', addresses: ['', '   ', '\t'] }],
      },
    });
    expect(getPreferredResourceIP(resource)).toBeUndefined();
  });

  it('penalizes virtual interfaces so a physical interface wins with the same family', () => {
    const resource = makeResource({
      agent: {
        networkInterfaces: [
          { name: 'docker0', addresses: ['172.18.0.1/16'] },
          { name: 'eth0', addresses: ['172.18.0.1/24'] },
        ],
      },
    });
    expect(getPreferredResourceIP(resource)).toBe('172.18.0.1');
  });
});

describe('getPreferredResourceDisplayName (branch coverage)', () => {
  it('uses the canonical display name when the resource display name is empty', () => {
    expect(
      getPreferredResourceDisplayName(
        makeResource({
          displayName: '',
          canonicalIdentity: { displayName: 'Canonical Label' },
        }),
      ),
    ).toBe('Canonical Label');
  });

  it('follows the normal path when a policy does not require governed display', () => {
    expect(
      getPreferredResourceDisplayName(
        makeResource({
          displayName: 'Visible',
          policy: { sensitivity: 'internal', routing: { scope: 'cloud-summary' } },
        }),
      ),
    ).toBe('Visible');
  });

  it('falls all the way back to the primary resource identity when no label resolves', () => {
    expect(
      getPreferredResourceDisplayName(
        makeResource({
          id: 'identity-only',
          type: 'vm',
          name: '',
          displayName: '',
          platformId: '',
        }),
      ),
    ).toBe('identity-only');
  });
});

describe('getInfrastructureMetadataId (branch coverage)', () => {
  const nodeWith = (overrides: Partial<Pick<Node, 'id' | 'name' | 'linkedAgentId'>>) =>
    ({ id: 'node-1', name: 'pve1', ...overrides }) as Pick<Node, 'id' | 'name' | 'linkedAgentId'>;

  it('falls back to the node id when no agent and no linked agent id resolve', () => {
    expect(getInfrastructureMetadataId(nodeWith({ linkedAgentId: undefined }))).toBe('node-1');
  });

  it('falls back to the node name when the id is empty and nothing else resolves', () => {
    expect(
      getInfrastructureMetadataId(nodeWith({ id: '', name: 'pve1', linkedAgentId: undefined })),
    ).toBe('pve1');
  });

  it('falls through an agent whose metadata ids are all whitespace', () => {
    const agent = {
      id: '   ',
      hostname: 'tower.local',
      status: 'online',
      lastSeen: Date.now(),
    } as unknown as Agent;
    expect(getInfrastructureMetadataId(nodeWith({ linkedAgentId: 'agent-linked' }), agent)).toBe(
      'agent-linked',
    );
  });
});

describe('getInfrastructureDiscoveryHostname / getAgentLikeDiscoveryHostname (branch coverage)', () => {
  it('prefers the canonical hostname over discovery, agent, and platform hostnames', () => {
    const agent = {
      id: 'a1',
      hostname: 'agent.host',
      status: 'online',
      lastSeen: Date.now(),
      canonicalIdentity: { hostname: 'canonical.host' },
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'a1',
        resourceId: 'a1',
        hostname: 'disc.host',
      },
      platformData: { agent: { hostname: 'plat.host' } },
    } as unknown as Agent;
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' }, agent)).toBe('canonical.host');
  });

  it('falls back to the discovery target hostname when canonical is absent', () => {
    const agent = {
      id: 'a1',
      hostname: 'agent.host',
      status: 'online',
      lastSeen: Date.now(),
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'a1',
        resourceId: 'a1',
        hostname: 'disc.host',
      },
    } as unknown as Agent;
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' }, agent)).toBe('disc.host');
  });

  it('falls back to the agent hostname when canonical and discovery are absent', () => {
    const agent = {
      id: 'a1',
      hostname: 'agent.host',
      status: 'online',
      lastSeen: Date.now(),
    } as unknown as Agent;
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' }, agent)).toBe('agent.host');
  });

  it('falls back to the platform agent hostname when no other hostname resolves', () => {
    const agent = {
      id: 'a1',
      status: 'online',
      lastSeen: Date.now(),
      platformData: { agent: { hostname: 'plat.host' } },
    } as unknown as Agent;
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' }, agent)).toBe('plat.host');
  });

  it('returns the node name when no agent is provided', () => {
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' })).toBe('pve1');
  });
});

describe('resolveGuestUrlWithIdentity (branch coverage)', () => {
  const withIP = (ip: string): Resource => makeResource({ identity: { ips: [ip] } });

  it('substitutes the LAN IP into an http URL and preserves port and path', () => {
    expect(resolveGuestUrlWithIdentity('http://pi:8006/console', withIP('192.168.0.2'))).toBe(
      'http://192.168.0.2:8006/console',
    );
  });

  it('substitutes the LAN IP into a url that has no port', () => {
    expect(resolveGuestUrlWithIdentity('https://pi', withIP('192.168.0.2'))).toBe(
      'https://192.168.0.2',
    );
  });

  it('returns the original url when the scheme is not http or https', () => {
    expect(resolveGuestUrlWithIdentity('ftp://pi', withIP('192.168.0.2'))).toBe('ftp://pi');
  });

  it('returns the original url when no protocol authority is present', () => {
    expect(resolveGuestUrlWithIdentity('pi:8006', withIP('192.168.0.2'))).toBe('pi:8006');
  });

  it('returns the original url when the host is a bracketed IPv6 literal that the regex cannot capture', () => {
    expect(resolveGuestUrlWithIdentity('https://[::1]:8006', withIP('192.168.0.2'))).toBe(
      'https://[::1]:8006',
    );
  });

  it('returns the original bare-host url when every candidate address normalizes to empty', () => {
    const resource = makeResource({
      agent: { networkInterfaces: [{ name: 'eth0', addresses: ['', '   '] }] },
    });
    expect(resolveGuestUrlWithIdentity('https://pi:8006', resource)).toBe('https://pi:8006');
  });
});

describe('dedupeTrimmedValues via getResourceIdentityAliases (branch coverage)', () => {
  it('drops whitespace-only and empty alias values', () => {
    expect(
      getResourceIdentityAliases(
        makeResource({
          metricsTarget: { resourceType: 'docker-host', resourceId: '   ' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: '',
            resourceId: '',
          },
          identity: { hostname: '\t' },
        }),
      ),
    ).toEqual([]);
  });

  it('deduplicates case-insensitively while keeping the first observed casing', () => {
    expect(
      getResourceIdentityAliases(
        makeResource({
          metricsTarget: { resourceType: 'docker-host', resourceId: 'Host-A' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-1',
            resourceId: 'host-a',
          },
        }),
      ),
    ).toEqual(['Host-A', 'agent-1']);
  });
});
