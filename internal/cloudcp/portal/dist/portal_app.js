(() => {
  // src/account_view.ts
  function normalizedAccessRole(role) {
    if (role === "member") return "read_only";
    return role || "read_only";
  }
  function roleLabel(role) {
    switch (normalizedAccessRole(role)) {
      case "owner":
        return "Owner";
      case "admin":
        return "Admin";
      case "tech":
        return "Tech";
      case "read_only":
        return "Read-only";
      case "member":
        return "Member";
      default:
        return role || "Member";
    }
  }
  function roleCapabilityCopy(role) {
    switch (normalizedAccessRole(role)) {
      case "owner":
        return "Full account control, including billing, access control, and workspace control.";
      case "admin":
        return "Can manage workspaces and billing for this account.";
      case "tech":
        return "Can manage workspaces without billing ownership.";
      case "read_only":
        return "Can review workspace status without making control-plane changes.";
      case "member":
        return "Has access through the account roster.";
      default:
        return "Has access through the account roster.";
    }
  }
  function getElement(id) {
    return document.getElementById(id);
  }
  function asHTMLElement(target) {
    return target instanceof HTMLElement ? target : null;
  }
  function focusElement(id) {
    var input = getElement(id);
    if (input) input.focus();
  }
  function workspaceActionLabel(workspace) {
    return workspace.state === "active" ? "Suspend workspace" : "Delete workspace";
  }
  function workspaceSummary(workspace) {
    if (workspace.health_status === "healthy") return "Live updates and health checks are currently good.";
    if (workspace.health_status === "unhealthy") return "This workspace needs attention before it is trustworthy.";
    return "This workspace is still waiting on a completed health check.";
  }
  function workspaceHealthLabel(workspace) {
    if (workspace.health_status === "healthy") return "Healthy";
    if (workspace.health_status === "unhealthy") return "Needs attention";
    return "Checking";
  }
  function workspaceCreatedLabel(workspace) {
    if (!workspace.created_at) return "Unknown";
    var date = new Date(workspace.created_at);
    if (Number.isNaN(date.getTime())) return "Unknown";
    return date.toLocaleDateString(void 0, { month: "short", day: "numeric", year: "numeric" });
  }
  function workspaceGuidance(workspace) {
    if (workspace.state === "active" && workspace.health_status === "healthy") {
      return "This workspace looks ready for normal use. Use the fleet table to open it, or suspend it here if you are intentionally taking it out of service.";
    }
    if (workspace.state === "active" && workspace.health_status === "checking") {
      return "This workspace is active but still waiting on a completed health check. Review it before you treat the account status as settled.";
    }
    if (workspace.health_status === "unhealthy") {
      return "This workspace needs review before it is treated as trustworthy. Use the management action only when you intend to suspend or remove it from the workspace list.";
    }
    if (workspace.state === "suspended") {
      return "This workspace is already suspended. The remaining lifecycle action here is deletion, so treat it as a deliberate irreversible step.";
    }
    return "Review the lifecycle state before taking the next explicit action for this workspace.";
  }
  function workspaceMeta(workspace) {
    var parts = [workspace.state];
    if (workspace.health_status) parts.push(workspace.health_status);
    if (workspace.created_at) {
      var date = new Date(workspace.created_at);
      if (!Number.isNaN(date.getTime())) {
        parts.push("Created " + date.toLocaleDateString(void 0, { month: "short", day: "numeric", year: "numeric" }));
      }
    }
    return parts.join(" \xB7 ");
  }
  function findWorkspace(account, workspaceID) {
    for (var i = 0; i < account.workspaces.length; i += 1) {
      if (account.workspaces[i].id === workspaceID) return account.workspaces[i];
    }
    return null;
  }
  function renderWorkspaceManagement(account, entry) {
    var panel = getElement("workspace-management-" + account.id);
    var shell = getElement("workspace-operations-shell-" + account.id);
    var detail = getElement("workspace-operations-detail-" + account.id);
    if (!panel) return;
    var empty = getElement("workspace-management-empty-" + account.id);
    var content = getElement("workspace-management-content-" + account.id);
    var title = getElement("workspace-management-title-" + account.id);
    var meta = getElement("workspace-management-meta-" + account.id);
    var summary = getElement("workspace-management-summary-" + account.id);
    var health = getElement("workspace-management-health-" + account.id);
    var lifecycle = getElement("workspace-management-lifecycle-" + account.id);
    var created = getElement("workspace-management-created-" + account.id);
    var guidance = getElement("workspace-management-guidance-" + account.id);
    var actionButton = getElement("workspace-management-action-" + account.id);
    var closeButton = getElement("workspace-management-close-" + account.id);
    if (!empty || !content || !title || !meta || !summary || !health || !lifecycle || !created || !guidance || !actionButton || !closeButton) return;
    var workspace = entry.selectedWorkspaceID ? findWorkspace(account, entry.selectedWorkspaceID) : null;
    var hasSelection = !!workspace;
    var showDetail = hasSelection || entry.addWorkspaceOpen;
    var rows = document.querySelectorAll("[data-workspace-row]");
    for (var i = 0; i < rows.length; i += 1) {
      rows[i].classList.toggle("selected", !!workspace && rows[i].getAttribute("data-workspace-row") === workspace.id);
    }
    if (shell) {
      shell.classList.toggle("workspace-operations-shell-selected", hasSelection);
      shell.classList.toggle("workspace-operations-shell-idle", !showDetail);
      shell.classList.toggle("workspace-operations-shell-form-open", entry.addWorkspaceOpen);
    }
    if (detail) {
      detail.classList.toggle("workspace-operations-detail-selected", hasSelection);
      detail.classList.toggle("workspace-operations-detail-idle", !showDetail);
      detail.hidden = !showDetail;
    }
    panel.classList.toggle("workspace-management-panel-selected", hasSelection);
    panel.classList.toggle("workspace-management-panel-idle", !hasSelection);
    panel.classList.toggle("visible", showDetail);
    panel.hidden = !showDetail;
    empty.hidden = hasSelection || !showDetail;
    content.hidden = !hasSelection;
    if (!workspace) {
      actionButton.disabled = false;
      actionButton.removeAttribute("data-workspace-id");
      actionButton.removeAttribute("data-workspace-name");
      actionButton.removeAttribute("data-workspace-action");
      return;
    }
    title.textContent = workspace.display_name;
    meta.textContent = workspaceMeta(workspace);
    summary.textContent = workspaceSummary(workspace);
    health.textContent = workspaceHealthLabel(workspace);
    lifecycle.textContent = workspace.state ? workspace.state.charAt(0).toUpperCase() + workspace.state.slice(1) : "Unknown";
    created.textContent = workspaceCreatedLabel(workspace);
    guidance.textContent = workspaceGuidance(workspace);
    actionButton.textContent = workspaceActionLabel(workspace);
    actionButton.disabled = entry.manageWorkspace.pending;
    actionButton.setAttribute("data-workspace-id", workspace.id);
    actionButton.setAttribute("data-workspace-name", workspace.display_name);
    actionButton.setAttribute("data-workspace-action", workspace.state === "active" ? "suspend" : "delete");
    closeButton.disabled = entry.manageWorkspace.pending;
  }
  function setContainerMessage(container, title, msg, isError) {
    container.textContent = "";
    container.classList.add("state-only");
    var message = document.createElement("div");
    message.className = "access-list-message" + (isError ? " error" : "");
    var heading = document.createElement("strong");
    heading.className = "access-list-message-title";
    heading.textContent = title;
    var copy = document.createElement("span");
    copy.className = "access-list-message-copy";
    copy.textContent = msg;
    message.appendChild(heading);
    message.appendChild(copy);
    container.appendChild(message);
  }
  function countMembersByRole(members, role) {
    var count = 0;
    for (var i = 0; i < members.length; i += 1) {
      if (normalizedAccessRole(members[i].role) === role) count += 1;
    }
    return count;
  }
  function accessJobTitle(job) {
    switch (job) {
      case "invite":
        return "Invite people";
      case "change_role":
        return "Change roles";
      case "remove":
        return "Remove access";
      default:
        return "";
    }
  }
  function accessJobCopy(job) {
    switch (job) {
      case "invite":
        return "Add one person with the minimum role they need on this account.";
      case "change_role":
        return "Use the roster to change one person at a time and keep each person on the smallest role they need.";
      case "remove":
        return "Use removal only when this person should no longer be on this hosted account.";
      default:
        return "";
    }
  }
  function renderAccessStats(accountID, entry, canManage) {
    var stats = getElement("access-stats-" + accountID);
    if (!stats) return;
    if (!entry.accessVisible) {
      stats.innerHTML = "";
      return;
    }
    if (entry.accessQuery.status === "loading") {
      stats.innerHTML = '<div class="access-stat-card"><span class="access-stat-label">Roster</span><span class="access-stat-value">Loading\u2026</span></div><div class="access-stat-card"><span class="access-stat-label">Mode</span><span class="access-stat-value">' + (canManage ? "Manage" : "View") + "</span></div>";
      return;
    }
    if (entry.accessQuery.status === "error") {
      stats.innerHTML = '<div class="access-stat-card"><span class="access-stat-label">Roster</span><span class="access-stat-value access-stat-error">Needs attention</span></div><div class="access-stat-card"><span class="access-stat-label">Mode</span><span class="access-stat-value">' + (canManage ? "Manage" : "View") + "</span></div>";
      return;
    }
    var members = entry.accessQuery.data;
    stats.innerHTML = '<div class="access-stat-card"><span class="access-stat-label">Members</span><span class="access-stat-value">' + String(members.length) + '</span></div><div class="access-stat-card"><span class="access-stat-label">Owners</span><span class="access-stat-value">' + String(countMembersByRole(members, "owner")) + '</span></div><div class="access-stat-card"><span class="access-stat-label">Admins</span><span class="access-stat-value">' + String(countMembersByRole(members, "admin")) + '</span></div><div class="access-stat-card"><span class="access-stat-label">Operators</span><span class="access-stat-value">' + String(countMembersByRole(members, "tech") + countMembersByRole(members, "read_only")) + "</span></div>";
  }
  function createAccessControlCell(className) {
    var cell = document.createElement("div");
    cell.className = "access-control-cell " + className;
    return cell;
  }
  function renderAccessRoleControl(accountID, member, isOwner, canManage, activeJob) {
    var currentRole = normalizedAccessRole(member.role);
    var group = createAccessControlCell("access-control-cell-role");
    if (!canManage || activeJob !== "change_role") {
      var badge = document.createElement("span");
      badge.className = "access-role-badge";
      badge.textContent = roleLabel(currentRole);
      group.appendChild(badge);
      return group;
    }
    if (currentRole === "owner" && !isOwner) {
      var locked = document.createElement("span");
      locked.className = "access-role-badge";
      locked.textContent = roleLabel(currentRole);
      group.appendChild(locked);
      return group;
    }
    var sel = document.createElement("select");
    sel.className = "access-role-select";
    var roles = isOwner ? ["owner", "admin", "tech", "read_only"] : ["admin", "tech", "read_only"];
    for (var j = 0; j < roles.length; j += 1) {
      var opt = document.createElement("option");
      opt.value = roles[j];
      opt.textContent = roleLabel(roles[j]);
      if (currentRole === roles[j]) opt.selected = true;
      sel.appendChild(opt);
    }
    sel.setAttribute("data-action", "change-role");
    sel.setAttribute("data-account-id", accountID);
    sel.setAttribute("data-user-id", member.user_id);
    group.appendChild(sel);
    return group;
  }
  function renderAccessMemberAction(accountID, member, isOwner, canManage, activeJob) {
    var group = createAccessControlCell("access-control-cell-access");
    if (!canManage) {
      var readonlyText = document.createElement("span");
      readonlyText.className = "access-control-locked";
      readonlyText.textContent = "View only";
      group.appendChild(readonlyText);
      return group;
    }
    if (activeJob !== "remove") {
      var idleText = document.createElement("span");
      idleText.className = "access-control-locked";
      idleText.textContent = activeJob === "change_role" ? "Role change" : "Review only";
      group.appendChild(idleText);
      return group;
    }
    if (normalizedAccessRole(member.role) === "owner" && !isOwner) {
      var lockedText = document.createElement("span");
      lockedText.className = "access-control-locked";
      lockedText.textContent = "Locked";
      group.appendChild(lockedText);
      return group;
    }
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "btn-remove";
    btn.textContent = "Remove access";
    btn.setAttribute("data-action", "remove-member");
    btn.setAttribute("data-account-id", accountID);
    btn.setAttribute("data-user-id", member.user_id);
    btn.setAttribute("data-member-email", member.email);
    group.appendChild(btn);
    return group;
  }
  function renderAccessMemberRow(accountID, member, isOwner, canManage, activeJob) {
    var row = document.createElement("div");
    row.className = "access-member-row";
    var identity = document.createElement("div");
    identity.className = "access-member-identity";
    var topline = document.createElement("div");
    topline.className = "access-member-topline";
    var email = document.createElement("div");
    email.className = "access-member-email";
    email.textContent = member.email;
    topline.appendChild(email);
    var roleBadge = document.createElement("span");
    roleBadge.className = "access-inline-role-badge";
    roleBadge.textContent = roleLabel(member.role);
    topline.appendChild(roleBadge);
    identity.appendChild(topline);
    var caption = document.createElement("div");
    caption.className = "access-member-caption";
    caption.textContent = roleCapabilityCopy(member.role);
    identity.appendChild(caption);
    row.appendChild(identity);
    row.appendChild(renderAccessRoleControl(accountID, member, isOwner, canManage, activeJob));
    row.appendChild(renderAccessMemberAction(accountID, member, isOwner, canManage, activeJob) || createAccessControlCell("access-control-cell-access"));
    return row;
  }
  function renderAccessRosterHead(container, activeJob) {
    var head = document.createElement("div");
    head.className = "access-roster-head";
    head.innerHTML = "<span>Operator</span><span>" + (activeJob === "change_role" ? "New role" : "Role") + "</span><span>" + (activeJob === "remove" ? "Remove" : "Action") + "</span>";
    container.appendChild(head);
  }
  function renderAddWorkspaceSection(accountID, entry) {
    var form = getElement("add-ws-form-" + accountID);
    var spinner = getElement("ws-spinner-" + accountID);
    if (!form) return;
    form.classList.toggle("visible", entry.addWorkspaceOpen);
    if (spinner) {
      spinner.hidden = !entry.createWorkspace.pending;
    }
  }
  function renderAccessSection(accountID, entry) {
    var section = getElement("access-section-" + accountID);
    var roster = getElement("access-list-" + accountID);
    if (!section || !roster) return;
    var rosterPanel = roster.closest(".access-roster");
    var shell = getElement("access-shell-" + accountID);
    var detail = getElement("access-detail-" + accountID);
    var taskPanel = getElement("access-task-panel-" + accountID);
    var taskTitle = getElement("access-task-title-" + accountID);
    var taskCopy = getElement("access-task-copy-" + accountID);
    var taskButtons = {
      invite: getElement("access-task-invite-" + accountID),
      change_role: getElement("access-task-change_role-" + accountID),
      remove: getElement("access-task-remove-" + accountID)
    };
    var taskBodies = {
      invite: getElement("access-task-body-invite-" + accountID),
      change_role: getElement("access-task-body-change_role-" + accountID),
      remove: getElement("access-task-body-remove-" + accountID)
    };
    var actorRole = section.getAttribute("data-actor-role") || "";
    var isOwner = actorRole === "owner";
    var canManage = section.getAttribute("data-can-manage") === "true";
    var activeJob = canManage ? entry.activeAccessJob : "";
    section.classList.toggle("visible", entry.accessVisible);
    renderAccessStats(accountID, entry, canManage);
    if (shell) {
      shell.classList.toggle("access-shell-job-open", !!activeJob);
      shell.classList.toggle("access-shell-idle", !activeJob);
    }
    if (detail) detail.hidden = !activeJob;
    if (taskPanel) taskPanel.hidden = !activeJob;
    if (taskTitle) taskTitle.textContent = accessJobTitle(activeJob);
    if (taskCopy) taskCopy.textContent = accessJobCopy(activeJob);
    taskButtons.invite?.classList.toggle("is-active", activeJob === "invite");
    taskButtons.change_role?.classList.toggle("is-active", activeJob === "change_role");
    taskButtons.remove?.classList.toggle("is-active", activeJob === "remove");
    if (taskBodies.invite) taskBodies.invite.hidden = activeJob !== "invite";
    if (taskBodies.change_role) taskBodies.change_role.hidden = activeJob !== "change_role";
    if (taskBodies.remove) taskBodies.remove.hidden = activeJob !== "remove";
    if (!entry.accessVisible) {
      return;
    }
    if (entry.accessQuery.status === "loading") {
      if (rosterPanel) rosterPanel.classList.add("state-only");
      setContainerMessage(roster, "Loading roster", "Checking who currently has access to this account.", false);
      return;
    }
    if (entry.accessQuery.status === "error") {
      if (rosterPanel) rosterPanel.classList.add("state-only");
      setContainerMessage(roster, "Roster needs attention", entry.accessQuery.error, true);
      return;
    }
    if (!entry.accessQuery.data.length) {
      if (rosterPanel) rosterPanel.classList.add("state-only");
      setContainerMessage(
        roster,
        "No one added yet",
        canManage ? "Invite someone when this hosted account needs shared access." : "There is no hosted roster to review yet on this account.",
        false
      );
      return;
    }
    roster.textContent = "";
    roster.classList.remove("state-only");
    if (rosterPanel) rosterPanel.classList.remove("state-only");
    renderAccessRosterHead(roster, activeJob);
    for (var i = 0; i < entry.accessQuery.data.length; i += 1) {
      var member = entry.accessQuery.data[i];
      roster.appendChild(renderAccessMemberRow(accountID, member, isOwner, canManage, activeJob));
    }
  }
  function renderAccountUI(accountState, accounts) {
    var accountIDs = Object.keys(accountState.byAccountID);
    for (var i = 0; i < accountIDs.length; i += 1) {
      var accountID = accountIDs[i];
      var entry = accountState.byAccountID[accountID];
      var account = null;
      for (var j = 0; j < accounts.length; j += 1) {
        if (accounts[j].id === accountID) {
          account = accounts[j];
          break;
        }
      }
      renderAddWorkspaceSection(accountID, entry);
      if (account) renderWorkspaceManagement(account, entry);
      renderAccessSection(accountID, entry);
    }
  }

  // src/account_controller.ts
  function installAccountController(deps) {
    document.addEventListener("click", function(event) {
      var target = asHTMLElement(event.target);
      if (!target) return;
      var actionEl = target.closest("[data-action]");
      if (!actionEl) return;
      var action = actionEl.getAttribute("data-action") || "";
      var accountID = actionEl.getAttribute("data-account-id") || "";
      switch (action) {
        case "toggle-add-workspace":
          event.preventDefault();
          deps.setShellSection("workspaces");
          deps.runtime.toggleAddWorkspace(accountID);
          return;
        case "open-billing":
          event.preventDefault();
          void deps.runtime.openBilling(accountID);
          return;
        case "show-access":
          event.preventDefault();
          deps.setShellSection("access");
          deps.runtime.ensureAccessVisible(accountID);
          return;
        case "set-access-job":
          event.preventDefault();
          deps.setShellSection("access");
          void deps.runtime.setAccessJob(accountID, actionEl.getAttribute("data-access-job") || "");
          return;
        case "clear-access-job":
          event.preventDefault();
          deps.setShellSection("access");
          deps.runtime.clearAccessJob(accountID);
          return;
        case "invite-member":
          event.preventDefault();
          void deps.runtime.inviteMember(accountID);
          return;
        case "create-workspace":
          event.preventDefault();
          deps.setShellSection("workspaces");
          void deps.runtime.createWorkspace(accountID);
          return;
        case "select-workspace":
          event.preventDefault();
          deps.setShellSection("workspaces");
          deps.runtime.selectWorkspace(
            accountID,
            actionEl.getAttribute("data-workspace-id") || ""
          );
          return;
        case "clear-workspace-selection":
          event.preventDefault();
          deps.setShellSection("workspaces");
          deps.runtime.clearWorkspaceSelection(accountID);
          return;
        case "workspace-action":
          event.preventDefault();
          deps.setShellSection("workspaces");
          void deps.runtime.manageWorkspaceAction(
            accountID,
            actionEl.getAttribute("data-workspace-id") || "",
            actionEl.getAttribute("data-workspace-action") || "",
            actionEl.getAttribute("data-workspace-name") || ""
          );
          return;
        case "remove-member":
          event.preventDefault();
          void deps.runtime.removeMember(
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
      void deps.runtime.changeRole(
        target.getAttribute("data-account-id") || "",
        target.getAttribute("data-user-id") || "",
        target.value
      );
    });
  }

  // src/async_state.ts
  function resetMutationState(state) {
    state.pending = false;
    state.error = "";
  }
  function beginMutationState(state) {
    resetMutationState(state);
    state.pending = true;
  }
  function succeedMutationState(state) {
    resetMutationState(state);
  }
  function failMutationState(state, message) {
    state.pending = false;
    state.error = message;
  }
  function beginQueryState(state, emptyData) {
    state.status = "loading";
    state.error = "";
    state.data = emptyData;
  }
  function resolveQueryState(state, data) {
    state.status = "ready";
    state.error = "";
    state.data = data;
  }
  function failQueryState(state, emptyData, message) {
    state.status = "error";
    state.error = message;
    state.data = emptyData;
  }

  // src/api.ts
  var PortalAPIError = class extends Error {
    constructor(message, status = 0, payload = null) {
      super(message);
      this.name = "PortalAPIError";
      this.status = status;
      this.payload = payload;
    }
  };
  function createPortalAPI(context) {
    function bootstrap() {
      return context.getBootstrap();
    }
    async function readPayload(response) {
      var contentType = response.headers && typeof response.headers.get === "function" ? response.headers.get("content-type") || "" : "";
      if (typeof response.json === "function") {
        try {
          return await response.json();
        } catch {
        }
      }
      if (contentType.includes("application/json")) {
        try {
          return await response.json();
        } catch {
          return null;
        }
      }
      try {
        var text = await response.text();
        return text || null;
      } catch {
        return null;
      }
    }
    function messageFromPayload(payload, fallback) {
      if (payload && typeof payload === "object") {
        var errorMessage = payload.error;
        if (typeof errorMessage === "string" && errorMessage.trim()) {
          return errorMessage;
        }
        var message = payload.message;
        if (typeof message === "string" && message.trim()) {
          return message;
        }
      }
      if (typeof payload === "string" && payload.trim()) {
        return payload;
      }
      return fallback;
    }
    async function request(input, init, fallbackMessage) {
      var response;
      try {
        if (Object.keys(init).length > 0) {
          response = await fetch(input, init);
        } else {
          response = await fetch(input);
        }
      } catch {
        throw new PortalAPIError("Network error.", 0, null);
      }
      var payload = await readPayload(response);
      if (!response.ok) {
        throw new PortalAPIError(messageFromPayload(payload, fallbackMessage), response.status, payload);
      }
      return payload;
    }
    function accountURL(accountID, suffix = "") {
      return bootstrap().account_api_base_path + "/" + encodeURIComponent(accountID) + suffix;
    }
    return {
      fetchBootstrap: function() {
        return request(bootstrap().bootstrap_path, {
          headers: { Accept: "application/json" }
        }, "Failed to refresh account state.");
      },
      requestMagicLink: function(email) {
        return request(bootstrap().magic_link_request_path, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, target: "portal" })
        }, "Failed to send magic link.");
      },
      logout: function() {
        return request(bootstrap().logout_path, {
          method: "POST"
        }, "Failed to sign out.");
      },
      postCommercialJSON: function(path, body) {
        return request(bootstrap().commercial_api_base_url + path, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        }, "Commercial request failed.");
      },
      createWorkspace: function(accountID, body) {
        return request(accountURL(accountID, "/tenants"), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        }, "Failed to create workspace.");
      },
      suspendWorkspace: function(accountID, tenantID) {
        return request(accountURL(accountID, "/tenants/" + encodeURIComponent(tenantID)), {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ state: "suspended" })
        }, "Failed to suspend workspace.");
      },
      deleteWorkspace: function(accountID, tenantID) {
        return request(accountURL(accountID, "/tenants/" + encodeURIComponent(tenantID)), {
          method: "DELETE"
        }, "Failed to delete workspace.");
      },
      openBilling: function(accountID) {
        return request(bootstrap().portal_api_base_path + "/billing?account_id=" + encodeURIComponent(accountID), {
          method: "POST"
        }, "Failed to open billing portal.");
      },
      listMembers: function(accountID) {
        return request(accountURL(accountID, "/members"), {}, "Failed to load access roster.");
      },
      inviteMember: function(accountID, body) {
        return request(accountURL(accountID, "/members"), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        }, "Failed to invite member.");
      },
      updateMemberRole: function(accountID, userID, body) {
        return request(accountURL(accountID, "/members/" + encodeURIComponent(userID)), {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        }, "Failed to update role.");
      },
      removeMember: function(accountID, userID) {
        return request(accountURL(accountID, "/members/" + encodeURIComponent(userID)), {
          method: "DELETE"
        }, "Failed to remove member.");
      }
    };
  }

  // src/state.ts
  function emptyStatus() {
    return {
      visible: false,
      message: "",
      error: false
    };
  }
  function newVerificationFlowState() {
    return {
      pendingEmail: "",
      request: createMutationState(),
      confirm: createMutationState(),
      step2Visible: false,
      status: emptyStatus(),
      result: null,
      emailValue: "",
      codeValue: "",
      checkboxChecked: false
    };
  }
  function createPortalLoginState() {
    return {
      emailValue: "",
      request: createMutationState(),
      success: false,
      successMessage: ""
    };
  }
  function createPortalAccountState() {
    return {
      byAccountID: {}
    };
  }
  function createPortalShellState() {
    return {
      activeSection: "overview"
    };
  }
  function createMutationState() {
    return {
      pending: false,
      error: ""
    };
  }
  function createQueryState(data) {
    return {
      status: "idle",
      data,
      error: ""
    };
  }
  function ensurePortalAccountUIEntry(accountState, accountID) {
    if (!accountState.byAccountID[accountID]) {
      accountState.byAccountID[accountID] = {
        addWorkspaceOpen: false,
        createWorkspace: createMutationState(),
        selectedWorkspaceID: "",
        manageWorkspace: createMutationState(),
        accessVisible: false,
        activeAccessJob: "",
        accessQuery: createQueryState([])
      };
    }
    return accountState.byAccountID[accountID];
  }
  function createPortalBillingState() {
    return {
      openBillingPanelID: "",
      flows: {
        manage: newVerificationFlowState(),
        retrieve: newVerificationFlowState(),
        export: newVerificationFlowState(),
        delete: newVerificationFlowState()
      },
      refund: {
        emailValue: "",
        tokenValue: "",
        submit: createMutationState(),
        status: emptyStatus()
      }
    };
  }
  function syncLoginStateBootstrapEmail(loginState, email) {
    if (!loginState.emailValue) {
      loginState.emailValue = email || "";
    }
  }
  function syncBillingStateBootstrapEmail(billingState, email) {
    if (!billingState.flows.manage.emailValue) billingState.flows.manage.emailValue = email || "";
    if (!billingState.flows.retrieve.emailValue) billingState.flows.retrieve.emailValue = email || "";
    if (!billingState.flows.export.emailValue) billingState.flows.export.emailValue = email || "";
    if (!billingState.flows.delete.emailValue) billingState.flows.delete.emailValue = email || "";
    if (!billingState.refund.emailValue) billingState.refund.emailValue = email || "";
  }
  function setFlowStatus(billingState, flowID, message, isError) {
    billingState.flows[flowID].status = {
      visible: true,
      message,
      error: !!isError
    };
  }
  function clearFlowStatus(billingState, flowID) {
    billingState.flows[flowID].status = emptyStatus();
  }
  function setRefundStatus(billingState, message, isError) {
    billingState.refund.status = {
      visible: true,
      message,
      error: !!isError
    };
  }
  function toggleBillingPanelState(billingState, panelID) {
    billingState.openBillingPanelID = billingState.openBillingPanelID === panelID ? "" : panelID;
  }
  function resetVerificationFlowState(billingState, flowID) {
    var previous = billingState.flows[flowID];
    billingState.flows[flowID] = newVerificationFlowState();
    billingState.flows[flowID].emailValue = previous.emailValue;
  }
  function updateBillingInputValue(billingState, inputKind, value) {
    switch (inputKind) {
      case "manage-email":
        billingState.flows.manage.emailValue = value;
        return;
      case "manage-code":
        billingState.flows.manage.codeValue = value;
        return;
      case "retrieve-email":
        billingState.flows.retrieve.emailValue = value;
        return;
      case "retrieve-code":
        billingState.flows.retrieve.codeValue = value;
        return;
      case "refund-email":
        billingState.refund.emailValue = value;
        return;
      case "refund-token":
        billingState.refund.tokenValue = value;
        return;
      case "data-export-email":
        billingState.flows.export.emailValue = value;
        return;
      case "data-export-code":
        billingState.flows.export.codeValue = value;
        return;
      case "data-delete-email":
        billingState.flows.delete.emailValue = value;
        return;
      case "data-delete-code":
        billingState.flows.delete.codeValue = value;
        return;
      default:
        return;
    }
  }
  function updateDeleteConfirmation(billingState, checked) {
    billingState.flows.delete.checkboxChecked = checked;
  }

  // src/account_runtime.ts
  function installAccountRuntime(deps) {
    var getPortalPath = function() {
      return deps.store.getBootstrap().portal_path;
    };
    var revealElementIfNeeded = function(element) {
      if (!element) return;
      var viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
      if (!viewportHeight) return;
      var rect = element.getBoundingClientRect();
      if (rect.top >= 0 && rect.top <= viewportHeight - 72 && rect.bottom > 0) {
        return;
      }
      if (typeof element.scrollIntoView === "function") {
        element.scrollIntoView({ block: "start", inline: "nearest" });
      }
    };
    var revealElementWhenReady = function(elementID, callback) {
      var reveal = function() {
        revealElementIfNeeded(getElement(elementID));
        if (callback) callback();
      };
      if (typeof window.requestAnimationFrame === "function") {
        window.requestAnimationFrame(reveal);
        return;
      }
      reveal();
    };
    var refreshOrRedirect = async function() {
      if (!await deps.refreshBootstrap()) {
        window.location.href = getPortalPath();
        return false;
      }
      return true;
    };
    var renderAccountRuntime = function() {
      renderAccountUI(deps.store.getAccountState(), deps.store.getBootstrap().accounts || []);
    };
    var loadAccessRoster = async function(accountID) {
      var section = getElement("access-section-" + accountID);
      if (!section) return;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.accessVisible = true;
        beginQueryState(entry.accessQuery, []);
      });
      try {
        var members = await deps.api.listMembers(accountID);
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          resolveQueryState(entry.accessQuery, Array.isArray(members) ? members : []);
        });
      } catch (error) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          failQueryState(entry.accessQuery, [], error instanceof Error ? error.message : "Network error.");
        });
      }
    };
    var refreshAccountAccessSection = async function(accountID) {
      if (!await refreshOrRedirect()) {
        return false;
      }
      var section = getElement("access-section-" + accountID);
      if (!section) {
        return true;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.accessVisible = true;
      });
      await loadAccessRoster(accountID);
      return true;
    };
    var toggleAddWorkspace = function(accountID) {
      var shouldFocus = false;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = !entry.addWorkspaceOpen;
        if (entry.addWorkspaceOpen) {
          entry.accessVisible = false;
          entry.activeAccessJob = "";
          entry.selectedWorkspaceID = "";
        }
        shouldFocus = entry.addWorkspaceOpen;
      });
      if (shouldFocus) {
        revealElementWhenReady("workspace-management-" + accountID, function() {
          focusElement("ws-name-" + accountID);
        });
      }
    };
    var selectWorkspace = function(accountID, workspaceID) {
      var selectedWorkspaceID = "";
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.selectedWorkspaceID = entry.selectedWorkspaceID === workspaceID ? "" : workspaceID;
        if (entry.selectedWorkspaceID) {
          entry.accessVisible = false;
          entry.activeAccessJob = "";
          entry.addWorkspaceOpen = false;
        }
        selectedWorkspaceID = entry.selectedWorkspaceID;
      });
      if (selectedWorkspaceID) {
        revealElementWhenReady("workspace-management-" + accountID);
      }
    };
    var clearWorkspaceSelection = function(accountID) {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.selectedWorkspaceID = "";
      });
    };
    var createWorkspace = async function(accountID) {
      var nameEl = getElement("ws-name-" + accountID);
      if (!nameEl) return;
      var name = nameEl.value.trim();
      if (!name) {
        nameEl.focus();
        return;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        beginMutationState(entry.createWorkspace);
      });
      try {
        await deps.api.createWorkspace(accountID, { display_name: name });
        if (!await refreshOrRedirect()) {
          deps.store.updateAccountState(function(accountState) {
            var entry = ensurePortalAccountUIEntry(accountState, accountID);
            resetMutationState(entry.createWorkspace);
          }, { notify: false });
          return;
        }
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          entry.addWorkspaceOpen = false;
          succeedMutationState(entry.createWorkspace);
        });
        deps.showToast("Workspace created!");
      } catch (error) {
        var message = error instanceof Error ? error.message : "Failed to create workspace.";
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          failMutationState(entry.createWorkspace, message);
        }, { notify: false });
        deps.showToast(message, true);
      }
    };
    var manageWorkspaceAction = async function(accountID, tenantID, action, name) {
      var verb = action === "suspend" ? "Suspend" : action === "delete" ? "Delete" : "";
      if (!verb) return;
      if (!window.confirm(verb + ' workspace "' + name + '"?')) return;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        beginMutationState(entry.manageWorkspace);
      });
      try {
        if (action === "suspend") {
          await deps.api.suspendWorkspace(accountID, tenantID);
        } else {
          await deps.api.deleteWorkspace(accountID, tenantID);
        }
        if (!await refreshOrRedirect()) {
          deps.store.updateAccountState(function(accountState) {
            var entry = ensurePortalAccountUIEntry(accountState, accountID);
            resetMutationState(entry.manageWorkspace);
          }, { notify: false });
          return;
        }
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          entry.selectedWorkspaceID = "";
          succeedMutationState(entry.manageWorkspace);
        });
        deps.showToast(verb + "ed workspace.");
      } catch (error) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          failMutationState(entry.manageWorkspace, error instanceof Error ? error.message : "Failed to " + verb.toLowerCase() + " workspace.");
        }, { notify: false });
        deps.showToast(error instanceof Error ? error.message : "Failed to " + verb.toLowerCase() + " workspace.", true);
      }
    };
    var openBilling = async function(accountID) {
      try {
        var data = await deps.api.openBilling(accountID);
        if (data && data.url) {
          window.location.href = data.url;
        } else {
          deps.showToast("Failed to open billing portal.", true);
        }
      } catch (error) {
        deps.showToast(error instanceof Error ? error.message : "Failed to open billing portal.", true);
      }
    };
    var ensureAccessVisible = function(accountID) {
      var shouldLoad = false;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        if (!entry.accessVisible) {
          entry.accessVisible = true;
        }
        entry.selectedWorkspaceID = "";
        entry.addWorkspaceOpen = false;
        shouldLoad = entry.accessQuery.status === "idle" || entry.accessQuery.status === "error";
      });
      if (shouldLoad) {
        void loadAccessRoster(accountID);
      }
    };
    var setAccessJob = async function(accountID, job) {
      var nextJob = "";
      var shouldLoad = false;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.accessVisible = true;
        entry.selectedWorkspaceID = "";
        entry.addWorkspaceOpen = false;
        entry.activeAccessJob = entry.activeAccessJob === job ? "" : job;
        nextJob = entry.activeAccessJob;
        shouldLoad = entry.accessQuery.status === "idle" || entry.accessQuery.status === "error";
      });
      if (shouldLoad) {
        await loadAccessRoster(accountID);
      }
      if (nextJob) {
        revealElementWhenReady("access-detail-" + accountID, function() {
          if (nextJob === "invite") {
            focusElement("invite-email-" + accountID);
          }
        });
      }
    };
    var clearAccessJob = function(accountID) {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.activeAccessJob = "";
      });
    };
    var inviteMember = async function(accountID) {
      var emailEl = getElement("invite-email-" + accountID);
      var roleEl = getElement("invite-role-" + accountID);
      if (!emailEl || !roleEl) return;
      var email = emailEl.value.trim();
      if (!email) {
        emailEl.focus();
        return;
      }
      try {
        await deps.api.inviteMember(accountID, { email, role: roleEl.value });
        emailEl.value = "";
        if (!await refreshAccountAccessSection(accountID)) {
          return;
        }
        deps.showToast("Member invited!");
      } catch (error) {
        if (error instanceof PortalAPIError && error.status === 409) {
          deps.showToast("Member already exists.", true);
          return;
        }
        deps.showToast(error instanceof Error ? error.message : "Failed to invite member.", true);
      }
    };
    var changeRole = async function(accountID, userID, newRole) {
      try {
        await deps.api.updateMemberRole(accountID, userID, { role: newRole });
        if (!await refreshAccountAccessSection(accountID)) {
          return;
        }
        deps.showToast("Role updated.");
      } catch (error) {
        if (error instanceof PortalAPIError && error.status === 409) {
          deps.showToast("Cannot demote last owner.", true);
          await loadAccessRoster(accountID);
          return;
        }
        deps.showToast(error instanceof Error ? error.message : "Failed to update role.", true);
        await loadAccessRoster(accountID);
      }
    };
    var removeMember = async function(accountID, userID, email) {
      if (!window.confirm("Remove " + email + " from this account?")) return;
      try {
        await deps.api.removeMember(accountID, userID);
        if (!await refreshAccountAccessSection(accountID)) {
          return;
        }
        deps.showToast("Member removed.");
      } catch (error) {
        if (error instanceof PortalAPIError && error.status === 409) {
          deps.showToast("Cannot remove last owner.", true);
          return;
        }
        deps.showToast(error instanceof Error ? error.message : "Failed to remove member.", true);
      }
    };
    deps.store.subscribeAccount(renderAccountRuntime);
    deps.store.subscribeBootstrap(renderAccountRuntime);
    return {
      toggleAddWorkspace,
      selectWorkspace,
      clearWorkspaceSelection,
      openBilling,
      ensureAccessVisible,
      setAccessJob,
      clearAccessJob,
      inviteMember,
      createWorkspace,
      manageWorkspaceAction,
      removeMember,
      changeRole
    };
  }

  // src/auth_controller.ts
  function asHTMLElement2(target) {
    return target instanceof HTMLElement ? target : null;
  }
  function getElement2(id) {
    return document.getElementById(id);
  }
  function installAuthController(deps) {
    deps.store.updateLoginState(function(loginState) {
      var bootstrap = deps.store.getBootstrap();
      syncLoginStateBootstrapEmail(loginState, bootstrap.email || "");
    }, { notify: false });
    async function sendMagicLink() {
      var loginState = deps.store.getLoginState();
      var email = String(loginState.emailValue || "").trim();
      if (!email) {
        var input = getElement2("portal-login-email");
        if (input) input.focus();
        return;
      }
      deps.store.updateLoginState(function(nextState) {
        beginMutationState(nextState.request);
        nextState.success = false;
        nextState.successMessage = "";
      });
      try {
        var response = await deps.api.requestMagicLink(email);
        deps.store.updateLoginState(function(nextState) {
          succeedMutationState(nextState.request);
          nextState.success = true;
          nextState.successMessage = String(response?.message || "").trim();
        });
        return;
      } catch (error) {
        if (error instanceof PortalAPIError && error.status === 404) {
          deps.store.updateLoginState(function(nextState) {
            succeedMutationState(nextState.request);
            nextState.success = true;
            nextState.successMessage = "";
          });
          return;
        }
        deps.store.updateLoginState(function(nextState) {
          failMutationState(
            nextState.request,
            error instanceof PortalAPIError && error.status === 429 ? "Too many requests. Please wait a moment and try again." : "Network error. Please check your connection and try again."
          );
        });
      }
    }
    document.addEventListener("click", function(event) {
      var portalActionEl = asHTMLElement2(event.target)?.closest("[data-portal-action]");
      if (portalActionEl) {
        var portalAction = portalActionEl.getAttribute("data-portal-action") || "";
        switch (portalAction) {
          case "send-magic-link":
            event.preventDefault();
            void sendMagicLink();
            return;
          case "resend-magic-link":
            event.preventDefault();
            deps.store.updateLoginState(function(nextState) {
              nextState.success = false;
              nextState.successMessage = "";
              resetMutationState(nextState.request);
            });
            void sendMagicLink();
            return;
          default:
            break;
        }
      }
      var logoutBtn = asHTMLElement2(event.target)?.closest("#logout-btn");
      if (!logoutBtn) {
        return;
      }
      event.preventDefault();
      logoutBtn.disabled = true;
      logoutBtn.textContent = "Signing out\u2026";
      (async function() {
        try {
          await deps.api.logout();
        } catch (_) {
        }
        window.location.href = deps.store.getBootstrap().portal_path;
      })();
    });
    document.addEventListener("input", function(event) {
      var target = asHTMLElement2(event.target);
      if (!target) return;
      if (target.getAttribute("data-portal-input") === "login-email") {
        deps.store.updateLoginState(function(nextState) {
          nextState.emailValue = target.value;
        }, { notify: false });
      }
    });
    return {
      getLoginState: function() {
        return deps.store.getLoginState();
      }
    };
  }

  // src/billing_view.ts
  function getElement3(id) {
    return document.getElementById(id);
  }
  function asHTMLElement3(target) {
    return target instanceof HTMLElement ? target : null;
  }
  function escapeText(value) {
    return String(value || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  function escapeAttribute(value) {
    return escapeText(value).replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
  function readValue(id) {
    var el = getElement3(id);
    return el ? el.value.trim() : "";
  }
  function focusElement2(id) {
    var el = getElement3(id);
    if (!el) return;
    if (typeof el.focus === "function") {
      try {
        el.focus({ preventScroll: true });
        return;
      } catch (_) {
        el.focus();
      }
    }
  }
  function setVisible(id, visible) {
    var el = getElement3(id);
    if (el) {
      el.hidden = !visible;
    }
  }
  function setValue(id, value) {
    var el = getElement3(id);
    if (el) {
      el.value = value;
    }
  }
  function renderBillingStatus(id, status) {
    var el = getElement3(id);
    if (!el) return;
    if (!status.visible) {
      el.textContent = "";
      el.className = "billing-status";
      return;
    }
    el.textContent = status.message;
    el.className = "billing-status visible" + (status.error ? " error" : " success");
  }
  function renderButton(id, disabled, label) {
    if (!id || !label) return;
    var button = getElement3(id);
    if (!button) return;
    button.disabled = disabled;
    button.textContent = label;
  }
  function renderOpenBillingPanels(openBillingPanelID) {
    var shell = document.querySelector(".billing-shell");
    var detailShell = getElement3("billing-detail-shell");
    var panels = ["manage-billing-panel", "retrieve-billing-panel", "refund-billing-panel", "data-billing-panel"];
    var hasOpenPanel = !!openBillingPanelID;
    if (shell) {
      shell.classList.toggle("billing-shell-job-open", hasOpenPanel);
      shell.classList.toggle("billing-shell-idle", !hasOpenPanel);
    }
    if (detailShell) {
      detailShell.hidden = !hasOpenPanel;
    }
    for (var i = 0; i < panels.length; i++) {
      var panel = getElement3(panels[i]);
      if (!panel) continue;
      var isActive = panels[i] === openBillingPanelID;
      panel.hidden = !isActive;
      panel.classList.toggle("visible", isActive);
    }
    var billingButtons = document.querySelectorAll('[data-account-billing-action="open-billing-panel"]');
    for (var j = 0; j < billingButtons.length; j += 1) {
      var button = billingButtons[j];
      var row = button.closest(".billing-action-row");
      if (!row) continue;
      row.classList.toggle("active", button.getAttribute("data-account-billing-panel") === openBillingPanelID);
    }
    if (hasOpenPanel && detailShell) {
      var reveal = function() {
        if (typeof detailShell.scrollIntoView !== "function") return;
        var rect = detailShell.getBoundingClientRect();
        if (rect.top >= 0 && rect.top <= window.innerHeight - 72 && rect.bottom > 0) {
          return;
        }
        detailShell.scrollIntoView({ block: "start", inline: "nearest" });
      };
      if (typeof window.requestAnimationFrame === "function") {
        window.requestAnimationFrame(reveal);
        return;
      }
      reveal();
    }
  }
  function renderRefundPanel(refundState, bootstrap) {
    var root = getElement3("refund-billing-root");
    if (!root) return;
    var refundSupportURL = (bootstrap.public_site_url || "") + "/refund.html?email=" + encodeURIComponent(refundState.emailValue || "");
    root.innerHTML = '<p>Process an eligible self-serve refund for a self-hosted purchase. This revokes the associated license immediately.</p><div class="warning"><strong>Warning:</strong> completing a refund immediately revokes the affected license. This should only be used when the refund window and commercial contract allow it.</div><div class="form-group"><label for="refund-inline-email">Email address</label><input type="email" id="refund-inline-email" value="' + escapeAttribute(refundState.emailValue || "") + '" autocomplete="email" data-account-billing-input="refund-email"></div><div class="form-group"><label for="refund-inline-token">License key</label><input type="text" id="refund-inline-token" value="' + escapeAttribute(refundState.tokenValue || "") + '" placeholder="pulse_xxxxx" data-account-billing-input="refund-token"></div><div class="form-actions"><button class="btn-danger" type="button" id="refund-inline-submit" data-account-billing-action="refund-inline-submit">Process Refund</button></div><div class="helper-text">If this purchase is not eligible for self-serve refund, use the public support path instead: <a href="' + escapeAttribute(refundSupportURL) + '">open refund support page</a>.</div><div class="billing-status" id="refund-inline-status"></div>';
  }
  function renderManagePanel(flowState) {
    var root = getElement3("manage-billing-root");
    if (!root) return;
    root.innerHTML = '<p>Request a verification code for the commercial email, then open the Stripe customer portal for billing changes, invoices, and subscription actions.</p><div id="manage-inline-step1"><div class="form-group"><label for="manage-inline-email">Email address</label><input type="email" id="manage-inline-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-billing-input="manage-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="manage-inline-request" data-account-billing-action="manage-inline-request">Send Verification Code</button></div></div><div id="manage-inline-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="manage-inline-code">Verification code</label><input type="text" id="manage-inline-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="manage-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="manage-inline-confirm" data-account-billing-action="manage-inline-confirm">Open Customer Portal</button></div><div class="helper-text">Need a new code? <a href="#" id="manage-inline-resend" data-account-billing-action="manage-inline-resend">Send again</a></div></div><div class="billing-status" id="manage-inline-status"></div>';
  }
  function renderRetrievePanel(flowState) {
    var root = getElement3("retrieve-billing-root");
    if (!root) return;
    var result = flowState.result;
    var invoiceURL = result && result.invoice_url ? result.invoice_url : "#";
    root.innerHTML = '<p>Request a verification code for the commercial email, then reveal the current active self-hosted license without leaving Pulse Account.</p><div id="retrieve-inline-step1"><div class="form-group"><label for="retrieve-inline-email">Email address</label><input type="email" id="retrieve-inline-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-billing-input="retrieve-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="retrieve-inline-request" data-account-billing-action="retrieve-inline-request">Send Verification Code</button></div></div><div id="retrieve-inline-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="retrieve-inline-code">Verification code</label><input type="text" id="retrieve-inline-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="retrieve-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="retrieve-inline-confirm" data-account-billing-action="retrieve-inline-confirm">Show License</button><button class="btn-secondary" type="button" id="retrieve-inline-copy" data-account-billing-action="retrieve-inline-copy"' + (result ? "" : " hidden") + '>Copy License Key</button><a class="btn-secondary" id="retrieve-inline-invoice" href="' + escapeAttribute(invoiceURL) + '" target="_blank" rel="noopener"' + (result && result.invoice_url ? "" : " hidden") + '>View Invoice</a></div><div class="helper-text">Use the latest active self-hosted license for this commercial email.</div></div><div class="billing-status" id="retrieve-inline-status"></div><div id="retrieve-inline-result" class="billing-result"' + (result ? "" : " hidden") + '><label for="retrieve-inline-token">License key</label><textarea id="retrieve-inline-token" readonly>' + escapeText(result ? result.token : "") + '</textarea><div class="result-grid"><div><div class="result-meta-label">Plan</div><div class="result-meta-value" id="retrieve-inline-tier">' + escapeText(result ? result.tier : "") + '</div></div><div><div class="result-meta-label">Issued</div><div class="result-meta-value" id="retrieve-inline-issued">' + escapeText(result ? new Date(result.issued_at).toLocaleString() : "") + '</div></div><div><div class="result-meta-label">Expires</div><div class="result-meta-value" id="retrieve-inline-expires">' + escapeText(result ? result.expires_at ? new Date(result.expires_at).toLocaleString() : "Does not expire" : "") + '</div></div><div><div class="result-meta-label">Purchase Email</div><div class="result-meta-value" id="retrieve-inline-email-value">' + escapeText(result ? result.email : "") + "</div></div></div></div>";
  }
  function renderExportPanel(flowState) {
    var root = getElement3("data-export-root");
    if (!root) return;
    root.innerHTML = '<h4>Export My Data</h4><div id="data-export-step1"><div class="form-group"><label for="data-export-email">Email address</label><input type="email" id="data-export-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-billing-input="data-export-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="data-export-request" data-account-billing-action="data-export-request">Send Verification Code</button></div></div><div id="data-export-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="data-export-code">Verification code</label><input type="text" id="data-export-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="data-export-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="data-export-confirm" data-account-billing-action="data-export-confirm">Export My Data</button></div><div class="helper-text">Need a new code? <a href="#" id="data-export-resend" data-account-billing-action="data-export-resend">Send again</a></div></div><div class="billing-status" id="data-export-status"></div><div id="data-export-result" class="billing-result"' + (flowState.result ? "" : " hidden") + '><label for="data-export-payload">Export payload</label><textarea id="data-export-payload" readonly>' + escapeText(flowState.result ? JSON.stringify(flowState.result, null, 2) : "") + "</textarea></div>";
  }
  function renderExportResult(result) {
    setVisible("data-export-result", !!result);
    setValue("data-export-payload", result ? JSON.stringify(result, null, 2) : "");
  }
  function renderDeletePanel(flowState) {
    var root = getElement3("data-delete-root");
    if (!root) return;
    root.innerHTML = '<h4>Delete My Data</h4><div class="warning"><strong>Warning:</strong> deleting commercial data also revokes license records and cannot be undone.</div><div id="data-delete-step1"><div class="form-group"><label for="data-delete-email">Email address</label><input type="email" id="data-delete-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-billing-input="data-delete-email"></div><div class="form-actions"><button class="btn-danger" type="button" id="data-delete-request" data-account-billing-action="data-delete-request">Send Verification Code</button></div></div><div id="data-delete-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="data-delete-code">Verification code</label><input type="text" id="data-delete-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="data-delete-code"></div><div class="checkbox-row"><input type="checkbox" id="data-delete-confirm-check"' + (flowState.checkboxChecked ? " checked" : "") + '><span>I understand this permanently deletes my commercial data and revokes associated licenses.</span></div><div class="form-actions"><button class="btn-danger" type="button" id="data-delete-confirm" data-account-billing-action="data-delete-confirm">Delete My Data</button></div><div class="helper-text">Need a new code? <a href="#" id="data-delete-resend" data-account-billing-action="data-delete-resend">Send again</a></div></div><div class="billing-status" id="data-delete-status"></div>';
  }

  // src/billing_controller.ts
  function installBillingController(deps) {
    function revealBillingPanelWhenReady(panelID) {
      var reveal = function() {
        var panel = document.getElementById(panelID);
        if (!panel || panel.hidden || typeof panel.scrollIntoView !== "function") return;
        panel.scrollIntoView({ block: "start", inline: "nearest" });
      };
      if (typeof window.requestAnimationFrame === "function") {
        window.requestAnimationFrame(function() {
          window.requestAnimationFrame(reveal);
        });
        return;
      }
      reveal();
    }
    document.addEventListener("click", function(event) {
      var target = asHTMLElement3(event.target)?.closest("[data-account-billing-action]");
      if (!target) return;
      var action = target.getAttribute("data-account-billing-action") || "";
      var panelID = target.getAttribute("data-account-billing-panel") || "";
      var focusID = target.getAttribute("data-account-billing-focus") || "";
      switch (action) {
        case "open-billing-panel":
          event.preventDefault();
          deps.setShellSection("billing");
          deps.toggleBillingPanel(panelID);
          var targetElement = target;
          if (typeof targetElement.blur === "function") {
            targetElement.blur();
          }
          deps.focusElement(focusID);
          if (panelID) {
            revealBillingPanelWhenReady(panelID);
          }
          return;
        case "clear-billing-panel":
          event.preventDefault();
          deps.setShellSection("billing");
          deps.clearBillingPanel();
          return;
        case "manage-inline-request":
          event.preventDefault();
          deps.requestVerificationCode("manage");
          return;
        case "manage-inline-resend":
          deps.resendVerificationCode("manage", event);
          return;
        case "manage-inline-confirm":
          event.preventDefault();
          deps.confirmVerificationCode("manage");
          return;
        case "retrieve-inline-request":
          event.preventDefault();
          deps.requestVerificationCode("retrieve");
          return;
        case "retrieve-inline-confirm":
          event.preventDefault();
          deps.confirmVerificationCode("retrieve");
          return;
        case "retrieve-inline-copy":
          event.preventDefault();
          deps.copyRetrievedLicense();
          return;
        case "refund-inline-submit":
          event.preventDefault();
          deps.submitRefund();
          return;
        case "data-export-request":
          event.preventDefault();
          deps.requestVerificationCode("export");
          return;
        case "data-export-resend":
          deps.resendVerificationCode("export", event);
          return;
        case "data-export-confirm":
          event.preventDefault();
          deps.confirmVerificationCode("export");
          return;
        case "data-delete-request":
          event.preventDefault();
          deps.requestVerificationCode("delete");
          return;
        case "data-delete-resend":
          deps.resendVerificationCode("delete", event);
          return;
        case "data-delete-confirm":
          event.preventDefault();
          deps.confirmVerificationCode("delete");
          return;
        default:
          return;
      }
    });
    document.addEventListener("input", function(event) {
      var target = asHTMLElement3(event.target);
      if (!target) return;
      var inputKind = target.getAttribute("data-account-billing-input") || "";
      if (!inputKind) return;
      deps.updateInputValue(inputKind, target.value);
    });
    document.addEventListener("change", function(event) {
      var target = asHTMLElement3(event.target);
      if (!target || target.id !== "data-delete-confirm-check") return;
      deps.updateDeleteConfirmation(!!target.checked);
    });
  }

  // src/billing.ts
  function installBillingRuntime(deps) {
    var api = deps.api;
    var store = deps.store;
    store.updateBillingState(function(billingState) {
      if (!billingState.flows) {
        var nextState = createPortalBillingState();
        billingState.openBillingPanelID = nextState.openBillingPanelID;
        billingState.flows = nextState.flows;
        billingState.refund = nextState.refund;
      }
    }, { notify: false });
    function getBillingState() {
      return store.getBillingState();
    }
    function updateBillingState(mutator, notify = true) {
      return store.updateBillingState(mutator, { notify });
    }
    function toggleBillingPanel(panelID) {
      updateBillingState(function(billingState) {
        toggleBillingPanelState(billingState, panelID);
      });
    }
    function clearBillingPanel() {
      updateBillingState(function(billingState) {
        billingState.openBillingPanelID = "";
      });
    }
    function renderFlow(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var flowState = getBillingState().flows[flowID];
      if (flow.renderPanel) {
        flow.renderPanel(flowState);
      }
      renderButton(flow.requestButtonID, flowState.request.pending, flowState.request.pending ? flow.requestPendingLabel : flow.requestLabel);
      renderButton(flow.confirmButtonID, flowState.confirm.pending, flowState.confirm.pending ? flow.confirmPendingLabel : flow.confirmLabel);
      renderBillingStatus(flow.statusID, flowState.status);
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
      var refundState = getBillingState().refund;
      renderRefundPanel(refundState, store.getBootstrap());
      renderButton("refund-inline-submit", refundState.submit.pending, refundState.submit.pending ? "Processing..." : "Process Refund");
      renderBillingStatus("refund-inline-status", refundState.status);
    }
    function resetVerificationFlow(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      updateBillingState(function(billingState) {
        resetVerificationFlowState(billingState, flowID);
      }, false);
      if (flow.codeInputID) {
        setValue(flow.codeInputID, "");
      }
    }
    var verificationFlows = {
      manage: {
        requestPath: "/v1/manage/request",
        confirmPath: "/v1/manage",
        panelID: "manage-billing-panel",
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
          return getBillingState().flows.manage.emailValue;
        },
        readCodeValue: function() {
          return getBillingState().flows.manage.codeValue;
        },
        onRequestStart: function() {
        },
        afterConfirmSuccess: function(data) {
          window.location.href = data.url;
        },
        renderPanel: renderManagePanel
      },
      retrieve: {
        requestPath: "/v1/retrieve-license/request",
        confirmPath: "/v1/retrieve-license",
        panelID: "retrieve-billing-panel",
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
          return getBillingState().flows.retrieve.emailValue;
        },
        readCodeValue: function() {
          return getBillingState().flows.retrieve.codeValue;
        },
        onRequestStart: function() {
          updateBillingState(function(billingState) {
            billingState.flows.retrieve.result = null;
          }, false);
        },
        applyConfirmSuccessState: function(billingState, data) {
          billingState.flows.retrieve.result = data.license;
          billingState.flows.retrieve.codeValue = "";
          setFlowStatus(billingState, "retrieve", "License retrieved successfully.", false);
        },
        renderPanel: renderRetrievePanel
      },
      export: {
        requestPath: "/v1/gdpr/request-export",
        confirmPath: "/v1/gdpr/export",
        panelID: "data-billing-panel",
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
          return getBillingState().flows.export.emailValue;
        },
        readCodeValue: function() {
          return getBillingState().flows.export.codeValue;
        },
        onRequestStart: function() {
          updateBillingState(function(billingState) {
            billingState.flows.export.result = null;
          }, false);
        },
        applyConfirmSuccessState: function(billingState, data) {
          var emailValue = billingState.flows.export.emailValue;
          resetVerificationFlowState(billingState, "export");
          billingState.flows.export.emailValue = emailValue;
          billingState.flows.export.result = data;
          setFlowStatus(billingState, "export", "Data export retrieved successfully.", false);
        },
        renderPanel: renderExportPanel,
        renderResult: renderExportResult
      },
      delete: {
        requestPath: "/v1/gdpr/request-delete",
        confirmPath: "/v1/gdpr/confirm-delete",
        panelID: "data-billing-panel",
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
          return getBillingState().flows.delete.emailValue;
        },
        readCodeValue: function() {
          return getBillingState().flows.delete.codeValue;
        },
        beforeConfirm: function() {
          if (!getElement3("data-delete-confirm-check")?.checked) {
            updateBillingState(function(billingState) {
              setFlowStatus(billingState, "delete", "You must confirm that you understand this action is permanent.", true);
            });
            return false;
          }
          return true;
        },
        applyConfirmSuccessState: function(billingState, data) {
          var emailValue = billingState.flows.delete.emailValue;
          resetVerificationFlowState(billingState, "delete");
          billingState.flows.delete.emailValue = emailValue;
          setFlowStatus(
            billingState,
            "delete",
            data.deleted_count > 0 && data.stripe_reminder ? data.message + " " + data.stripe_reminder : data.message,
            false
          );
        },
        afterConfirmSuccess: function() {
          var checkbox = getElement3("data-delete-confirm-check");
          if (checkbox) {
            checkbox.checked = false;
          }
        },
        renderPanel: renderDeletePanel
      }
    };
    async function requestVerificationCode(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var email = flow.readEmailValue ? flow.readEmailValue() : readValue(flow.emailInputID);
      if (!email) {
        focusElement2(flow.emailInputID);
        return;
      }
      if (flow.onRequestStart) {
        flow.onRequestStart();
      }
      updateBillingState(function(billingState) {
        beginMutationState(billingState.flows[flowID].request);
        clearFlowStatus(billingState, flowID);
      });
      try {
        await api.postCommercialJSON(flow.requestPath, { email });
        updateBillingState(function(billingState) {
          billingState.flows[flowID].pendingEmail = email;
          billingState.flows[flowID].step2Visible = !!flow.step2ID;
          succeedMutationState(billingState.flows[flowID].request);
          setFlowStatus(billingState, flowID, flow.requestSuccessMessage, false);
        });
      } catch (err) {
        var message = err instanceof Error ? err.message : flow.requestErrorMessage;
        updateBillingState(function(billingState) {
          failMutationState(billingState.flows[flowID].request, message);
          setFlowStatus(billingState, flowID, message, true);
        });
      }
    }
    async function resendVerificationCode(flowID, event) {
      if (event) event.preventDefault();
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var email = getBillingState().flows[flowID].pendingEmail;
      if (!email) return;
      try {
        await api.postCommercialJSON(flow.requestPath, { email });
        updateBillingState(function(billingState) {
          setFlowStatus(billingState, flowID, flow.resendSuccessMessage, false);
        });
      } catch (err) {
        updateBillingState(function(billingState) {
          setFlowStatus(billingState, flowID, err instanceof Error ? err.message : flow.requestErrorMessage, true);
        });
      }
    }
    async function confirmVerificationCode(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var email = getBillingState().flows[flowID].pendingEmail;
      var code = flow.readCodeValue ? flow.readCodeValue() : readValue(flow.codeInputID);
      if (!email || !code) return;
      if (flow.beforeConfirm && flow.beforeConfirm() === false) {
        return;
      }
      updateBillingState(function(billingState) {
        beginMutationState(billingState.flows[flowID].confirm);
      });
      try {
        var data = await api.postCommercialJSON(flow.confirmPath, { email, code });
        updateBillingState(function(billingState) {
          succeedMutationState(billingState.flows[flowID].confirm);
          if (flow.applyConfirmSuccessState) {
            flow.applyConfirmSuccessState(billingState, data, email);
          }
        });
        if (flow.afterConfirmSuccess) {
          flow.afterConfirmSuccess(data, email);
        }
      } catch (err) {
        var message = err instanceof Error ? err.message : flow.confirmErrorMessage;
        updateBillingState(function(billingState) {
          failMutationState(billingState.flows[flowID].confirm, message);
          setFlowStatus(billingState, flowID, message, true);
        });
      }
    }
    async function copyRetrievedLicense() {
      var result = getBillingState().flows.retrieve.result;
      var token = result && result.token ? result.token : "";
      if (!token) return;
      try {
        await navigator.clipboard.writeText(token);
        updateBillingState(function(billingState) {
          setFlowStatus(billingState, "retrieve", "License key copied to clipboard.", false);
        });
      } catch (_) {
        updateBillingState(function(billingState) {
          setFlowStatus(billingState, "retrieve", "Failed to copy automatically. Please copy the key manually.", true);
        });
      }
    }
    async function submitRefund() {
      var email = getBillingState().refund.emailValue;
      var token = getBillingState().refund.tokenValue;
      if (!email || !token) return;
      if (!confirm("Are you sure? This will immediately revoke the license and request the refund.")) return;
      updateBillingState(function(billingState) {
        beginMutationState(billingState.refund.submit);
        billingState.refund.status = emptyStatus();
      });
      try {
        await api.postCommercialJSON("/v1/self-refund", { email, token });
        updateBillingState(function(billingState) {
          billingState.refund.tokenValue = "";
          succeedMutationState(billingState.refund.submit);
          setRefundStatus(billingState, "Success! Your refund has been processed. Stripe will follow up by email.", false);
        });
      } catch (err) {
        var message = err instanceof Error ? err.message : "Refund failed";
        updateBillingState(function(billingState) {
          failMutationState(billingState.refund.submit, message);
          setRefundStatus(billingState, message, true);
        });
      }
    }
    function renderBillingRuntime() {
      renderOpenBillingPanels(getBillingState().openBillingPanelID);
      renderAllFlows();
    }
    renderBillingRuntime();
    store.subscribeBootstrap(renderBillingRuntime);
    store.subscribeBilling(renderBillingRuntime);
    installBillingController({
      setShellSection: function(section) {
        store.setActiveShellSection(section);
      },
      toggleBillingPanel,
      clearBillingPanel,
      focusElement: focusElement2,
      requestVerificationCode: function(flowID) {
        void requestVerificationCode(flowID);
      },
      resendVerificationCode: function(flowID, event) {
        void resendVerificationCode(flowID, event);
      },
      confirmVerificationCode: function(flowID) {
        void confirmVerificationCode(flowID);
      },
      copyRetrievedLicense: function() {
        void copyRetrievedLicense();
      },
      submitRefund: function() {
        void submitRefund();
      },
      updateInputValue: function(inputKind, value) {
        updateBillingState(function(billingState) {
          updateBillingInputValue(billingState, inputKind, value);
        }, false);
      },
      updateDeleteConfirmation: function(checked) {
        updateBillingState(function(billingState) {
          updateDeleteConfirmation(billingState, checked);
        }, false);
      }
    });
  }

  // src/shell_view.ts
  function escapeHTML(value) {
    return String(value || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
  function escapeAttr(value) {
    return escapeHTML(value);
  }
  function titleCase(value) {
    if (!value) return "";
    return value.charAt(0).toUpperCase() + value.slice(1);
  }
  function formatWorkspaceDate(value) {
    if (!value) return "";
    var date = new Date(String(value));
    if (Number.isNaN(date.getTime())) return "";
    return date.toLocaleDateString(void 0, { month: "short", day: "numeric", year: "numeric" });
  }
  function workspaceHealthState(workspace) {
    if (workspace.health_status === "healthy" || workspace.health_status === "checking" || workspace.health_status === "unhealthy") {
      return workspace.health_status;
    }
    if (workspace.healthy) return "healthy";
    if (workspace.last_health_check) return "unhealthy";
    return "checking";
  }
  function accountKindLabel(account) {
    if (account.kind === "msp") return "MSP account";
    if (account.kind === "cloud") return "Cloud account";
    if (account.kind === "individual") return "Hosted account";
    return account.kind_label ? account.kind_label + " account" : "Account";
  }
  function workspaceCountLabel(count) {
    return count === 1 ? "1 workspace" : String(count) + " workspaces";
  }
  function hasHostedAccounts(accounts) {
    return accounts.length > 0;
  }
  function hasSelfHostedCommercial(bootstrap) {
    var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
    return bootstrap.has_self_hosted_commercial === true || !hasHostedAccounts(accounts);
  }
  function collectWorkspaces(accounts) {
    var results = [];
    for (var i = 0; i < accounts.length; i += 1) {
      var workspaces = Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces : [];
      for (var j = 0; j < workspaces.length; j += 1) {
        results.push(workspaces[j]);
      }
    }
    return results;
  }
  function collectOverviewWorkspaceEntries(accounts) {
    var results = [];
    for (var i = 0; i < accounts.length; i += 1) {
      var workspaces = Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces : [];
      for (var j = 0; j < workspaces.length; j += 1) {
        results.push({
          account: accounts[i],
          workspace: workspaces[j]
        });
      }
    }
    return results;
  }
  function countWorkspacesByState(workspaces, state) {
    var count = 0;
    for (var i = 0; i < workspaces.length; i += 1) {
      if (String(workspaces[i].state || "") === state) count += 1;
    }
    return count;
  }
  function countReadyWorkspaces(workspaces) {
    var count = 0;
    for (var i = 0; i < workspaces.length; i += 1) {
      if (String(workspaces[i].state || "") === "active" && workspaceHealthState(workspaces[i]) === "healthy") {
        count += 1;
      }
    }
    return count;
  }
  function healthBadgeHTML(workspace) {
    var status = workspaceHealthState(workspace);
    if (status === "healthy") {
      return '<span class="badge badge-healthy">Healthy</span>';
    }
    if (status === "unhealthy") {
      return '<span class="badge badge-unhealthy">Needs attention</span>';
    }
    return '<span class="badge badge-checking">Checking</span>';
  }
  function renderBillingActionRow(id, kicker, title, actionLabel, description, panelID, focusID, highlights) {
    var meta = highlights.join(" \u2022 ");
    return '<article class="billing-action-row"><div class="billing-action-main"><div class="billing-action-tags billing-action-tags-tight"><span class="billing-card-kicker">' + kicker + '</span><span class="billing-action-meta-chip">' + escapeHTML(meta) + '</span></div><div class="billing-action-copy"><h3>' + title + "</h3><p>" + description + '</p></div></div><div class="billing-action-cta"><button class="btn-secondary billing-action-button" type="button" id="' + id + '" data-account-billing-action="open-billing-panel" data-account-billing-panel="' + panelID + '" data-account-billing-focus="' + focusID + '" data-shell-target="billing">' + escapeHTML(actionLabel) + "</button></div></article>";
  }
  function renderSectionContextChips(chips) {
    if (!chips.length) return "";
    return '<div class="section-context-strip">' + chips.map(function(chip) {
      return '<span class="section-context-chip">' + escapeHTML(chip) + "</span>";
    }).join("") + "</div>";
  }
  function workspaceStatusCopy(workspace) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    if (state === "suspended") return "This workspace is suspended and will stay closed until you resume it.";
    if (state === "failed") return "This workspace needs attention before it is trustworthy.";
    if (status === "healthy") return "Live updates and health checks are currently good.";
    if (status === "unhealthy") return "This workspace needs attention before it is trustworthy.";
    return "This workspace is still waiting on a completed health check.";
  }
  function workspaceRowNote(workspace) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    if (state === "suspended") return "Suspended until you resume it";
    if (state === "failed") return "Review this workspace before treating it as stable";
    if (status === "healthy") return "Ready to use";
    if (status === "unhealthy") return "Review this workspace before treating it as stable";
    return "Awaiting a completed health check";
  }
  function attentionWorkspaces(workspaces) {
    var results = [];
    for (var i = 0; i < workspaces.length; i += 1) {
      var status = workspaceHealthState(workspaces[i]);
      if (status === "unhealthy" || status === "checking") {
        results.push(workspaces[i]);
      }
    }
    return results;
  }
  function accountContextRoleMeta(account) {
    return titleCase(account.role) + (account.can_manage ? " access" : " role");
  }
  function accountContextLeadCopy(account) {
    var accountPrefix = account.kind === "msp" ? "Hosted workspace account" : "Hosted account";
    if (account.can_manage) {
      return accountPrefix + (account.has_billing ? " for workspace access, access control, and billing." : " for workspace access and access control.");
    }
    return accountPrefix + (account.has_billing ? " where you can open workspaces and review who already has access. An owner or admin handles access changes and billing." : " where you can open workspaces and review who already has access. An owner or admin handles account changes.");
  }
  function accountContextAccessSummary(account) {
    return account.can_manage ? titleCase(account.role) : "View only";
  }
  function accountContextBillingSummary(account) {
    if (!account.has_billing) return "Not attached";
    return account.can_manage ? "Billing enabled" : "Owner/admin required";
  }
  function renderAccountContextStrip(account) {
    var workspaceLabel = workspaceCountLabel((account.workspaces || []).length);
    var billingLabel = accountContextBillingSummary(account);
    return '<section class="portal-account-context"><div class="portal-account-context-copy"><div class="portal-account-context-meta"><span class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</span><span class="portal-account-context-separator">/</span><span class="portal-account-context-access">' + escapeHTML(accountContextRoleMeta(account)) + '</span></div><div class="portal-account-context-row"><h2>' + escapeHTML(account.name) + '</h2><div class="portal-account-context-chips"><span class="account-context-chip">' + escapeHTML(account.kind_label) + '</span><span class="account-context-chip">' + escapeHTML(titleCase(account.role)) + '</span><span class="account-context-chip">' + escapeHTML(workspaceLabel) + "</span></div></div><p>" + escapeHTML(accountContextLeadCopy(account)) + '</p></div><div class="portal-account-context-summary"><div class="portal-account-context-stat"><span>Access</span><strong>' + escapeHTML(accountContextAccessSummary(account)) + '</strong></div><div class="portal-account-context-stat"><span>Workspaces</span><strong>' + escapeHTML(workspaceLabel) + '</strong></div><div class="portal-account-context-stat"><span>Billing</span><strong>' + escapeHTML(billingLabel) + "</strong></div></div></section>";
  }
  function shellSectionButton(section, activeSection, index, title, copy, badge) {
    var badgeHTML = badge ? '<span class="portal-shell-nav-badge">' + escapeHTML(badge) + "</span>" : "";
    return '<button class="portal-shell-nav-link' + (activeSection === section ? " active" : "") + '" type="button" data-shell-action="activate-section" data-shell-section="' + section + '"><span class="portal-shell-nav-row"><span class="portal-shell-nav-label-group"><span class="portal-shell-nav-index">' + escapeHTML(index) + '</span><span class="portal-shell-nav-label">' + title + "</span></span>" + badgeHTML + '</span><span class="portal-shell-nav-copy">' + copy + "</span></button>";
  }
  function workspaceNavCopy(hosted, canManage) {
    if (!hosted) {
      return "Unavailable on this account. Hosted workspaces are not attached here.";
    }
    if (canManage) {
      return "Open a workspace, review lifecycle state, or create one.";
    }
    return "Open a workspace and review current state. An owner or admin must create or change hosted workspaces.";
  }
  function accessNavCopy(hosted, canManage) {
    if (!hosted) {
      return "Unavailable on this account. Hosted roster and role controls live only on hosted workspace accounts.";
    }
    if (canManage) {
      return "Invite people, change roles, and remove account access.";
    }
    return "Review who already has access to this hosted account. An owner or admin must make changes.";
  }
  function billingNavCopy(hostedBillingCount, canManageHostedBilling) {
    if (hostedBillingCount > 0) {
      if (canManageHostedBilling) {
        return "Hosted billing first, then self-hosted licenses, refunds, and privacy only when relevant.";
      }
      return "Hosted billing is attached here, but an owner or admin must open it.";
    }
    return "Self-hosted billing, licenses, refunds, and privacy.";
  }
  function supportNavCopy(hosted, canManageHostedTasks) {
    if (!hosted) {
      return "Escalation only after the billing path is exhausted.";
    }
    if (canManageHostedTasks) {
      return "Escalation only after the workspace, access, or billing path is exhausted.";
    }
    return "Escalation only after the review, owner/admin, or billing path is exhausted.";
  }
  function renderShellNavigation(accounts, supportEmail, activeSection) {
    var hosted = hasHostedAccounts(accounts);
    var workspaces = collectWorkspaces(accounts);
    var totalWorkspaces = workspaces.length;
    var readyWorkspaces = countReadyWorkspaces(workspaces);
    var attentionCount = attentionWorkspaces(workspaces).length;
    var hostedBillingCount = 0;
    var canManageHostedBilling = false;
    var canManage = false;
    for (var i = 0; i < accounts.length; i += 1) {
      if (accounts[i].can_manage) {
        canManage = true;
      }
      if (accounts[i].has_billing) {
        hostedBillingCount += 1;
        if (accounts[i].can_manage) {
          canManageHostedBilling = true;
        }
      }
    }
    return '<aside class="portal-shell-nav" aria-label="Pulse Account sections"><div class="portal-shell-nav-header"><div class="portal-shell-nav-eyebrow">Pulse Account</div><div class="portal-shell-nav-title">Account tasks</div><div class="portal-shell-nav-support">' + (hosted ? "Start with the job you need to finish: workspace work, access, billing, then escalation." : "Use billing tools first and escalate only when the self-serve path stops.") + '</div></div><div class="portal-shell-nav-group">' + shellSectionButton("overview", activeSection, "01", "Overview", "What needs attention, what is ready, and the next obvious action.", attentionCount > 0 ? String(attentionCount) + " review" : hosted ? String(readyWorkspaces) + " ready" : "Summary") + shellSectionButton("workspaces", activeSection, "02", "Workspaces", workspaceNavCopy(hosted, canManage), hosted ? String(readyWorkspaces) + " ready" : "Unavailable") + shellSectionButton("access", activeSection, "03", "Access", accessNavCopy(hosted, canManage), hosted ? canManage ? "Manage" : "View" : "Unavailable") + shellSectionButton("billing", activeSection, "04", "Billing", billingNavCopy(hostedBillingCount, canManageHostedBilling), hostedBillingCount > 0 ? hostedBillingCount > 1 ? "Hosted +" : "Hosted" : "Self-hosted") + shellSectionButton("support", activeSection, "05", "Support", supportNavCopy(hosted, canManage), supportEmail ? "Email" : "Help") + "</div></aside>";
  }
  function renderWorkspaceCard(account, workspace, accountAPIBasePath) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    var createdLabel = formatWorkspaceDate(workspace.created_at);
    var metaParts = [];
    if (createdLabel) {
      metaParts.push('<span class="workspace-meta-item">Created ' + escapeHTML(createdLabel) + "</span>");
    }
    if (workspace.last_health_check && status === "healthy") {
      metaParts.push('<span class="workspace-meta-item">Checked recently</span>');
    }
    var openAction = "";
    if (state === "active") {
      openAction = '<form method="POST" action="' + escapeAttr(accountAPIBasePath + "/" + account.id + "/tenants/" + workspace.id + "/handoff") + '"><button type="submit" class="btn-primary">Open workspace</button></form>';
    }
    var manageAction = "";
    if (account.can_manage && (state === "active" || state === "suspended" || state === "failed")) {
      manageAction = '<button type="button" class="btn-secondary btn-workspace-manage" data-action="select-workspace" data-account-id="' + escapeAttr(account.id) + '" data-workspace-id="' + escapeAttr(workspace.id) + '">Lifecycle</button>';
    }
    return '<article class="workspace-row workspace-row-health-' + escapeAttr(status) + " workspace-row-state-" + escapeAttr(state || "unknown") + '" data-workspace-row="' + escapeAttr(workspace.id) + '"><div class="workspace-row-primary"><div class="workspace-row-heading"><h4 class="workspace-name">' + escapeHTML(workspace.display_name) + '</h4><div class="workspace-meta">' + metaParts.join("") + '</div></div><div class="workspace-row-note">' + escapeHTML(workspaceRowNote(workspace)) + '</div></div><div class="workspace-row-status-cell workspace-row-status-cell-badge">' + healthBadgeHTML(workspace) + '</div><div class="workspace-row-status-cell workspace-row-status-cell-badge"><span class="badge badge-' + escapeHTML(state || "unknown") + '">' + escapeHTML(titleCase(state || "Unknown")) + '</span></div><div class="workspace-actions">' + openAction + manageAction + "</div></article>";
  }
  function renderWorkspaceHandoffForm(accountID, workspaceID, accountAPIBasePath, label, buttonClassName = "btn-secondary btn-compact") {
    if (!accountAPIBasePath) {
      return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + "</button>";
    }
    return '<form method="POST" action="' + escapeAttr(accountAPIBasePath + "/" + accountID + "/tenants/" + workspaceID + "/handoff") + '"><button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + "</button></form>";
  }
  function attentionOverviewEntries(entries) {
    var results = [];
    for (var i = 0; i < entries.length; i += 1) {
      var status = workspaceHealthState(entries[i].workspace);
      if (status === "unhealthy" || status === "checking") {
        results.push(entries[i]);
      }
    }
    return results;
  }
  function readyOverviewEntries(entries) {
    var results = [];
    for (var i = 0; i < entries.length; i += 1) {
      if (String(entries[i].workspace.state || "") === "active" && workspaceHealthState(entries[i].workspace) === "healthy") {
        results.push(entries[i]);
      }
    }
    return results;
  }
  function overviewWorkspaceContext(entry, includeAccountName, note) {
    if (!includeAccountName) return note;
    return entry.account.name + " \xB7 " + note;
  }
  function overviewBillingSeparationCopy(accounts, showSelfHostedCommercial) {
    var hostedBillingCount = 0;
    var canManageHostedBilling = false;
    for (var i = 0; i < accounts.length; i += 1) {
      if (accounts[i].has_billing) {
        hostedBillingCount += 1;
        if (accounts[i].can_manage) {
          canManageHostedBilling = true;
        }
      }
    }
    if (!accounts.length) {
      return {
        title: "Billing stays separate",
        copy: "Self-hosted billing, licenses, refunds, and privacy stay in Billing."
      };
    }
    if (showSelfHostedCommercial) {
      if (hostedBillingCount > 0) {
        return {
          title: "Billing stays separate",
          copy: canManageHostedBilling ? "Hosted billing stays in Billing, and self-hosted tools appear there only when relevant." : "Hosted billing stays in Billing, an owner or admin opens it, and self-hosted tools appear there only when relevant."
        };
      }
      return {
        title: "Billing stays separate",
        copy: "Self-hosted tools appear in Billing only when they are relevant to this account."
      };
    }
    if (hostedBillingCount > 0) {
      return {
        title: "Hosted billing stays separate",
        copy: canManageHostedBilling ? "Use Billing only for hosted invoices, payment methods, or subscription changes." : "Hosted billing stays in Billing, and an owner or admin must open it."
      };
    }
    return {
      title: "Billing stays separate",
      copy: "Use Billing only when the task is commercial, not operational."
    };
  }
  function renderOverviewAttentionCard(accounts, entries, showSelfHostedCommercial) {
    var attention = attentionOverviewEntries(entries);
    var includeAccountName = accounts.length > 1;
    var suspendedCount = countWorkspacesByState(entries.map(function(entry) {
      return entry.workspace;
    }), "suspended");
    var billingSeparation = overviewBillingSeparationCopy(accounts, showSelfHostedCommercial);
    if (!attention.length) {
      return '<article class="overview-task-card"><div class="account-panel-kicker">Needs attention</div><h4>Nothing urgent</h4><p>' + escapeHTML(
        entries.length > 0 ? "No active workspace is currently asking for review." : "No hosted workspace is currently asking for review."
      ) + '</p><div class="overview-task-list"><div class="overview-task-item"><strong>Healthy now</strong><span>' + escapeHTML(
        entries.length > 0 ? "Active workspaces look clear for routine use." : "There is no hosted workspace waiting for review yet."
      ) + '</span></div><div class="overview-task-item"><strong>' + escapeHTML(suspendedCount > 0 ? "Suspended stays parked" : billingSeparation.title) + "</strong><span>" + escapeHTML(
        suspendedCount > 0 ? String(suspendedCount) + " suspended workspace" + (suspendedCount === 1 ? " stays" : "s stay") + " out of the way until you deliberately resume it." : billingSeparation.copy
      ) + "</span></div></div></article>";
    }
    return '<article class="overview-task-card overview-task-card-attention"><div class="account-panel-kicker">Needs attention</div><h4>Review these first</h4><p>These workspaces still need a human check before you treat the account as settled.</p><div class="overview-task-list">' + attention.slice(0, 3).map(function(entry) {
      return '<div class="overview-task-item"><strong>' + escapeHTML(entry.workspace.display_name) + "</strong><span>" + escapeHTML(overviewWorkspaceContext(entry, includeAccountName, workspaceStatusCopy(entry.workspace))) + "</span></div>";
    }).join("") + "</div></article>";
  }
  function renderOverviewReadyCard(accounts, entries, accountAPIBasePath) {
    var ready = readyOverviewEntries(entries);
    var includeAccountName = accounts.length > 1;
    if (!ready.length) {
      return '<article class="overview-task-card"><div class="account-panel-kicker">Ready</div><h4>' + escapeHTML(accounts.length > 0 ? "No workspace is ready yet" : "Billing tools are ready") + "</h4><p>" + escapeHTML(
        accounts.length > 0 ? "Use Workspaces to review current state before you start routine work." : "Use Billing for self-hosted subscriptions, licenses, refunds, and privacy requests."
      ) + "</p></article>";
    }
    return '<article class="overview-task-card"><div class="account-panel-kicker">Ready</div><h4>Open and work</h4><p>These workspaces are active and healthy right now.</p><div class="overview-task-list">' + ready.slice(0, 3).map(function(entry) {
      return '<div class="overview-task-item overview-task-item-action"><div class="overview-task-copy"><strong>' + escapeHTML(entry.workspace.display_name) + "</strong><span>" + escapeHTML(overviewWorkspaceContext(entry, includeAccountName, workspaceRowNote(entry.workspace))) + "</span></div>" + renderWorkspaceHandoffForm(entry.account.id, entry.workspace.id, accountAPIBasePath, "Open workspace") + "</div>";
    }).join("") + "</div></article>";
  }
  function renderOverviewNextActionCard(accounts, entries, accountAPIBasePath) {
    var attention = attentionOverviewEntries(entries);
    var ready = readyOverviewEntries(entries);
    var primaryAction = "";
    var secondaryAction = "";
    var title = "";
    var description = "";
    var creatableAccount = accounts.find(function(account) {
      return account.kind === "msp" && account.can_manage;
    }) || null;
    var billingAccount = accounts.find(function(account) {
      return account.has_billing && account.can_manage;
    }) || null;
    var accessAccount = accounts.find(function(account) {
      return account.can_manage;
    }) || null;
    if (attention.length) {
      title = "Review workspace health";
      description = attention.length > 1 ? "Open Workspaces and resolve the pending health or lifecycle questions before you do anything else." : "Start in Workspaces with " + attention[0].workspace.display_name + " before you do anything else.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Review workspaces</button>';
      secondaryAction = accessAccount ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Review access</button>' : "";
    } else if (ready.length) {
      title = "Open the next workspace";
      description = accounts.length > 1 ? "The clearest next step is to open " + ready[0].workspace.display_name + " in " + ready[0].account.name + " and continue the actual work there." : "The most obvious next step is to open a ready workspace and continue the actual work there.";
      primaryAction = renderWorkspaceHandoffForm(ready[0].account.id, ready[0].workspace.id, accountAPIBasePath, "Open " + ready[0].workspace.display_name, "btn-primary btn-compact");
      secondaryAction = ready.length > 1 ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">See all workspaces</button>' : "";
    } else if (creatableAccount) {
      title = "Create the next workspace";
      description = "There is no ready workspace yet, so the next clear action is to create one in " + creatableAccount.name + ".";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(creatableAccount.id) + '">Create workspace</button>';
      secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Manage access</button>';
    } else if (billingAccount) {
      title = "Handle billing in its own place";
      description = "Operational work is clear, so the next separate task is billing if that is what you came here to change.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
      secondaryAction = accessAccount ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Review access</button>' : "";
    } else if (!accounts.length) {
      title = "Open billing";
      description = "No hosted workspace is attached, so the next obvious action is Billing for self-hosted subscriptions, licenses, refunds, or privacy requests.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
    } else if (accessAccount) {
      title = "Handle access in its own place";
      description = "If the next task is people or roles, keep it in Access instead of mixing it into routine workspace work.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open access</button>';
      secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
    } else {
      title = "Choose the right task path";
      description = "If this is an access change, go to Access. If it is a billing or license issue, go to Billing. Support is only for escalation.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
      secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="support">Escalate</button>';
    }
    return '<article class="overview-task-card overview-task-card-next"><div class="account-panel-kicker">Next action</div><h4>' + escapeHTML(title) + "</h4><p>" + escapeHTML(description) + '</p><div class="overview-task-actions">' + primaryAction + secondaryAction + "</div></article>";
  }
  function renderShellOverviewSection(context) {
    var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
    var entries = collectOverviewWorkspaceEntries(accounts);
    var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
    var totalCount = entries.length;
    var readyCount = readyOverviewEntries(entries).length;
    var attentionCount = attentionOverviewEntries(entries).length;
    var suspendedCount = countWorkspacesByState(collectWorkspaces(accounts), "suspended");
    var chips = accounts.length ? [
      accounts.length === 1 ? "1 account" : String(accounts.length) + " accounts",
      workspaceCountLabel(totalCount),
      String(readyCount) + " ready",
      attentionCount > 0 ? String(attentionCount) + " attention" : "Nothing urgent",
      suspendedCount > 0 ? String(suspendedCount) + " suspended" : "No suspended"
    ] : ["No hosted account", "Billing available", "Support only on escalation"];
    return '<section class="account-content-panel account-content-panel-overview"><div class="account-stage-header account-stage-header-overview overview-stage-header"><div><div class="account-panel-kicker">Overview</div><h3>Account triage</h3><p>Only three questions matter here.</p>' + renderSectionContextChips(chips) + '</div></div><div class="overview-task-grid">' + renderOverviewAttentionCard(accounts, entries, showSelfHostedCommercial) + renderOverviewReadyCard(accounts, entries, context.accountAPIBasePath) + renderOverviewNextActionCard(accounts, entries, context.accountAPIBasePath) + "</div></section>";
  }
  function renderNoHostedWorkspacesSection() {
    return '<section class="account-content-panel account-content-panel-workspaces"><div class="account-stage-header"><div><div class="account-panel-kicker">Workspaces</div><h3>Workspaces</h3><p>No hosted workspace is attached to this account.</p>' + renderSectionContextChips(["None attached", "Billing instead"]) + '</div></div><div class="empty-state empty-state-spaced"><p>There is nothing to open or manage here yet.</p><p class="support-copy">Use Billing for self-hosted subscriptions, licenses, refunds, or privacy requests.</p></div></section>';
  }
  function renderNoHostedAccessSection() {
    return '<section class="account-content-panel account-content-panel-access"><div class="account-stage-header"><div><div class="account-panel-kicker">Access</div><h3>Access</h3><p>No hosted account roster is attached here.</p>' + renderSectionContextChips(["No hosted roster", "Billing instead"]) + '</div></div><div class="empty-state empty-state-spaced"><p>There are no hosted roles or invites to manage for this account right now.</p><p class="support-copy">If the task is commercial access to licenses, refunds, or privacy, stay in Billing.</p></div></section>';
  }
  function renderAccountWorkspaceSection(account, accountAPIBasePath) {
    var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
    var readyCount = countReadyWorkspaces(workspaces);
    var suspendedCount = countWorkspacesByState(workspaces, "suspended");
    var sectionCopy = account.can_manage ? "Open a workspace, review lifecycle state, or create a new one without mixing in access or billing work." : "Open a workspace and review current state here. An owner or admin must create or change hosted workspaces.";
    var workspaceListSummary = account.can_manage ? "Open a workspace to work in it. Use the lifecycle view only when you are reviewing state or making account-level changes." : "Open a workspace to do the actual work. An owner or admin must handle lifecycle or creation changes.";
    var workspaceManagement = "";
    var addWorkspaceForm = "";
    var workspaceHeaderActions = "";
    if (account.can_manage) {
      if (account.kind === "msp") {
        workspaceHeaderActions += '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Create workspace</button>';
      }
      if (account.kind === "msp") {
        addWorkspaceForm = '<div class="add-workspace-form" id="add-ws-form-' + escapeAttr(account.id) + '"><label for="ws-name-' + escapeAttr(account.id) + '">Workspace name (for example, a client name)</label><input type="text" id="ws-name-' + escapeAttr(account.id) + '" placeholder="Acme Corp" maxlength="80" autocomplete="off"><div class="form-actions"><button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' + escapeAttr(account.id) + '">Create workspace</button><button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Cancel</button><div class="spinner" id="ws-spinner-' + escapeAttr(account.id) + '" hidden></div></div></div>';
      }
      workspaceManagement = '<section class="workspace-management-panel workspace-management-panel-idle" id="workspace-management-' + escapeAttr(account.id) + '" hidden><div class="workspace-management-header"><div><div class="account-panel-kicker">Workspace task</div><h3>Work on one workspace</h3><p>Open lifecycle for one workspace, or create a new one. Keep access and billing separate.</p></div><button type="button" class="btn-secondary btn-compact" id="workspace-management-close-' + escapeAttr(account.id) + '" data-action="clear-workspace-selection" data-account-id="' + escapeAttr(account.id) + '">Close panel</button></div><div class="workspace-management-empty" id="workspace-management-empty-' + escapeAttr(account.id) + '"><div class="workspace-management-empty-shell"><div class="workspace-management-empty-actions-card"><div class="workspace-management-empty-actions-copy"><div class="account-panel-kicker">Create workspace</div><h4>Open a new hosted workspace</h4><p>Create one workspace here when you need a new customer or operating boundary.</p></div>' + addWorkspaceForm + '</div><div class="workspace-management-empty-note">Access changes stay in Access. Billing changes stay in Billing.</div></div></div><div class="workspace-management-content" id="workspace-management-content-' + escapeAttr(account.id) + '" hidden><div class="workspace-management-meta" id="workspace-management-meta-' + escapeAttr(account.id) + '"></div><h4 id="workspace-management-title-' + escapeAttr(account.id) + '"></h4><p class="workspace-management-summary" id="workspace-management-summary-' + escapeAttr(account.id) + '"></p><div class="workspace-management-facts"><div class="workspace-management-fact"><span>Health</span><strong id="workspace-management-health-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Lifecycle</span><strong id="workspace-management-lifecycle-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Created</span><strong id="workspace-management-created-' + escapeAttr(account.id) + '"></strong></div></div><div class="workspace-management-guidance" id="workspace-management-guidance-' + escapeAttr(account.id) + '"></div><div class="workspace-management-actions"><button type="button" class="btn-danger" id="workspace-management-action-' + escapeAttr(account.id) + '" data-action="workspace-action" data-account-id="' + escapeAttr(account.id) + '">Manage workspace</button></div></div></section>';
    }
    var workspaceHTML = workspaces.length ? '<div class="workspace-list-wrap"><div class="workspace-list-toolbar"><div class="workspace-list-summary">' + escapeHTML(workspaceListSummary) + '</div></div><div class="workspace-list-head"><span>Workspace</span><span>Health</span><span>Lifecycle</span><span>Actions</span></div><div class="workspace-list">' + workspaces.map(function(workspace) {
      return renderWorkspaceCard(account, workspace, accountAPIBasePath);
    }).join("") + "</div></div>" : '<div class="empty-state"><p>' + escapeHTML(account.can_manage ? "No hosted workspaces yet. Create one to get started." : "No hosted workspaces are attached yet. An owner or admin must create the first one.") + "</p></div>";
    return '<section class="account-content-panel account-content-panel-workspaces"><div class="account-stage-header"><div class="account-stage-header-row"><div><div class="account-panel-kicker">Workspaces</div><h3>Workspaces</h3><p>' + escapeHTML(sectionCopy) + "</p>" + renderSectionContextChips([
      String(workspaces.length) + " total",
      String(readyCount) + " ready",
      String(suspendedCount) + " suspended"
    ]) + '</div><div class="account-stage-header-actions">' + workspaceHeaderActions + '</div></div></div><div class="workspace-operations-shell workspace-operations-shell-idle" id="workspace-operations-shell-' + escapeAttr(account.id) + '"><div class="workspace-operations-main">' + workspaceHTML + '</div><div class="workspace-operations-detail" id="workspace-operations-detail-' + escapeAttr(account.id) + '" hidden>' + workspaceManagement + "</div></div></section>";
  }
  function renderAccountAccessSection(account) {
    var accessHeaderTitle = account.can_manage ? "Manage access" : "Review access";
    var accessHeaderCopy = account.can_manage ? "Review the hosted roster, then open one access job at a time." : "Review who already has access to this hosted account. An owner or admin must make changes.";
    var accessTaskStrip = account.can_manage ? '<div class="access-task-strip"><button type="button" class="access-task-button" id="access-task-invite-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="invite">Invite people</button><button type="button" class="access-task-button" id="access-task-change_role-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="change_role">Change roles</button><button type="button" class="access-task-button" id="access-task-remove-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="remove">Remove access</button></div>' : renderSectionContextChips(["View roster", "Owner or admin required"]);
    var accessRoleGuide = '<div class="access-policy-panel"><div class="access-panel-heading"><h4>' + (account.can_manage ? "Choose the smallest role" : "Role meanings") + "</h4><p>" + (account.can_manage ? "Match each person to the narrowest role that still lets them do the job they own." : "Use these role meanings to understand what each person on this roster can do.") + '</p></div><div class="access-policy-list"><div class="access-policy-row"><strong>Owner</strong><span>Full account, billing, and access control.</span></div><div class="access-policy-row"><strong>Admin</strong><span>Workspace control, billing, and roster management.</span></div><div class="access-policy-row"><strong>Tech</strong><span>Workspace control without billing or roster ownership.</span></div><div class="access-policy-row"><strong>Read-only</strong><span>Review access without control-plane changes.</span></div></div></div>';
    var accessInvitePanel = account.can_manage ? '<div class="access-invite-panel"><div class="access-panel-heading"><h4>Invite people</h4><p>Add one person with the minimum role they need on this account.</p></div><div class="access-invite"><div><label for="invite-email-' + escapeAttr(account.id) + '">Email</label><input type="email" id="invite-email-' + escapeAttr(account.id) + '" placeholder="user@example.com" autocomplete="off"></div><div><label for="invite-role-' + escapeAttr(account.id) + '">Role</label><select id="invite-role-' + escapeAttr(account.id) + '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div><button type="button" class="btn-primary btn-compact" data-action="invite-member" data-account-id="' + escapeAttr(account.id) + '">Invite</button></div></div>' : "";
    var accessChangeRolePanel = '<div class="access-job-note-panel"><div class="access-panel-heading"><h4>Change roles on the roster</h4><p>Use the role column in the roster to change one person at a time. Keep each person on the smallest role they need.</p></div></div>' + accessRoleGuide;
    var accessRemovePanel = '<div class="access-job-note-panel"><div class="access-panel-heading"><h4>Remove stale access</h4><p>Use removal only when this person should no longer be on this hosted account. Owners may still be protected when they are the last owner.</p></div><div class="access-remove-points"><div class="access-remove-point"><strong>Pick the exact person</strong><span>Use the roster to remove one account member at a time.</span></div><div class="access-remove-point"><strong>Keep current owners safe</strong><span>The last owner cannot be removed until another owner exists.</span></div></div></div>';
    return '<section class="account-content-panel account-content-panel-access"><section class="access-management-panel access-section access-section-shell" id="access-section-' + escapeAttr(account.id) + '" data-actor-role="' + escapeAttr(account.role) + '" data-can-manage="' + escapeAttr(account.can_manage ? "true" : "false") + '"><div class="access-management-header"><div><div class="account-panel-kicker">Access</div><h3>' + accessHeaderTitle + "</h3><p>" + accessHeaderCopy + "</p>" + accessTaskStrip + '</div></div><div class="access-management-stats" id="access-stats-' + escapeAttr(account.id) + '"></div><div class="access-shell access-shell-idle" id="access-shell-' + escapeAttr(account.id) + '"><div class="access-shell-main"><div class="access-roster-column"><div class="access-roster"><div class="access-panel-heading"><h4>People on this account</h4><p>' + (account.can_manage ? "Review the hosted roster here, then open the exact access job you need." : "Review the hosted roster here. An owner or admin must make changes.") + '</p></div><div class="access-roster-list" id="access-list-' + escapeAttr(account.id) + '"><div class="access-list-message">Loading\u2026</div></div></div></div></div>' + (account.can_manage ? '<div class="access-shell-detail" id="access-detail-' + escapeAttr(account.id) + '" hidden><div class="access-task-panel" id="access-task-panel-' + escapeAttr(account.id) + '" hidden><div class="access-task-header"><div><div class="account-panel-kicker">Access task</div><h4 id="access-task-title-' + escapeAttr(account.id) + '">Invite people</h4><p id="access-task-copy-' + escapeAttr(account.id) + '"></p></div><button type="button" class="btn-secondary btn-compact" data-action="clear-access-job" data-account-id="' + escapeAttr(account.id) + '">Close panel</button></div><div class="access-task-body" id="access-task-body-invite-' + escapeAttr(account.id) + '" hidden>' + accessInvitePanel + accessRoleGuide + '</div><div class="access-task-body" id="access-task-body-change_role-' + escapeAttr(account.id) + '" hidden>' + accessChangeRolePanel + '</div><div class="access-task-body" id="access-task-body-remove-' + escapeAttr(account.id) + '" hidden>' + accessRemovePanel + "</div></div></div>" : "") + "</div></section></section>";
  }
  function renderHostedBillingCards(accounts, showSelfHostedCommercial) {
    var hostedBillingAccounts = accounts.filter(function(account) {
      return account.has_billing;
    });
    if (!hostedBillingAccounts.length) {
      return '<div class="billing-task-card billing-task-card-muted"><div class="account-panel-kicker">Hosted billing</div><h3>No hosted billing attached</h3><p>' + escapeHTML(
        showSelfHostedCommercial ? "Use the self-hosted billing tools below only if you are managing a self-hosted purchase." : "Hosted invoices, payment methods, and subscription changes are not attached to this Pulse account right now."
      ) + "</p></div>";
    }
    return hostedBillingAccounts.map(function(account) {
      var actionHTML = account.can_manage ? '<button type="button" class="btn-primary btn-compact" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Open hosted billing</button>' : '<div class="billing-task-note">An owner or admin on this account needs to open hosted billing.</div>';
      return '<article class="billing-task-card"><div class="account-panel-kicker">Hosted billing</div><h3>' + escapeHTML(account.name) + '</h3><p>Invoices, payment methods, and hosted subscription changes for this account live here.</p><div class="billing-task-points"><div class="billing-task-point"><strong>Use when hosted billing is the job</strong><span>Keep workspace lifecycle work in Workspaces and access changes in Access.</span></div><div class="billing-task-point"><strong>Stay account-specific</strong><span>Open billing from the exact hosted account you want to change.</span></div></div><div class="billing-task-actions">' + actionHTML + "</div></article>";
    }).join("");
  }
  function renderBillingTaskPanel(title, copy, panelID, bodyHTML) {
    return '<section class="billing-panel" id="' + escapeAttr(panelID) + '" hidden><div class="billing-task-header"><div><div class="account-panel-kicker">Billing task</div><h3>' + escapeHTML(title) + "</h3><p>" + escapeHTML(copy) + '</p></div><button type="button" class="btn-secondary btn-compact" data-account-billing-action="clear-billing-panel">Close panel</button></div><div class="billing-task-body">' + bodyHTML + "</div></section>";
  }
  function renderSupportSection(context) {
    var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
    var hasHostedAccounts2 = accounts.length > 0;
    var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
    var supportEmail = context.bootstrap.support_email || "";
    var canManageHostedTasks = false;
    for (var i = 0; i < accounts.length; i += 1) {
      if (accounts[i].can_manage) {
        canManageHostedTasks = true;
        break;
      }
    }
    var hostedViewOnly = hasHostedAccounts2 && !canManageHostedTasks;
    var supportLead = hasHostedAccounts2 ? hostedViewOnly ? showSelfHostedCommercial ? "Use support only when the same Workspaces review, Access review, owner/admin, or Billing path has already stopped you." : "Use support only when the same Workspaces review, Access review, owner/admin, or hosted Billing path has already stopped you." : showSelfHostedCommercial ? "Use support only when the Workspaces, Access, or Billing path has already stopped you." : "Use support only when the Workspaces, Access, or hosted Billing path has already stopped you." : "Use support only when the Billing path has already stopped you.";
    var supportChips = hasHostedAccounts2 ? ["Escalation only", hostedViewOnly ? "Owner/admin first" : showSelfHostedCommercial ? "Bring context" : "Hosted only", supportEmail ? "Email" : "Support"] : ["Escalation only", "Billing only", supportEmail ? "Email" : "Support"];
    var routeCards = hasHostedAccounts2 ? '<div class="portal-support-route-card"><div class="account-panel-kicker">Hosted path</div><h3>' + (hostedViewOnly ? "Hosted review or owner/admin path failed" : "Workspace or access path failed") + "</h3><p>" + (hostedViewOnly ? "Go back to the hosted task first. Review the same workspace or roster here, then have an owner or admin run the blocked change before you escalate." : "Go back to the hosted task first. Escalate only when the same workspace or access path still cannot finish the job.") + '</p><div class="portal-support-points"><div class="portal-support-point"><strong>' + (hostedViewOnly ? "Review the same task" : "Start from the same task") + "</strong><span>" + (hostedViewOnly ? "Use Workspaces to confirm workspace state and Access to confirm the current roster before you escalate." : "Use Workspaces for lifecycle issues and Access for roster issues before you escalate.") + '</span></div><div class="portal-support-point"><strong>' + (hostedViewOnly ? "Name the blocked owner/admin action" : "Keep the hosted context intact") + "</strong><span>" + (hostedViewOnly ? "Include the account, workspace, and the lifecycle or access change that still needs an owner or admin." : "Include the account, workspace, and failed action so support inherits the same request.") + '</span></div></div><div class="portal-support-actions"><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">' + (hostedViewOnly ? "Review workspaces" : "Open workspaces") + '</button><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">' + (hostedViewOnly ? "Review access" : "Open access") + '</button><a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a></div></div><div class="portal-support-route-card"><div class="account-panel-kicker">Billing path</div><h3>' + (hostedViewOnly ? showSelfHostedCommercial ? "Billing or owner/admin path failed" : "Hosted billing or owner/admin path failed" : showSelfHostedCommercial ? "Billing path failed" : "Hosted billing path failed") + "</h3><p>" + (hostedViewOnly ? showSelfHostedCommercial ? "Use this route only after the relevant billing job has failed, or the affected hosted account still needs an owner or admin to finish hosted billing." : "Use this route only after the affected hosted account still needs an owner or admin to finish hosted billing and that path still cannot complete cleanly." : showSelfHostedCommercial ? "Use this route only after hosted billing or one self-hosted billing job has failed to complete cleanly." : "Use this route only after hosted billing has failed to complete cleanly.") + '</p><div class="portal-support-points"><div class="portal-support-point"><strong>Name the billing job</strong><span>' + (hostedViewOnly ? showSelfHostedCommercial ? "Say whether the failed path was hosted billing, licenses, refunds, or privacy, and whether hosted billing still needed an owner or admin." : "Say whether the failed path was hosted billing and whether the account still needed an owner or admin to open it." : showSelfHostedCommercial ? "Say whether the failed path was hosted billing, licenses, refunds, or privacy." : "Say whether the failed path was hosted billing.") + '</span></div><div class="portal-support-point"><strong>Keep the request intact</strong><span>' + (hostedViewOnly ? showSelfHostedCommercial ? "Bring the same account or billing email and the failed owner/admin or billing step instead of reopening the story." : "Bring the same hosted account and the failed billing or owner/admin step instead of reopening the story." : showSelfHostedCommercial ? "Bring the same account or billing email and the failed action instead of reopening the story." : "Bring the same hosted account and the failed billing action instead of reopening the story.") + '</span></div></div><div class="portal-support-actions"><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button><a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + "</a></div></div>" : '<div class="portal-support-route-card"><div class="account-panel-kicker">Billing path</div><h3>Self-hosted billing path failed</h3><p>Use this route only after a self-hosted billing, license, refund, or privacy job has failed to complete cleanly.</p><div class="portal-support-points"><div class="portal-support-point"><strong>Name the billing job</strong><span>Say whether the failed path was billing, licenses, refunds, or privacy.</span></div><div class="portal-support-point"><strong>Keep the purchase context intact</strong><span>Bring the same commercial email and the failed action instead of reopening the story.</span></div></div><div class="portal-support-actions"><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button><a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + "</a></div></div>";
    var runbookSteps = hasHostedAccounts2 ? '<div class="portal-support-runbook-step"><strong>1. Failed path</strong><span>' + (hostedViewOnly ? showSelfHostedCommercial ? "Say whether the blocked path was Workspaces review, Access review, owner/admin hosted change, hosted billing, licenses, refunds, or privacy." : "Say whether the blocked path was Workspaces review, Access review, owner/admin hosted change, or hosted billing." : showSelfHostedCommercial ? "Say whether the blocked path was Workspaces, Access, hosted billing, licenses, refunds, or privacy." : "Say whether the blocked path was Workspaces, Access, or hosted billing.") + '</span></div><div class="portal-support-runbook-step"><strong>2. Account or email</strong><span>' + (hostedViewOnly ? showSelfHostedCommercial ? "Include the hosted account and workspace for the blocked review or owner/admin path, or the commercial billing email for self-hosted work." : "Include the hosted account and workspace or hosted billing account that still needed owner/admin action." : showSelfHostedCommercial ? "Include the hosted account and workspace when relevant, or the commercial billing email for self-hosted work." : "Include the hosted account and workspace or hosted billing account that the failed path belongs to.") + '</span></div><div class="portal-support-runbook-step"><strong>3. Failed action</strong><span>Name the exact button, form, or billing step that failed and what happened next.</span></div>' : '<div class="portal-support-runbook-step"><strong>1. Billing job</strong><span>Say whether the blocked path was billing, licenses, refunds, or privacy.</span></div><div class="portal-support-runbook-step"><strong>2. Purchase email</strong><span>Include the commercial billing email used for the self-hosted purchase.</span></div><div class="portal-support-runbook-step"><strong>3. Failed action</strong><span>Name the exact button, form, or billing step that failed and what happened next.</span></div>';
    return '<section class="portal-support-panel"><div class="account-panel-kicker">Support</div><h2>Escalation only</h2><p>' + escapeHTML(supportLead) + "</p>" + renderSectionContextChips(supportChips) + '<div class="portal-support-layout"><div class="portal-support-route-grid">' + routeCards + '</div><div class="portal-support-runbook"><div class="account-panel-kicker">What to send</div><h3>Keep the escalation short</h3><p>Support should inherit the same request, not reconstruct it from scratch.</p><div class="portal-support-runbook-list">' + runbookSteps + "</div></div></div></section>";
  }
  function renderHeaderHTML(context) {
    if (context.bootstrap.authenticated) {
      return '<div class="header-account-chip"><span class="header-account-email">' + escapeHTML(context.bootstrap.email || "") + '</span><button class="logout-btn" id="logout-btn" type="button">Sign out</button></div>';
    }
    return '<a class="logout-btn link-button" href="' + escapeAttr(context.signupPath) + '">Create account</a>';
  }
  function renderAuthenticatedPortalHTML(context) {
    var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
    var hosted = hasHostedAccounts(accounts);
    var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
    var activeSection = context.activeSection || "overview";
    var hostedBillingCount = accounts.filter(function(account) {
      return account.has_billing;
    }).length;
    var billingNote = hosted ? showSelfHostedCommercial ? "Use hosted billing first when the request belongs to a hosted workspace account. Self-hosted licenses, refunds, and privacy stay separate underneath it." : "Use this billing surface only for hosted billing on your hosted workspace accounts." : "Use this billing surface only for self-hosted subscriptions, licenses, refunds, and privacy requests.";
    var selfHostedBillingEscalationCopy = hosted ? "Escalate with the same hosted billing action or self-hosted path and the exact failed step." : "Escalate with the same self-hosted billing path and the exact failed step.";
    var hostedContent = accounts.length ? accounts.map(function(account) {
      return '<section class="account-surface"><div class="account-surface-body">' + renderAccountWorkspaceSection(account, context.accountAPIBasePath) + renderAccountAccessSection(account) + "</div></section>";
    }).join("") : renderNoHostedWorkspacesSection() + renderNoHostedAccessSection();
    return '<div class="portal-shell" data-shell-section="' + activeSection + '"><div class="portal-shell-layout">' + renderShellNavigation(accounts, context.bootstrap.support_email || "", activeSection) + '<div class="portal-shell-main">' + (accounts.length === 1 ? renderAccountContextStrip(accounts[0]) : "") + '<section class="portal-content-panel portal-content-panel-overview">' + renderShellOverviewSection(context) + '<div id="accounts-root">' + hostedContent + '</div></section><section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section"><div class="billing-header"><div><div class="account-panel-kicker">Billing</div><h2>Billing</h2>' + renderSectionContextChips([
      hostedBillingCount > 0 ? "Hosted billing" : "No hosted billing",
      showSelfHostedCommercial ? "Self-hosted tools" : "Hosted only"
    ]) + '</div><div class="billing-note">' + billingNote + "</div></div>" + (hosted ? '<div class="billing-overview-grid">' + renderHostedBillingCards(accounts, showSelfHostedCommercial) + "</div>" : "") + (showSelfHostedCommercial ? '<div class="billing-shell billing-shell-idle"><div class="billing-shell-main"><div class="billing-shell-main-head"><div class="account-panel-kicker">Self-hosted billing</div><h3>Pick the self-hosted job</h3><p>Use self-hosted billing only for self-hosted purchases. Open one path at a time when hosted billing does not apply.</p></div><div class="billing-action-list">' + renderBillingActionRow("open-manage-billing", "Self-hosted billing", "Manage subscriptions", "Billing", "Open Stripe for self-hosted plan, invoice, and payment changes.", "manage-billing-panel", "manage-inline-email", ["Plan changes", "Invoices"]) + renderBillingActionRow("open-retrieve-billing", "Licenses", "Retrieve licenses", "Licenses", "Recover the latest active self-hosted license and invoice link.", "retrieve-billing-panel", "retrieve-inline-email", ["Latest active license", "Invoice lookup"]) + renderBillingActionRow("open-refund-billing", "Refunds", "Refund requests", "Refunds", "Request a self-serve refund when the purchase is still eligible.", "refund-billing-panel", "refund-inline-email", ["Eligibility check", "Revocation"]) + renderBillingActionRow("open-data-billing", "Privacy", "Data and privacy", "Privacy", "Request export or deletion for commercial account data.", "data-billing-panel", "data-export-email", ["Export", "Deletion"]) + '</div><div class="billing-inline-support"><div class="account-panel-kicker">Escalation only</div><h4>Use Support only after Billing fails</h4><p>' + selfHostedBillingEscalationCopy + '</p><div class="billing-inline-support-actions"><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button><a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + '</a></div></div></div><div class="billing-shell-detail" id="billing-detail-shell" hidden>' + renderBillingTaskPanel(
      "Manage subscriptions",
      "Open Stripe for self-hosted plan, invoice, and payment changes.",
      "manage-billing-panel",
      '<div id="manage-billing-root"></div>'
    ) + renderBillingTaskPanel(
      "Retrieve licenses",
      "Recover the latest active self-hosted license and invoice link.",
      "retrieve-billing-panel",
      '<div id="retrieve-billing-root"></div>'
    ) + renderBillingTaskPanel(
      "Refund requests",
      "Request a self-serve refund when the purchase is still eligible.",
      "refund-billing-panel",
      '<div id="refund-billing-root"></div>'
    ) + renderBillingTaskPanel(
      "Data and privacy",
      "Request export or deletion for commercial account data.",
      "data-billing-panel",
      '<div class="subsection"><div id="data-export-root"></div></div><div class="subsection"><div id="data-delete-root"></div></div><div class="helper-text">Payment-card data stays with Stripe. For Stripe deletion support, contact <a href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + "</a>.</div>"
    ) + "</div></div>" : "") + '</section><section class="portal-content-panel portal-content-panel-support">' + renderSupportSection(context) + "</section></div></div></div>";
  }
  function renderSignedOutPortalHTML(context) {
    var statusHTML = "";
    if (context.loginState.request.error) {
      statusHTML = '<div class="billing-status visible error">' + escapeHTML(context.loginState.request.error) + "</div>";
    } else if (context.loginState.success) {
      var successMessage = context.loginState.successMessage || "If that email is registered, a magic link is on the way.";
      statusHTML = '<div class="billing-status visible success">' + escapeHTML(successMessage) + `<br><br><strong>Don't see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.</div>`;
    }
    return '<section class="intro-card"><div class="account-panel-kicker">Pulse Account</div><h1>Sign in to Pulse Account</h1><p>Use one commercial email address to get into workspaces, MSP access, billing, license recovery, refunds, and privacy actions.</p></section><section class="billing-section billing-section-auth"><div class="billing-panel visible auth-panel"><h3>Sign in</h3><p>Enter the commercial email address for your Pulse account. I will send a magic link so you can open Pulse Account without managing a password.</p><div class="form-group"><label for="portal-login-email">Email address</label><input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' + escapeAttr(context.loginState.emailValue || "") + '" data-portal-input="login-email"></div><div class="form-actions"><button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' + (context.loginState.request.pending ? "Sending\u2026" : "Send magic link") + '</button><a class="btn-secondary link-button" href="' + escapeAttr(context.signupPath) + '">Create an account</a></div>' + statusHTML + "</div></section>";
  }

  // src/shell.ts
  function installShell(deps) {
    function revealActiveNavLink(activeLink) {
      if (!activeLink) return;
      var group = activeLink.closest(".portal-shell-nav-group");
      if (!group || group.scrollWidth <= group.clientWidth) return;
      if (typeof activeLink.scrollIntoView === "function") {
        activeLink.scrollIntoView({ block: "nearest", inline: "center" });
      }
    }
    function syncShellSection() {
      var root = document.querySelector(".portal-shell");
      var activeSection = deps.store.getShellState().activeSection;
      var activeLink = null;
      if (root) {
        root.setAttribute("data-shell-section", activeSection);
      }
      var links = document.querySelectorAll('[data-shell-action="activate-section"]');
      links.forEach(function(node) {
        var button = node;
        var isActive = button.getAttribute("data-shell-section") === activeSection;
        button.classList.toggle("active", isActive);
        if (isActive && button.classList.contains("portal-shell-nav-link")) {
          activeLink = button;
        }
      });
      revealActiveNavLink(activeLink);
    }
    function renderHeader() {
      var userInfo = document.getElementById("portal-user-info");
      if (!userInfo) return;
      var portalBootstrap = deps.store.getBootstrap();
      userInfo.innerHTML = renderHeaderHTML({
        bootstrap: portalBootstrap,
        loginState: deps.store.getLoginState(),
        signupPath: portalBootstrap.signup_path,
        accountAPIBasePath: portalBootstrap.account_api_base_path
      });
    }
    function renderPortalApp() {
      renderHeader();
      var root = document.getElementById("portal-app-root");
      if (!root) return;
      var portalBootstrap = deps.store.getBootstrap();
      var context = {
        bootstrap: portalBootstrap,
        loginState: deps.store.getLoginState(),
        signupPath: portalBootstrap.signup_path,
        accountAPIBasePath: portalBootstrap.account_api_base_path,
        activeSection: deps.store.getShellState().activeSection
      };
      root.innerHTML = portalBootstrap.authenticated ? renderAuthenticatedPortalHTML(context) : renderSignedOutPortalHTML(context);
      syncShellSection();
    }
    deps.store.subscribeBootstrap(function() {
      renderPortalApp();
    });
    deps.store.subscribeLogin(function() {
      renderPortalApp();
    });
    deps.store.subscribeShell(function() {
      syncShellSection();
    });
    document.addEventListener("click", function(event) {
      var target = event.target instanceof HTMLElement ? event.target.closest('[data-shell-action="activate-section"]') : null;
      if (!target) return;
      event.preventDefault();
      var section = target.getAttribute("data-shell-section") || "overview";
      deps.store.setActiveShellSection(section);
      if (deps.onSectionChange) {
        deps.onSectionChange(section);
      }
    });
    renderPortalApp();
  }

  // src/store.ts
  function createAnonymousBootstrap(bootstrapDefaults, overrides = {}) {
    return {
      authenticated: false,
      email: "",
      ...bootstrapDefaults,
      ...overrides,
      accounts: normalizeAccounts(overrides.accounts)
    };
  }
  function normalizeBootstrap(bootstrapDefaults, raw) {
    return createAnonymousBootstrap(bootstrapDefaults, raw || {});
  }
  function createPortalStore(bootstrapDefaults, initialBootstrap) {
    var bootstrapState = normalizeBootstrap(bootstrapDefaults, initialBootstrap);
    var accountState = createPortalAccountState();
    var loginState = createPortalLoginState();
    var shellState = createPortalShellState();
    var billingState = createPortalBillingState();
    syncLoginStateBootstrapEmail(loginState, bootstrapState.email || "");
    syncBillingStateBootstrapEmail(billingState, bootstrapState.email || "");
    var accountSubscribers = /* @__PURE__ */ new Set();
    var bootstrapSubscribers = /* @__PURE__ */ new Set();
    var loginSubscribers = /* @__PURE__ */ new Set();
    var shellSubscribers = /* @__PURE__ */ new Set();
    var billingSubscribers = /* @__PURE__ */ new Set();
    function notify(subscribers) {
      subscribers.forEach(function(listener) {
        listener();
      });
    }
    return {
      getBootstrap: function() {
        return bootstrapState;
      },
      getAccountState: function() {
        return accountState;
      },
      getLoginState: function() {
        return loginState;
      },
      getShellState: function() {
        return shellState;
      },
      getBillingState: function() {
        return billingState;
      },
      setBootstrap: function(nextBootstrap) {
        bootstrapState = normalizeBootstrap(bootstrapDefaults, nextBootstrap);
        syncLoginStateBootstrapEmail(loginState, bootstrapState.email || "");
        syncBillingStateBootstrapEmail(billingState, bootstrapState.email || "");
        notify(bootstrapSubscribers);
        return bootstrapState;
      },
      updateAccountState: function(mutator, options) {
        mutator(accountState);
        if (!options || options.notify !== false) {
          notify(accountSubscribers);
        }
        return accountState;
      },
      updateLoginState: function(mutator, options) {
        mutator(loginState);
        if (!options || options.notify !== false) {
          notify(loginSubscribers);
        }
        return loginState;
      },
      setActiveShellSection: function(section) {
        shellState.activeSection = section;
        notify(shellSubscribers);
        return shellState;
      },
      updateBillingState: function(mutator, options) {
        mutator(billingState);
        if (!options || options.notify !== false) {
          notify(billingSubscribers);
        }
        return billingState;
      },
      subscribeBootstrap: function(listener) {
        bootstrapSubscribers.add(listener);
        return function() {
          bootstrapSubscribers.delete(listener);
        };
      },
      subscribeAccount: function(listener) {
        accountSubscribers.add(listener);
        return function() {
          accountSubscribers.delete(listener);
        };
      },
      subscribeLogin: function(listener) {
        loginSubscribers.add(listener);
        return function() {
          loginSubscribers.delete(listener);
        };
      },
      subscribeShell: function(listener) {
        shellSubscribers.add(listener);
        return function() {
          shellSubscribers.delete(listener);
        };
      },
      subscribeBilling: function(listener) {
        billingSubscribers.add(listener);
        return function() {
          billingSubscribers.delete(listener);
        };
      }
    };
  }
  function normalizeAccounts(accounts) {
    return Array.isArray(accounts) ? accounts : [];
  }

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
  function normalizeHandoffEmail(value) {
    return String(value || "").trim();
  }
  function normalizeHandoffBillingPanel(value) {
    switch (String(value || "").trim()) {
      case "manage":
        return "manage-billing-panel";
      case "retrieve":
        return "retrieve-billing-panel";
      case "refund":
        return "refund-billing-panel";
      case "data":
        return "data-billing-panel";
      default:
        return "";
    }
  }
  function readPortalRuntimeHandoff(locationHref = window.location.href) {
    try {
      var params = new URL(locationHref).searchParams;
      return {
        email: normalizeHandoffEmail(params.get("email")),
        openBillingPanelID: normalizeHandoffBillingPanel(params.get("service"))
      };
    } catch {
      return {
        email: "",
        openBillingPanelID: ""
      };
    }
  }
  function createBootstrapDefaults(embeddedBootstrap) {
    return {
      has_self_hosted_commercial: embeddedBootstrap.has_self_hosted_commercial === true,
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
  }
  function createPortalRuntime(embeddedBootstrap = readEmbeddedBootstrap(), handoff = readPortalRuntimeHandoff()) {
    var bootstrapDefaults = createBootstrapDefaults(embeddedBootstrap);
    var store = createPortalStore(bootstrapDefaults, embeddedBootstrap);
    if (handoff.email) {
      store.updateLoginState(function(loginState) {
        loginState.emailValue = handoff.email;
      }, { notify: false });
      store.updateBillingState(function(billingState) {
        billingState.flows.manage.emailValue = handoff.email;
        billingState.flows.retrieve.emailValue = handoff.email;
        billingState.flows.export.emailValue = handoff.email;
        billingState.flows.delete.emailValue = handoff.email;
        billingState.refund.emailValue = handoff.email;
      }, { notify: false });
    }
    if (handoff.openBillingPanelID) {
      store.updateBillingState(function(billingState) {
        billingState.openBillingPanelID = handoff.openBillingPanelID;
      }, { notify: false });
    }
    return {
      bootstrapDefaults,
      embeddedBootstrap,
      handoff,
      store
    };
  }

  // src/app.ts
  function installPortalApp(deps) {
    var api = createPortalAPI({
      getBootstrap: function() {
        return deps.store.getBootstrap();
      }
    });
    function applyBootstrap(data) {
      return deps.store.setBootstrap(data || createAnonymousBootstrap(deps.bootstrapDefaults));
    }
    async function refreshBootstrap() {
      var bootstrap = deps.store.getBootstrap();
      if (!bootstrap.bootstrap_path) return false;
      try {
        var data = await api.fetchBootstrap();
        applyBootstrap(data);
        return true;
      } catch (error) {
        if (error instanceof PortalAPIError && error.status === 401) {
          applyBootstrap(createAnonymousBootstrap(deps.bootstrapDefaults));
          return true;
        }
      }
      return false;
    }
    function showToast(message, isError = false) {
      var toast = document.getElementById("toast");
      if (!toast) return;
      toast.textContent = message;
      toast.className = "toast visible" + (isError ? " error" : "");
      clearTimeout(toast._timer);
      toast._timer = setTimeout(function() {
        toast.className = "toast";
      }, 4e3);
    }
    var accountRuntime = installAccountRuntime({
      api,
      store: deps.store,
      refreshBootstrap,
      showToast
    });
    installShell({
      store: deps.store,
      onSectionChange: function(section) {
        if (section === "access") {
          var accounts = deps.store.getBootstrap().accounts || [];
          for (var i = 0; i < accounts.length; i += 1) {
            accountRuntime.ensureAccessVisible(accounts[i].id);
          }
        }
      }
    });
    installBillingRuntime({
      api,
      store: deps.store
    });
    installAuthController({
      api,
      store: deps.store
    });
    installAccountController({
      runtime: accountRuntime,
      setShellSection: function(section) {
        deps.store.setActiveShellSection(section);
      }
    });
    var startupRefresh = deps.store.getBootstrap().authenticated ? refreshBootstrap() : null;
    return {
      applyBootstrap,
      refreshBootstrap,
      showToast,
      startupRefresh
    };
  }
  function startPortalApp() {
    var runtime = createPortalRuntime();
    return installPortalApp({
      bootstrapDefaults: runtime.bootstrapDefaults,
      store: runtime.store
    });
  }

  // src/index.ts
  startPortalApp();
})();
