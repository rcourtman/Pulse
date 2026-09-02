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

  it('keeps whole-row activation as a pointer convenience only', () => {
    const { onToggle, row } = renderRow();

    expect(row).not.toHaveAttribute('tabindex');
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row).not.toHaveAttribute('aria-controls');

    fireEvent.click(row);
    fireEvent.keyDown(row, { key: 'Enter' });
    fireEvent.keyDown(row, { key: ' ' });
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('does not hijack embedded interactive controls', () => {
    const { onToggle, childAction } = renderRow();

    fireEvent.click(childAction);
    fireEvent.keyDown(childAction, { key: 'Enter' });
    expect(onToggle).not.toHaveBeenCalled();
  });
});
