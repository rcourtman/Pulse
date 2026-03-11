import { describe, expect, it } from 'vitest';
import {
  getResourcePickerEmptyState,
  getResourcePickerTypeFilterLabel,
  matchesReportableResourceTypeFilter,
  normalizeReportableResourceType,
  REPORTABLE_RESOURCE_TYPES,
  RESOURCE_PICKER_TYPE_FILTERS,
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

  it('exports canonical picker filter labels and order', () => {
    expect(RESOURCE_PICKER_TYPE_FILTERS).toEqual([
      'all',
      'infrastructure',
      'workloads',
      'storage',
      'recovery',
    ]);
    expect(getResourcePickerTypeFilterLabel('all')).toBe('All');
    expect(getResourcePickerTypeFilterLabel('infrastructure')).toBe('Infrastructure');
    expect(getResourcePickerTypeFilterLabel('workloads')).toBe('Workloads');
    expect(getResourcePickerTypeFilterLabel('storage')).toBe('Storage');
    expect(getResourcePickerTypeFilterLabel('recovery')).toBe('Recovery');
  });

  it('exports canonical resource picker empty states', () => {
    expect(getResourcePickerEmptyState(false)).toEqual({
      title: 'No resources available',
      description: 'Resources appear as Pulse collects infrastructure and workload metrics',
    });
    expect(getResourcePickerEmptyState(true)).toEqual({
      title: 'No resources match your filters',
    });
  });
});
