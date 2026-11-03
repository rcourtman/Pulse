import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { TableCell, nativeRowClickTarget } from '@/components/shared/Table';

type TestRow = {
  id: string;
  name: string;
};

describe('PulseDataGrid', () => {
  afterEach(() => {
    cleanup();
  });

  it('preserves caller-owned table cell height attributes without inline styles', () => {
    render(() => (
      <table>
        <tbody>
          <tr>
            <TableCell height={64}>spacer</TableCell>
          </tr>
        </tbody>
      </table>
    ));

    const cell = screen.getByText('spacer').closest('td');
    expect(cell).not.toBeNull();
    expect(cell).toHaveAttribute('height', '64');
    expect(cell).not.toHaveAttribute('style');
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

  it('does not run the row handler for a drag that selects row text', () => {
    const onRowClick = vi.fn();

    render(() => (
      <PulseDataGrid<TestRow>
        data={[{ id: '1', name: 'Copyable tower' }]}
        columns={[{ key: 'name', label: 'Name' }]}
        keyExtractor={(row) => row.id}
        onRowClick={onRowClick}
      />
    ));

    const name = screen.getByText('Copyable tower');
    const range = document.createRange();
    range.selectNodeContents(name);
    const selection = document.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
    try {
      fireEvent.mouseDown(name, { clientX: 4, clientY: 8 });
      fireEvent.click(name, { clientX: 96, clientY: 8 });
      expect(onRowClick).not.toHaveBeenCalled();

      fireEvent.mouseDown(name, { clientX: 40, clientY: 8 });
      fireEvent.click(name, { clientX: 40, clientY: 8 });
      expect(onRowClick).toHaveBeenCalledTimes(1);
    } finally {
      selection.removeAllRanges();
    }
  });

  it('inherits the native touch click target from the shared TableRow', () => {
    const nativeListener = vi.spyOn(HTMLTableRowElement.prototype, 'addEventListener');
    const onRowClick = vi.fn();
    try {
      render(() => (
        <PulseDataGrid<TestRow>
          data={[{ id: '1', name: 'Touch tower' }]}
          columns={[{ key: 'name', label: 'Name' }]}
          keyExtractor={(row) => row.id}
          onRowClick={onRowClick}
        />
      ));
      expect(nativeListener.mock.calls.find(([type]) => type === 'click')?.[1]).toBe(
        nativeRowClickTarget,
      );
      nativeRowClickTarget();
      expect(onRowClick).not.toHaveBeenCalled();
      fireEvent.click(screen.getByText('Touch tower'));
      expect(onRowClick).toHaveBeenCalledTimes(1);
    } finally {
      nativeListener.mockRestore();
    }
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

  it('applies the shared flush frame for grids embedded inside existing panels', () => {
    render(() => (
      <PulseDataGrid<TestRow>
        data={[{ id: '1', name: 'Tower' }]}
        columns={[{ key: 'name', label: 'Name' }]}
        keyExtractor={(row) => row.id}
        frame="flush"
      />
    ));

    const table = screen.getByRole('table');
    const frame = table.parentElement?.parentElement;

    expect(frame).not.toBeNull();
    expect(frame).toHaveClass('rounded-none');
    expect(frame).toHaveClass('border-0');
    expect(frame).not.toHaveClass('rounded-md');
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

  it('lets the grid own header, cell and expansion padding instead of the shared base', () => {
    render(() => (
      <PulseDataGrid<TestRow>
        data={[{ id: '1', name: 'Tower' }]}
        columns={[{ key: 'name', label: 'Name' }]}
        keyExtractor={(row) => row.id}
        isRowExpanded={() => true}
        expandedRender={() => <div>Tower details</div>}
      />
    ));

    const head = screen.getByText('Name').closest('th')!;
    expect(head.className).toContain('px-3 sm:px-4 py-2.5');
    expect(head.className).not.toMatch(/(?:^|\s)(?:px|pl|pr)-2\b/);
    expect(head.className).not.toContain('sm:px-3');
    expect(head.className).not.toContain('py-1.5');

    const cell = screen.getByText('Tower').closest('td')!;
    expect(cell.className).toContain('px-3 sm:px-4 py-2 sm:py-3.5');
    expect(cell.className).not.toMatch(/(?:^|\s)(?:px|pl|pr)-2\b/);
    expect(cell.className).not.toContain('sm:px-3');
    expect(cell.className).not.toContain('py-0.5');

    const expansion = screen.getByText('Tower details').closest('td')!;
    expect(expansion).toHaveAttribute('colspan', '1');
    expect(expansion.className).toContain('px-0 py-0');
    expect(expansion.className).not.toMatch(/(?:^|\s)(?:px|pl|pr)-2\b/);
    expect(expansion.className).not.toContain('sm:px-3');
    expect(expansion.className).not.toContain('py-0.5');
  });
});
