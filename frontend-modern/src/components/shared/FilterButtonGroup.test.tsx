import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Sun from 'lucide-solid/icons/sun';
import { FilterButtonGroup } from './FilterButtonGroup';

describe('FilterButtonGroup', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders the active option as pressed and routes selection changes', () => {
    const onChange = vi.fn();

    render(() => (
      <FilterButtonGroup
        options={[
          { value: 'light', label: 'Light', icon: Sun },
          { value: 'dark', label: 'Dark' },
        ]}
        value="light"
        onChange={onChange}
      />
    ));

    const lightButton = screen.getByRole('button', { name: /light/i });
    const darkButton = screen.getByRole('button', { name: /dark/i });

    expect(lightButton).toHaveAttribute('aria-pressed', 'true');
    expect(darkButton).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(darkButton);
    expect(onChange).toHaveBeenCalledWith('dark');
  });

  it('supports the settings variant without default filter styling', () => {
    render(() => (
      <FilterButtonGroup
        options={[
          { value: 'celsius', label: 'Celsius' },
          { value: 'fahrenheit', label: 'Fahrenheit' },
        ]}
        value="celsius"
        onChange={() => undefined}
        variant="settings"
      />
    ));

    const activeButton = screen.getByRole('button', { name: /celsius/i });
    const inactiveButton = screen.getByRole('button', { name: /fahrenheit/i });

    expect(activeButton.className).toContain('bg-surface');
    expect(activeButton.className).toContain('shadow-sm');
    expect(activeButton.className).not.toContain('text-blue-600');

    expect(inactiveButton.className).toContain('text-muted');
    expect(inactiveButton.className).not.toContain('border-transparent');
  });

  it('supports the prominent variant for full-width segmented controls', () => {
    render(() => (
      <FilterButtonGroup
        options={[
          { value: '24h', label: 'Last 24 Hours' },
          { value: '7d', label: 'Last 7 Days' },
        ]}
        value="24h"
        onChange={() => undefined}
        variant="prominent"
      />
    ));

    const activeButton = screen.getByRole('button', { name: /last 24 hours/i });
    const inactiveButton = screen.getByRole('button', { name: /last 7 days/i });

    expect(activeButton.className).toContain('bg-blue-50');
    expect(activeButton.className).toContain('border-blue-500');
    expect(inactiveButton.className).toContain('border-border');
    expect(inactiveButton.className).not.toContain('text-muted');
  });
});
