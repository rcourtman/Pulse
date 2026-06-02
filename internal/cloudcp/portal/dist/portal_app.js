(() => {
  // src/account_roles.ts
  function normalizePortalRole(role) {
    if (role === "member") return "read_only";
    return role || "read_only";
  }
  function portalRoleLabel(role) {
    switch (normalizePortalRole(role)) {
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
  function portalRoleCapabilityCopy(role, clientLanguage = false, hasBilling = true) {
    switch (normalizePortalRole(role)) {
      case "owner":
        if (!hasBilling) {
          return clientLanguage ? "Full account control, including access control and client control." : "Full account control, including access control and workspace control.";
        }
        return clientLanguage ? "Full account control, including billing, access control, and client control." : "Full account control, including billing, access control, and workspace control.";
      case "admin":
        if (!hasBilling) {
          return clientLanguage ? "Can manage clients and account access." : "Can manage workspaces and account access.";
        }
        return clientLanguage ? "Can manage clients and billing for this account." : "Can manage workspaces and billing for this account.";
      case "tech":
        if (!hasBilling) {
          return clientLanguage ? "Can manage clients without access ownership." : "Can manage workspaces without access ownership.";
        }
        return clientLanguage ? "Can manage clients without billing ownership." : "Can manage workspaces without billing ownership.";
      case "read_only":
        return clientLanguage ? "Can review client status without making control-plane changes." : "Can review workspace status without making control-plane changes.";
      case "member":
        return "Has access through the account roster.";
      default:
        return "Has access through the account roster.";
    }
  }

  // src/workspace_presentation.ts
  function workspaceHealthState(workspace) {
    if (workspace.health_status === "healthy" || workspace.health_status === "checking" || workspace.health_status === "unhealthy") {
      return workspace.health_status;
    }
    if (workspace.healthy) return "healthy";
    if (workspace.last_health_check) return "unhealthy";
    return "checking";
  }
  function workspaceStatusCopy(workspace) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    if (state === "suspended") return "This workspace is suspended.";
    if (state === "failed") return "This workspace is in a failed state.";
    if (status === "healthy") return "Latest health check is healthy.";
    if (status === "unhealthy") return "Latest health check is unhealthy.";
    return "Latest health check is still pending.";
  }
  function workspaceHealthLabel(workspace) {
    var status = workspaceHealthState(workspace);
    if (status === "healthy") return "Healthy";
    if (status === "unhealthy") return "Unhealthy";
    return "Checking";
  }
  function workspaceRowNote(workspace) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    if (state === "suspended") return "Suspended";
    if (state === "failed") return "Failed";
    if (status === "checking") return "Health check pending";
    if (status === "unhealthy") return "Unhealthy";
    var setup = workspaceSetupState(workspace);
    if (setup === "ready") return "Ready";
    if (setup === "install_agents") return "Install first agent";
    if (setup === "configure_outputs") return "Configure alerts and reports";
    if (setup === "setup_path") return "Setup path ready";
    if (setup === "review") return "Review";
    return "Ready";
  }
  function hasNumber(value) {
    return typeof value === "number" && Number.isFinite(value);
  }
  function positiveCount(value) {
    return hasNumber(value) && Number(value) > 0;
  }
  function zeroCount(value) {
    return hasNumber(value) && Number(value) <= 0;
  }
  function explicitSetupStatus(workspace) {
    var value = String(workspace.setup_status || "");
    if (value === "ready" || value === "setup_path" || value === "install_agents" || value === "configure_outputs" || value === "review") {
      return value;
    }
    return "";
  }
  function workspaceSetupState(workspace) {
    var state = String(workspace.state || "");
    var health = workspaceHealthState(workspace);
    if (state === "suspended" || state === "failed" || health === "unhealthy") return "review";
    if (state !== "active" || health === "checking") return "setup_path";
    var knowsAgentCount = hasNumber(workspace.agent_count);
    var knowsAlertCount = hasNumber(workspace.alert_route_count);
    var knowsReportCount = hasNumber(workspace.report_schedule_count);
    var hasAgents = positiveCount(workspace.agent_count);
    var hasAlerts = positiveCount(workspace.alert_route_count);
    var hasReports = positiveCount(workspace.report_schedule_count);
    if (knowsAgentCount && !hasAgents) return "install_agents";
    if (hasAgents && (knowsAlertCount && !hasAlerts || knowsReportCount && !hasReports)) {
      return "configure_outputs";
    }
    if (hasAgents && (!knowsAlertCount || hasAlerts) && (!knowsReportCount || hasReports)) {
      return "ready";
    }
    var explicit = explicitSetupStatus(workspace);
    if (explicit) return explicit;
    return "setup_path";
  }
  function workspaceSetupLabel(workspace) {
    switch (workspaceSetupState(workspace)) {
      case "ready":
        return "Ready";
      case "install_agents":
        return "Install agent";
      case "configure_outputs":
        return "Configure outputs";
      case "review":
        return "Review";
      default:
        return "Setup path";
    }
  }
  function workspaceSetupNextStep(workspace) {
    switch (workspaceSetupState(workspace)) {
      case "ready":
        return "Open the workspace when you need to work inside this client boundary.";
      case "install_agents":
        return "Install the first agent from this workspace so client data lands in the isolated workspace boundary.";
      case "configure_outputs":
        return "Configure alert routing and reports before treating the client workspace as ready.";
      case "review":
        return "Review the workspace state before continuing setup.";
      default:
        return "Open the workspace or install agents from the workspace-bound setup path.";
    }
  }
  function workspaceIdentityCopy(workspace) {
    return "Client workspace boundary: " + workspace.display_name + ". Hostnames can repeat across clients because agents, alerts, and reports stay scoped to this workspace.";
  }
  function workspaceSetupDiagnostics(workspace) {
    var setup = workspaceSetupState(workspace);
    var diagnostics = [];
    if (setup === "ready") {
      diagnostics.push("Reporting agent, enabled alert route, and enabled report schedule are present.");
      return diagnostics;
    }
    if (setup === "review") {
      diagnostics.push(workspaceStatusCopy(workspace));
      return diagnostics;
    }
    if (zeroCount(workspace.agent_count)) {
      if (positiveCount(workspace.unused_agent_token_count) || positiveCount(workspace.agent_token_count)) {
        diagnostics.push("Agent install token exists, but no reporting agent has checked in yet.");
      } else {
        diagnostics.push("No reporting agent has checked in yet.");
      }
    }
    if (positiveCount(workspace.agent_count) && zeroCount(workspace.alert_route_count)) {
      diagnostics.push(
        positiveCount(workspace.disabled_alert_route_count) ? "Alert route configuration exists, but no route is enabled." : "No enabled alert route is configured yet."
      );
    }
    if (positiveCount(workspace.agent_count) && zeroCount(workspaceReportScheduleCount(workspace))) {
      diagnostics.push(
        positiveCount(workspace.disabled_report_schedule_count) ? "Report schedule exists, but no schedule is enabled." : "No enabled report schedule is configured yet."
      );
    }
    if (!diagnostics.length) {
      diagnostics.push(workspaceSetupNextStep(workspace));
    }
    return diagnostics;
  }
  function workspaceReportScheduleCount(workspace) {
    return workspace.report_schedule_count;
  }
  function workspaceSetupGuide(workspace) {
    var setup = workspaceSetupState(workspace);
    if (setup === "ready") {
      return {
        title: "Ready",
        description: "Agents, alert routing, and reports are in place for this client workspace.",
        primaryAction: "open",
        primaryLabel: "Open workspace",
        diagnostics: workspaceSetupDiagnostics(workspace)
      };
    }
    if (setup === "review") {
      return {
        title: "Review workspace state",
        description: "Resolve the workspace state before continuing client setup.",
        primaryAction: "review",
        primaryLabel: "Review workspace",
        diagnostics: workspaceSetupDiagnostics(workspace)
      };
    }
    if (setup === "install_agents") {
      return {
        title: "Install the first agent",
        description: "Start inside this isolated client workspace so the first reporting token and future hostnames stay scoped to the client.",
        primaryAction: "install",
        primaryLabel: "Install agents",
        diagnostics: workspaceSetupDiagnostics(workspace)
      };
    }
    if (setup === "configure_outputs") {
      var needsAlerts = zeroCount(workspace.alert_route_count);
      var needsReports = zeroCount(workspaceReportScheduleCount(workspace));
      return {
        title: needsAlerts && needsReports ? "Configure alerts and reports" : needsAlerts ? "Configure alert routes" : "Schedule reports",
        description: "Finish the output side before this workspace leaves onboarding.",
        primaryAction: "outputs",
        primaryLabel: needsReports && !needsAlerts ? "Open reports" : "Configure outputs",
        diagnostics: workspaceSetupDiagnostics(workspace)
      };
    }
    return {
      title: "Follow the setup path",
      description: "Open the client workspace and continue the next setup task from there.",
      primaryAction: "install",
      primaryLabel: "Open setup",
      diagnostics: workspaceSetupDiagnostics(workspace)
    };
  }
  function workspaceSetupSteps(workspace) {
    var setup = workspaceSetupState(workspace);
    var state = String(workspace.state || "");
    var isActive = state === "active";
    var hasAgents = positiveCount(workspace.agent_count) || setup === "configure_outputs" || setup === "ready";
    var hasAlerts = positiveCount(workspace.alert_route_count);
    var hasReports = positiveCount(workspaceReportScheduleCount(workspace));
    return [
      {
        id: "workspace",
        title: "Create workspace",
        detail: "Separate client boundary created.",
        tone: state ? "done" : "pending",
        label: state ? "Done" : "Pending"
      },
      {
        id: "agent",
        title: "Install first agent",
        detail: "First reporting agent checks in inside this workspace.",
        tone: hasAgents ? "done" : setup === "review" || !isActive ? "blocked" : "next",
        label: hasAgents ? "Done" : setup === "review" || !isActive ? "Review" : "Next"
      },
      {
        id: "alerts",
        title: "Configure alert routes",
        detail: "Enabled notification route exists for this client.",
        tone: hasAlerts ? "done" : setup === "review" ? "blocked" : isActive && hasAgents ? "next" : "pending",
        label: hasAlerts ? "Done" : setup === "review" ? "Review" : isActive && hasAgents ? "Next" : "Pending"
      },
      {
        id: "reports",
        title: "Schedule reports",
        detail: "Enabled report schedule exists for client reporting.",
        tone: hasReports ? "done" : setup === "review" ? "blocked" : isActive && hasAgents && hasAlerts ? "next" : "pending",
        label: hasReports ? "Done" : setup === "review" ? "Review" : isActive && hasAgents && hasAlerts ? "Next" : "Pending"
      },
      {
        id: "access",
        title: "Review access",
        detail: "Provider staff and client users are handled from Access.",
        tone: "available",
        label: "Available"
      }
    ];
  }
  function workspaceGuidanceCopy(workspace) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    if (state === "active" && status === "healthy") {
      return workspaceSetupNextStep(workspace);
    }
    if (state === "active" && status === "checking") {
      return "This workspace is active. The latest health check is still pending, but the workspace can still own agent install commands.";
    }
    if (status === "unhealthy") {
      return "The latest health check is unhealthy. Review the current state before suspending or deleting this workspace.";
    }
    if (state === "suspended") {
      return "This workspace is suspended. The remaining destructive action here is deletion.";
    }
    return "Review the current workspace state before taking action on this workspace.";
  }

  // src/account_view.ts
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
  function workspaceActionLabel(workspace, clientLanguage = false) {
    if (clientLanguage) {
      return workspace.state === "active" ? "Suspend client" : "Delete client";
    }
    return workspace.state === "active" ? "Suspend workspace" : "Delete workspace";
  }
  function workspaceCreatedLabel(workspace) {
    if (!workspace.created_at) return "Unknown";
    var date = new Date(workspace.created_at);
    if (Number.isNaN(date.getTime())) return "Unknown";
    return date.toLocaleDateString(void 0, { month: "short", day: "numeric", year: "numeric" });
  }
  function hasKnownCount(value) {
    return typeof value === "number" && Number.isFinite(value);
  }
  function setupCountLabel(value, singular, plural) {
    if (!hasKnownCount(value)) return "Unknown";
    var count = Number(value);
    return String(count) + " " + (count === 1 ? singular : plural);
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
  function setChecklistStatus(element, tone, label) {
    if (!element) return;
    element.textContent = label;
    element.className = "workspace-setup-status workspace-setup-status-" + tone;
  }
  function findWorkspace(account, workspaceID) {
    for (var i = 0; i < account.workspaces.length; i += 1) {
      if (account.workspaces[i].id === workspaceID) return account.workspaces[i];
    }
    return null;
  }
  var WORKSPACE_INSTALL_TARGET_PATH = "/settings/infrastructure?add=linux-host";
  var WORKSPACE_REPORTING_TARGET_PATH = "/settings/support/reporting";
  function workspaceHandoffActionPath(accountAPIBasePath, accountID, workspaceID, targetPath = "") {
    var path = accountAPIBasePath + "/" + encodeURIComponent(accountID) + "/tenants/" + encodeURIComponent(workspaceID) + "/handoff";
    if (!targetPath) return path;
    return path + "?target_path=" + encodeURIComponent(targetPath);
  }
  function setWorkspaceHandoffForm(form, button, accountAPIBasePath, accountID, workspace, targetPath = "", pending = false) {
    if (!form || !button) return;
    var canOpen = workspace.state === "active" && !!accountAPIBasePath;
    if (canOpen) {
      form.action = workspaceHandoffActionPath(accountAPIBasePath, accountID, workspace.id, targetPath);
    } else {
      form.removeAttribute("action");
    }
    button.disabled = pending || !canOpen;
  }
  function renderWorkspaceManagement(account, entry, accountAPIBasePath = "") {
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
    var setup = getElement("workspace-management-setup-" + account.id);
    var agents = getElement("workspace-management-agents-" + account.id);
    var alerts = getElement("workspace-management-alerts-" + account.id);
    var reports = getElement("workspace-management-reports-" + account.id);
    var created = getElement("workspace-management-created-" + account.id);
    var guidance = getElement("workspace-management-guidance-" + account.id);
    var identity = getElement("workspace-management-identity-" + account.id);
    var guideTitle = getElement("workspace-management-guide-title-" + account.id);
    var guideDescription = getElement("workspace-management-guide-description-" + account.id);
    var guideDiagnostics = getElement("workspace-management-guide-diagnostics-" + account.id);
    var guidePrimaryForm = getElement("workspace-management-primary-form-" + account.id);
    var guidePrimaryButton = getElement("workspace-management-primary-" + account.id);
    var checkCreated = getElement("workspace-management-check-created-" + account.id);
    var checkInstall = getElement("workspace-management-check-install-" + account.id);
    var checkAlerts = getElement("workspace-management-check-alerts-" + account.id);
    var checkReports = getElement("workspace-management-check-reports-" + account.id);
    var checkAccess = getElement("workspace-management-check-access-" + account.id);
    var actionButton = getElement("workspace-management-action-" + account.id);
    var closeButton = getElement("workspace-management-close-" + account.id);
    var openForm = getElement("workspace-management-open-form-" + account.id);
    var openButton = getElement("workspace-management-open-" + account.id);
    var installForm = getElement("workspace-management-install-form-" + account.id);
    var installButton = getElement("workspace-management-install-" + account.id);
    var reportingForm = getElement("workspace-management-reporting-form-" + account.id);
    var reportingButton = getElement("workspace-management-reporting-" + account.id);
    if (!empty || !content || !title || !meta || !summary || !health || !setup || !created || !guidance || !actionButton || !closeButton) return;
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
      openForm?.removeAttribute("action");
      installForm?.removeAttribute("action");
      reportingForm?.removeAttribute("action");
      guidePrimaryForm?.removeAttribute("action");
      if (openButton) openButton.disabled = true;
      if (installButton) installButton.disabled = true;
      if (reportingButton) reportingButton.disabled = true;
      if (guidePrimaryButton) guidePrimaryButton.disabled = true;
      return;
    }
    title.textContent = workspace.display_name;
    meta.textContent = workspaceMeta(workspace);
    summary.textContent = workspaceStatusCopy(workspace);
    health.textContent = workspaceHealthLabel(workspace);
    setup.textContent = workspaceSetupLabel(workspace);
    if (agents) agents.textContent = setupCountLabel(workspace.agent_count, "agent", "agents");
    if (alerts) alerts.textContent = setupCountLabel(workspace.alert_route_count, "route", "routes");
    if (reports) reports.textContent = setupCountLabel(workspace.report_schedule_count, "schedule", "schedules");
    created.textContent = workspaceCreatedLabel(workspace);
    guidance.textContent = workspaceGuidanceCopy(workspace);
    if (identity) identity.textContent = workspaceIdentityCopy(workspace);
    var guide = workspaceSetupGuide(workspace);
    if (guideTitle) guideTitle.textContent = guide.title;
    if (guideDescription) guideDescription.textContent = guide.description;
    if (guideDiagnostics) {
      guideDiagnostics.textContent = "";
      for (var d = 0; d < guide.diagnostics.length; d += 1) {
        var item = document.createElement("li");
        item.textContent = guide.diagnostics[d];
        guideDiagnostics.appendChild(item);
      }
    }
    var primaryTargetPath = "";
    if (guide.primaryAction === "install") {
      primaryTargetPath = WORKSPACE_INSTALL_TARGET_PATH;
    } else if (guide.primaryAction === "outputs") {
      primaryTargetPath = WORKSPACE_REPORTING_TARGET_PATH;
    }
    if (guidePrimaryButton) guidePrimaryButton.textContent = guide.primaryLabel;
    setWorkspaceHandoffForm(guidePrimaryForm, guidePrimaryButton, accountAPIBasePath, account.id, workspace, primaryTargetPath, entry.manageWorkspace.pending);
    var steps = workspaceSetupSteps(workspace);
    var stepByID = {
      workspace: checkCreated,
      agent: checkInstall,
      alerts: checkAlerts,
      reports: checkReports,
      access: checkAccess
    };
    for (var s = 0; s < steps.length; s += 1) {
      setChecklistStatus(stepByID[steps[s].id], steps[s].tone, steps[s].label);
    }
    actionButton.textContent = workspaceActionLabel(workspace, account.kind === "msp");
    actionButton.disabled = entry.manageWorkspace.pending;
    actionButton.setAttribute("data-workspace-id", workspace.id);
    actionButton.setAttribute("data-workspace-name", workspace.display_name);
    actionButton.setAttribute("data-workspace-action", workspace.state === "active" ? "suspend" : "delete");
    closeButton.disabled = entry.manageWorkspace.pending;
    setWorkspaceHandoffForm(openForm, openButton, accountAPIBasePath, account.id, workspace, "", entry.manageWorkspace.pending);
    setWorkspaceHandoffForm(installForm, installButton, accountAPIBasePath, account.id, workspace, WORKSPACE_INSTALL_TARGET_PATH, entry.manageWorkspace.pending);
    setWorkspaceHandoffForm(reportingForm, reportingButton, accountAPIBasePath, account.id, workspace, WORKSPACE_REPORTING_TARGET_PATH, entry.manageWorkspace.pending);
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
      if ((members[i].state || "active") !== "active") continue;
      if (normalizePortalRole(members[i].role) === role) count += 1;
    }
    return count;
  }
  function countPendingMembers(members) {
    var count = 0;
    for (var i = 0; i < members.length; i += 1) {
      if ((members[i].state || "active") === "pending") count += 1;
    }
    return count;
  }
  function renderAccessStatsSummary(summary, isError) {
    return '<div class="access-stat-summary' + (isError ? " access-stat-summary-error" : "") + '">' + summary + "</div>";
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
      stats.innerHTML = renderAccessStatsSummary("Access \u2022 " + (canManage ? "Manage access" : "View roster") + " \u2022 Loading roster", false);
      return;
    }
    if (entry.accessQuery.status === "error") {
      stats.innerHTML = renderAccessStatsSummary("Access \u2022 " + (canManage ? "Manage access" : "View roster") + " \u2022 Load failed", true);
      return;
    }
    var members = entry.accessQuery.data;
    stats.innerHTML = renderAccessStatsSummary(
      "Members " + String(members.length - countPendingMembers(members)) + " \u2022 Pending " + String(countPendingMembers(members)) + " \u2022 Owners " + String(countMembersByRole(members, "owner")) + " \u2022 Admins " + String(countMembersByRole(members, "admin")) + " \u2022 Operators " + String(countMembersByRole(members, "tech") + countMembersByRole(members, "read_only")),
      false
    );
  }
  function createAccessControlCell(className) {
    var cell = document.createElement("div");
    cell.className = "access-control-cell " + className;
    return cell;
  }
  function renderAccessRoleControl(accountID, member, isOwner, canManage, activeJob) {
    var currentRole = normalizePortalRole(member.role);
    var subjectID = member.subject_id || member.user_id || "";
    var group = createAccessControlCell("access-control-cell-role");
    if (!canManage || activeJob !== "change_role") {
      var badge = document.createElement("span");
      badge.className = "access-role-badge";
      badge.textContent = portalRoleLabel(currentRole);
      group.appendChild(badge);
      return group;
    }
    if (currentRole === "owner" && !isOwner) {
      var locked = document.createElement("span");
      locked.className = "access-role-badge";
      locked.textContent = portalRoleLabel(currentRole);
      group.appendChild(locked);
      return group;
    }
    var sel = document.createElement("select");
    sel.className = "access-role-select";
    var roles = isOwner ? ["owner", "admin", "tech", "read_only"] : ["admin", "tech", "read_only"];
    for (var j = 0; j < roles.length; j += 1) {
      var opt = document.createElement("option");
      opt.value = roles[j];
      opt.textContent = portalRoleLabel(roles[j]);
      if (currentRole === roles[j]) opt.selected = true;
      sel.appendChild(opt);
    }
    sel.setAttribute("data-action", "change-role");
    sel.setAttribute("data-account-id", accountID);
    sel.setAttribute("data-user-id", subjectID);
    group.appendChild(sel);
    return group;
  }
  function renderAccessMemberAction(accountID, member, isOwner, canManage, activeJob) {
    var subjectID = member.subject_id || member.user_id || "";
    if (!canManage || activeJob !== "remove") {
      return null;
    }
    var group = createAccessControlCell("access-control-cell-access");
    if (normalizePortalRole(member.role) === "owner" && !isOwner) {
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
    btn.setAttribute("data-user-id", subjectID);
    btn.setAttribute("data-member-email", member.email);
    group.appendChild(btn);
    return group;
  }
  function renderAccessMemberRow(accountID, member, isOwner, canManage, activeJob, clientLanguage, hasBilling) {
    var showActionColumn = canManage && activeJob === "remove";
    var row = document.createElement("div");
    row.className = "access-member-row" + (showActionColumn ? "" : " access-member-row-readonly");
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
    roleBadge.textContent = portalRoleLabel(member.role);
    topline.appendChild(roleBadge);
    if ((member.state || "active") === "pending") {
      var pendingBadge = document.createElement("span");
      pendingBadge.className = "access-inline-role-badge";
      pendingBadge.textContent = "Pending";
      topline.appendChild(pendingBadge);
    }
    identity.appendChild(topline);
    var caption = document.createElement("div");
    caption.className = "access-member-caption";
    caption.textContent = (member.state || "active") === "pending" ? "Invitation pending acceptance." : portalRoleCapabilityCopy(member.role, clientLanguage, hasBilling);
    identity.appendChild(caption);
    row.appendChild(identity);
    row.appendChild(renderAccessRoleControl(accountID, member, isOwner, canManage, activeJob));
    var actionCell = renderAccessMemberAction(accountID, member, isOwner, canManage, activeJob);
    if (actionCell) {
      row.appendChild(actionCell);
    }
    return row;
  }
  function renderAccessRosterHead(container, activeJob, canManage) {
    var showActionColumn = canManage && activeJob === "remove";
    var head = document.createElement("div");
    head.className = "access-roster-head" + (showActionColumn ? "" : " access-roster-head-readonly");
    head.innerHTML = showActionColumn ? "<span>Operator</span><span>Role</span><span>Remove</span>" : "<span>Operator</span><span>Role</span>";
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
  function renderAccessSection(accountID, entry, hasBilling = true) {
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
    var clientLanguage = section.getAttribute("data-client-language") === "true";
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
      setContainerMessage(roster, "Failed to load roster", entry.accessQuery.error, true);
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
    renderAccessRosterHead(roster, activeJob, canManage);
    for (var i = 0; i < entry.accessQuery.data.length; i += 1) {
      var member = entry.accessQuery.data[i];
      roster.appendChild(renderAccessMemberRow(accountID, member, isOwner, canManage, activeJob, clientLanguage, hasBilling));
    }
  }
  function renderAccountUI(accountState, accounts, accountAPIBasePath = "") {
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
      if (account) renderWorkspaceManagement(account, entry, accountAPIBasePath);
      renderAccessSection(accountID, entry, account ? account.has_billing === true : true);
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
        throw new PortalAPIError(fallbackMessage, 0, null);
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
      getCommercialJSON: function(path) {
        return request(bootstrap().commercial_api_base_url + path, {
          headers: { Accept: "application/json" }
        }, "Commercial request failed.");
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

  // src/shell_section.ts
  function preferredPortalShellSection(bootstrap) {
    var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
    var hasHostedAccounts2 = accounts.length > 0;
    var hasSelfHostedCommercial2 = bootstrap.has_self_hosted_commercial === true || !hasHostedAccounts2;
    if (hasHostedAccounts2) {
      return "workspaces";
    }
    if (hasSelfHostedCommercial2) {
      return "billing";
    }
    return "billing";
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
  function createPortalShellState(initialBootstrap) {
    return {
      activeSection: preferredPortalShellSection(initialBootstrap)
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
  function clonePortalAccessMembers(members) {
    var cloned = [];
    for (var i = 0; i < members.length; i += 1) {
      cloned.push({
        subject_id: members[i].subject_id,
        email: members[i].email,
        role: members[i].role,
        user_id: members[i].user_id,
        state: members[i].state,
        created_at: members[i].created_at
      });
    }
    return cloned;
  }
  function syncPortalAccountStateBootstrap(accountState, accounts) {
    for (var i = 0; i < accounts.length; i += 1) {
      var account = accounts[i];
      var entry = ensurePortalAccountUIEntry(accountState, account.id);
      entry.accessQuery.status = "ready";
      entry.accessQuery.error = "";
      entry.accessQuery.data = clonePortalAccessMembers(account.members || []);
    }
  }
  function createPortalBillingState() {
    return {
      openBillingPanelID: "",
      upgradeFeatureKey: "",
      upgradePortalHandoffID: "",
      upgradePortalHandoff: createQueryState(null),
      upgradePricing: createQueryState(null),
      upgradeCheckout: createMutationState(),
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
    var accountUsesClientLanguage2 = function(accountID) {
      var bootstrap = deps.store.getBootstrap();
      var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
      for (var i = 0; i < accounts.length; i += 1) {
        if (accounts[i].id === accountID) {
          return accounts[i].kind === "msp";
        }
      }
      return false;
    };
    var findWorkspaceIDByName = function(accountID, displayName) {
      var bootstrap = deps.store.getBootstrap();
      var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
      for (var i = 0; i < accounts.length; i += 1) {
        if (accounts[i].id !== accountID) continue;
        var workspaces = Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces : [];
        for (var j = 0; j < workspaces.length; j += 1) {
          if (workspaces[j].display_name === displayName) {
            return workspaces[j].id;
          }
        }
      }
      return "";
    };
    var renderAccountRuntime = function() {
      var bootstrap = deps.store.getBootstrap();
      renderAccountUI(deps.store.getAccountState(), bootstrap.accounts || [], bootstrap.account_api_base_path || "");
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
        var created = await deps.api.createWorkspace(accountID, { display_name: name });
        if (!await refreshOrRedirect()) {
          deps.store.updateAccountState(function(accountState) {
            var entry = ensurePortalAccountUIEntry(accountState, accountID);
            resetMutationState(entry.createWorkspace);
          }, { notify: false });
          return;
        }
        var createdWorkspaceID = created && typeof created.id === "string" ? created.id : "";
        if (!createdWorkspaceID) {
          createdWorkspaceID = findWorkspaceIDByName(accountID, name);
        }
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          entry.addWorkspaceOpen = false;
          entry.selectedWorkspaceID = createdWorkspaceID;
          entry.accessVisible = false;
          entry.activeAccessJob = "";
          succeedMutationState(entry.createWorkspace);
        });
        revealElementWhenReady("workspace-management-" + accountID);
        deps.showToast(accountUsesClientLanguage2(accountID) ? "Client added. Finish onboarding next." : "Workspace created. Finish setup next.");
      } catch (error) {
        var message = error instanceof Error ? error.message : accountUsesClientLanguage2(accountID) ? "Failed to add client." : "Failed to create workspace.";
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          failMutationState(entry.createWorkspace, message);
        }, { notify: false });
        deps.showToast(message, true);
      }
    };
    var manageWorkspaceAction = async function(accountID, tenantID, action, name) {
      var verb = action === "suspend" ? "Suspend" : action === "delete" ? "Delete" : "";
      var pastVerb = action === "suspend" ? "Suspended" : action === "delete" ? "Deleted" : "";
      if (!verb) return;
      var entityName = accountUsesClientLanguage2(accountID) ? "client" : "workspace";
      if (!window.confirm(verb + " " + entityName + ' "' + name + '"?')) return;
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
        deps.showToast(pastVerb + " " + entityName + ".");
      } catch (error) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          failMutationState(entry.manageWorkspace, error instanceof Error ? error.message : "Failed to " + verb.toLowerCase() + " " + entityName + ".");
        }, { notify: false });
        deps.showToast(error instanceof Error ? error.message : "Failed to " + verb.toLowerCase() + " " + entityName + ".", true);
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
        var result = await deps.api.inviteMember(accountID, { email, role: roleEl.value });
        emailEl.value = "";
        if (!await refreshAccountAccessSection(accountID)) {
          return;
        }
        deps.showToast(result && result.state === "active" ? "Member added." : "Invitation saved.");
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
  function renderUpgradePlansHTML(billingState) {
    var pricing = billingState.upgradePricing.data;
    if (!pricing || !Array.isArray(pricing.plans)) {
      return "";
    }
    var plans = pricing.plans.filter(function(plan) {
      return Array.isArray(plan.buttons) && plan.buttons.some(function(button) {
        return button.kind === "checkout" && button.planKey && button.billingCycle;
      });
    });
    if (plans.length === 0) {
      return "";
    }
    var portalHandoffID = String(billingState.upgradePortalHandoffID || "").trim();
    var handoffLifecycle = String(
      billingState.upgradePortalHandoff.data && billingState.upgradePortalHandoff.data.status || ""
    ).trim();
    var checkoutDisabled = billingState.upgradeCheckout.pending || !portalHandoffID || billingState.upgradePortalHandoff.status === "loading" || billingState.upgradePortalHandoff.status !== "ready" || handoffLifecycle === "completed";
    return '<div class="billing-upgrade-plan-grid">' + plans.map(function(plan) {
      var buttons = Array.isArray(plan.buttons) ? plan.buttons : [];
      var checkoutButtons = buttons.filter(function(button) {
        return button.kind === "checkout" && button.planKey && button.billingCycle;
      });
      return '<article class="billing-upgrade-plan-card' + (plan.highlight ? " highlight" : "") + '">' + (plan.badge ? '<div class="billing-upgrade-plan-badge">' + escapeText(plan.badge) + "</div>" : "") + '<div class="billing-upgrade-plan-header"><div class="billing-upgrade-plan-kicker">' + escapeText(plan.tierKicker) + "</div><h4>" + escapeText(plan.title) + '</h4><div class="billing-upgrade-plan-price">' + escapeText(plan.price) + '</div><div class="billing-upgrade-plan-period">' + escapeText(plan.period) + '</div></div><p class="billing-upgrade-plan-blurb">' + escapeText(plan.blurb) + '</p><ul class="billing-upgrade-plan-features">' + plan.features.map(function(feature) {
        return '<li class="billing-upgrade-plan-feature tone-' + escapeAttribute(feature.tone) + '"><span class="billing-upgrade-plan-feature-copy">' + String(feature.html || "") + "</span></li>";
      }).join("") + "</ul>" + (plan.note ? '<div class="helper-text">' + escapeText(plan.note) + "</div>" : "") + '<div class="form-actions">' + checkoutButtons.map(function(button) {
        return '<button type="button" class="' + escapeAttribute(button.className || "btn-primary") + '" data-account-billing-action="upgrade-start-checkout" data-upgrade-plan-key="' + escapeAttribute(button.planKey || "") + '" data-upgrade-tier="' + escapeAttribute(button.tier || "") + '" data-upgrade-billing-cycle="' + escapeAttribute(button.billingCycle || "") + '"' + (checkoutDisabled ? " disabled" : "") + ">" + escapeText(button.label) + "</button>";
      }).join("") + "</div></article>";
    }).join("") + "</div>";
  }
  function renderUpgradePanel(billingState, _bootstrap) {
    var root = getElement3("upgrade-billing-root");
    if (!root) return;
    var featureKey = String(billingState.upgradeFeatureKey || "").trim();
    var portalHandoffID = String(billingState.upgradePortalHandoffID || "").trim();
    var pricingState = billingState.upgradePricing;
    var handoffState = billingState.upgradePortalHandoff;
    var handoffLifecycle = String(handoffState.data && handoffState.data.status || "").trim();
    var explainer = pricingState.data && pricingState.data.explainer ? pricingState.data.explainer : "";
    var summaryItems = [];
    if (billingState.upgradeCheckout.pending) {
      summaryItems.push('<div class="billing-status visible">Redirecting to secure checkout...</div>');
    }
    if (billingState.upgradeCheckout.error) {
      summaryItems.push('<div class="billing-status visible error">' + escapeText(billingState.upgradeCheckout.error) + "</div>");
    }
    if (!portalHandoffID) {
      summaryItems.push(
        '<div class="billing-status visible error">Open this upgrade from the Plans page in Pulse so Pulse Account can verify the secure plan upgrade handoff before checkout.</div>'
      );
    } else if (handoffState.status === "loading") {
      summaryItems.push('<div class="billing-status visible">Verifying the secure plan upgrade handoff...</div>');
    } else if (handoffState.status === "error") {
      summaryItems.push('<div class="billing-status visible error">' + escapeText(handoffState.error || "Failed to verify the secure plan upgrade handoff.") + "</div>");
    } else if (handoffState.status === "ready") {
      if (handoffLifecycle === "completed") {
        summaryItems.push('<div class="billing-status visible success">This secure upgrade handoff already completed. Return to the Plans page in Pulse to review the live plan state.</div>');
      } else if (handoffLifecycle === "checkout_started") {
        summaryItems.push('<div class="billing-status visible">Secure checkout is already prepared for this upgrade. Continue below if you still need to reopen it.</div>');
      } else {
        summaryItems.push('<div class="billing-status visible success">Pulse Account will return completed checkout directly to the Plans page in Pulse.</div>');
      }
    }
    if (pricingState.status === "loading" && !pricingState.data) {
      summaryItems.push("<p>Loading self-hosted plan options...</p>");
    }
    if (pricingState.status === "error") {
      summaryItems.push(
        '<div class="billing-status visible error">' + escapeText(pricingState.error || "Failed to load self-hosted plans.") + '</div><div class="form-actions"><button type="button" class="btn-secondary" data-account-billing-action="upgrade-reload-pricing">Retry plan load</button></div>'
      );
    }
    if (explainer) {
      summaryItems.push('<div class="helper-text">' + explainer + "</div>");
    }
    root.innerHTML = '<div class="billing-upgrade-root">' + summaryItems.join("") + renderUpgradePlansHTML(billingState) + (pricingState.status === "ready" && pricingState.data && pricingState.data.description ? '<div class="helper-text">' + escapeText(pricingState.data.description) + "</div>" : "") + '<div class="helper-text">' + (featureKey === "self_hosted_plan" || featureKey === "max_monitored_systems" ? "Pulse Account keeps checkout tied to the Pulse instance that opened it, so completed Relay or Pro purchases return to the right Plans page automatically." : "Pulse Account compares self-hosted tiers and sends completed checkout straight back to the Plans page in Pulse.") + "</div></div>";
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
    var panels = [
      "upgrade-billing-panel",
      "manage-billing-panel",
      "retrieve-billing-panel",
      "refund-billing-panel",
      "data-billing-panel"
    ];
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
        case "upgrade-reload-pricing":
          event.preventDefault();
          deps.reloadUpgradePricing();
          return;
        case "upgrade-start-checkout":
          event.preventDefault();
          deps.startUpgradeCheckout(
            target.getAttribute("data-upgrade-plan-key") || "",
            target.getAttribute("data-upgrade-tier") || "",
            target.getAttribute("data-upgrade-billing-cycle") || ""
          );
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
        billingState.upgradeFeatureKey = nextState.upgradeFeatureKey;
        billingState.upgradePortalHandoffID = nextState.upgradePortalHandoffID;
        billingState.upgradePortalHandoff = nextState.upgradePortalHandoff;
        billingState.upgradePricing = nextState.upgradePricing;
        billingState.upgradeCheckout = nextState.upgradeCheckout;
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
      renderUpgrade();
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
    function renderUpgrade() {
      renderUpgradePanel(getBillingState(), store.getBootstrap());
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
    async function loadUpgradePricing(force) {
      var billingState = getBillingState();
      if (!force && (billingState.upgradePricing.status === "loading" || billingState.upgradePricing.status === "ready")) {
        return;
      }
      updateBillingState(function(nextBillingState) {
        beginQueryState(nextBillingState.upgradePricing, null);
      });
      try {
        var pricing = await api.getCommercialJSON("/v1/public/pricing-model?track=v6");
        updateBillingState(function(nextBillingState) {
          resolveQueryState(nextBillingState.upgradePricing, pricing);
        });
      } catch (err) {
        updateBillingState(function(nextBillingState) {
          failQueryState(
            nextBillingState.upgradePricing,
            null,
            err instanceof Error ? err.message : "Failed to load self-hosted plans."
          );
        });
      }
    }
    async function resolveUpgradePortalHandoff(force) {
      var billingState = getBillingState();
      var portalHandoffID = String(billingState.upgradePortalHandoffID || "").trim();
      if (!portalHandoffID) return;
      if (!force && (billingState.upgradePortalHandoff.status === "loading" || billingState.upgradePortalHandoff.status === "ready")) {
        return;
      }
      updateBillingState(function(nextBillingState) {
        beginQueryState(nextBillingState.upgradePortalHandoff, null);
      });
      try {
        var handoff = await api.getCommercialJSON(
          "/v1/checkout/portal-handoff?portal_handoff_id=" + encodeURIComponent(portalHandoffID)
        );
        updateBillingState(function(nextBillingState) {
          resolveQueryState(nextBillingState.upgradePortalHandoff, handoff);
          nextBillingState.upgradeFeatureKey = String(handoff.feature || "").trim();
        }, false);
      } catch (err) {
        updateBillingState(function(nextBillingState) {
          failQueryState(
            nextBillingState.upgradePortalHandoff,
            null,
            err instanceof Error ? err.message : "Failed to verify the secure plan upgrade handoff."
          );
        });
      }
    }
    async function startUpgradeCheckout(planKey, tier, billingCycle) {
      if (!planKey || !tier || !billingCycle) return;
      var billingState = getBillingState();
      var portalHandoffID = String(billingState.upgradePortalHandoffID || "").trim();
      var handoffLifecycle = String(
        billingState.upgradePortalHandoff.data && billingState.upgradePortalHandoff.data.status || ""
      ).trim();
      if (!portalHandoffID || billingState.upgradePortalHandoff.status !== "ready") {
        updateBillingState(function(nextBillingState) {
          failMutationState(
            nextBillingState.upgradeCheckout,
            "Pulse Account could not verify the secure upgrade handoff. Reopen the upgrade flow from the Plans page in Pulse."
          );
        });
        return;
      }
      if (handoffLifecycle === "completed") {
        updateBillingState(function(nextBillingState) {
          failMutationState(
            nextBillingState.upgradeCheckout,
            "This secure upgrade handoff already completed. Return to the Plans page in Pulse to review the live plan state."
          );
        });
        return;
      }
      updateBillingState(function(nextBillingState) {
        beginMutationState(nextBillingState.upgradeCheckout);
      });
      try {
        var data = await api.postCommercialJSON("/v1/checkout/session", {
          plan_key: planKey,
          tier,
          billing_cycle: billingCycle,
          portal_handoff_id: portalHandoffID
        });
        if (!data || !data.url) {
          throw new Error("Checkout URL was not returned.");
        }
        updateBillingState(function(nextBillingState) {
          succeedMutationState(nextBillingState.upgradeCheckout);
        });
        window.location.href = data.url;
      } catch (err) {
        updateBillingState(function(nextBillingState) {
          failMutationState(
            nextBillingState.upgradeCheckout,
            err instanceof Error ? err.message : "Failed to start checkout."
          );
        });
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
      var billingState = getBillingState();
      if (billingState.openBillingPanelID === "upgrade-billing-panel" || !!billingState.upgradeFeatureKey || !!billingState.upgradePortalHandoffID) {
        void loadUpgradePricing(false);
        void resolveUpgradePortalHandoff(false);
      }
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
      reloadUpgradePricing: function() {
        void loadUpgradePricing(true);
      },
      startUpgradeCheckout: function(planKey, tier, billingCycle) {
        void startUpgradeCheckout(planKey, tier, billingCycle);
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
  function hasSignupPath(signupPath) {
    return String(signupPath || "").trim() !== "";
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
  function accountKindLabel(account) {
    if (account.kind === "msp") return "MSP account";
    if (account.kind === "cloud") return "Cloud account";
    if (account.kind === "individual") return "Hosted account";
    return account.kind_label ? account.kind_label + " account" : "Account";
  }
  function accountUsesClientLanguage(account) {
    return account.kind === "msp";
  }
  function accountsUseClientLanguage(accounts) {
    return accounts.length > 0 && accounts.every(accountUsesClientLanguage);
  }
  function workspaceEntityName(clientLanguage, plural = false) {
    if (clientLanguage) return plural ? "clients" : "client";
    return plural ? "workspaces" : "workspace";
  }
  function workspaceCountLabel(count, clientLanguage = false) {
    return count === 1 ? "1 " + workspaceEntityName(clientLanguage) : String(count) + " " + workspaceEntityName(clientLanguage, true);
  }
  function accountCountLabel(count) {
    return count === 1 ? "1 account" : String(count) + " accounts";
  }
  function reviewWorkspaceChipLabel(count, clientLanguage = false) {
    return count === 1 ? "1 " + workspaceEntityName(clientLanguage) + " to review" : String(count) + " " + workspaceEntityName(clientLanguage, true) + " to review";
  }
  function readyWorkspaceChipLabel(count, clientLanguage = false) {
    return count === 1 ? "1 ready " + workspaceEntityName(clientLanguage) : String(count) + " ready " + workspaceEntityName(clientLanguage, true);
  }
  function suspendedWorkspaceChipLabel(count, clientLanguage = false) {
    return count === 1 ? "1 suspended " + workspaceEntityName(clientLanguage) : String(count) + " suspended " + workspaceEntityName(clientLanguage, true);
  }
  function setupNeededWorkspaceChipLabel(count, clientLanguage = false) {
    return count === 1 ? "1 " + workspaceEntityName(clientLanguage) + " in setup" : String(count) + " " + workspaceEntityName(clientLanguage, true) + " in setup";
  }
  function supportRunbookPathCopy(hasHostedAccounts2, hostedViewOnly, showSelfHostedCommercial, hasHostedBilling, clientLanguage = false) {
    var primarySection = clientLanguage ? "Clients" : "Workspaces";
    if (!hasHostedAccounts2) return "Billing, licenses, refunds, or privacy.";
    if (hostedViewOnly) {
      if (showSelfHostedCommercial && hasHostedBilling) return primarySection + ", Access review, owner/admin handoff, hosted billing, licenses, refunds, or privacy.";
      if (showSelfHostedCommercial) return primarySection + ", Access review, owner/admin handoff, licenses, refunds, or privacy.";
      if (hasHostedBilling) return primarySection + ", Access review, owner/admin handoff, or hosted billing.";
      return primarySection + ", Access review, or owner/admin handoff.";
    }
    if (showSelfHostedCommercial && hasHostedBilling) return primarySection + ", Access, hosted billing, licenses, refunds, or privacy.";
    if (showSelfHostedCommercial) return primarySection + ", Access, licenses, refunds, or privacy.";
    if (hasHostedBilling) return primarySection + ", Access, or hosted billing.";
    return primarySection + " or Access.";
  }
  function hasHostedAccounts(accounts) {
    return accounts.length > 0;
  }
  function hasHostedBillingAccounts(accounts) {
    return accounts.some(function(account) {
      return account.has_billing === true;
    });
  }
  function hasSelfHostedCommercial(bootstrap) {
    var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
    return bootstrap.has_self_hosted_commercial === true || !hasHostedAccounts(accounts);
  }
  function normalizeUpgradeFeatureKey(featureKey) {
    return String(featureKey || "").trim();
  }
  function isSelfHostedPlanUpgrade(featureKey) {
    var normalized = normalizeUpgradeFeatureKey(featureKey);
    return normalized === "self_hosted_plan" || normalized === "max_monitored_systems";
  }
  function selfHostedUpgradeActionTitle(featureKey) {
    return isSelfHostedPlanUpgrade(featureKey) ? "Compare self-hosted plans" : "Upgrade self-hosted plan";
  }
  function selfHostedUpgradeActionDescription(featureKey) {
    return isSelfHostedPlanUpgrade(featureKey) ? "Compare self-hosted plans as monitor, reach, or operate instead of by monitored-system volume." : "Compare self-hosted plans and continue into the commercial checkout path.";
  }
  function selfHostedUpgradeActionHighlights(featureKey) {
    return isSelfHostedPlanUpgrade(featureKey) ? ["Plan comparison", "Plan checkout"] : ["Plan comparison", "Checkout handoff"];
  }
  function renderSelfHostedUpgradeActionRow(context) {
    var featureKey = normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey);
    return renderBillingActionRow(
      "open-upgrade-billing",
      selfHostedUpgradeActionTitle(featureKey),
      "Open",
      selfHostedUpgradeActionDescription(featureKey),
      "upgrade-billing-panel",
      "upgrade-billing-link",
      selfHostedUpgradeActionHighlights(featureKey)
    );
  }
  function renderSelfHostedUpgradeBillingPanel(context) {
    var featureKey = normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey);
    var helperCopy = isSelfHostedPlanUpgrade(featureKey) ? "Choose the self-hosted tier that fits how you run Pulse: Community monitors, Relay reaches anywhere, and Pro investigates and helps fix issues. Pulse Account will send completed checkout directly back to the Plans page in Pulse." : "Choose the self-hosted tier that fits this upgrade. Pulse Account will send completed checkout directly back to the Plans page in Pulse.";
    return renderBillingTaskPanel(
      selfHostedUpgradeActionTitle(featureKey),
      "Pulse Account owns self-hosted plan selection and checkout for self-hosted upgrades.",
      "upgrade-billing-panel",
      '<div id="upgrade-billing-root"></div><div class="helper-text">' + escapeHTML(helperCopy) + "</div>"
    );
  }
  function countWorkspaces(accounts) {
    var total = 0;
    for (var i = 0; i < accounts.length; i += 1) {
      total += Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces.length : 0;
    }
    return total;
  }
  function collectWorkspaceSummaryEntries(accounts) {
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
      if (String(workspaces[i].state || "") === "active" && workspaceSetupState(workspaces[i]) === "ready") {
        count += 1;
      }
    }
    return count;
  }
  function healthBadgeHTML(workspace) {
    var status = workspaceHealthState(workspace);
    if (status === "healthy") {
      return '<span class="badge badge-healthy">' + escapeHTML(workspaceHealthLabel(workspace)) + "</span>";
    }
    if (status === "unhealthy") {
      return '<span class="badge badge-unhealthy">' + escapeHTML(workspaceHealthLabel(workspace)) + "</span>";
    }
    return '<span class="badge badge-checking">' + escapeHTML(workspaceHealthLabel(workspace)) + "</span>";
  }
  function setupBadgeHTML(workspace) {
    var setup = workspaceSetupState(workspace);
    return '<span class="badge badge-setup-' + escapeHTML(setup) + '">' + escapeHTML(workspaceSetupLabel(workspace)) + "</span>";
  }
  function renderBillingActionRow(id, title, actionLabel, description, panelID, focusID, highlights) {
    var meta = escapeHTML(highlights.join(" \u2022 "));
    return '<article class="billing-action-row"><div class="billing-action-main"><div class="billing-action-copy"><h3>' + title + "</h3><p>" + description + '</p></div><div class="billing-action-meta">' + meta + '</div></div><div class="billing-action-cta"><button class="btn-secondary billing-action-button" type="button" id="' + id + '" data-account-billing-action="open-billing-panel" data-account-billing-panel="' + panelID + '" data-account-billing-focus="' + focusID + '" data-shell-target="billing">' + escapeHTML(actionLabel) + "</button></div></article>";
  }
  function renderSectionContextChips(chips) {
    return "";
  }
  function renderFactLine(className, facts) {
    if (!facts.length) return "";
    return '<div class="' + className + '">' + facts.map(function(fact) {
      return "<span>" + escapeHTML(fact) + "</span>";
    }).join('<span class="portal-fact-separator">\u2022</span>') + "</div>";
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
  function renderIdentityBar(accounts, showSelfHostedCommercial) {
    if (accounts.length === 1) {
      var account = accounts[0];
      return '<div class="portal-identity-bar"><h2>' + escapeHTML(account.name) + '</h2><span class="portal-identity-sep">\xB7</span><span>' + escapeHTML(portalRoleLabel(account.role)) + '</span><span class="portal-identity-sep">\xB7</span><span>' + escapeHTML(accountKindLabel(account)) + "</span></div>";
    }
    if (accounts.length > 1) {
      return '<div class="portal-identity-bar"><h2>Pulse Account</h2><span class="portal-identity-sep">\xB7</span><span>' + String(accounts.length) + " accounts</span></div>";
    }
    return '<div class="portal-identity-bar"><h2>Pulse Account</h2><span class="portal-identity-sep">\xB7</span><span>' + escapeHTML(showSelfHostedCommercial ? "Self-hosted billing" : "Billing") + "</span></div>";
  }
  function primaryShellSections(bootstrap) {
    var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
    var hosted = hasHostedAccounts(accounts);
    var showSelfHostedCommercial = hasSelfHostedCommercial(bootstrap);
    var hasHostedBilling = hasHostedBillingAccounts(accounts);
    var sections = [];
    if (hosted) {
      sections.push({ section: "workspaces", title: accountsUseClientLanguage(accounts) ? "Clients" : "Workspaces" });
      sections.push({ section: "access", title: "Access" });
    }
    if (hasHostedBilling || showSelfHostedCommercial) {
      sections.push({ section: "billing", title: "Billing" });
    }
    return sections;
  }
  function utilityShellSections(_bootstrap) {
    return [{ section: "support", title: "Support" }];
  }
  function visibleShellSections(bootstrap) {
    return primaryShellSections(bootstrap).concat(utilityShellSections(bootstrap));
  }
  function renderTabBar(bootstrap, activeSection) {
    var sections = visibleShellSections(bootstrap);
    return '<nav class="portal-tab-bar" aria-label="Pulse Account sections">' + sections.map(function(entry) {
      var isActive = activeSection === entry.section;
      var cls = "portal-tab" + (isActive ? " active" : "") + (entry.section === "support" ? " portal-tab-utility" : "");
      return '<button class="' + cls + '" type="button" data-shell-action="activate-section" data-shell-section="' + entry.section + '">' + entry.title + "</button>";
    }).join("") + "</nav>";
  }
  function workspaceListAnchorID(accountID) {
    return "workspace-list-" + accountID;
  }
  function workspaceRowAnchorID(accountID, workspaceID) {
    return "workspace-row-" + accountID + "-" + workspaceID;
  }
  var WORKSPACE_INSTALL_TARGET_PATH2 = "/settings/infrastructure?add=linux-host";
  var WORKSPACE_REPORTING_TARGET_PATH2 = "/settings/support/reporting";
  function workspaceHandoffActionPath2(accountAPIBasePath, accountID, workspaceID, targetPath = "") {
    var path = accountAPIBasePath + "/" + encodeURIComponent(accountID) + "/tenants/" + encodeURIComponent(workspaceID) + "/handoff";
    if (!targetPath) return path;
    return path + "?target_path=" + encodeURIComponent(targetPath);
  }
  function renderWorkspaceCard(account, workspace, accountAPIBasePath) {
    var status = workspaceHealthState(workspace);
    var state = String(workspace.state || "");
    var clientLanguage = accountUsesClientLanguage(account);
    var createdLabel = formatWorkspaceDate(workspace.created_at);
    var metaParts = [];
    if (state) {
      metaParts.push('<span class="workspace-meta-item">' + escapeHTML(titleCase(state)) + "</span>");
    }
    if (createdLabel) {
      metaParts.push('<span class="workspace-meta-item">Created ' + escapeHTML(createdLabel) + "</span>");
    }
    var openAction = "";
    if (state === "active") {
      openAction = '<form method="POST" action="' + escapeAttr(workspaceHandoffActionPath2(accountAPIBasePath, account.id, workspace.id)) + '"><button type="submit" class="btn-primary">' + escapeHTML(clientLanguage ? "Open client" : "Open workspace") + "</button></form>";
    }
    var installAction = "";
    if (account.can_manage && state === "active") {
      installAction = '<form method="POST" action="' + escapeAttr(workspaceHandoffActionPath2(accountAPIBasePath, account.id, workspace.id, WORKSPACE_INSTALL_TARGET_PATH2)) + '"><button type="submit" class="btn-secondary">Install agents</button></form>';
    }
    var manageAction = "";
    if (account.can_manage && (state === "active" || state === "suspended" || state === "failed")) {
      manageAction = '<button type="button" class="btn-secondary btn-workspace-manage" data-action="select-workspace" data-account-id="' + escapeAttr(account.id) + '" data-workspace-id="' + escapeAttr(workspace.id) + '">' + escapeHTML(clientLanguage ? "Client onboarding" : "Setup checklist") + "</button>";
    }
    return '<article class="workspace-row workspace-row-health-' + escapeAttr(status) + " workspace-row-state-" + escapeAttr(state || "unknown") + '" id="' + escapeAttr(workspaceRowAnchorID(account.id, workspace.id)) + '" data-workspace-row="' + escapeAttr(workspace.id) + '"><div class="workspace-row-primary"><div class="workspace-row-heading"><h4 class="workspace-name">' + escapeHTML(workspace.display_name) + "</h4></div>" + (metaParts.length ? '<div class="workspace-meta">' + metaParts.join("") + "</div>" : "") + '</div><div class="workspace-row-status-cell workspace-row-status-cell-badge">' + setupBadgeHTML(workspace) + '</div><div class="workspace-row-status-cell workspace-row-status-cell-badge">' + healthBadgeHTML(workspace) + '</div><div class="workspace-actions">' + openAction + installAction + manageAction + "</div></article>";
  }
  function renderWorkspaceHandoffForm(accountID, workspaceID, accountAPIBasePath, label, buttonClassName = "btn-secondary btn-compact") {
    if (!accountAPIBasePath) {
      return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + "</button>";
    }
    return '<form method="POST" action="' + escapeAttr(workspaceHandoffActionPath2(accountAPIBasePath, accountID, workspaceID)) + '"><button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + "</button></form>";
  }
  function renderWorkspaceInstallHandoffForm(accountID, workspaceID, accountAPIBasePath, label = "Install agents", buttonClassName = "btn-secondary btn-compact") {
    if (!accountAPIBasePath) {
      return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + "</button>";
    }
    return '<form method="POST" action="' + escapeAttr(workspaceHandoffActionPath2(accountAPIBasePath, accountID, workspaceID, WORKSPACE_INSTALL_TARGET_PATH2)) + '"><button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + "</button></form>";
  }
  function renderWorkspaceReportingHandoffForm(accountID, workspaceID, accountAPIBasePath, label = "Open reports", buttonClassName = "btn-secondary btn-compact") {
    if (!accountAPIBasePath) {
      return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + "</button>";
    }
    return '<form method="POST" action="' + escapeAttr(workspaceHandoffActionPath2(accountAPIBasePath, accountID, workspaceID, WORKSPACE_REPORTING_TARGET_PATH2)) + '"><button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + "</button></form>";
  }
  function attentionWorkspaceEntries(entries) {
    var results = [];
    for (var i = 0; i < entries.length; i += 1) {
      var status = workspaceHealthState(entries[i].workspace);
      if (status === "unhealthy" || status === "checking") {
        results.push(entries[i]);
      }
    }
    return results;
  }
  function readyWorkspaceEntries(entries) {
    var results = [];
    for (var i = 0; i < entries.length; i += 1) {
      if (String(entries[i].workspace.state || "") === "active" && workspaceSetupState(entries[i].workspace) === "ready") {
        results.push(entries[i]);
      }
    }
    return results;
  }
  function suspendedWorkspaceEntries(entries) {
    var results = [];
    for (var i = 0; i < entries.length; i += 1) {
      if (String(entries[i].workspace.state || "") === "suspended") {
        results.push(entries[i]);
      }
    }
    return results;
  }
  function setupNeededWorkspaceEntries(entries) {
    var results = [];
    for (var i = 0; i < entries.length; i += 1) {
      if (String(entries[i].workspace.state || "") !== "active") continue;
      if (workspaceHealthState(entries[i].workspace) !== "healthy") continue;
      var setup = workspaceSetupState(entries[i].workspace);
      if (setup === "install_agents" || setup === "configure_outputs" || setup === "setup_path") {
        results.push(entries[i]);
      }
    }
    return results;
  }
  function setupFactCountLabel(value, singular, plural) {
    if (typeof value !== "number" || !Number.isFinite(value)) return "Unknown " + plural;
    return String(value) + " " + (value === 1 ? singular : plural);
  }
  function workspaceSetupFactsLine(workspace) {
    return [
      setupFactCountLabel(workspace.agent_count, "agent", "agents"),
      setupFactCountLabel(workspace.alert_route_count, "alert route", "alert routes"),
      setupFactCountLabel(workspace.report_schedule_count, "report schedule", "report schedules")
    ].join(" \xB7 ");
  }
  function workspaceSetupDiagnosticsLine(workspace) {
    var guide = workspaceSetupGuide(workspace);
    return guide.diagnostics.length ? guide.diagnostics[0] : workspaceSetupNextStep(workspace);
  }
  function workspaceSummaryStatusCopy(workspace, clientLanguage) {
    if (!clientLanguage) return workspaceStatusCopy(workspace);
    var state = String(workspace.state || "");
    if (state === "suspended") return "This client is suspended.";
    if (state === "failed") return "This client is in a failed state.";
    return workspaceStatusCopy(workspace);
  }
  function workspaceSummaryContext(entry, includeAccountName, note) {
    if (!includeAccountName) return note;
    return entry.account.name + " \xB7 " + note;
  }
  function renderWorkspaceAnchorAction(anchorID, label, className = "btn-secondary btn-compact workspace-summary-link") {
    return '<a class="' + escapeAttr(className) + '" href="#' + escapeAttr(anchorID) + '">' + escapeHTML(label) + "</a>";
  }
  function renderWorkspaceSummaryDecision(accounts, entries, accountAPIBasePath, showSelfHostedCommercial) {
    var clientLanguage = accountsUseClientLanguage(accounts);
    var attention = attentionWorkspaceEntries(entries);
    var suspended = suspendedWorkspaceEntries(entries);
    var setupNeeded = setupNeededWorkspaceEntries(entries);
    var ready = readyWorkspaceEntries(entries);
    var primaryAction = "";
    var secondaryAction = "";
    var title = "";
    var description = "";
    var totalWorkspaces = countWorkspaces(accounts);
    var creatableAccount = accounts.find(function(account) {
      return account.kind === "msp" && account.can_manage;
    }) || null;
    var accessAccount = accounts.find(function(account) {
      return account.can_manage;
    }) || null;
    var hostedViewOnly = accounts.length > 0 && !accessAccount;
    if (attention.length) {
      var attentionEntry = attention[0];
      title = "Review " + attentionEntry.workspace.display_name;
      description = workspaceSummaryContext(attentionEntry, accounts.length > 1, workspaceSummaryStatusCopy(attentionEntry.workspace, clientLanguage));
      primaryAction = renderWorkspaceAnchorAction(
        workspaceRowAnchorID(attentionEntry.account.id, attentionEntry.workspace.id),
        clientLanguage ? "Review client" : "Review workspace",
        "btn-primary btn-compact workspace-summary-link"
      );
      secondaryAction = attentionEntry.account.can_manage && (attentionEntry.workspace.state === "active" || attentionEntry.workspace.state === "suspended" || attentionEntry.workspace.state === "failed") ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' + escapeAttr(attentionEntry.account.id) + '" data-workspace-id="' + escapeAttr(attentionEntry.workspace.id) + '">' + escapeHTML(clientLanguage ? "Client onboarding" : "Setup checklist") + "</button>" : '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
    } else if (suspended.length) {
      var suspendedEntry = suspended[0];
      title = "Review " + suspendedEntry.workspace.display_name;
      description = workspaceSummaryContext(suspendedEntry, accounts.length > 1, workspaceSummaryStatusCopy(suspendedEntry.workspace, clientLanguage));
      primaryAction = renderWorkspaceAnchorAction(
        workspaceRowAnchorID(suspendedEntry.account.id, suspendedEntry.workspace.id),
        clientLanguage ? "Review client" : "Review workspace",
        "btn-primary btn-compact workspace-summary-link"
      );
      secondaryAction = suspendedEntry.account.can_manage ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' + escapeAttr(suspendedEntry.account.id) + '" data-workspace-id="' + escapeAttr(suspendedEntry.workspace.id) + '">' + escapeHTML(clientLanguage ? "Client onboarding" : "Setup checklist") + "</button>" : '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
    } else if (setupNeeded.length) {
      var setupEntry = setupNeeded[0];
      var setupState = workspaceSetupState(setupEntry.workspace);
      title = setupState === "configure_outputs" ? "Configure outputs for " + setupEntry.workspace.display_name : "Set up " + setupEntry.workspace.display_name;
      description = workspaceSummaryContext(setupEntry, accounts.length > 1, workspaceSetupNextStep(setupEntry.workspace));
      if (!setupEntry.account.can_manage) {
        primaryAction = renderWorkspaceHandoffForm(setupEntry.account.id, setupEntry.workspace.id, accountAPIBasePath, clientLanguage ? "Open client" : "Open workspace", "btn-primary btn-compact");
      } else if (setupState === "configure_outputs") {
        primaryAction = renderWorkspaceReportingHandoffForm(
          setupEntry.account.id,
          setupEntry.workspace.id,
          accountAPIBasePath,
          "Open reports",
          "btn-primary btn-compact"
        );
      } else {
        primaryAction = renderWorkspaceInstallHandoffForm(
          setupEntry.account.id,
          setupEntry.workspace.id,
          accountAPIBasePath,
          "Install agents",
          "btn-primary btn-compact"
        );
      }
      secondaryAction = setupEntry.account.can_manage ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' + escapeAttr(setupEntry.account.id) + '" data-workspace-id="' + escapeAttr(setupEntry.workspace.id) + '">' + escapeHTML(clientLanguage ? "Client onboarding" : "Setup checklist") + "</button>" : renderWorkspaceHandoffForm(setupEntry.account.id, setupEntry.workspace.id, accountAPIBasePath, clientLanguage ? "Open client" : "Open workspace");
    } else if (ready.length) {
      var readyEntry = ready[0];
      title = "Open " + readyEntry.workspace.display_name;
      description = workspaceSummaryContext(readyEntry, accounts.length > 1, workspaceRowNote(readyEntry.workspace));
      primaryAction = renderWorkspaceHandoffForm(
        readyEntry.account.id,
        readyEntry.workspace.id,
        accountAPIBasePath,
        clientLanguage ? "Open client" : "Open workspace",
        "btn-primary btn-compact"
      );
      secondaryAction = ready.length > 1 ? renderWorkspaceAnchorAction(workspaceListAnchorID(readyEntry.account.id), clientLanguage ? "See all clients" : "See all workspaces") : readyEntry.account.can_manage ? renderWorkspaceInstallHandoffForm(readyEntry.account.id, readyEntry.workspace.id, accountAPIBasePath) : "";
    } else if (creatableAccount) {
      title = clientLanguage ? "Add the first client" : "Create the first workspace";
      description = clientLanguage ? "No client is attached yet. Add the first client in " + creatableAccount.name + "." : "No hosted workspace is attached yet. Create the first workspace in " + creatableAccount.name + ".";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(creatableAccount.id) + '">' + escapeHTML(clientLanguage ? "Add client" : "Create workspace") + "</button>";
      secondaryAction = accessAccount ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>' : "";
    } else if (hostedViewOnly) {
      if (entries.length > 0) {
        title = "Review who can act";
        description = clientLanguage ? "Clients are attached here, but an owner or admin must make account-level changes." : "Hosted workspaces are attached here, but an owner or admin must make account-level changes.";
        primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
        secondaryAction = renderWorkspaceAnchorAction(workspaceListAnchorID(accounts[0].id), clientLanguage ? "Review client list" : "Review workspace list");
      } else {
        title = "Review who can act";
        description = clientLanguage ? "No client is attached yet. Review Access to see who can add or manage clients on this account." : showSelfHostedCommercial ? "No hosted workspace is attached. Review Access to see who can manage this hosted account, or use Billing for self-hosted tasks." : "No hosted workspace is attached yet. Review Access to see who can create or manage the first workspace on this account.";
        primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
        secondaryAction = showSelfHostedCommercial ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' : "";
      }
    } else if (accessAccount) {
      title = "Open access";
      description = "Use Access for invites, role changes, or access removal.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
      secondaryAction = showSelfHostedCommercial ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' : "";
    } else {
      title = "Open billing or support";
      description = totalWorkspaces > 0 ? "Review the " + workspaceEntityName(clientLanguage) + " list here, then use Billing for commercial work or Support only after a self-service path fails." : "Use Billing for commercial work. Use Support only after the billing path fails.";
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
      secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="support">Escalate</button>';
    }
    return {
      title,
      description,
      primaryAction,
      secondaryAction
    };
  }
  function renderWorkspaceSummaryFacts(accounts, entries) {
    var clientLanguage = accountsUseClientLanguage(accounts);
    return [
      accountCountLabel(accounts.length),
      workspaceCountLabel(entries.length, clientLanguage),
      readyWorkspaceChipLabel(readyWorkspaceEntries(entries).length, clientLanguage),
      setupNeededWorkspaceChipLabel(setupNeededWorkspaceEntries(entries).length, clientLanguage),
      reviewWorkspaceChipLabel(attentionWorkspaceEntries(entries).length, clientLanguage),
      suspendedWorkspaceChipLabel(suspendedWorkspaceEntries(entries).length, clientLanguage)
    ];
  }
  function renderWorkspaceSummaryInline(accounts, entries, accountAPIBasePath, showSelfHostedCommercial) {
    var decision = renderWorkspaceSummaryDecision(accounts, entries, accountAPIBasePath, showSelfHostedCommercial);
    return '<section class="workspace-summary-inline"><div class="workspace-summary-inline-copy"><p><strong>Next:</strong> ' + escapeHTML(decision.title) + "</p><p>" + escapeHTML(decision.description) + '</p></div><div class="workspace-summary-actions">' + decision.primaryAction + decision.secondaryAction + "</div></section>";
  }
  function renderWorkspaceSetupQueueAction(entry, accountAPIBasePath) {
    var setup = workspaceSetupState(entry.workspace);
    if (setup === "configure_outputs") {
      return renderWorkspaceReportingHandoffForm(
        entry.account.id,
        entry.workspace.id,
        accountAPIBasePath,
        "Configure outputs",
        "btn-primary btn-compact"
      );
    }
    return renderWorkspaceInstallHandoffForm(
      entry.account.id,
      entry.workspace.id,
      accountAPIBasePath,
      setup === "install_agents" ? "Install agents" : "Open setup",
      "btn-primary btn-compact"
    );
  }
  function renderProviderSetupTemplates(accounts) {
    var templateAccount = accounts.find(function(account) {
      return account.kind === "msp" && Array.isArray(account.setup_templates) && account.setup_templates.length > 0;
    });
    if (!templateAccount || !templateAccount.setup_templates || !templateAccount.setup_templates.length) return "";
    var template = templateAccount.setup_templates[0];
    return '<section class="workspace-template-panel" aria-label="Provider setup template"><details class="workspace-template-details"><summary class="workspace-template-summary"><span class="workspace-template-summary-title">' + escapeHTML(template.title || "Provider setup template") + '</span><span class="workspace-template-summary-meta">' + escapeHTML(templateAccount.name) + '</span></summary><p class="workspace-template-intro">Use the same onboarding shape for each client, then finish the workspace-owned configuration inside that isolated client boundary.</p><div class="workspace-template-grid"><div><strong>Agent naming</strong><span>' + escapeHTML(template.agent_naming) + "</span></div><div><strong>Alert routing</strong><span>" + escapeHTML(template.alert_routing) + "</span></div><div><strong>Reports</strong><span>" + escapeHTML(template.reporting) + "</span></div><div><strong>Access</strong><span>" + escapeHTML(template.access) + "</span></div></div></details></section>";
  }
  function renderWorkspaceSetupQueue(entries, accountAPIBasePath, clientLanguage = false) {
    var setupNeeded = setupNeededWorkspaceEntries(entries);
    if (!setupNeeded.length) return "";
    var visible = setupNeeded.slice(0, 5);
    return '<section class="workspace-setup-queue" aria-label="' + escapeAttr(clientLanguage ? "Client onboarding queue" : "Unfinished workspace setup") + '"><div class="workspace-setup-queue-header"><div><h3>' + escapeHTML(clientLanguage ? "Clients in setup" : "Unfinished setup") + "</h3><p>" + escapeHTML(clientLanguage ? "Clients stay here until agents, alert routing, and reports are in place." : "Client workspaces stay here until agents, alert routing, and reports are in place.") + "</p></div><span>" + escapeHTML(setupNeededWorkspaceChipLabel(setupNeeded.length, clientLanguage)) + '</span></div><div class="workspace-setup-queue-list">' + visible.map(function(entry) {
      return '<article class="workspace-setup-queue-row"><div class="workspace-setup-queue-main">' + setupBadgeHTML(entry.workspace) + "<div><strong>" + escapeHTML(entry.workspace.display_name) + "</strong><span>" + escapeHTML(entry.account.name + " \xB7 " + workspaceSetupDiagnosticsLine(entry.workspace)) + "</span><small>" + escapeHTML(workspaceSetupFactsLine(entry.workspace)) + '</small></div></div><div class="workspace-setup-queue-actions">' + renderWorkspaceSetupQueueAction(entry, accountAPIBasePath) + (entry.account.can_manage ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' + escapeAttr(entry.account.id) + '" data-workspace-id="' + escapeAttr(entry.workspace.id) + '">' + escapeHTML(clientLanguage ? "Onboarding" : "Checklist") + "</button>" : renderWorkspaceHandoffForm(entry.account.id, entry.workspace.id, accountAPIBasePath, clientLanguage ? "Open client" : "Open workspace")) + "</div></article>";
    }).join("") + "</div></section>";
  }
  function workspaceSectionHeaderCopy(accounts, entries) {
    var canManageAnyWorkspace = accounts.some(function(account) {
      return account.can_manage;
    });
    var clientLanguage = accountsUseClientLanguage(accounts);
    if (!entries.length) {
      if (clientLanguage) {
        return canManageAnyWorkspace ? "Add clients here, then use onboarding to finish agents, alert routing, and reports." : "Review client state here. An owner or admin must add or change clients.";
      }
      return canManageAnyWorkspace ? "Review hosted workspaces here, then create the next workspace when you are ready." : "Review hosted workspace state here. An owner or admin must create or change hosted workspaces.";
    }
    if (!canManageAnyWorkspace) {
      return clientLanguage ? "Review client health here and open ready clients. An owner or admin must handle onboarding and client changes." : "Review hosted workspace health here and open ready workspaces. An owner or admin must handle setup and workspace changes.";
    }
    return clientLanguage ? "Review clients here, use Client onboarding for setup, and keep destructive client actions separate from daily monitoring. Each client remains an isolated workspace." : "Review client workspaces here, use the setup checklist for onboarding, and keep destructive workspace actions separate from daily workspace work.";
  }
  function renderWorkspaceSummarySection(context) {
    var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
    var entries = collectWorkspaceSummaryEntries(accounts);
    var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
    var clientLanguage = accountsUseClientLanguage(accounts);
    return '<section class="workspace-summary-shell"><div class="portal-page-header"><h2>' + escapeHTML(clientLanguage ? "Clients" : "Workspaces") + "</h2><p>" + escapeHTML(workspaceSectionHeaderCopy(accounts, entries)) + "</p></div>" + renderFactLine("workspace-summary-facts", renderWorkspaceSummaryFacts(accounts, entries)) + renderWorkspaceSummaryInline(accounts, entries, context.accountAPIBasePath, showSelfHostedCommercial) + renderProviderSetupTemplates(accounts) + renderWorkspaceSetupQueue(entries, context.accountAPIBasePath, clientLanguage) + "</section>";
  }
  function renderAccountSurfaceHeader(account, showHeader) {
    if (!showHeader) return "";
    return '<div class="portal-section-header account-surface-header"><div><h3>' + escapeHTML(account.name) + '</h3><p class="portal-section-copy">' + escapeHTML(accountKindLabel(account) + " \xB7 " + portalRoleLabel(account.role)) + "</p></div></div>";
  }
  function renderNoHostedWorkspacesSection() {
    return '<section class="account-content-panel account-content-panel-workspaces"><div class="empty-state"><p>No hosted workspaces are attached to this account. Use Billing for self-hosted subscriptions and licenses.</p></div></section>';
  }
  function renderNoHostedAccessSection() {
    return '<section class="account-content-panel account-content-panel-access"><div class="empty-state"><p>No hosted account roster is attached. Use Billing for commercial access to licenses, refunds, or privacy.</p></div></section>';
  }
  function renderAccountWorkspaceSection(account, accountAPIBasePath) {
    var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
    var clientLanguage = accountUsesClientLanguage(account);
    var addEntityLabel = clientLanguage ? "Add client" : "Create workspace";
    var openEntityLabel = clientLanguage ? "Open client" : "Open workspace";
    var onboardingLabel = clientLanguage ? "Client onboarding" : "Setup checklist";
    var readyCount = countReadyWorkspaces(workspaces);
    var attentionCount = attentionWorkspaces(workspaces).length;
    var suspendedCount = countWorkspacesByState(workspaces, "suspended");
    var workspaceListSummary = account.can_manage ? clientLanguage ? "Open a client to work inside its isolated workspace boundary, or use Client onboarding to finish setup." : "Open a workspace to work in it, or use Setup checklist when onboarding a client." : "Open a workspace here. An owner or admin must create or change hosted workspaces.";
    var workspaceManagementEmptyNote = account.has_billing ? "Access changes stay in Access. Billing changes stay in Billing." : clientLanguage ? "Access changes stay in Access. Client runtime changes stay inside the client workspace." : "Access changes stay in Access. Workspace runtime changes stay inside the workspace.";
    var workspaceManagement = "";
    var addWorkspaceForm = "";
    var workspaceHeaderActions = "";
    if (account.can_manage) {
      if (account.kind === "msp") {
        workspaceHeaderActions += '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">' + escapeHTML(addEntityLabel) + "</button>";
      }
      if (account.kind === "msp") {
        addWorkspaceForm = '<div class="add-workspace-form" id="add-ws-form-' + escapeAttr(account.id) + '"><label for="ws-name-' + escapeAttr(account.id) + '">' + escapeHTML(clientLanguage ? "Client name" : "Workspace name (for example, a client name)") + '</label><input type="text" id="ws-name-' + escapeAttr(account.id) + '" placeholder="Acme Corp" maxlength="80" autocomplete="off"><div class="form-actions"><button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' + escapeAttr(account.id) + '">' + escapeHTML(addEntityLabel) + '</button><button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Cancel</button><div class="spinner" id="ws-spinner-' + escapeAttr(account.id) + '" hidden></div></div></div>';
      }
      workspaceManagement = '<section class="workspace-management-panel workspace-management-panel-idle" id="workspace-management-' + escapeAttr(account.id) + '" hidden><div class="workspace-management-header"><div><h3>' + escapeHTML(onboardingLabel) + "</h3><p>" + escapeHTML(clientLanguage ? "Finish setup while each client keeps a separate workspace boundary." : "Finish the client setup steps without mixing client data across workspaces.") + '</p></div><button type="button" class="btn-secondary btn-compact" id="workspace-management-close-' + escapeAttr(account.id) + '" data-action="clear-workspace-selection" data-account-id="' + escapeAttr(account.id) + '">Close panel</button></div><div class="workspace-management-empty" id="workspace-management-empty-' + escapeAttr(account.id) + '"><div class="workspace-management-empty-shell"><div class="workspace-management-empty-actions-card"><div class="workspace-management-empty-actions-copy"><h4>' + escapeHTML(clientLanguage ? "Add a client" : "Create a workspace") + "</h4><p>" + escapeHTML(clientLanguage ? "Create an isolated client workspace for a customer." : "Add a new hosted workspace for a customer or operating boundary.") + "</p></div>" + addWorkspaceForm + '</div><div class="workspace-management-empty-note">' + escapeHTML(workspaceManagementEmptyNote) + '</div></div></div><div class="workspace-management-content" id="workspace-management-content-' + escapeAttr(account.id) + '" hidden><div class="workspace-management-meta" id="workspace-management-meta-' + escapeAttr(account.id) + '"></div><h4 id="workspace-management-title-' + escapeAttr(account.id) + '"></h4><p class="workspace-management-summary" id="workspace-management-summary-' + escapeAttr(account.id) + '"></p><div class="workspace-management-facts"><div class="workspace-management-fact"><span>Health</span><strong id="workspace-management-health-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Setup</span><strong id="workspace-management-setup-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Agents</span><strong id="workspace-management-agents-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Alert routes</span><strong id="workspace-management-alerts-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Report schedules</span><strong id="workspace-management-reports-' + escapeAttr(account.id) + '"></strong></div><div class="workspace-management-fact"><span>Created</span><strong id="workspace-management-created-' + escapeAttr(account.id) + '"></strong></div></div><div class="workspace-management-guidance" id="workspace-management-guidance-' + escapeAttr(account.id) + '"></div><div class="workspace-management-identity" id="workspace-management-identity-' + escapeAttr(account.id) + '"></div><div class="workspace-setup-guide" aria-label="' + escapeAttr(clientLanguage ? "Guided client onboarding" : "Guided workspace setup") + '"><div class="workspace-setup-guide-copy"><span>Current step</span><h4 id="workspace-management-guide-title-' + escapeAttr(account.id) + '"></h4><p id="workspace-management-guide-description-' + escapeAttr(account.id) + '"></p><ul id="workspace-management-guide-diagnostics-' + escapeAttr(account.id) + '"></ul></div><form method="POST" id="workspace-management-primary-form-' + escapeAttr(account.id) + '"><button type="submit" class="btn-primary btn-compact" id="workspace-management-primary-' + escapeAttr(account.id) + '">Continue setup</button></form></div><div class="workspace-setup-checklist" aria-label="' + escapeAttr(clientLanguage ? "Client onboarding checklist" : "Workspace setup checklist") + '"><div class="workspace-setup-step workspace-setup-step-created"><span class="workspace-setup-status" id="workspace-management-check-created-' + escapeAttr(account.id) + '"></span><div><strong>' + escapeHTML(clientLanguage ? "Client added" : "Workspace created") + "</strong><span>" + escapeHTML(clientLanguage ? "A separate workspace boundary keeps this client isolated." : "This client has a separate workspace boundary.") + '</span></div></div><div class="workspace-setup-step"><span class="workspace-setup-status" id="workspace-management-check-install-' + escapeAttr(account.id) + '"></span><div><strong>Install the first agent</strong><span>Use the workspace-bound install path so data lands in this client.</span></div></div><div class="workspace-setup-step"><span class="workspace-setup-status" id="workspace-management-check-alerts-' + escapeAttr(account.id) + '"></span><div><strong>Configure alert routes</strong><span>Keep notifications scoped to this client.</span></div></div><div class="workspace-setup-step"><span class="workspace-setup-status" id="workspace-management-check-reports-' + escapeAttr(account.id) + '"></span><div><strong>Schedule reports</strong><span>Send client performance reports from this workspace.</span></div></div><div class="workspace-setup-step"><span class="workspace-setup-status" id="workspace-management-check-access-' + escapeAttr(account.id) + '"></span><div><strong>Review access</strong><span>Invite provider staff or client users from Access.</span></div></div></div><div class="workspace-management-next-steps" id="workspace-management-next-steps-' + escapeAttr(account.id) + '"><div class="workspace-next-step"><div><strong>' + escapeHTML(openEntityLabel) + "</strong><span>" + escapeHTML(clientLanguage ? "Work inside this client workspace boundary." : "Work inside this client boundary.") + '</span></div><form method="POST" id="workspace-management-open-form-' + escapeAttr(account.id) + '"><button type="submit" class="btn-primary btn-compact" id="workspace-management-open-' + escapeAttr(account.id) + '">' + escapeHTML(openEntityLabel) + '</button></form></div><div class="workspace-next-step"><div><strong>Install agents</strong><span>Open the workspace-bound install commands.</span></div><form method="POST" id="workspace-management-install-form-' + escapeAttr(account.id) + '"><button type="submit" class="btn-secondary btn-compact" id="workspace-management-install-' + escapeAttr(account.id) + '">Install agents</button></form></div><div class="workspace-next-step workspace-next-step-readonly"><div><strong>Alerts and reports</strong><span>Alerts and performance reports stay inside the client workspace.</span></div><form method="POST" id="workspace-management-reporting-form-' + escapeAttr(account.id) + '"><button type="submit" class="btn-secondary btn-compact" id="workspace-management-reporting-' + escapeAttr(account.id) + '">Open reports</button></form></div><div class="workspace-next-step workspace-next-step-readonly"><div><strong>Access</strong><span>Invite people or adjust roles from the account access boundary.</span></div><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Open Access</button></div></div><div class="workspace-management-actions"><button type="button" class="btn-danger" id="workspace-management-action-' + escapeAttr(account.id) + '" data-action="workspace-action" data-account-id="' + escapeAttr(account.id) + '">' + escapeHTML(clientLanguage ? "Manage client" : "Manage workspace") + "</button></div></div></section>";
    }
    var workspaceHTML = workspaces.length ? '<div class="workspace-list-wrap" id="' + escapeAttr(workspaceListAnchorID(account.id)) + '">' + (workspaceHeaderActions ? '<div class="workspace-list-toolbar">' + workspaceHeaderActions + "</div>" : "") + '<div class="workspace-list-head"><span>' + escapeHTML(clientLanguage ? "Client" : "Workspace") + '</span><span>Setup</span><span>Health</span><span>Actions</span></div><div class="workspace-list">' + workspaces.map(function(workspace) {
      return renderWorkspaceCard(account, workspace, accountAPIBasePath);
    }).join("") + "</div></div>" : '<div class="empty-state"><p>' + escapeHTML(
      account.can_manage ? clientLanguage ? "No clients yet. Add one to get started." : "No hosted workspaces yet. Create one to get started." : clientLanguage ? "No clients are attached yet. An owner or admin must add the first one." : "No hosted workspaces are attached yet. An owner or admin must create the first one."
    ) + "</p>" + (workspaceHeaderActions ? '<div style="margin-top: 8px">' + workspaceHeaderActions + "</div>" : "") + "</div>";
    return '<section class="account-content-panel account-content-panel-workspaces"><div class="workspace-operations-shell workspace-operations-shell-idle" id="workspace-operations-shell-' + escapeAttr(account.id) + '"><div class="workspace-operations-detail" id="workspace-operations-detail-' + escapeAttr(account.id) + '" hidden>' + workspaceManagement + '</div><div class="workspace-operations-main">' + workspaceHTML + "</div></div></section>";
  }
  function renderAccountAccessSection(account) {
    var clientLanguage = accountUsesClientLanguage(account);
    var hasBilling = account.has_billing === true;
    var accessRoleCopy = {
      admin: hasBilling ? clientLanguage ? "Client control, billing, and roster management." : "Workspace control, billing, and roster management." : clientLanguage ? "Client control and roster management." : "Workspace control and roster management.",
      tech: hasBilling ? clientLanguage ? "Client control without billing or roster ownership." : "Workspace control without billing or roster ownership." : clientLanguage ? "Client control without roster ownership." : "Workspace control without roster ownership.",
      readOnly: clientLanguage ? "Review client status without control-plane changes." : "Review access without control-plane changes."
    };
    var accessTaskStrip = account.can_manage ? '<div class="access-task-strip"><button type="button" class="access-task-button" id="access-task-invite-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="invite">Invite people</button><button type="button" class="access-task-button" id="access-task-change_role-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="change_role">Change roles</button><button type="button" class="access-task-button" id="access-task-remove-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="remove">Remove access</button></div>' : renderSectionContextChips(["View roster", "Owner or admin required"]);
    var accessRoleGuide = '<div class="access-policy-panel"><div class="access-panel-heading"><h4>' + (account.can_manage ? "Choose the smallest role" : "Role meanings") + "</h4><p>" + (account.can_manage ? "Match each person to the narrowest role that still lets them do the job they own." : "Use these role meanings to understand what each person on this roster can do.") + '</p></div><div class="access-policy-list"><div class="access-policy-row"><strong>Owner</strong><span>' + escapeHTML(hasBilling ? "Full account, billing, and access control." : "Full account and access control.") + '</span></div><div class="access-policy-row"><strong>Admin</strong><span>' + escapeHTML(accessRoleCopy.admin) + '</span></div><div class="access-policy-row"><strong>Tech</strong><span>' + escapeHTML(accessRoleCopy.tech) + '</span></div><div class="access-policy-row"><strong>Read-only</strong><span>' + escapeHTML(accessRoleCopy.readOnly) + "</span></div></div></div>";
    var accessInvitePanel = account.can_manage ? '<div class="access-invite-panel"><div class="access-panel-heading"><h4>Invite people</h4><p>Add one person with the minimum role they need on this account.</p></div><div class="access-invite"><div><label for="invite-email-' + escapeAttr(account.id) + '">Email</label><input type="email" id="invite-email-' + escapeAttr(account.id) + '" placeholder="user@example.com" autocomplete="off"></div><div><label for="invite-role-' + escapeAttr(account.id) + '">Role</label><select id="invite-role-' + escapeAttr(account.id) + '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div><button type="button" class="btn-primary btn-compact" data-action="invite-member" data-account-id="' + escapeAttr(account.id) + '">Invite</button></div></div>' : "";
    var accessChangeRolePanel = '<div class="access-job-note-panel"><div class="access-panel-heading"><h4>Change roles on the roster</h4><p>Use the role column in the roster to change one person at a time. Keep each person on the smallest role they need.</p></div></div>' + accessRoleGuide;
    var accessRemovePanel = '<div class="access-job-note-panel"><div class="access-panel-heading"><h4>Remove stale access</h4><p>Use removal only when this person should no longer be on this hosted account. Owners may still be protected when they are the last owner.</p></div><div class="access-remove-points"><div class="access-remove-point"><strong>Pick the exact person</strong><span>Use the roster to remove one account member at a time.</span></div><div class="access-remove-point"><strong>Keep current owners safe</strong><span>The last owner cannot be removed until another owner exists.</span></div></div></div>';
    return '<section class="account-content-panel account-content-panel-access"><section class="access-management-panel access-section access-section-shell" id="access-section-' + escapeAttr(account.id) + '" data-actor-role="' + escapeAttr(account.role) + '" data-can-manage="' + escapeAttr(account.can_manage ? "true" : "false") + '" data-client-language="' + escapeAttr(clientLanguage ? "true" : "false") + '">' + (!account.can_manage ? '<p class="portal-section-copy">' + escapeHTML("Review who has access. An owner or admin must make changes.") + "</p>" : "") + '<div class="access-management-stats" id="access-stats-' + escapeAttr(account.id) + '"></div><div class="access-shell access-shell-idle" id="access-shell-' + escapeAttr(account.id) + '">' + (account.can_manage ? '<div class="access-shell-detail" id="access-detail-' + escapeAttr(account.id) + '" hidden><div class="access-task-panel" id="access-task-panel-' + escapeAttr(account.id) + '" hidden><div class="access-task-header"><div><h4 id="access-task-title-' + escapeAttr(account.id) + '">Invite people</h4><p id="access-task-copy-' + escapeAttr(account.id) + '"></p></div><button type="button" class="btn-secondary btn-compact" data-action="clear-access-job" data-account-id="' + escapeAttr(account.id) + '">Close panel</button></div><div class="access-task-body" id="access-task-body-invite-' + escapeAttr(account.id) + '" hidden>' + accessInvitePanel + accessRoleGuide + '</div><div class="access-task-body" id="access-task-body-change_role-' + escapeAttr(account.id) + '" hidden>' + accessChangeRolePanel + '</div><div class="access-task-body" id="access-task-body-remove-' + escapeAttr(account.id) + '" hidden>' + accessRemovePanel + "</div></div></div>" : "") + '<div class="access-shell-main"><div class="access-roster-column"><div class="access-roster">' + (account.can_manage ? '<div class="access-roster-toolbar">' + accessTaskStrip + "</div>" : "") + '<div class="access-roster-list" id="access-list-' + escapeAttr(account.id) + '"><div class="access-list-message">Loading\u2026</div></div></div></div></div></div></section></section>';
  }
  function renderHostedBillingCards(accounts, showSelfHostedCommercial) {
    var hostedBillingAccounts = accounts.filter(function(account) {
      return account.has_billing;
    });
    var clientLanguage = accountsUseClientLanguage(accounts);
    if (!hostedBillingAccounts.length) {
      return '<section class="billing-surface-block billing-surface-block-empty"><div class="billing-surface-header"><h3>No hosted billing attached</h3></div><p>' + escapeHTML(
        showSelfHostedCommercial ? "Use self-hosted billing tools below for self-hosted purchases." : "Hosted invoices and payment methods are not attached to this account."
      ) + "</p></section>";
    }
    return '<section class="billing-surface-block"><div class="billing-surface-header"><h3>Hosted billing</h3></div><div class="billing-action-list billing-action-list-surface">' + hostedBillingAccounts.map(function(account) {
      var actionHTML = account.can_manage ? '<button type="button" class="btn-primary btn-compact" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Open hosted billing</button>' : '<div class="billing-task-note">An owner or admin on this account needs to open hosted billing.</div>';
      return '<article class="billing-action-row billing-action-row-surface"><div class="billing-action-main"><div class="billing-action-copy"><h3>' + escapeHTML(account.name) + '</h3><p>Invoices, payment methods, and hosted subscription changes for this account.</p></div><div class="billing-action-meta">' + escapeHTML(clientLanguage ? "Keep client onboarding in Clients and roster changes in Access." : "Keep workspace changes in Workspaces and roster changes in Access.") + '</div></div><div class="billing-action-cta">' + actionHTML + "</div></article>";
    }).join("") + "</div></section>";
  }
  function renderBillingTaskPanel(title, copy, panelID, bodyHTML) {
    return '<section class="billing-panel" id="' + escapeAttr(panelID) + '" hidden><div class="billing-task-header"><div><h3>' + escapeHTML(title) + "</h3><p>" + escapeHTML(copy) + '</p></div><button type="button" class="btn-secondary btn-compact" data-account-billing-action="clear-billing-panel">Close panel</button></div><div class="billing-task-body">' + bodyHTML + "</div></section>";
  }
  function renderSupportSection(context) {
    var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
    var isHosted = accounts.length > 0;
    var clientLanguage = accountsUseClientLanguage(accounts);
    var primarySectionLabel = clientLanguage ? "Clients" : "Workspaces";
    var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
    var hasBillingSection = hasHostedBillingAccounts(accounts) || showSelfHostedCommercial;
    var supportEmail = context.bootstrap.support_email || "";
    var canManageHostedTasks = false;
    for (var i = 0; i < accounts.length; i += 1) {
      if (accounts[i].can_manage) {
        canManageHostedTasks = true;
        break;
      }
    }
    var hostedViewOnly = isHosted && !canManageHostedTasks;
    var retryCopy = isHosted ? hostedViewOnly ? hasBillingSection ? "Review " + primarySectionLabel + " or Access first. If billing is involved, hand it to an owner or admin before you escalate." : "Review " + primarySectionLabel + " or Access first. If account ownership is involved, hand it to an owner or admin before you escalate." : "Retry the same " + primarySectionLabel + (hasBillingSection ? ", Access, or Billing" : " or Access") + " step before you escalate." : "Retry the same Billing step before you escalate.";
    var supportActions = isHosted ? '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(primarySectionLabel) + '</button><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Access</button>' + (hasBillingSection ? '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Billing</button>' : "") : '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Billing</button>';
    return '<section class="portal-support-panel"><p>Use Support only after the self-service path fails. Retry the same step before you escalate.</p><div class="portal-support-simple"><div class="portal-support-simple-card"><div class="portal-support-simple-list"><div class="portal-support-simple-row"><strong>Try first</strong><span>' + escapeHTML(retryCopy) + '</span></div><div class="portal-support-simple-row"><strong>Scope</strong><span>' + escapeHTML(supportRunbookPathCopy(isHosted, hostedViewOnly, showSelfHostedCommercial, hasHostedBillingAccounts(accounts), clientLanguage)) + '</span></div><div class="portal-support-simple-row"><strong>Include</strong><span>Account, email, and the exact action that failed.</span></div></div><div class="portal-support-simple-actions">' + supportActions + '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + "</a></div></div></div></section>";
  }
  function renderHeaderHTML(context) {
    if (context.bootstrap.authenticated) {
      return '<div class="header-account-chip"><span class="header-account-email">' + escapeHTML(context.bootstrap.email || "") + '</span><button class="logout-btn" id="logout-btn" type="button">Sign out</button></div>';
    }
    if (!hasSignupPath(context.signupPath)) {
      return "";
    }
    return '<a class="logout-btn link-button" href="' + escapeAttr(context.signupPath) + '">Create account</a>';
  }
  function renderAuthenticatedPortalHTML(context) {
    var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
    var hosted = hasHostedAccounts(accounts);
    var hasHostedBilling = hasHostedBillingAccounts(accounts);
    var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
    var showSelfHostedUpgradeHandoff = context.billingState.openBillingPanelID === "upgrade-billing-panel" || !!normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey) || !!String(context.billingState.upgradePortalHandoffID || "").trim();
    var showSelfHostedBillingShell = showSelfHostedCommercial || showSelfHostedUpgradeHandoff;
    var showBillingPanel = hasHostedBilling || showSelfHostedBillingShell;
    var shellSections = visibleShellSections(context.bootstrap);
    var preferredSection = context.activeSection || preferredPortalShellSection(context.bootstrap);
    var activeSection = shellSections.some(function(entry) {
      return entry.section === preferredSection;
    }) ? preferredSection : shellSections[0] ? shellSections[0].section : "billing";
    var selfHostedBillingEscalationCopy = hosted ? "Escalate with the same hosted billing action or self-hosted path and the exact failed step." : "Escalate with the same self-hosted billing path and the exact failed step.";
    var workspacesContent = accounts.length ? accounts.map(function(account) {
      return '<section class="account-surface">' + renderAccountSurfaceHeader(account, accounts.length > 1) + renderAccountWorkspaceSection(account, context.accountAPIBasePath) + "</section>";
    }).join("") : renderNoHostedWorkspacesSection();
    var workspaceSummaryContent = hosted ? renderWorkspaceSummarySection(context) : "";
    var accessContent = accounts.length ? accounts.map(function(account) {
      return '<section class="account-surface">' + renderAccountSurfaceHeader(account, accounts.length > 1) + renderAccountAccessSection(account) + "</section>";
    }).join("") : renderNoHostedAccessSection();
    var selfHostedBillingLeadCopy = showSelfHostedCommercial ? "Use self-hosted billing only for self-hosted purchases." : "Pulse Account owns the commercial handoff for self-hosted upgrades from the app.";
    var selfHostedBillingActionsHTML = renderSelfHostedUpgradeActionRow(context);
    if (showSelfHostedCommercial) {
      selfHostedBillingActionsHTML += renderBillingActionRow("open-manage-billing", "Manage subscriptions", "Open", "Open Stripe for self-hosted plan, invoice, and payment changes.", "manage-billing-panel", "manage-inline-email", ["Plan changes", "Invoices"]) + renderBillingActionRow("open-retrieve-billing", "Retrieve licenses", "Open", "Recover the latest active self-hosted license and invoice link.", "retrieve-billing-panel", "retrieve-inline-email", ["Latest active license", "Invoice lookup"]) + renderBillingActionRow("open-refund-billing", "Refund requests", "Open", "Request a self-serve refund when the purchase is still eligible.", "refund-billing-panel", "refund-inline-email", ["Eligibility check", "Revocation"]) + renderBillingActionRow("open-data-billing", "Data and privacy", "Open", "Request export or deletion for commercial account data.", "data-billing-panel", "data-export-email", ["Export", "Deletion"]);
    }
    var selfHostedBillingPanelsHTML = renderSelfHostedUpgradeBillingPanel(context);
    if (showSelfHostedCommercial) {
      selfHostedBillingPanelsHTML += renderBillingTaskPanel(
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
      );
    }
    return '<div class="portal-shell" data-shell-section="' + activeSection + '"><div class="portal-shell-main">' + renderIdentityBar(accounts, showSelfHostedCommercial) + renderTabBar(context.bootstrap, activeSection) + (hosted ? '<section class="portal-content-panel portal-content-panel-workspaces">' + workspaceSummaryContent + workspacesContent + '</section><section class="portal-content-panel portal-content-panel-access">' + accessContent + "</section>" : "") + (showBillingPanel ? '<section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section">' + (hosted && hasHostedBilling ? renderHostedBillingCards(accounts, showSelfHostedCommercial) : "") + (showSelfHostedBillingShell ? '<div class="billing-shell billing-shell-idle"><div class="billing-shell-main"><div class="billing-shell-main-head"><h3>Self-hosted billing</h3><p>' + escapeHTML(selfHostedBillingLeadCopy) + '</p></div><div class="billing-action-list">' + selfHostedBillingActionsHTML + '</div><div class="billing-inline-support"><h4>Support</h4><p>' + selfHostedBillingEscalationCopy + '</p><div class="billing-inline-support-actions"><button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button><a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || "") + '">' + escapeHTML(context.bootstrap.support_email || "") + '</a></div></div></div><div class="billing-shell-detail" id="billing-detail-shell" hidden>' + selfHostedBillingPanelsHTML + "</div></div>" : "") + "</section>" : "") + '<section class="portal-content-panel portal-content-panel-support">' + renderSupportSection(context) + "</section></div></div>";
  }
  function renderAuthScopeRow(title, copy) {
    return '<article class="portal-auth-scope-row"><h3>' + escapeHTML(title) + "</h3><p>" + escapeHTML(copy) + "</p></article>";
  }
  function renderSignedOutPortalHTML(context) {
    var statusHTML = "";
    if (context.loginState.request.error) {
      statusHTML = '<div class="billing-status visible error">' + escapeHTML(context.loginState.request.error) + "</div>";
    } else if (context.loginState.success) {
      var successMessage = context.loginState.successMessage || "If that email is registered, a sign-in link is on the way.";
      statusHTML = '<div class="billing-status visible success">' + escapeHTML(successMessage) + '<br><br><strong>Need another link?</strong> <a href="#" data-portal-action="resend-magic-link">Send it again</a>.</div>';
    }
    var signupHTML = hasSignupPath(context.signupPath) ? '<p class="portal-auth-secondary-action">Need a new Pulse Account? <a href="' + escapeAttr(context.signupPath) + '">Create an account</a>.</p>' : "";
    return '<section class="portal-auth-shell"><div class="portal-auth-intro"><h1>Sign in to Pulse Account</h1><p>Use one commercial email address for hosted workspaces, account access, billing, licenses, refunds, and privacy requests.</p><div class="portal-auth-scope-list" aria-label="Pulse Account scope">' + renderAuthScopeRow("Workspaces", "Open hosted workspaces and review workspace state.") + renderAuthScopeRow("Access", "Review account access and manage roles when permitted.") + renderAuthScopeRow("Billing", "Open hosted billing or self-hosted commercial tools when they apply.") + '</div></div><section class="portal-auth-panel" aria-labelledby="portal-auth-title"><div class="portal-auth-card"><h2 id="portal-auth-title">Email sign-in link</h2><p>Enter the commercial email address for your Pulse account. A sign-in link will be sent to that address.</p><div class="form-group portal-auth-form-group"><label for="portal-login-email">Commercial email</label><input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' + escapeAttr(context.loginState.emailValue || "") + '" data-portal-input="login-email"></div><div class="form-actions portal-auth-actions"><button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' + (context.loginState.request.pending ? "Sending\u2026" : "Send sign-in link") + "</button></div>" + signupHTML + statusHTML + "</div></section></section>";
  }

  // src/shell.ts
  function installShell(deps) {
    function revealActiveNavLink(activeLink) {
      if (!activeLink) return;
      var group = activeLink.closest(".portal-tab-bar");
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
        if (isActive && button.classList.contains("portal-tab")) {
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
        billingState: deps.store.getBillingState(),
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
        billingState: deps.store.getBillingState(),
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
      var section = target.getAttribute("data-shell-section") || deps.store.getShellState().activeSection || "billing";
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
    syncPortalAccountStateBootstrap(accountState, bootstrapState.accounts || []);
    var loginState = createPortalLoginState();
    var shellState = createPortalShellState(bootstrapState);
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
        syncPortalAccountStateBootstrap(accountState, bootstrapState.accounts || []);
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
    if (!Array.isArray(accounts)) {
      return [];
    }
    return accounts.map(function(account) {
      return {
        ...account,
        workspaces: normalizeWorkspaces(account && Array.isArray(account.workspaces) ? account.workspaces : []),
        members: normalizeMembers(account && Array.isArray(account.members) ? account.members : []),
        setup_templates: Array.isArray(account.setup_templates) ? account.setup_templates.slice() : []
      };
    });
  }
  function normalizeWorkspaces(workspaces) {
    return workspaces.map(function(workspace) {
      return {
        ...workspace
      };
    });
  }
  function normalizeMembers(members) {
    return members.map(function(member) {
      return {
        ...member,
        subject_id: member.subject_id || member.user_id || "",
        state: member.state || "active"
      };
    });
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
  function normalizeUpgradePortalHandoffID(value) {
    var trimmed = String(value || "").trim();
    if (!trimmed) return "";
    return /^[A-Za-z0-9_-]+$/.test(trimmed) ? trimmed : "";
  }
  function normalizeHandoffBillingPanel(value) {
    switch (String(value || "").trim()) {
      case "upgrade":
        return "upgrade-billing-panel";
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
  function normalizeUpgradeFeatureKey2(value) {
    return String(value || "").trim();
  }
  function readPortalRuntimeHandoff(locationHref = window.location.href) {
    try {
      var params = new URL(locationHref).searchParams;
      var upgradePortalHandoffID = normalizeUpgradePortalHandoffID(params.get("portal_handoff_id"));
      var openBillingPanelID = normalizeHandoffBillingPanel(params.get("service"));
      if (!openBillingPanelID && upgradePortalHandoffID) {
        openBillingPanelID = "upgrade-billing-panel";
      }
      return {
        email: normalizeHandoffEmail(params.get("email")),
        openBillingPanelID,
        upgradePortalHandoffID,
        upgradeFeatureKey: normalizeUpgradeFeatureKey2(params.get("feature"))
      };
    } catch {
      return {
        email: "",
        openBillingPanelID: "",
        upgradePortalHandoffID: "",
        upgradeFeatureKey: ""
      };
    }
  }
  function createBootstrapDefaults(embeddedBootstrap) {
    var signupPath = typeof embeddedBootstrap.signup_path === "string" ? embeddedBootstrap.signup_path : "/signup";
    return {
      has_self_hosted_commercial: embeddedBootstrap.has_self_hosted_commercial === true,
      public_site_url: embeddedBootstrap.public_site_url || "https://pulserelay.pro",
      support_email: embeddedBootstrap.support_email || "support@pulserelay.pro",
      commercial_api_base_url: embeddedBootstrap.commercial_api_base_url || "",
      portal_path: embeddedBootstrap.portal_path || "/portal",
      bootstrap_path: embeddedBootstrap.bootstrap_path || "/api/portal/bootstrap",
      magic_link_request_path: embeddedBootstrap.magic_link_request_path || "/api/public/magic-link/request",
      signup_path: signupPath,
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
      store.setActiveShellSection("billing");
      store.updateBillingState(function(billingState) {
        billingState.openBillingPanelID = handoff.openBillingPanelID;
        billingState.upgradePortalHandoffID = handoff.upgradePortalHandoffID;
        billingState.upgradeFeatureKey = handoff.upgradeFeatureKey;
      }, { notify: false });
    } else if (handoff.upgradeFeatureKey || handoff.upgradePortalHandoffID) {
      store.setActiveShellSection("billing");
      store.updateBillingState(function(billingState) {
        billingState.upgradePortalHandoffID = handoff.upgradePortalHandoffID;
        billingState.upgradeFeatureKey = handoff.upgradeFeatureKey;
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
