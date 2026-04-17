import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import operationsPageRouteSource from '@/pages/Operations.tsx?raw';
import settingsNavigationModelSource from '@/components/Settings/settingsNavigationModel.ts?raw';

describe('legacy operations route plumbing', () => {
  it('keeps /operations as a redirect-only compatibility page', () => {
    expect(appSource).toContain("const OperationsPage = lazy(() => import('./pages/Operations'));");
    expect(operationsPageRouteSource).toContain(
      "import { Navigate, useLocation } from '@solidjs/router';",
    );
    expect(operationsPageRouteSource).toContain(
      "import { buildLegacyOperationsSettingsPath } from '@/components/Settings/settingsNavigationModel';",
    );
    expect(operationsPageRouteSource).toContain(
      'const canonicalPath = buildLegacyOperationsSettingsPath(location.pathname);',
    );
    expect(operationsPageRouteSource).toContain(
      "return <Navigate href={`${canonicalPath}${location.search ?? ''}`} />;",
    );
    expect(operationsPageRouteSource).not.toContain('OperationsPageSurface');
    expect(settingsNavigationModelSource).toContain(
      'export function buildLegacyOperationsSettingsPath',
    );
  });
});
