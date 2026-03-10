import { describe, expect, it } from 'vitest';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
import { TYPE_COLUMN_LABEL } from '@/utils/typeColumnContract';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

describe('typeColumnPresentation', () => {
  it('returns the canonical Type column label', () => {
    expect(getTypeColumnLabel()).toBe(TYPE_COLUMN_LABEL);
  });

  it('keeps fixed runtime Type headers on the shared label utility', () => {
    expect(problemResourcesTableSource).toContain('getTypeColumnLabel()');
    expect(alertsPageSource).toContain('getTypeColumnLabel()');
  });
});
