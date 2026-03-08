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
});
