import { describe, expect, it } from 'vitest';
import pageControlsSource from '@/components/shared/PageControls.tsx?raw';
import dashboardFilterSource from '@/components/Dashboard/DashboardFilter.tsx?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';
import infrastructureSource from '@/pages/Infrastructure.tsx?raw';

describe('page controls guardrails', () => {
  it('keeps canonical page-level controls routed through PageControls', () => {
    expect(pageControlsSource).toContain('FilterHeader');
    expect(pageControlsSource).toContain('FilterMobileToggleButton');
    expect(pageControlsSource).toContain('ColumnPicker');

    expect(dashboardFilterSource).toContain('PageControls');
    expect(dashboardFilterSource).not.toContain('<FilterHeader');
    expect(dashboardFilterSource).not.toContain('<ColumnPicker');

    expect(recoverySource).toContain('PageControls');
    expect(recoverySource).not.toContain('<FilterHeader');
    expect(recoverySource).not.toContain('<ColumnPicker');

    expect(infrastructureSource).toContain('PageControls');
    expect(infrastructureSource).not.toContain('<FilterHeader');
  });
});
