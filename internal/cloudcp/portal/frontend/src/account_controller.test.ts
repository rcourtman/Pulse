import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installAccountController } from './account_controller';

describe('account controller', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('routes account actions to the matching runtime handlers', function() {
    var runtime = {
      toggleAddWorkspace: vi.fn(),
      selectWorkspace: vi.fn(),
      clearWorkspaceSelection: vi.fn(),
      openBilling: vi.fn(),
      toggleAccess: vi.fn(),
      ensureAccessVisible: vi.fn(),
      inviteMember: vi.fn(),
      createWorkspace: vi.fn(),
      manageWorkspaceAction: vi.fn(),
      removeMember: vi.fn(),
      changeRole: vi.fn(),
    };

    var setShellSection = vi.fn();

    installAccountController({ runtime: runtime, setShellSection: setShellSection });

    document.body.innerHTML =
      '<button id="toggle" data-action="toggle-add-workspace" data-account-id="acct_1">Toggle</button>' +
      '<button id="billing" data-action="open-billing" data-account-id="acct_1">Billing</button>' +
      '<button id="team" data-action="show-access" data-account-id="acct_1">Team</button>' +
      '<button id="invite" data-action="invite-member" data-account-id="acct_1">Invite</button>' +
      '<button id="create" data-action="create-workspace" data-account-id="acct_1">Create</button>' +
      '<button id="select" data-action="select-workspace" data-account-id="acct_1" data-workspace-id="ws_1">Manage</button>' +
      '<button id="close" data-action="clear-workspace-selection" data-account-id="acct_1">Done</button>' +
      '<button id="manage" data-action="workspace-action" data-account-id="acct_1" data-workspace-id="ws_1" data-workspace-action="suspend" data-workspace-name="Alpha">Suspend</button>' +
      '<button id="remove" data-action="remove-member" data-account-id="acct_1" data-user-id="u1" data-member-email="owner@example.com">Remove</button>' +
      '<select id="role" data-action="change-role" data-account-id="acct_1" data-user-id="u1"><option value="admin">Admin</option></select>';

    document.getElementById('toggle')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(setShellSection).toHaveBeenCalledWith('workspaces');
    expect(runtime.toggleAddWorkspace).toHaveBeenCalledWith('acct_1');

    document.getElementById('billing')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.openBilling).toHaveBeenCalledWith('acct_1');

    document.getElementById('team')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(setShellSection).toHaveBeenCalledWith('access');
    expect(runtime.ensureAccessVisible).toHaveBeenCalledWith('acct_1');

    document.getElementById('invite')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.inviteMember).toHaveBeenCalledWith('acct_1');

    document.getElementById('create')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.createWorkspace).toHaveBeenCalledWith('acct_1');

    document.getElementById('select')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.selectWorkspace).toHaveBeenCalledWith('acct_1', 'ws_1');

    document.getElementById('close')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.clearWorkspaceSelection).toHaveBeenCalledWith('acct_1');

    document.getElementById('manage')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.manageWorkspaceAction).toHaveBeenCalledWith('acct_1', 'ws_1', 'suspend', 'Alpha');

    document.getElementById('remove')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(runtime.removeMember).toHaveBeenCalledWith('acct_1', 'u1', 'owner@example.com');

    var role = document.getElementById('role') as HTMLSelectElement;
    role.value = 'admin';
    role.dispatchEvent(new Event('change', { bubbles: true }));
    expect(runtime.changeRole).toHaveBeenCalledWith('acct_1', 'u1', 'admin');
  });
});
