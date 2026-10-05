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

  it('only calls plain HTTP critical once the instance is publicly reachable', () => {
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
      <SecurityPostureSummary status={{ ...status, publicAccess: false, isPrivateNetwork: true }} />
    ));
    // Private network: HTTPS is a recommended step, so no Critical tag and the
    // critical share of the score is complete (0.7 + 2 of 7 x 0.3 = 79).
    expect(screen.queryByText('Critical')).toBeNull();
    expect(screen.getByText('79%')).toBeInTheDocument();
    unmount();

    render(() => (
      <SecurityPostureSummary status={{ ...status, publicAccess: true, isPrivateNetwork: false }} />
    ));
    expect(screen.getByText('Critical')).toBeInTheDocument();
    expect(screen.getByText('55%')).toBeInTheDocument();
  });
});
