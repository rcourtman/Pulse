import {
  createEffect,
  createMemo,
  createSignal,
  For,
  Show,
  type Accessor,
  type JSX,
} from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';

import { usePlatformWindowedItems } from './usePlatformWindowedItems';

export interface PlatformWindowedRowsProps<Row> {
  items: Accessor<readonly Row[]>;
  children: (item: Row, index: Accessor<number>) => JSX.Element;
  /**
   * Stable logical identity for rows rebuilt from live resource snapshots.
   * When omitted, rows with unique string/number `id` fields are stabilized
   * automatically; non-resource rows retain reference-keyed rendering.
   */
  keyExtractor?: (item: Row) => string | number;
  colSpan?: number;
  estimatedRowHeight?: number;
  enableThreshold?: number;
  windowSize?: number;
}

type StablePlatformRow<Row> = {
  __platformWindowKey: string | number;
  value: Row;
};

const defaultRowKey = <Row,>(item: Row): string | number | undefined => {
  if (typeof item !== 'object' || item === null || !('id' in item)) return undefined;
  const id = (item as { id?: unknown }).id;
  return typeof id === 'string' || typeof id === 'number' ? id : undefined;
};

const buildStableRows = <Row,>(
  items: readonly Row[],
  keyExtractor?: (item: Row) => string | number,
): StablePlatformRow<Row>[] | undefined => {
  const keys = items.map((item) => keyExtractor?.(item) ?? defaultRowKey(item));
  if (keys.some((key) => key === undefined) || new Set(keys).size !== keys.length) return undefined;
  return items.map((value, index) => ({
    __platformWindowKey: keys[index]!,
    value,
  }));
};

/**
 * Canonical bounded renderer for ordinary platform table rows.
 *
 * The full filtered/sorted result remains in memory, while only a directional
 * runway is mounted. Spacer rows preserve native page scrolling; wheel input
 * prewarms the runway while touch input remains compositor-native.
 */
export function PlatformWindowedRows<Row>(props: PlatformWindowedRowsProps<Row>) {
  const initialStableRows = buildStableRows(props.items(), props.keyExtractor);
  const [stableRows, setStableRows] = createStore<StablePlatformRow<Row>[]>(
    initialStableRows ?? [],
  );
  const [usesStableRows, setUsesStableRows] = createSignal(initialStableRows !== undefined);

  createEffect(() => {
    const next = buildStableRows(props.items(), props.keyExtractor);
    if (!next) {
      setUsesStableRows(false);
      return;
    }
    setStableRows(reconcile(next, { key: '__platformWindowKey' }));
    setUsesStableRows(true);
  });

  const renderItems = createMemo<readonly Row[]>(() =>
    usesStableRows() ? stableRows.map((stableRow) => stableRow.value) : props.items(),
  );
  const windowing = usePlatformWindowedItems({
    items: renderItems,
    estimatedItemHeight: props.estimatedRowHeight,
    enableThreshold: props.enableThreshold,
    windowSize: props.windowSize,
  });

  const renderRows = (items: readonly Row[], globalOffset: number) => (
    <For each={items}>{(item, index) => props.children(item, () => globalOffset + index())}</For>
  );

  return (
    <Show when={windowing.isWindowed()} fallback={renderRows(renderItems(), 0)}>
      <tr
        ref={windowing.setAnchorRef}
        aria-hidden="true"
        data-platform-window-spacer="top"
        style={{ height: `${windowing.topSpacerHeight()}px` }}
      >
        <td
          colspan={props.colSpan ?? 100}
          class="!p-0"
          style={{ height: `${windowing.topSpacerHeight()}px` }}
        />
      </tr>
      {renderRows(windowing.visibleItems(), windowing.startIndex())}
      <tr
        aria-hidden="true"
        data-platform-window-spacer="bottom"
        style={{ height: `${windowing.bottomSpacerHeight()}px` }}
      >
        <td
          colspan={props.colSpan ?? 100}
          class="!p-0"
          style={{ height: `${windowing.bottomSpacerHeight()}px` }}
        />
      </tr>
    </Show>
  );
}
