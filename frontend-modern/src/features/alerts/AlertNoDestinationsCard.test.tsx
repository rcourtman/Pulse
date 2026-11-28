import { MemoryRouter, Route } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertNoDestinationsCard } from './AlertNoDestinationsCard';

afterEach(cleanup);

describe('AlertNoDestinationsCard', () => {
  it('announces missing destinations with touch-friendly manage and dismiss actions', () => {
    const onDismiss = vi.fn();
    const { container } = render(() => (
      <MemoryRouter>
        <Route
          path="*"
          component={() => (
            <AlertNoDestinationsCard setupHref="/alerts/notifications" onDismiss={onDismiss} />
          )}
        />
      </MemoryRouter>
    ));

    const status = screen.getByRole('status');
    expect(status).toHaveTextContent('No notification destinations enabled');
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');

    const manageLink = screen.getByRole('link', { name: 'Manage destinations' });
    expect(manageLink).toHaveAttribute('href', '/alerts/notifications');
    expect(manageLink.className).toContain('min-h-11');

    const dismissButton = screen.getByRole('button', { name: 'Dismiss' });
    expect(dismissButton).toHaveAttribute('type', 'button');
    expect(dismissButton.className).toContain('min-h-11');

    fireEvent.click(dismissButton);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});
