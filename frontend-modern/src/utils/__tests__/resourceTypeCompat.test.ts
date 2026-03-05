import { describe, expect, it } from 'vitest';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

describe('resourceTypeCompat', () => {
  it('canonicalizes the small frontend compatibility alias set', () => {
    expect(canonicalizeFrontendResourceType('host')).toBe('agent');
    expect(canonicalizeFrontendResourceType('docker')).toBe('app-container');
    expect(canonicalizeFrontendResourceType('docker_host')).toBe('docker-host');
    expect(canonicalizeFrontendResourceType('k8s')).toBe('pod');
    expect(canonicalizeFrontendResourceType('kubernetes')).toBe('pod');
    expect(canonicalizeFrontendResourceType('kubernetes_cluster')).toBe('k8s-cluster');
    expect(canonicalizeFrontendResourceType('kubernetes-node')).toBe('k8s-node');
  });

  it('does not silently canonicalize removed non-canonical workload aliases', () => {
    expect(canonicalizeFrontendResourceType('docker-container')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('docker_service')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('container')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('qemu')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('lxc')).toBeUndefined();
  });
});
