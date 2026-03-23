import { createEffect, createMemo } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import type { PulseDataGridProps, PulseDataGridStableRow } from './pulseDataGridModel';

type PulseDataGridStateOptions<T> = Pick<
  PulseDataGridProps<T>,
  'data' | 'keyExtractor' | 'desktopMinWidth' | 'mobileMinWidth'
>;

export function usePulseDataGridState<T>(options: PulseDataGridStateOptions<T>) {
  const { isMobile } = useBreakpoint();
  const [stableRows, setStableRows] = createStore<PulseDataGridStableRow<T>[]>([]);

  const effectiveMinWidth = createMemo(() => {
    if (isMobile()) {
      return options.mobileMinWidth ?? '100%';
    }
    return options.desktopMinWidth ?? '800px';
  });

  createEffect(() => {
    setStableRows(
      reconcile(
        options.data.map((row) => ({
          __pulseKey: options.keyExtractor(row),
          value: row,
        })),
        { key: '__pulseKey' },
      ),
    );
  });

  return {
    effectiveMinWidth,
    stableRows,
  };
}
