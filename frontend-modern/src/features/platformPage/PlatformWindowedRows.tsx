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
  __platformWindowStableRow: true;
  __platformWindowKey: string | number;
  value: Accessor<Row>;
  update: (value: Row) => void;
};

const defaultRowKey = <Row,>(item: Row): string | number | undefined => {
  if (typeof item !== 'object' || item === null || !('id' in item)) return undefined;
  const id = (item as { id?: unknown }).id;
  return typeof id === 'string' || typeof id === 'number' ? id : undefined;
};

const stableRowKeys = <Row,>(
  items: readonly Row[],
  keyExtractor?: (item: Row) => string | number,
): (string | number)[] | undefined => {
  const keys = items.map((item) => keyExtractor?.(item) ?? defaultRowKey(item));
  if (keys.some((key) => key === undefined) || new Set(keys).size !== keys.length) return undefined;
  return keys as (string | number)[];
};

const createStableRow = <Row,>(key: string | number, initialValue: Row): StablePlatformRow<Row> => {
  if (typeof initialValue === 'object' && initialValue !== null) {
    const [value, setValue] = createStore(initialValue);
    return {
      __platformWindowStableRow: true,
      __platformWindowKey: key,
      value: () => value,
      update: (nextValue) => {
        if (typeof nextValue === 'object' && nextValue !== null) {
          setValue(reconcile(nextValue));
        }
      },
    };
  }

  const [value, setValue] = createSignal(initialValue);
  return {
    __platformWindowStableRow: true,
    __platformWindowKey: key,
    value,
    update: (nextValue) => setValue(() => nextValue),
  };
};

const isStableRow = <Row,>(item: Row | StablePlatformRow<Row>): item is StablePlatformRow<Row> =>
  typeof item === 'object' && item !== null && '__platformWindowStableRow' in item;

/**
 * Canonical bounded renderer for ordinary platform table rows.
 *
 * The full filtered/sorted result remains in memory, while only a directional
 * runway is mounted. Spacer rows preserve native page scrolling; wheel input
 * prewarms the runway while touch input remains compositor-native.
 */
export function PlatformWindowedRows<Row>(props: PlatformWindowedRowsProps<Row>) {
  const rowsByKey = new Map<string | number, StablePlatformRow<Row>>();
  const initialItems = props.items();
  const initialKeys = stableRowKeys(initialItems, props.keyExtractor);
  const initialRows = initialKeys?.map((key, index) => {
    const row = createStableRow<Row>(key, initialItems[index] as Row);
    rowsByKey.set(key, row);
    return row;
  });
  const [stableRows, setStableRows] = createStore<StablePlatformRow<Row>[]>(initialRows ?? []);
  const [usesStableRows, setUsesStableRows] = createSignal(initialRows !== undefined);

  createEffect(() => {
    const items = props.items();
    const keys = stableRowKeys(items, props.keyExtractor);
    if (!keys) {
      rowsByKey.clear();
      setUsesStableRows(false);
      return;
    }

    const activeKeys = new Set(keys);
    const nextRows = items.map((item, index) => {
      const key = keys[index]!;
      const existing = rowsByKey.get(key);
      if (existing) {
        existing.update(item);
        return existing;
      }
      const created = createStableRow<Row>(key, item);
      rowsByKey.set(key, created);
      return created;
    });
    for (const key of rowsByKey.keys()) {
      if (!activeKeys.has(key)) rowsByKey.delete(key);
    }
    setStableRows(reconcile(nextRows, { key: '__platformWindowKey' }));
    setUsesStableRows(true);
  });

  const renderItems = createMemo<readonly (Row | StablePlatformRow<Row>)[]>(() =>
    usesStableRows() ? stableRows : props.items(),
  );
  const windowing = usePlatformWindowedItems({
    items: renderItems,
    estimatedItemHeight: props.estimatedRowHeight,
    enableThreshold: props.enableThreshold,
    windowSize: props.windowSize,
  });

  const renderRows = (items: readonly (Row | StablePlatformRow<Row>)[], globalOffset: number) => (
    <For each={items}>
      {(item, index) =>
        props.children(isStableRow(item) ? item.value() : item, () => globalOffset + index())
      }
    </For>
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
