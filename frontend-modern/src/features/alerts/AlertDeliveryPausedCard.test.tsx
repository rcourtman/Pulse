import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertDeliveryPausedCard } from './AlertDeliveryPausedCard';

afterEach(cleanup);

describe('AlertDeliveryPausedCard', () => {
  it('warns that configured destinations receive nothing while delivery is paused', () => {
    render(() => (
      <AlertDeliveryPausedCard reason="not_activated" activating={false} onActivate={() => {}} />
    ));

    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.getByText('Notifications are paused')).toBeTruthy();
    expect(screen.getByText(/none of them will be sent to the destinations below/i)).toBeTruthy();
    // A passing test send is exactly what convinces users delivery works.
    expect(screen.getByText(/Test messages bypass the pause/i)).toBeTruthy();
  });

  it('activates delivery from the destinations surface', () => {
    const onActivate = vi.fn();
    render(() => (
      <AlertDeliveryPausedCard reason="not_activated" activating={false} onActivate={onActivate} />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Turn on delivery' }));
    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it('disables the action while activation is in flight', () => {
    render(() => (
      <AlertDeliveryPausedCard reason="snoozed" activating={true} onActivate={() => {}} />
    ));

    expect(screen.getByRole('button', { name: 'Turn on delivery' }).hasAttribute('disabled')).toBe(
      true,
    );
  });
});
