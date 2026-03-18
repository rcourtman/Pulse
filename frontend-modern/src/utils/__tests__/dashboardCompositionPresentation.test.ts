import { describe, expect, it } from 'vitest';
import {
  DASHBOARD_COMPOSITION_EMPTY_STATE,
  getDashboardCompositionIcon,
  getDashboardCompositionLabel,
} from '@/utils/dashboardCompositionPresentation';

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

  it('returns canonical composition icons', () => {
    expect(getDashboardCompositionIcon('vm')).toBeTruthy();
    expect(getDashboardCompositionIcon('system-container')).toBeTruthy();
    expect(getDashboardCompositionIcon('database')).toBeTruthy();
    expect(getDashboardCompositionIcon('')).toBeTruthy();
  });

  it('exports canonical dashboard composition empty-state copy', () => {
    expect(DASHBOARD_COMPOSITION_EMPTY_STATE).toBe('No resources detected');
  });
});
