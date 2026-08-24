import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getPlatformResourceDetailRowInteractionProps } from '../PlatformResourceDetailTableRow';

afterEach(cleanup);

describe('getPlatformResourceDetailRowInteractionProps', () => {
  const renderRow = (onToggle = vi.fn()) => {
    render(() => (
      <table>
        <tbody>
          <TableRow
            {...getPlatformResourceDetailRowInteractionProps({
              expanded: false,
              detailRowId: 'detail-row',
              onToggle,
            })}
          >
            <TableCell>
              <span>Resource</span>
              <button type="button">Open link action</button>
            </TableCell>
          </TableRow>
        </tbody>
      </table>
    ));
    return { onToggle, row: screen.getByRole('row'), childAction: screen.getByRole('button') };
  };

  it('owns pointer, focus, keyboard, and aria disclosure semantics', () => {
    const { onToggle, row } = renderRow();

    expect(row).toHaveAttribute('tabindex', '0');
    expect(row).toHaveAttribute('aria-expanded', 'false');
    expect(row).toHaveAttribute('aria-controls', 'detail-row');

    fireEvent.click(row);
    fireEvent.keyDown(row, { key: 'Enter' });
    fireEvent.keyDown(row, { key: ' ' });
    expect(onToggle).toHaveBeenCalledTimes(3);
  });

  it('does not hijack embedded interactive controls', () => {
    const { onToggle, childAction } = renderRow();

    fireEvent.click(childAction);
    fireEvent.keyDown(childAction, { key: 'Enter' });
    expect(onToggle).not.toHaveBeenCalled();
  });
});
