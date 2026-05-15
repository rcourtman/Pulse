import { createEffect, createMemo, type Accessor } from 'solid-js';
import { buildStorageSourceOptionsFromKeys } from '@/utils/storageSources';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import { normalizeStorageSourceKey } from '@/utils/storageSources';
import {
  DEFAULT_PHYSICAL_DISK_FACET_FILTER,
  PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
  PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
  normalizePhysicalDiskFacetFilter,
  type PhysicalDiskFilterOption,
} from '@/features/storageBackups/diskPresentation';
import {
  buildStorageNodeFilterOptions,
  coerceSelectedStorageNodeId,
  DEFAULT_STORAGE_DISK_GROUP_FILTER,
  DEFAULT_STORAGE_DISK_ROLE_FILTER,
  getActiveStorageNodeOptions,
  getStorageFilterGroupBy,
  getStorageStatusFilterValue,
  toStorageHealthFilterValue,
  type StorageStatusFilterValue,
  type StorageView,
} from './storagePageState';
import type { StorageNodeOption, StorageGroupKey } from './useStorageModel';

type UseStorageFilterStateOptions = {
  view: Accessor<StorageView>;
  nodeOptions: Accessor<StorageNodeOption[]>;
  diskNodeOptions: Accessor<StorageNodeOption[]>;
  selectedNodeId: Accessor<string>;
  setSelectedNodeId: (value: string) => void;
  sourceOptions: Accessor<string[]>;
  diskSourceOptions?: Accessor<string[]>;
  diskRoleOptions?: Accessor<PhysicalDiskFilterOption[]>;
  diskGroupOptions?: Accessor<PhysicalDiskFilterOption[]>;
  sourceFilter: Accessor<string>;
  setSourceFilter: (value: string) => void;
  lockedSourceFilter?: Accessor<string | undefined>;
  healthFilter: Accessor<StorageHealthFilter>;
  setHealthFilter: (value: StorageHealthFilter) => void;
  diskRoleFilter: Accessor<string>;
  setDiskRoleFilter: (value: string) => void;
  diskGroupFilter: Accessor<string>;
  setDiskGroupFilter: (value: string) => void;
  groupBy: Accessor<StorageGroupKey>;
};

export const useStorageFilterState = (options: UseStorageFilterStateOptions) => {
  const activeNodeOptions = createMemo(() =>
    getActiveStorageNodeOptions(options.view(), options.nodeOptions(), options.diskNodeOptions()),
  );
  const nodeFilterOptions = createMemo(() =>
    buildStorageNodeFilterOptions(options.view(), activeNodeOptions()),
  );
  const activeSourceKeys = createMemo(() =>
    options.view() === 'disks' && options.diskSourceOptions
      ? options.diskSourceOptions()
      : options.sourceOptions(),
  );
  const sourceFilterOptions = createMemo(() =>
    buildStorageSourceOptionsFromKeys(activeSourceKeys()),
  );
  const diskRoleFilterOptions = createMemo(() =>
    options.view() === 'disks' && options.diskRoleOptions
      ? options.diskRoleOptions()
      : [
          {
            value: DEFAULT_PHYSICAL_DISK_FACET_FILTER,
            label: PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
          },
        ],
  );
  const diskGroupFilterOptions = createMemo(() =>
    options.view() === 'disks' && options.diskGroupOptions
      ? options.diskGroupOptions()
      : [
          {
            value: DEFAULT_PHYSICAL_DISK_FACET_FILTER,
            label: PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
          },
        ],
  );

  const storageFilterGroupBy = (): StorageGroupKey => {
    return getStorageFilterGroupBy(options.groupBy());
  };

  const storageFilterStatus = (): StorageStatusFilterValue => {
    return getStorageStatusFilterValue(options.healthFilter());
  };

  const setStorageFilterStatus = (value: StorageStatusFilterValue) => {
    options.setHealthFilter(toStorageHealthFilterValue(value));
  };

  createEffect(() => {
    const next = coerceSelectedStorageNodeId(options.selectedNodeId(), activeNodeOptions());
    if (next !== options.selectedNodeId()) {
      options.setSelectedNodeId(next);
    }
  });

  createEffect(() => {
    const selectedSource = normalizeStorageSourceKey(options.sourceFilter());
    if (selectedSource === 'all') {
      return;
    }
    const lockedSource = normalizeStorageSourceKey(options.lockedSourceFilter?.() || '');
    if (lockedSource && selectedSource === lockedSource) {
      return;
    }
    const availableSources = new Set(
      activeSourceKeys().map((key) => normalizeStorageSourceKey(key)),
    );
    if (!availableSources.has(selectedSource)) {
      options.setSourceFilter('all');
    }
  });

  createEffect(() => {
    if (options.view() !== 'disks') {
      if (options.diskRoleFilter() !== DEFAULT_STORAGE_DISK_ROLE_FILTER) {
        options.setDiskRoleFilter(DEFAULT_STORAGE_DISK_ROLE_FILTER);
      }
      if (options.diskGroupFilter() !== DEFAULT_STORAGE_DISK_GROUP_FILTER) {
        options.setDiskGroupFilter(DEFAULT_STORAGE_DISK_GROUP_FILTER);
      }
      return;
    }

    const availableRoles = new Set(
      diskRoleFilterOptions().map((option) => normalizePhysicalDiskFacetFilter(option.value)),
    );
    const selectedRole = normalizePhysicalDiskFacetFilter(options.diskRoleFilter());
    if (selectedRole !== DEFAULT_STORAGE_DISK_ROLE_FILTER && !availableRoles.has(selectedRole)) {
      options.setDiskRoleFilter(DEFAULT_STORAGE_DISK_ROLE_FILTER);
    }

    const availableGroups = new Set(
      diskGroupFilterOptions().map((option) => normalizePhysicalDiskFacetFilter(option.value)),
    );
    const selectedGroup = normalizePhysicalDiskFacetFilter(options.diskGroupFilter());
    if (
      selectedGroup !== DEFAULT_STORAGE_DISK_GROUP_FILTER &&
      !availableGroups.has(selectedGroup)
    ) {
      options.setDiskGroupFilter(DEFAULT_STORAGE_DISK_GROUP_FILTER);
    }
  });

  return {
    activeNodeOptions,
    nodeFilterOptions,
    sourceFilterOptions,
    diskRoleFilterOptions,
    diskGroupFilterOptions,
    storageFilterGroupBy,
    storageFilterStatus,
    setStorageFilterStatus,
  };
};
