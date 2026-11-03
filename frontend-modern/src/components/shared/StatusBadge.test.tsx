import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { StatusBadge } from './StatusBadge';

describe('StatusBadge', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders label/title policy and blocks toggles when disabled', () => {
    const onToggle = vi.fn();

    render(() => (
      <StatusBadge
        isEnabled={false}
        disabled={true}
        onToggle={onToggle}
        titleEnabled="On"
        titleDisabled="Off"
        titleWhenDisabled="Locked"
      />
    ));

    const button = screen.getByRole('button', { name: 'Disabled' });
    expect(button).toHaveAttribute('title', 'Locked');
    fireEvent.click(button);
    expect(onToggle).not.toHaveBeenCalled();
  });

  it('allows toggles when enabled and not disabled', () => {
    const onToggle = vi.fn();

    render(() => <StatusBadge isEnabled={true} onToggle={onToggle} />);

    const button = screen.getByRole('button', { name: 'Enabled' });
    expect(button).toHaveClass('min-h-11', 'sm:min-h-0');
    fireEvent.click(button);
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
