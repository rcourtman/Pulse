import { describe, expect, it } from 'vitest';
import type { ResourceType } from '@/types/resource';
import type { ResourcePickerTypeFilter } from '@/utils/reportableResourceTypes';
import {
  matchesReportableResourceTypeFilter,
  normalizeReportableResourceType,
  reportableResourceTypeSortOrder,
} from '@/utils/reportableResourceTypes';

// Branch-coverage companion to reportableResourceTypes.test.ts. The sibling
// suite only exercises a representative subset of each exported function; the
// cases below drive the remaining arms of every conditional / early-return in
// normalizeReportableResourceType and matchesReportableResourceTypeFilter, plus
// the `?? 13` nullish-coalescing fallback in reportableResourceTypeSortOrder.
// Assertions are on concrete outputs, matching the metricThresholds.branchcov2
// convention.

describe('normalizeReportableResourceType — branch coverage (branchcov2)', () => {
  it("returns 'system-container' for both arms of the first guard's ||", () => {
    // Sibling suite only exercises the oci-container arm; cover system-container too.
    expect(normalizeReportableResourceType('system-container')).toBe('system-container');
    expect(normalizeReportableResourceType('oci-container')).toBe('system-container');
  });

  it("normalizes 'app-container' through its dedicated guard", () => {
    expect(normalizeReportableResourceType('app-container')).toBe('app-container');
  });

  it("normalizes 'docker-host' through its dedicated guard", () => {
    expect(normalizeReportableResourceType('docker-host')).toBe('docker-host');
  });

  it("normalizes 'k8s-node' to 'node' (distinct from k8s-cluster's 'k8s' arm)", () => {
    expect(normalizeReportableResourceType('k8s-node')).toBe('node');
    expect(normalizeReportableResourceType('k8s-cluster')).toBe('k8s');
  });

  it('returns the type verbatim for every remaining default-arm type', () => {
    // vm is covered by the sibling suite; add the other reportable types that
    // fall through to `return type`.
    const passthrough: ResourceType[] = [
      'agent',
      'vm',
      'pod',
      'pbs',
      'pmg',
      'storage',
      'datastore',
      'pool',
      'dataset',
      'network-share',
    ];
    for (const type of passthrough) {
      expect(normalizeReportableResourceType(type)).toBe(type);
    }
  });
});

describe('matchesReportableResourceTypeFilter — branch coverage (branchcov2)', () => {
  it("returns true unconditionally for the 'all' filter (first early-return)", () => {
    expect(matchesReportableResourceTypeFilter({ type: 'vm' }, 'all')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pbs' }, 'all')).toBe(true);
  });

  it('infrastructure filter: true for every INFRASTRUCTURE_TYPES member, false otherwise', () => {
    // Sibling suite only asserts the true arm (agent); cover false arm + the rest.
    expect(matchesReportableResourceTypeFilter({ type: 'agent' }, 'infrastructure')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'docker-host' }, 'infrastructure')).toBe(
      true,
    );
    expect(matchesReportableResourceTypeFilter({ type: 'k8s-cluster' }, 'infrastructure')).toBe(
      true,
    );
    expect(matchesReportableResourceTypeFilter({ type: 'k8s-node' }, 'infrastructure')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pbs' }, 'infrastructure')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pmg' }, 'infrastructure')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'vm' }, 'infrastructure')).toBe(false);
  });

  it('workloads filter: true for every WORKLOAD_TYPES member, false otherwise', () => {
    expect(matchesReportableResourceTypeFilter({ type: 'vm' }, 'workloads')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'system-container' }, 'workloads')).toBe(
      true,
    );
    expect(matchesReportableResourceTypeFilter({ type: 'app-container' }, 'workloads')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'oci-container' }, 'workloads')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pod' }, 'workloads')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'agent' }, 'workloads')).toBe(false);
  });

  it('storage filter: true for every STORAGE_TYPES member, false otherwise', () => {
    expect(matchesReportableResourceTypeFilter({ type: 'storage' }, 'storage')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'datastore' }, 'storage')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pool' }, 'storage')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'dataset' }, 'storage')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'network-share' }, 'storage')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'agent' }, 'storage')).toBe(false);
  });

  it('recovery filter: true for both RECOVERY_TYPES (pbs AND datastore), false otherwise', () => {
    // Sibling suite only asserts pbs->true; datastore is the other recovery type.
    expect(matchesReportableResourceTypeFilter({ type: 'pbs' }, 'recovery')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'datastore' }, 'recovery')).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'vm' }, 'recovery')).toBe(false);
    expect(matchesReportableResourceTypeFilter({ type: 'storage' }, 'recovery')).toBe(false);
  });

  it("returns true via the defensive fall-through for an unrecognized filter value", () => {
    // No declared ResourcePickerTypeFilter reaches the final `return true`, so a
    // bogus value is cast through unknown to satisfy strict mode while driving
    // the branch at runtime.
    const unknownFilter = 'everything-else' as unknown as ResourcePickerTypeFilter;
    expect(matchesReportableResourceTypeFilter({ type: 'vm' }, unknownFilter)).toBe(true);
    expect(matchesReportableResourceTypeFilter({ type: 'pbs' }, unknownFilter)).toBe(true);
  });
});

describe('reportableResourceTypeSortOrder — branch coverage (branchcov2)', () => {
  it('returns the exact SORT_ORDER bucket for each normalize-resolved type', () => {
    // Sibling suite only asserts relative ordering (toBeLessThan); assert the
    // concrete numbers so a SORT_ORDER edit is caught.
    expect(reportableResourceTypeSortOrder('agent')).toBe(0);
    expect(reportableResourceTypeSortOrder('docker-host')).toBe(1);
    expect(reportableResourceTypeSortOrder('k8s-cluster')).toBe(2); // normalizes to 'k8s'
    expect(reportableResourceTypeSortOrder('pbs')).toBe(3);
    expect(reportableResourceTypeSortOrder('pmg')).toBe(4);
    expect(reportableResourceTypeSortOrder('vm')).toBe(5);
    expect(reportableResourceTypeSortOrder('system-container')).toBe(6);
    expect(reportableResourceTypeSortOrder('oci-container')).toBe(6); // normalizes to 'system-container'
    expect(reportableResourceTypeSortOrder('app-container')).toBe(7);
    expect(reportableResourceTypeSortOrder('pod')).toBe(8);
    expect(reportableResourceTypeSortOrder('storage')).toBe(9);
    expect(reportableResourceTypeSortOrder('datastore')).toBe(10);
    expect(reportableResourceTypeSortOrder('pool')).toBe(11);
    expect(reportableResourceTypeSortOrder('dataset')).toBe(12);
    expect(reportableResourceTypeSortOrder('network-share')).toBe(13);
  });

  it('uses the `?? 13` fallback for default-arm types absent from SORT_ORDER', () => {
    // Valid ResourceType values that fall through normalize's default arm to
    // their own string, which is not a SORT_ORDER key -> nullish fallback fires.
    expect(reportableResourceTypeSortOrder('jail')).toBe(13);
    expect(reportableResourceTypeSortOrder('ceph')).toBe(13);
    expect(reportableResourceTypeSortOrder('network')).toBe(13);
    expect(reportableResourceTypeSortOrder('network-endpoint')).toBe(13);
    expect(reportableResourceTypeSortOrder('physical_disk')).toBe(13);
  });

  it("uses the `?? 13` fallback for 'k8s-node', which normalizes to the missing 'node' key", () => {
    // k8s-node is a reportable type that normalizes to 'node', but 'node' is
    // not a SORT_ORDER key, so the fallback fires and the node sorts alongside
    // network-share (13) instead of grouping with k8s-cluster (2).
    expect(reportableResourceTypeSortOrder('k8s-node')).toBe(13);
    expect(reportableResourceTypeSortOrder('k8s-cluster')).toBe(2);
  });
});
