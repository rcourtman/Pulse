import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertDestinationsLoadErrorCard } from './AlertDestinationsLoadErrorCard';

afterEach(cleanup);

describe('AlertDestinationsLoadErrorCard', () => {
  it('announces destination load errors as an alert with an explicit retry button', () => {
    const onRetry = vi.fn();
    const { container } = render(() => (
      <AlertDestinationsLoadErrorCard
        error="HTTP 503 Service Unavailable"
        isRetrying={false}
        onRetry={onRetry}
      />
    ));

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent(/HTTP 503 Service Unavailable/);
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');

    const retry = screen.getByRole('button', { name: 'Retry' });
    expect(retry).toHaveAttribute('type', 'button');
    expect(retry).toHaveAttribute('aria-busy', 'false');
    expect(retry).not.toBeDisabled();
    expect(retry.className).toContain('min-h-11');

    fireEvent.click(retry);
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('marks the retry action busy and disabled while retrying', () => {
    render(() => (
      <AlertDestinationsLoadErrorCard
        error="Connection refused"
        isRetrying={true}
        onRetry={vi.fn()}
      />
    ));

    const retry = screen.getByRole('button', { name: 'Retrying…' });
    expect(retry).toBeDisabled();
    expect(retry).toHaveAttribute('aria-busy', 'true');
    expect(retry.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});
