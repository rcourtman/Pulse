const LICENSE_API_BASE = 'https://license.pulserelay.pro';

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
