import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { SecurityOverviewPanel } from '../SecurityOverviewPanel';

describe('SecurityOverviewPanel', () => {
  it('shows calm hardening guidance for private authenticated setups', () => {
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => ({
          hasAuthentication: true,
          apiTokenConfigured: false,
          exportProtected: true,
          unprotectedExportAllowed: false,
          hasHTTPS: false,
          hasAuditLogging: false,
          requiresAuth: true,
          publicAccess: false,
          isPrivateNetwork: true,
          clientIP: '127.0.0.1',
        })}
        securityStatusLoading={() => false}
      />
    ));

    expect(screen.getByText('Recommended hardening steps')).toBeInTheDocument();
    expect(screen.getByText('Plan HTTPS before live use')).toBeInTheDocument();
    expect(screen.getByText('Create an API token')).toBeInTheDocument();
    expect(screen.getAllByText('Open API Access')[0]).toHaveAttribute('href', '/settings/security/api');
    expect(screen.getByText('Open security guide')).toHaveAttribute('href', '/docs/SECURITY.md');
    expect(screen.getAllByText('Recommended')).toHaveLength(2);
  });

  it('shows critical hardening priorities for exposed setups', () => {
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => ({
          hasAuthentication: false,
          apiTokenConfigured: false,
          exportProtected: false,
          unprotectedExportAllowed: true,
          hasHTTPS: false,
          hasAuditLogging: false,
          requiresAuth: false,
          publicAccess: true,
          isPrivateNetwork: false,
        })}
        securityStatusLoading={() => false}
      />
    ));

    expect(screen.getByText('Hardening priorities')).toBeInTheDocument();
    expect(screen.getByText('Enable authentication')).toBeInTheDocument();
    expect(screen.getByText('Protect exports')).toBeInTheDocument();
    expect(screen.getByText('Enable HTTPS for public access')).toBeInTheDocument();
    expect(screen.getAllByText('Act now')).toHaveLength(3);
  });
});
