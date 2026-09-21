import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal, onCleanup } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { PlatformWindowedRows } from '../PlatformWindowedRows';
import { PlatformWindowedList } from '../PlatformWindowedList';
import platformWindowedItemsSource from '../usePlatformWindowedItems.ts?raw';

describe('PlatformWindowedRows', () => {
  afterEach(cleanup);

  it('keeps phone touch scrolling on the browser-native page path', () => {
    expect(platformWindowedItemsSource).toContain('bindWindowedPageScrollEvents');
    expect(platformWindowedItemsSource).not.toContain("addEventListener('touch");
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
