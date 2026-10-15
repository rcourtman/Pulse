import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { StatusBadge } from '../StatusBadge';

afterEach(cleanup);

describe('StatusBadge', () => {
  // The alert thresholds page holds dozens of these; each must say what it
  // switches rather than announcing only "On" or "Off", and still contain its
  // visible word so voice control can name it (WCAG 2.5.3).
  it('names what it switches, then its visible state, and keeps its pressed state', () => {
    const onToggle = vi.fn();
    render(() => <StatusBadge isEnabled onToggle={onToggle} ariaLabel="CPU % default alerts" />);

    const badge = screen.getByRole('button', { name: 'CPU % default alerts: Enabled' });
    expect(badge).toHaveAttribute('aria-pressed', 'true');
    expect(badge).toHaveTextContent('Enabled');
    fireEvent.click(badge);
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('falls back to its visible label when no ariaLabel is given', () => {
    render(() => <StatusBadge isEnabled={false} />);

    expect(screen.getByRole('button', { name: 'Disabled' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });
});
