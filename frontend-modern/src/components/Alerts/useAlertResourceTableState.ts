import { createMemo, createSignal } from 'solid-js';
import {
  flattenAlertResourceTableResources,
  hasAlertResourceTableRows,
  hasCustomAlertResourceGlobalDefaults,
  type AlertResourceTableResourceLike,
  type AlertResourceThresholdMap,
} from './alertResourceTableModel';

interface UseAlertResourceTableStateOptions<T extends AlertResourceTableResourceLike> {
  resources?: T[];
  groupedResources?: Record<string, T[]>;
  globalDefaults?: AlertResourceThresholdMap;
  factoryDefaults?: AlertResourceThresholdMap;
}

export function useAlertResourceTableState<T extends AlertResourceTableResourceLike>(
  options: UseAlertResourceTableStateOptions<T>,
) {
  const [activeMetricInput, setActiveMetricInput] = createSignal<{
    resourceId: string;
    metric: string;
  } | null>(null);
  const [showDelayRow, setShowDelayRow] = createSignal(false);
  const [selectedIds, setSelectedIds] = createSignal<Set<string>>(new Set());

  const flatResources = createMemo(() =>
    flattenAlertResourceTableResources(options.resources, options.groupedResources),
  );
  const hasRows = createMemo(() =>
    hasAlertResourceTableRows(options.resources, options.groupedResources, options.globalDefaults),
  );
  const hasCustomGlobalDefaults = createMemo(() =>
    hasCustomAlertResourceGlobalDefaults(options.globalDefaults, options.factoryDefaults),
  );

  const toggleSelection = (id: string, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  };

  const toggleAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set<string>(flatResources().map((resource) => resource.id)));
      return;
    }
    setSelectedIds(new Set<string>());
  };

  const allSelected = createMemo(() => {
    const total = flatResources().length;
    return total > 0 && selectedIds().size === total;
  });

  const someSelected = createMemo(() => {
    return selectedIds().size > 0 && selectedIds().size < flatResources().length;
  });

  const clearSelectedIds = () => setSelectedIds(new Set<string>());

  return {
    activeMetricInput,
    setActiveMetricInput,
    showDelayRow,
    setShowDelayRow,
    selectedIds,
    setSelectedIds,
    hasRows,
    hasCustomGlobalDefaults,
    toggleSelection,
    toggleAll,
    allSelected,
    someSelected,
    clearSelectedIds,
  };
}
