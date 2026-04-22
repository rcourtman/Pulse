import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { TlsVerificationWarningBanner } from '../TlsVerificationWarningBanner';

describe('TlsVerificationWarningBanner', () => {
  afterEach(() => cleanup());

  it('renders the shared TLS risk copy with the supplied subject', () => {
    render(() => <TlsVerificationWarningBanner subject="this connection" />);

    expect(screen.getByRole('alert')).toHaveTextContent(
      'TLS verification disabled. Pulse will accept untrusted certificates for this connection. Use this only for controlled lab environments. Install a trusted certificate before using this in production.',
    );
  });

  it('renders custom remediation guidance when provided', () => {
    render(() => (
      <TlsVerificationWarningBanner
        subject="this endpoint"
        remediation="Install a trusted certificate on the endpoint before using this in production."
      />
    ));

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Install a trusted certificate on the endpoint before using this in production.',
    );
  });
});
