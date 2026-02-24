import { describe, expect, it } from 'vitest';
import {
  resolveWorkloadType,
  getWorkloadMetricsKind,
  getCanonicalWorkloadId,
} from '@/utils/workloads';
import type { WorkloadGuest } from '@/types/workloads';

describe('resolveWorkloadType', () => {
  it('returns workloadType when present', () => {
    const guest = { workloadType: 'vm' as const, type: 'lxc' };
    expect(resolveWorkloadType(guest)).toBe('vm');
  });

  it('returns vm for qemu type', () => {
    const guest = { type: 'qemu' };
    expect(resolveWorkloadType(guest)).toBe('vm');
  });

  it('returns vm for VM type (case insensitive)', () => {
    const guest = { type: 'VM' };
    expect(resolveWorkloadType(guest)).toBe('vm');
  });

  it('returns lxc for lxc type', () => {
    const guest = { type: 'lxc' };
    expect(resolveWorkloadType(guest)).toBe('lxc');
  });

  it('returns lxc for oci type', () => {
    const guest = { type: 'oci' };
    expect(resolveWorkloadType(guest)).toBe('lxc');
  });

  it('returns lxc for container type', () => {
    const guest = { type: 'container' };
    expect(resolveWorkloadType(guest)).toBe('lxc');
  });

  it('returns docker for docker type', () => {
    const guest = { type: 'docker' };
    expect(resolveWorkloadType(guest)).toBe('docker');
  });

  it('returns docker for docker-container type', () => {
    const guest = { type: 'docker-container' };
    expect(resolveWorkloadType(guest)).toBe('docker');
  });

  it('returns docker for docker_container type', () => {
    const guest = { type: 'docker_container' };
    expect(resolveWorkloadType(guest)).toBe('docker');
  });

  it('returns k8s for k8s type', () => {
    const guest = { type: 'k8s' };
    expect(resolveWorkloadType(guest)).toBe('k8s');
  });

  it('returns k8s for pod type', () => {
    const guest = { type: 'pod' };
    expect(resolveWorkloadType(guest)).toBe('k8s');
  });

  it('returns k8s for kubernetes type', () => {
    const guest = { type: 'kubernetes' };
    expect(resolveWorkloadType(guest)).toBe('k8s');
  });

  it('defaults to lxc for unknown type', () => {
    const guest = { type: 'unknown' };
    expect(resolveWorkloadType(guest)).toBe('lxc');
  });

  it('defaults to lxc for empty type', () => {
    const guest = { type: '' };
    expect(resolveWorkloadType(guest)).toBe('lxc');
  });

  it('defaults to lxc for undefined type', () => {
    const guest = { type: undefined } as unknown as WorkloadGuest;
    expect(resolveWorkloadType(guest)).toBe('lxc');
  });
});

describe('getWorkloadMetricsKind', () => {
  it('returns vm for vm workload', () => {
    const guest = { workloadType: 'vm' as const, type: 'qemu' };
    expect(getWorkloadMetricsKind(guest)).toBe('vm');
  });

  it('returns container for lxc workload', () => {
    const guest = { workloadType: 'lxc' as const, type: 'lxc' };
    expect(getWorkloadMetricsKind(guest)).toBe('container');
  });

  it('returns dockerContainer for docker workload', () => {
    const guest = { workloadType: 'docker' as const, type: 'docker' };
    expect(getWorkloadMetricsKind(guest)).toBe('dockerContainer');
  });

  it('returns k8s for k8s workload', () => {
    const guest = { workloadType: 'k8s' as const, type: 'kubernetes' };
    expect(getWorkloadMetricsKind(guest)).toBe('k8s');
  });

  it('defaults to container for unknown workload type', () => {
    const guest = { workloadType: 'unknown' as unknown as 'vm', type: 'unknown' };
    expect(getWorkloadMetricsKind(guest)).toBe('container');
  });
});

describe('getCanonicalWorkloadId', () => {
  it('returns composite id for vm with instance, node, vmid', () => {
    const guest = { id: 'orig', type: 'qemu', instance: 'qemu', node: 'node1', vmid: 100 };
    expect(getCanonicalWorkloadId(guest)).toBe('qemu:node1:100');
  });

  it('returns composite id for lxc with instance, node, vmid', () => {
    const guest = { id: 'orig', type: 'lxc', instance: 'lxc', node: 'node2', vmid: 200 };
    expect(getCanonicalWorkloadId(guest)).toBe('lxc:node2:200');
  });

  it('returns id for docker (no instance/node/vmid)', () => {
    const guest = {
      id: 'docker-123',
      type: 'docker',
      workloadType: 'docker' as const,
      instance: '',
      node: '',
      vmid: 0,
    };
    expect(getCanonicalWorkloadId(guest)).toBe('docker-123');
  });

  it('returns id for k8s (no instance/node/vmid)', () => {
    const guest = {
      id: 'pod-456',
      type: 'pod',
      workloadType: 'k8s' as const,
      instance: '',
      node: '',
      vmid: 0,
    };
    expect(getCanonicalWorkloadId(guest)).toBe('pod-456');
  });

  it('returns id when vmid is 0', () => {
    const guest = { id: 'test', type: 'qemu', instance: 'qemu', node: 'node1', vmid: 0 };
    expect(getCanonicalWorkloadId(guest)).toBe('test');
  });

  it('returns id when instance is missing', () => {
    const guest = { id: 'test', type: 'qemu', instance: '', node: 'node1', vmid: 100 };
    expect(getCanonicalWorkloadId(guest)).toBe('test');
  });

  it('returns id when node is missing', () => {
    const guest = { id: 'test', type: 'qemu', instance: 'qemu', node: '', vmid: 100 };
    expect(getCanonicalWorkloadId(guest)).toBe('test');
  });
});
