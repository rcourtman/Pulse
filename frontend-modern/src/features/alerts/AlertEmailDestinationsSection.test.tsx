import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AlertEmailDestinationsSection } from './AlertEmailDestinationsSection';
import type { UIEmailConfig } from './types';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getEmailProviders: vi.fn().mockResolvedValue([]),
  },
}));

function makeConfig(overrides: Partial<UIEmailConfig> = {}): UIEmailConfig {
  return {
    enabled: false,
    provider: '',
    server: '',
    port: 587,
    username: '',
    password: '',
    from: '',
    to: [],
    tls: false,
    startTLS: true,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 60,
    rateLimit: 10,
    ...overrides,
  };
}

describe('AlertEmailDestinationsSection', () => {
  afterEach(() => {
    cleanup();
  });

  it('associates the visible panel heading with its status toggle', () => {
    render(() => (
      <AlertEmailDestinationsSection
        config={makeConfig()}
        setConfig={vi.fn()}
        setHasUnsavedChanges={vi.fn()}
        onTest={vi.fn()}
        testing={false}
      />
    ));

    expect(screen.getByRole('button', { name: 'Email notifications Disabled' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });
});
