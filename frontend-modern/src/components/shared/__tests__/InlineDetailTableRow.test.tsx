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

  it('re-spans when a summary column is removed or restored while the row stays open', async () => {
    const [showSystem, setShowSystem] = createSignal(true);
    render(() => (
      <Table>
        <TableBody>
          <tr>
            <td>Machine</td>
            <Show when={showSystem()}>
              <td>System</td>
            </Show>
            <td>Seen</td>
          </tr>
          <InlineDetailTableRow colspan={3}>
            <div>Column picker detail</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));
    const detailCell = () => screen.getByText('Column picker detail').closest('td');

    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '3'));
    setShowSystem(false);
    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '2'));
    setShowSystem(true);
    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '3'));
    // The relayout nudge for a growing span must not leave a width behind.
    expect(detailCell()?.style.width).toBe('');
  });

  it('forces one layout of the widened detail cell when its span grows, none when it shrinks', async () => {
    const [showSystem, setShowSystem] = createSignal(false);
    render(() => (
      <Table>
        <TableBody>
          <tr>
            <td>Machine</td>
            <Show when={showSystem()}>
              <td>System</td>
            </Show>
            <td>Seen</td>
          </tr>
          <InlineDetailTableRow colspan={showSystem() ? 3 : 2}>
            <div>Growing span detail</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));
    const detailCell = screen.getByText('Growing span detail').closest('td')!;
    await waitFor(() => expect(detailCell).toHaveAttribute('colspan', '2'));

    // jsdom has no layout, so record the forced layout reads Chromium needs to
    // recompute fixed-layout column widths for the widened cell.
    const layoutReads: Array<{ width: string; colspan: string | null }> = [];
    const offsetWidth = vi
      .spyOn(HTMLElement.prototype, 'offsetWidth', 'get')
      .mockImplementation(function (this: HTMLElement) {
        if (this === detailCell) {
          layoutReads.push({ width: this.style.width, colspan: this.getAttribute('colspan') });
        }
        return 0;
      });
    try {
      // The requested span and the summary cells change in the same update.
      setShowSystem(true);
      await waitFor(() => expect(detailCell).toHaveAttribute('colspan', '3'));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(layoutReads).toEqual([{ width: '0px', colspan: '3' }]);
      expect(detailCell.style.width).toBe('');

      layoutReads.length = 0;
      setShowSystem(false);
      await waitFor(() => expect(detailCell).toHaveAttribute('colspan', '2'));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(layoutReads).toEqual([]);
    } finally {
      offsetWidth.mockRestore();
    }
  });

  it('re-measures for a summary cell span change but not for churn inside a cell', async () => {
    const [uptimeSpan, setUptimeSpan] = createSignal(1);
    const [reading, setReading] = createSignal({ value: '24%', tone: 'text-green-600' });
    const [busy, setBusy] = createSignal(false);
    render(() => (
      <Table>
        <TableBody>
          <tr>
            <td>Machine</td>
            <td colspan={uptimeSpan()}>
              <span class={reading().tone}>{reading().value}</span>
              <Show when={busy()}>
                <em>refreshing</em>
              </Show>
            </td>
          </tr>
          <InlineDetailTableRow colspan={2}>
            <div>Live metric detail</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));
    const detailCell = () => screen.getByText('Live metric detail').closest('td');
    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '2'));

    const getComputedStyleSpy = vi.spyOn(window, 'getComputedStyle');
    try {
      setReading({ value: '91%', tone: 'text-red-600' });
      setBusy(true);
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(getComputedStyleSpy).not.toHaveBeenCalled();

      setUptimeSpan(2);
      await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '3'));
      expect(getComputedStyleSpy).toHaveBeenCalled();
    } finally {
      getComputedStyleSpy.mockRestore();
    }
  });

  it('re-spans when a summary cell is hidden in place while the row stays open', async () => {
    const [hideUptime, setHideUptime] = createSignal(false);
    render(() => (
      <Table>
        <TableBody>
          <tr>
            <td>Machine</td>
            <td style={{ display: hideUptime() ? 'none' : undefined }}>Uptime</td>
            <td>Seen</td>
          </tr>
          <InlineDetailTableRow colspan={3}>
            <div>Hidden cell detail</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));
    const detailCell = () => screen.getByText('Hidden cell detail').closest('td');

    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '3'));
    setHideUptime(true);
    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '2'));
  });

  it('follows a changed requested span when there is no summary row to measure', async () => {
    const [colspan, setColspan] = createSignal(5);
    render(() => (
      <Table>
        <TableBody>
          <InlineDetailTableRow colspan={colspan()}>
            <div>Requested span detail</div>
          </InlineDetailTableRow>
        </TableBody>
      </Table>
    ));
    const detailCell = () => screen.getByText('Requested span detail').closest('td');

    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '5'));
    setColspan(6);
    await waitFor(() => expect(detailCell()).toHaveAttribute('colspan', '6'));
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
