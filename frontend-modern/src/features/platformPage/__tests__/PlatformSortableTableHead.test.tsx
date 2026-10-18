import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { For, createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { PlatformSortableTableHead, createPlatformTableSortState } from '../sharedPlatformPage';

const rows = [
  { name: 'bravo', cpu: 40 },
  { name: 'alpha', cpu: null },
  { name: 'charlie', cpu: 90 },
];

function SortableTable() {
  const sort = createPlatformTableSortState({
    storageKey: 'accessibleHeaders',
    sortKeys: ['name', 'cpu'] as const,
    descendingFirst: ['cpu'],
  });
  return (
    <table>
      <thead>
        <tr>
          <PlatformSortableTableHead kind="name" sort={sort} sortKey="name">
            <span>Name</span>
          </PlatformSortableTableHead>
          <PlatformSortableTableHead kind="numeric-value" sort={sort} sortKey="cpu">
            CPU
          </PlatformSortableTableHead>
          <PlatformSortableTableHead kind="text" sort={sort}>
            Actions
          </PlatformSortableTableHead>
        </tr>
      </thead>
      <tbody>
        <For each={sort.sortRows(rows, (row, key) => row[key])}>
          {(row) => (
            <tr>
              <td>{row.name}</td>
              <td>{row.cpu ?? 'Unknown'}</td>
              <td />
            </tr>
          )}
        </For>
      </tbody>
    </table>
  );
}

const rowNames = () =>
  [...document.querySelectorAll('tbody tr')].map((row) => row.firstElementChild?.textContent);

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

describe('PlatformSortableTableHead', () => {
  it('exposes native sort buttons within column headers, but no action for static columns', () => {
    render(() => <SortableTable />);
    expect(screen.getAllByRole('button')).toHaveLength(2);
    for (const name of ['Name', 'CPU']) {
      const header = screen.getByRole('columnheader', { name });
      const button = within(header).getByRole('button', { name });
      expect(header).toHaveAttribute('scope', 'col');
      expect(header).not.toHaveAttribute('tabindex');
      expect(header).not.toHaveAttribute('aria-sort');
      expect(button).toHaveAttribute('type', 'button');
      expect(button.tabIndex).toBe(0);
    }
    const staticHeader = screen.getByRole('columnheader', { name: 'Actions' });
    expect(staticHeader).toHaveAttribute('scope', 'col');
    expect(within(staticHeader).queryByRole('button')).not.toBeInTheDocument();
    expect(staticHeader).not.toHaveAttribute('tabindex');
    expect(staticHeader).not.toHaveAttribute('aria-sort');
  });

  it('runs one sort transition per native activation and keeps the button and name stable', async () => {
    render(() => <SortableTable />);
    const header = screen.getByRole('columnheader', { name: 'Name' });
    const button = within(header).getByRole('button', { name: 'Name' });
    button.focus();
    for (const [direction, names] of [
      ['ascending', ['alpha', 'bravo', 'charlie']],
      ['descending', ['charlie', 'bravo', 'alpha']],
      [undefined, ['bravo', 'alpha', 'charlie']],
    ] as const) {
      await fireEvent.click(button);
      if (direction) expect(header).toHaveAttribute('aria-sort', direction);
      else expect(header).not.toHaveAttribute('aria-sort');
      expect(rowNames()).toEqual(names);
      expect(within(header).getByRole('button', { name: 'Name' })).toBe(button);
      expect(button).toHaveFocus();
      expect(button).not.toHaveAttribute('aria-sort');
    }
  });

  it('keeps metric-first direction, missing-last ordering and persisted state', async () => {
    window.localStorage.setItem('accessibleHeadersSortKey', 'cpu');
    window.localStorage.setItem('accessibleHeadersSortDirection', 'desc');
    render(() => <SortableTable />);
    const header = screen.getByRole('columnheader', { name: 'CPU' });
    const button = within(header).getByRole('button', { name: 'CPU' });
    expect(header).toHaveAttribute('aria-sort', 'descending');
    expect(rowNames()).toEqual(['charlie', 'bravo', 'alpha']);
    expect(button.querySelector('[aria-hidden="true"]')).toHaveTextContent('▼');
    await fireEvent.click(button);
    expect(header).toHaveAttribute('aria-sort', 'ascending');
    expect(rowNames()).toEqual(['bravo', 'charlie', 'alpha']);
    expect(button.querySelector('[aria-hidden="true"]')).toHaveTextContent('▲');
    await fireEvent.click(button);
    expect(header).not.toHaveAttribute('aria-sort');
    await fireEvent.click(button);
    expect(header).toHaveAttribute('aria-sort', 'descending');
    cleanup();
    render(() => <SortableTable />);
    expect(screen.getByRole('columnheader', { name: 'CPU' })).toHaveAttribute(
      'aria-sort',
      'descending',
    );
  });

  it('removes the interactive control if a column ceases to be sortable', async () => {
    const [sortable, setSortable] = createSignal(true);
    render(() => {
      const sort = createPlatformTableSortState({
        storageKey: 'optionalHeader',
        sortKeys: ['name'] as const,
      });
      return (
        <table>
          <thead>
            <tr>
              <PlatformSortableTableHead
                kind="name"
                sort={sort}
                sortKey={sortable() ? 'name' : undefined}
              >
                Name
              </PlatformSortableTableHead>
            </tr>
          </thead>
        </table>
      );
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Name' }));
    setSortable(false);
    const header = screen.getByRole('columnheader', { name: 'Name' });
    expect(within(header).queryByRole('button')).not.toBeInTheDocument();
    expect(header).not.toHaveAttribute('aria-sort');
  });
});
