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
});
