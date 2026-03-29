import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildWorkloadsHref } from '@/components/Infrastructure/workloadsLink';

const makeResource = (overrides: Partial<Resource>): Resource => ({
  id: 'test-1',
  type: 'agent',
  name: 'test-resource',
  displayName: 'Test Resource',
  platformId: 'plat-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  ...overrides,
});

describe('buildWorkloadsHref', () => {
  // ── Kubernetes cluster ──────────────────────────────────────────

  describe('k8s-cluster resources', () => {
    it('uses kubernetes.clusterName from platformData as context', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: 'prod-cluster' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=prod-cluster',
      );
    });

    it('falls back to kubernetes.context when clusterName is missing', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { context: 'my-context' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=my-context',
      );
    });

    it('falls back to kubernetes.clusterId when clusterName and context are missing', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterId: 'cid-123' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=cid-123',
      );
    });

    it('falls back to resource.clusterId when platformData has no useful fields', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        clusterId: 'resource-cluster',
        platformData: { kubernetes: {} },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=resource-cluster',
      );
    });

    it('falls back to resource.displayName then resource.name for k8s-cluster', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: 'fallback-name',
        displayName: 'Fallback Display',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=Fallback+Display',
      );
    });

    it('uses the local infrastructure label for redacted k8s-cluster fallback', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: 'secret-cluster',
        displayName: 'secret-cluster',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
        aiSafeSummary: 'Governed Cluster',
      });

      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=secret-cluster',
      );
    });

    it('falls back to resource.name when displayName is empty for k8s-cluster', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: 'last-resort',
        displayName: '',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=last-resort',
      );
    });

    it('returns path without context when all k8s-cluster fields are empty', () => {
      const resource = makeResource({
        id: '',
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: '',
        displayName: '',
        clusterId: undefined,
        platformId: '',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=pod&platform=kubernetes');
    });

    it('skips whitespace-only values in k8s-cluster resolution', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: '   ', context: 'valid-context' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=valid-context',
      );
    });
  });

  // ── Kubernetes node ─────────────────────────────────────────────

  describe('k8s-node resources', () => {
    it('uses kubernetes.clusterName from platformData as context', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: 'node-cluster' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=node-cluster',
      );
    });

    it('falls back to kubernetes.context for k8s-node', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        platformData: { kubernetes: { context: 'node-ctx' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=node-ctx',
      );
    });

    it('falls back to kubernetes.clusterId for k8s-node', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterId: 'k-cid' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=k-cid',
      );
    });

    it('falls back to resource.clusterId for k8s-node', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        clusterId: 'res-cluster-id',
        platformData: { kubernetes: {} },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=res-cluster-id',
      );
    });

    it('does NOT fall back to displayName/name for k8s-node (unlike k8s-cluster)', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        name: 'node-name',
        displayName: 'Node Display',
        clusterId: undefined,
      });
      // k8s-node has no displayName/name fallback — context should be undefined
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=pod&platform=kubernetes');
    });
  });

  // ── Docker host ─────────────────────────────────────────────────

  describe('docker-host resources', () => {
    it('uses docker.hostname from platformData as host', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        platformData: { docker: { hostname: 'docker-box' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=docker-box',
      );
    });

    it('falls back to agent.hostname when docker.hostname is missing', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        platformData: { agent: { hostname: 'agent-host' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=agent-host',
      );
    });

    it('falls back to identity.hostname', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        identity: { hostname: 'identity-host' },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=identity-host',
      );
    });

    it('falls back through name → displayName → platformId → id', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        name: 'docker-name',
        displayName: 'Docker Display',
        platformId: 'docker-plat',
        id: 'docker-id',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=docker-name',
      );
    });

    it('falls back to platformId when name is empty for docker-host', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        name: '',
        displayName: 'Docker Display',
        platformId: 'plat-id',
        id: 'docker-id',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=plat-id',
      );
    });

    it('falls back to platformId when name and displayName are empty for docker-host', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        name: '',
        displayName: '',
        platformId: 'docker-plat-id',
        id: 'docker-id',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=docker-plat-id',
      );
    });

    it('falls back to resource.id as last resort for docker-host', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        name: '',
        displayName: '',
        platformId: '',
        id: 'last-resort-id',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=last-resort-id',
      );
    });
  });

  describe('node resources', () => {
    it('resolves Proxmox node using proxmox.nodeName', () => {
      const resource = makeResource({
        type: 'agent',
        platformData: { proxmox: { nodeName: 'pve-node-3' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?agent=pve-node-3');
    });

    it('routes dual-mode hosts to container workloads when docker facets are present', () => {
      const resource = makeResource({
        type: 'agent',
        platformData: {
          agent: { hostname: 'tower' },
          docker: { hostname: 'tower' },
        },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=tower',
      );
    });

    it('prefers canonical docker runtime ids for hybrid host workload routes', () => {
      const resource = makeResource({
        type: 'agent',
        metricsTarget: { resourceType: 'docker-host', resourceId: 'docker-host-1' },
        platformData: {
          agent: { hostname: 'tower.local' },
          docker: { hostname: 'tower.local' },
        },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=docker-host-1',
      );
    });

    it('falls back through the expected chain for node resources', () => {
      const resource = makeResource({
        type: 'agent',
        name: 'node-name',
        displayName: '',
        platformId: '',
        id: 'node-id',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?agent=node-name');
    });

    it('routes truenas systems to platform-scoped app workloads', () => {
      const resource = makeResource({
        type: 'agent',
        platformType: 'truenas',
        name: 'truenas-main',
      });

      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=truenas&agent=truenas-main',
      );
    });
  });

  // ── Unsupported types ───────────────────────────────────────────

  describe('unsupported resource types', () => {
    it.each([
      'vm',
      'container',
      'system-container',
      'app-container',
      'docker-container',
      'oci-container',
      'pod',
      'jail',
      'docker-service',
      'k8s-deployment',
      'k8s-service',
      'host',
      'pbs',
      'pmg',
      'storage',
      'datastore',
      'pool',
      'dataset',
      'physical_disk',
      'ceph',
      'truenas',
    ] as const)('returns null for %s', (type) => {
      const resource = makeResource({ type } as Partial<Resource>);
      expect(buildWorkloadsHref(resource)).toBeNull();
    });
  });

  // ── Edge cases ──────────────────────────────────────────────────

  describe('edge cases', () => {
    it('handles missing platformData gracefully', () => {
      const resource = makeResource({
        type: 'agent',
        platformData: undefined,
        platformId: '',
        name: '',
        displayName: '',
        id: 'bare-id',
      });
      // With no platformData and empty name/platformId, falls to resource.id
      expect(buildWorkloadsHref(resource)).toBe('/workloads?agent=bare-id');
    });

    it('trims whitespace from resolved values', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        platformData: { docker: { hostname: '  trimmed-host  ' } },
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=app-container&platform=docker&agent=trimmed-host',
      );
    });

    it('skips null values in the resolution chain', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: null as unknown as string } },
        clusterId: 'real-value',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=real-value',
      );
    });

    it('skips undefined values in the resolution chain', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: undefined, context: undefined } },
        clusterId: 'cluster-fallback',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=pod&platform=kubernetes&context=cluster-fallback',
      );
    });
  });
});
