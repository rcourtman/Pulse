import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ColumnPicker } from './ColumnPicker';
import columnPickerSource from './ColumnPicker.tsx?raw';
import columnPickerModelSource from './columnPickerModel.ts?raw';
import columnPickerStateSource from './useColumnPickerState.ts?raw';

describe('ColumnPicker', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps column picker on shell, runtime, and model owners', () => {
    expect(columnPickerSource).toContain('useColumnPickerState');
    expect(columnPickerSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerSource).toContain('widthClass="w-56 max-w-[calc(100vw-2rem)]"');
    expect(columnPickerSource).not.toContain('createSignal');
    expect(columnPickerSource).not.toContain('createEffect');
    expect(columnPickerSource).not.toContain('document.addEventListener');
    expect(columnPickerSource).not.toContain('getHiddenColumnCount');

    expect(columnPickerStateSource).toContain('export function useColumnPickerState');
    expect(columnPickerStateSource).toContain('createSignal');
    expect(columnPickerStateSource).toContain('createEffect');
    expect(columnPickerStateSource).toContain('document.addEventListener');
    expect(columnPickerStateSource).toContain('handleClickOutside');
    expect(columnPickerStateSource).toContain('hiddenCount');

    expect(columnPickerModelSource).toContain('COLUMN_PICKER_BUTTON_LABEL');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerModelSource).toContain('getHiddenColumnCount');
    expect(columnPickerModelSource).toContain('shouldShowColumnPickerReset');
    expect(columnPickerModelSource).toContain('getColumnPickerOptionTextClass');
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
    expect(screen.getByText('Enabled columns appear when table space allows.')).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Subject'));
    expect(onToggle).toHaveBeenCalledWith('subject');
  });

  it('expands inline when embedded in a scrolling preferences panel', async () => {
    render(() => (
      <ColumnPicker
        inline
        columns={[{ id: 'memory', label: 'Memory' }]}
        isHidden={() => false}
        onToggle={vi.fn()}
      />
    ));

    const button = screen.getByRole('button', { name: /columns/i });
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('Show Columns')).not.toBeInTheDocument();

    fireEvent.click(button);

    expect(button).toHaveAttribute('aria-expanded', 'true');
    expect(await screen.findByText('Show Columns')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory').parentElement?.parentElement).toHaveClass(
      'column-picker-inline-options',
    );
  });

  it('labels hidden column count with explicit context', () => {
    render(() => (
      <ColumnPicker
        columns={[
          { id: 'subject', label: 'Subject' },
          { id: 'verified', label: 'Verified' },
          { id: 'target', label: 'Target' },
        ]}
        isHidden={(columnId) => columnId !== 'subject'}
        onToggle={vi.fn()}
      />
    ));

    expect(screen.getByRole('button', { name: /columns 2 hidden/i })).toBeInTheDocument();
    expect(screen.getByText('2 hidden')).toBeInTheDocument();
  });

  it('keeps reset available for explicit overrides even when no column is hidden', async () => {
    const onReset = vi.fn();
    render(() => (
      <ColumnPicker
        columns={[{ id: 'subject', label: 'Subject' }]}
        isHidden={() => false}
        onToggle={vi.fn()}
        onReset={onReset}
        showReset
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /columns/i }));
    fireEvent.click(await screen.findByRole('button', { name: 'Reset' }));

    expect(onReset).toHaveBeenCalledOnce();
  });

  it('explains and clears an active manual-width layout independently of column visibility', async () => {
    const onResetWidths = vi.fn();
    render(() => (
      <ColumnPicker
        columns={[{ id: 'subject', label: 'Subject' }]}
        isHidden={() => false}
        onToggle={vi.fn()}
        onResetWidths={onResetWidths}
        hasManualWidths
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /columns/i }));

    expect(
      await screen.findByText(
        'Custom widths are active, so every enabled column stays visible and the table scrolls sideways.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText('Enabled columns appear when table space allows.'),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Reset widths' }));
    expect(onResetWidths).toHaveBeenCalledOnce();
  });

  it('closes when the user clicks outside the open picker', async () => {
    render(() => (
      <ColumnPicker
        columns={[{ id: 'subject', label: 'Subject' }]}
        isHidden={() => false}
        onToggle={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /columns/i }));
    expect(await screen.findByText('Show Columns')).toBeInTheDocument();

    fireEvent.mouseDown(document.body);
    await waitFor(() => {
      expect(screen.queryByText('Show Columns')).not.toBeInTheDocument();
    });
  });
});
