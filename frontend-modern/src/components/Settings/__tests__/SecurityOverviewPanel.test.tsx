import { describe, expect, it, vi } from 'vitest';
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

  it('does not infer private exposure from a non-private HTTPS request with recommendations only', () => {
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => ({
          detailLevel: 'privileged',
          hasAuthentication: true,
          apiTokenConfigured: false,
          exportProtected: true,
          hasHTTPS: true,
          hasAuditLogging: true,
          requiresAuth: true,
          publicAccess: true,
          isPrivateNetwork: false,
          clientIP: '203.0.113.5',
        })}
        securityStatusLoading={() => false}
      />
    ));

    expect(screen.getByText('Accessed from a non-private network')).toBeInTheDocument();
    expect(screen.getByText('Recommended hardening steps')).toBeInTheDocument();
    expect(screen.getByText('Create an API token')).toBeInTheDocument();
    expect(screen.queryByText(/appears private/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Act now')).not.toBeInTheDocument();
  });

  it('shows a retryable error callout instead of a blank panel when status fails to load', () => {
    const onRetry = vi.fn();
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => null}
        securityStatusLoading={() => false}
        onRetry={onRetry}
      />
    ));

    expect(screen.getByText('Unable to load security status')).toBeInTheDocument();
    screen.getByRole('button', { name: 'Retry' }).click();
    expect(onRetry).toHaveBeenCalledOnce();
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

  it('reports an SSO-only posture without claiming a local password login', () => {
    render(() => (
      <SecurityOverviewPanel
        securityStatus={() => ({
          detailLevel: 'privileged',
          hasAuthentication: true,
          requiresAuth: true,
          ssoEnabled: true,
          hasProxyAuth: false,
          apiTokenConfigured: false,
          exportProtected: true,
          hasHTTPS: true,
          hasAuditLogging: true,
          publicAccess: false,
          isPrivateNetwork: true,
        })}
        securityStatusLoading={() => false}
      />
    ));

    expect(screen.getByText('Authentication', { exact: true })).toBeInTheDocument();
    expect(screen.getByText('Configured or required')).toBeInTheDocument();
    expect(screen.queryByText('Password login')).not.toBeInTheDocument();
    expect(screen.getByText('Single sign-on')).toBeInTheDocument();
    expect(screen.getByText('Provider configured')).toBeInTheDocument();
    expect(screen.getByText('83%')).toBeInTheDocument();
  });
});
