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
    expect(REPORTABLE_RESOURCE_TYPES.has('network-share')).toBe(true);
    expect(REPORTABLE_RESOURCE_TYPES.has('truenas' as any)).toBe(false);
  });

  it('keeps native platform inventory rows out of the metric reporting picker', () => {
    const inventoryOnlyTypes = [
      'docker-image',
      'docker-volume',
      'docker-network',
      'docker-task',
      'docker-swarm-node',
      'docker-secret',
      'docker-config',
      'k8s-namespace',
      'k8s-service',
      'k8s-replicaset',
      'k8s-statefulset',
      'k8s-daemonset',
      'k8s-job',
      'k8s-cronjob',
      'k8s-ingress',
      'k8s-endpoint-slice',
      'k8s-network-policy',
      'k8s-persistent-volume',
      'k8s-persistent-volume-claim',
      'k8s-storage-class',
      'k8s-configmap',
      'k8s-secret',
      'k8s-serviceaccount',
      'k8s-resource-quota',
      'k8s-limit-range',
      'k8s-pod-disruption-budget',
      'k8s-horizontal-pod-autoscaler',
      'k8s-event',
    ] as const;

    for (const type of inventoryOnlyTypes) {
      expect(REPORTABLE_RESOURCE_TYPES.has(type)).toBe(false);
    }
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
    expect(matchesReportableResourceTypeFilter({ type: 'network-share' }, 'storage')).toBe(true);
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
