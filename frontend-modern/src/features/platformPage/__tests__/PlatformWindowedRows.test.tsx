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
});
