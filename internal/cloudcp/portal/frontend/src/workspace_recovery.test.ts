import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PortalAPIError } from './api';
import type { PortalAPI } from './api';
import { installAccountRuntime } from './account_runtime';
import { installShell } from './shell';
import { createBootstrapDefaults } from './runtime';
import { createPortalStore } from './store';
import { workspaceSetupState, workspaceStatusCopy } from './workspace_presentation';
import type { PortalAccountSummary, PortalBootstrapData, PortalWorkspaceSummary } from './types';

function workspace(id = 'ws_1', state = 'active'): PortalWorkspaceSummary {
  return { id, display_name: id === 'ws_1' ? 'Acme Dental' : 'Other Client', state, healthy: true, health_status: 'healthy', created_at: '2026-11-01T12:00:00Z' };
}
function account(id = 'acct_1', workspaces = [workspace()]): PortalAccountSummary {
  return { id, name: 'Fixture MSP', kind: 'msp', kind_label: 'MSP', role: 'owner', can_manage: true, has_billing: true, members: [], workspaces };
}
function deferred<T>() {
  var resolve!: (value: T) => void;
  var promise = new Promise<T>(function(done) { resolve = done; });
  return { promise, resolve };
}
function setup(accounts = [account()]) {
  var initial = { ...createBootstrapDefaults({}), authenticated: true, email: 'operator@example.test', accounts };
  var store = createPortalStore(createBootstrapDefaults({}), initial);
  var api = { suspendWorkspace: vi.fn().mockResolvedValue(undefined), deleteWorkspace: vi.fn().mockResolvedValue(undefined), createWorkspace: vi.fn() } as unknown as PortalAPI;
  var refreshBootstrap = vi.fn().mockResolvedValue(false);
  var showToast = vi.fn();
  var runtime = installAccountRuntime({ api, store, refreshBootstrap, showToast });
  installShell({ store });
  runtime.selectWorkspace('acct_1', accounts[0].workspaces[0].id);
  return { api, store, runtime, refreshBootstrap, showToast, initial };
}
function element<T extends HTMLElement = HTMLElement>(id: string): T { return document.getElementById(id)! as T; }

beforeEach(function() {
  vi.restoreAllMocks();
  document.body.innerHTML = '<header id="portal-user-info"></header><div id="portal-app-root"></div><div id="toast"></div>';
  vi.spyOn(window, 'confirm').mockReturnValue(true);
});

describe('workspace write recovery through the actual account shell', function() {
  it.each([[409, 'rejected'], [403, 'rejected'], [500, 'unknown'], [0, 'unknown']] as const)('retains %s uncertainty beyond the toast without replay or unsafe re-enabling', async function(status, outcome) {
    var s = setup();
    vi.mocked(s.api.suspendWorkspace).mockRejectedValue(new PortalAPIError('Current state changed.', status));
    element('workspace-management-action-acct_1').focus();
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'not trusted');
    var request = s.store.getAccountState().byAccountID.acct_1.manageWorkspace;
    expect(request).toMatchObject({ pending: false, recoveryRequired: true, outcome, workspaceID: 'ws_1', workspaceName: 'Acme Dental', action: 'suspend' });
    expect(element('workspace-recovery-acct_1').hidden).toBe(false);
    expect(element('workspace-recovery-acct_1').textContent).toContain('not confirmed');
    expect(element('workspace-recovery-acct_1').textContent).toContain('Current state changed.');
    expect(element<HTMLButtonElement>('workspace-management-action-acct_1').disabled).toBe(true);
    expect(element<HTMLButtonElement>('workspace-management-close-acct_1').disabled).toBe(false);
    expect(document.activeElement?.id).toBe('workspace-recovery-refresh-acct_1');
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    expect(s.api.suspendWorkspace).toHaveBeenCalledTimes(1);
    expect(s.refreshBootstrap).not.toHaveBeenCalled();
    expect(s.showToast).not.toHaveBeenCalled();
    s.runtime.clearWorkspaceSelection('acct_1');
    expect(element('workspace-recovery-acct_1').hidden).toBe(false);
  });

  it('keeps a failed read retryable and a confirmed canceled client present but non-actionable', async function() {
    var s = setup();
    vi.mocked(s.api.suspendWorkspace).mockRejectedValue(new PortalAPIError('Subscription inactive.', 409));
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    await s.runtime.refreshWorkspaceState('acct_1');
    expect(element('workspace-recovery-acct_1').textContent).toContain('Could not refresh the current state.');
    expect(element('workspace-management-meta-acct_1').textContent).toContain('active');
    s.refreshBootstrap.mockImplementation(async function() {
      s.store.setBootstrap({ ...s.initial, accounts: [account('acct_1', [workspace('ws_1', 'canceled')])] });
      return true;
    });
    await s.runtime.refreshWorkspaceState('acct_1');
    expect(s.store.getAccountState().byAccountID.acct_1.manageWorkspace.recoveryRequired).toBe(false);
    expect(element('workspace-recovery-acct_1').textContent).toContain('canceled. No action was repeated.');
    expect(document.activeElement?.id).toBe('workspace-recovery-acct_1');
    expect(document.body.textContent).not.toContain('No client is attached yet');
    expect(document.body.textContent).not.toContain('Add the first client');
    expect(document.querySelector('[data-workspace-row="ws_1"]')?.textContent).toContain('Canceled');
    expect(element('workspace-management-summary-acct_1').textContent).toContain('canceled');
    expect(element('workspace-management-health-acct_1').textContent).toBe('Healthy');
    expect(element('workspace-management-action-acct_1').getAttribute('data-workspace-action')).toBeNull();
    expect(element<HTMLButtonElement>('workspace-management-action-acct_1').disabled).toBe(true);
    expect(element<HTMLButtonElement>('workspace-management-open-acct_1').disabled).toBe(true);
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'delete', 'Acme Dental');
    expect(s.api.deleteWorkspace).not.toHaveBeenCalled();
    expect(s.api.suspendWorkspace).toHaveBeenCalledTimes(1);
    // Cancellation of one client is not account-wide creation policy.
    expect(Array.from(document.querySelectorAll<HTMLButtonElement>('[data-action="toggle-add-workspace"]')).some(b => !b.disabled)).toBe(true);
  });

  it('admits one mutation and one deliberate recovery read while each is pending', async function() {
    var s = setup(); var mutation = deferred<void>();
    vi.mocked(s.api.suspendWorkspace).mockReturnValue(mutation.promise);
    var first = s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    expect(s.api.suspendWorkspace).toHaveBeenCalledTimes(1);
    mutation.resolve(undefined); await first;
    var read = deferred<boolean>(); s.refreshBootstrap.mockReturnValue(read.promise);
    var recovery = s.runtime.refreshWorkspaceState('acct_1');
    await s.runtime.refreshWorkspaceState('acct_1');
    expect(s.refreshBootstrap).toHaveBeenCalledTimes(2); // initial accepted-write read plus one recovery
    read.resolve(false); await recovery;
    expect(s.store.getAccountState().byAccountID.acct_1.manageWorkspace.refreshing).toBe(false);
  });

  it('distinguishes an accepted write with unavailable readback without navigating or claiming completion', async function() {
    var s = setup(); var location = window.location.href;
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    expect(window.location.href).toBe(location);
    expect(s.store.getAccountState().byAccountID.acct_1.manageWorkspace.outcome).toBe('accepted');
    expect(element('workspace-recovery-acct_1').textContent).toContain('Request accepted; current state not confirmed');
    expect(s.showToast).not.toHaveBeenCalled();
  });

  it('recovers an unknown DELETE to absence without replay or a fabricated Deleted success', async function() {
    var s = setup([account('acct_1', [workspace('ws_1', 'suspended')])]);
    vi.mocked(s.api.deleteWorkspace).mockRejectedValue(new PortalAPIError('Connection lost.', 0));
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'delete', 'Acme Dental');
    s.refreshBootstrap.mockImplementation(async function() { s.store.setBootstrap({ ...s.initial, accounts: [account('acct_1', [])] }); return true; });
    await s.runtime.refreshWorkspaceState('acct_1');
    expect(s.api.deleteWorkspace).toHaveBeenCalledTimes(1);
    expect(element('workspace-recovery-acct_1').textContent).toContain('no longer listed in this account');
    expect(s.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('');
    expect(s.showToast).not.toHaveBeenCalled();
  });

  it('does not apply a late accepted response to a different principal even with the same account ID', async function() {
    var s = setup(); var mutation = deferred<void>();
    vi.mocked(s.api.suspendWorkspace).mockReturnValue(mutation.promise);
    var first = s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    s.store.setBootstrap({ ...s.initial, email: 'other@example.test', accounts: [account('acct_1', [workspace('ws_other')])] });
    mutation.resolve(undefined); await first;
    expect(document.body.textContent).not.toContain('Acme Dental');
    expect(s.refreshBootstrap).not.toHaveBeenCalled(); expect(s.showToast).not.toHaveBeenCalled();
    expect(s.store.getAccountState().byAccountID.acct_1.manageWorkspace.recoveryRequired).toBeUndefined();
  });

  it('does not claim completion when readback withdraws the session', async function() {
    var s = setup();
    s.refreshBootstrap.mockImplementation(async function() { s.store.setBootstrap({ authenticated: false, email: '', accounts: [] }); return true; });
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    expect(s.showToast).not.toHaveBeenCalled();
    expect(document.body.textContent).not.toContain('Acme Dental');
    expect(s.store.getAccountState().byAccountID).toEqual({});
  });

  it('rechecks the target after confirm and never invents a delete for a canceled client', async function() {
    var s = setup();
    vi.mocked(window.confirm).mockImplementation(function() {
      s.store.setBootstrap({ ...s.initial, accounts: [account('acct_1', [workspace('ws_1', 'canceled')])] }); return true;
    });
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'delete', 'Acme Dental');
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', '', 'Acme Dental');
    expect(s.api.suspendWorkspace).not.toHaveBeenCalled(); expect(s.api.deleteWorkspace).not.toHaveBeenCalled();
  });

  it('does not close another selected client or steal focus when the first action settles', async function() {
    var s = setup([account('acct_1', [workspace(), workspace('ws_2')])]); var mutation = deferred<void>();
    vi.mocked(s.api.suspendWorkspace).mockReturnValue(mutation.promise);
    s.refreshBootstrap.mockImplementation(async function() { s.store.setBootstrap({ ...s.initial, accounts: [account('acct_1', [workspace('ws_1', 'suspended'), workspace('ws_2')])] }); return true; });
    element('workspace-management-action-acct_1').focus();
    var first = s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    s.runtime.selectWorkspace('acct_1', 'ws_2'); element('ws-name-acct_1').focus();
    mutation.resolve(undefined); await first;
    expect(s.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('ws_2');
    expect(element('workspace-management-title-acct_1').textContent).toBe('Other Client');
    expect(document.activeElement?.id).not.toBe('workspace-recovery-acct_1');
    expect(s.showToast).toHaveBeenCalledWith('Suspended client.');
  });

  it('keeps other accounts drafts and the selected identity through the recovery remount', async function() {
    var s = setup([account(), account('acct_2', [workspace('ws_2')])]);
    element<HTMLInputElement>('ws-name-acct_2').value = 'Unsubmitted client';
    element<HTMLInputElement>('invite-email-acct_2').value = 'staff@example.test';
    element<HTMLSelectElement>('invite-role-acct_2').value = 'tech';
    vi.mocked(s.api.suspendWorkspace).mockRejectedValue(new PortalAPIError('Conflict.', 409));
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    s.refreshBootstrap.mockImplementation(async function() { s.store.setBootstrap(s.initial); return true; });
    await s.runtime.refreshWorkspaceState('acct_1');
    expect(element<HTMLInputElement>('ws-name-acct_2').value).toBe('Unsubmitted client');
    expect(element<HTMLInputElement>('invite-email-acct_2').value).toBe('staff@example.test');
    expect(element<HTMLSelectElement>('invite-role-acct_2').value).toBe('tech');
    expect(s.store.getAccountState().byAccountID.acct_1.selectedWorkspaceID).toBe('ws_1');
  });

  it('retains an honest unconfirmed notice if the same session loses the account', async function() {
    var s = setup(); s.refreshBootstrap.mockImplementation(async function() { s.store.setBootstrap({ ...s.initial, accounts: [] }); return true; });
    await s.runtime.manageWorkspaceAction('acct_1', 'ws_1', 'suspend', 'Acme Dental');
    expect(s.showToast).not.toHaveBeenCalled();
    expect(element('workspace-account-unavailable').hidden).toBe(false);
    expect(element('workspace-account-unavailable').textContent).toContain('not confirmed');
  });

  it.each(['canceled', 'deleting', 'provisioning', 'failed', 'unknown'])('keeps %s rows as review, including health and alert distinctions', function(state) {
    var s = setup([account('acct_1', [workspace('ws_1', state)])]);
    expect(workspaceSetupState(workspace('ws_1', state))).toBe('review');
    expect(workspaceStatusCopy(workspace('ws_1', state))).not.toBe('Latest health check is healthy.');
    expect(document.body.textContent).not.toContain('No client is attached yet');
    expect(document.body.textContent).not.toContain('Add the first client');
    expect(document.querySelector('[data-workspace-row="ws_1"]')?.textContent).toContain('Healthy');
    if (state !== 'failed') expect(element<HTMLButtonElement>('workspace-management-action-acct_1').disabled).toBe(true);
    expect(s.api.deleteWorkspace).not.toHaveBeenCalled();
  });
});
