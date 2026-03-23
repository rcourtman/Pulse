import { describe, expect, it } from 'vitest';
import infrastructurePageSurfaceSource from '@/features/infrastructure/InfrastructurePageSurface.tsx?raw';
import infrastructurePageStateSource from '@/features/infrastructure/useInfrastructurePageState.ts?raw';
import infrastructurePageRouteStateSource from '@/features/infrastructure/useInfrastructurePageRouteState.ts?raw';

describe('InfrastructurePageSurface guardrails', () => {
  it('keeps the feature shell separate from route-sync ownership', () => {
    expect(infrastructurePageSurfaceSource).toContain('useInfrastructurePageState');
    expect(infrastructurePageSurfaceSource).toContain('useNavigate');
    expect(infrastructurePageSurfaceSource).not.toContain('useLocation(');
    expect(infrastructurePageSurfaceSource).not.toContain('buildInfrastructurePath(');

    expect(infrastructurePageStateSource).toContain('useInfrastructurePageRouteState');
    expect(infrastructurePageStateSource).not.toContain('useLocation(');
    expect(infrastructurePageStateSource).not.toContain('useNavigate(');
    expect(infrastructurePageStateSource).not.toContain('parseInfrastructureLinkSearch(');
    expect(infrastructurePageStateSource).not.toContain('buildInfrastructurePath(');
    expect(infrastructurePageStateSource).not.toContain('areSearchParamsEquivalent(');

    expect(infrastructurePageRouteStateSource).toContain('useLocation');
    expect(infrastructurePageRouteStateSource).toContain('useNavigate');
    expect(infrastructurePageRouteStateSource).toContain('parseInfrastructureLinkSearch');
    expect(infrastructurePageRouteStateSource).toContain('buildInfrastructurePath');
    expect(infrastructurePageRouteStateSource).toContain('areSearchParamsEquivalent');
  });
});
