import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import type { RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import type { RecoveryArtifactMode } from '@/utils/recoveryArtifactModePresentation';

type ArtifactMode = RecoveryArtifactMode;
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

interface UseRecoveryHistorySectionStateParams {
  clusterFilter: Accessor<string>;
  currentPage: Accessor<number>;
  historyOutcomeFilter: Accessor<'all' | RecoveryOutcome>;
  itemTypeFilter: Accessor<string>;
  modeFilter: Accessor<'all' | ArtifactMode>;
  namespaceFilter: Accessor<string>;
  nodeFilter: Accessor<string>;
  platformFilter: Accessor<string>;
  queryFilter: Accessor<string>;
  scopeFilter: Accessor<'all' | 'workload'>;
  verificationFilter: Accessor<VerificationFilter>;
}

export function useRecoveryHistorySectionState(
  params: UseRecoveryHistorySectionStateParams,
) {
  const [selectedPoint, setSelectedPoint] = createSignal<RecoveryPoint | null>(null);
  const [moreFiltersOpen, setMoreFiltersOpen] = createSignal(false);
  const [historyFiltersOpen, setHistoryFiltersOpen] = createSignal(false);
  let advancedFiltersPanelRef: HTMLDivElement | undefined;
  let advancedFiltersButtonRef: HTMLButtonElement | undefined;

  const historyActiveFilterCount = createMemo(() => {
    let count = 0;
    if (params.queryFilter().trim() !== '') count += 1;
    if (params.platformFilter() !== 'all') count += 1;
    if (params.itemTypeFilter() !== 'all') count += 1;
    if (params.historyOutcomeFilter() !== 'all') count += 1;
    if (params.scopeFilter() !== 'all') count += 1;
    if (params.modeFilter() !== 'all') count += 1;
    if (params.verificationFilter() !== 'all') count += 1;
    if (params.clusterFilter() !== 'all') count += 1;
    if (params.nodeFilter() !== 'all') count += 1;
    if (params.namespaceFilter() !== 'all') count += 1;
    return count;
  });

  createEffect(() => {
    params.currentPage();
    params.platformFilter();
    params.itemTypeFilter();
    params.historyOutcomeFilter();
    params.scopeFilter();
    params.modeFilter();
    params.verificationFilter();
    params.clusterFilter();
    params.nodeFilter();
    params.namespaceFilter();
    params.queryFilter();
    setSelectedPoint(null);
  });

  const handleAdvancedFiltersClickOutside = (event: MouseEvent) => {
    const target = event.target as Node;
    if (advancedFiltersPanelRef?.contains(target) || advancedFiltersButtonRef?.contains(target)) {
      return;
    }
    setMoreFiltersOpen(false);
  };

  createEffect(() => {
    if (moreFiltersOpen()) {
      document.addEventListener('mousedown', handleAdvancedFiltersClickOutside);
    } else {
      document.removeEventListener('mousedown', handleAdvancedFiltersClickOutside);
    }
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleAdvancedFiltersClickOutside);
  });

  return {
    advancedFiltersButtonRef: (element: HTMLButtonElement) => {
      advancedFiltersButtonRef = element;
    },
    advancedFiltersPanelRef: (element: HTMLDivElement) => {
      advancedFiltersPanelRef = element;
    },
    clearSelectedPoint: () => setSelectedPoint(null),
    historyActiveFilterCount,
    historyFiltersOpen,
    moreFiltersOpen,
    selectedPoint,
    setHistoryFiltersOpen,
    setMoreFiltersOpen,
    toggleSelectedPoint: (point: RecoveryPoint) =>
      setSelectedPoint((current) => (current?.id === point.id ? null : point)),
  };
}
