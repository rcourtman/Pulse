import { describe, expect, it } from 'vitest';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

describe('frontend resource type boundaries', () => {
  it('keeps the shared compatibility adapter narrow and explicit', () => {
    expect(canonicalizeFrontendResourceType('host')).toBe('agent');
    expect(canonicalizeFrontendResourceType('docker')).toBe('app-container');
    expect(canonicalizeFrontendResourceType('docker_host')).toBe('docker-host');
    expect(canonicalizeFrontendResourceType('k8s')).toBe('pod');
    expect(canonicalizeFrontendResourceType('kubernetes_cluster')).toBe('k8s-cluster');

    // Platform-native workload types are not frontend aliases.
    expect(canonicalizeFrontendResourceType('qemu')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('lxc')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('container')).toBeUndefined();
  });
});
