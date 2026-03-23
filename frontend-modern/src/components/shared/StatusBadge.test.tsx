import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { StatusBadge } from './StatusBadge';
import statusBadgeSource from './StatusBadge.tsx?raw';
import statusBadgeModelSource from './statusBadgeModel.ts?raw';
import statusBadgeStateSource from './useStatusBadgeState.ts?raw';

describe('StatusBadge', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps status badge on shell, runtime, and model owners', () => {
    expect(statusBadgeSource).toContain('useStatusBadgeState');
    expect(statusBadgeSource).toContain('getStatusBadgeClass');
    expect(statusBadgeSource).toContain('getStatusBadgeLabel');
    expect(statusBadgeSource).toContain('getStatusBadgeTitle');
    expect(statusBadgeSource).not.toContain('cursor-not-allowed');
    expect(statusBadgeSource).not.toContain('props.onToggle?.()');
    expect(statusBadgeSource).not.toContain('labelEnabled ??');

    expect(statusBadgeStateSource).toContain('export function useStatusBadgeState');
    expect(statusBadgeStateSource).toContain('Boolean(props.disabled)');
    expect(statusBadgeStateSource).toContain('props.onToggle?.()');
    expect(statusBadgeStateSource).toContain('if (isDisabled())');

    expect(statusBadgeModelSource).toContain('STATUS_BADGE_PADDING_BY_SIZE');
    expect(statusBadgeModelSource).toContain('getStatusBadgeClass');
    expect(statusBadgeModelSource).toContain('getStatusBadgeLabel');
    expect(statusBadgeModelSource).toContain('getStatusBadgeTitle');
    expect(statusBadgeModelSource).toContain("labelEnabled ?? 'Enabled'");
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

    fireEvent.click(screen.getByRole('button', { name: 'Enabled' }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
