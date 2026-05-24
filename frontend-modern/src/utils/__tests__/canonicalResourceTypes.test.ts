import { describe, expect, it } from 'vitest';
import {
  CANONICAL_RESOURCE_TYPES,
  INVALID_RESOURCE_TYPE_ERROR,
  isCanonicalResourceType,
  normalizeCanonicalResourceTypeInput,
} from '@/utils/canonicalResourceTypes';

describe('canonicalResourceTypes', () => {
  it('exports the shared canonical resource type list', () => {
    expect(CANONICAL_RESOURCE_TYPES).toContain('agent');
    expect(CANONICAL_RESOURCE_TYPES).toContain('physical_disk');
    expect(CANONICAL_RESOURCE_TYPES).toContain('ceph');
    expect(CANONICAL_RESOURCE_TYPES).toContain('network-endpoint');
    expect(CANONICAL_RESOURCE_TYPES).toContain('docker-image');
    expect(CANONICAL_RESOURCE_TYPES).toContain('docker-volume');
    expect(CANONICAL_RESOURCE_TYPES).toContain('docker-network');
    expect(CANONICAL_RESOURCE_TYPES).toContain('docker-task');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-namespace');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-service');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-statefulset');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-daemonset');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-job');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-cronjob');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-ingress');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-persistent-volume');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-persistent-volume-claim');
    expect(CANONICAL_RESOURCE_TYPES).toContain('k8s-event');
  });

  it('normalizes manual input consistently', () => {
    expect(normalizeCanonicalResourceTypeInput('  VM  ')).toBe('vm');
    expect(normalizeCanonicalResourceTypeInput(' Docker-Host ')).toBe('docker-host');
  });

  it('validates only canonical v6 resource types', () => {
    expect(isCanonicalResourceType('vm')).toBe(true);
    expect(isCanonicalResourceType('physical_disk')).toBe(true);
    expect(isCanonicalResourceType('network-endpoint')).toBe(true);
    expect(isCanonicalResourceType('docker-image')).toBe(true);
    expect(isCanonicalResourceType('k8s-event')).toBe(true);
    expect(isCanonicalResourceType('host')).toBe(false);
    expect(isCanonicalResourceType('lxc')).toBe(false);
  });

  it('keeps the shared invalid-type message aligned with the canonical list', () => {
    expect(INVALID_RESOURCE_TYPE_ERROR).toContain('agent');
    expect(INVALID_RESOURCE_TYPE_ERROR).toContain('physical_disk');
  });
});
