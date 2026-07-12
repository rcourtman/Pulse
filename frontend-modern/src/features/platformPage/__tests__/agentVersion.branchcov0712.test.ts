import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { collectOutdatedAgentHosts, hostAgentConnectionID } from '../agentVersion';

const host = (over: Partial<Resource>): Resource =>
  ({ id: 'x', name: 'host', type: 'docker-host', ...over }) as Resource;

describe('hostAgentConnectionID', () => {
  it('returns undefined for a plain host with no agent id anywhere', () => {
    expect(hostAgentConnectionID(host({}))).toBeUndefined();
  });

  it('passes through an already-prefixed agent meta id without re-prefixing', () => {
    expect(
      hostAgentConnectionID(host({ agent: { agentId: 'agent:abc-1' } as Resource['agent'] })),
    ).toBe('agent:abc-1');
  });

  it('trims whitespace on the agent meta id before prefixing', () => {
    expect(
      hostAgentConnectionID(
        host({ agent: { agentId: '   agent-delly   ' } as Resource['agent'] }),
      ),
    ).toBe('agent:agent-delly');
  });

  it('falls back to kubernetes.agentId for a non-kubernetes platform row', () => {
    expect(
      hostAgentConnectionID(
        host({ kubernetes: { agentId: 'agent-k8s' } as Resource['kubernetes'] }),
      ),
    ).toBe('agent:agent-k8s');
  });

  it('uses host.id for a type="agent" row with no agent meta id', () => {
    expect(hostAgentConnectionID(host({ type: 'agent', id: 'agent-123' }))).toBe(
      'agent:agent-123',
    );
  });

  it('uses an already-prefixed host.id for a type="agent" row without re-prefixing', () => {
    expect(hostAgentConnectionID(host({ type: 'agent', id: 'agent:from-id' }))).toBe(
      'agent:from-id',
    );
  });

  it('treats a k8s-cluster row as a kubernetes platform row', () => {
    expect(
      hostAgentConnectionID(
        host({
          type: 'k8s-cluster',
          kubernetes: { agentId: 'agent:cluster-1' } as Resource['kubernetes'],
        }),
      ),
    ).toBe('agent:cluster-1');
  });

  it('treats platformType="kubernetes" as a kubernetes platform row and prefixes a bare id', () => {
    expect(
      hostAgentConnectionID(
        host({
          type: 'docker-host',
          platformType: 'kubernetes',
          kubernetes: { agentId: 'node-9' } as Resource['kubernetes'],
        }),
      ),
    ).toBe('agent:node-9');
  });

  it('treats a sources entry of "kubernetes" as a kubernetes platform row', () => {
    expect(
      hostAgentConnectionID(
        host({
          type: 'docker-host',
          sources: ['kubernetes'],
          kubernetes: { agentId: 'agent:src-1' } as Resource['kubernetes'],
        }),
      ),
    ).toBe('agent:src-1');
  });

  it('falls through to agent meta when a kubernetes row has no cluster agent id', () => {
    expect(
      hostAgentConnectionID(
        host({
          type: 'k8s-node',
          agent: { agentId: 'agent-host' } as Resource['agent'],
        }),
      ),
    ).toBe('agent:agent-host');
  });

  it('trims whitespace on the kubernetes cluster agent id', () => {
    expect(
      hostAgentConnectionID(
        host({
          type: 'k8s-node',
          kubernetes: { agentId: '  agent-k8s  ' } as Resource['kubernetes'],
        }),
      ),
    ).toBe('agent:agent-k8s');
  });

  it('returns undefined for a kubernetes node row with no ids at all', () => {
    expect(hostAgentConnectionID(host({ type: 'k8s-node' }))).toBeUndefined();
  });

  it('tolerates a sparse resource object that only carries a type', () => {
    const sparse = { type: 'docker-host' } as unknown as Parameters<
      typeof hostAgentConnectionID
    >[0];
    expect(hostAgentConnectionID(sparse)).toBeUndefined();
  });
});

describe('collectOutdatedAgentHosts', () => {
  const server = 'v6.0.0-rc.6+git.151.gd8f5519ee';

  it('skips a host whose reported version is present but unparseable', () => {
    const hosts = [
      host({ name: 'mystery', docker: { agentVersion: 'dev' } as Resource['docker'] }),
    ];
    expect(collectOutdatedAgentHosts(hosts, server)).toEqual([]);
  });

  it('does not flag a host whose version is ahead of the server', () => {
    const hosts = [
      host({ name: 'ahead', docker: { agentVersion: 'v6.1.0' } as Resource['docker'] }),
    ];
    expect(collectOutdatedAgentHosts(hosts, 'v6.0.0')).toEqual([]);
  });

  it('falls back to the resource id when the name is blank', () => {
    const hosts = [
      host({
        id: 'host-7',
        name: '   ',
        docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'],
      }),
    ];
    expect(collectOutdatedAgentHosts(hosts, server)).toEqual([
      { name: 'host-7', version: 'v6.0.0-rc.5' },
    ]);
  });

  it('uses the literal "host" when both name and id are blank', () => {
    const hosts = [
      host({
        id: '',
        name: '',
        docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'],
      }),
    ];
    expect(collectOutdatedAgentHosts(hosts, server)).toEqual([
      { name: 'host', version: 'v6.0.0-rc.5' },
    ]);
  });

  it('attaches the kubernetes cluster agent id for an outdated k8s node', () => {
    const hosts = [
      host({
        name: 'knode-1',
        type: 'k8s-node',
        kubernetes: {
          agentId: 'agent:cluster-1',
          agentVersion: 'v6.0.0-rc.5',
        } as Resource['kubernetes'],
      }),
    ];
    expect(collectOutdatedAgentHosts(hosts, server)).toEqual([
      { name: 'knode-1', version: 'v6.0.0-rc.5', agentId: 'agent:cluster-1' },
    ]);
  });

  it('flags a type="agent" host using its host.id as the connection id', () => {
    const hosts = [
      host({
        name: 'unified-1',
        type: 'agent',
        id: 'unified-1-id',
        agent: { agentVersion: 'v6.0.0-rc.5' } as Resource['agent'],
      }),
    ];
    expect(collectOutdatedAgentHosts(hosts, server)).toEqual([
      { name: 'unified-1', version: 'v6.0.0-rc.5', agentId: 'agent:unified-1-id' },
    ]);
  });

  it('omits the agentId field when the outdated host has no connection id', () => {
    const hosts = [
      host({
        id: 'plain-1',
        name: 'plain',
        docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'],
      }),
    ];
    const result = collectOutdatedAgentHosts(hosts, server);
    expect(result).toEqual([{ name: 'plain', version: 'v6.0.0-rc.5' }]);
    expect(result[0]).not.toHaveProperty('agentId');
  });

  it('returns an empty list for null and undefined server versions', () => {
    const hosts = [
      host({ name: 'tower', docker: { agentVersion: 'v6.0.0-rc.5' } as Resource['docker'] }),
    ];
    expect(collectOutdatedAgentHosts(hosts, null)).toEqual([]);
    expect(collectOutdatedAgentHosts(hosts, undefined)).toEqual([]);
  });

  it('returns an empty list when there are no hosts', () => {
    expect(collectOutdatedAgentHosts([], server)).toEqual([]);
  });
});
