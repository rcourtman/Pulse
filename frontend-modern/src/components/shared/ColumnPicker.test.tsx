import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ColumnPicker } from './ColumnPicker';

describe('ColumnPicker', () => {
  afterEach(() => {
    cleanup();
  });

  it('uses the canonical columns label and modal copy', async () => {
    const onToggle = vi.fn();
    const onReset = vi.fn();

    render(() => (
      <ColumnPicker
        columns={[{ id: 'subject', label: 'Subject' }]}
        isHidden={() => false}
        onToggle={onToggle}
        onReset={onReset}
      />
    ));

    const button = screen.getByRole('button', { name: /columns/i });
    expect(button).toBeInTheDocument();
    expect(screen.queryByText('Display')).not.toBeInTheDocument();

    fireEvent.click(button);

    expect(await screen.findByText('Show Columns')).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Subject'));
    expect(onToggle).toHaveBeenCalledWith('subject');
  });
});
