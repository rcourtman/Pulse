import { describe, expect, it } from 'vitest';

import type { CountActiveWorkloadsFiltersOptions } from '../workloadsFilterModel';
import {
  countActiveWorkloadsFilters,
  DEFAULT_WORKLOADS_STATUS_MODE,
  DEFAULT_WORKLOADS_VIEW_MODE,
  hasActiveWorkloadsFilters,
} from '../workloadsFilterModel';

// `countActiveWorkloadsFilters` is an 8-accumulator reducer with eight independent
// `if` arms (search / viewMode / statusMode / hostFilter / platformFilter /
// namespaceFilter / clusterFilter / containerRuntimeFilter). Each arm is exercised
// below in both the true (active) and false (default) direction by asserting on
// the real returned count, not on source text.

// Build a fixture where EVERY arm is at its inactive default. The five optional
// filter-value fields are omitted entirely so the `(value ?? '')` nullish arms
// are also driven (they must collapse to the empty string and NOT count).
const inactiveOptions = (): CountActiveWorkloadsFiltersOptions => ({
  search: '',
  viewMode: DEFAULT_WORKLOADS_VIEW_MODE,
  statusMode: DEFAULT_WORKLOADS_STATUS_MODE,
});

describe('countActiveWorkloadsFilters — all-default baseline (every if false)', () => {
  it('returns 0 when every field is at its real default', () => {
    expect(countActiveWorkloadsFilters(inactiveOptions())).toBe(0);
  });

  it('returns 0 when the five optional filter values are explicitly undefined', () => {
    // Drives the `(value ?? '')` nullish-coalesce arm on each of the five
    // filter-value guards — undefined must collapse to '' and not count.
    expect(
      countActiveWorkloadsFilters({
        ...inactiveOptions(),
        hostFilterValue: undefined,
        platformFilterValue: undefined,
        namespaceFilterValue: undefined,
        clusterFilterValue: undefined,
        containerRuntimeFilterValue: undefined,
      }),
    ).toBe(0);
  });

  it('treats a whitespace-only search as inactive (search.trim() !== "" false arm)', () => {
    expect(countActiveWorkloadsFilters({ ...inactiveOptions(), search: '   ' })).toBe(0);
  });
});

describe('countActiveWorkloadsFilters — single-arm active (each if true independently)', () => {
  it('counts only the search arm when search has non-whitespace content', () => {
    expect(countActiveWorkloadsFilters({ ...inactiveOptions(), search: 'nginx' })).toBe(1);
  });

  it('counts only the viewMode arm when viewMode differs from DEFAULT_WORKLOADS_VIEW_MODE', () => {
    expect(countActiveWorkloadsFilters({ ...inactiveOptions(), viewMode: 'vm' })).toBe(1);
  });

  it('counts only the statusMode arm when statusMode differs from DEFAULT_WORKLOADS_STATUS_MODE', () => {
    expect(countActiveWorkloadsFilters({ ...inactiveOptions(), statusMode: 'running' })).toBe(1);
  });

  it('counts only the hostFilter arm when hostFilterValue is non-empty', () => {
    expect(countActiveWorkloadsFilters({ ...inactiveOptions(), hostFilterValue: 'host-1' })).toBe(
      1,
    );
  });

  it('counts only the platformFilter arm when platformFilterValue is non-empty', () => {
    expect(
      countActiveWorkloadsFilters({ ...inactiveOptions(), platformFilterValue: 'vmware' }),
    ).toBe(1);
  });

  it('counts only the namespaceFilter arm when namespaceFilterValue is non-empty', () => {
    expect(
      countActiveWorkloadsFilters({ ...inactiveOptions(), namespaceFilterValue: 'kube-system' }),
    ).toBe(1);
  });

  it('counts only the clusterFilter arm when clusterFilterValue is non-empty', () => {
    expect(
      countActiveWorkloadsFilters({ ...inactiveOptions(), clusterFilterValue: 'prod-us-1' }),
    ).toBe(1);
  });

  it('counts only the containerRuntimeFilter arm when containerRuntimeFilterValue is non-empty', () => {
    expect(
      countActiveWorkloadsFilters({
        ...inactiveOptions(),
        containerRuntimeFilterValue: 'containerd',
      }),
    ).toBe(1);
  });
});

describe('countActiveWorkloadsFilters — all arms active simultaneously', () => {
  it('returns 8 when every arm is set to a non-default value', () => {
    expect(
      countActiveWorkloadsFilters({
        search: 'nginx',
        viewMode: 'vm',
        statusMode: 'running',
        hostFilterValue: 'host-1',
        platformFilterValue: 'vmware',
        namespaceFilterValue: 'kube-system',
        clusterFilterValue: 'prod-us-1',
        containerRuntimeFilterValue: 'containerd',
      }),
    ).toBe(8);
  });
});

describe('hasActiveWorkloadsFilters — wraps countActiveWorkloadsFilters', () => {
  it('returns false when the count is 0 (all-default baseline)', () => {
    expect(hasActiveWorkloadsFilters(inactiveOptions())).toBe(false);
  });

  it('returns true when the count is greater than 0 (single search arm active)', () => {
    expect(hasActiveWorkloadsFilters({ ...inactiveOptions(), search: 'nginx' })).toBe(true);
  });

  it('returns true when every arm is active (count === 8)', () => {
    expect(
      hasActiveWorkloadsFilters({
        search: 'nginx',
        viewMode: 'vm',
        statusMode: 'running',
        hostFilterValue: 'host-1',
        platformFilterValue: 'vmware',
        namespaceFilterValue: 'kube-system',
        clusterFilterValue: 'prod-us-1',
        containerRuntimeFilterValue: 'containerd',
      }),
    ).toBe(true);
  });
});
