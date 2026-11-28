import { MemoryRouter, Route } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertDeliveryPausedCard } from './AlertDeliveryPausedCard';

afterEach(cleanup);

describe('AlertDeliveryPausedCard', () => {
  it('warns that configured destinations receive nothing while delivery is paused', () => {
    const { container } = render(() => (
      <AlertDeliveryPausedCard reason="not_activated" activating={false} onActivate={() => {}} />
    ));

    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.getByText('Notifications are paused')).toBeTruthy();
    expect(screen.getByText(/none of them will be sent to the destinations below/i)).toBeTruthy();
    // A passing test send is exactly what convinces users delivery works.
    expect(screen.getByText(/Test messages bypass the pause/i)).toBeTruthy();
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('activates delivery from the destinations surface', () => {
    const onActivate = vi.fn();
    render(() => (
      <AlertDeliveryPausedCard reason="not_activated" activating={false} onActivate={onActivate} />
    ));

    const button = screen.getByRole('button', { name: 'Turn on delivery' });
    expect(button).toHaveAttribute('aria-busy', 'false');
    expect(button.className).toContain('min-h-11');
    fireEvent.click(button);
    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it('disables and marks the action busy while activation is in flight', () => {
    render(() => (
      <AlertDeliveryPausedCard reason="snoozed" activating={true} onActivate={() => {}} />
    ));

    const button = screen.getByRole('button', { name: 'Turn on delivery' });
    expect(button.hasAttribute('disabled')).toBe(true);
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(button.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('gives the overview setup link a touch-friendly target on narrow viewports', () => {
    render(() => (
      <MemoryRouter>
        <Route
          path="*"
          component={() => (
            <AlertDeliveryPausedCard
              reason="not_activated"
              activating={false}
              onActivate={() => {}}
              surface="overview"
              setupHref="/alerts/notifications"
            />
          )}
        />
      </MemoryRouter>
    ));

    const setupLink = screen.getByRole('link', { name: 'Set up where alerts go' });
    expect(setupLink).toHaveAttribute('href', '/alerts/notifications');
    expect(setupLink.className).toContain('min-h-11');
  });
});
