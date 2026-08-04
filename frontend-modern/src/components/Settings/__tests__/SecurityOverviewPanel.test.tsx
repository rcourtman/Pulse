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
    expect(screen.getAllByText('Open API Access')[0]).toHaveAttribute(
      'href',
      '/settings/security/api',
    );
    expect(screen.getByText('Open security guide')).toHaveAttribute('href', '/docs/SECURITY');
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

  // A truncated status response omits the settings-derived posture fields, so
  // scoring it would show false negatives to non-admin roles (#1675).
  it('hides the posture score behind an explanation for non-privileged viewers', () => {
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => ({
          detailLevel: 'authenticated',
          hasAuthentication: true,
          requiresAuth: true,
        })}
        securityStatusLoading={() => false}
      />
    ));

    expect(screen.getByText('Security posture needs an admin session')).toBeInTheDocument();
    expect(screen.queryByText('Security Posture')).not.toBeInTheDocument();
    expect(screen.queryByText('Hardening priorities')).not.toBeInTheDocument();
    expect(screen.queryByText('Recommended hardening steps')).not.toBeInTheDocument();
    expect(screen.queryByText('Act now')).not.toBeInTheDocument();
  });

  it('shows the posture score for privileged viewers', () => {
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => ({
          detailLevel: 'privileged',
          hasAuthentication: true,
          apiTokenConfigured: true,
          exportProtected: true,
          unprotectedExportAllowed: false,
          hasHTTPS: true,
          hasAuditLogging: true,
          requiresAuth: true,
          publicAccess: false,
          isPrivateNetwork: true,
        })}
        securityStatusLoading={() => false}
      />
    ));

    expect(screen.queryByText('Security posture needs an admin session')).not.toBeInTheDocument();
    expect(screen.getByText('Security Posture')).toBeInTheDocument();
  });
});
