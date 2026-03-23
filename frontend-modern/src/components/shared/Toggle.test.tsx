import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Toggle, TogglePrimitive } from './Toggle';
import toggleSource from './Toggle.tsx?raw';
import toggleModelSource from './toggleModel.ts?raw';
import toggleStateSource from './useToggleState.ts?raw';

describe('Toggle', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps toggle on shell, runtime, and model owners', () => {
    expect(toggleSource).toContain('useToggleState');
    expect(toggleSource).toContain('getToggleTrackClass');
    expect(toggleSource).toContain('getToggleKnobClass');
    expect(toggleSource).not.toContain('defaultPrevented');
    expect(toggleSource).not.toContain('toggleSizeConfig');
    expect(toggleSource).not.toContain('handleClick =');

    expect(toggleStateSource).toContain('export function useToggleState');
    expect(toggleStateSource).toContain('defaultPrevented');
    expect(toggleStateSource).toContain('currentTarget: { checked: next }');
    expect(toggleStateSource).toContain('props.onChange?.(event)');
    expect(toggleStateSource).toContain('props.onToggle?.()');

    expect(toggleModelSource).toContain('toggleSizeConfig');
    expect(toggleModelSource).toContain('resolveToggleSize');
    expect(toggleModelSource).toContain('getToggleTrackClass');
    expect(toggleModelSource).toContain('getToggleKnobClass');
    expect(toggleModelSource).toContain('ToggleChangeEvent');
  });

  it('emits the synthetic next checked state and respects preventDefault', () => {
    const onToggle = vi.fn();
    const onChange = vi.fn((event) => event.preventDefault());

    render(() => <TogglePrimitive checked={false} onToggle={onToggle} onChange={onChange} />);

    fireEvent.click(screen.getByRole('button'));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange.mock.calls[0][0].currentTarget.checked).toBe(true);
    expect(onToggle).not.toHaveBeenCalled();
  });

  it('renders label and description through the shell contract', () => {
    render(() => (
      <Toggle checked={true} label={<span>Enabled</span>} description={<span>Turns it on</span>} />
    ));

    expect(screen.getByText('Enabled')).toBeInTheDocument();
    expect(screen.getByText('Turns it on')).toBeInTheDocument();
  });
});
