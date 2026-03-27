(() => {
  // src/account_view.ts
  function normalizedTeamRole(role) {
    if (role === "member") return "read_only";
    return role || "read_only";
  }
  function roleLabel(role) {
    switch (normalizedTeamRole(role)) {
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
    switch (normalizedTeamRole(role)) {
      case "owner":
        return "Full account control, including billing, team access, and hosted operations.";
      case "admin":
        return "Can manage hosted operations and billing for this account.";
      case "tech":
        return "Can operate hosted workspaces without billing ownership.";
      case "read_only":
        return "Can review hosted state without making control-plane changes.";
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
    if (!panel) return;
    var empty = getElement("workspace-management-empty-" + account.id);
    var content = getElement("workspace-management-content-" + account.id);
    var title = getElement("workspace-management-title-" + account.id);
    var meta = getElement("workspace-management-meta-" + account.id);
    var summary = getElement("workspace-management-summary-" + account.id);
    var actionButton = getElement("workspace-management-action-" + account.id);
    var closeButton = getElement("workspace-management-close-" + account.id);
    if (!empty || !content || !title || !meta || !summary || !actionButton || !closeButton) return;
    var workspace = entry.selectedWorkspaceID ? findWorkspace(account, entry.selectedWorkspaceID) : null;
    var hasSelection = !!workspace;
    panel.classList.add("visible");
    empty.hidden = hasSelection;
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
    actionButton.textContent = workspaceActionLabel(workspace);
    actionButton.disabled = entry.manageWorkspace.pending;
    actionButton.setAttribute("data-workspace-id", workspace.id);
    actionButton.setAttribute("data-workspace-name", workspace.display_name);
    actionButton.setAttribute("data-workspace-action", workspace.state === "active" ? "suspend" : "delete");
    closeButton.disabled = entry.manageWorkspace.pending;
  }
  function setContainerMessage(container, msg, isError) {
    container.textContent = "";
    var message = document.createElement("div");
    message.className = "team-list-message" + (isError ? " error" : "");
    message.textContent = msg;
    container.appendChild(message);
  }
  function countMembersByRole(members, role) {
    var count = 0;
    for (var i = 0; i < members.length; i += 1) {
      if (normalizedTeamRole(members[i].role) === role) count += 1;
    }
    return count;
  }
  function renderTeamStats(accountID, entry) {
    var stats = getElement("team-stats-" + accountID);
    if (!stats) return;
    if (!entry.teamVisible) {
      stats.innerHTML = "";
      return;
    }
    if (entry.teamQuery.status === "loading") {
      stats.innerHTML = '<div class="team-stat-card"><span class="team-stat-label">Roster</span><span class="team-stat-value">Loading\u2026</span></div>';
      return;
    }
    if (entry.teamQuery.status === "error") {
      stats.innerHTML = '<div class="team-stat-card"><span class="team-stat-label">Roster</span><span class="team-stat-value team-stat-error">Needs attention</span></div>';
      return;
    }
    var members = entry.teamQuery.data;
    stats.innerHTML = '<div class="team-stat-card"><span class="team-stat-label">Members</span><span class="team-stat-value">' + String(members.length) + '</span></div><div class="team-stat-card"><span class="team-stat-label">Owners</span><span class="team-stat-value">' + String(countMembersByRole(members, "owner")) + '</span></div><div class="team-stat-card"><span class="team-stat-label">Admins</span><span class="team-stat-value">' + String(countMembersByRole(members, "admin")) + '</span></div><div class="team-stat-card"><span class="team-stat-label">Operators</span><span class="team-stat-value">' + String(countMembersByRole(members, "tech") + countMembersByRole(members, "read_only")) + "</span></div>";
  }
  function renderTeamRoleControl(accountID, member, isOwner) {
    var currentRole = normalizedTeamRole(member.role);
    if (currentRole === "owner" && !isOwner) {
      var locked = document.createElement("span");
      locked.className = "team-role-badge";
      locked.textContent = roleLabel(currentRole);
      return locked;
    }
    var sel = document.createElement("select");
    sel.className = "team-role-select";
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
    return sel;
  }
  function renderTeamMemberAction(accountID, member, isOwner) {
    if (normalizedTeamRole(member.role) === "owner" && !isOwner) {
      return null;
    }
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "btn-remove";
    btn.textContent = "Remove";
    btn.setAttribute("data-action", "remove-member");
    btn.setAttribute("data-account-id", accountID);
    btn.setAttribute("data-user-id", member.user_id);
    btn.setAttribute("data-member-email", member.email);
    return btn;
  }
  function renderTeamMemberRow(accountID, member, isOwner) {
    var row = document.createElement("div");
    row.className = "team-member-row";
    var identity = document.createElement("div");
    identity.className = "team-member-identity";
    var topline = document.createElement("div");
    topline.className = "team-member-topline";
    var email = document.createElement("div");
    email.className = "team-member-email";
    email.textContent = member.email;
    topline.appendChild(email);
    var roleBadge = document.createElement("span");
    roleBadge.className = "team-inline-role-badge";
    roleBadge.textContent = roleLabel(member.role);
    topline.appendChild(roleBadge);
    identity.appendChild(topline);
    var caption = document.createElement("div");
    caption.className = "team-member-caption";
    caption.textContent = roleCapabilityCopy(member.role);
    identity.appendChild(caption);
    var controls = document.createElement("div");
    controls.className = "team-member-controls";
    controls.appendChild(renderTeamRoleControl(accountID, member, isOwner));
    var action = renderTeamMemberAction(accountID, member, isOwner);
    if (action) controls.appendChild(action);
    row.appendChild(identity);
    row.appendChild(controls);
    return row;
  }
  function ensureRosterHead(container) {
    var existing = container.querySelector(".team-roster-head");
    if (existing) return;
    var head = document.createElement("div");
    head.className = "team-roster-head";
    head.innerHTML = "<span>Operator</span><span>Controls</span>";
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
  function renderTeamSection(accountID, entry) {
    var section = getElement("team-section-" + accountID);
    var roster = getElement("team-list-" + accountID);
    if (!section || !roster) return;
    var actorRole = section.getAttribute("data-actor-role") || "";
    var isOwner = actorRole === "owner";
    section.classList.toggle("visible", entry.teamVisible);
    renderTeamStats(accountID, entry);
    if (!entry.teamVisible) {
      return;
    }
    if (entry.teamQuery.status === "loading") {
      setContainerMessage(roster, "Loading\u2026", false);
      return;
    }
    if (entry.teamQuery.status === "error") {
      setContainerMessage(roster, entry.teamQuery.error, true);
      return;
    }
    if (!entry.teamQuery.data.length) {
      setContainerMessage(roster, "No team members.", false);
      return;
    }
    roster.textContent = "";
    ensureRosterHead(roster);
    for (var i = 0; i < entry.teamQuery.data.length; i += 1) {
      var member = entry.teamQuery.data[i];
      roster.appendChild(renderTeamMemberRow(accountID, member, isOwner));
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
      renderTeamSection(accountID, entry);
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
        case "toggle-team":
          event.preventDefault();
          deps.setShellSection("team");
          deps.runtime.ensureTeamVisible(accountID);
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
        return request(accountURL(accountID, "/members"), {}, "Failed to load team.");
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
        teamVisible: false,
        teamQuery: createQueryState([])
      };
    }
    return accountState.byAccountID[accountID];
  }
  function createPortalServiceState() {
    return {
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
  function syncServiceStateBootstrapEmail(serviceState, email) {
    if (!serviceState.flows.manage.emailValue) serviceState.flows.manage.emailValue = email || "";
    if (!serviceState.flows.retrieve.emailValue) serviceState.flows.retrieve.emailValue = email || "";
    if (!serviceState.flows.export.emailValue) serviceState.flows.export.emailValue = email || "";
    if (!serviceState.flows.delete.emailValue) serviceState.flows.delete.emailValue = email || "";
    if (!serviceState.refund.emailValue) serviceState.refund.emailValue = email || "";
  }
  function setFlowStatus(serviceState, flowID, message, isError) {
    serviceState.flows[flowID].status = {
      visible: true,
      message,
      error: !!isError
    };
  }
  function clearFlowStatus(serviceState, flowID) {
    serviceState.flows[flowID].status = emptyStatus();
  }
  function setRefundStatus(serviceState, message, isError) {
    serviceState.refund.status = {
      visible: true,
      message,
      error: !!isError
    };
  }
  function toggleServicePanelState(serviceState, panelID) {
    serviceState.openPanelID = serviceState.openPanelID === panelID ? "" : panelID;
  }
  function resetVerificationFlowState(serviceState, flowID) {
    var previous = serviceState.flows[flowID];
    serviceState.flows[flowID] = newVerificationFlowState();
    serviceState.flows[flowID].emailValue = previous.emailValue;
  }
  function updateServiceInputValue(serviceState, inputKind, value) {
    switch (inputKind) {
      case "manage-email":
        serviceState.flows.manage.emailValue = value;
        return;
      case "manage-code":
        serviceState.flows.manage.codeValue = value;
        return;
      case "retrieve-email":
        serviceState.flows.retrieve.emailValue = value;
        return;
      case "retrieve-code":
        serviceState.flows.retrieve.codeValue = value;
        return;
      case "refund-email":
        serviceState.refund.emailValue = value;
        return;
      case "refund-token":
        serviceState.refund.tokenValue = value;
        return;
      case "data-export-email":
        serviceState.flows.export.emailValue = value;
        return;
      case "data-export-code":
        serviceState.flows.export.codeValue = value;
        return;
      case "data-delete-email":
        serviceState.flows.delete.emailValue = value;
        return;
      case "data-delete-code":
        serviceState.flows.delete.codeValue = value;
        return;
      default:
        return;
    }
  }
  function updateDeleteConfirmation(serviceState, checked) {
    serviceState.flows.delete.checkboxChecked = checked;
  }

  // src/account_runtime.ts
  function installAccountRuntime(deps) {
    var getPortalPath = function() {
      return deps.store.getBootstrap().portal_path;
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
    var loadTeam = async function(accountID) {
      var section = getElement("team-section-" + accountID);
      if (!section) return;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamVisible = true;
        beginQueryState(entry.teamQuery, []);
      });
      try {
        var members = await deps.api.listMembers(accountID);
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          resolveQueryState(entry.teamQuery, Array.isArray(members) ? members : []);
        });
      } catch (error) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          failQueryState(entry.teamQuery, [], error instanceof Error ? error.message : "Network error.");
        });
      }
    };
    var refreshAccountTeamSection = async function(accountID) {
      if (!await refreshOrRedirect()) {
        return false;
      }
      var section = getElement("team-section-" + accountID);
      if (!section) {
        return true;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamVisible = true;
      });
      await loadTeam(accountID);
      return true;
    };
    var toggleAddWorkspace = function(accountID) {
      var shouldFocus = false;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = !entry.addWorkspaceOpen;
        if (entry.addWorkspaceOpen) {
          entry.teamVisible = false;
          entry.selectedWorkspaceID = "";
        }
        shouldFocus = entry.addWorkspaceOpen;
      });
      if (shouldFocus) {
        focusElement("ws-name-" + accountID);
      }
    };
    var selectWorkspace = function(accountID, workspaceID) {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.selectedWorkspaceID = entry.selectedWorkspaceID === workspaceID ? "" : workspaceID;
        if (entry.selectedWorkspaceID) {
          entry.teamVisible = false;
          entry.addWorkspaceOpen = false;
        }
      });
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
    var toggleTeam = function(accountID) {
      var nextVisible = false;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamVisible = !entry.teamVisible;
        if (entry.teamVisible) {
          entry.selectedWorkspaceID = "";
          entry.addWorkspaceOpen = false;
        }
        nextVisible = entry.teamVisible;
      });
      if (nextVisible) {
        void loadTeam(accountID);
      }
    };
    var ensureTeamVisible = function(accountID) {
      var shouldLoad = false;
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        if (!entry.teamVisible) {
          entry.teamVisible = true;
          entry.selectedWorkspaceID = "";
          entry.addWorkspaceOpen = false;
        }
        shouldLoad = entry.teamQuery.status === "idle" || entry.teamQuery.status === "error";
      });
      if (shouldLoad) {
        void loadTeam(accountID);
      }
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
        if (!await refreshAccountTeamSection(accountID)) {
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
        if (!await refreshAccountTeamSection(accountID)) {
          return;
        }
        deps.showToast("Role updated.");
      } catch (error) {
        if (error instanceof PortalAPIError && error.status === 409) {
          deps.showToast("Cannot demote last owner.", true);
          await loadTeam(accountID);
          return;
        }
        deps.showToast(error instanceof Error ? error.message : "Failed to update role.", true);
        await loadTeam(accountID);
      }
    };
    var removeMember = async function(accountID, userID, email) {
      if (!window.confirm("Remove " + email + " from this account?")) return;
      try {
        await deps.api.removeMember(accountID, userID);
        if (!await refreshAccountTeamSection(accountID)) {
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
      toggleTeam,
      ensureTeamVisible,
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

  // src/services_view.ts
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
    if (el) el.focus();
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
  function renderStatus(id, status) {
    var el = getElement3(id);
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
    if (!id || !label) return;
    var button = getElement3(id);
    if (!button) return;
    button.disabled = disabled;
    button.textContent = label;
  }
  function renderOpenPanels(openPanelID) {
    var panels = ["manage-service-panel", "retrieve-service-panel", "refund-service-panel", "data-service-panel"];
    var emptyPanel = getElement3("service-panel-empty");
    if (emptyPanel) {
      emptyPanel.classList.toggle("visible", !openPanelID);
    }
    for (var i = 0; i < panels.length; i++) {
      var panel = getElement3(panels[i]);
      if (!panel) continue;
      panel.classList.toggle("visible", panels[i] === openPanelID);
    }
    var serviceButtons = document.querySelectorAll('[data-account-service-action="open-service-panel"]');
    for (var j = 0; j < serviceButtons.length; j += 1) {
      var button = serviceButtons[j];
      var row = button.closest(".service-action-row");
      if (!row) continue;
      row.classList.toggle("active", button.getAttribute("data-account-service-panel") === openPanelID);
    }
  }
  function renderRefundPanel(refundState, bootstrap) {
    var root = getElement3("refund-service-root");
    if (!root) return;
    var refundSupportURL = (bootstrap.public_site_url || "") + "/refund.html?email=" + encodeURIComponent(refundState.emailValue || "");
    root.innerHTML = '<h3>Refund requests</h3><p>Process an eligible self-serve refund for a self-hosted purchase. This revokes the associated license immediately.</p><div class="warning"><strong>Warning:</strong> completing a refund immediately revokes the affected license. This should only be used when the refund window and commercial contract allow it.</div><div class="form-group"><label for="refund-inline-email">Email address</label><input type="email" id="refund-inline-email" value="' + escapeAttribute(refundState.emailValue || "") + '" autocomplete="email" data-account-service-input="refund-email"></div><div class="form-group"><label for="refund-inline-token">License key</label><input type="text" id="refund-inline-token" value="' + escapeAttribute(refundState.tokenValue || "") + '" placeholder="pulse_xxxxx" data-account-service-input="refund-token"></div><div class="form-actions"><button class="btn-danger" type="button" id="refund-inline-submit" data-account-service-action="refund-inline-submit">Process Refund</button></div><div class="helper-text">If this purchase is not eligible for self-serve refund, use the public support path instead: <a href="' + escapeAttribute(refundSupportURL) + '">open refund support page</a>.</div><div class="service-status" id="refund-inline-status"></div>';
  }
  function renderManagePanel(flowState) {
    var root = getElement3("manage-service-root");
    if (!root) return;
    root.innerHTML = '<h3>Manage subscriptions</h3><p>Request a verification code for the commercial email, then open the Stripe customer portal for billing changes, invoices, and subscription actions.</p><div id="manage-inline-step1"><div class="form-group"><label for="manage-inline-email">Email address</label><input type="email" id="manage-inline-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="manage-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="manage-inline-request" data-account-service-action="manage-inline-request">Send Verification Code</button></div></div><div id="manage-inline-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="manage-inline-code">Verification code</label><input type="text" id="manage-inline-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="manage-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="manage-inline-confirm" data-account-service-action="manage-inline-confirm">Open Customer Portal</button></div><div class="helper-text">Need a new code? <a href="#" id="manage-inline-resend" data-account-service-action="manage-inline-resend">Send again</a></div></div><div class="service-status" id="manage-inline-status"></div>';
  }
  function renderRetrievePanel(flowState) {
    var root = getElement3("retrieve-service-root");
    if (!root) return;
    var result = flowState.result;
    var invoiceURL = result && result.invoice_url ? result.invoice_url : "#";
    root.innerHTML = '<h3>Retrieve licenses</h3><p>Request a verification code for the commercial email, then reveal the current active self-hosted license without leaving Pulse Account.</p><div id="retrieve-inline-step1"><div class="form-group"><label for="retrieve-inline-email">Email address</label><input type="email" id="retrieve-inline-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="retrieve-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="retrieve-inline-request" data-account-service-action="retrieve-inline-request">Send Verification Code</button></div></div><div id="retrieve-inline-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="retrieve-inline-code">Verification code</label><input type="text" id="retrieve-inline-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="retrieve-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="retrieve-inline-confirm" data-account-service-action="retrieve-inline-confirm">Show License</button><button class="btn-secondary" type="button" id="retrieve-inline-copy" data-account-service-action="retrieve-inline-copy"' + (result ? "" : " hidden") + '>Copy License Key</button><a class="btn-secondary" id="retrieve-inline-invoice" href="' + escapeAttribute(invoiceURL) + '" target="_blank" rel="noopener"' + (result && result.invoice_url ? "" : " hidden") + '>View Invoice</a></div><div class="helper-text">Use the latest active self-hosted license for this commercial email.</div></div><div class="service-status" id="retrieve-inline-status"></div><div id="retrieve-inline-result" class="service-result"' + (result ? "" : " hidden") + '><label for="retrieve-inline-token">License key</label><textarea id="retrieve-inline-token" readonly>' + escapeText(result ? result.token : "") + '</textarea><div class="result-grid"><div><div class="result-meta-label">Plan</div><div class="result-meta-value" id="retrieve-inline-tier">' + escapeText(result ? result.tier : "") + '</div></div><div><div class="result-meta-label">Issued</div><div class="result-meta-value" id="retrieve-inline-issued">' + escapeText(result ? new Date(result.issued_at).toLocaleString() : "") + '</div></div><div><div class="result-meta-label">Expires</div><div class="result-meta-value" id="retrieve-inline-expires">' + escapeText(result ? result.expires_at ? new Date(result.expires_at).toLocaleString() : "Does not expire" : "") + '</div></div><div><div class="result-meta-label">Purchase Email</div><div class="result-meta-value" id="retrieve-inline-email-value">' + escapeText(result ? result.email : "") + "</div></div></div></div>";
  }
  function renderExportPanel(flowState) {
    var root = getElement3("data-export-root");
    if (!root) return;
    root.innerHTML = '<h4>Export My Data</h4><div id="data-export-step1"><div class="form-group"><label for="data-export-email">Email address</label><input type="email" id="data-export-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="data-export-email"></div><div class="form-actions"><button class="btn-primary" type="button" id="data-export-request" data-account-service-action="data-export-request">Send Verification Code</button></div></div><div id="data-export-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="data-export-code">Verification code</label><input type="text" id="data-export-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="data-export-code"></div><div class="form-actions"><button class="btn-primary" type="button" id="data-export-confirm" data-account-service-action="data-export-confirm">Export My Data</button></div><div class="helper-text">Need a new code? <a href="#" id="data-export-resend" data-account-service-action="data-export-resend">Send again</a></div></div><div class="service-status" id="data-export-status"></div><div id="data-export-result" class="service-result"' + (flowState.result ? "" : " hidden") + '><label for="data-export-payload">Export payload</label><textarea id="data-export-payload" readonly>' + escapeText(flowState.result ? JSON.stringify(flowState.result, null, 2) : "") + "</textarea></div>";
  }
  function renderExportResult(result) {
    setVisible("data-export-result", !!result);
    setValue("data-export-payload", result ? JSON.stringify(result, null, 2) : "");
  }
  function renderDeletePanel(flowState) {
    var root = getElement3("data-delete-root");
    if (!root) return;
    root.innerHTML = '<h4>Delete My Data</h4><div class="warning"><strong>Warning:</strong> deleting commercial data also revokes license records and cannot be undone.</div><div id="data-delete-step1"><div class="form-group"><label for="data-delete-email">Email address</label><input type="email" id="data-delete-email" value="' + escapeAttribute(flowState.emailValue || "") + '" autocomplete="email" data-account-service-input="data-delete-email"></div><div class="form-actions"><button class="btn-danger" type="button" id="data-delete-request" data-account-service-action="data-delete-request">Send Verification Code</button></div></div><div id="data-delete-step2"' + (flowState.step2Visible ? "" : " hidden") + '><div class="form-group"><label for="data-delete-code">Verification code</label><input type="text" id="data-delete-code" value="' + escapeAttribute(flowState.codeValue || "") + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="data-delete-code"></div><div class="checkbox-row"><input type="checkbox" id="data-delete-confirm-check"' + (flowState.checkboxChecked ? " checked" : "") + '><span>I understand this permanently deletes my commercial data and revokes associated licenses.</span></div><div class="form-actions"><button class="btn-danger" type="button" id="data-delete-confirm" data-account-service-action="data-delete-confirm">Delete My Data</button></div><div class="helper-text">Need a new code? <a href="#" id="data-delete-resend" data-account-service-action="data-delete-resend">Send again</a></div></div><div class="service-status" id="data-delete-status"></div>';
  }

  // src/services_controller.ts
  function installServicesController(deps) {
    document.addEventListener("click", function(event) {
      var target = asHTMLElement3(event.target)?.closest("[data-account-service-action]");
      if (!target) return;
      var action = target.getAttribute("data-account-service-action") || "";
      var panelID = target.getAttribute("data-account-service-panel") || "";
      var focusID = target.getAttribute("data-account-service-focus") || "";
      switch (action) {
        case "open-service-panel":
          event.preventDefault();
          deps.setShellSection("services");
          deps.toggleServicePanel(panelID);
          deps.focusElement(focusID);
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
      var inputKind = target.getAttribute("data-account-service-input") || "";
      if (!inputKind) return;
      deps.updateInputValue(inputKind, target.value);
    });
    document.addEventListener("change", function(event) {
      var target = asHTMLElement3(event.target);
      if (!target || target.id !== "data-delete-confirm-check") return;
      deps.updateDeleteConfirmation(!!target.checked);
    });
  }

  // src/services.ts
  function installServicesRuntime(deps) {
    var api = deps.api;
    var store = deps.store;
    store.updateServiceState(function(serviceState) {
      if (!serviceState.flows) {
        var nextState = createPortalServiceState();
        serviceState.openPanelID = nextState.openPanelID;
        serviceState.flows = nextState.flows;
        serviceState.refund = nextState.refund;
      }
    }, { notify: false });
    function getServiceState() {
      return store.getServiceState();
    }
    function updateServiceState(mutator, notify = true) {
      return store.updateServiceState(mutator, { notify });
    }
    function toggleServicePanel(panelID) {
      updateServiceState(function(serviceState) {
        toggleServicePanelState(serviceState, panelID);
      });
    }
    function renderFlow(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var flowState = getServiceState().flows[flowID];
      if (flow.renderPanel) {
        flow.renderPanel(flowState);
      }
      renderButton(flow.requestButtonID, flowState.request.pending, flowState.request.pending ? flow.requestPendingLabel : flow.requestLabel);
      renderButton(flow.confirmButtonID, flowState.confirm.pending, flowState.confirm.pending ? flow.confirmPendingLabel : flow.confirmLabel);
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
      var refundState = getServiceState().refund;
      renderRefundPanel(refundState, store.getBootstrap());
      renderButton("refund-inline-submit", refundState.submit.pending, refundState.submit.pending ? "Processing..." : "Process Refund");
      renderStatus("refund-inline-status", refundState.status);
    }
    function resetVerificationFlow(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      updateServiceState(function(serviceState) {
        resetVerificationFlowState(serviceState, flowID);
      }, false);
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
          return getServiceState().flows.manage.emailValue;
        },
        readCodeValue: function() {
          return getServiceState().flows.manage.codeValue;
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
          return getServiceState().flows.retrieve.emailValue;
        },
        readCodeValue: function() {
          return getServiceState().flows.retrieve.codeValue;
        },
        onRequestStart: function() {
          updateServiceState(function(serviceState) {
            serviceState.flows.retrieve.result = null;
          }, false);
        },
        applyConfirmSuccessState: function(serviceState, data) {
          serviceState.flows.retrieve.result = data.license;
          serviceState.flows.retrieve.codeValue = "";
          setFlowStatus(serviceState, "retrieve", "License retrieved successfully.", false);
        },
        renderPanel: renderRetrievePanel
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
          return getServiceState().flows.export.emailValue;
        },
        readCodeValue: function() {
          return getServiceState().flows.export.codeValue;
        },
        onRequestStart: function() {
          updateServiceState(function(serviceState) {
            serviceState.flows.export.result = null;
          }, false);
        },
        applyConfirmSuccessState: function(serviceState, data) {
          var emailValue = serviceState.flows.export.emailValue;
          resetVerificationFlowState(serviceState, "export");
          serviceState.flows.export.emailValue = emailValue;
          serviceState.flows.export.result = data;
          setFlowStatus(serviceState, "export", "Data export retrieved successfully.", false);
        },
        renderPanel: renderExportPanel,
        renderResult: renderExportResult
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
          return getServiceState().flows.delete.emailValue;
        },
        readCodeValue: function() {
          return getServiceState().flows.delete.codeValue;
        },
        beforeConfirm: function() {
          if (!getElement3("data-delete-confirm-check")?.checked) {
            updateServiceState(function(serviceState) {
              setFlowStatus(serviceState, "delete", "You must confirm that you understand this action is permanent.", true);
            });
            return false;
          }
          return true;
        },
        applyConfirmSuccessState: function(serviceState, data) {
          var emailValue = serviceState.flows.delete.emailValue;
          resetVerificationFlowState(serviceState, "delete");
          serviceState.flows.delete.emailValue = emailValue;
          setFlowStatus(
            serviceState,
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
      updateServiceState(function(serviceState) {
        beginMutationState(serviceState.flows[flowID].request);
        clearFlowStatus(serviceState, flowID);
      });
      try {
        await api.postCommercialJSON(flow.requestPath, { email });
        updateServiceState(function(serviceState) {
          serviceState.flows[flowID].pendingEmail = email;
          serviceState.flows[flowID].step2Visible = !!flow.step2ID;
          succeedMutationState(serviceState.flows[flowID].request);
          setFlowStatus(serviceState, flowID, flow.requestSuccessMessage, false);
        });
      } catch (err) {
        var message = err instanceof Error ? err.message : flow.requestErrorMessage;
        updateServiceState(function(serviceState) {
          failMutationState(serviceState.flows[flowID].request, message);
          setFlowStatus(serviceState, flowID, message, true);
        });
      }
    }
    async function resendVerificationCode(flowID, event) {
      if (event) event.preventDefault();
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var email = getServiceState().flows[flowID].pendingEmail;
      if (!email) return;
      try {
        await api.postCommercialJSON(flow.requestPath, { email });
        updateServiceState(function(serviceState) {
          setFlowStatus(serviceState, flowID, flow.resendSuccessMessage, false);
        });
      } catch (err) {
        updateServiceState(function(serviceState) {
          setFlowStatus(serviceState, flowID, err instanceof Error ? err.message : flow.requestErrorMessage, true);
        });
      }
    }
    async function confirmVerificationCode(flowID) {
      var flow = verificationFlows[flowID];
      if (!flow) return;
      var email = getServiceState().flows[flowID].pendingEmail;
      var code = flow.readCodeValue ? flow.readCodeValue() : readValue(flow.codeInputID);
      if (!email || !code) return;
      if (flow.beforeConfirm && flow.beforeConfirm() === false) {
        return;
      }
      updateServiceState(function(serviceState) {
        beginMutationState(serviceState.flows[flowID].confirm);
      });
      try {
        var data = await api.postCommercialJSON(flow.confirmPath, { email, code });
        updateServiceState(function(serviceState) {
          succeedMutationState(serviceState.flows[flowID].confirm);
          if (flow.applyConfirmSuccessState) {
            flow.applyConfirmSuccessState(serviceState, data, email);
          }
        });
        if (flow.afterConfirmSuccess) {
          flow.afterConfirmSuccess(data, email);
        }
      } catch (err) {
        var message = err instanceof Error ? err.message : flow.confirmErrorMessage;
        updateServiceState(function(serviceState) {
          failMutationState(serviceState.flows[flowID].confirm, message);
          setFlowStatus(serviceState, flowID, message, true);
        });
      }
    }
    async function copyRetrievedLicense() {
      var result = getServiceState().flows.retrieve.result;
      var token = result && result.token ? result.token : "";
      if (!token) return;
      try {
        await navigator.clipboard.writeText(token);
        updateServiceState(function(serviceState) {
          setFlowStatus(serviceState, "retrieve", "License key copied to clipboard.", false);
        });
      } catch (_) {
        updateServiceState(function(serviceState) {
          setFlowStatus(serviceState, "retrieve", "Failed to copy automatically. Please copy the key manually.", true);
        });
      }
    }
    async function submitRefund() {
      var email = getServiceState().refund.emailValue;
      var token = getServiceState().refund.tokenValue;
      if (!email || !token) return;
      if (!confirm("Are you sure? This will immediately revoke the license and request the refund.")) return;
      updateServiceState(function(serviceState) {
        beginMutationState(serviceState.refund.submit);
        serviceState.refund.status = emptyStatus();
      });
      try {
        await api.postCommercialJSON("/v1/self-refund", { email, token });
        updateServiceState(function(serviceState) {
          serviceState.refund.tokenValue = "";
          succeedMutationState(serviceState.refund.submit);
          setRefundStatus(serviceState, "Success! Your refund has been processed. Stripe will follow up by email.", false);
        });
      } catch (err) {
        var message = err instanceof Error ? err.message : "Refund failed";
        updateServiceState(function(serviceState) {
          failMutationState(serviceState.refund.submit, message);
          setRefundStatus(serviceState, message, true);
        });
      }
    }
    function renderServiceRuntime() {
      renderOpenPanels(getServiceState().openPanelID);
      renderAllFlows();
    }
    renderServiceRuntime();
    store.subscribeBootstrap(renderServiceRuntime);
    store.subscribeServices(renderServiceRuntime);
    installServicesController({
      setShellSection: function(section) {
        store.setActiveShellSection(section);
      },
      toggleServicePanel,
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
        updateServiceState(function(serviceState) {
          updateServiceInputValue(serviceState, inputKind, value);
        }, false);
      },
      updateDeleteConfirmation: function(checked) {
        updateServiceState(function(serviceState) {
          updateDeleteConfirmation(serviceState, checked);
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
  function countWorkspaces(accounts) {
    var total = 0;
    for (var i = 0; i < accounts.length; i += 1) {
      total += Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces.length : 0;
    }
    return total;
  }
  function countWorkspacesByState(workspaces, state) {
    var count = 0;
    for (var i = 0; i < workspaces.length; i += 1) {
      if (String(workspaces[i].state || "") === state) count += 1;
    }
    return count;
  }
  function countWorkspacesByHealth(workspaces, status) {
    var count = 0;
    for (var i = 0; i < workspaces.length; i += 1) {
      if (workspaceHealthState(workspaces[i]) === status) count += 1;
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
  function renderServiceActionRow(id, kicker, title, description, panelID, focusID, highlights) {
    var highlightHTML = highlights.map(function(item) {
      return '<span class="service-action-highlight">' + escapeHTML(item) + "</span>";
    }).join("");
    return '<article class="service-action-row"><div class="service-action-main"><div class="service-action-tags"><span class="service-card-kicker">' + kicker + '</span><span class="service-action-tag">Self-hosted</span></div><div class="service-action-copy"><h3>' + title + "</h3><p>" + description + '</p></div><div class="service-action-highlights">' + highlightHTML + '</div></div><div class="service-action-cta"><button class="btn-secondary service-action-button" type="button" id="' + id + '" data-account-service-action="open-service-panel" data-account-service-panel="' + panelID + '" data-account-service-focus="' + focusID + '" data-shell-target="services">Open</button></div></article>';
  }
  function workspaceStatusCopy(workspace) {
    var status = workspaceHealthState(workspace);
    if (status === "healthy") return "Live updates and health checks are currently good.";
    if (status === "unhealthy") return "This workspace needs attention before it is trustworthy.";
    return "This workspace is still waiting on a completed health check.";
  }
  function workspaceRowNote(workspace) {
    var status = workspaceHealthState(workspace);
    if (status === "healthy") return "Ready for operator work";
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
  function renderAttentionPanel(workspaces) {
    var attention = attentionWorkspaces(workspaces);
    if (!attention.length) {
      return '<div class="overview-side-card"><div class="account-panel-kicker">Attention</div><h4>Fleet is stable</h4><p>Every visible hosted workspace currently reports a healthy posture.</p></div>';
    }
    var items = attention.slice(0, 3).map(function(workspace) {
      return '<div class="overview-alert-row"><div class="overview-alert-main"><strong>' + escapeHTML(workspace.display_name) + "</strong><span>" + escapeHTML(workspaceStatusCopy(workspace)) + "</span></div>" + healthBadgeHTML(workspace) + "</div>";
    }).join("");
    return '<div class="overview-side-card"><div class="account-panel-kicker">Attention</div><h4>Needs review</h4><p>These workspaces should be checked before you treat the hosted fleet as fully healthy.</p><div class="overview-alert-list">' + items + "</div></div>";
  }
  function renderOverviewBand(accounts) {
    var hosted = hasHostedAccounts(accounts);
    var workspaceTotal = countWorkspaces(accounts);
    var statusText = hosted ? "Hosted access is active on this account." : "No hosted workspace access is attached to this account yet.";
    var summary = hosted ? "Hosted operations, operator access, and commercial account services." : "Billing, license recovery, refunds, and privacy actions until hosted access is attached.";
    return '<section class="portal-shell-head"><div class="portal-shell-head-main"><div class="portal-shell-head-kicker">Pulse Account</div><div class="portal-shell-head-row"><div class="portal-shell-head-copy"><div class="portal-shell-head-brand-row"><h1 class="portal-shell-head-title">' + (hosted ? "Operator console" : "Account console") + '</h1><span class="portal-shell-head-chip">' + (hosted ? "Operator ready" : "Self-hosted only") + "</span></div><p><strong>" + statusText + "</strong> " + summary + '</p></div><div class="portal-shell-head-stats"><div class="portal-shell-head-stat"><span class="portal-shell-head-stat-label">Hosted access</span><span class="portal-shell-head-stat-value">' + (hosted ? "Active" : "Not attached") + '</span></div><div class="portal-shell-head-stat"><span class="portal-shell-head-stat-label">Accounts</span><span class="portal-shell-head-stat-value">' + (accounts.length === 1 ? "1 account" : String(accounts.length) + " accounts") + '</span></div><div class="portal-shell-head-stat"><span class="portal-shell-head-stat-label">Workspace fleet</span><span class="portal-shell-head-stat-value">' + (workspaceTotal ? workspaceCountLabel(workspaceTotal) : "0 workspaces") + "</span></div></div></div></div></section>";
  }
  function renderPrimaryAccountBar(account) {
    var workspaceLabel = workspaceCountLabel((account.workspaces || []).length);
    var actionHTML = "";
    if (account.kind === "msp" && account.can_manage) {
      actionHTML += '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '" data-shell-target="workspaces">Add workspace</button>';
    }
    if (account.has_billing && account.can_manage) {
      actionHTML += '<button type="button" class="btn-secondary btn-compact" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Manage billing</button>';
    }
    if (account.can_manage) {
      actionHTML += '<button type="button" class="btn-secondary btn-compact" data-action="toggle-team" data-account-id="' + escapeAttr(account.id) + '" data-shell-target="team">Manage team</button>';
    }
    return '<section class="portal-account-bar"><div class="portal-account-bar-copy"><div class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</div><div class="portal-account-bar-row"><h2>' + escapeHTML(account.name) + '</h2><div class="portal-account-bar-chips"><span class="account-context-chip">' + escapeHTML(account.kind_label) + '</span><span class="account-context-chip">' + escapeHTML(titleCase(account.role)) + '</span><span class="account-context-chip">' + escapeHTML(workspaceLabel) + "</span></div></div><p>" + escapeHTML(account.kind === "msp" ? "Operator workspace account" : "Hosted account operations") + '</p></div><div class="portal-account-bar-actions">' + actionHTML + "</div></section>";
  }
  function shellSectionButton(section, activeSection, title, copy) {
    return '<button class="portal-shell-nav-link' + (activeSection === section ? " active" : "") + '" type="button" data-shell-action="activate-section" data-shell-section="' + section + '"><span class="portal-shell-nav-label">' + title + '</span><span class="portal-shell-nav-copy">' + copy + "</span></button>";
  }
  function renderShellNavigation(accounts, supportEmail, activeSection) {
    var hosted = hasHostedAccounts(accounts);
    return '<aside class="portal-shell-nav" aria-label="Pulse Account sections"><div class="portal-shell-nav-header"><div class="portal-shell-nav-eyebrow">Pulse Account</div><div class="portal-shell-nav-title">' + (hosted ? "Operator console" : "Account console") + '</div><div class="portal-shell-nav-support">' + (hosted ? "Hosted operations and account services" : "Commercial account services and support") + '</div></div><div class="portal-shell-nav-group">' + shellSectionButton("overview", activeSection, "Overview", hosted ? "Posture, accounts, and quick actions" : "Account summary and access state") + shellSectionButton("workspaces", activeSection, hosted ? "Workspaces" : "Hosted access", hosted ? "Hosted fleet and lifecycle actions" : "No hosted workspaces are attached yet") + shellSectionButton("team", activeSection, "Team", hosted ? "Access and operator roster" : "Account membership") + shellSectionButton("services", activeSection, "Account services", "Licenses, billing, refunds, and privacy") + shellSectionButton("support", activeSection, "Support", supportEmail || "Support contact") + '</div><div class="portal-shell-nav-footer"><div class="portal-shell-nav-footer-label">Need help?</div><a class="portal-shell-nav-footer-link" href="mailto:' + escapeAttr(supportEmail || "support@pulserelay.pro") + '">' + escapeHTML(supportEmail || "support@pulserelay.pro") + "</a></div></aside>";
  }
  function renderWorkspaceCard(account, workspace, accountAPIBasePath) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    var createdLabel = formatWorkspaceDate(workspace.created_at);
    var metaParts = [
      '<span class="workspace-meta-item">' + escapeHTML(workspace.id) + "</span>"
    ];
    if (createdLabel) {
      metaParts.push('<span class="workspace-meta-item">Created ' + escapeHTML(createdLabel) + "</span>");
    }
    if (workspace.last_health_check && status === "healthy") {
      metaParts.push('<span class="workspace-meta-item">Checked recently</span>');
    }
    var openAction = "";
    if (state === "active") {
      openAction = '<form method="POST" action="' + escapeAttr(accountAPIBasePath + "/" + account.id + "/tenants/" + workspace.id + "/handoff") + '"><button type="submit" class="btn-primary">Open workspace</button></form>';
    } else {
      openAction = '<span class="workspace-state-label">' + escapeHTML(titleCase(state || "Unknown")) + "</span>";
    }
    var manageAction = "";
    if (account.can_manage && (state === "active" || state === "suspended" || state === "failed")) {
      manageAction = '<button type="button" class="btn-secondary btn-workspace-manage" data-action="select-workspace" data-account-id="' + escapeAttr(account.id) + '" data-workspace-id="' + escapeAttr(workspace.id) + '">Manage</button>';
    }
    return '<article class="workspace-row"><div class="workspace-row-primary"><div class="workspace-row-heading"><h4 class="workspace-name">' + escapeHTML(workspace.display_name) + '</h4><div class="workspace-meta">' + metaParts.join("") + '</div></div><div class="workspace-row-note">' + escapeHTML(workspaceRowNote(workspace)) + '</div></div><div class="workspace-row-status-cell workspace-row-status-cell-badge">' + healthBadgeHTML(workspace) + '</div><div class="workspace-row-status-cell workspace-row-status-cell-badge"><span class="badge badge-' + escapeHTML(state || "unknown") + '">' + escapeHTML(titleCase(state || "Unknown")) + '</span></div><div class="workspace-actions">' + openAction + manageAction + "</div></article>";
  }
  function renderAccountOverviewSection(account) {
    var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
    var healthyCount = countWorkspacesByHealth(workspaces, "healthy");
    var checkingCount = countWorkspacesByHealth(workspaces, "checking");
    var unhealthyCount = countWorkspacesByHealth(workspaces, "unhealthy");
    var activeCount = countWorkspacesByState(workspaces, "active");
    var postureTitle = unhealthyCount > 0 ? "Hosted posture needs review" : checkingCount > 0 ? "Hosted posture is still settling" : "Hosted posture is stable";
    var postureCopy = unhealthyCount > 0 ? "One or more workspaces still need attention before the hosted fleet is trustworthy." : checkingCount > 0 ? "The hosted fleet is mostly healthy, but some workspaces are still waiting on a completed health check." : "The hosted fleet is healthy and ready for routine operator work.";
    var addWorkspaceForm = "";
    var nextStepTitle = unhealthyCount > 0 ? "Start in Workspaces" : checkingCount > 0 ? "Review pending checks" : "Fleet is clear";
    var nextStepCopy = unhealthyCount > 0 ? "One or more workspaces need review before you treat the hosted fleet as trustworthy." : checkingCount > 0 ? "The fleet is mostly healthy, but there are still workspaces waiting on a completed health check." : "Hosted posture looks stable. Move into team or account services only if you need to change access or billing.";
    var nextStepActions = '<div class="overview-next-actions"><button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Open workspaces</button><button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="' + (account.can_manage ? "team" : "services") + '">' + (account.can_manage ? "Review team access" : "Open account services") + "</button></div>";
    if (account.can_manage) {
      if (account.kind === "msp") {
        addWorkspaceForm = '<div class="add-workspace-form" id="add-ws-form-' + escapeAttr(account.id) + '"><label for="ws-name-' + escapeAttr(account.id) + '">Workspace name (for example, a client name)</label><input type="text" id="ws-name-' + escapeAttr(account.id) + '" placeholder="Acme Corp" maxlength="80" autocomplete="off"><div class="form-actions"><button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' + escapeAttr(account.id) + '">Create workspace</button><button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Cancel</button><div class="spinner" id="ws-spinner-' + escapeAttr(account.id) + '" hidden></div></div></div>';
      }
    }
    return '<section class="account-content-panel account-content-panel-overview"><div class="account-command-deck"><div class="account-overview-card"><div class="account-overview-lead"><div class="account-panel-kicker">Hosted posture</div><h3>' + escapeHTML(postureTitle) + "</h3><p>" + escapeHTML(postureCopy) + '</p><div class="account-overview-callout">' + escapeHTML(
      account.kind === "msp" ? "Use this console to run client workspaces, account billing, and operator access from one place." : "Use this console to run hosted workspaces, account billing, and operator access from one place."
    ) + '</div></div><div class="account-metric-strip"><div class="account-stat-card account-stat-card-inline"><span class="account-stat-label">Active workspaces</span><span class="account-stat-value">' + String(activeCount) + '</span></div><div class="account-stat-card account-stat-card-inline"><span class="account-stat-label">Healthy</span><span class="account-stat-value account-stat-healthy">' + String(healthyCount) + '</span></div><div class="account-stat-card account-stat-card-inline"><span class="account-stat-label">Checking</span><span class="account-stat-value account-stat-checking">' + String(checkingCount) + '</span></div><div class="account-stat-card account-stat-card-inline"><span class="account-stat-label">Needs attention</span><span class="account-stat-value account-stat-unhealthy">' + String(unhealthyCount) + "</span></div></div></div>" + addWorkspaceForm + '<div class="account-overview-secondary"><div class="overview-side-card overview-side-card-primary"><div class="account-panel-kicker">Next move</div><h4>' + escapeHTML(nextStepTitle) + "</h4><p>" + escapeHTML(nextStepCopy) + "</p>" + nextStepActions + "</div>" + renderAttentionPanel(workspaces) + "</div></div></section>";
  }
  function renderAccountWorkspaceSection(account, accountAPIBasePath) {
    var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
    var healthyCount = countWorkspacesByHealth(workspaces, "healthy");
    var checkingCount = countWorkspacesByHealth(workspaces, "checking");
    var unhealthyCount = countWorkspacesByHealth(workspaces, "unhealthy");
    var workspaceManagement = "";
    if (account.can_manage) {
      var workspaceDeskActions = "";
      if (account.kind === "msp") {
        workspaceDeskActions += '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Add workspace</button>';
      }
      if (account.has_billing) {
        workspaceDeskActions += '<button type="button" class="btn-secondary btn-compact" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Manage billing</button>';
      }
      workspaceDeskActions += '<button type="button" class="btn-secondary btn-compact" data-action="toggle-team" data-account-id="' + escapeAttr(account.id) + '" data-shell-target="team">Manage team</button>';
      workspaceManagement = '<section class="workspace-management-panel" id="workspace-management-' + escapeAttr(account.id) + '"><div class="workspace-management-header"><div><div class="account-panel-kicker">Workspace management</div><h3>Review one workspace at a time</h3><p>Select a workspace from the fleet to review its lifecycle state and run explicit management actions.</p></div><button type="button" class="btn-secondary btn-compact" id="workspace-management-close-' + escapeAttr(account.id) + '" data-action="clear-workspace-selection" data-account-id="' + escapeAttr(account.id) + '">Done</button></div><div class="workspace-management-empty" id="workspace-management-empty-' + escapeAttr(account.id) + '"><div class="workspace-management-empty-copy">Choose a workspace to manage from the fleet above.</div><div class="workspace-management-empty-actions">' + workspaceDeskActions + '</div><div class="workspace-management-empty-note">Use the fleet table for workspace-level work, or run account-wide billing and team actions from here.</div></div><div class="workspace-management-content" id="workspace-management-content-' + escapeAttr(account.id) + '" hidden><div class="workspace-management-meta" id="workspace-management-meta-' + escapeAttr(account.id) + '"></div><h4 id="workspace-management-title-' + escapeAttr(account.id) + '"></h4><p class="workspace-management-summary" id="workspace-management-summary-' + escapeAttr(account.id) + '"></p><div class="workspace-management-actions"><button type="button" class="btn-danger" id="workspace-management-action-' + escapeAttr(account.id) + '" data-action="workspace-action" data-account-id="' + escapeAttr(account.id) + '">Manage workspace</button></div></div></section>';
    }
    var workspaceHTML = workspaces.length ? '<div class="workspace-list-wrap"><div class="workspace-list-toolbar"><div class="workspace-list-summary">Review the hosted fleet, open a workspace, and keep lifecycle actions explicit.</div><div class="workspace-list-stats"><span class="workspace-list-stat"><strong>' + String(workspaces.length) + '</strong> total</span><span class="workspace-list-stat"><strong>' + String(healthyCount) + '</strong> healthy</span><span class="workspace-list-stat"><strong>' + String(checkingCount) + '</strong> checking</span><span class="workspace-list-stat workspace-list-stat-attention"><strong>' + String(unhealthyCount) + '</strong> needs attention</span></div></div><div class="workspace-list-head"><span>Workspace</span><span>Health</span><span>Lifecycle</span><span>Actions</span></div><div class="workspace-list">' + workspaces.map(function(workspace) {
      return renderWorkspaceCard(account, workspace, accountAPIBasePath);
    }).join("") + "</div></div>" : '<div class="empty-state"><p>No hosted workspaces yet. Create one to get started.</p></div>';
    return '<section class="account-content-panel account-content-panel-workspaces"><div class="account-stage-header"><div><div class="account-panel-kicker">Workspace fleet</div><h3>Hosted fleet</h3><p>Use the fleet view to open workspaces, watch health posture, and keep management actions explicit.</p></div></div><div class="workspace-operations-shell"><div class="workspace-operations-main">' + workspaceHTML + '</div><div class="workspace-operations-detail">' + workspaceManagement + "</div></div></section>";
  }
  function renderAccountTeamSection(account) {
    var accessPolicy = '<div class="team-policy-panel"><div class="team-panel-heading"><h4>Access model</h4><p>Assign the smallest role that still lets someone do the work they own on this account.</p></div><div class="team-policy-list"><div class="team-policy-row"><strong>Owner</strong><span>Billing, team access, and full hosted control.</span></div><div class="team-policy-row"><strong>Admin</strong><span>Hosted operations plus billing for the account.</span></div><div class="team-policy-row"><strong>Tech</strong><span>Workspace operations without billing ownership.</span></div><div class="team-policy-row"><strong>Read-only</strong><span>State review and verification without control-plane changes.</span></div></div></div>';
    return '<section class="account-content-panel account-content-panel-team"><section class="team-management-panel team-section team-section-shell" id="team-section-' + escapeAttr(account.id) + '" data-actor-role="' + escapeAttr(account.role) + '"><div class="team-management-header"><div><div class="account-panel-kicker">Team management</div><h3>Control who can operate this account</h3><p>Owners manage billing and access. Admins and techs keep the hosted fleet running day to day.</p></div><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Done</button></div><div class="team-management-stats" id="team-stats-' + escapeAttr(account.id) + '"></div><div class="team-management-grid"><div class="team-roster"><div class="team-panel-heading"><h4>People on this account</h4><p>Keep the roster small and role assignment explicit. The people listed here are the ones who can operate the hosted fleet.</p></div><div class="team-roster-list" id="team-list-' + escapeAttr(account.id) + '"><div class="team-list-message">Loading\u2026</div></div></div><div class="team-side-column"><div class="team-invite-panel"><div class="team-panel-heading"><h4>Invite someone new</h4><p>Add another operator with the minimum role they need for this account.</p></div><div class="team-invite"><div><label for="invite-email-' + escapeAttr(account.id) + '">Email</label><input type="email" id="invite-email-' + escapeAttr(account.id) + '" placeholder="user@example.com" autocomplete="off"></div><div><label for="invite-role-' + escapeAttr(account.id) + '">Role</label><select id="invite-role-' + escapeAttr(account.id) + '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div><button type="button" class="btn-primary btn-compact" data-action="invite-member" data-account-id="' + escapeAttr(account.id) + '">Invite</button></div></div>' + accessPolicy + "</div></div></section></section>";
  }
  function renderSupportSection(context) {
    return '<section class="portal-support-panel"><div class="account-panel-kicker">Support</div><h2>Support and escalation</h2><p>Use support when hosted access looks wrong, billing does not behave as expected, or you need help with commercial licensing and privacy actions.</p><div class="portal-support-card-grid"><div class="portal-support-card"><h3>Account support</h3><p>For access, tenant handoff, team, and billing issues, contact the hosted operations desk.</p><a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + '</a></div><div class="portal-support-card"><h3>Commercial services</h3><p>Self-hosted subscriptions, license recovery, refunds, and privacy requests all route through the same account surface.</p><button type="button" class="btn-secondary" data-shell-action="activate-section" data-shell-section="services">Open account services</button></div></div></section>';
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
    var activeSection = context.activeSection || "overview";
    var serviceHeading = hosted ? "Self-hosted licenses and billing" : "Account services";
    var serviceNote = hosted ? "Hosted operations live above. Use these commercial tools for self-hosted licenses, billing, refunds, and privacy actions." : "Use these account tools for self-hosted licenses, billing, refunds, and privacy actions.";
    var hostedContent = accounts.map(function(account) {
      return '<section class="account-surface"><div class="account-surface-body">' + renderAccountOverviewSection(account) + renderAccountWorkspaceSection(account, context.accountAPIBasePath) + renderAccountTeamSection(account) + "</div></section>";
    }).join("");
    return '<div class="portal-shell" data-shell-section="' + activeSection + '"><div class="portal-shell-layout">' + renderShellNavigation(accounts, context.bootstrap.support_email || "", activeSection) + '<div class="portal-shell-main">' + renderOverviewBand(accounts) + (accounts.length === 1 ? renderPrimaryAccountBar(accounts[0]) : "") + '<section class="portal-content-panel portal-content-panel-overview"><div id="accounts-root">' + hostedContent + '</div></section><section class="portal-content-panel portal-content-panel-services service-section" id="account-services-section"><div class="service-header"><div><div class="account-panel-kicker">Account services</div><h2>' + serviceHeading + '</h2></div><div class="service-note">' + serviceNote + '</div></div><div class="service-shell"><aside class="service-shell-sidebar"><div class="service-list-intro"><div class="account-panel-kicker">Commercial actions</div><p>Use these tools for self-hosted billing, license recovery, refunds, and privacy. Hosted workspace operations stay in Workspaces and Team.</p></div><div class="service-action-list">' + renderServiceActionRow("open-manage-service", "Billing", "Manage subscriptions", "Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.", "manage-service-panel", "manage-inline-email", ["Invoices and plan changes", "Subscription self-service"]) + renderServiceActionRow("open-retrieve-service", "Licenses", "Retrieve licenses", "Recover the latest active self-hosted license and invoice link for a commercial email address.", "retrieve-service-panel", "retrieve-inline-email", ["Latest active license", "Invoice lookup"]) + renderServiceActionRow("open-refund-service", "Refunds", "Refund requests", "Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.", "refund-service-panel", "refund-inline-email", ["Eligibility check", "Explicit revocation"]) + renderServiceActionRow("open-data-service", "Privacy", "Data and privacy", "Request commercial data export or deletion without leaving the account shell.", "data-service-panel", "data-export-email", ["Export or deletion", "Support escalation path"]) + '</div></aside><div class="service-shell-main"><div class="service-detail-shell"><div class="service-panel service-panel-empty visible" id="service-panel-empty"><div class="account-panel-kicker">Select a service</div><h3>Choose the next commercial action</h3><p>Open a billing, license, refund, or privacy flow from the service navigator. The active request stays here so the account-services area behaves like one operating desk instead of a list of disconnected tools.</p><div class="service-empty-columns"><div class="service-empty-column"><div class="service-empty-column-title">Start here</div><div class="service-empty-points"><div class="service-empty-point"><strong>Billing</strong><span>Open Stripe customer portal access after verification.</span></div><div class="service-empty-point"><strong>Licenses</strong><span>Recover the latest active self-hosted license and invoice link.</span></div><div class="service-empty-point"><strong>Refunds</strong><span>Confirm eligibility before revoking active commercial access.</span></div><div class="service-empty-point"><strong>Privacy</strong><span>Request export or deletion without leaving Pulse Account.</span></div></div></div><div class="service-empty-column"><div class="service-empty-column-title">What to expect</div><div class="service-empty-checklist"><div class="service-empty-check"><strong>Verification first</strong><span>Each flow confirms the commercial email before opening sensitive account actions.</span></div><div class="service-empty-check"><strong>One task at a time</strong><span>The active commercial request stays in this panel until you finish or switch tools.</span></div><div class="service-empty-check"><strong>Support stays close</strong><span>If billing, licenses, refunds, or privacy behave unexpectedly, escalate from this surface.</span></div></div></div></div><div class="service-empty-support">Need help with billing, refund, privacy, or license actions? <a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + '</a></div></div><div class="service-panel" id="manage-service-panel"><div id="manage-service-root"></div></div><div class="service-panel" id="retrieve-service-panel"><div id="retrieve-service-root"></div></div><div class="service-panel" id="refund-service-panel"><div id="refund-service-root"></div></div><div class="service-panel" id="data-service-panel"><h3>Data and privacy</h3><p>Request export or deletion of the commercial data tied to an email address. Payment data held directly by Stripe still requires support handling.</p><div class="subsection"><div id="data-export-root"></div></div><div class="subsection"><div id="data-delete-root"></div></div><div class="helper-text">Payment-card data stays with Stripe. For Stripe deletion support, contact <a href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + '</a>.</div></div></div><div class="service-inline-support"><div class="account-panel-kicker">Support</div><p>Use support if a billing, refund, privacy, or license flow does not behave as expected for this account.</p><a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + '</a></div></div></div></section><section class="portal-content-panel portal-content-panel-support">' + renderSupportSection(context) + "</section></div></div></div>";
  }
  function renderSignedOutPortalHTML(context) {
    var statusHTML = "";
    if (context.loginState.request.error) {
      statusHTML = '<div class="service-status visible error">' + escapeHTML(context.loginState.request.error) + "</div>";
    } else if (context.loginState.success) {
      var successMessage = context.loginState.successMessage || "If that email is registered, a magic link is on the way.";
      statusHTML = '<div class="service-status visible success">' + escapeHTML(successMessage) + `<br><br><strong>Don't see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.</div>`;
    }
    return '<section class="intro-card"><div class="account-panel-kicker">Pulse Account</div><h1>Sign in to the account surface</h1><p>Use one commercial email address to get into hosted workspaces, MSP access, billing, license recovery, refunds, and privacy actions.</p></section><section class="service-section service-section-auth"><div class="service-panel visible auth-panel"><h3>Sign in</h3><p>Enter the commercial email address for your Pulse account. I will send a magic link so you can open Pulse Account without managing a password.</p><div class="form-group"><label for="portal-login-email">Email address</label><input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' + escapeAttr(context.loginState.emailValue || "") + '" data-portal-input="login-email"></div><div class="form-actions"><button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' + (context.loginState.request.pending ? "Sending\u2026" : "Send magic link") + '</button><a class="btn-secondary link-button" href="' + escapeAttr(context.signupPath) + '">Create an account</a></div>' + statusHTML + "</div></section>";
  }

  // src/shell.ts
  function installShell(deps) {
    function syncShellSection() {
      var root = document.querySelector(".portal-shell");
      var activeSection = deps.store.getShellState().activeSection;
      if (root) {
        root.setAttribute("data-shell-section", activeSection);
      }
      var links = document.querySelectorAll('[data-shell-action="activate-section"]');
      links.forEach(function(node) {
        var button = node;
        button.classList.toggle("active", button.getAttribute("data-shell-section") === activeSection);
      });
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
    var serviceState = createPortalServiceState();
    syncLoginStateBootstrapEmail(loginState, bootstrapState.email || "");
    syncServiceStateBootstrapEmail(serviceState, bootstrapState.email || "");
    var accountSubscribers = /* @__PURE__ */ new Set();
    var bootstrapSubscribers = /* @__PURE__ */ new Set();
    var loginSubscribers = /* @__PURE__ */ new Set();
    var shellSubscribers = /* @__PURE__ */ new Set();
    var serviceSubscribers = /* @__PURE__ */ new Set();
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
      getServiceState: function() {
        return serviceState;
      },
      setBootstrap: function(nextBootstrap) {
        bootstrapState = normalizeBootstrap(bootstrapDefaults, nextBootstrap);
        syncLoginStateBootstrapEmail(loginState, bootstrapState.email || "");
        syncServiceStateBootstrapEmail(serviceState, bootstrapState.email || "");
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
      updateServiceState: function(mutator, options) {
        mutator(serviceState);
        if (!options || options.notify !== false) {
          notify(serviceSubscribers);
        }
        return serviceState;
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
      subscribeServices: function(listener) {
        serviceSubscribers.add(listener);
        return function() {
          serviceSubscribers.delete(listener);
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
  function normalizeHandoffService(value) {
    switch (String(value || "").trim()) {
      case "manage":
        return "manage-service-panel";
      case "retrieve":
        return "retrieve-service-panel";
      case "refund":
        return "refund-service-panel";
      case "data":
        return "data-service-panel";
      default:
        return "";
    }
  }
  function readPortalRuntimeHandoff(locationHref = window.location.href) {
    try {
      var params = new URL(locationHref).searchParams;
      return {
        email: normalizeHandoffEmail(params.get("email")),
        openPanelID: normalizeHandoffService(params.get("service"))
      };
    } catch {
      return {
        email: "",
        openPanelID: ""
      };
    }
  }
  function createBootstrapDefaults(embeddedBootstrap) {
    return {
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
      store.updateServiceState(function(serviceState) {
        serviceState.flows.manage.emailValue = handoff.email;
        serviceState.flows.retrieve.emailValue = handoff.email;
        serviceState.flows.export.emailValue = handoff.email;
        serviceState.flows.delete.emailValue = handoff.email;
        serviceState.refund.emailValue = handoff.email;
      }, { notify: false });
    }
    if (handoff.openPanelID) {
      store.updateServiceState(function(serviceState) {
        serviceState.openPanelID = handoff.openPanelID;
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
        if (section === "team") {
          var accounts = deps.store.getBootstrap().accounts || [];
          if (accounts[0]) {
            accountRuntime.ensureTeamVisible(accounts[0].id);
          }
        }
      }
    });
    installServicesRuntime({
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
