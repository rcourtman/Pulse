import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

// --- Mock setup (must be before component import) ---
const getEmailProvidersMock = vi.fn();

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getEmailProviders: (...args: unknown[]) => getEmailProvidersMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
  },
}));

import { EmailProviderSelect } from './EmailProviderSelect';
import { logger } from '@/utils/logger';

// --- Helpers ---
interface EmailConfig {
  enabled: boolean;
  provider: string;
  server: string;
  port: number;
  from: string;
  username: string;
  password: string;
  to: string[];
  tls: boolean;
  startTLS: boolean;
  replyTo: string;
  maxRetries: number;
  retryDelay: number;
  rateLimit: number;
}

function makeConfig(overrides: Partial<EmailConfig> = {}): EmailConfig {
  return {
    enabled: true,
    provider: '',
    server: '',
    port: 587,
    from: 'noreply@example.com',
    username: '',
    password: '',
    to: [],
    tls: false,
    startTLS: true,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 5,
    rateLimit: 60,
    ...overrides,
  };
}

const gmailProvider = {
  name: 'Gmail',
  smtpHost: 'smtp.gmail.com',
  smtpPort: 587,
  tls: false,
  startTLS: true,
  authRequired: true,
  instructions: 'Use an App Password from your Google Account settings.',
};

const sendgridProvider = {
  name: 'SendGrid',
  smtpHost: 'smtp.sendgrid.net',
  smtpPort: 587,
  tls: false,
  startTLS: true,
  authRequired: true,
  instructions: 'Create an API key in the SendGrid dashboard.',
};

const outlookProvider = {
  name: 'Outlook',
  smtpHost: 'smtp.office365.com',
  smtpPort: 587,
  tls: false,
  startTLS: true,
  authRequired: true,
  instructions: 'Use your Outlook email and password.',
};

const mockProviders = [gmailProvider, sendgridProvider, outlookProvider];

// --- Tests ---
describe('EmailProviderSelect', () => {
  let onChangeMock: ReturnType<typeof vi.fn>;
  let onTestMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onChangeMock = vi.fn();
    onTestMock = vi.fn();
    getEmailProvidersMock.mockReset();
    getEmailProvidersMock.mockResolvedValue(mockProviders);
    vi.mocked(logger.debug).mockReset();
    vi.mocked(logger.error).mockReset();
    vi.mocked(logger.warn).mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  // --- Rendering & Provider Loading ---

  it('renders the provider select and form fields', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(getEmailProvidersMock).toHaveBeenCalled();
    });

    expect(screen.getByText('Email provider')).toBeInTheDocument();
    expect(screen.getByText('SMTP server')).toBeInTheDocument();
    expect(screen.getByText('SMTP port')).toBeInTheDocument();
    expect(screen.getByText('From address')).toBeInTheDocument();
    expect(screen.getByText('Username')).toBeInTheDocument();
    expect(screen.getByText('Password / API key')).toBeInTheDocument();
    expect(screen.getByText('Recipients (one per line)')).toBeInTheDocument();
  });

  it('loads providers from the API and populates the select', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    expect(screen.getByText('SendGrid (smtp.sendgrid.net:587)')).toBeInTheDocument();
    expect(screen.getByText('Outlook (smtp.office365.com:587)')).toBeInTheDocument();
    expect(screen.getByText('Manual configuration')).toBeInTheDocument();
  });

  it('handles API error loading providers gracefully', async () => {
    getEmailProvidersMock.mockRejectedValue(new Error('Network error'));
    const { logger } = await import('@/utils/logger');

    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(logger.error).toHaveBeenCalledWith('Failed to load email providers', expect.any(Error));
    });

    // Should still have the manual option
    expect(screen.getByText('Manual configuration')).toBeInTheDocument();
  });

  // --- Provider Selection ---

  it('applies provider settings when selecting a provider', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    const select = screen.getByRole('combobox') as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'Gmail' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'Gmail',
        server: 'smtp.gmail.com',
        port: 587,
        tls: false,
        startTLS: true,
      })
    );
  });

  it('sets username to "apikey" when selecting SendGrid', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ username: 'myuser' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('SendGrid (smtp.sendgrid.net:587)')).toBeInTheDocument();
    });

    const select = screen.getByRole('combobox') as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'SendGrid' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'SendGrid',
        username: 'apikey',
      })
    );
  });

  it('preserves existing username when selecting non-SendGrid provider', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ username: 'myuser@example.com' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    const select = screen.getByRole('combobox') as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'Gmail' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'Gmail',
        username: 'myuser@example.com',
      })
    );
  });

  it('clears provider when selecting "Manual configuration"', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: 'Gmail', server: 'smtp.gmail.com' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    const select = screen.getByRole('combobox') as HTMLSelectElement;
    fireEvent.change(select, { target: { value: '' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: '',
      })
    );
  });

  // --- Form Input Fields ---

  it('updates SMTP server on input', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const serverInput = screen.getByPlaceholderText('smtp.example.com');
    fireEvent.input(serverInput, { target: { value: 'mail.test.com' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ server: 'mail.test.com' })
    );
  });

  it('updates port on input and defaults to 587 on blur if empty', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ port: 465 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const portInput = screen.getByPlaceholderText('587');

    // Type a valid port
    fireEvent.input(portInput, { target: { value: '465' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ port: 465 })
    );

    // Clear the port and blur — should default to 587
    onChangeMock.mockClear();
    fireEvent.input(portInput, { target: { value: '' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ port: 0 })
    );

    onChangeMock.mockClear();
    fireEvent.blur(portInput);
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ port: 587 })
    );
  });

  it('updates from address on input', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const fromInput = screen.getByPlaceholderText('noreply@example.com');
    fireEvent.input(fromInput, { target: { value: 'alerts@myco.com' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ from: 'alerts@myco.com' })
    );
  });

  it('updates reply-to on input', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const replyToInput = screen.getByPlaceholderText('admin@example.com');
    fireEvent.input(replyToInput, { target: { value: 'help@myco.com' } });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ replyTo: 'help@myco.com' })
    );
  });

  it('updates username on input', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    // Username placeholder depends on provider — when no provider, it's generic
    const usernameInputs = screen.getAllByRole('textbox');
    // Find the one after "Username" label
    const usernameInput = usernameInputs.find(
      (el) => (el as HTMLInputElement).placeholder === 'username@example.com'
    );
    expect(usernameInput).toBeTruthy();

    fireEvent.input(usernameInput!, { target: { value: 'me@test.com' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ username: 'me@test.com' })
    );
  });

  it('parses recipients textarea into array', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ to: [] })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
    expect(textarea).toBeTruthy();
    fireEvent.input(textarea, {
      target: { value: 'alice@test.com\nbob@test.com\n\n  charlie@test.com  ' },
    });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        to: ['alice@test.com', 'bob@test.com', 'charlie@test.com'],
      })
    );
  });

  it('filters empty lines from recipients', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ to: [] })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
    expect(textarea).toBeTruthy();
    fireEvent.input(textarea, {
      target: { value: '\n\n  \n' },
    });

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ to: [] })
    );
  });

  // --- Advanced Options ---

  it('toggles advanced options section', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    // Advanced options hidden by default
    expect(screen.queryByText('Security')).not.toBeInTheDocument();

    // Click to show
    fireEvent.click(screen.getByText('Show advanced options'));
    expect(screen.getByText('Security')).toBeInTheDocument();
    expect(screen.getByText('Rate limit')).toBeInTheDocument();
    expect(screen.getByText('Max retries')).toBeInTheDocument();
    expect(screen.getByText('Retry delay (seconds)')).toBeInTheDocument();
    expect(screen.getByText('Hide advanced options')).toBeInTheDocument();

    // Click to hide
    fireEvent.click(screen.getByText('Hide advanced options'));
    expect(screen.queryByText('Security')).not.toBeInTheDocument();
  });

  it('changes security mode via advanced options', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ tls: false, startTLS: true })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Show advanced options'));

    // Find the security select (there are two comboboxes now — provider and security)
    const selects = screen.getAllByRole('combobox');
    const securitySelect = selects.find((el) => {
      const options = (el as HTMLSelectElement).options;
      return Array.from(options).some((opt) => opt.value === 'tls');
    }) as HTMLSelectElement;
    expect(securitySelect).toBeTruthy();

    // Change to TLS
    fireEvent.change(securitySelect!, { target: { value: 'tls' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ tls: true, startTLS: false })
    );

    // Change to none
    onChangeMock.mockClear();
    fireEvent.change(securitySelect!, { target: { value: 'none' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ tls: false, startTLS: false })
    );
  });

  it('updates rate limit via advanced options', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ rateLimit: 60 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Show advanced options'));

    // Rate limit input — find by current value
    const numberInputs = screen.getAllByRole('spinbutton');
    const rateLimitInput = numberInputs.find(
      (el) => (el as HTMLInputElement).value === '60'
    );
    expect(rateLimitInput).toBeTruthy();

    fireEvent.input(rateLimitInput!, { target: { value: '120' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ rateLimit: 120 })
    );
  });

  it('updates max retries via advanced options', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ maxRetries: 3 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Show advanced options'));

    const numberInputs = screen.getAllByRole('spinbutton');
    const retriesInput = numberInputs.find(
      (el) => (el as HTMLInputElement).value === '3'
    );
    expect(retriesInput).toBeTruthy();

    fireEvent.input(retriesInput!, { target: { value: '5' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ maxRetries: 5 })
    );
  });

  it('updates retry delay via advanced options', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ retryDelay: 5 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Show advanced options'));

    const numberInputs = screen.getAllByRole('spinbutton');
    const delayInput = numberInputs.find(
      (el) => (el as HTMLInputElement).value === '5'
    );
    expect(delayInput).toBeTruthy();

    fireEvent.input(delayInput!, { target: { value: '15' } });
    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({ retryDelay: 15 })
    );
  });

  it('coerces maxRetries=0 to default display value of 3 (known || coercion)', async () => {
    // NOTE: The source uses `value={props.config.maxRetries || 3}` which
    // coerces 0 to 3. This test documents the current behavior.
    // maxRetries has min={0} so 0 is semantically valid but displays as 3.
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ maxRetries: 0 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Show advanced options'));

    const numberInputs = screen.getAllByRole('spinbutton');
    // With maxRetries=0, the input shows "3" due to || coercion
    const retriesInput = numberInputs.find(
      (el) => (el as HTMLInputElement).value === '3'
    );
    expect(retriesInput).toBeTruthy();
  });

  it('coerces rateLimit=0 to default display value of 60 (known || coercion)', async () => {
    // NOTE: Same || coercion pattern as maxRetries.
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ rateLimit: 0 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Show advanced options'));

    const numberInputs = screen.getAllByRole('spinbutton');
    const rateLimitInput = numberInputs.find(
      (el) => (el as HTMLInputElement).value === '60'
    );
    expect(rateLimitInput).toBeTruthy();
  });

  it('stops Enter key propagation in recipients textarea', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ to: [] })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
    expect(textarea).toBeTruthy();

    const event = new KeyboardEvent('keydown', {
      key: 'Enter',
      bubbles: true,
      cancelable: true,
    });
    const stopPropagationSpy = vi.spyOn(event, 'stopPropagation');
    textarea.dispatchEvent(event);

    expect(stopPropagationSpy).toHaveBeenCalled();
  });

  it('does not stop propagation for non-Enter keys in recipients textarea', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ to: [] })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
    expect(textarea).toBeTruthy();

    const event = new KeyboardEvent('keydown', {
      key: 'Tab',
      bubbles: true,
      cancelable: true,
    });
    const stopPropagationSpy = vi.spyOn(event, 'stopPropagation');
    textarea.dispatchEvent(event);

    expect(stopPropagationSpy).not.toHaveBeenCalled();
  });

  // --- Mobile Instructions Toggle ---

  it('toggles instructions on mobile view', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: 'Gmail' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    // Mobile toggle starts with showInstructions=false (signal default)
    const toggleBtn = screen.getByText('Show setup instructions');
    expect(toggleBtn).toBeInTheDocument();

    // The sm:hidden container exists but no instruction content in mobile section yet
    const mobileContainer = toggleBtn.closest('.sm\\:hidden') as HTMLElement;
    expect(mobileContainer).toBeTruthy();

    // Click to show — instruction text appears in the mobile section
    fireEvent.click(toggleBtn);
    expect(screen.getByText('Hide setup instructions')).toBeInTheDocument();
    const instructionTexts = screen.getAllByText(
      'Use an App Password from your Google Account settings.'
    );
    // Should have at least 2: one in mobile section, one in desktop section
    expect(instructionTexts.length).toBeGreaterThanOrEqual(2);

    // Click to hide — mobile instruction text should disappear
    fireEvent.click(screen.getByText('Hide setup instructions'));
    expect(screen.getByText('Show setup instructions')).toBeInTheDocument();
    // Desktop always shows instructions; mobile hides them, so count drops
    const afterHide = screen.getAllByText(
      'Use an App Password from your Google Account settings.'
    );
    expect(afterHide.length).toBeLessThan(instructionTexts.length);
  });

  // --- Test Email Button ---

  it('renders "Send test email" button', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const button = screen.getByRole('button', { name: 'Send test email' });
    expect(button).toBeInTheDocument();
    expect(button).not.toBeDisabled();
  });

  it('calls onTest when test button is clicked', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Send test email' }));
    expect(onTestMock).toHaveBeenCalledOnce();
  });

  it('disables test button when testing is true', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig()}
        onChange={onChangeMock}
        onTest={onTestMock}
        testing={true}
      />
    ));

    const button = screen.getByRole('button', { name: 'Sending test email…' });
    expect(button).toBeDisabled();
  });

  it('disables test button when email is not enabled', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ enabled: false })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const button = screen.getByRole('button', { name: 'Send test email' });
    expect(button).toBeDisabled();
  });

  // --- Reapply Defaults Button ---

  it('shows "Reapply defaults" when a provider is selected', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: 'Gmail' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    expect(screen.getByText('Reapply defaults')).toBeInTheDocument();
  });

  it('does not show "Reapply defaults" when no provider is selected', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: '' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.queryByText('Reapply defaults')).not.toBeInTheDocument();
  });

  it('reapplies provider defaults when clicking "Reapply defaults"', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: 'Gmail', server: 'custom.server.com', port: 25 })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Reapply defaults'));

    expect(onChangeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'Gmail',
        server: 'smtp.gmail.com',
        port: 587,
        tls: false,
        startTLS: true,
      })
    );
  });

  // --- Instructions Display ---

  it('shows provider instructions when a provider is selected', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: 'Gmail' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Gmail (smtp.gmail.com:587)')).toBeInTheDocument();
    });

    // Instructions appear in the desktop (sm:block) view
    expect(
      screen.getByText('Use an App Password from your Google Account settings.')
    ).toBeInTheDocument();
  });

  it('does not show instructions when no provider is selected', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: '' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(getEmailProvidersMock).toHaveBeenCalled();
    });

    expect(
      screen.queryByText('Use an App Password from your Google Account settings.')
    ).not.toBeInTheDocument();
  });

  // --- SendGrid Username Placeholder ---

  it('shows "apikey" placeholder for username when SendGrid is selected', async () => {
    render(() => (
      <EmailProviderSelect
        config={makeConfig({ provider: 'SendGrid' })}
        onChange={onChangeMock}
        onTest={onTestMock}
      />
    ));

    const usernameInputs = screen.getAllByRole('textbox');
    const usernameInput = usernameInputs.find(
      (el) => (el as HTMLInputElement).placeholder === 'apikey'
    );
    expect(usernameInput).toBeTruthy();
  });
});
