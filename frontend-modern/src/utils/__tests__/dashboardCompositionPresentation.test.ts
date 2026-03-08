import { describe, expect, it } from 'vitest';
import { getDashboardCompositionLabel } from '@/utils/dashboardCompositionPresentation';

describe('dashboardCompositionPresentation', () => {
  it('returns composition-specific labels for known dashboard buckets', () => {
    expect(getDashboardCompositionLabel('vm')).toBe('Virtual Machines');
    expect(getDashboardCompositionLabel('system-container')).toBe('System Containers');
    expect(getDashboardCompositionLabel('app-container')).toBe('App Containers');
    expect(getDashboardCompositionLabel('pod')).toBe('Kubernetes Pods');
    expect(getDashboardCompositionLabel('database')).toBe('Databases');
  });

  it('falls back to canonical resource type labels or titleized labels', () => {
    expect(getDashboardCompositionLabel('storage')).toBe('Storage');
    expect(getDashboardCompositionLabel('custom-backend')).toBe('Custom Backend');
    expect(getDashboardCompositionLabel('')).toBe('Unknown');
  });
});
