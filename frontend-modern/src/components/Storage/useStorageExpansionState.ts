import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';
import {
  haveSameStorageGroups,
  resolveExpandedStorageGroups,
  toggleCollapsedStorageGroup,
  type StorageView,
} from './storagePageState';
import type { StorageGroupKey } from './useStorageModel';

type UseStorageExpansionStateOptions = {
  groupBy: Accessor<StorageGroupKey>;
  groupedKeys: Accessor<string[]>;
  view: Accessor<StorageView>;
};

export const useStorageExpansionState = (options: UseStorageExpansionStateOptions) => {
  const [collapsedGroups, setCollapsedGroups] = createSignal<Set<string>>(new Set());
  const [expandedPoolId, setExpandedPoolId] = createSignal<string | null>(null);
  const expandedGroups = createMemo(
    () => resolveExpandedStorageGroups(options.groupedKeys(), collapsedGroups()),
    undefined,
    { equals: haveSameStorageGroups },
  );

  // Group keys are bare labels, so collapse state belongs to one grouping:
  // regrouping opens every group, including the headerless ungrouped one.
  createEffect(() => {
    options.groupBy();
    setCollapsedGroups(new Set<string>());
  });

  createEffect(() => {
    if (options.view() !== 'pools') {
      setExpandedPoolId(null);
    }
  });

  const toggleGroup = (key: string) => {
    setCollapsedGroups((prev) => toggleCollapsedStorageGroup(prev, key));
  };

  return {
    expandedGroups,
    expandedPoolId,
    setExpandedPoolId,
    toggleGroup,
  };
};
