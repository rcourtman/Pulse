import { createEffect, createSignal, type Accessor } from 'solid-js';
import {
  syncExpandedStorageGroups,
  toggleExpandedStorageGroup,
  type StorageView,
} from './storagePageState';

type UseStorageExpansionStateOptions = {
  groupedKeys: Accessor<string[]>;
  view: Accessor<StorageView>;
};

export const useStorageExpansionState = (options: UseStorageExpansionStateOptions) => {
  const [expandedGroups, setExpandedGroups] = createSignal<Set<string>>(new Set());
  const [expandedPoolId, setExpandedPoolId] = createSignal<string | null>(null);

  createEffect(() => {
    setExpandedGroups((prev) => syncExpandedStorageGroups(prev, options.groupedKeys()));
  });

  createEffect(() => {
    if (options.view() !== 'pools') {
      setExpandedPoolId(null);
    }
  });

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => toggleExpandedStorageGroup(prev, key));
  };

  return {
    expandedGroups,
    expandedPoolId,
    setExpandedPoolId,
    toggleGroup,
  };
};
