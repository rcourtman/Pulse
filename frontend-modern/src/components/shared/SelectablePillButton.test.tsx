import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SelectablePillButton } from './SelectablePillButton';
import selectablePillButtonSource from './SelectablePillButton.tsx?raw';
import selectablePillModelSource from './selectablePillModel.ts?raw';

describe('SelectablePillButton', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps pressed semantics and class ownership in the shared primitive', () => {
    expect(selectablePillButtonSource).toContain('getSelectablePillButtonClass');
    expect(selectablePillButtonSource).toContain("aria-pressed={local.active ? 'true' : 'false'}");
    expect(selectablePillButtonSource).not.toContain('border-blue-500 bg-blue-600');
    expect(selectablePillButtonSource).not.toContain('hover:border-blue-400');

    expect(selectablePillModelSource).toContain('SELECTABLE_PILL_BUTTON_BASE_CLASS');
    expect(selectablePillModelSource).toContain('SELECTABLE_PILL_BUTTON_ACTIVE_CLASS');
    expect(selectablePillModelSource).toContain('SELECTABLE_PILL_BUTTON_INACTIVE_CLASS');
    expect(selectablePillModelSource).toContain('getSelectablePillButtonClass');
  });

  it('renders active and inactive pill states consistently', () => {
    const onActiveClick = vi.fn();
    const onInactiveClick = vi.fn();

    render(() => (
      <div>
        <SelectablePillButton active onClick={onActiveClick}>
          Active
        </SelectablePillButton>
        <SelectablePillButton active={false} onClick={onInactiveClick}>
          Inactive
        </SelectablePillButton>
      </div>
    ));

    const active = screen.getByRole('button', { name: 'Active' });
    const inactive = screen.getByRole('button', { name: 'Inactive' });

    expect(active).toHaveAttribute('aria-pressed', 'true');
    expect(active.className).toContain('border-blue-500');
    expect(active.className).toContain('bg-blue-600');
    expect(inactive).toHaveAttribute('aria-pressed', 'false');
    expect(inactive.className).toContain('border-border');
    expect(inactive.className).toContain('bg-surface');

    fireEvent.click(active);
    fireEvent.click(inactive);
    expect(onActiveClick).toHaveBeenCalledTimes(1);
    expect(onInactiveClick).toHaveBeenCalledTimes(1);
  });

  it('preserves disabled behavior through the native button contract', () => {
    const onClick = vi.fn();

    render(() => (
      <SelectablePillButton active={false} disabled onClick={onClick}>
        Disabled
      </SelectablePillButton>
    ));

    const disabled = screen.getByRole('button', { name: 'Disabled' });
    expect(disabled).toBeDisabled();
    fireEvent.click(disabled);
    expect(onClick).not.toHaveBeenCalled();
  });
});
