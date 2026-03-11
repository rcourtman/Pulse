import { STORAGE_VIEW_OPTIONS } from '@/features/storageBackups/storagePagePresentation';
import type { StorageView } from './storagePageState';

type StorageControlsModelOptions = {
  selectedNodeId: () => string;
  setSelectedNodeId: (value: string) => void;
  onViewChange: (value: StorageView) => void;
};

export const useStorageControlsModel = (options: StorageControlsModelOptions) => {
  const handleNodeFilterChange = (value: string) => {
    options.setSelectedNodeId(value);
  };

  const handleViewChange = (value: string) => {
    options.onViewChange(value as StorageView);
  };

  return {
    viewTabs: STORAGE_VIEW_OPTIONS as { value: string; label: string }[],
    handleNodeFilterChange,
    handleViewChange,
  };
};
