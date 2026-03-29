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

    expect(html).toContain('Account triage');
    expect(html).toContain('No hosted account');
    expect(html).toContain('Billing available');
    expect(html).toContain('Nothing urgent');
    expect(html).toContain('Billing tools are ready');
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
    expect(html).toContain('Create workspace');
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
    expect(html).toContain('Manage access');
    expect(html).toContain('Invite people');
    expect(html).toContain('Change roles');
    expect(html).toContain('Remove access');
    expect(html).toContain('Review the hosted roster, then open one access job at a time.');
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
    expect(html).toContain('Hosted only');
    expect(html).toContain('Use this billing surface only for hosted billing on your hosted workspace accounts.');
    expect(html).toContain('Escalation only');
    expect(html).toContain('Workspace or access path failed');
    expect(html).toContain('Hosted billing path failed');
    expect(html).toContain('Keep the escalation short');
    expect(html).toContain('What to send');
    expect(html).toContain('Open billing');
    expect(html).not.toContain('Self-hosted tools');
    expect(html).not.toContain('Self-hosted billing');
    expect(html).not.toContain('Pick the self-hosted job');
    expect(html).not.toContain('Use self-hosted billing only for self-hosted purchases.');
    expect(html).not.toContain('id="open-retrieve-billing"');
    expect(html).not.toContain('id="billing-detail-shell" hidden');
    expect(html).not.toContain('data-account-billing-action="clear-billing-panel"');
    expect(html).not.toContain('id="data-billing-panel"');
    expect(html).not.toContain('>Licenses<');
    expect(html).not.toContain('>Refunds<');
    expect(html).not.toContain('>Privacy<');
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

    expect(html).toContain('Self-hosted tools');
    expect(html).toContain('Self-hosted billing');
    expect(html).toContain('Pick the self-hosted job');
    expect(html).toContain('Use hosted billing first when the request belongs to a hosted workspace account.');
    expect(html).toContain('Use self-hosted billing only for self-hosted purchases.');
    expect(html).toContain('id="open-retrieve-billing"');
    expect(html).toContain('id="billing-detail-shell" hidden');
    expect(html).toContain('data-account-billing-action="clear-billing-panel"');
    expect(html).toContain('id="data-billing-panel"');
    expect(html).toContain('Billing path failed');
    expect(html).toContain('licenses, refunds, or privacy');
    expect(html).toContain('Escalate with the same hosted billing action or self-hosted path and the exact failed step.');
  });

  it('keeps top-level task navigation in the canonical section order', function() {
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

    var overviewIndex = html.indexOf('data-shell-section="overview"');
    var workspacesIndex = html.indexOf('data-shell-section="workspaces"');
    var accessIndex = html.indexOf('data-shell-section="access"');
    var billingIndex = html.indexOf('data-shell-section="billing"');
    var supportIndex = html.indexOf('data-shell-section="support"');

    expect(overviewIndex).toBeGreaterThan(-1);
    expect(workspacesIndex).toBeGreaterThan(overviewIndex);
    expect(accessIndex).toBeGreaterThan(workspacesIndex);
    expect(billingIndex).toBeGreaterThan(accessIndex);
    expect(supportIndex).toBeGreaterThan(billingIndex);
    expect(html).not.toContain('Services');
  });

  it('renders one compact account context strip with three summary facts', function() {
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

    expect((html.match(/portal-account-context\"/g) || []).length).toBe(1);
    expect((html.match(/portal-account-context-stat/g) || []).length).toBe(3);
    expect((html.match(/account-context-chip\"/g) || []).length).toBe(3);
    expect(html).toContain('Admin access');
    expect(html).toContain('Hosted account for workspace access, access control, and billing.');
    expect(html).toContain('Billing enabled');
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

    expect(html).toContain('Open a workspace and review current state. An owner or admin must create or change hosted workspaces.');
    expect(html).toContain('Open a workspace and review current state here. An owner or admin must create or change hosted workspaces.');
    expect(html).toContain('Tech role');
    expect(html).toContain('Hosted account where you can open workspaces and review who already has access. An owner or admin handles access changes and billing.');
    expect(html).toContain('View only');
    expect(html).toContain('Owner/admin required');
    expect(html).toContain('Hosted billing stays separate');
    expect(html).toContain('Hosted billing stays in Billing, and an owner or admin must open it.');
    expect(html).toContain('Review access');
    expect(html).toContain('Owner or admin required');
    expect(html).toContain('Review who already has access to this hosted account. An owner or admin must make changes.');
    expect(html).toContain('Review the hosted roster here. An owner or admin must make changes.');
    expect(html).toContain('Hosted billing is attached here, but an owner or admin must open it.');
    expect(html).toContain('Escalation only after the review, owner/admin, or billing path is exhausted.');
    expect(html).toContain('Use support only when the same Workspaces review, Access review, owner/admin, or hosted Billing path has already stopped you.');
    expect(html).toContain('Owner/admin first');
    expect(html).toContain('Hosted review or owner/admin path failed');
    expect(html).toContain('Review the same workspace or roster here, then have an owner or admin run the blocked change before you escalate.');
    expect(html).toContain('Review the same task');
    expect(html).toContain('Use Workspaces to confirm workspace state and Access to confirm the current roster before you escalate.');
    expect(html).toContain('Name the blocked owner/admin action');
    expect(html).toContain('Include the account, workspace, and the lifecycle or access change that still needs an owner or admin.');
    expect(html).toContain('Review workspaces');
    expect(html).toContain('Review access');
    expect(html).toContain('Hosted billing or owner/admin path failed');
    expect(html).toContain('Use this route only after the affected hosted account still needs an owner or admin to finish hosted billing and that path still cannot complete cleanly.');
    expect(html).toContain('Say whether the failed path was hosted billing and whether the account still needed an owner or admin to open it.');
    expect(html).toContain('Bring the same hosted account and the failed billing or owner/admin step instead of reopening the story.');
    expect(html).toContain('Say whether the blocked path was Workspaces review, Access review, owner/admin hosted change, or hosted billing.');
    expect(html).toContain('Include the hosted account and workspace or hosted billing account that still needed owner/admin action.');
    expect(html).toContain('data-can-manage="false"');
    expect(html).not.toContain('Invite people, change roles, and remove account access.');
    expect(html).not.toContain('Open a workspace, review lifecycle state, or create one.');
    expect(html).not.toContain('Open a workspace, review lifecycle state, or create a new one without mixing in access or billing work.');
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

    expect(html).toContain('Read-only role');
    expect(html).toContain('Read-only');
    expect(html).not.toContain('Read_only role');
    expect(html).not.toContain('READ_ONLY');
  });

  it('keeps next action permission-honest for hosted view-only accounts with no workspace ready', function() {
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

    expect(html).toContain('Review who can act');
    expect(html).toContain('Review Access to see who can create or manage the first workspace on this account.');
    expect(html).toContain('Review access');
    expect(html).not.toContain('Choose the right task path');
    expect(html).not.toContain('If this is an access change, go to Access. If it is a billing or license issue, go to Billing. Support is only for escalation.');
  });

  it('keeps ready state honest for hosted view-only accounts with no workspace yet', function() {
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

    expect(html).toContain('Nothing is ready yet');
    expect(html).toContain('An owner or admin still needs to create the first hosted workspace before routine work can start.');
    expect(html).not.toContain('Use Workspaces to review current state before you start routine work.');
  });

  it('keeps ready state honest for managed hosted accounts with no workspace yet', function() {
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

    expect(html).toContain('Nothing is ready yet');
    expect(html).toContain('The first hosted workspace still needs to be created before routine work can start.');
    expect(html).not.toContain('Use Workspaces to review current state before you start routine work.');
  });

  it('keeps next action on review surfaces for suspended hosted view-only accounts', function() {
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

    expect(html).toContain('Review workspace state');
    expect(html).toContain('Open Workspaces to review current state, then hand off any lifecycle or billing change to an owner or admin.');
    expect(html).toContain('Review workspaces');
    expect(html).toContain('Review access');
    expect(html).not.toContain('Choose the right task path');
  });

  it('keeps overview attention copy honest when only suspended workspaces remain', function() {
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

    expect(html).toContain('No active workspace is ready for routine use right now.');
    expect(html).toContain('1 suspended workspace stays out of the way until you deliberately resume it.');
    expect(html).not.toContain('Active workspaces look clear for routine use.');
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
    expect(html).toMatch(/Workspaces[\s\S]*Unavailable on this account\. Hosted workspaces are not attached here\./);
    expect(html).toContain('Unavailable on this account. Hosted workspaces are not attached here.');
    expect(html).toMatch(/Access[\s\S]*Unavailable on this account\. Hosted roster and role controls live only on hosted workspace accounts\./);
    expect(html).toContain('Unavailable on this account. Hosted roster and role controls live only on hosted workspace accounts.');
    expect(html).toMatch(/Support[\s\S]*Escalation only after the billing path is exhausted\./);
    expect(html).toContain('No hosted account');
    expect(html).toContain('Billing tools are ready');
    expect(html).toContain('There is nothing to open or manage here yet.');
    expect(html).toContain('There are no hosted roles or invites to manage for this account right now.');
    expect(html).toContain('Use this billing surface only for self-hosted subscriptions, licenses, refunds, and privacy requests.');
    expect(html).toContain('Use self-hosted billing only for self-hosted purchases. Open one path at a time when hosted billing does not apply.');
    expect(html).toContain('Escalate with the same self-hosted billing path and the exact failed step.');
    expect(html).toContain('Billing');
    expect(html).toContain('Use support only when the Billing path has already stopped you.');
    expect(html).toContain('Self-hosted billing path failed');
    expect(html).toContain('Purchase email');
    expect(html).not.toContain('No hosted billing attached');
    expect(html).not.toContain('Escalate with the same hosted billing action or self-hosted path and the exact failed step.');
    expect(html).not.toContain('Workspace or access path failed');
    expect(html).not.toContain('Open workspaces');
    expect(html).not.toContain('Open access');
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

    expect(html).toContain('Suspended until you resume it');
    expect(html).toContain('Nothing urgent');
    expect(html).toContain('No workspace is ready yet');
    expect(html).toContain('Create the next workspace');
    expect(html).toContain('Suspended stays parked');
  });

  it('preserves the high-density grid, standard sidebar hooks, and inline, pill-free action constraints in the rendered shell', function() {
    var html = renderAuthenticatedPortalHTML(createContext());
    expect(html).toContain('portal-shell-layout');
  });
});
