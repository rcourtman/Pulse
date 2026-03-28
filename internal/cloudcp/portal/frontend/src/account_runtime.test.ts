import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import { createPortalAPI } from './api';
import { installAccountRuntime } from './account_runtime';
import { createPortalStore } from './store';
import type { PortalBootstrapData } from './types';

function jsonResponse(payload: unknown, ok = true, status = ok ? 200 : 500) {
  return {
    ok,
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
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

const bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> = {
  public_site_url: 'https://pulserelay.pro',
  support_email: 'support@pulserelay.pro',
  commercial_api_base_url: '/api/portal/commercial',
  portal_path: '/portal',
  bootstrap_path: '/api/portal/bootstrap',
  magic_link_request_path: '/api/public/magic-link/request',
  signup_path: '/signup',
  logout_path: '/auth/logout',
  account_api_base_path: '/api/accounts',
  portal_api_base_path: '/api/portal',
};

const deps = {
  api: null as any,
  store: createPortalStore(bootstrapDefaults, {
    authenticated: true,
    email: 'owner@example.com',
  }),
  refreshBootstrap: vi.fn(),
  showToast: vi.fn(),
};

describe('account runtime', function() {
  var runtime: ReturnType<typeof installAccountRuntime>;

  beforeAll(function() {
    deps.api = createPortalAPI({
      getBootstrap: function() {
        return deps.store.getBootstrap();
      },
    });
    runtime = installAccountRuntime(deps);
  });

  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    deps.store.setBootstrap({
      authenticated: true,
      email: 'owner@example.com',
    });
    deps.store.updateAccountState(function(accountState) {
      accountState.byAccountID = {};
    }, { notify: false });
    deps.refreshBootstrap = vi.fn().mockResolvedValue(true);
    deps.showToast = vi.fn();
  });

  it('creates a workspace through the managed account action flow', async function() {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ tenant_id: 't_123' })));

    document.body.innerHTML =
      '<div id="add-ws-form-acct_1" class="add-workspace-form">' +
      '<input id="ws-name-acct_1" value="Acme Corp">' +
      '<div id="ws-spinner-acct_1" hidden></div>' +
      '<div id="workspace-management-acct_1" class="workspace-management-panel"><button id="workspace-management-close-acct_1"></button><div id="workspace-management-empty-acct_1"></div><div id="workspace-management-content-acct_1" hidden><div id="workspace-management-meta-acct_1"></div><h4 id="workspace-management-title-acct_1"></h4><p id="workspace-management-summary-acct_1"></p><div id="workspace-management-health-acct_1"></div><div id="workspace-management-lifecycle-acct_1"></div><div id="workspace-management-created-acct_1"></div><div id="workspace-management-guidance-acct_1"></div><button id="workspace-management-action-acct_1"></button></div></div>' +
      '</div>';

    runtime.toggleAddWorkspace('acct_1');
    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(true);
    expect(deps.store.getAccountState().byAccountID.acct_1.addWorkspaceOpen).toBe(true);
    expect(deps.store.getAccountState().byAccountID.acct_1.createWorkspace.pending).toBe(false);

    await runtime.createWorkspace('acct_1');
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
    expect(deps.store.getAccountState().byAccountID.acct_1.addWorkspaceOpen).toBe(false);
    expect(deps.store.getAccountState().byAccountID.acct_1.createWorkspace.pending).toBe(false);
    expect((document.getElementById('ws-spinner-acct_1') as HTMLElement).hidden).toBe(true);
  });

  it('selects a workspace and routes explicit workspace actions through the management panel', async function() {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({})));
    var confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    deps.store.setBootstrap({
      authenticated: true,
      email: 'owner@example.com',
      accounts: [{
        id: 'acct_1',
        name: 'Acme MSP',
        kind: 'msp',
        kind_label: 'MSP',
        role: 'owner',
        can_manage: true,
        has_billing: true,
        workspaces: [{
          id: 'ws_2',
          display_name: 'Alpha Workspace',
          state: 'active',
          healthy: true,
          health_status: 'healthy',
        }],
      }],
    });

    document.body.innerHTML =
      '<div id="workspace-operations-shell-acct_1" class="workspace-operations-shell workspace-operations-shell-idle">' +
      '<div id="workspace-operations-detail-acct_1" class="workspace-operations-detail workspace-operations-detail-idle">' +
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
      '</div>' +
      '</div>' +
      '</div>';

    deps.store.updateAccountState(function(accountState) {
      var entry = accountState.byAccountID.acct_1 || (accountState.byAccountID.acct_1 = {
        addWorkspaceOpen: true,
        createWorkspace: { pending: false, error: '' },
        selectedWorkspaceID: '',
        manageWorkspace: { pending: false, error: '' },
        accessVisible: true,
        accessQuery: { status: 'idle', error: '', data: [] },
      });
      entry.addWorkspaceOpen = true;
      entry.accessVisible = true;
    }, { notify: false });

    runtime.selectWorkspace('acct_1', 'ws_2');
    expect(deps.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('ws_2');
    expect(deps.store.getAccountState().byAccountID.acct_1.accessVisible).toBe(false);
    expect(deps.store.getAccountState().byAccountID.acct_1.addWorkspaceOpen).toBe(false);
    expect(document.getElementById('workspace-operations-shell-acct_1')?.classList.contains('workspace-operations-shell-selected')).toBe(true);
    expect(document.getElementById('workspace-management-title-acct_1')?.textContent).toContain('Alpha Workspace');
    expect(document.getElementById('workspace-management-action-acct_1')?.textContent).toContain('Suspend workspace');

    await runtime.manageWorkspaceAction('acct_1', 'ws_2', 'suspend', 'Alpha Workspace');
    await flushAsync();

    expect(confirmSpy).toHaveBeenCalledWith('Suspend workspace "Alpha Workspace"?');
    expect(fetch).toHaveBeenCalledWith(
      '/api/accounts/acct_1/tenants/ws_2',
      expect.objectContaining({
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ state: 'suspended' }),
      })
    );
    expect(deps.showToast).toHaveBeenCalledWith('Suspended workspace.');
    expect(deps.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('');
  });

  it('reveals the lifecycle panel when a workspace job opens below the viewport', function() {
    deps.store.setBootstrap({
      authenticated: true,
      email: 'owner@example.com',
      accounts: [{
        id: 'acct_1',
        name: 'Acme MSP',
        kind: 'msp',
        kind_label: 'MSP',
        role: 'owner',
        can_manage: true,
        has_billing: true,
        workspaces: [{
          id: 'ws_2',
          display_name: 'Alpha Workspace',
          state: 'active',
          healthy: true,
          health_status: 'healthy',
        }],
      }],
    });

    document.body.innerHTML =
      '<div id="workspace-operations-shell-acct_1" class="workspace-operations-shell workspace-operations-shell-idle">' +
      '<div id="workspace-operations-detail-acct_1" class="workspace-operations-detail workspace-operations-detail-idle">' +
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
      '</div>' +
      '</div>' +
      '</div>';

    var panel = document.getElementById('workspace-management-acct_1') as HTMLElement | null;
    expect(panel).not.toBeNull();
    if (!panel) return;
    var scrollIntoView = vi.fn();
    var requestAnimationFrame = vi.fn(function(callback: FrameRequestCallback) {
      callback(0);
      return 1;
    });
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 664 });
    Object.defineProperty(window, 'requestAnimationFrame', { configurable: true, value: requestAnimationFrame });
    Object.defineProperty(panel, 'scrollIntoView', { configurable: true, value: scrollIntoView });
    Object.defineProperty(panel, 'getBoundingClientRect', {
      configurable: true,
      value: function() {
        return {
          top: 764,
          bottom: 1433,
          left: 0,
          right: 320,
          width: 320,
          height: 669,
          x: 0,
          y: 764,
          toJSON: function() { return {}; },
        };
      },
    });

    runtime.selectWorkspace('acct_1', 'ws_2');

    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', inline: 'nearest' });
    expect(deps.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('ws_2');
  });

  it('reveals the create workspace form when it opens below the viewport', function() {
    document.body.innerHTML =
      '<div id="workspace-operations-shell-acct_1" class="workspace-operations-shell workspace-operations-shell-idle">' +
      '<div id="workspace-operations-detail-acct_1" class="workspace-operations-detail workspace-operations-detail-idle">' +
      '<div id="workspace-management-acct_1" class="workspace-management-panel">' +
      '<button id="workspace-management-close-acct_1"></button>' +
      '<div id="workspace-management-empty-acct_1">' +
      '<div id="add-ws-form-acct_1" class="add-workspace-form">' +
      '<input id="ws-name-acct_1" value="">' +
      '</div>' +
      '</div>' +
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
      '</div>' +
      '</div>' +
      '</div>';

    var panel = document.getElementById('workspace-management-acct_1') as HTMLElement | null;
    var shell = document.getElementById('workspace-operations-shell-acct_1') as HTMLElement | null;
    expect(panel).not.toBeNull();
    if (!panel || !shell) return;
    var scrollIntoView = vi.fn();
    var requestAnimationFrame = vi.fn(function(callback: FrameRequestCallback) {
      callback(0);
      return 1;
    });
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 664 });
    Object.defineProperty(window, 'requestAnimationFrame', { configurable: true, value: requestAnimationFrame });
    Object.defineProperty(panel, 'scrollIntoView', { configurable: true, value: scrollIntoView });
    Object.defineProperty(panel, 'getBoundingClientRect', {
      configurable: true,
      value: function() {
        return {
          top: 1324,
          bottom: 1517,
          left: 0,
          right: 320,
          width: 320,
          height: 193,
          x: 0,
          y: 1324,
          toJSON: function() { return {}; },
        };
      },
    });

    runtime.toggleAddWorkspace('acct_1');

    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', inline: 'nearest' });
    expect(deps.store.getAccountState().byAccountID.acct_1.addWorkspaceOpen).toBe(true);
  });

  it('loads and updates team membership from runtime actions', async function() {
    vi.stubGlobal(
      'fetch',
      vi.fn()
        .mockResolvedValueOnce(jsonResponse([{ email: 'owner@example.com', role: 'owner', user_id: 'u1' }]))
        .mockResolvedValueOnce(jsonResponse({}))
        .mockResolvedValueOnce(jsonResponse([{ email: 'owner@example.com', role: 'admin', user_id: 'u1' }]))
    );

    document.body.innerHTML =
      '<div id="workspace-management-acct_1" class="workspace-management-panel"><button id="workspace-management-close-acct_1"></button><div id="workspace-management-empty-acct_1"></div><div id="workspace-management-content-acct_1" hidden><div id="workspace-management-meta-acct_1"></div><h4 id="workspace-management-title-acct_1"></h4><p id="workspace-management-summary-acct_1"></p><div id="workspace-management-health-acct_1"></div><div id="workspace-management-lifecycle-acct_1"></div><div id="workspace-management-created-acct_1"></div><div id="workspace-management-guidance-acct_1"></div><button id="workspace-management-action-acct_1"></button></div></div>' +
      '<div id="access-section-acct_1" class="access-section" data-actor-role="owner" data-can-manage="true">' +
      '<div id="access-stats-acct_1"></div>' +
      '<table><tbody id="access-list-acct_1"></tbody></table>' +
      '<input id="invite-email-acct_1">' +
      '<select id="invite-role-acct_1"><option value="admin">Admin</option></select>' +
      '</div>';

    deps.store.updateAccountState(function(accountState) {
      var entry = accountState.byAccountID.acct_1 || (accountState.byAccountID.acct_1 = {
        addWorkspaceOpen: true,
        createWorkspace: { pending: false, error: '' },
        selectedWorkspaceID: 'ws_1',
        manageWorkspace: { pending: false, error: '' },
        accessVisible: false,
        accessQuery: { status: 'idle', error: '', data: [] },
      });
      entry.addWorkspaceOpen = true;
      entry.selectedWorkspaceID = 'ws_1';
    }, { notify: false });

    runtime.toggleAccess('acct_1');
    await flushAsync();

    expect(deps.store.getAccountState().byAccountID.acct_1.accessVisible).toBe(true);
    expect(deps.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('');
    expect(deps.store.getAccountState().byAccountID.acct_1.addWorkspaceOpen).toBe(false);
    expect(deps.store.getAccountState().byAccountID.acct_1.accessQuery.status).toBe('ready');
    expect(deps.store.getAccountState().byAccountID.acct_1.accessQuery.data).toHaveLength(1);
    expect(document.getElementById('access-stats-acct_1')?.textContent).toContain('Members');
    var roleSelect = document.querySelector('[data-action="change-role"]') as HTMLSelectElement | null;
    expect(roleSelect).not.toBeNull();
    expect(roleSelect?.value).toBe('owner');

    await runtime.changeRole('acct_1', 'u1', 'admin');
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
