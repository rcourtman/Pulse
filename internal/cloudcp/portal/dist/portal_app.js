(() => {
  // src/runtime.ts
  function readEmbeddedBootstrap() {
    const bootstrapEl = document.getElementById("pulse-account-bootstrap");
    if (!bootstrapEl) {
      return {};
    }
    try {
      return JSON.parse(bootstrapEl.textContent || "{}");
    } catch {
      return {};
    }
  }
  var embeddedBootstrap = readEmbeddedBootstrap();
  var bootstrapDefaults = {
    public_site_url: embeddedBootstrap.public_site_url || "https://pulserelay.pro",
    support_email: embeddedBootstrap.support_email || "support@pulserelay.pro",
    commercial_api_base_url: embeddedBootstrap.commercial_api_base_url || "",
    portal_path: embeddedBootstrap.portal_path || "/portal",
    bootstrap_path: embeddedBootstrap.bootstrap_path || "/api/portal/bootstrap",
    magic_link_request_path: embeddedBootstrap.magic_link_request_path || "/api/public/magic-link/request",
    signup_path: embeddedBootstrap.signup_path || "/signup",
    logout_path: embeddedBootstrap.logout_path || "/auth/logout",
    account_api_base_path: embeddedBootstrap.account_api_base_path || "/api/accounts",
    portal_api_base_path: embeddedBootstrap.portal_api_base_path || "/api/portal"
  };
  function normalizeAccounts(accounts) {
    return Array.isArray(accounts) ? accounts : [];
  }
  function createAnonymousBootstrap(overrides = {}) {
    return {
      authenticated: false,
      email: "",
      ...bootstrapDefaults,
      ...overrides,
      accounts: normalizeAccounts(overrides.accounts)
    };
  }
  function normalizeBootstrap(raw) {
    return createAnonymousBootstrap(raw || {});
  }
  var bootstrapState = normalizeBootstrap(embeddedBootstrap);
  var renderSubscribers = /* @__PURE__ */ new Set();
  function getBootstrap() {
    return bootstrapState;
  }
  function setBootstrap(nextBootstrap) {
    bootstrapState = normalizeBootstrap(nextBootstrap);
    return bootstrapState;
  }
  function getCommercialAPIBaseURL() {
    return bootstrapState.commercial_api_base_url;
  }
  function getPortalPath() {
    return bootstrapState.portal_path;
  }
  function getBootstrapPath() {
    return bootstrapState.bootstrap_path;
  }
  function getMagicLinkRequestPath() {
    return bootstrapState.magic_link_request_path;
  }
  function getSignupPath() {
    return bootstrapState.signup_path;
  }
  function getLogoutPath() {
    return bootstrapState.logout_path;
  }
  function getAccountAPIBasePath() {
    return bootstrapState.account_api_base_path;
  }
  function getPortalAPIBasePath() {
    return bootstrapState.portal_api_base_path;
  }
  function notifyPortalRender() {
    renderSubscribers.forEach((listener) => {
      listener();
    });
  }
  function subscribePortalRender(listener) {
    renderSubscribers.add(listener);
    return () => {
      renderSubscribers.delete(listener);
    };
  }

  // src/shell.ts
  var portalBootstrap = getBootstrap();
  var LICENSE_API_BASE = getCommercialAPIBaseURL();
  var PORTAL_PATH = getPortalPath();
  var BOOTSTRAP_PATH = getBootstrapPath();
  var MAGIC_LINK_REQUEST_PATH = getMagicLinkRequestPath();
  var SIGNUP_PATH = getSignupPath();
  var LOGOUT_PATH = getLogoutPath();
  var ACCOUNT_API_BASE_PATH = getAccountAPIBasePath();
  var PORTAL_API_BASE_PATH = getPortalAPIBasePath();
  var loginState = {
    emailValue: "",
    sending: false,
    success: false,
    error: ""
  };
  function getElement(id) {
    return document.getElementById(id);
  }
  function asHTMLElement(target) {
    return target instanceof HTMLElement ? target : null;
  }
  function escapeHTML(value) {
    return String(value || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
  function escapeAttr(value) {
    return escapeHTML(value);
  }
  function formatWorkspaceDate(value) {
    if (!value) return "";
    var date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";
    return date.toLocaleDateString(void 0, { month: "short", day: "numeric", year: "numeric" });
  }
  function roleBadgeHTML(role) {
    if (role === "owner") return '<span class="badge" style="background:#f1f5f9;color:#64748b">Owner</span>';
    if (role === "admin") return '<span class="badge" style="background:#f1f5f9;color:#64748b">Admin</span>';
    if (role === "tech") return '<span class="badge" style="background:#f1f5f9;color:#64748b">Tech</span>';
    return "";
  }
  function renderHeader() {
    var userInfo = document.getElementById("portal-user-info");
    if (!userInfo) return;
    if (portalBootstrap.authenticated) {
      userInfo.innerHTML = "<span>" + escapeHTML(portalBootstrap.email || "") + '</span><button class="logout-btn" id="logout-btn" type="button">Sign out</button>';
      return;
    }
    userInfo.innerHTML = '<a class="logout-btn" href="' + escapeAttr(SIGNUP_PATH) + '" style="text-decoration:none">Create account</a>';
  }
  function renderWorkspaceCard(account, workspace) {
    var state = String(workspace.state || "");
    var safeState = escapeHTML(state);
    var createdLabel = formatWorkspaceDate(workspace.created_at);
    var openAction = "";
    if (state === "active") {
      openAction = '<form method="POST" action="' + escapeAttr(ACCOUNT_API_BASE_PATH + "/" + account.id + "/tenants/" + workspace.id + "/handoff") + '"><button type="submit" class="btn-primary">Open \u2192</button></form>';
    } else {
      openAction = '<span style="font-size:13px;color:#94a3b8">' + safeState + "</span>";
    }
    var manageAction = "";
    if (account.can_manage && (state === "active" || state === "suspended" || state === "failed")) {
      manageAction = '<button type="button" class="btn-danger" data-action="workspace-manage" data-account-id="' + escapeAttr(account.id) + '" data-workspace-id="' + escapeAttr(workspace.id) + '" data-workspace-state="' + escapeAttr(state) + '" data-workspace-name="' + escapeAttr(workspace.display_name) + '">\u22EF</button>';
    }
    var createdMeta = createdLabel ? '<span class="ws-created">Created ' + escapeHTML(createdLabel) + "</span>" : "";
    return '<div class="workspace-card"><div class="ws-info"><span class="ws-name">' + escapeHTML(workspace.display_name) + '</span><div class="ws-meta">' + (workspace.healthy ? '<span class="badge badge-healthy">Healthy</span>' : '<span class="badge badge-unhealthy">Checking</span>') + '<span class="badge badge-' + safeState + '">' + safeState + "</span>" + createdMeta + '</div></div><div class="ws-actions">' + openAction + manageAction + "</div></div>";
  }
  function renderAccountSection(account) {
    var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
    var workspaceHTML = "";
    if (workspaces.length === 0) {
      workspaceHTML = '<div class="empty-state"><p>No workspaces yet. Create one to get started.</p></div>';
    } else {
      workspaceHTML = '<div class="workspace-list">' + workspaces.map(function(workspace) {
        return renderWorkspaceCard(account, workspace);
      }).join("") + "</div>";
    }
    var actions = "";
    var teamSection = "";
    var addWorkspaceForm = "";
    if (account.can_manage) {
      actions = '<div class="account-actions">' + (account.kind === "msp" ? '<button type="button" class="btn-secondary" id="add-ws-btn-' + escapeAttr(account.id) + '" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">+ Add workspace</button>' : "") + (account.has_billing ? '<button type="button" class="btn-secondary" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Manage billing</button>' : "") + '<button type="button" class="btn-secondary" id="team-btn-' + escapeAttr(account.id) + '" data-action="toggle-team" data-account-id="' + escapeAttr(account.id) + '">Manage team</button></div>';
      teamSection = '<div class="team-section" id="team-section-' + escapeAttr(account.id) + '" data-actor-role="' + escapeAttr(account.role) + '"><h3>Team members</h3><table class="team-table"><thead><tr><th>Email</th><th>Role</th><th></th></tr></thead><tbody id="team-list-' + escapeAttr(account.id) + '"><tr><td colspan="3" style="color:#94a3b8;text-align:center;padding:16px">Loading\u2026</td></tr></tbody></table><div class="team-invite"><div><label for="invite-email-' + escapeAttr(account.id) + '">Email</label><input type="email" id="invite-email-' + escapeAttr(account.id) + '" placeholder="user@example.com" autocomplete="off"></div><div><label for="invite-role-' + escapeAttr(account.id) + '">Role</label><select id="invite-role-' + escapeAttr(account.id) + '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div><button type="button" class="btn-primary" style="padding:8px 14px;font-size:13px" data-action="invite-member" data-account-id="' + escapeAttr(account.id) + '">Invite</button></div></div>';
      if (account.kind === "msp") {
        addWorkspaceForm = '<div class="add-workspace-form" id="add-ws-form-' + escapeAttr(account.id) + '"><label for="ws-name-' + escapeAttr(account.id) + '">Workspace name (e.g. client name)</label><input type="text" id="ws-name-' + escapeAttr(account.id) + '" placeholder="Acme Corp" maxlength="80" autocomplete="off"><div class="form-actions"><button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' + escapeAttr(account.id) + '">Create workspace</button><button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Cancel</button><div class="spinner" id="ws-spinner-' + escapeAttr(account.id) + '"></div></div></div>';
      }
    }
    return '<section class="account-section"><div class="account-header"><h2>' + escapeHTML(account.name) + '</h2><span class="badge badge-' + escapeHTML(account.kind) + '">' + escapeHTML(account.kind_label) + "</span>" + roleBadgeHTML(account.role) + "</div>" + workspaceHTML + actions + teamSection + addWorkspaceForm + "</section>";
  }
  function renderAccounts(accounts) {
    var root = document.getElementById("accounts-root");
    if (!root) return;
    var safeAccounts = Array.isArray(accounts) ? accounts : [];
    if (safeAccounts.length === 0) {
      root.innerHTML = '<div class="empty-state" style="margin-top:48px"><p>No workspaces found. If you just signed up, check your email for setup instructions.</p><p style="margin-top:12px;font-size:13px">Need help? Contact <a href="mailto:' + escapeAttr(portalBootstrap.support_email || "") + '" style="color:#1d4ed8">' + escapeHTML(portalBootstrap.support_email || "") + "</a></p></div>";
      return;
    }
    root.innerHTML = safeAccounts.map(renderAccountSection).join("");
  }
  function renderAuthenticatedPortal() {
    return '<section class="intro-card"><h1>Pulse Account</h1><p>Manage Cloud workspaces, MSP access, and self-hosted commercial account services from one account surface. Hosted workspace lifecycle lives here today, and the self-hosted billing, license recovery, refund, and privacy tools below now share the same Pulse Account shell instead of staying fragmented across public utility pages.</p></section><div id="accounts-root"></div><section class="service-section"><div class="service-header"><h2>Other account services</h2><div class="service-note">Self-hosted commercial account actions now live here. The public utility pages remain as compatibility entry points, not the primary account surface.</div></div><div class="service-grid"><button class="service-card service-card-button" type="button" id="open-manage-service" data-account-service-action="open-service-panel" data-account-service-panel="manage-service-panel" data-account-service-focus="manage-inline-email"><h3>Manage subscriptions</h3><p>Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.</p></button><button class="service-card service-card-button" type="button" id="open-retrieve-service" data-account-service-action="open-service-panel" data-account-service-panel="retrieve-service-panel" data-account-service-focus="retrieve-inline-email"><h3>Retrieve licenses</h3><p>Recover the latest active self-hosted license and invoice link for a commercial email address.</p></button><button class="service-card service-card-button" type="button" id="open-refund-service" data-account-service-action="open-service-panel" data-account-service-panel="refund-service-panel" data-account-service-focus="refund-inline-email"><h3>Refund requests</h3><p>Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.</p></button><button class="service-card service-card-button" type="button" id="open-data-service" data-account-service-action="open-service-panel" data-account-service-panel="data-service-panel" data-account-service-focus="data-export-email"><h3>Data and privacy</h3><p>Request commercial data export or deletion without leaving the account shell.</p></button></div><div class="service-panel" id="manage-service-panel"><div id="manage-service-root"></div></div><div class="service-panel" id="retrieve-service-panel"><div id="retrieve-service-root"></div></div><div class="service-panel" id="refund-service-panel"><div id="refund-service-root"></div></div><div class="service-panel" id="data-service-panel"><h3>Data and privacy</h3><p>Request export or deletion of the commercial data tied to an email address. Payment data held directly by Stripe still requires support handling.</p><div class="subsection"><div id="data-export-root"></div></div><div class="subsection"><div id="data-delete-root"></div></div><div class="helper-text">Payment-card data stays with Stripe. For Stripe deletion support, contact <a href="mailto:' + escapeAttr(portalBootstrap.support_email || "") + '">' + escapeHTML(portalBootstrap.support_email || "") + "</a>.</div></div></section>";
  }
  function renderSignedOutPortal() {
    var statusHTML = "";
    if (loginState.error) {
      statusHTML = '<div class="service-status visible error">' + escapeHTML(loginState.error) + "</div>";
    } else if (loginState.success) {
      statusHTML = `<div class="service-status visible success">Magic link sent. Check your inbox and click the link to sign in.<br><br><strong>Don't see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.</div>`;
    }
    return '<section class="intro-card"><h1>Pulse Account</h1><p>Sign in to manage Cloud workspaces, MSP access, and commercial account services from one account surface.</p></section><section class="service-section"><div class="service-panel visible"><h3>Sign in</h3><p>Enter the commercial email address for your Pulse account. I will send a magic link so you can open Pulse Account without managing a password.</p><div class="form-group"><label for="portal-login-email">Email address</label><input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' + escapeAttr(loginState.emailValue || "") + '" data-portal-input="login-email"></div><div class="form-actions"><button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' + (loginState.sending ? "Sending\u2026" : "Send magic link") + '</button><a class="btn-secondary" href="' + escapeAttr(SIGNUP_PATH) + '" style="text-decoration:none">Create an account</a></div>' + statusHTML + "</div></section>";
  }
  function renderPortalApp() {
    renderHeader();
    var root = document.getElementById("portal-app-root");
    if (!root) return;
    root.innerHTML = portalBootstrap.authenticated ? renderAuthenticatedPortal() : renderSignedOutPortal();
    if (portalBootstrap.authenticated) {
      renderAccounts(portalBootstrap.accounts || []);
    }
    notifyPortalRender();
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
      loginState.emailValue = portalBootstrap.email || "";
    }
    renderPortalApp();
  }
  async function refreshBootstrap() {
    if (!BOOTSTRAP_PATH) return false;
    try {
      var response = await fetch(BOOTSTRAP_PATH, {
        headers: { "Accept": "application/json" }
      });
      if (response.status === 401) {
        applyBootstrap(createAnonymousBootstrap());
        return true;
      }
      if (!response.ok) return false;
      var data = await response.json();
      applyBootstrap(data);
      return true;
    } catch (_) {
    }
    return false;
  }
  function showToast(msg, isError = false) {
    var t = getElement("toast");
    if (!t) return;
    t.textContent = msg;
    t.className = "toast visible" + (isError ? " error" : "");
    clearTimeout(t._timer);
    t._timer = setTimeout(function() {
      t.className = "toast";
    }, 4e3);
  }
  async function sendMagicLink() {
    var email = String(loginState.emailValue || "").trim();
    if (!email) {
      var input = getElement("portal-login-email");
      if (input) input.focus();
      return;
    }
    loginState.sending = true;
    loginState.error = "";
    loginState.success = false;
    renderPortalApp();
    try {
      var response = await fetch(MAGIC_LINK_REQUEST_PATH, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email })
      });
      if (response.ok || response.status === 404) {
        loginState.sending = false;
        loginState.success = true;
        renderPortalApp();
        return;
      }
      if (response.status === 429) {
        loginState.error = "Too many requests. Please wait a moment and try again.";
      } else {
        loginState.error = "Something went wrong. Please try again.";
      }
    } catch (_) {
      loginState.error = "Network error. Please check your connection and try again.";
    }
    loginState.sending = false;
    renderPortalApp();
  }
  document.addEventListener("click", function(event) {
    var portalActionEl = asHTMLElement(event.target)?.closest("[data-portal-action]");
    if (portalActionEl) {
      var portalAction = portalActionEl.getAttribute("data-portal-action") || "";
      switch (portalAction) {
        case "send-magic-link":
          event.preventDefault();
          sendMagicLink();
          return;
        case "resend-magic-link":
          event.preventDefault();
          loginState.success = false;
          loginState.error = "";
          renderPortalApp();
          sendMagicLink();
          return;
        default:
          break;
      }
    }
    var logoutBtn = asHTMLElement(event.target)?.closest("#logout-btn");
    if (logoutBtn) {
      event.preventDefault();
      logoutBtn.disabled = true;
      logoutBtn.textContent = "Signing out\u2026";
      (async function() {
        try {
          await fetch(LOGOUT_PATH, { method: "POST" });
        } catch (_) {
        }
        window.location.href = PORTAL_PATH;
      })();
      return;
    }
    var actionEl = asHTMLElement(event.target)?.closest("[data-action]");
    if (!actionEl) return;
    var action = actionEl.getAttribute("data-action") || "";
    var accountID = actionEl.getAttribute("data-account-id") || "";
    switch (action) {
      case "toggle-add-workspace":
        event.preventDefault();
        toggleAddWorkspace(accountID);
        return;
      case "open-billing":
        event.preventDefault();
        openBilling(accountID);
        return;
      case "toggle-team":
        event.preventDefault();
        toggleTeam(accountID);
        return;
      case "invite-member":
        event.preventDefault();
        inviteMember(accountID);
        return;
      case "create-workspace":
        event.preventDefault();
        createWorkspace(accountID);
        return;
      case "workspace-manage":
        event.preventDefault();
        suspendOrDelete(
          event,
          accountID,
          actionEl.getAttribute("data-workspace-id") || "",
          actionEl.getAttribute("data-workspace-state") || "",
          actionEl.getAttribute("data-workspace-name") || ""
        );
        return;
      case "remove-member":
        event.preventDefault();
        removeMember(
          accountID,
          actionEl.getAttribute("data-user-id") || "",
          actionEl.getAttribute("data-member-email") || ""
        );
        return;
      default:
        return;
    }
  });
  document.addEventListener("change", function(event) {
    var target = asHTMLElement(event.target);
    if (!target || target.getAttribute("data-action") !== "change-role") return;
    changeRole(
      target.getAttribute("data-account-id") || "",
      target.getAttribute("data-user-id") || "",
      target.value
    );
  });
  document.addEventListener("input", function(event) {
    var target = asHTMLElement(event.target);
    if (!target) return;
    if (target.getAttribute("data-portal-input") === "login-email") {
      loginState.emailValue = target.value;
    }
  });
  function toggleAddWorkspace(accountID) {
    var form = document.getElementById("add-ws-form-" + accountID);
    if (!form) return;
    var visible = form.classList.contains("visible");
    form.classList.toggle("visible", !visible);
    if (!visible) {
      var input = getElement("ws-name-" + accountID);
      if (input) input.focus();
    }
  }
  async function createWorkspace(accountID) {
    var nameEl = getElement("ws-name-" + accountID);
    if (!nameEl) return;
    var name = nameEl.value.trim();
    if (!name) {
      nameEl.focus();
      return;
    }
    var spinner = getElement("ws-spinner-" + accountID);
    if (spinner) spinner.style.display = "block";
    try {
      var resp = await fetch(ACCOUNT_API_BASE_PATH + "/" + accountID + "/tenants", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ display_name: name })
      });
      if (!resp.ok) {
        var err = await resp.json().catch(function() {
          return {};
        });
        showToast(err && err.error || "Failed to create workspace", true);
        return;
      }
      if (!await refreshBootstrap()) {
        window.location.href = PORTAL_PATH;
        return;
      }
      showToast("Workspace created!");
    } catch (_) {
      showToast("Network error. Please try again.", true);
    } finally {
      if (spinner) spinner.style.display = "none";
    }
  }
  async function suspendOrDelete(evt, accountID, tenantID, state, name) {
    evt.stopPropagation();
    var action = state === "active" ? "Suspend" : "Delete";
    if (!confirm(action + ' workspace "' + name + '"?')) return;
    var method = state === "active" ? "PATCH" : "DELETE";
    var body = state === "active" ? JSON.stringify({ state: "suspended" }) : void 0;
    try {
      var response = await fetch(ACCOUNT_API_BASE_PATH + "/" + accountID + "/tenants/" + tenantID, {
        method,
        headers: body ? { "Content-Type": "application/json" } : {},
        body
      });
      if (!response.ok) {
        showToast("Failed to " + action.toLowerCase() + " workspace.", true);
        return;
      }
      if (!await refreshBootstrap()) {
        window.location.href = PORTAL_PATH;
        return;
      }
      showToast(action + "d workspace.");
    } catch (_) {
      showToast("Network error.", true);
    }
  }
  async function openBilling(accountID) {
    try {
      var r = await fetch(PORTAL_API_BASE_PATH + "/billing?account_id=" + encodeURIComponent(accountID), { method: "POST" });
      if (!r.ok) {
        var err = await r.json().catch(function() {
          return {};
        });
        showToast(err && err.error || "Failed to open billing portal.", true);
        return;
      }
      var data = await r.json();
      if (data && data.url) {
        window.location.href = data.url;
      } else {
        showToast("Failed to open billing portal.", true);
      }
    } catch (_) {
      showToast("Network error.", true);
    }
  }
  function toggleTeam(accountID) {
    var section = document.getElementById("team-section-" + accountID);
    if (!section) return;
    var visible = section.classList.contains("visible");
    section.classList.toggle("visible", !visible);
    if (!visible) loadTeam(accountID);
  }
  function setTbodyMessage(tbody, msg, isError) {
    tbody.textContent = "";
    var tr = document.createElement("tr");
    var td = document.createElement("td");
    td.setAttribute("colspan", "3");
    td.style.cssText = "text-align:center;padding:16px;color:" + (isError ? "#991b1b" : "#94a3b8");
    td.textContent = msg;
    tr.appendChild(td);
    tbody.appendChild(tr);
  }
  async function loadTeam(accountID) {
    var tbody = document.getElementById("team-list-" + accountID);
    var section = document.getElementById("team-section-" + accountID);
    if (!tbody || !section) return;
    var actorRole = section.getAttribute("data-actor-role") || "";
    var isOwner = actorRole === "owner";
    setTbodyMessage(tbody, "Loading\u2026", false);
    try {
      var r = await fetch(ACCOUNT_API_BASE_PATH + "/" + encodeURIComponent(accountID) + "/members");
      if (!r.ok) {
        setTbodyMessage(tbody, "Failed to load team.", true);
        return;
      }
      var members = await r.json();
      if (!members || members.length === 0) {
        setTbodyMessage(tbody, "No team members.", false);
        return;
      }
      var allRoles = ["owner", "admin", "tech", "read_only"];
      var nonOwnerRoles = ["admin", "tech", "read_only"];
      tbody.textContent = "";
      for (var i = 0; i < members.length; i++) {
        (function(m) {
          var tr = document.createElement("tr");
          var tdEmail = document.createElement("td");
          tdEmail.textContent = m.email;
          tr.appendChild(tdEmail);
          var tdRole = document.createElement("td");
          if (m.role === "owner" && !isOwner) {
            tdRole.textContent = "owner";
          } else {
            var sel = document.createElement("select");
            var roles = isOwner ? allRoles : nonOwnerRoles;
            for (var j = 0; j < roles.length; j++) {
              var opt = document.createElement("option");
              opt.value = roles[j];
              opt.textContent = roles[j].replace("_", " ");
              if (m.role === roles[j]) opt.selected = true;
              sel.appendChild(opt);
            }
            sel.setAttribute("data-action", "change-role");
            sel.setAttribute("data-account-id", accountID);
            sel.setAttribute("data-user-id", m.user_id);
            tdRole.appendChild(sel);
          }
          tr.appendChild(tdRole);
          var tdAction = document.createElement("td");
          if (!(m.role === "owner" && !isOwner)) {
            var btn = document.createElement("button");
            btn.type = "button";
            btn.className = "btn-remove";
            btn.textContent = "Remove";
            btn.setAttribute("data-action", "remove-member");
            btn.setAttribute("data-account-id", accountID);
            btn.setAttribute("data-user-id", m.user_id);
            btn.setAttribute("data-member-email", m.email);
            tdAction.appendChild(btn);
          }
          tr.appendChild(tdAction);
          tbody.appendChild(tr);
        })(members[i]);
      }
    } catch (_) {
      setTbodyMessage(tbody, "Network error.", true);
    }
  }
  async function refreshAccountTeamSection(accountID) {
    if (!await refreshBootstrap()) {
      window.location.href = PORTAL_PATH;
      return false;
    }
    var section = document.getElementById("team-section-" + accountID);
    if (!section) {
      return true;
    }
    section.classList.add("visible");
    await loadTeam(accountID);
    return true;
  }
  async function inviteMember(accountID) {
    var emailEl = getElement("invite-email-" + accountID);
    var roleEl = getElement("invite-role-" + accountID);
    if (!emailEl || !roleEl) return;
    var email = emailEl.value.trim();
    if (!email) {
      emailEl.focus();
      return;
    }
    try {
      var r = await fetch(ACCOUNT_API_BASE_PATH + "/" + encodeURIComponent(accountID) + "/members", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, role: roleEl.value })
      });
      if (r.status === 409) {
        showToast("Member already exists.", true);
        return;
      }
      if (!r.ok) {
        var err = await r.text();
        showToast(err || "Failed to invite member.", true);
        return;
      }
      emailEl.value = "";
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      showToast("Member invited!");
    } catch (_) {
      showToast("Network error.", true);
    }
  }
  async function changeRole(accountID, userID, newRole) {
    try {
      var r = await fetch(ACCOUNT_API_BASE_PATH + "/" + encodeURIComponent(accountID) + "/members/" + encodeURIComponent(userID), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ role: newRole })
      });
      if (r.status === 409) {
        showToast("Cannot demote last owner.", true);
        loadTeam(accountID);
        return;
      }
      if (!r.ok) {
        showToast("Failed to update role.", true);
        loadTeam(accountID);
        return;
      }
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      showToast("Role updated.");
    } catch (_) {
      showToast("Network error.", true);
      loadTeam(accountID);
    }
  }
  async function removeMember(accountID, userID, email) {
    if (!confirm("Remove " + email + " from this account?")) return;
    try {
      var r = await fetch(ACCOUNT_API_BASE_PATH + "/" + encodeURIComponent(accountID) + "/members/" + encodeURIComponent(userID), {
        method: "DELETE"
      });
      if (r.status === 409) {
        showToast("Cannot remove last owner.", true);
        return;
      }
      if (!r.ok) {
        showToast("Failed to remove member.", true);
        return;
      }
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      showToast("Member removed.");
    } catch (_) {
      showToast("Network error.", true);
    }
  }
  loginState.emailValue = portalBootstrap.email || "";
  applyBootstrap(portalBootstrap);
  if (portalBootstrap.authenticated) {
    refreshBootstrap();
  }

  // src/services.ts
  var serviceState = {
    openPanelID: "",
    flows: {
      manage: newVerificationFlowState(),
      retrieve: newVerificationFlowState(),
      export: newVerificationFlowState(),
      delete: newVerificationFlowState()
    },
    refund: {
      emailValue: "",
      tokenValue: "",
      submitting: false,
      status: emptyStatus()
    }
  };
  function newVerificationFlowState() {
    return {
      pendingEmail: "",
      requesting: false,
      confirming: false,
      step2Visible: false,
      status: emptyStatus(),
      result: null,
      emailValue: "",
      codeValue: "",
      checkboxChecked: false
    };
  }
  function emptyStatus() {
    return {
      visible: false,
      message: "",
      error: false
    };
  }
  function getCommercialAPIBaseURL2() {
    return getCommercialAPIBaseURL();
  }
  function getElement2(id) {
    return document.getElementById(id);
  }
  function asHTMLElement2(target) {
    return target instanceof HTMLElement ? target : null;
  }
  function escapeText(value) {
    return String(value || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  function escapeAttribute(value) {
    return escapeText(value).replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
  function readValue(id) {
    var el = getElement2(id);
    return el ? el.value.trim() : "";
  }
  function focusElement(id) {
    var el = getElement2(id);
    if (el) el.focus();
  }
  function setVisible(id, visible) {
    var el = getElement2(id);
    if (el) {
      el.style.display = visible ? "block" : "none";
    }
  }
  function setValue(id, value) {
    var el = getElement2(id);
    if (el) {
      el.value = value;
    }
  }
  function serviceFetch(path, body) {
    return fetch(getCommercialAPIBaseURL2() + path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  }
  function setFlowStatus(flowID, message, isError) {
    serviceState.flows[flowID].status = {
      visible: true,
      message,
      error: !!isError
    };
  }
  function clearFlowStatus(flowID) {
    serviceState.flows[flowID].status = emptyStatus();
  }
  function setRefundStatus(message, isError) {
    serviceState.refund.status = {
      visible: true,
      message,
      error: !!isError
    };
  }
  function renderStatus(id, status) {
    var el = getElement2(id);
    if (!el) return;
    if (!status.visible) {
      el.textContent = "";
      el.className = "service-status";
      return;
    }
    el.textContent = status.message;
    el.className = "service-status visible" + (status.error ? " error" : " success");
  }
  function renderButton(id, disabled, label) {
    var button = getElement2(id);
    if (!button) return;
    button.disabled = disabled;
    button.textContent = label;
  }
  function toggleServicePanel(panelID) {
    serviceState.openPanelID = serviceState.openPanelID === panelID ? "" : panelID;
    renderOpenPanels();
  }
  function renderOpenPanels() {
    var panels = ["manage-service-panel", "retrieve-service-panel", "refund-service-panel", "data-service-panel"];
    for (var i = 0; i < panels.length; i++) {
      var panel = getElement2(panels[i]);
      if (!panel) continue;
      panel.classList.toggle("visible", panels[i] === serviceState.openPanelID);
    }
  }
  function renderFlow(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var flowState = serviceState.flows[flowID];
    if (flow.renderPanel) {
      flow.renderPanel(flowState);
    }
    renderButton(flow.requestButtonID, flowState.requesting, flowState.requesting ? flow.requestPendingLabel : flow.requestLabel);
    renderButton(flow.confirmButtonID, flowState.confirming, flowState.confirming ? flow.confirmPendingLabel : flow.confirmLabel);
    renderStatus(flow.statusID, flowState.status);
    if (flow.step2ID) {
      setVisible(flow.step2ID, flowState.step2Visible);
    }
    if (flow.renderResult) {
      flow.renderResult(flowState.result);
    }
  }
  function renderAllFlows() {
    renderFlow("manage");
    renderFlow("retrieve");
    renderFlow("export");
    renderFlow("delete");
    renderRefund();
  }
  function renderRefund() {
    renderRefundPanel();
    renderButton("refund-inline-submit", serviceState.refund.submitting, serviceState.refund.submitting ? "Processing..." : "Process Refund");
    renderStatus("refund-inline-status", serviceState.refund.status);
  }
  function renderRefundPanel() {
    var root = getElement2("refund-service-root");
    if (!root) return;
    var bootstrap = getBootstrap();
    var refundSupportURL = (bootstrap.public_site_url || "") + "/refund.html?email=" + encodeURIComponent(serviceState.refund.emailValue || "");
    root.innerHTML = '<h3>Refund requests</h3><p>Process an eligible self-serve refund for a self-hosted purchase. This revokes the associated license immediately.</p><div class="warning"><strong>Warning:</strong> completing a refund immediately revokes the affected license. This should only be used when the refund window and commercial contract allow it.</div><div class="form-group"><label for="refund-inline-email">Email address</label><input type="email" id="refund-inline-email" value="' + escapeAttribute(serviceState.refund.emailValue || "") + '" autocomplete="email" data-account-service-input="refund-email"></div><div class="form-group"><label for="refund-inline-token">License key</label><input type="text" id="refund-inline-token" value="' + escapeAttribute(serviceState.refund.tokenValue || "") + '" placeholder="pulse_xxxxx" data-account-service-input="refund-token"></div><div class="form-actions"><button class="btn-danger" type="button" id="refund-inline-submit" data-account-service-action="refund-inline-submit">Process Refund</button></div><div class="helper-text">If this purchase is not eligible for self-serve refund, use the public support path instead: <a href="' + escapeAttribute(refundSupportURL) + '">open refund support page</a>.</div><div class="service-status" id="refund-inline-status"></div>';
  }
  function resetVerificationFlow(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var previous = serviceState.flows[flowID];
    serviceState.flows[flowID] = newVerificationFlowState();
    serviceState.flows[flowID].emailValue = previous.emailValue;
    if (flow.codeInputID) {
      setValue(flow.codeInputID, "");
    }
  }
  var verificationFlows = {
    manage: {
      requestPath: "/v1/manage/request",
      confirmPath: "/v1/manage",
      panelID: "manage-service-panel",
      emailInputID: "manage-inline-email",
      codeInputID: "manage-inline-code",
      requestButtonID: "manage-inline-request",
      confirmButtonID: "manage-inline-confirm",
      step2ID: "manage-inline-step2",
      statusID: "manage-inline-status",
      requestLabel: "Send Verification Code",
      requestPendingLabel: "Sending...",
      confirmLabel: "Open Customer Portal",
      confirmPendingLabel: "Redirecting...",
      requestSuccessMessage: "Verification code sent. Check your email.",
      resendSuccessMessage: "New verification code sent.",
      requestErrorMessage: "Failed to send verification code",
      confirmErrorMessage: "Failed to open customer portal",
      readEmailValue: function() {
        return serviceState.flows.manage.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.manage.codeValue;
      },
      onRequestStart: function() {
      },
      onConfirmSuccess: function(data) {
        window.location.href = data.url;
      },
      renderPanel: function(flowState) {
        var root = getElement2("manage-service-root");
        if (!root) return;
        root.innerHTML = '<h3>Manage subscriptions</h3><p>Request a verification code for the commercial email, then open the Stripe customer portal for billing changes, invoices, and subscription actions.</p><div id="manage-inline-step1"><div class="form-group"><label for="manage-inline-email">Email address</label><input type="email" id="manage-inline-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="manage-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="manage-inline-request" data-account-service-action="manage-inline-request">Send Verification Code</button></div></div><div id="manage-inline-step2" style="display:' + (flowState.step2Visible ? "block" : "none") + '"><div class="form-group"><label for="manage-inline-code">Verification code</label><input type="text" id="manage-inline-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="manage-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="manage-inline-confirm" data-account-service-action="manage-inline-confirm">Open Customer Portal</button></div><div class="helper-text">Need a new code? <a href="#" id="manage-inline-resend" data-account-service-action="manage-inline-resend">Send again</a></div></div><div class="service-status" id="manage-inline-status"></div>';
      }
    },
    retrieve: {
      requestPath: "/v1/retrieve-license/request",
      confirmPath: "/v1/retrieve-license",
      panelID: "retrieve-service-panel",
      emailInputID: "retrieve-inline-email",
      codeInputID: "retrieve-inline-code",
      requestButtonID: "retrieve-inline-request",
      confirmButtonID: "retrieve-inline-confirm",
      step2ID: "retrieve-inline-step2",
      statusID: "retrieve-inline-status",
      requestLabel: "Send Verification Code",
      requestPendingLabel: "Sending...",
      confirmLabel: "Show License",
      confirmPendingLabel: "Loading...",
      requestSuccessMessage: "Verification code sent. Check your email.",
      resendSuccessMessage: "New verification code sent.",
      requestErrorMessage: "Failed to send verification code",
      confirmErrorMessage: "Failed to retrieve license",
      readEmailValue: function() {
        return serviceState.flows.retrieve.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.retrieve.codeValue;
      },
      onRequestStart: function() {
        serviceState.flows.retrieve.result = null;
      },
      onConfirmSuccess: function(data) {
        serviceState.flows.retrieve.result = data.license;
        serviceState.flows.retrieve.codeValue = "";
        setFlowStatus("retrieve", "License retrieved successfully.", false);
      },
      renderPanel: function(flowState) {
        var root = getElement2("retrieve-service-root");
        if (!root) return;
        var result = flowState.result;
        var invoiceURL = result && result.invoice_url ? result.invoice_url : "#";
        var invoiceDisplay = result && result.invoice_url ? "inline-block" : "none";
        var copyDisplay = result ? "inline-block" : "none";
        var resultDisplay = result ? "block" : "none";
        root.innerHTML = '<h3>Retrieve licenses</h3><p>Request a verification code for the commercial email, then reveal the current active self-hosted license without leaving Pulse Account.</p><div id="retrieve-inline-step1"><div class="form-group"><label for="retrieve-inline-email">Email address</label><input type="email" id="retrieve-inline-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="retrieve-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="retrieve-inline-request" data-account-service-action="retrieve-inline-request">Send Verification Code</button></div></div><div id="retrieve-inline-step2" style="display:' + (flowState.step2Visible ? "block" : "none") + '"><div class="form-group"><label for="retrieve-inline-code">Verification code</label><input type="text" id="retrieve-inline-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="retrieve-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="retrieve-inline-confirm" data-account-service-action="retrieve-inline-confirm">Show License</button><button class="btn-secondary" type="button" id="retrieve-inline-copy" data-account-service-action="retrieve-inline-copy" style="display:' + copyDisplay + '">Copy License Key</button><a class="btn-secondary" id="retrieve-inline-invoice" href="' + escapeAttribute(invoiceURL) + '" target="_blank" rel="noopener" style="display:' + invoiceDisplay + '">View Invoice</a></div><div class="helper-text">Use the latest active self-hosted license for this commercial email.</div></div><div class="service-status" id="retrieve-inline-status"></div><div id="retrieve-inline-result" style="display:' + resultDisplay + '; margin-top:14px"><label for="retrieve-inline-token">License key</label><textarea id="retrieve-inline-token" readonly>' + escapeText(result ? result.token : "") + '</textarea><div class="result-grid"><div><div class="result-meta-label">Plan</div><div class="result-meta-value" id="retrieve-inline-tier">' + escapeText(result ? result.tier : "") + '</div></div><div><div class="result-meta-label">Issued</div><div class="result-meta-value" id="retrieve-inline-issued">' + escapeText(result ? new Date(result.issued_at).toLocaleString() : "") + '</div></div><div><div class="result-meta-label">Expires</div><div class="result-meta-value" id="retrieve-inline-expires">' + escapeText(result ? result.expires_at ? new Date(result.expires_at).toLocaleString() : "Does not expire" : "") + '</div></div><div><div class="result-meta-label">Purchase Email</div><div class="result-meta-value" id="retrieve-inline-email-value">' + escapeText(result ? result.email : "") + "</div></div></div></div>";
      },
      renderResult: function(result) {
        void result;
      }
    },
    export: {
      requestPath: "/v1/gdpr/request-export",
      confirmPath: "/v1/gdpr/export",
      panelID: "data-service-panel",
      emailInputID: "data-export-email",
      codeInputID: "data-export-code",
      requestButtonID: "data-export-request",
      confirmButtonID: "data-export-confirm",
      step2ID: "data-export-step2",
      statusID: "data-export-status",
      requestLabel: "Send Verification Code",
      requestPendingLabel: "Sending...",
      confirmLabel: "Export My Data",
      confirmPendingLabel: "Exporting...",
      requestSuccessMessage: "Verification code sent. Check your email.",
      resendSuccessMessage: "New verification code sent.",
      requestErrorMessage: "Request failed",
      confirmErrorMessage: "Export failed",
      readEmailValue: function() {
        return serviceState.flows.export.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.export.codeValue;
      },
      onRequestStart: function() {
        serviceState.flows.export.result = null;
      },
      onConfirmSuccess: function(data) {
        serviceState.flows.export.result = data;
        serviceState.flows.export.codeValue = "";
        setFlowStatus("export", "Data export retrieved successfully.", false);
        resetVerificationFlow("export");
        serviceState.flows.export.result = data;
      },
      renderPanel: function(flowState) {
        var root = getElement2("data-export-root");
        if (!root) return;
        var resultDisplay = flowState.result ? "block" : "none";
        root.innerHTML = '<h4>Export My Data</h4><div id="data-export-step1"><div class="form-group"><label for="data-export-email">Email address</label><input type="email" id="data-export-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="data-export-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="data-export-request" data-account-service-action="data-export-request">Send Verification Code</button></div></div><div id="data-export-step2" style="display:' + (flowState.step2Visible ? "block" : "none") + '"><div class="form-group"><label for="data-export-code">Verification code</label><input type="text" id="data-export-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="data-export-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="data-export-confirm" data-account-service-action="data-export-confirm">Export My Data</button></div><div class="helper-text">Need a new code? <a href="#" id="data-export-resend" data-account-service-action="data-export-resend">Send again</a></div></div><div class="service-status" id="data-export-status"></div><div id="data-export-result" style="display:' + resultDisplay + '; margin-top:14px"><label for="data-export-payload">Export payload</label><textarea id="data-export-payload" readonly>' + escapeText(flowState.result ? JSON.stringify(flowState.result, null, 2) : "") + "</textarea></div>";
      },
      renderResult: function(result) {
        setVisible("data-export-result", !!result);
        setValue("data-export-payload", result ? JSON.stringify(result, null, 2) : "");
      }
    },
    delete: {
      requestPath: "/v1/gdpr/request-delete",
      confirmPath: "/v1/gdpr/confirm-delete",
      panelID: "data-service-panel",
      emailInputID: "data-delete-email",
      codeInputID: "data-delete-code",
      requestButtonID: "data-delete-request",
      confirmButtonID: "data-delete-confirm",
      step2ID: "data-delete-step2",
      statusID: "data-delete-status",
      requestLabel: "Send Verification Code",
      requestPendingLabel: "Sending...",
      confirmLabel: "Delete My Data",
      confirmPendingLabel: "Deleting...",
      requestSuccessMessage: "Verification code sent. Check your email.",
      resendSuccessMessage: "New verification code sent.",
      requestErrorMessage: "Request failed",
      confirmErrorMessage: "Deletion failed",
      readEmailValue: function() {
        return serviceState.flows.delete.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.delete.codeValue;
      },
      beforeConfirm: function() {
        if (!getElement2("data-delete-confirm-check")?.checked) {
          setFlowStatus("delete", "You must confirm that you understand this action is permanent.", true);
          renderFlow("delete");
          return false;
        }
        return true;
      },
      onConfirmSuccess: function(data) {
        var checkbox = getElement2("data-delete-confirm-check");
        if (checkbox) {
          checkbox.checked = false;
        }
        resetVerificationFlow("delete");
        setFlowStatus("delete", data.deleted_count > 0 && data.stripe_reminder ? data.message + " " + data.stripe_reminder : data.message, false);
      },
      renderPanel: function(flowState) {
        var root = getElement2("data-delete-root");
        if (!root) return;
        root.innerHTML = '<h4>Delete My Data</h4><div class="warning"><strong>Warning:</strong> deleting commercial data also revokes license records and cannot be undone.</div><div id="data-delete-step1"><div class="form-group"><label for="data-delete-email">Email address</label><input type="email" id="data-delete-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="data-delete-email"></div><div class="form-actions"><button class="btn-danger" type="button" id="data-delete-request" data-account-service-action="data-delete-request">Send Verification Code</button></div></div><div id="data-delete-step2" style="display:' + (flowState.step2Visible ? "block" : "none") + '"><div class="form-group"><label for="data-delete-code">Verification code</label><input type="text" id="data-delete-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="data-delete-code"></div><div class="checkbox-row"><input type="checkbox" id="data-delete-confirm-check"' + (flowState.checkboxChecked ? " checked" : "") + '><span>I understand this permanently deletes my commercial data and revokes associated licenses.</span></div><div class="form-actions"><button class="btn-danger" type="button" id="data-delete-confirm" data-account-service-action="data-delete-confirm">Delete My Data</button></div><div class="helper-text">Need a new code? <a href="#" id="data-delete-resend" data-account-service-action="data-delete-resend">Send again</a></div></div><div class="service-status" id="data-delete-status"></div>';
      }
    }
  };
  async function requestVerificationCode(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = flow.readEmailValue ? flow.readEmailValue() : readValue(flow.emailInputID);
    if (!email) {
      focusElement(flow.emailInputID);
      return;
    }
    if (flow.onRequestStart) {
      flow.onRequestStart();
    }
    serviceState.flows[flowID].requesting = true;
    clearFlowStatus(flowID);
    renderFlow(flowID);
    try {
      var res = await serviceFetch(flow.requestPath, { email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.requestErrorMessage);
      serviceState.flows[flowID].pendingEmail = email;
      serviceState.flows[flowID].step2Visible = !!flow.step2ID;
      setFlowStatus(flowID, flow.requestSuccessMessage, false);
    } catch (err) {
      setFlowStatus(flowID, err.message, true);
    } finally {
      serviceState.flows[flowID].requesting = false;
      renderFlow(flowID);
    }
  }
  async function resendVerificationCode(flowID, event) {
    if (event) event.preventDefault();
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = serviceState.flows[flowID].pendingEmail;
    if (!email) return;
    try {
      var res = await serviceFetch(flow.requestPath, { email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.requestErrorMessage);
      setFlowStatus(flowID, flow.resendSuccessMessage, false);
    } catch (err) {
      setFlowStatus(flowID, err.message, true);
    }
    renderFlow(flowID);
  }
  async function confirmVerificationCode(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = serviceState.flows[flowID].pendingEmail;
    var code = flow.readCodeValue ? flow.readCodeValue() : readValue(flow.codeInputID);
    if (!email || !code) return;
    if (flow.beforeConfirm && flow.beforeConfirm() === false) {
      return;
    }
    serviceState.flows[flowID].confirming = true;
    renderFlow(flowID);
    try {
      var res = await serviceFetch(flow.confirmPath, { email, code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.confirmErrorMessage);
      flow.onConfirmSuccess(data, email);
    } catch (err) {
      setFlowStatus(flowID, err.message, true);
    } finally {
      serviceState.flows[flowID].confirming = false;
      renderFlow(flowID);
    }
  }
  async function copyRetrievedLicense() {
    var result = serviceState.flows.retrieve.result;
    var token = result && result.token ? result.token : "";
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token);
      setFlowStatus("retrieve", "License key copied to clipboard.", false);
    } catch (_) {
      setFlowStatus("retrieve", "Failed to copy automatically. Please copy the key manually.", true);
    }
    renderFlow("retrieve");
  }
  async function submitRefund() {
    var email = serviceState.refund.emailValue;
    var token = serviceState.refund.tokenValue;
    if (!email || !token) return;
    if (!confirm("Are you sure? This will immediately revoke the license and request the refund.")) return;
    serviceState.refund.submitting = true;
    serviceState.refund.status = emptyStatus();
    renderRefund();
    try {
      var res = await serviceFetch("/v1/self-refund", { email, token });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || "Refund failed");
      serviceState.refund.tokenValue = "";
      setRefundStatus("Success! Your refund has been processed. Stripe will follow up by email.", false);
    } catch (err) {
      setRefundStatus(err.message, true);
    } finally {
      serviceState.refund.submitting = false;
      renderRefund();
    }
  }
  function syncServiceStateFromBootstrap() {
    var bootstrap = getBootstrap();
    if (!bootstrap.authenticated) {
      return;
    }
    if (!serviceState.flows.manage.emailValue) serviceState.flows.manage.emailValue = bootstrap.email || "";
    if (!serviceState.flows.retrieve.emailValue) serviceState.flows.retrieve.emailValue = bootstrap.email || "";
    if (!serviceState.flows.export.emailValue) serviceState.flows.export.emailValue = bootstrap.email || "";
    if (!serviceState.flows.delete.emailValue) serviceState.flows.delete.emailValue = bootstrap.email || "";
    if (!serviceState.refund.emailValue) serviceState.refund.emailValue = bootstrap.email || "";
  }
  function renderServiceRuntime() {
    syncServiceStateFromBootstrap();
    renderOpenPanels();
    renderAllFlows();
  }
  renderServiceRuntime();
  subscribePortalRender(renderServiceRuntime);
  document.addEventListener("click", function(event) {
    var target = asHTMLElement2(event.target)?.closest("[data-account-service-action]");
    if (!target) return;
    var action = target.getAttribute("data-account-service-action") || "";
    var panelID = target.getAttribute("data-account-service-panel") || "";
    var focusID = target.getAttribute("data-account-service-focus") || "";
    switch (action) {
      case "open-service-panel":
        event.preventDefault();
        toggleServicePanel(panelID);
        focusElement(focusID);
        return;
      case "manage-inline-request":
        event.preventDefault();
        requestVerificationCode("manage");
        return;
      case "manage-inline-resend":
        resendVerificationCode("manage", event);
        return;
      case "manage-inline-confirm":
        event.preventDefault();
        confirmVerificationCode("manage");
        return;
      case "retrieve-inline-request":
        event.preventDefault();
        requestVerificationCode("retrieve");
        return;
      case "retrieve-inline-confirm":
        event.preventDefault();
        confirmVerificationCode("retrieve");
        return;
      case "retrieve-inline-copy":
        event.preventDefault();
        copyRetrievedLicense();
        return;
      case "refund-inline-submit":
        event.preventDefault();
        submitRefund();
        return;
      case "data-export-request":
        event.preventDefault();
        requestVerificationCode("export");
        return;
      case "data-export-resend":
        resendVerificationCode("export", event);
        return;
      case "data-export-confirm":
        event.preventDefault();
        confirmVerificationCode("export");
        return;
      case "data-delete-request":
        event.preventDefault();
        requestVerificationCode("delete");
        return;
      case "data-delete-resend":
        resendVerificationCode("delete", event);
        return;
      case "data-delete-confirm":
        event.preventDefault();
        confirmVerificationCode("delete");
        return;
      default:
        return;
    }
  });
  document.addEventListener("input", function(event) {
    var target = asHTMLElement2(event.target);
    if (!target) return;
    var inputKind = target.getAttribute("data-account-service-input") || "";
    switch (inputKind) {
      case "manage-email":
        serviceState.flows.manage.emailValue = target.value;
        return;
      case "manage-code":
        serviceState.flows.manage.codeValue = target.value;
        return;
      case "retrieve-email":
        serviceState.flows.retrieve.emailValue = target.value;
        return;
      case "retrieve-code":
        serviceState.flows.retrieve.codeValue = target.value;
        return;
      case "refund-email":
        serviceState.refund.emailValue = target.value;
        return;
      case "refund-token":
        serviceState.refund.tokenValue = target.value;
        return;
      case "data-export-email":
        serviceState.flows.export.emailValue = target.value;
        return;
      case "data-export-code":
        serviceState.flows.export.codeValue = target.value;
        return;
      case "data-delete-email":
        serviceState.flows.delete.emailValue = target.value;
        return;
      case "data-delete-code":
        serviceState.flows.delete.codeValue = target.value;
        return;
      default:
        return;
    }
  });
  document.addEventListener("change", function(event) {
    var target = asHTMLElement2(event.target);
    if (!target || target.id !== "data-delete-confirm-check") return;
    serviceState.flows.delete.checkboxChecked = !!target.checked;
  });
})();
