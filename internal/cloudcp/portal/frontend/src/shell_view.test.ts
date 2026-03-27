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
    commercial_api_base_url: 'https://license.pulserelay.pro',
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

    expect(html).toContain('No hosted workspaces are attached to this account.');
    expect(html).toContain('self-hosted licensing and billing tools below');
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
                  health_status: 'healthy',
                  last_health_check: '2026-03-26T10:10:00Z',
                  created_at: '2026-03-26T10:00:00Z',
                },
                {
                  id: 'ws_failed',
                  display_name: 'Beta Workspace',
                  state: 'failed',
                  healthy: false,
                  health_status: 'unhealthy',
                },
                {
                  id: 'ws_pending',
                  display_name: 'Gamma Workspace',
                  state: 'provisioning',
                  healthy: false,
                  health_status: 'checking',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('<h1>Pulse Account</h1>');
    expect(html).toContain('Hosted access is active on this account.');
    expect(html).toContain('portal-shell-nav');
    expect(html).toContain('Overview');
    expect(html).toContain('Workspaces');
    expect(html).toContain('Account services');
    expect(html).toContain('Support');
    expect(html).toContain('id="account-services-section"');
    expect(html).toContain('Self-hosted licenses and billing');
    expect(html).toContain('id="accounts-root"');
    expect(html).toContain('MSP account');
    expect(html).toContain('MSP account · Owner · 3 workspaces');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('Account operations');
    expect(html).toContain('Manage the client fleet from this account surface.');
    expect(html).toContain('Workspace fleet');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('Beta Workspace');
    expect(html).toContain('Gamma Workspace');
    expect(html).toContain('Active workspaces');
    expect(html).toContain('Needs attention');
    expect(html).toContain('Healthy</span>');
    expect(html).toContain('Needs attention</span>');
    expect(html).toContain('Checking</span>');
    expect(html).toContain('Live updates and health checks are currently good.');
    expect(html).toContain('This workspace needs attention before it is trustworthy.');
    expect(html).toContain('This workspace is still waiting on a completed health check.');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('Open workspace');
    expect(html).toContain('data-action="select-workspace"');
    expect(html).toContain('Workspace management');
    expect(html).toContain('Choose a workspace to manage from the fleet above.');
    expect(html).toContain('data-action="clear-workspace-selection"');
    expect(html).toContain('Team management');
    expect(html).toContain('Invite someone new');
    expect(html).toContain('People on this account');
    expect(html).toContain('data-action="workspace-action"');
    expect(html).toContain('service-action-row');
    expect(html).toContain('service-action-button');
    expect(html).toContain('id="open-retrieve-service"');
    expect(html).toContain('id="data-service-panel"');
  });

  it('renders self-hosted overview copy when no hosted accounts are attached', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [],
        }),
      })
    );

    expect(html).toContain('<h1>Self-hosted Pulse Account</h1>');
    expect(html).toContain('No hosted workspace access is attached to this account yet.');
    expect(html).toContain('This account does not currently have hosted workspace access.');
    expect(html).toContain('Account services');
    expect(html).not.toContain('Self-hosted licenses and billing');
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
          request: {
            pending: false,
            error: '',
          },
        }),
      })
    );

    expect(errorHTML).toContain('value="buyer@example.com"');
    expect(errorHTML).toContain('Invalid email');
    expect(successHTML).toContain('If that email is registered, a magic link is on the way.');
    expect(successHTML).toContain('data-portal-action="resend-magic-link"');
  });
});
