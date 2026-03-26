import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import { installAccountController } from './account_controller';

function jsonResponse(payload: unknown, ok = true, status = ok ? 200 : 500) {
  return {
    ok,
    status,
    json: vi.fn().mockResolvedValue(payload),
    text: vi.fn().mockResolvedValue(typeof payload === 'string' ? payload : JSON.stringify(payload)),
  };
}

async function flushAsync() {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await new Promise(function(resolve) {
    setTimeout(resolve, 0);
  });
}

const deps = {
  getAccountAPIBasePath: vi.fn(),
  getPortalAPIBasePath: vi.fn(),
  getPortalPath: vi.fn(),
  refreshBootstrap: vi.fn(),
  showToast: vi.fn(),
};

describe('account controller', function() {
  beforeAll(function() {
    installAccountController(deps);
  });

  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    deps.getAccountAPIBasePath = vi.fn().mockReturnValue('/api/accounts');
    deps.getPortalAPIBasePath = vi.fn().mockReturnValue('/api/portal');
    deps.getPortalPath = vi.fn().mockReturnValue('/portal');
    deps.refreshBootstrap = vi.fn().mockResolvedValue(true);
    deps.showToast = vi.fn();
  });

  it('creates a workspace through the managed account action flow', async function() {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ tenant_id: 't_123' })));

    document.body.innerHTML =
      '<button id="toggle" data-action="toggle-add-workspace" data-account-id="acct_1">Toggle</button>' +
      '<div id="add-ws-form-acct_1" class="add-workspace-form">' +
      '<input id="ws-name-acct_1" value="Acme Corp">' +
      '<div id="ws-spinner-acct_1" style="display:none"></div>' +
      '<button id="create" data-action="create-workspace" data-account-id="acct_1">Create</button>' +
      '</div>';

    document.getElementById('toggle')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(true);

    document.getElementById('create')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushAsync();

    expect(fetch).toHaveBeenCalledWith(
      '/api/accounts/acct_1/tenants',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ display_name: 'Acme Corp' }),
      })
    );
    expect(deps.refreshBootstrap).toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith('Workspace created!');
    expect((document.getElementById('ws-spinner-acct_1') as HTMLElement).style.display).toBe('none');
  });

  it('loads and updates team membership from delegated team actions', async function() {
    vi.stubGlobal(
      'fetch',
      vi.fn()
        .mockResolvedValueOnce(jsonResponse([{ email: 'owner@example.com', role: 'owner', user_id: 'u1' }]))
        .mockResolvedValueOnce(jsonResponse({}))
        .mockResolvedValueOnce(jsonResponse([{ email: 'owner@example.com', role: 'admin', user_id: 'u1' }]))
    );

    document.body.innerHTML =
      '<button id="team-toggle" data-action="toggle-team" data-account-id="acct_1">Team</button>' +
      '<div id="team-section-acct_1" class="team-section" data-actor-role="owner">' +
      '<table><tbody id="team-list-acct_1"></tbody></table>' +
      '<input id="invite-email-acct_1">' +
      '<select id="invite-role-acct_1"><option value="admin">Admin</option></select>' +
      '</div>';

    document.getElementById('team-toggle')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushAsync();

    var roleSelect = document.querySelector('[data-action="change-role"]') as HTMLSelectElement | null;
    expect(roleSelect).not.toBeNull();
    expect(roleSelect?.value).toBe('owner');

    roleSelect!.value = 'admin';
    roleSelect!.dispatchEvent(new Event('change', { bubbles: true }));
    await flushAsync();

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/accounts/acct_1/members');
    expect(fetch).toHaveBeenNthCalledWith(
      2,
      '/api/accounts/acct_1/members/u1',
      expect.objectContaining({
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role: 'admin' }),
      })
    );
    expect(deps.refreshBootstrap).toHaveBeenCalled();
    expect(deps.showToast).toHaveBeenCalledWith('Role updated.');
  });
});
