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
    commercial_api_base_url: '/api/portal/commercial',
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

    expect(html).toContain('portal-shell-nav');
    expect(html).toContain('Overview');
    expect(html).toContain('Workspaces');
    expect(html).toContain('Account services');
    expect(html).toContain('Support');
    expect(html).toContain('id="account-services-section"');
    expect(html).toContain('Self-hosted licenses and billing');
    expect(html).toContain('portal-account-context');
    expect(html).toContain('Owner access');
    expect(html).toContain('id="accounts-root"');
    expect(html).toContain('MSP account');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('Operator workspace account');
    expect(html).toContain('3 workspaces');
    expect(html).toContain('Add workspace');
    expect(html).toContain('Manage billing');
    expect(html).toContain('Manage team');
    expect(html).toContain('Hosted posture');
    expect(html).toContain('Start here to judge fleet posture');
    expect(html).toContain('Next move');
    expect(html).toContain('Start in Workspaces');
    expect(html).toContain('Hosted posture needs review');
    expect(html).toContain('Fleet posture');
    expect(html).toContain('Use this console to run client workspaces, account billing, and operator access from one place.');
    expect(html).toContain('Open workspaces');
    expect(html).toContain('Review team access');
    expect(html).toContain('Needs review');
    expect(html).toContain('Workspace fleet');
    expect(html).toContain('account-stage-header-actions');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('Beta Workspace');
    expect(html).toContain('Gamma Workspace');
    expect(html).toContain('Ready now');
    expect(html).toContain('Suspended');
    expect(html).toContain('Needs attention');
    expect(html).toContain('ready</span>');
    expect(html).toContain('Needs attention</span>');
    expect(html).toContain('Checking</span>');
    expect(html).toContain('Ready for operator work');
    expect(html).toContain('This workspace needs attention before it is trustworthy.');
    expect(html).toContain('This workspace is still waiting on a completed health check.');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('Open workspace');
    expect(html).toContain('data-action="select-workspace"');
    expect(html).toContain('Workspace management');
    expect(html).toContain('Pick a workspace to open its operator desk.');
    expect(html).toContain('Desk flow');
    expect(html).toContain('Read posture first');
    expect(html).toContain('Escalate account changes separately');
    expect(html.indexOf('id="add-ws-form-acct_1"')).toBeGreaterThan(html.indexOf('Workspace management'));
    expect(html.indexOf('id="add-ws-form-acct_1"')).toBeGreaterThan(html.indexOf('Keep account actions close'));
    expect(html).toContain('data-action="clear-workspace-selection"');
    expect(html).toContain('Team management');
    expect(html).toContain('Invite someone new');
    expect(html).toContain('Access model');
    expect(html).toContain('Review desk');
    expect(html).toContain('Keep access disciplined');
    expect(html).toContain('Owners stay rare');
    expect(html).toContain('Workspace operations without billing ownership.');
    expect(html).toContain('People on this account');
    expect(html).toContain('data-action="workspace-action"');
    expect(html).toContain('service-action-row');
    expect(html).toContain('service-action-button');
    expect(html).toContain('id="service-panel-empty"');
    expect(html).toContain('Choose the next commercial action');
    expect(html).toContain('Task desk');
    expect(html).toContain('Available flows');
    expect(html).toContain('Escalate quickly');
    expect(html).toContain('id="open-retrieve-service"');
    expect(html).toContain('id="data-service-panel"');
    expect(html).toContain('Open billing');
    expect(html).toContain('Open license recovery');
    expect(html).toContain('Open refunds');
    expect(html).toContain('Open privacy tools');
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
    expect(html).toContain('Account console');
    expect(html).toContain('Use these account tools for self-hosted licenses, billing, refunds, and privacy actions.');
    expect(html).toContain('Keep self-hosted commercial work here.');
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

  it('treats suspended workspaces as parked rather than ready', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_suspend',
              name: 'Suspended MSP',
              kind: 'msp',
              kind_label: 'MSP',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              workspaces: [
                {
                  id: 'ws_suspended',
                  display_name: 'Paused Workspace',
                  state: 'suspended',
                  healthy: true,
                  health_status: 'healthy',
                  last_health_check: '2026-03-26T10:10:00Z',
                  created_at: '2026-03-26T10:00:00Z',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('Suspended until you resume it');
    expect(html).toContain('Active fleet is stable');
    expect(html).toContain('Active hosted workspaces are healthy. Suspended workspaces stay parked until you resume them.');
    expect(html).toContain('Suspended stays parked');
  });
});
