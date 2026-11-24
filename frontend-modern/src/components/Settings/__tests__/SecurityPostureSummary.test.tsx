import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { SecurityPostureSummary } from '../SecurityPostureSummary';

describe('SecurityPostureSummary', () => {
  it('renders a strong posture label for highly secured setups', () => {
    render(() => (
      <SecurityPostureSummary
        status={{
          hasAuthentication: true,
          ssoEnabled: true,
          hasProxyAuth: true,
          apiTokenConfigured: true,
          exportProtected: true,
          unprotectedExportAllowed: false,
          hasHTTPS: true,
          hasAuditLogging: true,
          requiresAuth: true,
          publicAccess: false,
          isPrivateNetwork: true,
        }}
      />
    ));

    expect(screen.getByText('Security Posture')).toBeInTheDocument();
    expect(screen.getByText('Strong')).toBeInTheDocument();
  });

  it('renders a weak posture label for unsecured setups', () => {
    render(() => (
      <SecurityPostureSummary
        status={{
          hasAuthentication: false,
          ssoEnabled: false,
          hasProxyAuth: false,
          apiTokenConfigured: false,
          exportProtected: false,
          unprotectedExportAllowed: true,
          hasHTTPS: false,
          hasAuditLogging: false,
          requiresAuth: false,
          publicAccess: true,
          isPrivateNetwork: false,
        }}
      />
    ));

    expect(screen.getByText('Weak')).toBeInTheDocument();
  });

  it('calibrates scores honestly across private and non-private HTTP setups', () => {
    const status = {
      hasAuthentication: true,
      ssoEnabled: false,
      hasProxyAuth: false,
      apiTokenConfigured: false,
      exportProtected: true,
      unprotectedExportAllowed: false,
      hasHTTPS: false,
      hasAuditLogging: false,
      requiresAuth: true,
    };
    const { unmount } = render(() => (
      <SecurityPostureSummary
        status={{ ...status, clientIP: '10.0.0.5', publicAccess: false, isPrivateNetwork: true }}
      />
    ));
    // Private network with 2 of 6 security dimensions active (Auth + Export):
    // no Critical tag on HTTPS, score reflects 2/6 = 33% instead of inflated 79%.
    expect(screen.queryByText('Critical')).toBeNull();
    expect(screen.getByText('33%')).toBeInTheDocument();
    expect(screen.getByText('2 of 6 checks active')).toBeInTheDocument();
    expect(screen.getByText('Accessed from a private network')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Authentication/i })).toHaveAttribute(
      'href',
      '/settings/security-auth',
    );
    unmount();

    render(() => (
      <SecurityPostureSummary
        status={{ ...status, clientIP: '100.64.0.5', publicAccess: true, isPrivateNetwork: false }}
      />
    ));
    expect(screen.getByText('Critical')).toBeInTheDocument();
    expect(screen.getByText('33%')).toBeInTheDocument();
    expect(screen.getByText('Accessed from a non-private network')).toBeInTheDocument();
  });
});
