import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
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
      screen.getByRole('checkbox', { name: 'Allow self-signed certificates' }),
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
});
