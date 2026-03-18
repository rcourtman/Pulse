package portal

import (
	"html/template"
	"net/http"
	"time"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

// portalPageWorkspace holds per-workspace display data for the portal template.
type portalPageWorkspace struct {
	ID          string
	DisplayName string
	State       string
	Healthy     bool
	CreatedAt   time.Time
}

// portalPageAccount holds per-account display data including the user's role.
type portalPageAccount struct {
	ID         string
	Kind       string
	KindLabel  string
	Name       string
	Role       string
	CanManage  bool // owner or admin
	HasBilling bool // true when a Stripe customer exists for the account
	Workspaces []portalPageWorkspace
}

// portalPageData is passed to the portal HTML template.
type portalPageData struct {
	Nonce    string
	Email    string
	Accounts []portalPageAccount
}

var portalPageTmpl = template.Must(template.New("portal").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pulse Cloud — Portal</title>
  <style nonce="{{.Nonce}}">
    :root { color-scheme: light; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f1f5f9; color: #0f172a; }
    header { background: #1e293b; color: #f8fafc; padding: 12px 24px; display: flex; align-items: center; justify-content: space-between; }
    header .brand { font-weight: 700; font-size: 18px; letter-spacing: -0.3px; }
    header .user-info { display: flex; align-items: center; gap: 16px; font-size: 13px; color: #94a3b8; }
    header .logout-btn { background: none; border: 1px solid #475569; color: #94a3b8; border-radius: 6px; padding: 5px 12px; cursor: pointer; font-size: 13px; }
    header .logout-btn:hover { border-color: #94a3b8; color: #f8fafc; }
    .main { max-width: 860px; margin: 32px auto; padding: 0 16px 48px; }
    .account-section { margin-bottom: 40px; }
    .account-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
    .account-header h2 { margin: 0; font-size: 20px; font-weight: 700; }
    .badge { font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 999px; text-transform: uppercase; letter-spacing: 0.5px; }
    .badge-msp { background: #dbeafe; color: #1e40af; }
    .badge-cloud { background: #dcfce7; color: #166534; }
    .badge-individual { background: #dcfce7; color: #166534; }
    .badge-healthy { background: #dcfce7; color: #166534; }
    .badge-unhealthy { background: #fee2e2; color: #991b1b; }
    .badge-active { background: #f0fdf4; color: #166534; border: 1px solid #bbf7d0; }
    .badge-suspended { background: #fef9c3; color: #854d0e; border: 1px solid #fef08a; }
    .badge-failed { background: #fee2e2; color: #991b1b; border: 1px solid #fecaca; }
    .badge-provisioning { background: #eff6ff; color: #1d4ed8; border: 1px solid #bfdbfe; }
    .badge-canceled { background: #f1f5f9; color: #64748b; border: 1px solid #e2e8f0; }
    .badge-deleting { background: #fef3c7; color: #92400e; border: 1px solid #fde68a; }
    .workspace-list { display: flex; flex-direction: column; gap: 10px; }
    .workspace-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 14px 18px; display: flex; align-items: center; justify-content: space-between; gap: 12px; box-shadow: 0 1px 3px rgba(15,23,42,.04); }
    .workspace-card:hover { border-color: #cbd5e1; }
    .ws-info { flex: 1; min-width: 0; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
    .ws-name { font-weight: 600; font-size: 15px; }
    .ws-meta { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
    .ws-created { font-size: 11px; color: #94a3b8; }
    .ws-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
    .btn-primary { background: #1d4ed8; color: #fff; border: 0; border-radius: 8px; padding: 8px 18px; font-size: 14px; font-weight: 600; cursor: pointer; text-decoration: none; display: inline-block; }
    .btn-primary:hover { background: #1e40af; }
    .btn-secondary { background: #fff; color: #334155; border: 1px solid #cbd5e1; border-radius: 8px; padding: 7px 14px; font-size: 13px; font-weight: 500; cursor: pointer; }
    .btn-secondary:hover { background: #f8fafc; border-color: #94a3b8; }
    .btn-danger { background: #fff; color: #dc2626; border: 1px solid #fca5a5; border-radius: 8px; padding: 7px 14px; font-size: 13px; cursor: pointer; }
    .btn-danger:hover { background: #fef2f2; }
    .account-actions { margin-top: 14px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .add-workspace-form { margin-top: 12px; display: none; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; padding: 16px; }
    .add-workspace-form.visible { display: block; }
    .add-workspace-form label { display: block; font-size: 13px; font-weight: 600; margin-bottom: 6px; }
    .add-workspace-form input { width: 100%; border: 1px solid #cbd5e1; border-radius: 8px; padding: 8px 12px; font-size: 14px; margin-bottom: 10px; }
    .add-workspace-form .form-actions { display: flex; gap: 8px; }
    .team-section { margin-top: 12px; display: none; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; padding: 16px; }
    .team-section.visible { display: block; }
    .team-section h3 { margin: 0 0 12px; font-size: 15px; font-weight: 700; }
    .team-table { width: 100%; border-collapse: collapse; font-size: 14px; }
    .team-table th { text-align: left; font-size: 12px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; padding: 6px 8px; border-bottom: 1px solid #e2e8f0; }
    .team-table td { padding: 8px; border-bottom: 1px solid #f1f5f9; vertical-align: middle; }
    .team-table select { border: 1px solid #cbd5e1; border-radius: 6px; padding: 4px 8px; font-size: 13px; background: #fff; }
    .team-table .btn-remove { background: none; border: none; color: #dc2626; cursor: pointer; font-size: 13px; padding: 4px 8px; }
    .team-table .btn-remove:hover { text-decoration: underline; }
    .team-invite { margin-top: 12px; display: flex; gap: 8px; align-items: flex-end; flex-wrap: wrap; }
    .team-invite label { font-size: 12px; font-weight: 600; display: block; margin-bottom: 4px; }
    .team-invite input { border: 1px solid #cbd5e1; border-radius: 8px; padding: 8px 12px; font-size: 14px; min-width: 200px; }
    .team-invite select { border: 1px solid #cbd5e1; border-radius: 6px; padding: 8px 10px; font-size: 14px; background: #fff; }
    .empty-state { background: #fff; border: 1px dashed #cbd5e1; border-radius: 10px; padding: 32px; text-align: center; color: #64748b; }
    .empty-state p { margin: 0; font-size: 15px; }
    .spinner { display: none; width: 18px; height: 18px; border: 2px solid #93c5fd; border-top-color: #1d4ed8; border-radius: 50%; animation: spin 0.6s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    .toast { position: fixed; bottom: 24px; right: 24px; background: #1e293b; color: #f8fafc; border-radius: 8px; padding: 12px 20px; font-size: 14px; display: none; z-index: 100; }
    .toast.visible { display: block; animation: fadein 0.2s; }
    @keyframes fadein { from { opacity: 0; transform: translateY(8px); } }
    .toast.error { background: #991b1b; }
    @media (max-width: 560px) {
      .workspace-card { flex-direction: column; align-items: flex-start; }
      .ws-actions { align-self: stretch; }
      .ws-actions form, .ws-actions .btn-primary { width: 100%; text-align: center; }
    }
  </style>
</head>
<body>
<header>
  <span class="brand">Pulse Cloud</span>
  <div class="user-info">
    <span>{{.Email}}</span>
    <button class="logout-btn" id="logout-btn">Sign out</button>
  </div>
</header>

<main class="main">
  {{if eq (len .Accounts) 0}}
  <div class="empty-state" style="margin-top:48px">
    <p>No workspaces found. If you just signed up, check your email for setup instructions.</p>
    <p style="margin-top:12px;font-size:13px">Need help? Contact <a href="mailto:support@pulserelay.pro" style="color:#1d4ed8">support@pulserelay.pro</a></p>
  </div>
  {{else}}
  {{range $ai, $account := .Accounts}}
  <section class="account-section">
    <div class="account-header">
      <h2>{{$account.Name}}</h2>
      <span class="badge badge-{{$account.Kind}}">{{$account.KindLabel}}</span>
      {{if eq $account.Role "owner"}}<span class="badge" style="background:#f1f5f9;color:#64748b">Owner</span>
      {{else if eq $account.Role "admin"}}<span class="badge" style="background:#f1f5f9;color:#64748b">Admin</span>
      {{else if eq $account.Role "tech"}}<span class="badge" style="background:#f1f5f9;color:#64748b">Tech</span>{{end}}
    </div>

    {{if eq (len $account.Workspaces) 0}}
    <div class="empty-state"><p>No workspaces yet. Create one to get started.</p></div>
    {{else}}
    <div class="workspace-list">
      {{range $wi, $ws := $account.Workspaces}}
      <div class="workspace-card">
        <div class="ws-info">
          <span class="ws-name">{{$ws.DisplayName}}</span>
          <div class="ws-meta">
            {{if $ws.Healthy}}
            <span class="badge badge-healthy">Healthy</span>
            {{else}}
            <span class="badge badge-unhealthy">Checking</span>
            {{end}}
            <span class="badge badge-{{$ws.State}}">{{$ws.State}}</span>
            <span class="ws-created">Created {{$ws.CreatedAt.Format "Jan 2, 2006"}}</span>
          </div>
        </div>
        <div class="ws-actions">
          {{if eq $ws.State "active"}}
          <form method="POST" action="/api/accounts/{{$account.ID}}/tenants/{{$ws.ID}}/handoff">
            <button type="submit" class="btn-primary">Open →</button>
          </form>
          {{else}}
          <span style="font-size:13px;color:#94a3b8">{{$ws.State}}</span>
          {{end}}
          {{if and $account.CanManage (or (eq $ws.State "active") (eq $ws.State "suspended") (eq $ws.State "failed"))}}
          <button class="btn-danger" onclick="suspendOrDelete(event,'{{$account.ID}}','{{$ws.ID}}','{{$ws.State}}','{{$ws.DisplayName}}')">⋯</button>
          {{end}}
        </div>
      </div>
      {{end}}
    </div>
    {{end}}

    {{if $account.CanManage}}
    <div class="account-actions">
      {{if eq $account.Kind "msp"}}
      <button class="btn-secondary" id="add-ws-btn-{{$account.ID}}" onclick="toggleAddWorkspace('{{$account.ID}}')">+ Add workspace</button>
      {{end}}
      {{if $account.HasBilling}}
      <button class="btn-secondary" onclick="openBilling('{{$account.ID}}')">Manage billing</button>
      {{end}}
      <button class="btn-secondary" id="team-btn-{{$account.ID}}" onclick="toggleTeam('{{$account.ID}}','{{$account.Role}}')">Manage team</button>
    </div>

    <div class="team-section" id="team-section-{{$account.ID}}" data-actor-role="{{$account.Role}}">
      <h3>Team members</h3>
      <table class="team-table">
        <thead><tr><th>Email</th><th>Role</th><th></th></tr></thead>
        <tbody id="team-list-{{$account.ID}}">
          <tr><td colspan="3" style="color:#94a3b8;text-align:center;padding:16px">Loading…</td></tr>
        </tbody>
      </table>
      <div class="team-invite">
        <div><label for="invite-email-{{$account.ID}}">Email</label><input type="email" id="invite-email-{{$account.ID}}" placeholder="user@example.com" autocomplete="off"></div>
        <div><label for="invite-role-{{$account.ID}}">Role</label><select id="invite-role-{{$account.ID}}"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div>
        <button class="btn-primary" style="padding:8px 14px;font-size:13px" onclick="inviteMember('{{$account.ID}}')">Invite</button>
      </div>
    </div>

    {{if eq $account.Kind "msp"}}
    <div class="add-workspace-form" id="add-ws-form-{{$account.ID}}">
      <label for="ws-name-{{$account.ID}}">Workspace name (e.g. client name)</label>
      <input type="text" id="ws-name-{{$account.ID}}" placeholder="Acme Corp" maxlength="80" autocomplete="off">
      <div class="form-actions">
        <button class="btn-primary" onclick="createWorkspace('{{$account.ID}}')">Create workspace</button>
        <button class="btn-secondary" onclick="toggleAddWorkspace('{{$account.ID}}')">Cancel</button>
        <div class="spinner" id="ws-spinner-{{$account.ID}}"></div>
      </div>
    </div>
    {{end}}
    {{end}}
  </section>
  {{end}}
  {{end}}
</main>

<div class="toast" id="toast"></div>

<script nonce="{{.Nonce}}">
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
    await fetch('/auth/logout', { method: 'POST' });
  } catch(_) {}
  window.location.href = '/portal';
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
    var resp = await fetch('/api/accounts/' + accountID + '/tenants', {
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
  fetch('/api/accounts/' + accountID + '/tenants/' + tenantID, {
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
    var r = await fetch('/api/portal/billing?account_id=' + encodeURIComponent(accountID), { method: 'POST' });
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
    var r = await fetch('/api/accounts/' + encodeURIComponent(accountID) + '/members');
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
    var r = await fetch('/api/accounts/' + encodeURIComponent(accountID) + '/members', {
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
    var r = await fetch('/api/accounts/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
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
    var r = await fetch('/api/accounts/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
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
</script>
</body>
</html>
`))

var loginPageTmpl = template.Must(template.New("portal-login").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pulse Cloud — Sign In</title>
  <style nonce="{{.Nonce}}">
    :root { color-scheme: light; }
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: linear-gradient(140deg, #f8fafc, #e2e8f0); color: #0f172a; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
    .card { background: #fff; border-radius: 12px; border: 1px solid #e2e8f0; box-shadow: 0 8px 30px rgba(15,23,42,.08); padding: 32px 28px; width: 100%; max-width: 400px; margin: 16px; }
    .brand { font-size: 22px; font-weight: 700; margin-bottom: 6px; }
    .subtitle { color: #64748b; font-size: 14px; margin-bottom: 24px; }
    label { display: block; font-size: 14px; font-weight: 600; margin-bottom: 6px; }
    input { width: 100%; border: 1px solid #cbd5e1; border-radius: 8px; padding: 10px 12px; font-size: 15px; }
    input:focus { outline: 2px solid #1d4ed8; border-color: transparent; }
    .cta { margin-top: 14px; border: 0; border-radius: 10px; background: #1d4ed8; color: #fff; font-size: 15px; font-weight: 600; padding: 11px 16px; width: 100%; cursor: pointer; }
    .cta:hover { background: #1e40af; }
    .cta:disabled { opacity: 0.6; cursor: default; }
    .success { background: #f0fdf4; border: 1px solid #bbf7d0; border-radius: 8px; padding: 14px; color: #166534; font-size: 14px; line-height: 1.5; display: none; margin-top: 14px; }
    .error { background: #fef2f2; border: 1px solid #fecaca; border-radius: 8px; padding: 12px; color: #991b1b; font-size: 14px; display: none; margin-top: 14px; }
    .footer { margin-top: 20px; text-align: center; font-size: 12px; color: #94a3b8; }
    .footer a { color: #1d4ed8; text-decoration: none; }
  </style>
</head>
<body>
<div class="card">
  <div class="brand">Pulse Cloud</div>
  <div class="subtitle">Sign in to manage your workspaces</div>
  <label for="email">Email address</label>
  <input id="email" type="email" autocomplete="email" placeholder="you@example.com" />
  <button class="cta" id="send-btn">Send magic link</button>
  <div class="success" id="success">
    Magic link sent! Check your inbox and click the link to sign in.
    <br><br>
    <strong>Don't see it?</strong> Check your spam folder, or <a href="#" id="resend-link">send a new link</a>.
  </div>
  <div class="error" id="err-msg"></div>
  <div class="footer">
    New here? <a href="/signup">Create an account</a>
  </div>
</div>
<script nonce="{{.Nonce}}">
var emailEl = document.getElementById('email');
var sendBtn = document.getElementById('send-btn');
var successEl = document.getElementById('success');
var errEl = document.getElementById('err-msg');

async function sendMagicLink() {
  var email = emailEl.value.trim();
  if (!email) { emailEl.focus(); return; }
  sendBtn.disabled = true;
  sendBtn.textContent = 'Sending…';
  errEl.style.display = 'none';
  try {
    var r = await fetch('/api/public/magic-link/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    if (r.ok || r.status === 404) {
      // 404 = email not found, but we don't reveal that (rate-limiting still applies)
      successEl.style.display = 'block';
      sendBtn.style.display = 'none';
      emailEl.style.display = 'none';
      document.querySelector('label').style.display = 'none';
    } else if (r.status === 429) {
      errEl.textContent = 'Too many requests. Please wait a moment and try again.';
      errEl.style.display = 'block';
      sendBtn.disabled = false;
      sendBtn.textContent = 'Send magic link';
    } else {
      errEl.textContent = 'Something went wrong. Please try again.';
      errEl.style.display = 'block';
      sendBtn.disabled = false;
      sendBtn.textContent = 'Send magic link';
    }
  } catch(_) {
    errEl.textContent = 'Network error. Please check your connection and try again.';
    errEl.style.display = 'block';
    sendBtn.disabled = false;
    sendBtn.textContent = 'Send magic link';
  }
}

sendBtn.onclick = sendMagicLink;
emailEl.addEventListener('keydown', function(e) { if (e.key === 'Enter') sendMagicLink(); });
document.getElementById('resend-link').onclick = function(e) {
  e.preventDefault();
  successEl.style.display = 'none';
  sendBtn.style.display = '';
  emailEl.style.display = '';
  document.querySelector('label').style.display = '';
  sendBtn.disabled = false;
  sendBtn.textContent = 'Send magic link';
};
</script>
</body>
</html>
`))

// HandlePortalPage serves the MSP/Cloud portal dashboard (browser-facing HTML).
// Route: GET /portal
//   - No session or invalid session → shows a magic-link login form
//   - Valid session → shows workspace list with management actions
func HandlePortalPage(sessionSvc *cpauth.Service, reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nonce := cpsec.NonceFromContext(r.Context())

		// Validate session from cookie or Bearer token.
		token := cpauth.SessionTokenFromRequest(r)
		if token == "" || sessionSvc == nil || reg == nil {
			renderLoginPage(w, nonce)
			return
		}

		claims, err := sessionSvc.ValidateSessionToken(token)
		if err != nil {
			renderLoginPage(w, nonce)
			return
		}
		sessionVersion, err := reg.GetUserSessionVersion(claims.UserID)
		if err != nil {
			log.Error().Err(err).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: get session version")
			renderLoginPage(w, nonce)
			return
		}
		if claims.SessionVersion != sessionVersion {
			renderLoginPage(w, nonce)
			return
		}

		// Look up which accounts this user belongs to.
		accountIDs, err := reg.ListAccountsByUser(claims.UserID)
		if err != nil {
			log.Error().Err(err).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: list accounts by user")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Build display data for each account.
		accounts := make([]portalPageAccount, 0, len(accountIDs))
		for _, accountID := range accountIDs {
			a, err := reg.GetAccount(accountID)
			if err != nil {
				log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal.page: get account")
				continue
			}
			if a == nil {
				continue
			}

			m, err := reg.GetMembership(accountID, claims.UserID)
			if err != nil {
				log.Error().Err(err).Str("account_id", accountID).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: get membership")
				continue
			}
			if m == nil {
				continue
			}

			tenants, err := reg.ListByAccountID(accountID)
			if err != nil {
				log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal.page: list tenants")
				continue
			}

			workspaces := make([]portalPageWorkspace, 0, len(tenants))
			for _, t := range tenants {
				if t == nil {
					continue
				}
				// Skip tenants that are gone or going away.
				if t.State == registry.TenantStateDeleted || t.State == registry.TenantStateDeleting {
					continue
				}
				workspaces = append(workspaces, portalPageWorkspace{
					ID:          t.ID,
					DisplayName: t.DisplayName,
					State:       string(t.State),
					Healthy:     t.HealthCheckOK,
					CreatedAt:   t.CreatedAt,
				})
			}

			kindLabel := "Cloud"
			if a.Kind == registry.AccountKindMSP {
				kindLabel = "MSP"
			}

			hasBilling := false
			if sa, saErr := reg.GetStripeAccount(accountID); saErr != nil {
				log.Warn().Err(saErr).Str("account_id", accountID).Msg("cloudcp.portal.page: lookup stripe account for billing button")
			} else if sa != nil && sa.StripeCustomerID != "" {
				hasBilling = true
			}

			accounts = append(accounts, portalPageAccount{
				ID:         a.ID,
				Kind:       string(a.Kind),
				KindLabel:  kindLabel,
				Name:       a.DisplayName,
				Role:       string(m.Role),
				CanManage:  m.Role == registry.MemberRoleOwner || m.Role == registry.MemberRoleAdmin,
				HasBilling: hasBilling,
				Workspaces: workspaces,
			})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := portalPageTmpl.Execute(w, portalPageData{
			Nonce:    nonce,
			Email:    claims.Email,
			Accounts: accounts,
		}); err != nil {
			log.Error().Err(err).Msg("cloudcp.portal.page: render portal page")
		}
	}
}

func renderLoginPage(w http.ResponseWriter, nonce string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := loginPageTmpl.Execute(w, struct{ Nonce string }{Nonce: nonce}); err != nil {
		log.Error().Err(err).Msg("cloudcp.portal.page: render login page")
	}
}
