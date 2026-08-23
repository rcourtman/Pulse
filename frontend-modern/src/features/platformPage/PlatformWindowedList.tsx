import { For, Show, type Accessor, type JSX } from 'solid-js';

import { usePlatformWindowedItems } from './usePlatformWindowedItems';

export interface PlatformWindowedListProps<Item> {
  items: Accessor<readonly Item[]>;
  children: (item: Item, index: Accessor<number>) => JSX.Element;
  estimatedItemHeight?: number;
  enableThreshold?: number;
  windowSize?: number;
}

/** Canonical bounded renderer for native-scrolling platform card/list surfaces. */
export function PlatformWindowedList<Item>(props: PlatformWindowedListProps<Item>) {
  const windowing = usePlatformWindowedItems({
    items: props.items,
    estimatedItemHeight: props.estimatedItemHeight,
    enableThreshold: props.enableThreshold,
    windowSize: props.windowSize,
  });
  const renderItems = (items: readonly Item[], globalOffset: number) => (
    <For each={items}>{(item, index) => props.children(item, () => globalOffset + index())}</For>
  );

  return (
    <Show when={windowing.isWindowed()} fallback={renderItems(props.items(), 0)}>
      <div
        ref={windowing.setAnchorRef}
        aria-hidden="true"
        data-platform-window-spacer="top"
        style={{ height: `${windowing.topSpacerHeight()}px` }}
      />
      {renderItems(windowing.visibleItems(), windowing.startIndex())}
      <div
        aria-hidden="true"
        data-platform-window-spacer="bottom"
        style={{ height: `${windowing.bottomSpacerHeight()}px` }}
      />
    </Show>
  );
}
