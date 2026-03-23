import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { getHiddenColumnCount, shouldShowColumnPickerReset } from './columnPickerModel';

export interface ColumnPickerProps {
  columns: ColumnDef[];
  isHidden: (id: string) => boolean;
  onToggle: (id: string) => void;
  onReset?: () => void;
}

export function useColumnPickerState(props: ColumnPickerProps) {
  const [isOpen, setIsOpen] = createSignal(false);
  let containerRef: HTMLDivElement | undefined;

  const handleClickOutside = (event: MouseEvent) => {
    if (containerRef && !containerRef.contains(event.target as Node)) {
      setIsOpen(false);
    }
  };

  createEffect(() => {
    if (!isOpen()) {
      return;
    }

    document.addEventListener('mousedown', handleClickOutside);
    onCleanup(() => {
      document.removeEventListener('mousedown', handleClickOutside);
    });
  });

  const hiddenCount = createMemo(() => getHiddenColumnCount(props.columns, props.isHidden));
  const showReset = createMemo(() => shouldShowColumnPickerReset(props.onReset, hiddenCount()));

  return {
    handleColumnToggle: (id: string) => props.onToggle(id),
    handleResetClick: () => props.onReset?.(),
    hiddenCount,
    isColumnChecked: (id: string) => !props.isHidden(id),
    isOpen,
    setContainerRef: (element: HTMLDivElement) => {
      containerRef = element;
    },
    showReset,
    toggleOpen: () => setIsOpen((open) => !open),
  };
}
