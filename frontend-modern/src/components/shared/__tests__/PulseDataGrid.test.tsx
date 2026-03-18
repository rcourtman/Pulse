import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';

type TestRow = {
  id: string;
  name: string;
};

describe('PulseDataGrid', () => {
  afterEach(() => {
    cleanup();
  });

  it('triggers the row handler when a non-interactive cell is clicked', () => {
    const onRowClick = vi.fn();

    render(() => (
      <PulseDataGrid<TestRow>
        data={[{ id: '1', name: 'Tower' }]}
        columns={[{ key: 'name', label: 'Name' }]}
        keyExtractor={(row) => row.id}
        onRowClick={onRowClick}
      />
    ));

    fireEvent.click(screen.getByText('Tower'));
    expect(onRowClick).toHaveBeenCalledTimes(1);
  });

  it('does not trigger the row handler when an interactive child is clicked', () => {
    const onRowClick = vi.fn();
    const onRemove = vi.fn();

    render(() => (
      <PulseDataGrid<TestRow>
        data={[{ id: '1', name: 'Tower' }]}
        columns={[
          { key: 'name', label: 'Name' },
          {
            key: 'actions',
            label: 'Actions',
            render: () => (
              <button type="button" onClick={onRemove}>
                Remove
              </button>
            ),
          },
        ]}
        keyExtractor={(row) => row.id}
        onRowClick={onRowClick}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));
    expect(onRemove).toHaveBeenCalledTimes(1);
    expect(onRowClick).not.toHaveBeenCalled();
  });

  it('keeps the same row DOM node when data refreshes with the same key', () => {
    const [rows, setRows] = createSignal<TestRow[]>([{ id: '1', name: 'Tower' }]);

    render(() => (
      <PulseDataGrid<TestRow>
        data={rows()}
        columns={[{ key: 'name', label: 'Name' }]}
        keyExtractor={(row) => row.id}
      />
    ));

    const initialCell = screen.getByText('Tower');
    setRows([{ id: '1', name: 'Tower' }]);
    const refreshedCell = screen.getByText('Tower');

    expect(refreshedCell).toBe(initialCell);
  });
});
