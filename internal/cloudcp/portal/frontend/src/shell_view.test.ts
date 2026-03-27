import { describe, expect, it } from 'vitest';

import type { PortalBootstrapData, PortalLoginState } from './types';
import {
  renderAccountsHTML,
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
  type ShellViewContext,
} from './shell_view';

function createBootstrap(overrides: Partial<PortalBootstrapData> = {}): PortalBootstrapData {
  return {
    authenticated: true,
    email: 'owner@example.com',
    public_site_url: 'https://pulserelay.pro',
    support_email: 'support@pulserelay.pro',
    commercial_api_base_path: '/api/portal/commercial',
    portal_path: '/portal',
    bootstrap_path: '/api/portal/bootstrap',
    magic_link_request_path: '/auth/magic-link',
    signup_path: '/signup',
    logout_path: '/auth/logout',
    account_api_base_path: '/api/accounts',
    portal_api_base_path: '/api/portal',
    accounts: [],
    ...overrides,
  };
}

function createLoginState(overrides: Partial<PortalLoginState> = {}): PortalLoginState {
  return {
    emailValue: '',
    request: {
      pending: false,
      error: '',
    },
    success: false,
    successMessage: '',
    ...overrides,
  };
}

function createContext(overrides: Partial<ShellViewContext> = {}): ShellViewContext {
  return {
    bootstrap: createBootstrap(),
    loginState: createLoginState(),
    signupPath: '/signup',
    accountAPIBasePath: '/api/accounts',
    ...overrides,
  };
}

describe('shell view', function() {
  it('renders authenticated header with account email and sign-out button', function() {
    var html = renderHeaderHTML(createContext());

    expect(html).toContain('owner@example.com');
    expect(html).toContain('id="logout-btn"');
    expect(html).toContain('Sign out');
  });

  it('renders signed-out header with create-account link', function() {
    var html = renderHeaderHTML(
      createContext({
        bootstrap: createBootstrap({ authenticated: false, email: '' }),
      })
    );

    expect(html).toContain('href="/signup"');
    expect(html).toContain('Create account');
    expect(html).not.toContain('logout-btn" id="logout-btn"');
  });

  it('renders empty accounts state with support contact', function() {
    var html = renderAccountsHTML(createContext());

    expect(html).toContain('No hosted workspaces on this account');
    expect(html).toContain('mailto:support@pulserelay.pro');
    expect(html).toContain('support@pulserelay.pro');
  });

  it('renders authenticated portal accounts, workspaces, and service entrypoints', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_1',
              name: 'Acme MSP',
              kind: 'msp',
              kind_label: 'MSP',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              workspaces: [
                {
                  id: 'ws_active',
                  display_name: 'Alpha Workspace',
                  state: 'active',
                  healthy: true,
                  created_at: '2026-03-26T10:00:00Z',
                },
                {
                  id: 'ws_failed',
                  display_name: 'Beta Workspace',
                  state: 'failed',
                  healthy: false,
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('<h1>Pulse Account</h1>');
    expect(html).toContain('Hosted access is active on this account.');
    expect(html).toContain('MSP operator');
    expect(html).toContain('Hosted workspaces');
    expect(html).toContain('id="accounts-root"');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('MSP account · Owner · 2 workspaces');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('Open workspace');
    expect(html).toContain('Manage</button>');
    expect(html).toContain('data-action="workspace-manage"');
    expect(html).toContain('service-card-button');
    expect(html).toContain('Self-hosted licenses and billing');
    expect(html).toContain('id="open-retrieve-service"');
    expect(html).toContain('id="data-service-panel"');
  });

  it('renders a self-hosted overview when the signed-in account has no hosted workspaces', function() {
    var html = renderAuthenticatedPortalHTML(createContext());

    expect(html).toContain('This account currently uses Pulse Account for self-hosted commercial services.');
    expect(html).toContain('Signed in as');
    expect(html).toContain('owner@example.com');
    expect(html).toContain('Hosted access');
    expect(html).toContain('None on this account');
    expect(html).toContain('Self-hosted account services');
    expect(html).not.toContain('Other account services');
  });

  it('renders signed-out portal with error and success login states', function() {
    var errorHTML = renderSignedOutPortalHTML(
      createContext({
        bootstrap: createBootstrap({ authenticated: false, email: '' }),
        loginState: createLoginState({
          emailValue: 'buyer@example.com',
          request: {
            pending: false,
            error: 'Invalid email',
          },
        }),
      })
    );
    var successHTML = renderSignedOutPortalHTML(
      createContext({
        bootstrap: createBootstrap({ authenticated: false, email: '' }),
        loginState: createLoginState({
          emailValue: 'buyer@example.com',
          success: true,
          successMessage: "If that email is registered, you'll receive a magic link shortly.",
          request: {
            pending: false,
            error: '',
          },
        }),
      })
    );

    expect(errorHTML).toContain('value="buyer@example.com"');
    expect(errorHTML).toContain('Invalid email');
    expect(successHTML).toContain("If that email is registered, you&#39;ll receive a magic link shortly.");
    expect(successHTML).toContain('data-portal-action="resend-magic-link"');
  });
});
