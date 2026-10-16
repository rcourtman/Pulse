import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createMemo, createSignal, onCleanup } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { PlatformWindowedRows } from '../PlatformWindowedRows';
import { PlatformWindowedList } from '../PlatformWindowedList';
import { findWindowedItemIndex, usePlatformWindowedItems } from '../usePlatformWindowedItems';
import platformWindowedItemsSource from '../usePlatformWindowedItems.ts?raw';

describe('PlatformWindowedRows', () => {
  afterEach(cleanup);

  it('keeps phone touch scrolling on the browser-native page path', () => {
    expect(platformWindowedItemsSource).toContain('bindWindowedPageScrollEvents');
    expect(platformWindowedItemsSource).not.toContain("addEventListener('touch");
  });

  it('uses opt-in logical-block measurement on native scroll and layout changes without widening the window', async () => {
    const [height, setHeight] = createSignal(120);
    const measure = vi.fn(() => height());
    const rect = vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 200,
      bottom: 120,
      width: 200,
      height: 120,
    } as DOMRect);
    const prototype = Element.prototype as Element & { checkVisibility?: () => boolean };
    const visibility = prototype.checkVisibility;
    prototype.checkVisibility = () => true;
    const Fixture = () => {
      const state = usePlatformWindowedItems({
        items: () => Array.from({ length: 50 }, (_, index) => index),
        windowSize: 8,
        estimatedItemHeight: 32,
        measureItemHeight: measure,
        measurementKey: height,
      });
      return (
        <div ref={state.setAnchorRef}>
          <span data-testid="mounted-items">{state.visibleItems().length}</span>
          <span data-testid="bottom-runway">{state.bottomSpacerHeight()}</span>
        </div>
      );
    };
    try {
      render(() => <Fixture />);
      expect(screen.getByTestId('mounted-items')).toHaveTextContent('8');
      expect(screen.getByTestId('bottom-runway')).toHaveTextContent('5040');
      const initial = measure.mock.calls.length;
      await fireEvent.scroll(window);
      expect(measure.mock.calls.length).toBeGreaterThan(initial);
      setHeight(180);
      expect(screen.getByTestId('bottom-runway')).toHaveTextContent('7560');
      expect(screen.getByTestId('mounted-items')).toHaveTextContent('8');
      cleanup();
      const disposed = measure.mock.calls.length;
      await fireEvent.scroll(window);
      expect(measure).toHaveBeenCalledTimes(disposed);
    } finally {
      cleanup();
      rect.mockRestore();
      if (visibility) prototype.checkVisibility = visibility;
      else Reflect.deleteProperty(prototype, 'checkVisibility');
    }
  });

  it('mounts small tables in full without virtual spacer rows', () => {
    const items = () => Array.from({ length: 20 }, (_, index) => index);
    const { container } = render(() => (
      <table>
        <tbody>
          <PlatformWindowedRows items={items}>
            {(item) => <tr data-row-id={item} />}
          </PlatformWindowedRows>
        </tbody>
      </table>
    ));

    expect(container.querySelectorAll('[data-row-id]')).toHaveLength(20);
    expect(container.querySelectorAll('[data-platform-window-spacer]')).toHaveLength(0);
  });

  it('keeps estate-sized tables within the shared mounted-row budget', () => {
    const items = () => Array.from({ length: 1_000 }, (_, index) => index);
    const { container } = render(() => (
      <table>
        <tbody>
          <PlatformWindowedRows items={items}>
            {(item) => <tr data-row-id={item} />}
          </PlatformWindowedRows>
        </tbody>
      </table>
    ));

    expect(container.querySelectorAll('[data-row-id]')).toHaveLength(140);
    expect(container.querySelectorAll('[data-platform-window-spacer]')).toHaveLength(2);
  });

  it('preserves keyed row component state when live snapshots replace row objects', async () => {
    let mounts = 0;
    let disposals = 0;
    const [items, setItems] = createSignal([{ id: 'node-a', label: 'First snapshot' }]);

    const StatefulRow = (props: { item: { id: string; label: string } }) => {
      mounts += 1;
      onCleanup(() => {
        disposals += 1;
      });
      return (
        <tr>
          <td>{props.item.label}</td>
          <td>
            <input aria-label="Row-local state" />
          </td>
        </tr>
      );
    };

    render(() => (
      <table>
        <tbody>
          <PlatformWindowedRows items={items}>
            {(item) => <StatefulRow item={item} />}
          </PlatformWindowedRows>
        </tbody>
      </table>
    ));

    const input = screen.getByRole('textbox', {
      name: 'Row-local state',
    }) as HTMLInputElement;
    input.value = 'still editing';
    setItems([{ id: 'node-a', label: 'Refreshed snapshot' }]);

    expect(await screen.findByText('Refreshed snapshot')).toBeInTheDocument();
    expect(mounts).toBe(1);
    expect(disposals).toBe(0);
    expect(screen.getByRole('textbox', { name: 'Row-local state' })).toBe(input);
    expect(input).toHaveValue('still editing');
  });

  it.each(['table', 'list'] as const)(
    'preserves editing state in retained %s items when the scroll window moves',
    async (kind) => {
      let scrollTop = 0;
      const rect = vi
        .spyOn(Element.prototype, 'getBoundingClientRect')
        .mockImplementation(function (this: Element) {
          const top = this.getAttribute('data-platform-window-spacer') === 'top' ? -scrollTop : 0;
          return {
            x: 0,
            y: top,
            top,
            left: 0,
            right: 200,
            bottom: top + 40,
            width: 200,
            height: 40,
            toJSON: () => ({}),
          } as DOMRect;
        });
      const height = Object.getOwnPropertyDescriptor(window, 'innerHeight');
      Object.defineProperty(window, 'innerHeight', { configurable: true, value: 120 });
      const visibility = (Element.prototype as Element & { checkVisibility?: () => boolean })
        .checkVisibility;
      (Element.prototype as Element & { checkVisibility?: () => boolean }).checkVisibility = () =>
        true;
      const items = Array.from({ length: 30 }, (_, id) => ({ id }));
      try {
        const content = (item: { id: number }) => <input aria-label={`Draft ${item.id}`} />;
        const { container } = render(() =>
          kind === 'table' ? (
            <table>
              <tbody>
                <PlatformWindowedRows items={() => items} windowSize={8} enableThreshold={8}>
                  {(item, index) => (
                    <tr data-row={item.id} data-position={index()}>
                      <td>{content(item)}</td>
                    </tr>
                  )}
                </PlatformWindowedRows>
              </tbody>
            </table>
          ) : (
            <PlatformWindowedList items={() => items} windowSize={8} enableThreshold={8}>
              {(item, index) => (
                <div data-row={item.id} data-position={index()}>
                  {content(item)}
                </div>
              )}
            </PlatformWindowedList>
          ),
        );
        const draft = screen.getByRole('textbox', { name: 'Draft 4' }) as HTMLInputElement;
        await fireEvent.input(draft, { target: { value: 'keep this edit' } });
        scrollTop = 160;
        await fireEvent.scroll(window);
        expect(container.querySelector('[data-platform-window-spacer="top"]')).not.toHaveStyle({
          height: '0px',
        });
        expect(screen.getByRole('textbox', { name: 'Draft 4' })).toBe(draft);
        expect(draft).toHaveValue('keep this edit');
        expect(container.querySelector('[data-row="4"]')).toHaveAttribute('data-position', '4');
        expect(container.querySelectorAll('[data-row]')).toHaveLength(8);
      } finally {
        cleanup();
        rect.mockRestore();
        if (height) Object.defineProperty(window, 'innerHeight', height);
        if (visibility)
          (Element.prototype as Element & { checkVisibility?: () => boolean }).checkVisibility =
            visibility;
        else Reflect.deleteProperty(Element.prototype, 'checkVisibility');
      }
    },
  );

  it('renders keyed rows once in their latest order', async () => {
    const [items, setItems] = createSignal([
      { key: 'alpha', label: 'Alpha' },
      { key: 'mike', label: 'Mike' },
      { key: 'zulu', label: 'Zulu' },
    ]);
    const { container } = render(() => (
      <table>
        <tbody>
          <PlatformWindowedRows items={items} keyExtractor={(item) => item.key}>
            {(item) => <tr data-row-key={item.key}>{item.label}</tr>}
          </PlatformWindowedRows>
        </tbody>
      </table>
    ));

    setItems([
      { key: 'zulu', label: 'Zulu refreshed' },
      { key: 'mike', label: 'Mike refreshed' },
      { key: 'alpha', label: 'Alpha refreshed' },
    ]);

    expect(await screen.findByText('Zulu refreshed')).toBeInTheDocument();
    expect([...container.querySelectorAll('[data-row-key]')].map((row) => row.textContent)).toEqual(
      ['Zulu refreshed', 'Mike refreshed', 'Alpha refreshed'],
    );
  });

  it('keeps estate-sized card lists within their configured mounted-item budget', () => {
    const items = () => Array.from({ length: 1_000 }, (_, index) => index);
    const { container } = render(() => (
      <PlatformWindowedList items={items} enableThreshold={24} windowSize={32}>
        {(item) => <article data-card-id={item} />}
      </PlatformWindowedList>
    ));

    expect(container.querySelectorAll('[data-card-id]')).toHaveLength(32);
    expect(container.querySelectorAll('[data-platform-window-spacer]')).toHaveLength(2);
  });

  it('keeps the caller estimate when the leading row is a short group header', () => {
    // Regression: a grouped card list renders a short group header first. The
    // viewport measurement used to sample only that leading sibling, collapse
    // the estimate to the header height and desynchronise the window from the
    // real scroll position, so a host's rows vanished while scrolling (#2130).
    const items = () => [
      { kind: 'group' as const },
      ...Array.from({ length: 40 }, (_, index) => ({ kind: 'resource' as const, index })),
    ];
    const originalRect = Element.prototype.getBoundingClientRect;
    const originalVisibility = (Element.prototype as Element & { checkVisibility?: () => boolean })
      .checkVisibility;
    Element.prototype.getBoundingClientRect = function (this: Element) {
      const raw = (this as HTMLElement).getAttribute?.('data-height');
      const height = raw ? Number(raw) : 0;
      return {
        height,
        width: 0,
        top: 0,
        left: 0,
        right: 0,
        bottom: height,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      } as DOMRect;
    };
    (Element.prototype as Element & { checkVisibility?: () => boolean }).checkVisibility = () =>
      true;

    try {
      const { container } = render(() => (
        <PlatformWindowedList
          items={items}
          estimatedItemHeight={200}
          enableThreshold={4}
          windowSize={8}
        >
          {(item) => (
            <article data-item-kind={item.kind} data-height={item.kind === 'group' ? 30 : 200} />
          )}
        </PlatformWindowedList>
      ));

      const bottomSpacer = container.querySelector(
        '[data-platform-window-spacer="bottom"]',
      ) as HTMLElement;
      // 41 items, 8 mounted: 33 unmounted rows at the caller's 200px estimate.
      expect(bottomSpacer.style.height).toBe('6600px');
    } finally {
      Element.prototype.getBoundingClientRect = originalRect;
      if (originalVisibility) {
        (Element.prototype as Element & { checkVisibility?: () => boolean }).checkVisibility =
          originalVisibility;
      } else {
        Reflect.deleteProperty(Element.prototype, 'checkVisibility');
      }
    }
  });
});

describe('opt-in nonuniform platform windowing', () => {
  it('maps exact edges and positions inside a tall logical item, never the terminal offset', () => {
    const offsets = [0, 2000, 2040, 2080];
    expect([-1, 0, 1000, 1999, 2000, 2039, 2040, 2080, 9000].map((position) =>
      findWindowedItemIndex(offsets, position),
    )).toEqual([0, 0, 0, 0, 1, 1, 2, 2, 2]);
    expect(findWindowedItemIndex([0], 100)).toBe(0);
  });

  it('keeps a tall detail mounted while read, uses cumulative spacers and retains bounds through collapse', async () => {
    const [detailHeight, setDetailHeight] = createSignal(2400);
    let scrollTop = 1000;
    const measure = vi.fn();
    const rect = vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(() => ({
      x: 0, y: -scrollTop, top: -scrollTop, bottom: 4000 - scrollTop,
      left: 0, right: 320, width: 320, height: 4000,
    }) as DOMRect);
    const height = Object.getOwnPropertyDescriptor(window, 'innerHeight');
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 240 });
    const Fixture = () => {
      const offsets = createMemo(() => [0, ...Array.from({ length: 50 }, (_, index) => detailHeight() + (index + 1) * 40)]);
      const state = usePlatformWindowedItems({
        items: () => Array.from({ length: 50 }, (_, index) => index),
        windowSize: 8,
        itemGeometry: { offsets, measure },
        measurementKey: detailHeight,
      });
      return (
        <div ref={state.setAnchorRef}>
          <span data-testid="first">{state.visibleItems()[0]}</span>
          <span data-testid="count">{state.visibleItems().length}</span>
          <span data-testid="top">{state.topSpacerHeight()}</span>
          <span data-testid="bottom">{state.bottomSpacerHeight()}</span>
        </div>
      );
    };
    try {
      render(() => <Fixture />);
      expect(screen.getByTestId('first')).toHaveTextContent('0');
      expect(screen.getByTestId('top')).toHaveTextContent('0');
      expect(screen.getByTestId('bottom')).toHaveTextContent('1680');
      scrollTop = 3200;
      await fireEvent.scroll(window);
      const first = Number(screen.getByTestId('first').textContent);
      expect(first).toBeGreaterThan(0);
      expect(Number(screen.getByTestId('top').textContent)).toBe(2400 + first * 40);
      expect(screen.getByTestId('count')).toHaveTextContent('8');
      // Collapsing the first row changes real geometry, not the item count.
      scrollTop = 0;
      setDetailHeight(0);
      expect(screen.getByTestId('first')).toHaveTextContent('0');
      expect(screen.getByTestId('top')).toHaveTextContent('0');
      expect(screen.getByTestId('bottom')).toHaveTextContent('1680');
      cleanup();
      const calls = measure.mock.calls.length;
      await fireEvent.scroll(window);
      expect(measure).toHaveBeenCalledTimes(calls);
    } finally {
      cleanup(); rect.mockRestore();
      if (height) Object.defineProperty(window, 'innerHeight', height);
    }
  });
});
