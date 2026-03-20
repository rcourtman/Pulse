import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import operationsPageRouteSource from '@/pages/Operations.tsx?raw';
import operationsPageSurfaceSource from '@/features/operations/OperationsPageSurface.tsx?raw';
import operationsPageModelSource from '@/features/operations/operationsPageModel.ts?raw';

describe('operations page route shell', () => {
  it('keeps App routing on a page shell instead of a page-local route controller', () => {
    expect(appSource).toContain("const OperationsPage = lazy(() => import('./pages/Operations'));");
    expect(operationsPageRouteSource).toContain(
      "import { OperationsPageSurface } from '@/features/operations/OperationsPageSurface';",
    );
    expect(operationsPageRouteSource).toContain('<OperationsPageSurface />');
    expect(operationsPageRouteSource).not.toContain('useLocation');
    expect(operationsPageRouteSource).not.toContain('useNavigate');
    expect(operationsPageRouteSource).not.toContain('createSignal');
    expect(operationsPageSurfaceSource).toContain('@/components/shared/Subtabs');
    expect(operationsPageSurfaceSource).toContain('getOperationsTabFromPath');
    expect(operationsPageSurfaceSource).toContain('buildOperationsPath');
    expect(operationsPageSurfaceSource).not.toContain('-webkit-overflow-scrolling');
    expect(operationsPageModelSource).toContain('export const OPERATIONS_TABS');
    expect(operationsPageModelSource).toContain('export function getOperationsTabFromPath');
    expect(operationsPageModelSource).toContain('export function buildOperationsPath');
  });
});
