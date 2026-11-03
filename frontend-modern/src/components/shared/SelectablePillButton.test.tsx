import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SelectablePillButton } from './SelectablePillButton';

describe('SelectablePillButton', () => {
  afterEach(() => {
    cleanup();
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
    expect(active.className).toContain('max-w-full');
    expect(active.className).toContain('whitespace-normal');
    expect(active.className).not.toContain('whitespace-nowrap');
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
