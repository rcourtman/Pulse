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
  });

  it('normalizes manual input consistently', () => {
    expect(normalizeCanonicalResourceTypeInput('  VM  ')).toBe('vm');
    expect(normalizeCanonicalResourceTypeInput(' Docker-Host ')).toBe('docker-host');
  });

  it('validates only canonical v6 resource types', () => {
    expect(isCanonicalResourceType('vm')).toBe(true);
    expect(isCanonicalResourceType('physical_disk')).toBe(true);
    expect(isCanonicalResourceType('host')).toBe(false);
    expect(isCanonicalResourceType('lxc')).toBe(false);
  });

  it('keeps the shared invalid-type message aligned with the canonical list', () => {
    expect(INVALID_RESOURCE_TYPE_ERROR).toContain('agent');
    expect(INVALID_RESOURCE_TYPE_ERROR).toContain('physical_disk');
  });
});
