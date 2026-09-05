import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Show, createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';

import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { Table, TableBody } from '@/components/shared/Table';

describe('InlineDetailTableRow', () => {
  it('renders the canonical inline detail shell with caller row attributes', () => {
    render(() => (
      <Table>
        <TableBody>
          <InlineDetailTableRow
            cellId="detail-row-a"
            colspan={4}
            data-inline-detail-for="row-a"
            data-platform-detail-row="row-a"
          >
            <div>Detail content</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));

    const detail = screen.getByText('Detail content');
    const row = detail.closest('tr');
    const cell = detail.closest('td');

    expect(row).toHaveAttribute('data-inline-detail-for', 'row-a');
    expect(row).toHaveAttribute('data-platform-detail-row', 'row-a');
    expect(cell).toHaveAttribute('id', 'detail-row-a');
    expect(cell).toHaveAttribute('colspan', '4');
    expect(cell).toHaveClass('p-0');
    expect(cell).toHaveClass('border-b');
    expect(cell).toHaveClass('bg-surface-alt');
    expect(detail.parentElement).toHaveClass('px-2');
    expect(detail.parentElement).toHaveClass('sm:px-4');
    expect(detail.parentElement).toHaveClass('sticky');
    expect(detail.parentElement).toHaveClass('left-0');
    expect(detail.parentElement).toHaveClass('min-w-0');
    expect(detail.parentElement).toHaveClass('whitespace-normal');
    expect(detail.parentElement).toHaveClass('max-w-[calc(100vw-3.5rem)]');
    expect(detail.parentElement).toHaveClass('lg:static');
    expect(detail.parentElement).toHaveClass('lg:max-w-none');
  });

  it('contains clicks inside the detail content by default', () => {
    const onRowClick = vi.fn();

    render(() => (
      <Table>
        <TableBody>
          <InlineDetailTableRow colspan={1} onClick={onRowClick}>
            <button type="button">Nested action</button>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));

    screen.getByRole('button', { name: 'Nested action' }).click();

    expect(onRowClick).not.toHaveBeenCalled();
  });

  it('spans only the summary cells visible at the current responsive layout', async () => {
    render(() => (
      <Table>
        <TableBody>
          <tr>
            <td>Identity</td>
            <td style={{ display: 'none' }}>Desktop-only detail</td>
            <td>State</td>
          </tr>
          <InlineDetailTableRow colspan={3}>
            <div>Responsive detail content</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));

    await waitFor(() => {
      expect(screen.getByText('Responsive detail content').closest('td')).toHaveAttribute(
        'colspan',
        '2',
      );
    });
  });

  it('restores focus to the controlling disclosure when focused detail content closes', async () => {
    const Fixture = () => {
      const [open, setOpen] = createSignal(true);
      return (
        <Table>
          <TableBody>
            <tr>
              <td>
                <button type="button" aria-controls="detail-row-focus">
                  Resource details
                </button>
              </td>
            </tr>
            <Show when={open()}>
              <InlineDetailTableRow cellId="detail-row-focus" colspan={1}>
                <button type="button" onClick={() => setOpen(false)}>
                  Close details
                </button>
              </InlineDetailTableRow>
            </Show>
          </TableBody>
        </Table>
      );
    };

    render(() => <Fixture />);
    const disclosure = screen.getByRole('button', { name: 'Resource details' });
    const disclosureFocus = vi.spyOn(disclosure, 'focus');
    const close = screen.getByRole('button', { name: 'Close details' });
    close.focus();
    await fireEvent.click(close);

    await waitFor(() => expect(disclosure).toHaveFocus());
    expect(disclosureFocus).toHaveBeenCalledWith({ preventScroll: true });
  });
});
