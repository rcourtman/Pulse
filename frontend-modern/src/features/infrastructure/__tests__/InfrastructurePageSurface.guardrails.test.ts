import { describe, expect, it } from 'vitest';
import infrastructurePageSurfaceSource from '@/features/infrastructure/InfrastructurePageSurface.tsx?raw';
import infrastructurePageModelSource from '@/features/infrastructure/infrastructurePageModel.ts?raw';
import infrastructurePageStateSource from '@/features/infrastructure/useInfrastructurePageState.ts?raw';
import infrastructurePageRouteStateSource from '@/features/infrastructure/useInfrastructurePageRouteState.ts?raw';
import unifiedResourceTableStateSource from '@/components/Infrastructure/useUnifiedResourceTableState.ts?raw';
import unifiedResourceTableViewportSyncSource from '@/components/Infrastructure/useUnifiedResourceTableViewportSync.ts?raw';

describe('InfrastructurePageSurface guardrails', () => {
  it('keeps the feature shell separate from route-sync and page-model ownership', () => {
    expect(infrastructurePageSurfaceSource).toContain('useInfrastructurePageState');
    expect(infrastructurePageSurfaceSource).toContain('useNavigate');
    expect(infrastructurePageSurfaceSource).not.toContain('useLocation(');
    expect(infrastructurePageSurfaceSource).not.toContain('buildInfrastructurePath(');

    expect(infrastructurePageStateSource).toContain('useInfrastructurePageRouteState');
    expect(infrastructurePageStateSource).toContain('buildInfrastructurePageFilterDerivation');
    expect(infrastructurePageStateSource).not.toContain('useLocation(');
    expect(infrastructurePageStateSource).not.toContain('useNavigate(');
    expect(infrastructurePageStateSource).not.toContain('parseInfrastructureLinkSearch(');
    expect(infrastructurePageStateSource).not.toContain('buildInfrastructurePath(');
    expect(infrastructurePageStateSource).not.toContain('areSearchParamsEquivalent(');
    expect(infrastructurePageStateSource).not.toContain('collectAvailableSources(');
    expect(infrastructurePageStateSource).not.toContain('collectAvailableStatuses(');
    expect(infrastructurePageStateSource).not.toContain('buildStatusOptions(');
    expect(infrastructurePageStateSource).not.toContain('tokenizeSearch(');
    expect(infrastructurePageStateSource).not.toContain('filterResources(');

    expect(infrastructurePageRouteStateSource).toContain('useLocation');
    expect(infrastructurePageRouteStateSource).toContain('useNavigate');
    expect(infrastructurePageRouteStateSource).toContain('parseInfrastructureLinkSearch');
    expect(infrastructurePageRouteStateSource).toContain('buildInfrastructurePath');
    expect(infrastructurePageRouteStateSource).toContain('areSearchParamsEquivalent');

    expect(infrastructurePageModelSource).toContain('export function buildInfrastructurePageFilterDerivation');
    expect(infrastructurePageModelSource).toContain('collectAvailableSources');
    expect(infrastructurePageModelSource).toContain('collectAvailableStatuses');
    expect(infrastructurePageModelSource).toContain('buildStatusOptions');
    expect(infrastructurePageModelSource).toContain('tokenizeSearch');
    expect(infrastructurePageModelSource).toContain('filterResources');
  });

  it('keeps summary-to-table coordination on the page-state owner', () => {
    expect(infrastructurePageSurfaceSource).toContain('showJumpToActiveRow={shouldShowJumpToActiveResourceRow()}');
    expect(infrastructurePageSurfaceSource).toContain('onJumpToActiveRow={jumpToActiveResourceRow}');
    expect(infrastructurePageSurfaceSource).toContain('hoveredGroupScope={hoveredSummaryResourceGroupScope()}');
    expect(infrastructurePageSurfaceSource).toContain('focusedGroupScope={focusedSummaryResourceGroupScope()}');
    expect(infrastructurePageSurfaceSource).toContain('activeSummaryGroupScope={activeSummaryResourceGroupScope()}');
    expect(infrastructurePageSurfaceSource).toContain('onGroupHoverChange={setHoveredResourceGroupScope}');
    expect(infrastructurePageSurfaceSource).toContain('setTableRootRef={setSummaryTableRootRef}');
    expect(infrastructurePageSurfaceSource).not.toContain('useSummaryPageInteractionState');

    expect(infrastructurePageStateSource).toContain('useSummaryPageInteractionState');
    expect(infrastructurePageStateSource).toContain('hoveredSummaryResourceGroupScope');
    expect(infrastructurePageStateSource).toContain('activeSummaryResourceGroupScope');
    expect(infrastructurePageStateSource).toContain('focusedSummaryResourceGroupScope');
    expect(infrastructurePageStateSource).toContain('setHoveredResourceGroupScope');
    expect(infrastructurePageStateSource).toContain('jumpToActiveResourceRow');
    expect(infrastructurePageStateSource).toContain('setSummaryTableRootRef');
    expect(infrastructurePageStateSource).toContain('shouldShowJumpToActiveResourceRow');
    expect(infrastructurePageStateSource).not.toContain('querySelector<HTMLElement>(');
    expect(infrastructurePageStateSource).not.toContain('scrollIntoView({ behavior: \'smooth\', block: \'center\' })');

    expect(infrastructurePageRouteStateSource).not.toContain('useSummaryPageInteractionState');
    expect(infrastructurePageRouteStateSource).not.toContain('setSummaryTableRootRef');
  });

  it('keeps inline-detail reveal out of infrastructure viewport sync helpers', () => {
    expect(unifiedResourceTableStateSource).toContain('useUnifiedResourceTableViewportSync');
    expect(unifiedResourceTableStateSource).not.toContain('scrollIntoView');
    expect(unifiedResourceTableViewportSyncSource).toContain('syncHostWindowToViewport');
    expect(unifiedResourceTableViewportSyncSource).toContain("window.addEventListener('scroll'");
    expect(unifiedResourceTableViewportSyncSource).not.toContain('expandedResourceId');
    expect(unifiedResourceTableViewportSyncSource).not.toContain('scrollIntoView');
    expect(unifiedResourceTableViewportSyncSource).not.toContain('querySelector<HTMLElement>(');
  });
});
