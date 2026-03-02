import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildWorkloadsHref } from '@/components/Infrastructure/workloadsLink';

const makeResource = (overrides: Partial<Resource>): Resource => ({
  id: 'test-1',
  type: 'host',
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=prod-cluster');
    });

    it('falls back to kubernetes.context when clusterName is missing', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { context: 'my-context' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=my-context');
    });

    it('falls back to kubernetes.clusterId when clusterName and context are missing', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterId: 'cid-123' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=cid-123');
    });

    it('falls back to resource.clusterId when platformData has no useful fields', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        clusterId: 'resource-cluster',
        platformData: { kubernetes: {} },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=resource-cluster');
    });

    it('falls back to resource.displayName then resource.name for k8s-cluster', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: 'fallback-name',
        displayName: 'Fallback Display',
      });
      expect(buildWorkloadsHref(resource)).toBe(
        '/workloads?type=k8s&context=Fallback+Display',
      );
    });

    it('falls back to resource.name when displayName is empty for k8s-cluster', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: 'last-resort',
        displayName: '',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=last-resort');
    });

    it('returns path without context when all k8s-cluster fields are empty', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        name: '',
        displayName: '',
        clusterId: undefined,
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s');
    });

    it('skips whitespace-only values in k8s-cluster resolution', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: '   ', context: 'valid-context' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=valid-context');
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=node-cluster');
    });

    it('falls back to kubernetes.context for k8s-node', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        platformData: { kubernetes: { context: 'node-ctx' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=node-ctx');
    });

    it('falls back to kubernetes.clusterId for k8s-node', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterId: 'k-cid' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=k-cid');
    });

    it('falls back to resource.clusterId for k8s-node', () => {
      const resource = makeResource({
        type: 'k8s-node',
        platformType: 'kubernetes',
        clusterId: 'res-cluster-id',
        platformData: { kubernetes: {} },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=res-cluster-id');
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s');
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=docker-box');
    });

    it('falls back to agent.hostname when docker.hostname is missing', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        platformData: { agent: { hostname: 'agent-host' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=agent-host');
    });

    it('falls back to identity.hostname', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        identity: { hostname: 'identity-host' },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=identity-host');
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=docker-name');
    });

    it('falls back to displayName when name is empty for docker-host', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        name: '',
        displayName: 'Docker Display',
        platformId: 'plat-id',
        id: 'docker-id',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=Docker+Display');
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=docker-plat-id');
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
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=last-resort-id');
    });
  });

  // ── Host / Node ─────────────────────────────────────────────────

  describe('host resources', () => {
    it('uses proxmox.nodeName from platformData as host', () => {
      const resource = makeResource({
        type: 'host',
        platformData: { proxmox: { nodeName: 'pve1' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=pve1');
    });

    it('falls back to agent.hostname when proxmox.nodeName is missing', () => {
      const resource = makeResource({
        type: 'host',
        platformData: { agent: { hostname: 'agent-hostname' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=agent-hostname');
    });

    it('falls back to identity.hostname', () => {
      const resource = makeResource({
        type: 'host',
        identity: { hostname: 'id-hostname' },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=id-hostname');
    });

    it('falls back to platformId for host type', () => {
      const resource = makeResource({
        type: 'host',
        name: '',
        displayName: '',
        platformId: 'host-plat-id',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=host-plat-id');
    });

    it('prefers platformId over name for host type', () => {
      const resource = makeResource({
        type: 'host',
        platformId: 'plat-wins',
        name: 'name-loses',
        displayName: 'display-loses',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=plat-wins');
    });

    it('prefers name over displayName for host type', () => {
      const resource = makeResource({
        type: 'host',
        platformId: '',
        name: 'name-wins',
        displayName: 'display-loses',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=name-wins');
    });

    it('falls back to displayName then id for host type', () => {
      const resource = makeResource({
        type: 'host',
        platformId: '',
        name: '',
        displayName: 'display-wins',
        id: 'id-loses',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=display-wins');
    });
  });

  describe('node resources', () => {
    it('resolves Proxmox node using proxmox.nodeName', () => {
      const resource = makeResource({
        type: 'node',
        platformData: { proxmox: { nodeName: 'pve-node-3' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=pve-node-3');
    });

    it('falls back through the same chain as host type', () => {
      const resource = makeResource({
        type: 'node',
        name: 'node-name',
        displayName: '',
        platformId: '',
        id: 'node-id',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=node-name');
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
      const resource = makeResource({ type });
      expect(buildWorkloadsHref(resource)).toBeNull();
    });
  });

  // ── Edge cases ──────────────────────────────────────────────────

  describe('edge cases', () => {
    it('handles missing platformData gracefully', () => {
      const resource = makeResource({
        type: 'host',
        platformData: undefined,
        platformId: '',
        name: '',
        displayName: '',
        id: 'bare-id',
      });
      // With no platformData and empty name/platformId, falls to resource.id
      expect(buildWorkloadsHref(resource)).toBe('/workloads?host=bare-id');
    });

    it('trims whitespace from resolved values', () => {
      const resource = makeResource({
        type: 'docker-host',
        platformType: 'docker',
        platformData: { docker: { hostname: '  trimmed-host  ' } },
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=trimmed-host');
    });

    it('skips null values in the resolution chain', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: null as unknown as string } },
        clusterId: 'real-value',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=real-value');
    });

    it('skips undefined values in the resolution chain', () => {
      const resource = makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        platformData: { kubernetes: { clusterName: undefined, context: undefined } },
        clusterId: 'cluster-fallback',
      });
      expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=cluster-fallback');
    });
  });
});
