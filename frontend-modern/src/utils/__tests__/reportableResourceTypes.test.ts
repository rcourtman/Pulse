import { describe, expect, it } from 'vitest';
import {
  matchesReportableResourceTypeFilter,
  normalizeReportableResourceType,
  REPORTABLE_RESOURCE_TYPES,
  reportableResourceTypeSortOrder,
} from '@/utils/reportableResourceTypes';

describe('reportableResourceTypes', () => {
  it('exports the shared reportable resource set', () => {
    expect(REPORTABLE_RESOURCE_TYPES.has('agent')).toBe(true);
    expect(REPORTABLE_RESOURCE_TYPES.has('pbs')).toBe(true);
    expect(REPORTABLE_RESOURCE_TYPES.has('truenas')).toBe(false);
  });

  it('normalizes resource types for picker ordering', () => {
    expect(normalizeReportableResourceType('k8s-cluster')).toBe('k8s');
    expect(normalizeReportableResourceType('oci-container')).toBe('system-container');
    expect(normalizeReportableResourceType('vm')).toBe('vm');
  });

  it('matches shared type filters consistently', () => {
    expect(matchesReportableResourceTypeFilter({ type: 'agent' }, 'infrastructure')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'vm' }, 'workloads')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'storage' }, 'storage')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pbs' }, 'recovery')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pbs' }, 'storage')).toBe(false);
  });

  it('returns stable sort order buckets for reportable types', () => {
    expect(reportableResourceTypeSortOrder('agent')).toBeLessThan(
      reportableResourceTypeSortOrder('vm'),
    );
    expect(reportableResourceTypeSortOrder('storage')).toBeLessThan(
      reportableResourceTypeSortOrder('dataset'),
    );
  });
});
