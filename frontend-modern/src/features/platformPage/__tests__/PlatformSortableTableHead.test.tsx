import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { For } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { TableCell, TableRow } from '@/components/shared/Table';
import {
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableShell,
  createPlatformTableSortState,
} from '../sharedPlatformPage';

function mountTable() {
  return render(() => {
    const sort = createPlatformTableSortState({
      storageKey: 'accessibleHeaders',
      sortKeys: ['name', 'completed'] as const,
      descendingFirst: ['completed'],
    });
    const rows = [
      { name: 'bravo', completed: 2 },
      { name: 'alpha', completed: 1 },
      { name: 'unknown', completed: null },
    ];
    return (
      <PlatformTableShell
        header={
          <>
            <PlatformSortableTableHead kind="name" sort={sort} sortKey="name">
              Name
            </PlatformSortableTableHead>
            <PlatformSortableTableHead kind="numeric-value" sort={sort} sortKey="completed">
              <PlatformResponsiveTableLabel compact="Finished" full="Completed" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead kind="text" sort={sort}>
              Source
            </PlatformSortableTableHead>
          </>
        }
        body={
          <For each={sort.sortRows(rows, (row, key) => row[key])}>
            {(row) => (
              <TableRow data-header-test-row={row.name}>
                <TableCell>{row.name}</TableCell>
                <TableCell>{row.completed ?? '-'}</TableCell>
                <TableCell>TrueNAS</TableCell>
              </TableRow>
            )}
          </For>
        }
      />
    );
  });
}

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe('PlatformSortableTableHead', () => {
  it('does not recreate or erase responsive heading text when the pointer sort marker changes', async () => {
    mountTable();
    const heading = screen.getByRole('columnheader', { name: /Completed/ });
    const label = heading.querySelector('.platform-table-label-full');
    for (let i = 0; i < 3; i++) {
      await fireEvent.click(heading);
      expect(heading.querySelector('.platform-table-label-full')).toBe(label);
      expect(heading).toHaveTextContent('Completed');
    }
  });

  it('uses scoped column headings and native, non-submitting sort controls only for sortable columns', () => {
    mountTable();
    const headings = screen.getAllByRole('columnheader');
    expect(headings).toHaveLength(3);
    for (const heading of headings) expect(heading).toHaveAttribute('scope', 'col');
    for (const button of screen.getAllByRole('button')) {
      expect(button).toHaveAttribute('type', 'button');
      button.focus();
      expect(button).toHaveFocus();
    }
    expect(screen.getAllByRole('button')).toHaveLength(2);
    expect(within(headings[2]!).queryByRole('button')).not.toBeInTheDocument();
  });

  it('keeps responsive labels and focus stable through sorting, reset and persisted restoration', async () => {
    const { unmount } = mountTable();
    const heading = screen.getByRole('columnheader', { name: /Completed/ });
    const control = within(heading).getByRole('button');
    const label = heading.querySelector('.platform-table-label-full');
    const names = () =>
      [...document.querySelectorAll('[data-header-test-row]')].map((row) =>
        row.getAttribute('data-header-test-row'),
      );
    control.focus();
    for (const [direction, expected] of [
      ['descending', ['bravo', 'alpha', 'unknown']],
      ['ascending', ['alpha', 'bravo', 'unknown']],
      [undefined, ['bravo', 'alpha', 'unknown']],
    ] as const) {
      await fireEvent.click(control);
      if (direction) expect(heading).toHaveAttribute('aria-sort', direction);
      else expect(heading).not.toHaveAttribute('aria-sort');
      expect(names()).toEqual(expected);
      expect(control).toHaveFocus();
      expect(heading.querySelector('.platform-table-label-full')).toBe(label);
      expect(control).toHaveAccessibleName(/Completed/);
      expect(control).not.toHaveAccessibleName(/[▲▼]/);
    }
    await fireEvent.click(
      within(screen.getByRole('columnheader', { name: 'Name' })).getByRole('button'),
    );
    expect(heading).not.toHaveAttribute('aria-sort');
    expect(screen.getByRole('columnheader', { name: /Name/ })).toHaveAttribute(
      'aria-sort',
      'ascending',
    );
    expect(localStorage.getItem('accessibleHeadersSortKey')).toBe('name');
    unmount();
    mountTable();
    expect(screen.getByRole('columnheader', { name: 'Name' })).toHaveAttribute(
      'aria-sort',
      'ascending',
    );
    expect(names()).toEqual(['alpha', 'bravo', 'unknown']);
  });
});
