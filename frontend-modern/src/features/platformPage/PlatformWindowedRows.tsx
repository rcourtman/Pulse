import { For, Show, type Accessor, type JSX } from 'solid-js';

import { usePlatformWindowedItems } from './usePlatformWindowedItems';

export interface PlatformWindowedRowsProps<Row> {
  items: Accessor<readonly Row[]>;
  children: (item: Row, index: Accessor<number>) => JSX.Element;
  colSpan?: number;
  estimatedRowHeight?: number;
  enableThreshold?: number;
  windowSize?: number;
}

/**
 * Canonical bounded renderer for ordinary platform table rows.
 *
 * The full filtered/sorted result remains in memory, while only a directional
 * runway is mounted. Spacer rows preserve native page scrolling and wheel/touch
 * listeners move the runway before the compositor advances the viewport.
 */
export function PlatformWindowedRows<Row>(props: PlatformWindowedRowsProps<Row>) {
  const windowing = usePlatformWindowedItems({
    items: props.items,
    estimatedItemHeight: props.estimatedRowHeight,
    enableThreshold: props.enableThreshold,
    windowSize: props.windowSize,
  });

  const renderRows = (items: readonly Row[], globalOffset: number) => (
    <For each={items}>{(item, index) => props.children(item, () => globalOffset + index())}</For>
  );

  return (
    <Show when={windowing.isWindowed()} fallback={renderRows(props.items(), 0)}>
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
