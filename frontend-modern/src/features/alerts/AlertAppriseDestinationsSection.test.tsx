import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { AlertAppriseDestinationsSection } from './AlertAppriseDestinationsSection';
import type { UIAppriseConfig } from './types';

function makeConfig(overrides: Partial<UIAppriseConfig> = {}): UIAppriseConfig {
  return {
    enabled: true,
    mode: 'cli',
    targetsText: '',
    cliPath: 'apprise',
    timeoutSeconds: 15,
    serverUrl: '',
    configKey: 'default',
    apiKey: '',
    apiKeyHeader: 'X-API-KEY',
    skipTlsVerify: false,
    hasApiKey: false,
    ...overrides,
  };
}

function renderSection(config: UIAppriseConfig) {
  return render(() => (
    <AlertAppriseDestinationsSection
      config={config}
      updateApprise={vi.fn()}
      setHasUnsavedChanges={vi.fn()}
      onTest={vi.fn()}
      testing={false}
    />
  ));
}

describe('AlertAppriseDestinationsSection', () => {
  afterEach(() => {
    cleanup();
  });

  it('associates visible labels with CLI mode fields', () => {
    renderSection(makeConfig({ mode: 'cli' }));

    expect(screen.getByRole('button', { name: 'Apprise notifications Enabled' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('combobox', { name: 'Delivery mode' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Delivery targets' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'CLI path' })).toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: 'Timeout (seconds)' })).toBeInTheDocument();
    expect(
      screen.getByRole('option', { name: 'Warnings and critical alerts' }),
    ).toBeInTheDocument();
  });

  it('associates visible labels with HTTP mode fields', () => {
    renderSection(makeConfig({ mode: 'http' }));

    expect(screen.getByRole('textbox', { name: 'Server URL' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Config key (optional)' })).toBeInTheDocument();
    expect(screen.getByLabelText('API key')).toHaveAttribute('type', 'password');
    expect(screen.getByRole('textbox', { name: 'API key header' })).toBeInTheDocument();
    expect(
      screen.getByRole('checkbox', { name: 'Skip TLS certificate verification (unsafe)' }),
    ).toBeInTheDocument();
  });

  // The API never returns the stored key, so the field renders empty; the
  // placeholder is the only signal that a key is already saved.
  it('tells the user a saved API key is kept when the field is left blank', () => {
    renderSection(makeConfig({ mode: 'http', apiKey: '', hasApiKey: true }));

    expect(screen.getByLabelText('API key')).toHaveAttribute(
      'placeholder',
      'Saved. Leave blank to keep the current key',
    );
  });

  it('offers the optional-key placeholder when no key is saved', () => {
    renderSection(makeConfig({ mode: 'http', apiKey: '', hasApiKey: false }));

    expect(screen.getByLabelText('API key')).toHaveAttribute('placeholder', 'Optional API key');
  });

  it.each([false, true])(
    'explains plain HTTP independently of skipTlsVerify=%s',
    (skipTlsVerify) => {
      renderSection(
        makeConfig({ mode: 'http', serverUrl: ' http://apprise.example.invalid ', skipTlsVerify }),
      );
      const warning = screen.getByRole('status', { name: 'Apprise API transport warning' });
      expect(warning).toHaveTextContent('HTTP does not use TLS');
      expect(warning).toHaveTextContent('API keys, destination URLs and alert content');
      expect(warning).toHaveTextContent('checkbox has no effect on HTTP');
      expect(warning).not.toHaveTextContent('TLS verification disabled');
      expect(screen.getByRole('textbox', { name: 'Server URL' })).toHaveAccessibleDescription(
        /HTTP does not use TLS/,
      );
    },
  );

  it('explains the full HTTPS bypass and associates it with the checkbox', () => {
    renderSection(
      makeConfig({
        mode: 'http',
        serverUrl: 'HTTPS://apprise.example.invalid',
        skipTlsVerify: true,
      }),
    );
    const warning = screen.getByRole('status', { name: 'Apprise API transport warning' });
    expect(warning).toHaveTextContent('Certificate and hostname checks are disabled');
    expect(warning).toHaveTextContent('impersonated server');
    expect(screen.getByRole('checkbox')).toHaveAccessibleDescription(/not just.*self-signed/);
  });

  it.each(['', 'not a URL', 'https://apprise.example.invalid'])(
    'does not invent a transport failure for %s',
    (serverUrl) => {
      renderSection(makeConfig({ mode: 'http', serverUrl, skipTlsVerify: false }));
      expect(
        screen.queryByRole('status', { name: 'Apprise API transport warning' }),
      ).not.toBeInTheDocument();
    },
  );

  it('updates the explanation without changing credentials, settings or sending a test', () => {
    const initial = makeConfig({
      mode: 'http',
      serverUrl: 'https://apprise.example.invalid',
      hasApiKey: true,
      targetsText: 'fixture://target',
    });
    const [config, setConfig] = createSignal(initial);
    const updates = vi.fn((partial: Partial<UIAppriseConfig>) =>
      setConfig((prior) => ({ ...prior, ...partial })),
    );
    const dirty = vi.fn();
    const test = vi.fn();
    render(() => (
      <AlertAppriseDestinationsSection
        config={config()}
        updateApprise={updates}
        setHasUnsavedChanges={dirty}
        onTest={test}
        testing={false}
      />
    ));
    const checkbox = screen.getByRole('checkbox');
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
    expect(screen.getByRole('status', { name: 'Apprise API transport warning' })).toHaveTextContent(
      'impersonated server',
    );
    fireEvent.input(screen.getByRole('textbox', { name: 'Server URL' }), {
      target: { value: 'http://apprise.example.invalid' },
    });
    expect(screen.getByRole('status', { name: 'Apprise API transport warning' })).toHaveTextContent(
      'HTTP does not use TLS',
    );
    fireEvent.input(screen.getByRole('combobox', { name: 'Delivery mode' }), {
      target: { value: 'cli' },
    });
    expect(
      screen.queryByRole('status', { name: 'Apprise API transport warning' }),
    ).not.toBeInTheDocument();
    expect(config()).toEqual({
      ...initial,
      mode: 'cli',
      serverUrl: 'http://apprise.example.invalid',
      skipTlsVerify: true,
    });
    expect(updates).toHaveBeenCalledTimes(3);
    expect(dirty).toHaveBeenCalledTimes(3);
    expect(test).not.toHaveBeenCalled();
  });
});
