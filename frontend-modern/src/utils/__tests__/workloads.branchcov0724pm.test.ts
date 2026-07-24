import { describe, expect, it } from 'vitest';

import {
  buildCanonicalNodeScopedWorkloadId,
  buildKubernetesWorkloadMetadataId,
  getCanonicalWorkloadIdForResource,
  getWorkloadMetadataId,
  getWorkloadMetadataIdCandidates,
  isDockerManagedAppContainer,
} from '@/utils/workloads';
import type { WorkloadGuest } from '@/types/workloads';

// `type`, `platformType`, and `containerRuntime` are optional on WorkloadGuest
// (`platformType`/`containerRuntime` via `?`, `type` via the VM | Container
// base). isDockerManagedAppContainer reads them defensively with `|| ''`, so
// the malformed-input cases below opt out of the contract intentionally.
type DockerManagedAppContainerInput = Pick<
  WorkloadGuest,
  'workloadType' | 'type' | 'platformType' | 'containerRuntime'
>;

// Covers the type/runtime fallback arms of isDockerManagedAppContainer that the
// happy-path specs never reach (existing specs only exercise the platformType
// 'docker' -> true and platformType 'truenas' -> false early returns).
describe('isDockerManagedAppContainer fallback signals', () => {
  it('returns true from the raw type fallback when type is docker and platform type is unset', () => {
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: 'docker',
        platformType: undefined,
        containerRuntime: undefined,
      } as unknown as DockerManagedAppContainerInput),
    ).toBe(true);
  });

  it('returns true from the container runtime fallback when runtime is docker', () => {
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: 'app-container',
        platformType: '',
        containerRuntime: 'docker',
      }),
    ).toBe(true);
  });

  it('returns true from the container runtime fallback when runtime is podman', () => {
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: 'app-container',
        platformType: '',
        containerRuntime: 'podman',
      }),
    ).toBe(true);
  });

  it('returns false when no docker signal matches across type, platform, or runtime', () => {
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: 'app-container',
        platformType: '',
        containerRuntime: 'containerd',
      }),
    ).toBe(false);
  });

  it('returns false when type is undefined and runtime is unset', () => {
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: undefined,
        platformType: undefined,
        containerRuntime: undefined,
      } as unknown as DockerManagedAppContainerInput),
    ).toBe(false);
  });

  it('is case-insensitive across the type and runtime fallbacks', () => {
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: '  Docker  ',
        platformType: '  Unraid  ',
        containerRuntime: '',
      }),
    ).toBe(true);
    expect(
      isDockerManagedAppContainer({
        workloadType: 'app-container',
        type: 'app-container',
        platformType: '',
        containerRuntime: 'PODMAN',
      }),
    ).toBe(true);
  });
});

// Covers the kind-coercion ternary in buildKubernetesWorkloadMetadataId. The
// happy-path specs only use the default kind ('pod'); the deployment/service
// arms and the falsy-kind guard are uncovered.
describe('buildKubernetesWorkloadMetadataId kind coercion', () => {
  const base = {
    kubernetesClusterId: 'cluster-a',
    namespace: 'payments',
    name: 'checkout',
  };

  it('maps k8s-deployment kind to the deployment segment', () => {
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: 'k8s-deployment' })).toBe(
      'k8s-workload:cluster-a:deployment:payments:checkout',
    );
  });

  it('maps deployment kind to the deployment segment', () => {
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: 'deployment' })).toBe(
      'k8s-workload:cluster-a:deployment:payments:checkout',
    );
  });

  it('maps k8s-service kind to the service segment', () => {
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: 'k8s-service' })).toBe(
      'k8s-workload:cluster-a:service:payments:checkout',
    );
  });

  it('maps service kind to the service segment', () => {
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: 'service' })).toBe(
      'k8s-workload:cluster-a:service:payments:checkout',
    );
  });

  it('is case-insensitive when matching the kind token', () => {
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: 'Deployment' })).toBe(
      'k8s-workload:cluster-a:deployment:payments:checkout',
    );
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: '  K8S-Service  ' })).toBe(
      'k8s-workload:cluster-a:service:payments:checkout',
    );
  });

  it('returns null when kind is empty (no workload kind segment)', () => {
    expect(buildKubernetesWorkloadMetadataId({ ...base, kind: '' })).toBeNull();
  });
});

// Covers the empty-candidates fallback (`candidates[0] || getCanonicalWorkloadId`)
// in getWorkloadMetadataId. The happy-path specs always resolve at least one
// candidate, so the `||` fallback is never exercised.
describe('getWorkloadMetadataId empty-candidates fallback', () => {
  const baseGuest = {
    id: '',
    name: '',
    workloadType: 'app-container' as const,
    type: 'app-container',
    platformType: 'truenas',
    instance: '',
    node: '',
    vmid: 0,
  };

  it('resolves zero metadata id candidates for a non-docker-managed app-container with a blank id', () => {
    expect(getWorkloadMetadataIdCandidates(baseGuest)).toEqual([]);
  });

  it('falls back to the canonical workload id (the blank id) when no candidates resolve', () => {
    expect(getWorkloadMetadataId(baseGuest)).toBe('');
  });
});

// Covers the non-finite vmid arm (`Number.isFinite(vmid) ? Number(vmid) : 0`)
// in buildCanonicalNodeScopedWorkloadId. The happy-path specs only pass numeric
// vmids, so the `: 0` coercion (which then short-circuits to null) is cold.
describe('buildCanonicalNodeScopedWorkloadId non-finite vmid coercion', () => {
  it('coerces a null vmid to 0 and returns null', () => {
    expect(
      buildCanonicalNodeScopedWorkloadId({ instance: 'homelab', node: 'pve1', vmid: null }),
    ).toBeNull();
  });
});

// Covers the proxmox.nodeName fallback in getCanonicalWorkloadIdForResource.
// The happy-path specs populate proxmox.node, so the `node || nodeName` fallback
// to nodeName is never exercised.
describe('getCanonicalWorkloadIdForResource proxmox nodeName fallback', () => {
  it('uses proxmox.nodeName when proxmox.node is absent', () => {
    expect(
      getCanonicalWorkloadIdForResource({
        id: 'vm-fallback',
        type: 'vm',
        clusterId: 'Core Fabric',
        proxmox: { nodeName: 'pve3', vmid: 112 },
      } as any),
    ).toBe('Core Fabric:pve3:112');
  });

  it('prefers proxmox.node over proxmox.nodeName when both are present', () => {
    expect(
      getCanonicalWorkloadIdForResource({
        id: 'lxc-fallback',
        type: 'system-container',
        clusterId: 'Core Fabric',
        proxmox: { node: 'pve1', nodeName: 'pve3', vmid: 200 },
      } as any),
    ).toBe('Core Fabric:pve1:200');
  });

  it('falls back to the resource id when only nodeName is present but it is blank', () => {
    expect(
      getCanonicalWorkloadIdForResource({
        id: 'vm-orphan',
        type: 'vm',
        clusterId: 'Core Fabric',
        proxmox: { nodeName: '   ', vmid: 112 },
      } as any),
    ).toBe('vm-orphan');
  });
});
