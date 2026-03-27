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

    expect(html).toContain('portal-shell-head-kicker">Pulse Account</div>');
    expect(html).toContain('portal-shell-head-title">Operator console</h1>');
    expect(html).toContain('Hosted operations, operator access, and commercial account services.');
    expect(html).toContain('portal-shell-nav');
    expect(html).toContain('Overview');
    expect(html).toContain('Workspaces');
    expect(html).toContain('Account services');
    expect(html).toContain('Support');
    expect(html).toContain('id="account-services-section"');
    expect(html).toContain('Self-hosted licenses and billing');
    expect(html).toContain('portal-account-bar');
    expect(html).toContain('id="accounts-root"');
    expect(html).toContain('MSP account');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('Operator workspace account');
    expect(html).toContain('3 workspaces');
    expect(html).toContain('Add workspace');
    expect(html).toContain('Manage billing');
    expect(html).toContain('Manage team');
    expect(html).toContain('Next move');
    expect(html).toContain('Start in Workspaces');
    expect(html).toContain('Hosted posture needs review');
    expect(html).toContain('Use this console to run client workspaces, account billing, and operator access from one place.');
    expect(html).toContain('Open workspaces');
    expect(html).toContain('Review team access');
    expect(html).toContain('Needs review');
    expect(html).toContain('Workspace fleet');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('Beta Workspace');
    expect(html).toContain('Gamma Workspace');
    expect(html).toContain('Active workspaces');
    expect(html).toContain('Needs attention');
    expect(html).toContain('Healthy</span>');
    expect(html).toContain('Needs attention</span>');
    expect(html).toContain('Checking</span>');
    expect(html).toContain('Ready for operator work');
    expect(html).toContain('This workspace needs attention before it is trustworthy.');
    expect(html).toContain('This workspace is still waiting on a completed health check.');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('Open workspace');
    expect(html).toContain('data-action="select-workspace"');
    expect(html).toContain('Workspace management');
    expect(html).toContain('Choose a workspace from the fleet to open its management desk.');
    expect(html).toContain('What shows here');
    expect(html).toContain('Lifecycle and health summary for the selected workspace');
    expect(html).toContain('data-action="clear-workspace-selection"');
    expect(html).toContain('Team management');
    expect(html).toContain('Invite someone new');
    expect(html).toContain('Access model');
    expect(html).toContain('Workspace operations without billing ownership.');
    expect(html).toContain('People on this account');
    expect(html).toContain('data-action="workspace-action"');
    expect(html).toContain('service-action-row');
    expect(html).toContain('service-action-button');
    expect(html).toContain('id="service-panel-empty"');
    expect(html).toContain('Choose the next commercial action');
    expect(html).toContain('What to expect');
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

    expect(html).toContain('Pulse Account');
    expect(html).toContain('Self-hosted only');
    expect(html).toContain('portal-shell-head-title">Account console</h1>');
    expect(html).toContain('Billing, license recovery, refunds, and privacy actions until hosted access is attached.');
    expect(html).toContain('Use these tools for self-hosted billing, license recovery, refunds, and privacy.');
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
