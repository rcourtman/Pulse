import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EmailProviderSelect } from './EmailProviderSelect';
import { createDefaultEmailConfig, normalizeEmailConfigFromAPI } from '@/features/alerts/helpers';
import { buildEmailConfigPayload } from '@/features/alerts/alertDestinationsModel';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: { getEmailProviders: vi.fn().mockResolvedValue([]) },
}));

afterEach(cleanup);

describe('email form settings must be backed by the public configuration', () => {
  it.each(['Reply-to address', 'Max retries', 'Retry delay (seconds)'])(
    'does not offer the ineffective %s control',
    (name) => {
      render(() => (
        <EmailProviderSelect
          config={createDefaultEmailConfig()}
          onChange={vi.fn()}
          onTest={vi.fn()}
        />
      ));
      expect(screen.queryByLabelText(name)).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Show advanced options' }));
      expect(screen.queryByLabelText(name)).not.toBeInTheDocument();
    },
  );

  it('keeps defaults limited to settings the payload can persist', () => {
    const config = createDefaultEmailConfig();
    expect(config).toEqual(buildEmailConfigPayload(config));
  });

  it.each([true, false])(
    'preserves supported settings through an edit and reload, enabled=%s',
    (enabled) => {
      const saved = normalizeEmailConfigFromAPI({
        enabled,
        provider: 'Custom SMTP Server',
        server: 'smtp.example.test',
        port: 587,
        username: 'operator@example.test',
        password: 'synthetic-only',
        from: 'pulse@example.test',
        to: ['ops@example.test'],
        tls: false,
        startTLS: true,
        rateLimit: 17,
        tagFilter: ['production'],
        tagFilterMode: 'any',
        minimumSeverity: 'warning',
      });
      const [config, setConfig] = createSignal(saved);
      render(() => <EmailProviderSelect config={config()} onChange={setConfig} onTest={vi.fn()} />);
      fireEvent.input(screen.getByRole('textbox', { name: 'From address' }), {
        target: { value: 'changed@example.test' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'Show advanced options' }));
      fireEvent.input(screen.getByRole('spinbutton', { name: 'Rate limit' }), {
        target: { value: '12' },
      });
      const payload = buildEmailConfigPayload(config());
      expect(payload).toEqual({
        ...buildEmailConfigPayload(saved),
        from: 'changed@example.test',
        rateLimit: 12,
      });
      expect(normalizeEmailConfigFromAPI(payload)).toEqual(config());
      expect(screen.getByRole('button', { name: 'Send test email' })).toHaveProperty(
        'disabled',
        !enabled,
      );
    },
  );
});
