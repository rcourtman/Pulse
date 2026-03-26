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
    manageAction = '<button type="button" class="btn-danger" data-action="workspace-manage" data-account-id="' + escapeAttr(account.id) + '" data-workspace-id="' + escapeAttr(workspace.id) + '" data-workspace-state="' + escapeAttr(state) + '" data-workspace-name="' + escapeAttr(workspace.display_name) + '">⋯</button>';
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
        ? '<button type="button" class="btn-secondary" id="add-ws-btn-' + escapeAttr(account.id) + '" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">+ Add workspace</button>'
        : '') +
      (account.has_billing
        ? '<button type="button" class="btn-secondary" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Manage billing</button>'
        : '') +
      '<button type="button" class="btn-secondary" id="team-btn-' + escapeAttr(account.id) + '" data-action="toggle-team" data-account-id="' + escapeAttr(account.id) + '">Manage team</button>' +
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
          '<button type="button" class="btn-primary" style="padding:8px 14px;font-size:13px" data-action="invite-member" data-account-id="' + escapeAttr(account.id) + '">Invite</button>' +
        '</div>' +
      '</div>';

    if (account.kind === 'msp') {
      addWorkspaceForm = '' +
        '<div class="add-workspace-form" id="add-ws-form-' + escapeAttr(account.id) + '">' +
          '<label for="ws-name-' + escapeAttr(account.id) + '">Workspace name (e.g. client name)</label>' +
          '<input type="text" id="ws-name-' + escapeAttr(account.id) + '" placeholder="Acme Corp" maxlength="80" autocomplete="off">' +
          '<div class="form-actions">' +
            '<button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' + escapeAttr(account.id) + '">Create workspace</button>' +
            '<button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Cancel</button>' +
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
  if (!BOOTSTRAP_PATH) return false;
  try {
    var response = await fetch(BOOTSTRAP_PATH, {
      headers: { 'Accept': 'application/json' }
    });
    if (!response.ok) return false;
    var data = await response.json();
    applyBootstrap(data);
    return true;
  } catch (_) {}
  return false;
}

function showToast(msg, isError) {
  var t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast visible' + (isError ? ' error' : '');
  clearTimeout(t._timer);
  t._timer = setTimeout(function() { t.className = 'toast'; }, 4000);
}

window.PulseAccountPortal = {
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

document.addEventListener('click', function(event) {
  var actionEl = event.target.closest('[data-action]');
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
  var target = event.target;
  if (!target || target.getAttribute('data-action') !== 'change-role') return;
  changeRole(
    target.getAttribute('data-account-id') || '',
    target.getAttribute('data-user-id') || '',
    target.value
  );
});

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
    if (!await refreshBootstrap()) {
      window.location.href = PORTAL_PATH;
      return;
    }
    showToast('Workspace created!');
  } catch(e) {
    showToast('Network error. Please try again.', true);
  } finally {
    spinner.style.display = 'none';
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
  } catch(_) {
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
  } catch(e) {
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
  } catch(e) {
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
    if (!await refreshAccountTeamSection(accountID)) {
      return;
    }
    showToast('Member invited!');
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
    if (!await refreshAccountTeamSection(accountID)) {
      return;
    }
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
    if (!await refreshAccountTeamSection(accountID)) {
      return;
    }
    showToast('Member removed.');
  } catch(e) {
    showToast('Network error.', true);
  }
}
