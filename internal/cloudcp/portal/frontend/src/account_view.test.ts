import { beforeEach, describe, expect, it } from 'vitest';

import { renderAccountUI, renderAddWorkspaceSection, renderTeamSection, renderWorkspaceManagement } from './account_view';
import type { PortalAccountState, PortalAccountSummary, PortalAccountUIEntry } from './types';

function createEntry(overrides: Partial<PortalAccountUIEntry> = {}): PortalAccountUIEntry {
  return {
    addWorkspaceOpen: false,
    createWorkspace: {
      pending: false,
      error: '',
    },
    selectedWorkspaceID: '',
    manageWorkspace: {
      pending: false,
      error: '',
    },
    teamVisible: false,
    teamQuery: {
      status: 'idle',
      error: '',
      data: [],
    },
    ...overrides,
  };
}

describe('account view', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
  });

  function createAccount(overrides: Partial<PortalAccountSummary> = {}): PortalAccountSummary {
    return {
      id: 'acct_1',
      name: 'Acme MSP',
      kind: 'msp',
      kind_label: 'MSP',
      role: 'owner',
      can_manage: true,
      has_billing: true,
      workspaces: [
        {
          id: 'ws_1',
          display_name: 'Alpha Workspace',
          state: 'active',
          healthy: true,
          health_status: 'healthy',
          created_at: '2026-03-26T10:00:00Z',
        },
      ],
      ...overrides,
    };
  }

  it('renders add-workspace visibility from account UI state', function() {
    document.body.innerHTML =
      '<div id="add-ws-form-acct_1" class="add-workspace-form"></div>' +
      '<div id="ws-spinner-acct_1" hidden></div>';

    renderAddWorkspaceSection('acct_1', createEntry({ addWorkspaceOpen: true }));
    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(true);
    expect((document.getElementById('ws-spinner-acct_1') as HTMLElement).hidden).toBe(true);

    renderAddWorkspaceSection('acct_1', createEntry({
      addWorkspaceOpen: true,
      createWorkspace: { pending: true, error: '' },
    }));
    expect((document.getElementById('ws-spinner-acct_1') as HTMLElement).hidden).toBe(false);

    renderAddWorkspaceSection('acct_1', createEntry({ addWorkspaceOpen: false }));
    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(false);
  });

  it('renders team loading, error, and populated member states', function() {
    document.body.innerHTML =
      '<div id="team-section-acct_1" class="team-section" data-actor-role="owner">' +
      '<div id="team-stats-acct_1"></div>' +
      '<table><tbody id="team-list-acct_1"></tbody></table>' +
      '</div>';

    renderTeamSection('acct_1', createEntry({
      teamVisible: true,
      teamQuery: { status: 'loading', error: '', data: [] },
    }));
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Loading roster');
    expect(document.getElementById('team-stats-acct_1')?.textContent).toContain('Invites');

    renderTeamSection('acct_1', createEntry({
      teamVisible: true,
      teamQuery: { status: 'error', error: 'Failed to load team.', data: [] },
    }));
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Roster needs attention');
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Failed to load team.');
    expect(document.getElementById('team-stats-acct_1')?.textContent).toContain('Invite only');

    renderTeamSection(
      'acct_1',
      createEntry({
        teamVisible: true,
        teamQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u1' },
            { email: 'tech@example.com', role: 'tech', user_id: 'u2' },
          ],
        },
      })
    );
    expect(document.querySelector('[data-action="change-role"]')).not.toBeNull();
    expect(document.querySelector('[data-action="remove-member"]')).not.toBeNull();
    expect(document.getElementById('team-stats-acct_1')?.textContent).toContain('Members');
    expect(document.getElementById('team-stats-acct_1')?.textContent).toContain('2');
  });

  it('normalizes legacy member roles into the current read-only operator model', function() {
    document.body.innerHTML =
      '<div id="team-section-acct_1" class="team-section" data-actor-role="owner">' +
      '<div id="team-stats-acct_1"></div>' +
      '<div id="team-list-acct_1"></div>' +
      '</div>';

    renderTeamSection(
      'acct_1',
      createEntry({
        teamVisible: true,
        teamQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'legacy@example.com', role: 'member', user_id: 'u_legacy' },
          ],
        },
      })
    );

    var roleSelect = document.querySelector('.team-role-select') as HTMLSelectElement;
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Read-only');
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Role');
    expect(roleSelect.value).toBe('read_only');
    expect(document.getElementById('team-stats-acct_1')?.textContent).toContain('Operators');
    expect(document.getElementById('team-stats-acct_1')?.textContent).toContain('1');
  });

  it('renders workspace management selection from account UI state', function() {
    document.body.innerHTML =
      '<div id="workspace-management-acct_1" class="workspace-management-panel">' +
      '<button id="workspace-management-close-acct_1"></button>' +
      '<div id="workspace-management-empty-acct_1"></div>' +
      '<div id="workspace-management-content-acct_1" hidden>' +
      '<div id="workspace-management-meta-acct_1"></div>' +
      '<h4 id="workspace-management-title-acct_1"></h4>' +
      '<p id="workspace-management-summary-acct_1"></p>' +
      '<div id="workspace-management-health-acct_1"></div>' +
      '<div id="workspace-management-lifecycle-acct_1"></div>' +
      '<div id="workspace-management-created-acct_1"></div>' +
      '<div id="workspace-management-guidance-acct_1"></div>' +
      '<button id="workspace-management-action-acct_1"></button>' +
      '</div>' +
      '</div>';

    renderWorkspaceManagement(createAccount(), createEntry({ selectedWorkspaceID: 'ws_1' }));
    expect(document.getElementById('workspace-management-acct_1')?.classList.contains('visible')).toBe(true);
    expect((document.getElementById('workspace-management-empty-acct_1') as HTMLElement).hidden).toBe(true);
    expect((document.getElementById('workspace-management-content-acct_1') as HTMLElement).hidden).toBe(false);
    expect(document.getElementById('workspace-management-title-acct_1')?.textContent).toContain('Alpha Workspace');
    expect(document.getElementById('workspace-management-health-acct_1')?.textContent).toContain('Healthy');
    expect(document.getElementById('workspace-management-action-acct_1')?.textContent).toContain('Suspend workspace');
  });

  it('renders account UI for every tracked account entry', function() {
    document.body.innerHTML =
      '<div id="add-ws-form-acct_1" class="add-workspace-form"></div><div id="ws-spinner-acct_1"></div>' +
      '<div id="workspace-management-acct_1" class="workspace-management-panel"><button id="workspace-management-close-acct_1"></button><div id="workspace-management-empty-acct_1"></div><div id="workspace-management-content-acct_1" hidden><div id="workspace-management-meta-acct_1"></div><h4 id="workspace-management-title-acct_1"></h4><p id="workspace-management-summary-acct_1"></p><div id="workspace-management-health-acct_1"></div><div id="workspace-management-lifecycle-acct_1"></div><div id="workspace-management-created-acct_1"></div><div id="workspace-management-guidance-acct_1"></div><button id="workspace-management-action-acct_1"></button></div></div>' +
      '<div id="team-section-acct_1" class="team-section" data-actor-role="admin"><div id="team-stats-acct_1"></div><table><tbody id="team-list-acct_1"></tbody></table></div>' +
      '<div id="add-ws-form-acct_2" class="add-workspace-form"></div><div id="ws-spinner-acct_2"></div>' +
      '<div id="workspace-management-acct_2" class="workspace-management-panel"><button id="workspace-management-close-acct_2"></button><div id="workspace-management-empty-acct_2"></div><div id="workspace-management-content-acct_2" hidden><div id="workspace-management-meta-acct_2"></div><h4 id="workspace-management-title-acct_2"></h4><p id="workspace-management-summary-acct_2"></p><div id="workspace-management-health-acct_2"></div><div id="workspace-management-lifecycle-acct_2"></div><div id="workspace-management-created-acct_2"></div><div id="workspace-management-guidance-acct_2"></div><button id="workspace-management-action-acct_2"></button></div></div>' +
      '<div id="team-section-acct_2" class="team-section" data-actor-role="owner"><div id="team-stats-acct_2"></div><table><tbody id="team-list-acct_2"></tbody></table></div>';

    var state: PortalAccountState = {
      byAccountID: {
        acct_1: createEntry({ addWorkspaceOpen: true, teamVisible: true }),
        acct_2: createEntry({
          addWorkspaceOpen: false,
          selectedWorkspaceID: 'ws_2',
          teamVisible: true,
          teamQuery: {
            status: 'ready',
            error: '',
            data: [{ email: 'tech@example.com', role: 'tech', user_id: 'u2' }],
          },
        }),
      },
    };

    renderAccountUI(state, [
      createAccount(),
      createAccount({
        id: 'acct_2',
        name: 'Beta MSP',
        workspaces: [
          {
            id: 'ws_2',
            display_name: 'Beta Workspace',
            state: 'failed',
            healthy: false,
            health_status: 'unhealthy',
          },
        ],
      }),
    ]);

    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(true);
    expect(document.getElementById('add-ws-form-acct_2')?.classList.contains('visible')).toBe(false);
    expect(document.getElementById('team-list-acct_2')?.textContent).toContain('tech@example.com');
    expect(document.getElementById('workspace-management-title-acct_2')?.textContent).toContain('Beta Workspace');
  });
});
