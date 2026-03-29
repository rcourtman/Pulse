import { beforeEach, describe, expect, it } from 'vitest';

import { renderAccountUI, renderAddWorkspaceSection, renderAccessSection, renderWorkspaceManagement } from './account_view';
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
    accessVisible: false,
    activeAccessJob: '',
    accessQuery: {
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
    var defaultWorkspaces = [
      {
        id: 'ws_1',
        display_name: 'Alpha Workspace',
        state: 'active',
        healthy: true,
        health_status: 'healthy' as const,
        created_at: '2026-03-26T10:00:00Z',
      },
    ];
    return {
      id: 'acct_1',
      name: 'Acme MSP',
      kind: 'msp',
      kind_label: 'MSP',
      role: 'owner',
      can_manage: true,
      has_billing: true,
      ...overrides,
      members: overrides.members || [],
      workspaces: overrides.workspaces || defaultWorkspaces,
    };
  }

  function createWorkspaceManagementDOM(accountID: string): string {
    return (
      '<div id="workspace-operations-shell-' +
      accountID +
      '" class="workspace-operations-shell workspace-operations-shell-idle">' +
        '<div id="workspace-operations-detail-' +
        accountID +
        '" class="workspace-operations-detail workspace-operations-detail-idle" hidden>' +
          '<div id="workspace-management-' +
          accountID +
          '" class="workspace-management-panel" hidden>' +
            '<button id="workspace-management-close-' + accountID + '"></button>' +
            '<div id="workspace-management-empty-' + accountID + '"></div>' +
            '<div id="workspace-management-content-' + accountID + '" hidden>' +
              '<div id="workspace-management-meta-' + accountID + '"></div>' +
              '<h4 id="workspace-management-title-' + accountID + '"></h4>' +
              '<p id="workspace-management-summary-' + accountID + '"></p>' +
              '<div id="workspace-management-health-' + accountID + '"></div>' +
              '<div id="workspace-management-lifecycle-' + accountID + '"></div>' +
              '<div id="workspace-management-created-' + accountID + '"></div>' +
              '<div id="workspace-management-guidance-' + accountID + '"></div>' +
              '<button id="workspace-management-action-' + accountID + '"></button>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>'
    );
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

  it('renders access loading, error, and populated member states', function() {
    document.body.innerHTML =
      '<div id="access-section-acct_1" class="access-section" data-actor-role="owner" data-can-manage="true">' +
      '<div id="access-stats-acct_1"></div>' +
      '<table><tbody id="access-list-acct_1"></tbody></table>' +
      '</div>';

    renderAccessSection('acct_1', createEntry({
      accessVisible: true,
      accessQuery: { status: 'loading', error: '', data: [] },
    }));
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Loading roster');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Access');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Manage access');

    renderAccessSection('acct_1', createEntry({
      accessVisible: true,
      accessQuery: { status: 'error', error: 'Failed to load access roster.', data: [] },
    }));
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Failed to load roster');
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Failed to load access roster.');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Load failed');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Manage access');

    renderAccessSection(
      'acct_1',
      createEntry({
        accessVisible: true,
        accessQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u1' },
            { email: 'tech@example.com', role: 'tech', user_id: 'u2' },
          ],
        },
      })
    );
    expect(document.querySelector('[data-action="change-role"]')).toBeNull();
    expect(document.querySelector('[data-action="remove-member"]')).toBeNull();
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Operator');
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Role');
    expect(document.getElementById('access-list-acct_1')?.textContent).not.toContain('Review only');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Members');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('2');
  });

  it('renders access controls only for the active access job', function() {
    document.body.innerHTML =
      '<div id="access-section-acct_1" class="access-section" data-actor-role="owner" data-can-manage="true">' +
      '<div id="access-shell-acct_1"></div>' +
      '<div id="access-detail-acct_1"></div>' +
      '<div id="access-task-panel-acct_1"></div>' +
      '<div id="access-task-title-acct_1"></div>' +
      '<div id="access-task-copy-acct_1"></div>' +
      '<button id="access-task-invite-acct_1"></button>' +
      '<button id="access-task-change_role-acct_1"></button>' +
      '<button id="access-task-remove-acct_1"></button>' +
      '<div id="access-task-body-invite-acct_1"></div>' +
      '<div id="access-task-body-change_role-acct_1"></div>' +
      '<div id="access-task-body-remove-acct_1"></div>' +
      '<div id="access-stats-acct_1"></div>' +
      '<div id="access-list-acct_1"></div>' +
      '</div>';

    renderAccessSection(
      'acct_1',
      createEntry({
        accessVisible: true,
        activeAccessJob: 'change_role',
        accessQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u1' },
          ],
        },
      })
    );

    expect(document.querySelector('[data-action="change-role"]')).not.toBeNull();
    expect(document.querySelector('[data-action="remove-member"]')).toBeNull();
    expect(document.getElementById('access-task-title-acct_1')?.textContent).toContain('Change roles');
    expect(document.getElementById('access-detail-acct_1')?.hidden).toBe(false);

    renderAccessSection(
      'acct_1',
      createEntry({
        accessVisible: true,
        activeAccessJob: 'remove',
        accessQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u1' },
          ],
        },
      })
    );

    expect(document.querySelector('[data-action="change-role"]')).toBeNull();
    expect(document.querySelector('[data-action="remove-member"]')).not.toBeNull();
    expect(document.getElementById('access-task-title-acct_1')?.textContent).toContain('Remove access');
  });

  it('normalizes legacy member roles into the current read-only operator model', function() {
    document.body.innerHTML =
      '<div id="access-section-acct_1" class="access-section" data-actor-role="owner" data-can-manage="true">' +
      '<div id="access-stats-acct_1"></div>' +
      '<div id="access-list-acct_1"></div>' +
      '</div>';

    renderAccessSection(
      'acct_1',
      createEntry({
        accessVisible: true,
        activeAccessJob: 'change_role',
        accessQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'legacy@example.com', role: 'member', user_id: 'u_legacy' },
          ],
        },
      })
    );

    var roleSelect = document.querySelector('.access-role-select') as HTMLSelectElement;
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Read-only');
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Role');
    expect(roleSelect.value).toBe('read_only');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Operators');
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('1');
  });

  it('renders access roster as view-only when the account is not manageable', function() {
    document.body.innerHTML =
      '<div id="access-section-acct_1" class="access-section" data-actor-role="tech" data-can-manage="false">' +
      '<div id="access-stats-acct_1"></div>' +
      '<div id="access-list-acct_1"></div>' +
      '</div>';

    renderAccessSection(
      'acct_1',
      createEntry({
        accessVisible: true,
        accessQuery: {
          status: 'ready',
          error: '',
          data: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u1' },
            { email: 'tech@example.com', role: 'tech', user_id: 'u2' },
          ],
        },
      })
    );

    expect(document.querySelector('[data-action="change-role"]')).toBeNull();
    expect(document.querySelector('[data-action="remove-member"]')).toBeNull();
    expect(document.querySelector('.access-role-select')).toBeNull();
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Operator');
    expect(document.getElementById('access-list-acct_1')?.textContent).toContain('Role');
    expect(document.getElementById('access-list-acct_1')?.textContent).not.toContain('Action');
    expect(document.getElementById('access-list-acct_1')?.textContent).not.toContain('View only');
    expect(document.querySelector('.access-roster-head')?.classList.contains('access-roster-head-readonly')).toBe(true);
    expect(document.querySelector('.access-member-row')?.classList.contains('access-member-row-readonly')).toBe(true);
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Members');
  });

  it('hides workspace management when no workspace task is active', function() {
    document.body.innerHTML = createWorkspaceManagementDOM('acct_1');

    renderWorkspaceManagement(createAccount(), createEntry());
    expect(document.getElementById('workspace-management-acct_1')?.hidden).toBe(true);
    expect(document.getElementById('workspace-operations-detail-acct_1')?.hidden).toBe(true);
    expect(document.getElementById('workspace-operations-shell-acct_1')?.classList.contains('workspace-operations-shell-idle')).toBe(true);
  });

  it('renders workspace management selection from account UI state', function() {
    document.body.innerHTML = createWorkspaceManagementDOM('acct_1');

    renderWorkspaceManagement(createAccount(), createEntry({ selectedWorkspaceID: 'ws_1' }));
    expect(document.getElementById('workspace-management-acct_1')?.classList.contains('visible')).toBe(true);
    expect(document.getElementById('workspace-management-acct_1')?.hidden).toBe(false);
    expect(document.getElementById('workspace-operations-detail-acct_1')?.hidden).toBe(false);
    expect(document.getElementById('workspace-operations-shell-acct_1')?.classList.contains('workspace-operations-shell-selected')).toBe(true);
    expect(document.getElementById('workspace-operations-shell-acct_1')?.classList.contains('workspace-operations-shell-idle')).toBe(false);
    expect((document.getElementById('workspace-management-empty-acct_1') as HTMLElement).hidden).toBe(true);
    expect((document.getElementById('workspace-management-content-acct_1') as HTMLElement).hidden).toBe(false);
    expect(document.getElementById('workspace-management-title-acct_1')?.textContent).toContain('Alpha Workspace');
    expect(document.getElementById('workspace-management-health-acct_1')?.textContent).toContain('Healthy');
    expect(document.getElementById('workspace-management-action-acct_1')?.textContent).toContain('Suspend workspace');
  });

  it('renders account UI for every tracked account entry', function() {
    document.body.innerHTML =
      '<div id="add-ws-form-acct_1" class="add-workspace-form"></div><div id="ws-spinner-acct_1"></div>' +
      createWorkspaceManagementDOM('acct_1') +
      '<div id="access-section-acct_1" class="access-section" data-actor-role="admin" data-can-manage="true"><div id="access-stats-acct_1"></div><table><tbody id="access-list-acct_1"></tbody></table></div>' +
      '<div id="add-ws-form-acct_2" class="add-workspace-form"></div><div id="ws-spinner-acct_2"></div>' +
      createWorkspaceManagementDOM('acct_2') +
      '<div id="access-section-acct_2" class="access-section" data-actor-role="owner" data-can-manage="true"><div id="access-stats-acct_2"></div><table><tbody id="access-list-acct_2"></tbody></table></div>';

    var state: PortalAccountState = {
      byAccountID: {
        acct_1: createEntry({ addWorkspaceOpen: true, accessVisible: true }),
        acct_2: createEntry({
          addWorkspaceOpen: false,
          selectedWorkspaceID: 'ws_2',
          accessVisible: true,
          accessQuery: {
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
    expect(document.getElementById('workspace-management-acct_1')?.hidden).toBe(false);
    expect(document.getElementById('workspace-operations-shell-acct_1')?.classList.contains('workspace-operations-shell-idle')).toBe(false);
    expect(document.getElementById('workspace-operations-shell-acct_1')?.classList.contains('workspace-operations-shell-form-open')).toBe(true);
    expect(document.getElementById('workspace-management-acct_1')?.classList.contains('workspace-management-panel-idle')).toBe(true);
    expect(document.getElementById('access-list-acct_2')?.textContent).toContain('tech@example.com');
    expect(document.getElementById('workspace-management-acct_2')?.hidden).toBe(false);
    expect(document.getElementById('workspace-management-title-acct_2')?.textContent).toContain('Beta Workspace');
  });
});
