import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { EmailProviderSelect } from '../EmailProviderSelect';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getEmailProviders: vi.fn().mockResolvedValue([]),
  },
}));

const baseConfig = {
  enabled: true,
  provider: '',
  server: 'smtp.example.com',
  port: 587,
  from: 'pulse@example.com',
  username: '',
  password: '',
  to: [] as string[],
  tls: false,
  startTLS: true,
  replyTo: '',
  maxRetries: 3,
  retryDelay: 60,
  rateLimit: 60,
};

afterEach(() => {
  cleanup();
});

describe('EmailProviderSelect', () => {
  it('keeps a trailing recipient newline editable while saving cleaned recipients', async () => {
    const [config, setConfig] = createSignal(baseConfig);
    const onTest = vi.fn();

    render(() => (
      <EmailProviderSelect
        config={config()}
        onChange={(next) => setConfig(next)}
        onTest={onTest}
      />
    ));

    const recipients = screen.getByLabelText('Recipients (one per line)') as HTMLTextAreaElement;
    recipients.focus();

    await fireEvent.input(recipients, { target: { value: 'admin@example.com\n' } });

    expect(recipients.value).toBe('admin@example.com\n');
    expect(config().to).toEqual(['admin@example.com']);

    await fireEvent.input(recipients, {
      target: { value: 'admin@example.com\nops@example.com' },
    });

    expect(recipients.value).toBe('admin@example.com\nops@example.com');
    expect(config().to).toEqual(['admin@example.com', 'ops@example.com']);
  });
});
