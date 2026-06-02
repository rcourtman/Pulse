import { describe, expect, it } from 'vitest';

import type { PortalBillingState, PortalBootstrapData, PortalLoginState } from './types';
import { createPortalBillingState } from './state';
import {
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
  renderWorkspaceSummarySection,
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

function createBillingState(overrides: Partial<PortalBillingState> = {}): PortalBillingState {
  return {
    ...createPortalBillingState(),
    ...overrides,
  };
}

function createContext(overrides: Partial<ShellViewContext> = {}): ShellViewContext {
  return {
    bootstrap: createBootstrap(),
    billingState: createBillingState(),
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

  it('hides create-account actions when signup is unavailable', function() {
    var context = createContext({
      bootstrap: createBootstrap({ authenticated: false, email: '', signup_path: '' }),
      signupPath: '',
    });

    expect(renderHeaderHTML(context)).toBe('');
    expect(renderSignedOutPortalHTML(context)).not.toContain('Create an account');
    expect(renderSignedOutPortalHTML(context)).not.toContain('Create account');
  });

  it('renders workspace summary inside the canonical workspaces shell', function() {
    var html = renderWorkspaceSummarySection(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_summary',
              name: 'Summary Account',
              kind: 'msp',
              kind_label: 'MSP',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              members: [],
              setup_templates: [{
                id: 'standard-client-onboarding',
                title: 'Standard client onboarding',
                agent_naming: 'Workspace boundary keeps repeated hostnames separate.',
                alert_routing: 'Enable one route per client.',
                reporting: 'Schedule one report per client.',
                access: 'Invite staff from Access.',
              }],
              workspaces: [
                {
                  id: 'ws_summary',
                  display_name: 'Summary Workspace',
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

    expect(html).toContain('workspace-summary-shell');
    expect(html).toContain('workspace-summary-facts');
    expect(html).toContain('Next:</strong> Set up Summary Workspace');
    expect(html).toContain('Clients in setup');
    expect(html).toContain('Unknown agents');
    expect(html).toContain('Standard client onboarding');
    expect(html).toContain('Workspace boundary keeps repeated hostnames separate.');
    expect(html).toContain('1 client in setup');
    expect(html).toContain('Summary Workspace');
    expect(html).not.toContain('overview-task-grid');
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
              setup_templates: [{
                id: 'standard-client-onboarding',
                title: 'Standard client onboarding',
                agent_naming: 'Client workspace is the identity boundary.',
                alert_routing: 'Enabled route required.',
                reporting: 'Enabled report schedule required.',
                access: 'Provider staff use Access.',
              }],
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
    expect(html).toContain('Access');
    expect(html).toContain('Billing');
    expect(html).toContain('Support');
    expect(html).toContain('Clients');
    expect(html).toContain('id="billing-section"');
    expect(html).toContain('portal-identity-bar');
    expect(html).toContain('Owner');
    expect(html).toContain('MSP account');
    expect(html).toContain('Acme MSP');
    expect(html).toContain('workspace-summary-facts');
    expect(html).toContain('1 account');
    expect(html).toContain('3 clients');
    expect(html).toContain('0 ready clients');
    expect(html).toContain('1 client in setup');
    expect(html).toContain('2 clients to review');
    expect(html).toContain('Next:</strong> Review Beta Workspace');
    expect(html).toContain('Clients in setup');
    expect(html).toContain('Provider setup template');
    expect(html).toContain('Client workspace is the identity boundary.');
    expect(html).toContain('Add client');
    expect(html).not.toContain('Create workspace');
    expect(html).not.toContain('Manage billing');
    expect(html).not.toContain('Manage team');
    expect(html).toContain('Alpha Workspace');
    expect(html).toContain('Beta Workspace');
    expect(html).toContain('Gamma Workspace');
    expect(html).toContain('Unhealthy</span>');
    expect(html).toContain('Checking</span>');
    expect(html).toContain('This client is in a failed state.');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff');
    expect(html).toContain('/api/accounts/acct_1/tenants/ws_active/handoff?target_path=%2Fsettings%2Finfrastructure%3Fadd%3Dlinux-host');
    expect(html).toContain('Open client');
    expect(html).toContain('Install agents');
    expect(html).toContain('Configure alert routes');
    expect(html).toContain('Schedule reports');
    expect(html).toContain('Open the workspace-bound install commands.');
    expect(html).toContain('Alerts and performance reports stay inside the client workspace.');
    expect(html).toContain('data-action="select-workspace"');
    expect(html).toContain('Add a client');
    expect(html).toContain('Access changes stay in Access. Billing changes stay in Billing.');
    expect(html).toContain('Close panel');
    expect(html).toContain('id="workspace-management-acct_1" hidden');
    expect(html).toContain('id="workspace-operations-detail-acct_1" hidden');
    expect(html).toContain('data-action="clear-workspace-selection"');
    expect(html).toContain('Invite people');
    expect(html).toContain('Change roles');
    expect(html).toContain('Remove access');
    expect(html).toContain('data-action="set-access-job"');
    expect(html).toContain('id="access-detail-acct_1" hidden');
    expect(html).toContain('Choose the smallest role');
    expect(html).toContain('Full account, billing, and access control.');
    expect(html).toContain('Client control, billing, and roster management.');
    expect(html).toContain('Client control without billing or roster ownership.');
    expect(html).toContain('Review client status without control-plane changes.');
    expect(html).not.toContain('Workspace control, billing, and roster management.');
    expect(html).toContain('data-can-manage="true"');
    expect(html).toContain('Remove stale access');
    expect(html).toContain('data-action="workspace-action"');
    expect(html).toContain('Hosted billing');
    expect(html).toContain('Try first');
    expect(html).toContain('Scope');
    expect(html).toContain('Include');
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
    expect(html).not.toContain('section-context-strip');
    expect(html).not.toContain('View roster');
    expect(html).not.toContain('Owner or admin required');
    expect(html).not.toContain('portal-content-panel-overview');
    expect(html).not.toContain('data-shell-section="overview"');
  });

  it('omits hosted billing surfaces for provider-hosted MSP accounts without billing records', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_provider',
              name: 'Provider MSP',
              kind: 'msp',
              kind_label: 'MSP',
              role: 'owner',
              can_manage: true,
              has_billing: false,
              members: [],
              workspaces: [],
            },
          ],
        }),
      })
    );

    expect(html).toContain('data-shell-section="workspaces">Clients</button>');
    expect(html).toContain('data-shell-section="access">Access</button>');
    expect(html).toContain('data-shell-section="support">Support</button>');
    expect(html).not.toContain('data-shell-section="billing"');
    expect(html).not.toContain('id="billing-section"');
    expect(html).not.toContain('hosted billing');
    expect(html).not.toContain('Full account, billing, and access control.');
    expect(html).not.toContain('without billing');
    expect(html).not.toContain('Billing changes stay in Billing.');
    expect(html).toContain('Access changes stay in Access. Client runtime changes stay inside the client workspace.');
    expect(html).toContain('Retry the same Clients or Access step before you escalate.');
    expect(html).toContain('Clients or Access.');
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
    expect(html).toContain('id="open-upgrade-billing"');
    expect(html).toContain('id="open-retrieve-billing"');
    expect(html).toContain('id="billing-detail-shell" hidden');
    expect(html).toContain('data-account-billing-action="clear-billing-panel"');
    expect(html).toContain('id="upgrade-billing-panel"');
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

  it('labels mixed account surfaces and keeps MSP client copy scoped', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          accounts: [
            {
              id: 'acct_msp_mixed',
              name: 'Provider Account',
              kind: 'msp',
              kind_label: 'MSP',
              role: 'owner',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_client',
                  display_name: 'Acme Dental',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                },
              ],
            },
            {
              id: 'acct_cloud_mixed',
              name: 'Hosted Ops',
              kind: 'cloud',
              kind_label: 'Cloud',
              role: 'admin',
              can_manage: true,
              has_billing: true,
              members: [],
              workspaces: [
                {
                  id: 'ws_ops',
                  display_name: 'Operations Workspace',
                  state: 'active',
                  healthy: true,
                  health_status: 'healthy',
                  agent_count: 1,
                  alert_route_count: 1,
                  report_schedule_count: 1,
                },
              ],
            },
          ],
        }),
      })
    );

    expect(html).toContain('data-shell-section="workspaces">Workspaces</button>');
    expect(html).toContain('<h3>Provider Account</h3>');
    expect(html).toContain('MSP account · Owner');
    expect(html).toContain('<h3>Hosted Ops</h3>');
    expect(html).toContain('Cloud account · Admin');
    expect(html).toContain('data-client-language="true"');
    expect(html).toContain('data-client-language="false"');
    expect(html).toContain('Add client');
    expect(html).toContain('Open client');
    expect(html).toContain('Open workspace');
    expect(html).toContain('Client control, billing, and roster management.');
    expect(html).toContain('Workspace control, billing, and roster management.');
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
    expect(html).toContain('Review hosted workspace health here and open ready workspaces. An owner or admin must handle setup and workspace changes.');
    expect(html).toContain('Review who has access. An owner or admin must make changes.');
    expect(html).toContain('An owner or admin on this account needs to open hosted billing.');
    expect(html).toContain('Try first');
    expect(html).toContain('Scope');
    expect(html).toContain('Include');
    expect(html).toContain('data-can-manage="false"');
    expect(html).toContain('Open workspace');
    expect(html).not.toContain('/settings/infrastructure?add=linux-host');
    expect(html).not.toContain('>Install agents</button>');
    expect(html).not.toContain('Invite people, change roles, and remove account access.');
    expect(html).not.toContain('data-action="invite-member"');
    expect(html).not.toContain('data-action="set-access-job"');
    expect(html).not.toContain('Self-hosted billing, licenses, refunds, and privacy stay in Billing.');
    expect(html).not.toContain('Use support only when the Workspaces, Access, or hosted Billing path has already stopped you.');
    expect(html).not.toContain('Workspace or access path failed');
    expect(html).not.toContain('use Lifecycle only when an account-level change is required');
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
    expect(html).toContain('data-can-manage="false"');
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

    expect(html).toContain('No hosted workspaces are attached yet. An owner or admin must create the first one.');
    expect(html).toContain('data-can-manage="false"');
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

    expect(html).toContain('No clients yet. Add one to get started.');
    expect(html).toContain('Add client');
    expect(html).not.toContain('No hosted workspaces yet. Create one to get started.');
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
    expect(html).toContain('Suspended');
    expect(html).toContain('data-can-manage="false"');
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

    expect(html).toContain('Suspended');
    expect(html).toContain('Paused Workspace');
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
    expect(html).not.toContain('data-shell-section="workspaces"');
    expect(html).not.toContain('data-shell-section="access"');
    expect(html).toContain('Self-hosted billing');
    expect(html).toContain('Use self-hosted billing only for self-hosted purchases.');
    expect(html).toContain('Upgrade self-hosted plan');
    expect(html).toContain('Escalate with the same self-hosted billing path and the exact failed step.');
    expect(html).toContain('Billing');
    expect(html).not.toContain('Escalate with the same hosted billing action or self-hosted path and the exact failed step.');
    expect(html).not.toContain('Workspace or access path failed');
    expect(html).not.toContain('Hosted workspace or access');
    expect(html).not.toContain('Self-hosted commercial services');
  });

  it('renders a portal-owned self-hosted upgrade path when the app hands off an upgrade intent', function() {
    var html = renderAuthenticatedPortalHTML(
      createContext({
        bootstrap: createBootstrap({
          has_self_hosted_commercial: false,
          accounts: [
            {
              id: 'acct_hosted',
              name: 'Hosted Account',
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
        billingState: createBillingState({
          openBillingPanelID: 'upgrade-billing-panel',
          upgradeFeatureKey: 'self_hosted_plan',
        }),
        activeSection: 'billing',
      })
    );

    expect(html).toContain('Pulse Account owns the commercial handoff for self-hosted upgrades from the app.');
    expect(html).toContain('Compare self-hosted plans');
    expect(html).toContain(
      'Compare self-hosted plans as monitor, reach, or operate instead of by monitored-system volume.',
    );
    expect(html).toContain('Plan comparison');
    expect(html).toContain('id="upgrade-billing-root"');
    expect(html).toContain(
      'Choose the self-hosted tier that fits how you run Pulse: Community monitors, Relay reaches anywhere, and Pro investigates and helps fix issues. Pulse Account will send completed checkout directly back to the Plans page in Pulse.',
    );
    expect(html).toContain('Pulse Account owns self-hosted plan selection and checkout for self-hosted upgrades.');
    expect(html).not.toContain('id="open-manage-billing"');
    expect(html).not.toContain('id="open-retrieve-billing"');
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
    expect(html).toContain('This client is suspended.');
    expect(html).not.toContain('This workspace is suspended.');
    expect(html).toContain('Add client');
    expect(html).toContain('Paused Workspace');
  });

  it('preserves the simplified top-tab shell hooks in the rendered shell', function() {
    var html = renderAuthenticatedPortalHTML(createContext());
    expect(html).toContain('portal-shell-main');
    expect(html).toContain('portal-tab-bar');
    expect(html).not.toContain('portal-shell-layout');
  });
});
