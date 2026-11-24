import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { NetworkBoundarySettingsSection } from '../NetworkBoundarySettingsSection';

function setup(overridden = false) {
  const setAllowedOrigins = vi.fn();
  const setHasUnsavedChanges = vi.fn();
  const Harness = () => {
    const [publicURL, setPublicURL] = createSignal('');
    const [allowEmbedding, setAllowEmbedding] = createSignal(false);
    const [allowedEmbedOrigins, setAllowedEmbedOrigins] = createSignal('');
    const [webhookAllowedPrivateCIDRs, setWebhookAllowedPrivateCIDRs] = createSignal('');
    return (
      <NetworkBoundarySettingsSection
        publicURL={publicURL}
        setPublicURL={setPublicURL}
        allowEmbedding={allowEmbedding}
        setAllowEmbedding={setAllowEmbedding}
        allowedEmbedOrigins={allowedEmbedOrigins}
        setAllowedEmbedOrigins={setAllowedEmbedOrigins}
        webhookAllowedPrivateCIDRs={webhookAllowedPrivateCIDRs}
        setWebhookAllowedPrivateCIDRs={setWebhookAllowedPrivateCIDRs}
        allowedOrigins={() => (overridden ? 'https://managed.example.com' : '')}
        envOverrides={() => ({ allowedOrigins: overridden })}
        setAllowedOrigins={setAllowedOrigins}
        setHasUnsavedChanges={setHasUnsavedChanges}
      />
    );
  };
  render(() => <Harness />);
  return {
    input: screen.getByRole('textbox', { name: 'CORS Allowed Origins' }),
    setAllowedOrigins,
    setHasUnsavedChanges,
  };
}

afterEach(cleanup);

describe('CORS settings guidance', () => {
  it('associates exact-origin and credential limits with the labelled control', () => {
    const { input, setAllowedOrigins, setHasUnsavedChanges } = setup();
    expect(input).toHaveAccessibleDescription(/same-origin reverse proxy needs no CORS exception/);
    expect(input).toHaveAccessibleDescription(/scheme, host and port/);
    expect(input).toHaveAccessibleDescription(/without a path or trailing slash/);
    expect(input).toHaveAccessibleDescription(/Separate origins with commas/);
    expect(input).toHaveAccessibleDescription(/without credentialed browser access/);
    expect(input).toHaveAccessibleDescription(/credentialed browser access\. It is not/);
    expect(input).toHaveAccessibleDescription(/not a login or proxy repair/);
    expect(input).toHaveAccessibleDescription(/authentication, CSRF protection or TLS/);
    expect(input).toHaveAttribute('placeholder', 'https://app.example.com:8443');
    expect(input).toHaveValue('');
    expect(setAllowedOrigins).not.toHaveBeenCalled();
    expect(setHasUnsavedChanges).not.toHaveBeenCalled();
  });

  it('preserves explicit user input and marks the existing form dirty', () => {
    const { input, setAllowedOrigins, setHasUnsavedChanges } = setup();
    const value = 'https://one.example.com,https://two.example.com:8443';
    fireEvent.input(input, { target: { value } });
    expect(setAllowedOrigins).toHaveBeenCalledExactlyOnceWith(value);
    expect(setHasUnsavedChanges).toHaveBeenCalledExactlyOnceWith(true);
  });

  it('preserves environment override and does not silently widen its value', () => {
    const { input, setAllowedOrigins, setHasUnsavedChanges } = setup(true);
    expect(input).toBeDisabled();
    expect(input).toHaveValue('https://managed.example.com');
    expect(input).toHaveAccessibleDescription(/not a login or proxy repair/);
    expect(
      screen.getByText('Overridden by ALLOWED_ORIGINS environment variable'),
    ).toBeInTheDocument();
    fireEvent.input(input, { target: { value: '*' } });
    expect(setAllowedOrigins).not.toHaveBeenCalled();
    expect(setHasUnsavedChanges).not.toHaveBeenCalled();
  });

  it('marks embed origins and webhook CIDRs dirty on input and validates malformed values', () => {
    const { setHasUnsavedChanges } = setup();
    const webhookInput = screen.getByLabelText('Allowed Private IP Ranges for Webhooks');
    fireEvent.input(webhookInput, { target: { value: 'not-a-cidr' } });
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    expect(screen.getByRole('alert')).toHaveTextContent(/Invalid CIDR range/);

    const embedToggle = screen.getByRole('button', { name: 'Allow iframe embedding' });
    fireEvent.click(embedToggle);
    const embedInput = screen.getByLabelText('Allowed Embed Origins (optional)');
    fireEvent.input(embedInput, { target: { value: 'https://home.example.com/path' } });
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    expect(
      screen
        .getAllByRole('alert')
        .some((el) => /no path or trailing slash/.test(el.textContent || '')),
    ).toBe(true);
  });
});
