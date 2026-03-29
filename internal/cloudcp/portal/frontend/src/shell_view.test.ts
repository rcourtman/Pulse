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
    has_self_hosted_commercial: false,
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

    expect(html).toContain('Current state');
    expect(html).toContain('No hosted account');
    expect(html).toContain('0 hosted workspaces');
    expect(html).toContain('Billing available');
    expect(html).toContain('0 hosted workspaces need review');
    expect(html).toContain('Billing is available');
    expect(html).toContain('Open billing');
  });

  it('renders authenticated portal accounts, workspaces, and hosted-only billing entrypoints', function() {
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
              members: [],
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

    expect(html).toContain('portal-tab-bar');
    expect(html).toContain('Summary');
    expect(html).toContain('Access');
    expect(html).toContain('Billing');
    expect(html).toContain('Support');
    expect(html).toContain('Workspaces');
    expect(html).toContain('Ready');
    expect(html).toContain('2 workspaces to review');
    expect(html).toContain('id="billing-section"');
    expect(html).toContain('portal-identity-bar');
    expect(html).toContain('Owner');
    expect(html).toContain('MSP account');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('3 workspaces');
    expect(html).toContain('Create workspace');
    expect(html).not.toContain('Manage billing');
    expect(html).not.toContain('Manage team');
    expect(html).toContain('0 suspended workspaces');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('Beta Workspace');
    expect(html).toContain('Gamma Workspace');
    expect(html).toContain('1 ready workspace');
    expect(html).toContain('Unhealthy</span>');
    expect(html).toContain('Checking</span>');
    expect(html).toContain('This workspace is in a failed state.');
    expect(html).toContain('Latest health check is still pending.');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('Open workspace');
    expect(html).toContain('data-action="select-workspace"');
    expect(html).toContain('Workspace task');
    expect(html).toContain('Work on one workspace');
    expect(html).toContain('Open lifecycle for one workspace, or create a new one. Keep access and billing separate.');
    expect(html).toContain('Open a new hosted workspace');
    expect(html).toContain('Access changes stay in Access. Billing changes stay in Billing.');
    expect(html).toContain('Close panel');
    expect(html).toContain('id="workspace-management-acct_1" hidden');
    expect(html).toContain('id="workspace-operations-detail-acct_1" hidden');
    expect(html.indexOf('id="add-ws-form-acct_1"')).toBeGreaterThan(html.indexOf('Open a new hosted workspace'));
    expect(html).toContain('data-action="clear-workspace-selection"');
    expect(html).toContain('Invite people');
    expect(html).toContain('Change roles');
    expect(html).toContain('Remove access');
    expect(html).toContain('data-action="set-access-job"');
    expect(html).toContain('Access task');
    expect(html).toContain('id="access-detail-acct_1" hidden');
    expect(html).toContain('Choose the smallest role');
    expect(html).toContain('Full account, billing, and access control.');
    expect(html).toContain('Workspace control, billing, and roster management.');
    expect(html).toContain('Workspace control without billing or roster ownership.');
    expect(html).toContain('Review access without control-plane changes.');
    expect(html).toContain('People on this account');
    expect(html).toContain('data-can-manage="true"');
    expect(html).toContain('Remove stale access');
    expect(html).toContain('data-action="workspace-action"');
    expect(html).toContain('Hosted billing');
    expect(html).toContain('Try first');
    expect(html).toContain('Path');
    expect(html).toContain('Account or email');
    expect(html).toContain('Failed action');
    expect(html).toContain('Billing');
    expect(html).not.toContain('Hosted only');
    expect(html).not.toContain('Self-hosted tools');
    expect(html).not.toContain('Use self-hosted billing only for self-hosted purchases.');
    expect(html).not.toContain('id="open-retrieve-billing"');
    expect(html).not.toContain('id="billing-detail-shell" hidden');
    expect(html).not.toContain('data-account-billing-action="clear-billing-panel"');
    expect(html).not.toContain('id="data-billing-panel"');
    expect(html).not.toContain('>Licenses<');
    expect(html).not.toContain('>Refunds<');
    expect(html).not.toContain('>Privacy<');
  });

  it('defaults the authenticated hosted shell to workspaces', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_default',
              name: 'Default Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [],
            },
          ],
        }),
      })
    );

    expect(html).toContain('data-shell-section="workspaces"');
  });

  it('renders mixed billing tools only when self-hosted commercial history is relevant', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          has_self_hosted_commercial: true,
          accounts: [
            {
              id: 'acct_mixed',
              name: 'Mixed Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_mixed',
                  display_name: 'Mixed Workspace',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('Self-hosted billing');
    expect(html).toContain('Use self-hosted billing only for self-hosted purchases.');
    expect(html).toContain('id="open-retrieve-billing"');
    expect(html).toContain('id="billing-detail-shell" hidden');
    expect(html).toContain('data-account-billing-action="clear-billing-panel"');
    expect(html).toContain('id="data-billing-panel"');
    expect(html).toContain('licenses, refunds, or privacy');
    expect(html).toContain('Escalate with the same hosted billing action or self-hosted path and the exact failed step.');
  });

  it('defaults the authenticated self-hosted shell to billing', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          has_self_hosted_commercial: true,
          accounts: [],
        }),
      })
    );

    expect(html).toContain('data-shell-section="billing"');
  });

  it('keeps top-level task navigation in the simplified section order', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_nav',
              name: 'Navigation Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_nav',
                  display_name: 'Navigation Workspace',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    var workspacesIndex = html.indexOf('data-shell-action="activate-section" data-shell-section="workspaces"');
    var accessIndex = html.indexOf('data-shell-action="activate-section" data-shell-section="access"');
    var billingIndex = html.indexOf('data-shell-action="activate-section" data-shell-section="billing"');
    var supportIndex = html.indexOf('data-shell-action="activate-section" data-shell-section="support"');

    expect(workspacesIndex).toBeGreaterThan(-1);
    expect(accessIndex).toBeGreaterThan(workspacesIndex);
    expect(billingIndex).toBeGreaterThan(accessIndex);
    expect(supportIndex).toBeGreaterThan(billingIndex);
    expect(html).not.toContain('Services');
  });

  it('renders one simple account context strip without summary facts', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_context',
              name: 'Context Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'admin',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_context',
                  display_name: 'Context Workspace',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    expect((html.match(/portal-identity-bar\"/g) || []).length).toBe(1);
    expect((html.match(/portal-account-context-stat/g) || []).length).toBe(0);
    expect((html.match(/account-context-chip\"/g) || []).length).toBe(0);
    expect(html).toContain('Admin');
    expect(html).toContain('Cloud account');
    expect(html).not.toContain('Manage workspaces, access, and billing for this account.');
    expect(html).not.toContain('portal-account-context-summary');
  });

  it('renders a view-only access surface when the account cannot manage access', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_ro',
              name: 'Observer Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'tech',
              can_manage: false,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_ro',
                  display_name: 'Observed Workspace',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('Tech');
    expect(html).toContain('1 workspace is ready to use');
    expect(html).toContain('Owner or admin required');
    expect(html).toContain('Review the hosted roster here. An owner or admin must make changes.');
    expect(html).toContain('An owner or admin on this account needs to open hosted billing.');
    expect(html).toContain('Try first');
    expect(html).toContain('Review Workspaces or Access first. If billing is involved, hand it to an owner or admin before you escalate.');
    expect(html).toContain('Path');
    expect(html).toContain('Workspaces review, Access review, owner/admin handoff, or hosted billing.');
    expect(html).toContain('Account or email');
    expect(html).toContain('Hosted account and workspace, or hosted billing account.');
    expect(html).toContain('data-can-manage="false"');
    expect(html).not.toContain('Invite people, change roles, and remove account access.');
    expect(html).not.toContain('data-action="invite-member"');
    expect(html).not.toContain('data-action="set-access-job"');
    expect(html).not.toContain('Self-hosted billing, licenses, refunds, and privacy stay in Billing.');
    expect(html).not.toContain('Use support only when the Workspaces, Access, or hosted Billing path has already stopped you.');
    expect(html).not.toContain('Workspace or access path failed');
  });

  it('renders read-only account roles without leaking internal identifiers', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_view',
              name: 'Viewer Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'read_only',
              can_manage: false,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_view',
                  display_name: 'Viewer Workspace',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('Read-only');
    expect(html).not.toContain('Read_only role');
    expect(html).not.toContain('READ_ONLY');
  });

  it('keeps hosted view-only empty accounts on plain review surfaces', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_view_empty',
              name: 'Empty Hosted Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'read_only',
              can_manage: false,
              has_billing: true,
              members: [],
              workspaces: [],
            },
          ],
        }),
      })
    );

    expect(html).toContain('No hosted workspaces are attached yet. An owner or admin must create the first one.');
    expect(html).toContain('Owner or admin required');
  });

  it('keeps workspaces empty state honest for hosted view-only accounts with no workspace yet', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_view_empty',
              name: 'Empty Hosted Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'read_only',
              can_manage: false,
              has_billing: true,
              members: [],
              workspaces: [],
            },
          ],
        }),
      })
    );

    expect(html).toContain('0 ready workspaces');
    expect(html).toContain('No hosted workspace exists yet. An owner or admin must create the first one.');
    expect(html).not.toContain('Open Workspaces to see the current state of each hosted workspace.');
  });

  it('keeps workspaces empty state honest for managed hosted accounts with no workspace yet', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_manage_empty',
              name: 'Managed Empty Account',
              kind: 'msp',
              kind_label: 'MSP',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [],
            },
          ],
        }),
      })
    );

    expect(html).toContain('0 ready workspaces');
    expect(html).toContain('No hosted workspace exists yet. Create the first one in Workspaces.');
    expect(html).not.toContain('Open Workspaces to see the current state of each hosted workspace.');
  });

  it('keeps suspended hosted view-only accounts on review surfaces', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_view_suspended',
              name: 'Suspended Hosted Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'read_only',
              can_manage: false,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_view_suspended',
                  display_name: 'Paused Workspace',
                  state: 'suspended',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('Paused Workspace');
    expect(html).toContain('1 suspended workspace');
    expect(html).toContain('Owner or admin required');
  });

  it('keeps workspace facts honest when only suspended workspaces remain', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_view_suspended',
              name: 'Suspended Hosted Account',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'read_only',
              can_manage: false,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_view_suspended',
                  display_name: 'Paused Workspace',
                  state: 'suspended',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('0 ready workspaces');
    expect(html).toContain('1 suspended workspace');
    expect(html).not.toContain('Active workspaces look clear for routine use.');
  });

  it('renders a billing-first self-hosted shell when no hosted accounts are attached', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [],
        }),
      })
    );

    expect(html).toContain('Support');
    expect(html).not.toContain('Current state');
    expect(html).not.toContain('data-shell-section="workspaces"');
    expect(html).not.toContain('data-shell-section="access"');
    expect(html).toContain('Self-hosted billing');
    expect(html).toContain('Use self-hosted billing only for self-hosted purchases.');
    expect(html).toContain('Escalate with the same self-hosted billing path and the exact failed step.');
    expect(html).toContain('Billing');
    expect(html).toContain('Account or email');
    expect(html).toContain('Commercial billing email used for the self-hosted purchase.');
    expect(html).not.toContain('Escalate with the same hosted billing action or self-hosted path and the exact failed step.');
    expect(html).not.toContain('Workspace or access path failed');
    expect(html).not.toContain('Hosted workspace or access');
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
    expect(errorHTML).toContain('portal-auth-shell');
    expect(errorHTML).toContain('Email sign-in link');
    expect(errorHTML).toContain('Commercial email');
    expect(errorHTML).toContain('Send sign-in link');
    expect(successHTML).toContain('If that email is registered, a sign-in link is on the way.');
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
              members: [],
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

    expect(html).toContain('Suspended');
    expect(html).toContain('0 workspaces to review');
    expect(html).toContain('0 ready workspaces');
    expect(html).toContain('Create workspace');
    expect(html).toContain('1 suspended workspace');
  });

  it('preserves the simplified top-tab shell hooks in the rendered shell', function() {
    var html = renderAuthenticatedPortalHTML(createContext());
    expect(html).toContain('portal-shell-main');
    expect(html).toContain('portal-tab-bar');
    expect(html).not.toContain('portal-shell-layout');
  });
});
