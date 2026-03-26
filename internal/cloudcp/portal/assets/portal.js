var bootstrapEl = document.getElementById('pulse-account-bootstrap');
var portalBootstrap = {};
if (bootstrapEl) {
  try {
    portalBootstrap = JSON.parse(bootstrapEl.textContent || '{}');
  } catch (_) {
    portalBootstrap = {};
  }
}
var LICENSE_API_BASE = portalBootstrap.commercial_api_base_url || '';
var PORTAL_PATH = portalBootstrap.portal_path || '/portal';
var BOOTSTRAP_PATH = portalBootstrap.bootstrap_path || '/api/portal/bootstrap';
var LOGOUT_PATH = portalBootstrap.logout_path || '/auth/logout';
var ACCOUNT_API_BASE_PATH = portalBootstrap.account_api_base_path || '/api/accounts';
var PORTAL_API_BASE_PATH = portalBootstrap.portal_api_base_path || '/api/portal';

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

function renderWorkspaceCard(account, workspace) {
  var state = String(workspace.state || '');
  var safeState = escapeHTML(state);
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var openAction = '';
  if (state === 'active') {
    openAction = '<form method="POST" action="' + escapeAttr(ACCOUNT_API_BASE_PATH + '/' + account.id + '/tenants/' + workspace.id + '/handoff') + '">' +
      '<button type="submit" class="btn-primary">Open →</button>' +
      '</form>';
  } else {
    openAction = '<span style="font-size:13px;color:#94a3b8">' + safeState + '</span>';
  }

  var manageAction = '';
  if (account.can_manage && (state === 'active' || state === 'suspended' || state === 'failed')) {
    manageAction = '<button class="btn-danger" onclick="suspendOrDelete(event,\'' + escapeAttr(account.id) + '\',\'' + escapeAttr(workspace.id) + '\',\'' + escapeAttr(state) + '\',\'' + escapeAttr(workspace.display_name) + '\')">⋯</button>';
  }

  var createdMeta = createdLabel ? '<span class="ws-created">Created ' + escapeHTML(createdLabel) + '</span>' : '';
  return '' +
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
    '</div>';
}

function renderAccountSection(account) {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var workspaceHTML = '';
  if (workspaces.length === 0) {
    workspaceHTML = '<div class="empty-state"><p>No workspaces yet. Create one to get started.</p></div>';
  } else {
    workspaceHTML = '<div class="workspace-list">' + workspaces.map(function(workspace) {
      return renderWorkspaceCard(account, workspace);
    }).join('') + '</div>';
  }

  var actions = '';
  var teamSection = '';
  var addWorkspaceForm = '';
  if (account.can_manage) {
    actions = '<div class="account-actions">' +
      (account.kind === 'msp'
        ? '<button class="btn-secondary" id="add-ws-btn-' + escapeAttr(account.id) + '" onclick="toggleAddWorkspace(\'' + escapeAttr(account.id) + '\')">+ Add workspace</button>'
        : '') +
      (account.has_billing
        ? '<button class="btn-secondary" onclick="openBilling(\'' + escapeAttr(account.id) + '\')">Manage billing</button>'
        : '') +
      '<button class="btn-secondary" id="team-btn-' + escapeAttr(account.id) + '" onclick="toggleTeam(\'' + escapeAttr(account.id) + '\',\'' + escapeAttr(account.role) + '\')">Manage team</button>' +
    '</div>';

    teamSection = '' +
      '<div class="team-section" id="team-section-' + escapeAttr(account.id) + '" data-actor-role="' + escapeAttr(account.role) + '">' +
        '<h3>Team members</h3>' +
        '<table class="team-table">' +
          '<thead><tr><th>Email</th><th>Role</th><th></th></tr></thead>' +
          '<tbody id="team-list-' + escapeAttr(account.id) + '">' +
            '<tr><td colspan="3" style="color:#94a3b8;text-align:center;padding:16px">Loading…</td></tr>' +
          '</tbody>' +
        '</table>' +
        '<div class="team-invite">' +
          '<div><label for="invite-email-' + escapeAttr(account.id) + '">Email</label><input type="email" id="invite-email-' + escapeAttr(account.id) + '" placeholder="user@example.com" autocomplete="off"></div>' +
          '<div><label for="invite-role-' + escapeAttr(account.id) + '">Role</label><select id="invite-role-' + escapeAttr(account.id) + '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div>' +
          '<button class="btn-primary" style="padding:8px 14px;font-size:13px" onclick="inviteMember(\'' + escapeAttr(account.id) + '\')">Invite</button>' +
        '</div>' +
      '</div>';

    if (account.kind === 'msp') {
      addWorkspaceForm = '' +
        '<div class="add-workspace-form" id="add-ws-form-' + escapeAttr(account.id) + '">' +
          '<label for="ws-name-' + escapeAttr(account.id) + '">Workspace name (e.g. client name)</label>' +
          '<input type="text" id="ws-name-' + escapeAttr(account.id) + '" placeholder="Acme Corp" maxlength="80" autocomplete="off">' +
          '<div class="form-actions">' +
            '<button class="btn-primary" onclick="createWorkspace(\'' + escapeAttr(account.id) + '\')">Create workspace</button>' +
            '<button class="btn-secondary" onclick="toggleAddWorkspace(\'' + escapeAttr(account.id) + '\')">Cancel</button>' +
            '<div class="spinner" id="ws-spinner-' + escapeAttr(account.id) + '"></div>' +
          '</div>' +
        '</div>';
    }
  }

  return '' +
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
    '</section>';
}

function renderAccounts(accounts) {
  var root = document.getElementById('accounts-root');
  if (!root) return;
  var safeAccounts = Array.isArray(accounts) ? accounts : [];
  if (safeAccounts.length === 0) {
    root.innerHTML = '' +
      '<div class="empty-state" style="margin-top:48px">' +
        '<p>No workspaces found. If you just signed up, check your email for setup instructions.</p>' +
        '<p style="margin-top:12px;font-size:13px">Need help? Contact <a href="mailto:' + escapeAttr(portalBootstrap.support_email || '') + '" style="color:#1d4ed8">' + escapeHTML(portalBootstrap.support_email || '') + '</a></p>' +
      '</div>';
    return;
  }
  root.innerHTML = safeAccounts.map(renderAccountSection).join('');
}

function applyBootstrap(data) {
  portalBootstrap = data || {};
  LICENSE_API_BASE = portalBootstrap.commercial_api_base_url || LICENSE_API_BASE;
  PORTAL_PATH = portalBootstrap.portal_path || PORTAL_PATH;
  BOOTSTRAP_PATH = portalBootstrap.bootstrap_path || BOOTSTRAP_PATH;
  LOGOUT_PATH = portalBootstrap.logout_path || LOGOUT_PATH;
  ACCOUNT_API_BASE_PATH = portalBootstrap.account_api_base_path || ACCOUNT_API_BASE_PATH;
  PORTAL_API_BASE_PATH = portalBootstrap.portal_api_base_path || PORTAL_API_BASE_PATH;
  renderAccounts(portalBootstrap.accounts || []);
}

async function refreshBootstrap() {
  if (!BOOTSTRAP_PATH) return;
  try {
    var response = await fetch(BOOTSTRAP_PATH, {
      headers: { 'Accept': 'application/json' }
    });
    if (!response.ok) return;
    var data = await response.json();
    applyBootstrap(data);
  } catch (_) {}
}

function showToast(msg, isError) {
  var t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast visible' + (isError ? ' error' : '');
  clearTimeout(t._timer);
  t._timer = setTimeout(function() { t.className = 'toast'; }, 4000);
}

document.getElementById('logout-btn').onclick = async function() {
  this.disabled = true;
  this.textContent = 'Signing out…';
  try {
    await fetch(LOGOUT_PATH, { method: 'POST' });
  } catch(_) {}
  window.location.href = PORTAL_PATH;
};

applyBootstrap(portalBootstrap);
refreshBootstrap();

function toggleServicePanel(panelID) {
  var panels = ['manage-service-panel', 'retrieve-service-panel', 'refund-service-panel', 'data-service-panel'];
  for (var i = 0; i < panels.length; i++) {
    var panel = document.getElementById(panels[i]);
    panel.classList.toggle('visible', panels[i] === panelID ? !panel.classList.contains('visible') : false);
  }
}

function setServiceStatus(id, message, isError) {
  var el = document.getElementById(id);
  el.textContent = message;
  el.className = 'service-status visible' + (isError ? ' error' : ' success');
}

document.getElementById('open-manage-service').onclick = function() {
  toggleServicePanel('manage-service-panel');
  document.getElementById('manage-inline-email').focus();
};

document.getElementById('open-retrieve-service').onclick = function() {
  toggleServicePanel('retrieve-service-panel');
  document.getElementById('retrieve-inline-email').focus();
};

document.getElementById('open-refund-service').onclick = function() {
  toggleServicePanel('refund-service-panel');
  document.getElementById('refund-inline-email').focus();
};

document.getElementById('open-data-service').onclick = function() {
  toggleServicePanel('data-service-panel');
  document.getElementById('data-export-email').focus();
};

var pendingManageEmail = '';
var pendingRetrieveEmail = '';
var pendingExportEmail = '';
var pendingDeleteEmail = '';

document.getElementById('manage-inline-request').onclick = async function() {
  var email = document.getElementById('manage-inline-email').value.trim();
  if (!email) { document.getElementById('manage-inline-email').focus(); return; }
  this.disabled = true;
  this.textContent = 'Sending...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/manage/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to send verification code');
    pendingManageEmail = email;
    document.getElementById('manage-inline-step2').style.display = 'block';
    setServiceStatus('manage-inline-status', 'Verification code sent. Check your email.', false);
  } catch (err) {
    setServiceStatus('manage-inline-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Send Verification Code';
  }
};

document.getElementById('manage-inline-resend').onclick = async function(e) {
  e.preventDefault();
  if (!pendingManageEmail) return;
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/manage/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingManageEmail })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to send verification code');
    setServiceStatus('manage-inline-status', 'New verification code sent.', false);
  } catch (err) {
    setServiceStatus('manage-inline-status', err.message, true);
  }
};

document.getElementById('manage-inline-confirm').onclick = async function() {
  var code = document.getElementById('manage-inline-code').value.trim();
  if (!pendingManageEmail || !code) return;
  this.disabled = true;
  this.textContent = 'Redirecting...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/manage', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingManageEmail, code: code })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to open customer portal');
    window.location.href = data.url;
  } catch (err) {
    setServiceStatus('manage-inline-status', err.message, true);
    this.disabled = false;
    this.textContent = 'Open Customer Portal';
  }
};

document.getElementById('retrieve-inline-request').onclick = async function() {
  var email = document.getElementById('retrieve-inline-email').value.trim();
  if (!email) { document.getElementById('retrieve-inline-email').focus(); return; }
  this.disabled = true;
  this.textContent = 'Sending...';
  document.getElementById('retrieve-inline-result').style.display = 'none';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/retrieve-license/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to send verification code');
    pendingRetrieveEmail = email;
    document.getElementById('retrieve-inline-step2').style.display = 'block';
    setServiceStatus('retrieve-inline-status', 'Verification code sent. Check your email.', false);
  } catch (err) {
    setServiceStatus('retrieve-inline-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Send Verification Code';
  }
};

document.getElementById('retrieve-inline-confirm').onclick = async function() {
  var code = document.getElementById('retrieve-inline-code').value.trim();
  if (!pendingRetrieveEmail || !code) return;
  this.disabled = true;
  this.textContent = 'Loading...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/retrieve-license', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingRetrieveEmail, code: code })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to retrieve license');
    var license = data.license;
    document.getElementById('retrieve-inline-token').value = license.token;
    document.getElementById('retrieve-inline-tier').textContent = license.tier;
    document.getElementById('retrieve-inline-issued').textContent = new Date(license.issued_at).toLocaleString();
    document.getElementById('retrieve-inline-expires').textContent = license.expires_at ? new Date(license.expires_at).toLocaleString() : 'Does not expire';
    document.getElementById('retrieve-inline-email-value').textContent = license.email;
    document.getElementById('retrieve-inline-result').style.display = 'block';
    document.getElementById('retrieve-inline-copy').style.display = 'inline-block';
    if (license.invoice_url) {
      var invoice = document.getElementById('retrieve-inline-invoice');
      invoice.href = license.invoice_url;
      invoice.style.display = 'inline-block';
    }
    setServiceStatus('retrieve-inline-status', 'License retrieved successfully.', false);
  } catch (err) {
    setServiceStatus('retrieve-inline-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Show License';
  }
};

document.getElementById('retrieve-inline-copy').onclick = async function() {
  var token = document.getElementById('retrieve-inline-token').value;
  if (!token) return;
  try {
    await navigator.clipboard.writeText(token);
    setServiceStatus('retrieve-inline-status', 'License key copied to clipboard.', false);
  } catch (_) {
    setServiceStatus('retrieve-inline-status', 'Failed to copy automatically. Please copy the key manually.', true);
  }
};

document.getElementById('refund-inline-submit').onclick = async function() {
  var email = document.getElementById('refund-inline-email').value.trim();
  var token = document.getElementById('refund-inline-token').value.trim();
  if (!email || !token) return;
  if (!confirm('Are you sure? This will immediately revoke the license and request the refund.')) return;
  this.disabled = true;
  this.textContent = 'Processing...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/self-refund', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email, token: token })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Refund failed');
    document.getElementById('refund-inline-token').value = '';
    setServiceStatus('refund-inline-status', 'Success! Your refund has been processed. Stripe will follow up by email.', false);
  } catch (err) {
    setServiceStatus('refund-inline-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Process Refund';
  }
};

document.getElementById('data-export-request').onclick = async function() {
  var email = document.getElementById('data-export-email').value.trim();
  if (!email) return;
  this.disabled = true;
  this.textContent = 'Sending...';
  document.getElementById('data-export-result').style.display = 'none';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/gdpr/request-export', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Request failed');
    pendingExportEmail = email;
    document.getElementById('data-export-step2').style.display = 'block';
    setServiceStatus('data-export-status', 'Verification code sent. Check your email.', false);
  } catch (err) {
    setServiceStatus('data-export-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Send Verification Code';
  }
};

document.getElementById('data-export-resend').onclick = async function(e) {
  e.preventDefault();
  if (!pendingExportEmail) return;
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/gdpr/request-export', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingExportEmail })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Request failed');
    setServiceStatus('data-export-status', 'New verification code sent.', false);
  } catch (err) {
    setServiceStatus('data-export-status', err.message, true);
  }
};

document.getElementById('data-export-confirm').onclick = async function() {
  var code = document.getElementById('data-export-code').value.trim();
  if (!pendingExportEmail || !code) return;
  this.disabled = true;
  this.textContent = 'Exporting...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/gdpr/export', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingExportEmail, code: code })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Export failed');
    document.getElementById('data-export-payload').value = JSON.stringify(data, null, 2);
    document.getElementById('data-export-result').style.display = 'block';
    setServiceStatus('data-export-status', 'Data export retrieved successfully.', false);
    document.getElementById('data-export-code').value = '';
    pendingExportEmail = '';
    document.getElementById('data-export-step2').style.display = 'none';
  } catch (err) {
    setServiceStatus('data-export-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Export My Data';
  }
};

document.getElementById('data-delete-request').onclick = async function() {
  var email = document.getElementById('data-delete-email').value.trim();
  if (!email) return;
  this.disabled = true;
  this.textContent = 'Sending...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/gdpr/request-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Request failed');
    pendingDeleteEmail = email;
    document.getElementById('data-delete-step2').style.display = 'block';
    setServiceStatus('data-delete-status', 'Verification code sent. Check your email.', false);
  } catch (err) {
    setServiceStatus('data-delete-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Send Verification Code';
  }
};

document.getElementById('data-delete-resend').onclick = async function(e) {
  e.preventDefault();
  if (!pendingDeleteEmail) return;
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/gdpr/request-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingDeleteEmail })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Request failed');
    setServiceStatus('data-delete-status', 'New verification code sent.', false);
  } catch (err) {
    setServiceStatus('data-delete-status', err.message, true);
  }
};

document.getElementById('data-delete-confirm').onclick = async function() {
  var code = document.getElementById('data-delete-code').value.trim();
  if (!pendingDeleteEmail || !code) return;
  if (!document.getElementById('data-delete-confirm-check').checked) {
    setServiceStatus('data-delete-status', 'You must confirm that you understand this action is permanent.', true);
    return;
  }
  this.disabled = true;
  this.textContent = 'Deleting...';
  try {
    var res = await fetch(LICENSE_API_BASE + '/v1/gdpr/confirm-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: pendingDeleteEmail, code: code })
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Deletion failed');
    setServiceStatus('data-delete-status', data.deleted_count > 0 && data.stripe_reminder ? data.message + ' ' + data.stripe_reminder : data.message, false);
    document.getElementById('data-delete-step2').style.display = 'none';
    document.getElementById('data-delete-code').value = '';
    document.getElementById('data-delete-confirm-check').checked = false;
    pendingDeleteEmail = '';
  } catch (err) {
    setServiceStatus('data-delete-status', err.message, true);
  } finally {
    this.disabled = false;
    this.textContent = 'Delete My Data';
  }
};

function toggleAddWorkspace(accountID) {
  var form = document.getElementById('add-ws-form-' + accountID);
  var visible = form.classList.contains('visible');
  form.classList.toggle('visible', !visible);
  if (!visible) {
    document.getElementById('ws-name-' + accountID).focus();
  }
}

async function createWorkspace(accountID) {
  var nameEl = document.getElementById('ws-name-' + accountID);
  var name = nameEl.value.trim();
  if (!name) { nameEl.focus(); return; }
  var spinner = document.getElementById('ws-spinner-' + accountID);
  spinner.style.display = 'block';
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
    showToast('Workspace created!');
    setTimeout(function() { window.location.reload(); }, 800);
  } catch(e) {
    showToast('Network error. Please try again.', true);
  } finally {
    spinner.style.display = 'none';
  }
}

function suspendOrDelete(evt, accountID, tenantID, state, name) {
  evt.stopPropagation();
  var action = state === 'active' ? 'Suspend' : 'Delete';
  if (!confirm(action + ' workspace "' + name + '"?')) return;
  var method = state === 'active' ? 'PATCH' : 'DELETE';
  var body = state === 'active' ? JSON.stringify({ state: 'suspended' }) : undefined;
  fetch(ACCOUNT_API_BASE_PATH + '/' + accountID + '/tenants/' + tenantID, {
    method: method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body
  }).then(function(r) {
    if (r.ok) {
      showToast(action + 'd workspace.');
      setTimeout(function() { window.location.reload(); }, 800);
    } else {
      showToast('Failed to ' + action.toLowerCase() + ' workspace.', true);
    }
  }).catch(function() {
    showToast('Network error.', true);
  });
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
  } catch(e) {
    showToast('Network error.', true);
  }
}

function toggleTeam(accountID) {
  var section = document.getElementById('team-section-' + accountID);
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
  var actorRole = section.getAttribute('data-actor-role') || '';
  var isOwner = actorRole === 'owner';
  setTbodyMessage(tbody, 'Loading\u2026', false);
  try {
    var r = await fetch(ACCOUNT_API_BASE_PATH + '/' + encodeURIComponent(accountID) + '/members');
    if (!r.ok) { setTbodyMessage(tbody, 'Failed to load team.', true); return; }
    var members = await r.json();
    if (!members || members.length === 0) {
      setTbodyMessage(tbody, 'No team members.', false);
      return;
    }
    var allRoles = ['owner','admin','tech','read_only'];
    var nonOwnerRoles = ['admin','tech','read_only'];
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
          sel.addEventListener('change', function() { changeRole(accountID, m.user_id, this.value); });
          tdRole.appendChild(sel);
        }
        tr.appendChild(tdRole);
        var tdAction = document.createElement('td');
        if (!(m.role === 'owner' && !isOwner)) {
          var btn = document.createElement('button');
          btn.className = 'btn-remove';
          btn.textContent = 'Remove';
          btn.addEventListener('click', function() { removeMember(accountID, m.user_id, m.email); });
          tdAction.appendChild(btn);
        }
        tr.appendChild(tdAction);
        tbody.appendChild(tr);
      })(members[i]);
    }
  } catch(e) {
    setTbodyMessage(tbody, 'Network error.', true);
  }
}

async function inviteMember(accountID) {
  var emailEl = document.getElementById('invite-email-' + accountID);
  var roleEl = document.getElementById('invite-role-' + accountID);
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
    showToast('Member invited!');
    loadTeam(accountID);
  } catch(e) {
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
    showToast('Role updated.');
  } catch(e) {
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
    showToast('Member removed.');
    loadTeam(accountID);
  } catch(e) {
    showToast('Network error.', true);
  }
}
