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
    fireEvent.click(screen.getByLabelText('Subject'));
    expect(onToggle).toHaveBeenCalledWith('subject');
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
