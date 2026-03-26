import {
  createAnonymousBootstrap,
  dispatchPortalRender,
  getAccountAPIBasePath,
  getBootstrap,
  getBootstrapPath,
  getCommercialAPIBaseURL,
  getLogoutPath,
  getMagicLinkRequestPath,
  getPortalAPIBasePath,
  getPortalPath,
  getSignupPath,
  installRuntime,
  setBootstrap,
} from './runtime';
import type { PortalBootstrapData, PortalLoginState, PortalRuntime } from './types';

var portalBootstrap: PortalBootstrapData = getBootstrap();
var LICENSE_API_BASE = getCommercialAPIBaseURL();
var PORTAL_PATH = getPortalPath();
var BOOTSTRAP_PATH = getBootstrapPath();
var MAGIC_LINK_REQUEST_PATH = getMagicLinkRequestPath();
var SIGNUP_PATH = getSignupPath();
var LOGOUT_PATH = getLogoutPath();
var ACCOUNT_API_BASE_PATH = getAccountAPIBasePath();
var PORTAL_API_BASE_PATH = getPortalAPIBasePath();

var loginState: PortalLoginState = {
  emailValue: '',
  sending: false,
  success: false,
  error: '',
};

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
type ToastElement = HTMLElement & { _timer?: ReturnType<typeof setTimeout> };

function getElement<T extends HTMLElement = HTMLElement>(id): T | null {
  return document.getElementById(id) as T | null;
}

function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

function escapeHTML(value) {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escapeAttr(value) {
  return escapeHTML(value);
}

function formatWorkspaceDate(value) {
  if (!value) return '';
  var date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function roleBadgeHTML(role) {
  if (role === 'owner') return '<span class="badge" style="background:#f1f5f9;color:#64748b">Owner</span>';
  if (role === 'admin') return '<span class="badge" style="background:#f1f5f9;color:#64748b">Admin</span>';
  if (role === 'tech') return '<span class="badge" style="background:#f1f5f9;color:#64748b">Tech</span>';
  return '';
}

function renderHeader() {
  var userInfo = document.getElementById('portal-user-info');
  if (!userInfo) return;
  if (portalBootstrap.authenticated) {
    userInfo.innerHTML =
      '<span>' + escapeHTML(portalBootstrap.email || '') + '</span>' +
      '<button class="logout-btn" id="logout-btn" type="button">Sign out</button>';
    return;
  }
  userInfo.innerHTML =
    '<a class="logout-btn" href="' + escapeAttr(SIGNUP_PATH) + '" style="text-decoration:none">Create account</a>';
}

function renderWorkspaceCard(account, workspace) {
  var state = String(workspace.state || '');
  var safeState = escapeHTML(state);
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var openAction = '';
  if (state === 'active') {
    openAction =
      '<form method="POST" action="' +
      escapeAttr(ACCOUNT_API_BASE_PATH + '/' + account.id + '/tenants/' + workspace.id + '/handoff') +
      '">' +
      '<button type="submit" class="btn-primary">Open →</button>' +
      '</form>';
  } else {
    openAction = '<span style="font-size:13px;color:#94a3b8">' + safeState + '</span>';
  }

  var manageAction = '';
  if (account.can_manage && (state === 'active' || state === 'suspended' || state === 'failed')) {
    manageAction =
      '<button type="button" class="btn-danger" data-action="workspace-manage" data-account-id="' +
      escapeAttr(account.id) +
      '" data-workspace-id="' +
      escapeAttr(workspace.id) +
      '" data-workspace-state="' +
      escapeAttr(state) +
      '" data-workspace-name="' +
      escapeAttr(workspace.display_name) +
      '">⋯</button>';
  }

  var createdMeta = createdLabel ? '<span class="ws-created">Created ' + escapeHTML(createdLabel) + '</span>' : '';
  return (
    '<div class="workspace-card">' +
      '<div class="ws-info">' +
        '<span class="ws-name">' + escapeHTML(workspace.display_name) + '</span>' +
        '<div class="ws-meta">' +
          (workspace.healthy
            ? '<span class="badge badge-healthy">Healthy</span>'
            : '<span class="badge badge-unhealthy">Checking</span>') +
          '<span class="badge badge-' + safeState + '">' + safeState + '</span>' +
          createdMeta +
        '</div>' +
      '</div>' +
      '<div class="ws-actions">' +
        openAction +
        manageAction +
      '</div>' +
    '</div>'
  );
}

function renderAccountSection(account) {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var workspaceHTML = '';
  if (workspaces.length === 0) {
    workspaceHTML = '<div class="empty-state"><p>No workspaces yet. Create one to get started.</p></div>';
  } else {
    workspaceHTML =
      '<div class="workspace-list">' +
      workspaces.map(function(workspace) {
        return renderWorkspaceCard(account, workspace);
      }).join('') +
      '</div>';
  }

  var actions = '';
  var teamSection = '';
  var addWorkspaceForm = '';
  if (account.can_manage) {
    actions =
      '<div class="account-actions">' +
      (account.kind === 'msp'
        ? '<button type="button" class="btn-secondary" id="add-ws-btn-' +
          escapeAttr(account.id) +
          '" data-action="toggle-add-workspace" data-account-id="' +
          escapeAttr(account.id) +
          '">+ Add workspace</button>'
        : '') +
      (account.has_billing
        ? '<button type="button" class="btn-secondary" data-action="open-billing" data-account-id="' +
          escapeAttr(account.id) +
          '">Manage billing</button>'
        : '') +
      '<button type="button" class="btn-secondary" id="team-btn-' +
      escapeAttr(account.id) +
      '" data-action="toggle-team" data-account-id="' +
      escapeAttr(account.id) +
      '">Manage team</button>' +
      '</div>';

    teamSection =
      '<div class="team-section" id="team-section-' +
      escapeAttr(account.id) +
      '" data-actor-role="' +
      escapeAttr(account.role) +
      '">' +
      '<h3>Team members</h3>' +
      '<table class="team-table">' +
      '<thead><tr><th>Email</th><th>Role</th><th></th></tr></thead>' +
      '<tbody id="team-list-' +
      escapeAttr(account.id) +
      '">' +
      '<tr><td colspan="3" style="color:#94a3b8;text-align:center;padding:16px">Loading…</td></tr>' +
      '</tbody>' +
      '</table>' +
      '<div class="team-invite">' +
      '<div><label for="invite-email-' +
      escapeAttr(account.id) +
      '">Email</label><input type="email" id="invite-email-' +
      escapeAttr(account.id) +
      '" placeholder="user@example.com" autocomplete="off"></div>' +
      '<div><label for="invite-role-' +
      escapeAttr(account.id) +
      '">Role</label><select id="invite-role-' +
      escapeAttr(account.id) +
      '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div>' +
      '<button type="button" class="btn-primary" style="padding:8px 14px;font-size:13px" data-action="invite-member" data-account-id="' +
      escapeAttr(account.id) +
      '">Invite</button>' +
      '</div>' +
      '</div>';

    if (account.kind === 'msp') {
      addWorkspaceForm =
        '<div class="add-workspace-form" id="add-ws-form-' +
        escapeAttr(account.id) +
        '">' +
        '<label for="ws-name-' +
        escapeAttr(account.id) +
        '">Workspace name (e.g. client name)</label>' +
        '<input type="text" id="ws-name-' +
        escapeAttr(account.id) +
        '" placeholder="Acme Corp" maxlength="80" autocomplete="off">' +
        '<div class="form-actions">' +
        '<button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">Create workspace</button>' +
        '<button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">Cancel</button>' +
        '<div class="spinner" id="ws-spinner-' +
        escapeAttr(account.id) +
        '"></div>' +
        '</div>' +
        '</div>';
    }
  }

  return (
    '<section class="account-section">' +
      '<div class="account-header">' +
        '<h2>' + escapeHTML(account.name) + '</h2>' +
        '<span class="badge badge-' + escapeHTML(account.kind) + '">' + escapeHTML(account.kind_label) + '</span>' +
        roleBadgeHTML(account.role) +
      '</div>' +
      workspaceHTML +
      actions +
      teamSection +
      addWorkspaceForm +
    '</section>'
  );
}

function renderAccounts(accounts) {
  var root = document.getElementById('accounts-root');
  if (!root) return;
  var safeAccounts = Array.isArray(accounts) ? accounts : [];
  if (safeAccounts.length === 0) {
    root.innerHTML =
      '<div class="empty-state" style="margin-top:48px">' +
        '<p>No workspaces found. If you just signed up, check your email for setup instructions.</p>' +
        '<p style="margin-top:12px;font-size:13px">Need help? Contact <a href="mailto:' +
        escapeAttr(portalBootstrap.support_email || '') +
        '" style="color:#1d4ed8">' +
        escapeHTML(portalBootstrap.support_email || '') +
        '</a></p>' +
      '</div>';
    return;
  }
  root.innerHTML = safeAccounts.map(renderAccountSection).join('');
}

function renderAuthenticatedPortal() {
  return (
    '<section class="intro-card">' +
      '<h1>Pulse Account</h1>' +
      '<p>Manage Cloud workspaces, MSP access, and self-hosted commercial account services from one account surface. Hosted workspace lifecycle lives here today, and the self-hosted billing, license recovery, refund, and privacy tools below now share the same Pulse Account shell instead of staying fragmented across public utility pages.</p>' +
    '</section>' +
    '<div id="accounts-root"></div>' +
    '<section class="service-section">' +
      '<div class="service-header">' +
        '<h2>Other account services</h2>' +
        '<div class="service-note">Self-hosted commercial account actions now live here. The public utility pages remain as compatibility entry points, not the primary account surface.</div>' +
      '</div>' +
      '<div class="service-grid">' +
        '<button class="service-card service-card-button" type="button" id="open-manage-service" data-account-service-action="open-service-panel" data-account-service-panel="manage-service-panel" data-account-service-focus="manage-inline-email">' +
          '<h3>Manage subscriptions</h3>' +
          '<p>Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.</p>' +
        '</button>' +
        '<button class="service-card service-card-button" type="button" id="open-retrieve-service" data-account-service-action="open-service-panel" data-account-service-panel="retrieve-service-panel" data-account-service-focus="retrieve-inline-email">' +
          '<h3>Retrieve licenses</h3>' +
          '<p>Recover the latest active self-hosted license and invoice link for a commercial email address.</p>' +
        '</button>' +
        '<button class="service-card service-card-button" type="button" id="open-refund-service" data-account-service-action="open-service-panel" data-account-service-panel="refund-service-panel" data-account-service-focus="refund-inline-email">' +
          '<h3>Refund requests</h3>' +
          '<p>Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.</p>' +
        '</button>' +
        '<button class="service-card service-card-button" type="button" id="open-data-service" data-account-service-action="open-service-panel" data-account-service-panel="data-service-panel" data-account-service-focus="data-export-email">' +
          '<h3>Data and privacy</h3>' +
          '<p>Request commercial data export or deletion without leaving the account shell.</p>' +
        '</button>' +
      '</div>' +
      '<div class="service-panel" id="manage-service-panel"><div id="manage-service-root"></div></div>' +
      '<div class="service-panel" id="retrieve-service-panel"><div id="retrieve-service-root"></div></div>' +
      '<div class="service-panel" id="refund-service-panel"><div id="refund-service-root"></div></div>' +
      '<div class="service-panel" id="data-service-panel">' +
        '<h3>Data and privacy</h3>' +
        '<p>Request export or deletion of the commercial data tied to an email address. Payment data held directly by Stripe still requires support handling.</p>' +
        '<div class="subsection"><div id="data-export-root"></div></div>' +
        '<div class="subsection"><div id="data-delete-root"></div></div>' +
        '<div class="helper-text">Payment-card data stays with Stripe. For Stripe deletion support, contact <a href="mailto:' +
        escapeAttr(portalBootstrap.support_email || '') +
        '">' +
        escapeHTML(portalBootstrap.support_email || '') +
        '</a>.</div>' +
      '</div>' +
    '</section>'
  );
}

function renderSignedOutPortal() {
  var statusHTML = '';
  if (loginState.error) {
    statusHTML = '<div class="service-status visible error">' + escapeHTML(loginState.error) + '</div>';
  } else if (loginState.success) {
    statusHTML =
      '<div class="service-status visible success">' +
        'Magic link sent. Check your inbox and click the link to sign in.' +
        '<br><br><strong>Don\'t see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.' +
      '</div>';
  }
  return (
    '<section class="intro-card">' +
      '<h1>Pulse Account</h1>' +
      '<p>Sign in to manage Cloud workspaces, MSP access, and commercial account services from one account surface.</p>' +
    '</section>' +
    '<section class="service-section">' +
      '<div class="service-panel visible">' +
        '<h3>Sign in</h3>' +
        '<p>Enter the commercial email address for your Pulse account. I will send a magic link so you can open Pulse Account without managing a password.</p>' +
        '<div class="form-group">' +
          '<label for="portal-login-email">Email address</label>' +
          '<input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' +
          escapeAttr(loginState.emailValue || '') +
          '" data-portal-input="login-email">' +
        '</div>' +
        '<div class="form-actions">' +
          '<button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' +
          (loginState.sending ? 'Sending…' : 'Send magic link') +
          '</button>' +
          '<a class="btn-secondary" href="' + escapeAttr(SIGNUP_PATH) + '" style="text-decoration:none">Create an account</a>' +
        '</div>' +
        statusHTML +
      '</div>' +
    '</section>'
  );
}

function renderPortalApp() {
  renderHeader();
  var root = document.getElementById('portal-app-root');
  if (!root) return;
  root.innerHTML = portalBootstrap.authenticated ? renderAuthenticatedPortal() : renderSignedOutPortal();
  if (portalBootstrap.authenticated) {
    renderAccounts(portalBootstrap.accounts || []);
  }
  dispatchPortalRender();
}

function applyBootstrap(data) {
  portalBootstrap = setBootstrap(data || createAnonymousBootstrap());
  LICENSE_API_BASE = getCommercialAPIBaseURL();
  PORTAL_PATH = getPortalPath();
  BOOTSTRAP_PATH = getBootstrapPath();
  MAGIC_LINK_REQUEST_PATH = getMagicLinkRequestPath();
  SIGNUP_PATH = getSignupPath();
  LOGOUT_PATH = getLogoutPath();
  ACCOUNT_API_BASE_PATH = getAccountAPIBasePath();
  PORTAL_API_BASE_PATH = getPortalAPIBasePath();
  if (!portalBootstrap.authenticated && !loginState.emailValue) {
    loginState.emailValue = portalBootstrap.email || '';
  }
  renderPortalApp();
}

async function refreshBootstrap() {
  if (!BOOTSTRAP_PATH) return false;
  try {
    var response = await fetch(BOOTSTRAP_PATH, {
      headers: { 'Accept': 'application/json' }
    });
    if (response.status === 401) {
      applyBootstrap(createAnonymousBootstrap());
      return true;
    }
    if (!response.ok) return false;
    var data = await response.json();
    applyBootstrap(data);
    return true;
  } catch (_) {}
  return false;
}

function showToast(msg, isError = false) {
  var t = getElement<ToastElement>('toast');
  if (!t) return;
  t.textContent = msg;
  t.className = 'toast visible' + (isError ? ' error' : '');
  clearTimeout(t._timer);
  t._timer = setTimeout(function() { t.className = 'toast'; }, 4000);
}

function resetLoginState(options) {
  loginState.sending = false;
  loginState.error = '';
  loginState.success = false;
  if (options && options.keepEmail) return;
  loginState.emailValue = '';
}

async function sendMagicLink() {
  var email = String(loginState.emailValue || '').trim();
  if (!email) {
    var input = getElement<FormValueElement>('portal-login-email');
    if (input) input.focus();
    return;
  }
  loginState.sending = true;
  loginState.error = '';
  loginState.success = false;
  renderPortalApp();
  try {
    var response = await fetch(MAGIC_LINK_REQUEST_PATH, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    if (response.ok || response.status === 404) {
      loginState.sending = false;
      loginState.success = true;
      renderPortalApp();
      return;
    }
    if (response.status === 429) {
      loginState.error = 'Too many requests. Please wait a moment and try again.';
    } else {
      loginState.error = 'Something went wrong. Please try again.';
    }
  } catch (_) {
    loginState.error = 'Network error. Please check your connection and try again.';
  }
  loginState.sending = false;
  renderPortalApp();
}

var portalRuntime: PortalRuntime = {
  getBootstrap: function() {
    return portalBootstrap;
  },
  getCommercialAPIBaseURL: function() {
    return LICENSE_API_BASE;
  },
  getPortalPath: function() {
    return PORTAL_PATH;
  },
  getAccountAPIBasePath: function() {
    return ACCOUNT_API_BASE_PATH;
  },
  getPortalAPIBasePath: function() {
    return PORTAL_API_BASE_PATH;
  },
  refreshBootstrap: refreshBootstrap,
  showToast: showToast,
};
installRuntime(portalRuntime);

document.addEventListener('click', function(event) {
  var portalActionEl = asHTMLElement(event.target)?.closest('[data-portal-action]');
  if (portalActionEl) {
    var portalAction = portalActionEl.getAttribute('data-portal-action') || '';
    switch (portalAction) {
      case 'send-magic-link':
        event.preventDefault();
        sendMagicLink();
        return;
      case 'resend-magic-link':
        event.preventDefault();
        loginState.success = false;
        loginState.error = '';
        renderPortalApp();
        sendMagicLink();
        return;
      default:
        break;
    }
  }

  var logoutBtn = asHTMLElement(event.target)?.closest('#logout-btn') as HTMLButtonElement | null;
  if (logoutBtn) {
    event.preventDefault();
    logoutBtn.disabled = true;
    logoutBtn.textContent = 'Signing out…';
    (async function() {
      try {
        await fetch(LOGOUT_PATH, { method: 'POST' });
      } catch (_) {}
      window.location.href = PORTAL_PATH;
    })();
    return;
  }

  var actionEl = asHTMLElement(event.target)?.closest('[data-action]');
  if (!actionEl) return;
  var action = actionEl.getAttribute('data-action') || '';
  var accountID = actionEl.getAttribute('data-account-id') || '';

  switch (action) {
    case 'toggle-add-workspace':
      event.preventDefault();
      toggleAddWorkspace(accountID);
      return;
    case 'open-billing':
      event.preventDefault();
      openBilling(accountID);
      return;
    case 'toggle-team':
      event.preventDefault();
      toggleTeam(accountID);
      return;
    case 'invite-member':
      event.preventDefault();
      inviteMember(accountID);
      return;
    case 'create-workspace':
      event.preventDefault();
      createWorkspace(accountID);
      return;
    case 'workspace-manage':
      event.preventDefault();
      suspendOrDelete(
        event,
        accountID,
        actionEl.getAttribute('data-workspace-id') || '',
        actionEl.getAttribute('data-workspace-state') || '',
        actionEl.getAttribute('data-workspace-name') || ''
      );
      return;
    case 'remove-member':
      event.preventDefault();
      removeMember(
        accountID,
        actionEl.getAttribute('data-user-id') || '',
        actionEl.getAttribute('data-member-email') || ''
      );
      return;
    default:
      return;
  }
});

document.addEventListener('change', function(event) {
  var target = asHTMLElement(event.target) as HTMLSelectElement | null;
  if (!target || target.getAttribute('data-action') !== 'change-role') return;
  changeRole(
    target.getAttribute('data-account-id') || '',
    target.getAttribute('data-user-id') || '',
    target.value
  );
});

document.addEventListener('input', function(event) {
  var target = asHTMLElement(event.target) as FormValueElement | null;
  if (!target) return;
  if (target.getAttribute('data-portal-input') === 'login-email') {
    loginState.emailValue = target.value;
  }
});

function toggleAddWorkspace(accountID) {
  var form = document.getElementById('add-ws-form-' + accountID);
  if (!form) return;
  var visible = form.classList.contains('visible');
  form.classList.toggle('visible', !visible);
  if (!visible) {
    var input = getElement<FormValueElement>('ws-name-' + accountID);
    if (input) input.focus();
  }
}

async function createWorkspace(accountID) {
  var nameEl = getElement<FormValueElement>('ws-name-' + accountID);
  if (!nameEl) return;
  var name = nameEl.value.trim();
  if (!name) { nameEl.focus(); return; }
  var spinner = getElement('ws-spinner-' + accountID);
  if (spinner) spinner.style.display = 'block';
  try {
    var resp = await fetch(ACCOUNT_API_BASE_PATH + '/' + accountID + '/tenants', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ display_name: name })
    });
    if (!resp.ok) {
      var err = await resp.json().catch(function() { return {}; });
      showToast((err && err.error) || 'Failed to create workspace', true);
      return;
    }
    if (!await refreshBootstrap()) {
      window.location.href = PORTAL_PATH;
      return;
    }
    showToast('Workspace created!');
  } catch (_) {
    showToast('Network error. Please try again.', true);
  } finally {
    if (spinner) spinner.style.display = 'none';
  }
}

async function suspendOrDelete(evt, accountID, tenantID, state, name) {
  evt.stopPropagation();
  var action = state === 'active' ? 'Suspend' : 'Delete';
  if (!confirm(action + ' workspace "' + name + '"?')) return;
  var method = state === 'active' ? 'PATCH' : 'DELETE';
  var body = state === 'active' ? JSON.stringify({ state: 'suspended' }) : undefined;
  try {
    var response = await fetch(ACCOUNT_API_BASE_PATH + '/' + accountID + '/tenants/' + tenantID, {
      method: method,
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body
    });
    if (!response.ok) {
      showToast('Failed to ' + action.toLowerCase() + ' workspace.', true);
      return;
    }
    if (!await refreshBootstrap()) {
      window.location.href = PORTAL_PATH;
      return;
    }
    showToast(action + 'd workspace.');
  } catch (_) {
    showToast('Network error.', true);
  }
}

async function openBilling(accountID) {
  try {
    var r = await fetch(PORTAL_API_BASE_PATH + '/billing?account_id=' + encodeURIComponent(accountID), { method: 'POST' });
    if (!r.ok) {
      var err = await r.json().catch(function() { return {}; });
      showToast((err && err.error) || 'Failed to open billing portal.', true);
      return;
    }
    var data = await r.json();
    if (data && data.url) {
      window.location.href = data.url;
    } else {
      showToast('Failed to open billing portal.', true);
    }
  } catch (_) {
    showToast('Network error.', true);
  }
}

function toggleTeam(accountID) {
  var section = document.getElementById('team-section-' + accountID);
  if (!section) return;
  var visible = section.classList.contains('visible');
  section.classList.toggle('visible', !visible);
  if (!visible) loadTeam(accountID);
}

function setTbodyMessage(tbody, msg, isError) {
  tbody.textContent = '';
  var tr = document.createElement('tr');
  var td = document.createElement('td');
  td.setAttribute('colspan', '3');
  td.style.cssText = 'text-align:center;padding:16px;color:' + (isError ? '#991b1b' : '#94a3b8');
  td.textContent = msg;
  tr.appendChild(td);
  tbody.appendChild(tr);
}

async function loadTeam(accountID) {
  var tbody = document.getElementById('team-list-' + accountID);
  var section = document.getElementById('team-section-' + accountID);
  if (!tbody || !section) return;
  var actorRole = section.getAttribute('data-actor-role') || '';
  var isOwner = actorRole === 'owner';
  setTbodyMessage(tbody, 'Loading…', false);
  try {
    var r = await fetch(ACCOUNT_API_BASE_PATH + '/' + encodeURIComponent(accountID) + '/members');
    if (!r.ok) { setTbodyMessage(tbody, 'Failed to load team.', true); return; }
    var members = await r.json();
    if (!members || members.length === 0) {
      setTbodyMessage(tbody, 'No team members.', false);
      return;
    }
    var allRoles = ['owner', 'admin', 'tech', 'read_only'];
    var nonOwnerRoles = ['admin', 'tech', 'read_only'];
    tbody.textContent = '';
    for (var i = 0; i < members.length; i++) {
      (function(m) {
        var tr = document.createElement('tr');
        var tdEmail = document.createElement('td');
        tdEmail.textContent = m.email;
        tr.appendChild(tdEmail);
        var tdRole = document.createElement('td');
        if (m.role === 'owner' && !isOwner) {
          tdRole.textContent = 'owner';
        } else {
          var sel = document.createElement('select');
          var roles = isOwner ? allRoles : nonOwnerRoles;
          for (var j = 0; j < roles.length; j++) {
            var opt = document.createElement('option');
            opt.value = roles[j];
            opt.textContent = roles[j].replace('_', ' ');
            if (m.role === roles[j]) opt.selected = true;
            sel.appendChild(opt);
          }
          sel.setAttribute('data-action', 'change-role');
          sel.setAttribute('data-account-id', accountID);
          sel.setAttribute('data-user-id', m.user_id);
          tdRole.appendChild(sel);
        }
        tr.appendChild(tdRole);
        var tdAction = document.createElement('td');
        if (!(m.role === 'owner' && !isOwner)) {
          var btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'btn-remove';
          btn.textContent = 'Remove';
          btn.setAttribute('data-action', 'remove-member');
          btn.setAttribute('data-account-id', accountID);
          btn.setAttribute('data-user-id', m.user_id);
          btn.setAttribute('data-member-email', m.email);
          tdAction.appendChild(btn);
        }
        tr.appendChild(tdAction);
        tbody.appendChild(tr);
      })(members[i]);
    }
  } catch (_) {
    setTbodyMessage(tbody, 'Network error.', true);
  }
}

async function refreshAccountTeamSection(accountID) {
  if (!await refreshBootstrap()) {
    window.location.href = PORTAL_PATH;
    return false;
  }
  var section = document.getElementById('team-section-' + accountID);
  if (!section) {
    return true;
  }
  section.classList.add('visible');
  await loadTeam(accountID);
  return true;
}

async function inviteMember(accountID) {
  var emailEl = getElement<HTMLInputElement>('invite-email-' + accountID);
  var roleEl = getElement<HTMLSelectElement>('invite-role-' + accountID);
  if (!emailEl || !roleEl) return;
  var email = emailEl.value.trim();
  if (!email) { emailEl.focus(); return; }
  try {
    var r = await fetch(ACCOUNT_API_BASE_PATH + '/' + encodeURIComponent(accountID) + '/members', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email, role: roleEl.value })
    });
    if (r.status === 409) { showToast('Member already exists.', true); return; }
    if (!r.ok) {
      var err = await r.text();
      showToast(err || 'Failed to invite member.', true);
      return;
    }
    emailEl.value = '';
    if (!await refreshAccountTeamSection(accountID)) {
      return;
    }
    showToast('Member invited!');
  } catch (_) {
    showToast('Network error.', true);
  }
}

async function changeRole(accountID, userID, newRole) {
  try {
    var r = await fetch(ACCOUNT_API_BASE_PATH + '/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ role: newRole })
    });
    if (r.status === 409) { showToast('Cannot demote last owner.', true); loadTeam(accountID); return; }
    if (!r.ok) { showToast('Failed to update role.', true); loadTeam(accountID); return; }
    if (!await refreshAccountTeamSection(accountID)) {
      return;
    }
    showToast('Role updated.');
  } catch (_) {
    showToast('Network error.', true);
    loadTeam(accountID);
  }
}

async function removeMember(accountID, userID, email) {
  if (!confirm('Remove ' + email + ' from this account?')) return;
  try {
    var r = await fetch(ACCOUNT_API_BASE_PATH + '/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
      method: 'DELETE'
    });
    if (r.status === 409) { showToast('Cannot remove last owner.', true); return; }
    if (!r.ok) { showToast('Failed to remove member.', true); return; }
    if (!await refreshAccountTeamSection(accountID)) {
      return;
    }
    showToast('Member removed.');
  } catch (_) {
    showToast('Network error.', true);
  }
}

loginState.emailValue = portalBootstrap.email || '';
applyBootstrap(portalBootstrap);
if (portalBootstrap.authenticated) {
  refreshBootstrap();
}
