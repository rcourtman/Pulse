import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { readSSOReturnError, SSOReturnError } from '@/components/SSOReturnError';

describe('SSOReturnError', () => {
  afterEach(() => {
    cleanup();
  });

  it('returns the verified-email message for OIDC email_unverified', () => {
    const message = readSSOReturnError(
      new URLSearchParams('oidc=error&oidc_error=email_unverified'),
    );
    expect(message).toBe(
      'Your identity provider has not verified your email address. Verify it there, then sign in again.',
    );

    render(() => <SSOReturnError message={message} />);
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Your identity provider has not verified your email address. Verify it there, then sign in again.',
    );
  });

  it('returns an empty string when no SSO error is present', () => {
    expect(readSSOReturnError(new URLSearchParams(''))).toBe('');
    expect(readSSOReturnError(new URLSearchParams('oidc=success'))).toBe('');

    render(() => <SSOReturnError message="" />);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('falls back to the generic message for unknown error codes', () => {
    expect(readSSOReturnError(new URLSearchParams('oidc=error&oidc_error=unlisted_code'))).toBe(
      'Single sign-on failed. Please try again or contact an administrator.',
    );
  });
});
