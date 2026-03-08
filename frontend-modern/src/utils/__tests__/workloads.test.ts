import { describe, expect, it } from 'vitest';
import {
  canonicalizeWorkloadFilterType,
  resolveWorkloadType,
  resolveWorkloadTypeFromString,
  getWorkloadMetricsKind,
  getCanonicalWorkloadId,
} from '@/utils/workloads';
import type { WorkloadGuest } from '@/types/workloads';

describe('resolveWorkloadType', () => {
  it('returns workloadType when present', () => {
    const guest = { workloadType: 'vm' as const, type: 'system-container' };
    expect(resolveWorkloadType(guest)).toBe('vm');
  });

  it('returns vm for VM type (case insensitive)', () => {
    const guest = { type: 'VM' };
    expect(resolveWorkloadType(guest)).toBe('vm');
  });

  it('returns system-container for oci-container type', () => {
    const guest = { type: 'oci-container' };
    expect(resolveWorkloadType(guest)).toBe('system-container');
  });

  it('returns app-container for docker type', () => {
    const guest = { type: 'docker' };
    expect(resolveWorkloadType(guest)).toBe('app-container');
  });

  it('does not map removed docker-container type alias', () => {
    const guest = { type: 'docker-container' };
    expect(resolveWorkloadType(guest)).toBe('system-container');
  });

  it('does not map removed docker_container type alias', () => {
    const guest = { type: 'docker_container' };
    expect(resolveWorkloadType(guest)).toBe('system-container');
  });

  it('returns pod for k8s type', () => {
    const guest = { type: 'k8s' };
    expect(resolveWorkloadType(guest)).toBe('pod');
  });

  it('returns pod for pod type', () => {
    const guest = { type: 'pod' };
    expect(resolveWorkloadType(guest)).toBe('pod');
  });

  it('returns pod for kubernetes type', () => {
    const guest = { type: 'kubernetes' };
    expect(resolveWorkloadType(guest)).toBe('pod');
  });

  it('defaults to system-container for unknown type', () => {
    const guest = { type: 'unknown' };
    expect(resolveWorkloadType(guest)).toBe('system-container');
  });

  it('defaults to system-container for empty type', () => {
    const guest = { type: '' };
    expect(resolveWorkloadType(guest)).toBe('system-container');
  });

  it('defaults to system-container for undefined type', () => {
    const guest = { type: undefined } as unknown as WorkloadGuest;
    expect(resolveWorkloadType(guest)).toBe('system-container');
  });
});

describe('resolveWorkloadTypeFromString', () => {
  it('returns vm for canonical vm type', () => {
    expect(resolveWorkloadTypeFromString('vm')).toBe('vm');
  });

  it('returns system-container for canonical system-container type', () => {
    expect(resolveWorkloadTypeFromString('system-container')).toBe('system-container');
  });

  it('returns system-container for canonical oci-container type', () => {
    expect(resolveWorkloadTypeFromString('oci-container')).toBe('system-container');
  });

  it('normalizes qemu to vm', () => {
    expect(resolveWorkloadTypeFromString('qemu')).toBe('vm');
  });

  it('normalizes lxc to system-container', () => {
    expect(resolveWorkloadTypeFromString('lxc')).toBe('system-container');
  });

  it('does not normalize removed container alias', () => {
    expect(resolveWorkloadTypeFromString('container')).toBeNull();
  });

  it('does not normalize removed docker-container alias', () => {
    expect(resolveWorkloadTypeFromString('docker-container')).toBeNull();
  });

  it('does not normalize removed docker_container alias', () => {
    expect(resolveWorkloadTypeFromString('docker_container')).toBeNull();
  });
});

describe('canonicalizeWorkloadFilterType', () => {
  it('canonicalizes supported workload filter aliases to v6 keys', () => {
    expect(canonicalizeWorkloadFilterType('docker')).toBe('app-container');
    expect(canonicalizeWorkloadFilterType('k8s')).toBe('pod');
    expect(canonicalizeWorkloadFilterType('host')).toBe('agent');
  });

  it('preserves unknown and removed aliases instead of inventing compatibility', () => {
    expect(canonicalizeWorkloadFilterType('docker-container')).toBe('docker-container');
    expect(canonicalizeWorkloadFilterType('custom-type')).toBe('custom-type');
    expect(canonicalizeWorkloadFilterType('')).toBe('');
  });
});

describe('getWorkloadMetricsKind', () => {
  it('returns vm for vm workload', () => {
    const guest = { workloadType: 'vm' as const, type: 'vm' };
    expect(getWorkloadMetricsKind(guest)).toBe('vm');
  });

  it('returns container for system-container workload', () => {
    const guest = { workloadType: 'system-container' as const, type: 'system-container' };
    expect(getWorkloadMetricsKind(guest)).toBe('container');
  });

  it('returns dockerContainer for app-container workload', () => {
    const guest = { workloadType: 'app-container' as const, type: 'docker' };
    expect(getWorkloadMetricsKind(guest)).toBe('dockerContainer');
  });

  it('returns k8s for pod workload', () => {
    const guest = { workloadType: 'pod' as const, type: 'k8s-pod' };
    expect(getWorkloadMetricsKind(guest)).toBe('k8s');
  });

  it('defaults to container for unknown workload type', () => {
    const guest = { workloadType: 'unknown' as unknown as 'vm', type: 'unknown' };
    expect(getWorkloadMetricsKind(guest)).toBe('container');
  });
});

describe('getCanonicalWorkloadId', () => {
  it('returns composite id for vm with instance, node, vmid', () => {
    const guest = { id: 'orig', type: 'vm', instance: 'homelab', node: 'node1', vmid: 100 };
    expect(getCanonicalWorkloadId(guest)).toBe('homelab:node1:100');
  });

  it('returns composite id for system-container with instance, node, vmid', () => {
    const guest = {
      id: 'orig',
      type: 'system-container',
      instance: 'homelab',
      node: 'node2',
      vmid: 200,
    };
    expect(getCanonicalWorkloadId(guest)).toBe('homelab:node2:200');
  });

  it('returns id for app-container workloads (no instance/node/vmid)', () => {
    const guest = {
      id: 'docker-123',
      type: 'docker',
      workloadType: 'app-container' as const,
      instance: '',
      node: '',
      vmid: 0,
    };
    expect(getCanonicalWorkloadId(guest)).toBe('docker-123');
  });

  it('returns id for pod workloads (no instance/node/vmid)', () => {
    const guest = {
      id: 'pod-456',
      type: 'pod',
      workloadType: 'pod' as const,
      instance: '',
      node: '',
      vmid: 0,
    };
    expect(getCanonicalWorkloadId(guest)).toBe('pod-456');
  });

  it('returns id when vmid is 0', () => {
    const guest = { id: 'test', type: 'vm', instance: 'homelab', node: 'node1', vmid: 0 };
    expect(getCanonicalWorkloadId(guest)).toBe('test');
  });

  it('returns id when instance is missing', () => {
    const guest = { id: 'test', type: 'vm', instance: '', node: 'node1', vmid: 100 };
    expect(getCanonicalWorkloadId(guest)).toBe('test');
  });

  it('returns id when node is missing', () => {
    const guest = { id: 'test', type: 'vm', instance: 'homelab', node: '', vmid: 100 };
    expect(getCanonicalWorkloadId(guest)).toBe('test');
  });
});
