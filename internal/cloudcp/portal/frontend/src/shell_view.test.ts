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

    expect(html).toContain('Account triage');
    expect(html).toContain('No hosted account');
    expect(html).toContain('Billing available');
    expect(html).toContain('Nothing urgent');
    expect(html).toContain('Billing tools are ready');
    expect(html).toContain('Open billing');
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
    expect(html).toContain('Access');
    expect(html).toContain('Billing');
    expect(html).toContain('Support');
    expect(html).toContain('Account tasks');
    expect(html).toContain('What needs attention, what is ready, and the next obvious action.');
    expect(html).toContain('1 account');
    expect(html).toContain('3 workspaces');
    expect(html).toContain('1 ready');
    expect(html).toContain('2 attention');
    expect(html).toContain('Manage');
    expect(html).toContain('id="billing-section"');
    expect(html).toContain('portal-account-context');
    expect(html).toContain('Owner access');
    expect(html).toContain('portal-account-context-summary');
    expect(html).toContain('Billing enabled');
    expect(html).toContain('id="accounts-root"');
    expect(html).toContain('MSP account');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('Hosted workspace account');
    expect(html).toContain('3 workspaces');
    expect(html).toContain('Add workspace');
    expect(html).not.toContain('Manage billing');
    expect(html).not.toContain('Manage team');
    expect(html).toContain('Account triage');
    expect(html).toContain('Only three questions matter here.');
    expect(html).toContain('section-context-strip');
    expect(html).toContain('Needs attention');
    expect(html).toContain('Ready');
    expect(html).toContain('Next action');
    expect(html).toContain('Review these first');
    expect(html).toContain('Open and work');
    expect(html).toContain('Review workspaces');
    expect(html).toContain('Review access');
    expect(html).toContain('account-stage-header-actions');
    expect(html).toContain('No suspended');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('Beta Workspace');
    expect(html).toContain('Gamma Workspace');
    expect(html).toContain('Needs attention');
    expect(html).toContain('ready</span>');
    expect(html).toContain('Needs attention</span>');
    expect(html).toContain('Checking</span>');
    expect(html).toContain('Ready to use');
    expect(html).toContain('This workspace needs attention before it is trustworthy.');
    expect(html).toContain('This workspace is still waiting on a completed health check.');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('Open workspace');
    expect(html).toContain('data-action="select-workspace"');
    expect(html).toContain('Workspace management');
    expect(html).toContain('Lifecycle');
    expect(html).toContain('Pick one workspace for lifecycle review. Keep access and billing changes in their own sections.');
    expect(html).toContain('Keep this section workspace-only');
    expect(html).toContain('Inspect status');
    expect(html).toContain('Stay deliberate');
    expect(html).toContain('Close panel');
    expect(html.indexOf('id="add-ws-form-acct_1"')).toBeGreaterThan(html.indexOf('Workspace management'));
    expect(html.indexOf('id="add-ws-form-acct_1"')).toBeGreaterThan(html.indexOf('Keep this section workspace-only'));
    expect(html).toContain('data-action="clear-workspace-selection"');
    expect(html).toContain('People and roles');
    expect(html).toContain('Invite');
    expect(html).toContain('Roles');
    expect(html).toContain('Remove access');
    expect(html).toContain('Invite someone new');
    expect(html).toContain('Role rules');
    expect(html).toContain('Access review');
    expect(html).toContain('Keep access explicit');
    expect(html).toContain('Owners stay rare');
    expect(html).toContain('Billing, access control, and full account control.');
    expect(html).toContain('Workspace control plus billing for the account.');
    expect(html).toContain('Workspace control without billing ownership.');
    expect(html).toContain('Workspace review and verification without control-plane changes.');
    expect(html).toContain('People on this account');
    expect(html).toContain('data-action="workspace-action"');
    expect(html).toContain('billing-action-row');
    expect(html).toContain('billing-action-button');
    expect(html).toContain('id="billing-panel-empty"');
    expect(html).toContain('Choose the billing task');
    expect(html).toContain('Billing brief');
    expect(html).toContain('Hosted billing');
    expect(html).toContain('Self-hosted tools');
    expect(html).toContain('Self-hosted billing');
    expect(html).toContain('Keep the billing request contained');
    expect(html).toContain('Hosted billing first when present');
    expect(html).toContain('Open support');
    expect(html).toContain('What each tool does');
    expect(html).toContain('Before you start');
    expect(html).toContain('Escalate quickly');
    expect(html).toContain('Identity first');
    expect(html).toContain('id="open-retrieve-billing"');
    expect(html).toContain('id="data-billing-panel"');
    expect(html).toContain('>Billing<');
    expect(html).toContain('>Licenses<');
    expect(html).toContain('>Refunds<');
    expect(html).toContain('>Privacy<');
    expect(html).toContain('Escalation');
    expect(html).toContain('Hosted problems');
    expect(html).toContain('Billing and self-hosted issues');
    expect(html).toContain('Escalation packet');
    expect(html).toContain('What to send');
    expect(html).toContain('Open billing');
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
    expect(html).toContain('Account tasks');
    expect(html).toContain('Self-hosted');
    expect(html).toContain('Account triage');
    expect(html).toContain('No hosted account');
    expect(html).toContain('Billing tools are ready');
    expect(html).toContain('There is nothing to open or manage here yet.');
    expect(html).toContain('There are no hosted roles or invites to manage for this account right now.');
    expect(html).toContain('Use this billing surface for self-hosted subscriptions, licenses, refunds, and privacy requests.');
    expect(html).toContain('No hosted billing attached');
    expect(html).toContain('Billing');
    expect(html).not.toContain('Self-hosted commercial services');
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
    expect(html).toContain('Nothing urgent');
    expect(html).toContain('No workspace is ready yet');
    expect(html).toContain('Create the next workspace');
    expect(html).toContain('Suspended stays parked');
  });
});
