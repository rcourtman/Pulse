import { describe, expect, it } from 'vitest';
import {
  buildKubernetesPath,
  buildStorageRouteSearch,
  buildTrueNASPath,
  buildVmwarePath,
  buildWorkloadsRouteSearch,
  KUBERNETES_PATH,
  STORAGE_QUERY_PARAMS,
  TRUENAS_PATH,
  VMWARE_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';

// Branch-coverage companion to resourceLinks.test.ts. Each block below
// targets branches in the named builders that the sibling suite does not
// yet exercise: per-field `if (x) params.set(...)` arms for option keys
// the sibling suite omits, the empty-options/falsy-normalized arms of the
// tab path builders, canonicalization switch arms reached through the
// build* entry points, and the default-parameter arm triggered by an
// explicit `undefined` argument.

describe('buildWorkloadsRouteSearch branch coverage', () => {
  it('returns an empty string when no options are supplied (empty URLSearchParams arm)', () => {
    expect(buildWorkloadsRouteSearch()).toBe('');
    expect(buildWorkloadsRouteSearch({})).toBe('');
  });

  it('emits the runtime, namespace, cluster, and summaryGroup params the sibling suite omits', () => {
    expect(
      buildWorkloadsRouteSearch({
        runtime: 'containerd',
        namespace: 'kube-system',
        cluster: 'prod-us-east',
        summaryGroup: 'compute',
      }),
    ).toBe('?runtime=containerd&namespace=kube-system&cluster=prod-us-east&summaryGroup=compute');
  });

  it('verifies the omitted query-param keys map to the canonical names', () => {
    expect(WORKLOADS_QUERY_PARAMS.runtime).toBe('runtime');
    expect(WORKLOADS_QUERY_PARAMS.namespace).toBe('namespace');
    expect(WORKLOADS_QUERY_PARAMS.cluster).toBe('cluster');
    expect(WORKLOADS_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('canonicalizes the "host" type alias to "agent" via canonicalizeWorkloadFilterType', () => {
    expect(buildWorkloadsRouteSearch({ type: 'host' })).toBe('?type=agent');
  });

  it('passes an unknown type through unchanged (canonicalize returns undefined fallback arm)', () => {
    expect(buildWorkloadsRouteSearch({ type: 'storage' })).toBe('?type=storage');
  });

  it('canonicalizes platform="all" to the "all" sentinel via normalizeSourcePlatformQueryValue', () => {
    expect(buildWorkloadsRouteSearch({ platform: 'all' })).toBe('?platform=all');
  });

  it('trims whitespace around every supported field before serializing', () => {
    expect(
      buildWorkloadsRouteSearch({
        type: '  pod  ',
        platform: '  kubernetes  ',
        runtime: '  cri-o  ',
        context: '  ctx-1  ',
        namespace: '  ns-1  ',
        cluster: '  cl-1  ',
        agent: '  ag-1  ',
        resource: '  res-1  ',
        summaryGroup: '  sg-1  ',
      }),
    ).toBe(
      '?type=pod&platform=kubernetes&runtime=cri-o&context=ctx-1&namespace=ns-1&cluster=cl-1&agent=ag-1&resource=res-1&summaryGroup=sg-1',
    );
  });

  it('drops fields whose normalized value is empty (each per-field falsy arm)', () => {
    expect(
      buildWorkloadsRouteSearch({
        type: '   ',
        platform: '',
        runtime: null,
        context: undefined,
        namespace: '   ',
        cluster: '',
        agent: null,
        resource: undefined,
        summaryGroup: '   ',
      }),
    ).toBe('');
  });
});

describe('buildKubernetesPath branch coverage', () => {
  it('falls back to the default tab when called with an explicit undefined argument', () => {
    expect(
      buildKubernetesPath(undefined as unknown as Parameters<typeof buildKubernetesPath>[0]),
    ).toBe(`${KUBERNETES_PATH}/overview`);
  });

  it('returns the base path when the tab is empty (falsy normalized arm)', () => {
    expect(buildKubernetesPath('')).toBe(KUBERNETES_PATH);
  });

  it('returns the base path when the tab is whitespace-only (trim collapses to empty)', () => {
    expect(buildKubernetesPath('    ')).toBe(KUBERNETES_PATH);
    expect(buildKubernetesPath('\t\n')).toBe(KUBERNETES_PATH);
  });

  it('strips leading and trailing slashes but preserves inner path segments', () => {
    expect(buildKubernetesPath('/workloads/')).toBe(`${KUBERNETES_PATH}/workloads`);
    expect(buildKubernetesPath('///workloads///')).toBe(`${KUBERNETES_PATH}/workloads`);
    expect(buildKubernetesPath('/config/networking/')).toBe(`${KUBERNETES_PATH}/config/networking`);
  });

  it('trims surrounding whitespace before slash-stripping', () => {
    expect(buildKubernetesPath('  workloads  ')).toBe(`${KUBERNETES_PATH}/workloads`);
    expect(buildKubernetesPath('  /workloads/  ')).toBe(`${KUBERNETES_PATH}/workloads`);
  });
});

describe('buildTrueNASPath branch coverage', () => {
  it('falls back to the default tab when called with an explicit undefined argument', () => {
    expect(buildTrueNASPath(undefined as unknown as Parameters<typeof buildTrueNASPath>[0])).toBe(
      `${TRUENAS_PATH}/overview`,
    );
  });

  it('returns the base path when the tab is empty or whitespace-only', () => {
    expect(buildTrueNASPath('')).toBe(TRUENAS_PATH);
    expect(buildTrueNASPath('   ')).toBe(TRUENAS_PATH);
    expect(buildTrueNASPath('\t')).toBe(TRUENAS_PATH);
  });

  it('strips surrounding slashes and whitespace while preserving inner segments', () => {
    expect(buildTrueNASPath('/storage/')).toBe(`${TRUENAS_PATH}/storage`);
    expect(buildTrueNASPath('///datasets///')).toBe(`${TRUENAS_PATH}/datasets`);
    expect(buildTrueNASPath('  /sharing/smb/  ')).toBe(`${TRUENAS_PATH}/sharing/smb`);
  });
});

describe('buildVmwarePath branch coverage', () => {
  it('falls back to the default tab when called with an explicit undefined argument', () => {
    expect(buildVmwarePath(undefined as unknown as Parameters<typeof buildVmwarePath>[0])).toBe(
      `${VMWARE_PATH}/overview`,
    );
  });

  it('returns the base path when the tab is empty or whitespace-only', () => {
    expect(buildVmwarePath('')).toBe(VMWARE_PATH);
    expect(buildVmwarePath('   ')).toBe(VMWARE_PATH);
  });

  it('strips surrounding slashes and whitespace while preserving inner segments', () => {
    expect(buildVmwarePath('/storage/')).toBe(`${VMWARE_PATH}/storage`);
    expect(buildVmwarePath('///hosts/clusters///')).toBe(`${VMWARE_PATH}/hosts/clusters`);
    expect(buildVmwarePath('  /networking/  ')).toBe(`${VMWARE_PATH}/networking`);
  });
});

describe('buildStorageRouteSearch branch coverage', () => {
  it('returns an empty string when no options are supplied', () => {
    expect(buildStorageRouteSearch()).toBe('');
    expect(buildStorageRouteSearch({})).toBe('');
  });

  it('emits the summaryGroup param the sibling suite omits', () => {
    expect(buildStorageRouteSearch({ summaryGroup: 'capacity' })).toBe(
      `?${STORAGE_QUERY_PARAMS.summaryGroup}=capacity`,
    );
    expect(STORAGE_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('canonicalizes raw ceph-family storage types through normalizeStorageSourceKey switch arms', () => {
    expect(buildStorageRouteSearch({ source: 'ceph' })).toBe('?source=ceph');
    expect(buildStorageRouteSearch({ source: 'cephfs' })).toBe('?source=ceph');
    expect(buildStorageRouteSearch({ source: 'rbd' })).toBe('?source=ceph');
  });

  it('maps the "all" storage source sentinel through the switch arm', () => {
    expect(buildStorageRouteSearch({ source: 'all' })).toBe('?source=all');
  });

  it('slugifies storage source input (lowercase, non-alnum collapsed to hyphens)', () => {
    expect(buildStorageRouteSearch({ source: ' Proxmox PBS ' })).toBe('?source=proxmox-pbs');
    expect(buildStorageRouteSearch({ source: 'CephFS!!' })).toBe('?source=ceph');
  });

  it('drops fields whose normalized value is empty (per-field falsy arms)', () => {
    expect(
      buildStorageRouteSearch({
        tab: '   ',
        group: '',
        source: null,
        status: undefined,
        diskRole: '   ',
        diskGroup: '',
        node: null,
        query: undefined,
        resource: '   ',
        sort: '',
        order: null,
        summaryGroup: undefined,
      }),
    ).toBe('');
  });

  it('serializes every supported option in canonical insertion order', () => {
    expect(
      buildStorageRouteSearch({
        tab: 'disks',
        group: 'storage',
        source: 'ceph',
        status: 'available',
        diskRole: 'nvme',
        diskGroup: 'data',
        node: 'pve1',
        query: 'lvm',
        resource: 'r-1',
        sort: 'usage',
        order: 'desc',
        summaryGroup: 'capacity',
      }),
    ).toBe(
      '?tab=disks&group=storage&source=ceph&status=available&diskRole=nvme&diskGroup=data&node=pve1&q=lvm&resource=r-1&sort=usage&order=desc&summaryGroup=capacity',
    );
  });
});
