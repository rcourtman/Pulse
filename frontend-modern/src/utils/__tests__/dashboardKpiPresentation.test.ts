import { describe, expect, it } from 'vitest';
import { getDashboardKpiPresentation } from '@/utils/dashboardKpiPresentation';

describe('dashboardKpiPresentation', () => {
  it('returns canonical KPI presentation for infrastructure/workloads/storage/alerts', () => {
    expect(getDashboardKpiPresentation('infrastructure')).toMatchObject({
      label: 'Infrastructure',
    });
    expect(getDashboardKpiPresentation('workloads')).toMatchObject({
      label: 'Workloads',
    });
    expect(getDashboardKpiPresentation('storage')).toMatchObject({
      label: 'Storage',
    });
    expect(getDashboardKpiPresentation('alerts')).toMatchObject({
      label: 'Alerts',
    });
  });

  it('exposes canonical KPI card and icon classes', () => {
    expect(getDashboardKpiPresentation('infrastructure').cardClassName).toContain(
      'border-l-blue-500',
    );
    expect(getDashboardKpiPresentation('workloads').iconClassName).toContain('violet');
    expect(getDashboardKpiPresentation('storage').iconClassName).toContain('cyan');
    expect(getDashboardKpiPresentation('alerts').cardClassName).toContain('border-l-amber-500');
  });
});
