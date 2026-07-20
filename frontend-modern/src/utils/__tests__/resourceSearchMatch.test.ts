import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { resourceMatchesSearch } from '@/utils/resourceSearchMatch';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'res-1',
    type: 'agent',
    name: 'res-1',
    displayName: 'Resource One',
    platformId: 'res-1',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('resourceMatchesSearch', () => {
  describe('empty / nullish terms match everything', () => {
    it.each([
      ['null', null],
      ['undefined', undefined],
      ['empty string', ''],
      ['only spaces', '   '],
      ['tab + newline', '\t\n'],
    ])('returns true for %s', (_label, term) => {
      expect(resourceMatchesSearch(makeResource(), term)).toBe(true);
    });

    it('returns true for an empty term even when resource has no candidates', () => {
      const empty = makeResource({ id: '', name: '', displayName: '' });
      expect(resourceMatchesSearch(empty, '')).toBe(true);
    });
  });

  describe('case-insensitive substring matching on core identity fields', () => {
    it.each([
      ['id', { id: 'prod-web-01' }, 'web'],
      ['name', { name: 'prod-web-01' }, 'WEB'],
      ['displayName', { displayName: 'Tower Server' }, 'tower'],
      ['parentName', { parentName: 'cluster-alpha' }, 'alpha'],
      ['agent.hostname', { agent: { hostname: 'node-a.local' } }, 'node-a'],
      ['identity.hostname', { identity: { hostname: 'box.internal' } }, 'box'],
    ])('matches via %s', (_label, overrides, term) => {
      const resource = makeResource(overrides as Partial<Resource>);
      expect(resourceMatchesSearch(resource, term)).toBe(true);
    });

    it('returns false when no candidate contains the term', () => {
      expect(resourceMatchesSearch(makeResource({ displayName: 'isolated' }), 'web')).toBe(false);
    });

    it('matches a partial substring, not just whole values', () => {
      expect(resourceMatchesSearch(makeResource({ displayName: 'hyper-tower' }), 'tower')).toBe(
        true,
      );
    });
  });

  describe('canonicalIdentity fields', () => {
    it.each([
      ['displayName', { canonicalIdentity: { displayName: 'Canonical Box' } }, 'canonical'],
      ['hostname', { canonicalIdentity: { hostname: 'canon.local' } }, 'canon'],
      ['primaryId', { canonicalIdentity: { primaryId: 'node:instance-1' } }, 'instance-1'],
    ])('matches canonicalIdentity.%s', (_label, overrides, term) => {
      expect(resourceMatchesSearch(makeResource(overrides as Partial<Resource>), term)).toBe(true);
    });

    it('matches any alias in canonicalIdentity.aliases', () => {
      const resource = makeResource({
        canonicalIdentity: { aliases: ['first', 'second-alias', 'third'] },
      });
      expect(resourceMatchesSearch(resource, 'second-alias')).toBe(true);
      expect(resourceMatchesSearch(resource, 'THIRD')).toBe(true);
      expect(resourceMatchesSearch(resource, 'nope')).toBe(false);
    });
  });

  describe('docker fields', () => {
    it.each([
      ['hostname', { docker: { hostname: 'docker-host' } }, 'docker-host'],
      ['image', { docker: { image: 'nginx:latest' } }, 'nginx'],
      ['imageId', { docker: { imageId: 'sha256:abc123' } }, 'abc123'],
      ['volumeName', { docker: { volumeName: 'data-vol' } }, 'data-vol'],
      ['networkId', { docker: { networkId: 'net-99' } }, 'net-99'],
      ['driver', { docker: { driver: 'overlay2' } }, 'overlay'],
      ['serviceName', { docker: { serviceName: 'swarm-svc' } }, 'swarm'],
      ['taskId', { docker: { taskId: 'task-7' } }, 'task-7'],
    ])('matches docker.%s', (_label, overrides, term) => {
      expect(resourceMatchesSearch(makeResource(overrides as Partial<Resource>), term)).toBe(true);
    });

    it('matches any entry in docker.repoTags', () => {
      const resource = makeResource({ docker: { repoTags: ['redis:7', 'postgres:16'] } });
      expect(resourceMatchesSearch(resource, 'postgres')).toBe(true);
      expect(resourceMatchesSearch(resource, 'redis:7')).toBe(true);
    });

    it('matches any entry in docker.repoDigests', () => {
      const resource = makeResource({
        docker: { repoDigests: ['nginx@sha256:aaa', 'redis@sha256:bbb'] },
      });
      expect(resourceMatchesSearch(resource, 'sha256:bbb')).toBe(true);
    });

    it('handles empty repoTags / repoDigests arrays without matching', () => {
      const resource = makeResource({ docker: { repoTags: [], repoDigests: [] } });
      expect(resourceMatchesSearch(resource, 'anything')).toBe(false);
    });
  });

  describe('kubernetes fields', () => {
    const k8sBase = {
      clusterName: 'prod-cluster',
      nodeName: 'k8s-node-1',
      namespace: 'kube-system',
      resourceKind: 'Deployment',
      serviceType: 'ClusterIP',
      serviceName: 'api-service',
      clusterIp: '10.0.0.5',
      storageClass: 'fast-ssd',
      provisioner: 'pd.csi.storage',
      volumeBindingMode: 'WaitForFirstConsumer',
      addressType: 'Endpointslice',
      phase: 'Bound',
      secretType: 'Opaque',
      minAvailable: '2',
      maxUnavailable: '25%',
      targetKind: 'Deployment',
      targetName: 'api-target',
      reason: 'BackOff',
      involvedName: 'involved-pod',
      volumeName: 'pvc-1234',
    };

    it.each([
      ['clusterName', 'prod'],
      ['nodeName', 'node-1'],
      ['namespace', 'kube'],
      ['resourceKind', 'deploy'],
      ['serviceType', 'clusterip'],
      ['serviceName', 'api'],
      ['clusterIp', '10.0.0.5'],
      ['storageClass', 'ssd'],
      ['provisioner', 'csi'],
      ['volumeBindingMode', 'first'],
      ['addressType', 'slice'],
      ['phase', 'bound'],
      ['secretType', 'opaque'],
      ['minAvailable', '2'],
      ['maxUnavailable', '25%'],
      ['targetKind', 'deploy'],
      ['targetName', 'target'],
      ['reason', 'backoff'],
      ['involvedName', 'involved'],
      ['volumeName', 'pvc-1234'],
    ])('matches kubernetes.%s', (field, term) => {
      const resource = makeResource({
        kubernetes: { [field]: (k8sBase as Record<string, unknown>)[field] } as never,
      });
      expect(resourceMatchesSearch(resource, term)).toBe(true);
    });

    it.each([
      ['externalIps', ['203.0.113.5', '198.51.100.2'], '198.51.100.2'],
      ['hosts', ['host-a.example', 'host-b.example'], 'host-b'],
      ['addresses', ['1.1.1.1', '2.2.2.2'], '2.2.2.2'],
      ['policyTypes', ['Ingress', 'Egress'], 'egress'],
      ['parameterKeys', ['fsType', 'mode'], 'fstype'],
      ['dataKeys', ['config.json', 'data'], 'config'],
      ['binaryDataKeys', ['bin1', 'bin2'], 'bin2'],
      ['imagePullSecrets', ['regcred', 'ghcr'], 'ghcr'],
      ['limitTypes', ['Container', 'Pod'], 'container'],
      ['metricTypes', ['Resource', 'External'], 'resource'],
      ['accessModes', ['ReadWriteOnce', 'ReadOnlyMany'], 'readonly'],
    ])('matches any entry in kubernetes.%s', (field, values, term) => {
      const resource = makeResource({ kubernetes: { [field]: values } as never });
      expect(resourceMatchesSearch(resource, term)).toBe(true);
    });

    it('matches object keys from kubernetes.hard', () => {
      const resource = makeResource({ kubernetes: { hard: { cpu: '4', memory: '8Gi' } } });
      expect(resourceMatchesSearch(resource, 'memory')).toBe(true);
      expect(resourceMatchesSearch(resource, 'cpu')).toBe(true);
    });

    it('matches object keys from kubernetes.used', () => {
      const resource = makeResource({ kubernetes: { used: { pods: '10' } } });
      expect(resourceMatchesSearch(resource, 'pods')).toBe(true);
    });

    it('handles empty arrays and empty maps without matching', () => {
      const resource = makeResource({
        kubernetes: {
          externalIps: [],
          hosts: [],
          addresses: [],
          hard: {},
          used: {},
        } as never,
      });
      expect(resourceMatchesSearch(resource, 'whatever')).toBe(false);
    });
  });

  describe('platform metadata fields', () => {
    it.each([
      ['vmware.runtimeHostName', { vmware: { runtimeHostName: 'esxi-01' } }, 'esxi'],
      ['vmware.clusterName', { vmware: { clusterName: 'vc-cluster' } }, 'vc-cluster'],
      ['vmware.datacenterName', { vmware: { datacenterName: 'DC-East' } }, 'east'],
      ['proxmox.node', { proxmox: { node: 'pve1' } }, 'pve1'],
      ['proxmox.nodeName', { proxmox: { nodeName: 'node-a' } }, 'node-a'],
      ['proxmox.clusterName', { proxmox: { clusterName: 'pmx-cluster' } }, 'pmx'],
    ])('matches %s', (_label, overrides, term) => {
      expect(resourceMatchesSearch(makeResource(overrides as Partial<Resource>), term)).toBe(true);
    });
  });

  describe('tags', () => {
    it('matches any tag', () => {
      const resource = makeResource({ tags: ['env:prod', 'tier:frontend'] });
      expect(resourceMatchesSearch(resource, 'frontend')).toBe(true);
      expect(resourceMatchesSearch(resource, 'env:prod')).toBe(true);
    });

    it('does not match when tags are absent', () => {
      expect(resourceMatchesSearch(makeResource(), 'env:prod')).toBe(false);
    });
  });

  describe('whitespace handling and filtering', () => {
    it('trims the search term before matching', () => {
      expect(resourceMatchesSearch(makeResource({ displayName: 'tower' }), '   tower   ')).toBe(
        true,
      );
    });

    it('trims candidate values before matching', () => {
      expect(
        resourceMatchesSearch(makeResource({ displayName: '   padded-host   ' }), 'padded-host'),
      ).toBe(true);
    });

    it('ignores candidates that are empty after trimming', () => {
      const resource = makeResource({ id: '   ', name: '', displayName: '   ' });
      expect(resourceMatchesSearch(resource, '')).toBe(true);
      expect(resourceMatchesSearch(resource, 'anything')).toBe(false);
    });

    it('does not match a term against whitespace-only candidates', () => {
      const resource = makeResource({ displayName: '     ' });
      expect(resourceMatchesSearch(resource, 'x')).toBe(false);
    });
  });

  describe('resource with no populated metadata', () => {
    it('matches empty term but not real terms', () => {
      const resource = makeResource({ id: '', name: '', displayName: '' });
      expect(resourceMatchesSearch(resource, '')).toBe(true);
      expect(resourceMatchesSearch(resource, 'nonsense')).toBe(false);
    });
  });
});
