import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { context } from 'esbuild';

import { createPortalBuildOptions, frontendRoot } from './build_config.mjs';

const scenarioCookieName = 'pulse_portal_preview_scenario';
const previewHost = process.env.PULSE_PORTAL_PREVIEW_HOST || '127.0.0.1';
const previewPort = Number(process.env.PULSE_PORTAL_PREVIEW_PORT || '8765');
const previewScenarios = ['managed', 'readonly', 'selfhosted', 'empty', 'provider', 'onboarding', 'mixed'];
const previewFaviconSVG = fs.readFileSync(path.join(frontendRoot, '..', '..', 'favicon.svg'), 'utf8');
const previewFaviconHref = '/favicon.svg?v=' + createHash('sha256').update(previewFaviconSVG).digest('hex').slice(0, 16);

function iso(value) {
  return new Date(value).toISOString();
}

function deepClone(value) {
  return JSON.parse(JSON.stringify(value));
}

function standardSetupTemplates() {
  return [{
    id: 'standard-client-onboarding',
    title: 'Standard client onboarding',
    agent_naming: 'Keep the client workspace as the identity boundary; repeated hostnames are expected across clients.',
    alert_routing: 'Create at least one enabled alert route inside each client workspace.',
    reporting: 'Schedule at least one client performance report before the workspace is marked ready.',
    access: 'Invite provider staff from Access and keep client users on the smallest useful role.',
  }];
}

function buildScenarioTemplate(name) {
  const base = {
    authenticated: true,
    email: 'courtman@gmail.com',
    has_self_hosted_commercial: false,
    public_site_url: 'https://pulserelay.pro',
    support_email: 'support@pulserelay.pro',
    commercial_api_base_url: '/__portal_preview/commercial',
    portal_path: '/portal',
    bootstrap_path: '/api/portal/bootstrap',
    magic_link_request_path: '/api/public/magic-link/request',
    signup_path: '/signup',
    logout_path: '/auth/logout',
    account_api_base_path: '/api/accounts',
    portal_api_base_path: '/api/portal',
    accounts: [],
  };

  if (name === 'readonly') {
    return {
      ...base,
      accounts: [{
        id: 'acct_readonly',
        name: 'Pulse',
        kind: 'msp',
        kind_label: 'MSP',
        role: 'read_only',
        can_manage: false,
        has_billing: true,
        setup_templates: standardSetupTemplates(),
        workspaces: [
          {
            id: 'ws_alpha',
            display_name: 'MSP Test Workspace A',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
            setup_status: 'ready',
            agent_count: 2,
            agent_token_count: 2,
            unused_agent_token_count: 0,
            alert_route_count: 1,
            disabled_alert_route_count: 0,
            report_schedule_count: 1,
            disabled_report_schedule_count: 0,
            created_at: iso('2026-03-20T10:00:00Z'),
          },
          {
            id: 'ws_beta',
            display_name: 'MSP Test Workspace B',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
            setup_status: 'ready',
            agent_count: 1,
            agent_token_count: 1,
            unused_agent_token_count: 0,
            alert_route_count: 1,
            disabled_alert_route_count: 0,
            report_schedule_count: 1,
            disabled_report_schedule_count: 0,
            created_at: iso('2026-03-21T10:00:00Z'),
          },
        ],
        members: [
          { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
          { email: 'ops@example.com', role: 'tech', user_id: 'u_ops' },
          { email: 'courtman@gmail.com', role: 'read_only', user_id: 'u_view' },
        ],
      }],
    };
  }

  if (name === 'selfhosted') {
    return {
      ...base,
      email: 'buyer@example.com',
      has_self_hosted_commercial: true,
      accounts: [],
    };
  }

  if (name === 'empty') {
    return {
      ...base,
      accounts: [{
        id: 'acct_empty',
        name: 'Pulse',
        kind: 'msp',
        kind_label: 'MSP',
        role: 'owner',
        can_manage: true,
        has_billing: true,
        setup_templates: standardSetupTemplates(),
        workspaces: [],
        members: [
          { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
        ],
      }],
    };
  }

  if (name === 'provider') {
    return {
      ...base,
      signup_path: '',
      accounts: [{
        id: 'acct_provider',
        name: 'Provider MSP',
        kind: 'msp',
        kind_label: 'MSP',
        role: 'owner',
        can_manage: true,
        has_billing: false,
        setup_templates: standardSetupTemplates(),
        workspaces: [],
        members: [
          { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
          { email: 'helpdesk@example.com', role: 'tech', user_id: 'u_helpdesk' },
        ],
      }],
    };
  }

  if (name === 'onboarding') {
    return {
      ...base,
      accounts: [{
        id: 'acct_onboarding',
        name: 'Pulse MSP Demo',
        kind: 'msp',
        kind_label: 'MSP',
        role: 'owner',
        can_manage: true,
        has_billing: true,
        setup_templates: standardSetupTemplates(),
        workspaces: [
          {
            id: 'ws_acme',
            display_name: 'Acme Dental',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
            setup_status: 'install_agents',
            agent_count: 0,
            agent_token_count: 1,
            unused_agent_token_count: 1,
            alert_route_count: 0,
            disabled_alert_route_count: 0,
            report_schedule_count: 0,
            disabled_report_schedule_count: 0,
            created_at: iso('2026-05-28T10:00:00Z'),
          },
          {
            id: 'ws_northbridge',
            display_name: 'Northbridge Legal',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
            setup_status: 'configure_outputs',
            agent_count: 1,
            agent_token_count: 1,
            unused_agent_token_count: 0,
            alert_route_count: 0,
            disabled_alert_route_count: 1,
            report_schedule_count: 0,
            disabled_report_schedule_count: 1,
            created_at: iso('2026-05-29T10:00:00Z'),
          },
          {
            id: 'ws_hilltop',
            display_name: 'Hilltop Care',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
            setup_status: 'ready',
            agent_count: 2,
            agent_token_count: 2,
            unused_agent_token_count: 0,
            alert_route_count: 1,
            disabled_alert_route_count: 0,
            report_schedule_count: 1,
            disabled_report_schedule_count: 0,
            created_at: iso('2026-05-30T10:00:00Z'),
          },
        ],
        members: [
          { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
          { email: 'helpdesk@example.com', role: 'tech', user_id: 'u_helpdesk' },
        ],
      }],
    };
  }

  if (name === 'mixed') {
    return {
      ...base,
      accounts: [
        {
          id: 'acct_mixed_msp',
          name: 'Provider Account',
          kind: 'msp',
          kind_label: 'MSP',
          role: 'owner',
          can_manage: true,
          has_billing: true,
          setup_templates: standardSetupTemplates(),
          workspaces: [
            {
              id: 'ws_mixed_client',
              display_name: 'Acme Dental',
              state: 'active',
              healthy: true,
              health_status: 'healthy',
              setup_status: 'install_agents',
              agent_count: 0,
              agent_token_count: 1,
              unused_agent_token_count: 1,
              alert_route_count: 0,
              disabled_alert_route_count: 0,
              report_schedule_count: 0,
              disabled_report_schedule_count: 0,
              created_at: iso('2026-05-28T10:00:00Z'),
            },
          ],
          members: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
            { email: 'helpdesk@example.com', role: 'tech', user_id: 'u_helpdesk' },
          ],
        },
        {
          id: 'acct_mixed_cloud',
          name: 'Hosted Ops',
          kind: 'cloud',
          kind_label: 'Cloud',
          role: 'admin',
          can_manage: true,
          has_billing: true,
          workspaces: [
            {
              id: 'ws_mixed_cloud',
              display_name: 'Operations Workspace',
              state: 'active',
              healthy: true,
              health_status: 'healthy',
              setup_status: 'ready',
              agent_count: 2,
              agent_token_count: 2,
              unused_agent_token_count: 0,
              alert_route_count: 1,
              disabled_alert_route_count: 0,
              report_schedule_count: 1,
              disabled_report_schedule_count: 0,
              created_at: iso('2026-05-30T10:00:00Z'),
            },
          ],
          members: [
            { email: 'admin@example.com', role: 'admin', user_id: 'u_admin' },
            { email: 'viewer@example.com', role: 'read_only', user_id: 'u_viewer' },
          ],
        },
      ],
    };
  }

  return {
    ...base,
    accounts: [{
      id: 'acct_managed',
      name: 'Pulse',
      kind: 'msp',
      kind_label: 'MSP',
      role: 'owner',
      can_manage: true,
      has_billing: true,
      setup_templates: standardSetupTemplates(),
      workspaces: [
        {
          id: 'ws_alpha',
          display_name: 'MSP Test Workspace A',
          state: 'suspended',
          healthy: false,
          health_status: 'unhealthy',
          setup_status: 'review',
          created_at: iso('2026-03-20T10:00:00Z'),
        },
        {
          id: 'ws_beta',
          display_name: 'MSP Test Workspace B',
          state: 'active',
          healthy: true,
          health_status: 'healthy',
          setup_status: 'configure_outputs',
          agent_count: 1,
          agent_token_count: 1,
          unused_agent_token_count: 0,
          alert_route_count: 0,
          disabled_alert_route_count: 1,
          report_schedule_count: 0,
          disabled_report_schedule_count: 1,
          created_at: iso('2026-03-21T10:00:00Z'),
        },
        {
          id: 'ws_gamma',
          display_name: 'MSP Test Workspace C',
          state: 'active',
          healthy: true,
          health_status: 'healthy',
          setup_status: 'install_agents',
          agent_count: 0,
          agent_token_count: 1,
          unused_agent_token_count: 1,
          alert_route_count: 0,
          disabled_alert_route_count: 0,
          report_schedule_count: 0,
          disabled_report_schedule_count: 0,
          created_at: iso('2026-03-22T10:00:00Z'),
        },
      ],
      members: [
        { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
        { email: 'admin@example.com', role: 'admin', user_id: 'u_admin' },
        { email: 'ops@example.com', role: 'tech', user_id: 'u_ops' },
      ],
    }],
  };
}

const scenarioTemplates = Object.fromEntries(
  previewScenarios.map(function(name) {
    return [name, buildScenarioTemplate(name)];
  }),
);

const scenarioState = new Map();
const sseClients = new Set();

function resetScenario(name) {
  const scenario = previewScenarios.includes(name) ? name : 'managed';
  const nextState = deepClone(scenarioTemplates[scenario]);
  scenarioState.set(scenario, nextState);
  return nextState;
}

function getScenarioState(name) {
  const scenario = previewScenarios.includes(name) ? name : 'managed';
  if (!scenarioState.has(scenario)) {
    resetScenario(scenario);
  }
  return scenarioState.get(scenario);
}

function parseCookies(headerValue) {
  const cookies = {};
  for (const part of String(headerValue || '').split(';')) {
    const index = part.indexOf('=');
    if (index === -1) continue;
    const key = part.slice(0, index).trim();
    const value = part.slice(index + 1).trim();
    if (!key) continue;
    cookies[key] = decodeURIComponent(value);
  }
  return cookies;
}

function resolveScenario(url, request) {
  const requestedScenario = String(url.searchParams.get('scenario') || '').trim();
  if (previewScenarios.includes(requestedScenario)) {
    return requestedScenario;
  }
  const cookies = parseCookies(request.headers.cookie || '');
  if (previewScenarios.includes(cookies[scenarioCookieName])) {
    return cookies[scenarioCookieName];
  }
  return 'managed';
}

function readRequestBody(request) {
  return new Promise(function(resolve, reject) {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', function(chunk) {
      body += chunk;
    });
    request.on('end', function() {
      resolve(body);
    });
    request.on('error', reject);
  });
}

async function readJSONBody(request) {
  const body = await readRequestBody(request);
  if (!body.trim()) return {};
  return JSON.parse(body);
}

function sendJSON(response, statusCode, payload, extraHeaders = {}) {
  response.writeHead(statusCode, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
    ...extraHeaders,
  });
  response.end(JSON.stringify(payload));
}

function sendText(response, statusCode, payload, extraHeaders = {}) {
  response.writeHead(statusCode, {
    'Content-Type': 'text/plain; charset=utf-8',
    'Cache-Control': 'no-store',
    ...extraHeaders,
  });
  response.end(payload);
}

function escapeHTML(value) {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function previewReturnURL(request, scenario, toastMessage) {
  const origin = 'http://' + previewHost + ':' + String(previewPort);
  const url = new URL('/', origin);
  url.searchParams.set('scenario', scenario);
  if (toastMessage) {
    url.searchParams.set('preview_toast', toastMessage);
  }
  return url.toString();
}

function previewWorkspaceURL(scenario, workspaceID, targetPath) {
  const origin = 'http://' + previewHost + ':' + String(previewPort);
  const url = new URL('/__portal_preview/workspaces/' + encodeURIComponent(workspaceID), origin);
  url.searchParams.set('scenario', scenario);
  if (targetPath) {
    url.searchParams.set('target_path', targetPath);
  }
  return url.toString();
}

function findWorkspaceByID(bootstrap, workspaceID) {
  for (const account of bootstrap.accounts || []) {
    for (const workspace of account.workspaces || []) {
      if (workspace.id === workspaceID) {
        return { account, workspace };
      }
    }
  }
  return null;
}

function updatePreviewWorkspaceSetupStatus(workspace) {
  if (!workspace || workspace.state !== 'active' || workspace.health_status === 'unhealthy') {
    if (workspace) workspace.setup_status = 'review';
    return;
  }
  if (Number(workspace.agent_count || 0) <= 0) {
    workspace.setup_status = 'install_agents';
    return;
  }
  if (Number(workspace.alert_route_count || 0) <= 0 || Number(workspace.report_schedule_count || 0) <= 0) {
    workspace.setup_status = 'configure_outputs';
    return;
  }
  workspace.setup_status = 'ready';
}

function applyPreviewWorkspaceSetupAction(bootstrap, workspaceID, action) {
  const entry = findWorkspaceByID(bootstrap, workspaceID);
  if (!entry) {
    return { ok: false, status: 404, message: 'Client workspace not found.' };
  }
  const workspace = entry.workspace;
  if (workspace.state !== 'active') {
    return { ok: false, status: 409, message: 'Client workspace is not active.' };
  }
  if (action === 'agent-checkin') {
    workspace.agent_count = Math.max(1, Number(workspace.agent_count || 0));
    workspace.agent_token_count = Math.max(1, Number(workspace.agent_token_count || 0));
    workspace.unused_agent_token_count = 0;
    workspace.last_agent_seen_at = iso(new Date());
    updatePreviewWorkspaceSetupStatus(workspace);
    return {
      ok: true,
      targetPath: '/settings/support/reporting',
      message: 'Preview agent check-in recorded. Finish alerts and reports next.',
    };
  }
  if (action === 'enable-alert-route') {
    workspace.alert_route_count = Math.max(1, Number(workspace.alert_route_count || 0));
    workspace.disabled_alert_route_count = 0;
    updatePreviewWorkspaceSetupStatus(workspace);
    return {
      ok: true,
      targetPath: '/settings/support/reporting',
      message: 'Preview alert route enabled.',
    };
  }
  if (action === 'schedule-report') {
    workspace.report_schedule_count = Math.max(1, Number(workspace.report_schedule_count || 0));
    workspace.disabled_report_schedule_count = 0;
    updatePreviewWorkspaceSetupStatus(workspace);
    return {
      ok: true,
      targetPath: '/settings/support/reporting',
      message: workspace.setup_status === 'ready'
        ? 'Preview report scheduled. This client workspace is ready.'
        : 'Preview report scheduled.',
    };
  }
  return { ok: false, status: 400, message: 'Preview setup action is invalid.' };
}

function previewSetupActionURL(scenario, workspaceID, action, targetPath) {
  const url = new URL('/__portal_preview/workspaces/' + encodeURIComponent(workspaceID) + '/setup', 'http://' + previewHost + ':' + String(previewPort));
  url.searchParams.set('scenario', scenario);
  url.searchParams.set('action', action);
  if (targetPath) {
    url.searchParams.set('target_path', targetPath);
  }
  return url.pathname + url.search;
}

function previewSetupControlHTML(workspace, scenario, workspaceID, targetPath) {
  if (!workspace || workspace.state !== 'active') return '';
  const controls = [];
  if (Number(workspace.agent_count || 0) <= 0) {
    controls.push({
      action: 'agent-checkin',
      label: 'Record agent check-in',
      note: 'Simulates the tenant runtime seeing the first workspace-scoped reporting agent.',
    });
  }
  if (Number(workspace.agent_count || 0) > 0 && Number(workspace.alert_route_count || 0) <= 0) {
    controls.push({
      action: 'enable-alert-route',
      label: 'Enable alert route',
      note: 'Simulates enabling a tenant-owned notification route for this client.',
    });
  }
  if (Number(workspace.agent_count || 0) > 0 && Number(workspace.report_schedule_count || 0) <= 0) {
    controls.push({
      action: 'schedule-report',
      label: 'Schedule report',
      note: 'Simulates enabling a tenant-owned performance report schedule.',
    });
  }
  if (!controls.length) {
    return '<div class="preview-controls"><strong>Client setup controls</strong><p>This client already has a reporting agent, alert route, and report schedule.</p></div>';
  }
  return '<div class="preview-controls"><strong>Client setup controls</strong><p>These controls simulate tenant-local setup facts so the Pulse Account client onboarding flow can be tested end to end.</p>' +
    controls.map(function(control) {
      return '<form method="POST" action="' + escapeHTML(previewSetupActionURL(scenario, workspaceID, control.action, targetPath)) + '">' +
          '<button class="copy-button" type="submit">' + escapeHTML(control.label) + '</button>' +
          '<span>' + escapeHTML(control.note) + '</span>' +
        '</form>';
    }).join('') +
    '</div>';
}

function previewTargetLabel(targetPath) {
  if (targetPath === '/settings/infrastructure?add=linux-host') {
    return 'Settings -> Infrastructure, with the agent install drawer open';
  }
  if (targetPath === '/settings/support/reporting') {
    return 'Settings -> Reporting for this client workspace';
  }
  return 'the client workspace dashboard';
}

function buildPreviewWorkspaceHTML(bootstrap, workspaceID, targetPath, scenario, previewToast = '') {
  const entry = findWorkspaceByID(bootstrap, workspaceID);
  const title = entry ? entry.workspace.display_name : workspaceID;
  const accountName = entry ? entry.account.name : 'Pulse Account';
  const accountID = entry ? entry.account.id : '';
  const setupStatus = entry && entry.workspace.setup_status ? entry.workspace.setup_status : 'workspace';
  const targetLabel = previewTargetLabel(targetPath);
  const portalURL = '/?scenario=' + encodeURIComponent(scenario);
  const portalWorkspaceURL = accountID
    ? portalURL + '#workspace-row-' + encodeURIComponent(accountID) + '-' + encodeURIComponent(workspaceID)
    : portalURL;
  const targetPathLabel = targetPath || '/';
  const targetHeading = targetPath === '/settings/infrastructure?add=linux-host'
    ? 'Install agents for ' + title
    : targetPath === '/settings/support/reporting'
      ? 'Reports for ' + title
      : title + ' client workspace';
  const taskCopy = targetPath === '/settings/infrastructure?add=linux-host'
    ? 'Use the install command from this workspace. Agent data must be created inside this client boundary, not on the provider account.'
    : targetPath === '/settings/support/reporting'
      ? 'Configure scheduled reports and alert routing for this client workspace before treating onboarding as complete.'
      : 'Use this workspace for client-specific monitoring work after the Pulse Account handoff.';
  const installCommand = 'pulse-agent install --workspace ' + workspaceID + ' --name <agent-name>';
  return '<!DOCTYPE html>' +
    '<html lang="en">' +
      '<head>' +
        '<meta charset="utf-8">' +
        '<meta name="viewport" content="width=device-width, initial-scale=1">' +
        '<title>' + escapeHTML(title) + ' - Client onboarding preview</title>' +
        '<style>' +
          'body{margin:0;font-family:Inter,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f6f7f9;color:#101828}' +
          'main{max-width:840px;margin:0 auto;padding:48px 20px}' +
          '.panel{background:#fff;border:1px solid #d0d5dd;border-radius:8px;padding:24px;box-shadow:0 1px 2px rgba(16,24,40,.06)}' +
          '.preview-message{border:1px solid #bbf7d0;border-radius:6px;background:#f0fdf4;color:#166534;padding:10px;margin:12px 0 16px;font-size:13px}' +
          '.crumbs{display:flex;align-items:center;gap:6px;flex-wrap:wrap;margin-bottom:18px;color:#667085;font-size:13px}' +
          '.crumbs a{color:#155eef;text-decoration:none}' +
          'h1{margin:0 0 8px;font-size:24px;letter-spacing:0}' +
          'h2{margin:24px 0 8px;font-size:16px}' +
          'p{margin:0 0 16px;color:#475467;line-height:1.5}' +
          '.facts{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px;margin:20px 0}' +
          '.fact{border:1px solid #eaecf0;border-radius:6px;padding:10px;background:#fcfcfd}' +
          '.fact span{display:block;font-size:12px;color:#667085}' +
          '.fact strong{display:block;margin-top:2px;font-size:13px}' +
          '.task{border:1px solid #eaecf0;border-radius:6px;background:#f8fafc;padding:14px;margin:16px 0}' +
          '.task code{display:block;margin:8px 0 0;padding:10px;border:1px solid #d0d5dd;border-radius:6px;background:#fff;color:#101828;white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px}' +
          '.preview-controls{border:1px solid #bfdbfe;border-radius:6px;background:#eff6ff;padding:14px;margin:16px 0}' +
          '.preview-controls strong{display:block;font-size:13px}.preview-controls p{margin:4px 0 10px;font-size:13px;color:#344054}' +
          '.preview-controls form{display:grid;grid-template-columns:max-content minmax(0,1fr);gap:8px;align-items:center;margin-top:8px}' +
          '.preview-controls span{font-size:12px;line-height:1.45;color:#475467}' +
          '.steps{display:grid;gap:8px;margin:16px 0}' +
          '.step{display:grid;grid-template-columns:28px minmax(0,1fr);gap:10px;padding:10px;border:1px solid #eaecf0;border-radius:6px;background:#fff}' +
          '.step b{display:flex;align-items:center;justify-content:center;width:22px;height:22px;border-radius:999px;background:#eef4ff;color:#155eef;font-size:12px}' +
          '.step strong{display:block;font-size:13px}.step span{display:block;margin-top:2px;color:#667085;font-size:12px;line-height:1.45}' +
          '.button{display:inline-flex;align-items:center;min-height:34px;padding:0 12px;border-radius:6px;background:#155eef;color:#fff;text-decoration:none;font-size:14px;font-weight:600}' +
          '.copy-button{display:inline-flex;align-items:center;min-height:32px;margin-top:10px;padding:0 10px;border:1px solid #d0d5dd;border-radius:6px;background:#fff;color:#344054;font:inherit;font-size:13px;font-weight:600;cursor:pointer}' +
          '.secondary{background:#fff;color:#344054;border:1px solid #d0d5dd}' +
          '.actions{display:flex;gap:8px;flex-wrap:wrap}' +
          '@media(max-width:640px){.facts{grid-template-columns:1fr}.preview-controls form{grid-template-columns:1fr}}' +
        '</style>' +
      '</head>' +
      '<body>' +
        '<main>' +
          '<section class="panel">' +
            '<div class="crumbs"><a href="' + escapeHTML(portalWorkspaceURL) + '">Pulse Account</a><span>/</span><span>' + escapeHTML(accountName) + '</span><span>/</span><strong>' + escapeHTML(title) + '</strong></div>' +
            '<p>Client onboarding preview</p>' +
            '<h1>' + escapeHTML(targetHeading) + '</h1>' +
            (previewToast ? '<div class="preview-message">' + escapeHTML(previewToast) + '</div>' : '') +
            '<p>You are inside the client workspace for <strong>' + escapeHTML(title) + '</strong>. In production the signed handoff creates a tenant session and opens ' + escapeHTML(targetLabel) + '.</p>' +
            '<p>This client boundary is the thing that keeps repeated hostnames, alerts, and reports from being mixed across MSP customers.</p>' +
            '<div class="facts">' +
              '<div class="fact"><span>Account</span><strong>' + escapeHTML(accountName) + '</strong></div>' +
              '<div class="fact"><span>Client workspace</span><strong>' + escapeHTML(workspaceID) + '</strong></div>' +
              '<div class="fact"><span>Setup status</span><strong>' + escapeHTML(setupStatus) + '</strong></div>' +
              '<div class="fact"><span>Target</span><strong>' + escapeHTML(targetPathLabel) + '</strong></div>' +
            '</div>' +
            '<div class="task">' +
              '<strong>' + escapeHTML(targetLabel) + '</strong>' +
              '<p>' + escapeHTML(taskCopy) + '</p>' +
              (targetPath === '/settings/infrastructure?add=linux-host'
                ? '<code id="preview-install-command">' + escapeHTML(installCommand) + '</code><button class="copy-button" type="button" onclick="navigator.clipboard&&navigator.clipboard.writeText(document.getElementById(\'preview-install-command\').textContent)">Copy command</button>'
                : '') +
            '</div>' +
            (entry ? previewSetupControlHTML(entry.workspace, scenario, workspaceID, targetPath) : '') +
            '<div class="steps" aria-label="Client onboarding flow">' +
              '<div class="step"><b>1</b><div><strong>Client added</strong><span>The client gets a separate workspace boundary before monitoring data arrives.</span></div></div>' +
              '<div class="step"><b>2</b><div><strong>Install agents in this workspace</strong><span>The handoff keeps the agent command tied to this client boundary.</span></div></div>' +
              '<div class="step"><b>3</b><div><strong>Confirm data is arriving</strong><span>Pulse Account marks the workspace as installed only after an agent-scoped token is used.</span></div></div>' +
              '<div class="step"><b>4</b><div><strong>Add alert routes</strong><span>Notification routes stay tenant-owned and enabled inside the client workspace.</span></div></div>' +
              '<div class="step"><b>5</b><div><strong>Schedule reports</strong><span>Performance reports are part of ready state for MSP onboarding.</span></div></div>' +
            '</div>' +
            '<div class="actions">' +
              '<a class="button" href="' + escapeHTML(portalWorkspaceURL) + '">Back to client row</a>' +
              '<a class="button secondary" href="' + escapeHTML(portalURL + '#workspace-management-' + accountID) + '">Refresh setup state</a>' +
              '<a class="button secondary" href="' + escapeHTML(portalURL) + '">Pulse Account home</a>' +
            '</div>' +
          '</section>' +
        '</main>' +
      '</body>' +
    '</html>';
}

function buildPreviewHTML(assets, bootstrap, previewToast) {
  const bootstrapJSON = JSON.stringify(bootstrap).replace(/</g, '\\u003c');
  const safeToast = previewToast ? JSON.stringify(String(previewToast)) : 'null';
  return '<!DOCTYPE html>' +
    '<html lang="en">' +
      '<head>' +
        '<meta charset="utf-8">' +
        '<meta name="viewport" content="width=device-width, initial-scale=1">' +
        '<title>Pulse Account Preview</title>' +
        '<link rel="icon" href="' + previewFaviconHref + '" type="image/svg+xml">' +
        '<style>' + assets.css + '</style>' +
      '</head>' +
      '<body>' +
        '<header>' +
          '<span class="brand">Pulse Account</span>' +
          '<div class="user-info" id="portal-user-info"></div>' +
        '</header>' +
        '<script id="pulse-account-bootstrap" type="application/json">' + bootstrapJSON + '</script>' +
        '<main class="main"><div id="portal-app-root"></div></main>' +
        '<div class="toast" id="toast"></div>' +
        '<script>' + assets.js + '</script>' +
        '<script>' +
          '(function(){' +
            'var source=new EventSource("/__portal_preview/events");' +
            'source.addEventListener("reload",function(){window.location.reload();});' +
            'var previewToast=' + safeToast + ';' +
            'if(previewToast){' +
              'var toast=document.getElementById("toast");' +
              'if(toast){toast.textContent=previewToast;toast.className="toast visible";setTimeout(function(){toast.className="toast";},3200);}' +
            '}' +
          '})();' +
        '</script>' +
      '</body>' +
    '</html>';
}

const buildState = {
  assets: {
    css: '',
    js: '',
  },
  errors: [],
  version: 0,
};

function notifyReload() {
  const payload = JSON.stringify({ version: buildState.version });
  for (const response of sseClients) {
    response.write('event: reload\n');
    response.write('data: ' + payload + '\n\n');
  }
}

function buildErrorText(errors) {
  return errors.map(function(error) {
    const location = error.location
      ? error.location.file + ':' + error.location.line + ':' + error.location.column
      : 'unknown';
    return location + '\n' + error.text;
  }).join('\n\n');
}

const previewAssetsPlugin = {
  name: 'pulse-account-preview-assets',
  setup(build) {
    build.onEnd(function(result) {
      if (result.errors.length > 0) {
        buildState.errors = result.errors.slice();
        console.error('[portal-preview] build failed\n' + buildErrorText(result.errors));
        return;
      }
      const jsOutput = result.outputFiles.find(function(file) {
        return file.path.endsWith('portal_app.js');
      });
      const cssOutput = result.outputFiles.find(function(file) {
        return file.path.endsWith('portal_app.css');
      });
      if (!jsOutput || !cssOutput) {
        buildState.errors = [{
          text: 'Preview build did not produce both portal_app.js and portal_app.css.',
          location: null,
        }];
        console.error('[portal-preview] build failed\nPreview build did not produce both portal_app.js and portal_app.css.');
        return;
      }
      buildState.assets.js = jsOutput.text;
      buildState.assets.css = cssOutput.text;
      buildState.errors = [];
      buildState.version += 1;
      console.log('[portal-preview] build ' + String(buildState.version) + ' ready');
      notifyReload();
    });
  },
};

function findAccount(bootstrap, accountID) {
  return (bootstrap.accounts || []).find(function(account) {
    return account.id === accountID;
  }) || null;
}

function routeAccountAPI(request, response, url, bootstrap, scenario) {
  const match = url.pathname.match(/^\/api\/accounts\/([^/]+)\/(tenants|members)(?:\/([^/]+))?(?:\/([^/]+))?$/);
  if (!match) {
    sendText(response, 404, 'Not found');
    return;
  }

  const accountID = decodeURIComponent(match[1]);
  const resource = match[2];
  const resourceID = match[3] ? decodeURIComponent(match[3]) : '';
  const action = match[4] ? decodeURIComponent(match[4]) : '';
  const account = findAccount(bootstrap, accountID);
  if (!account) {
    sendJSON(response, 404, { error: 'Account not found.' });
    return;
  }

  if (action && !(resource === 'tenants' && action === 'handoff')) {
    sendText(response, 404, 'Not found');
    return;
  }

  if (resource === 'members') {
    if (request.method === 'GET' && !resourceID) {
      sendJSON(response, 200, account.members || []);
      return;
    }
    if (request.method === 'POST' && !resourceID) {
      readJSONBody(request).then(function(body) {
        const email = String(body.email || '').trim().toLowerCase();
        const role = String(body.role || 'read_only').trim() || 'read_only';
        const members = account.members || [];
        if (!email) {
          sendJSON(response, 400, { error: 'Email is required.' });
          return;
        }
        if (members.some(function(member) { return String(member.email || '').toLowerCase() === email; })) {
          sendJSON(response, 409, { error: 'Member already exists.' });
          return;
        }
        members.push({
          email,
          role,
          user_id: 'u_' + Math.random().toString(36).slice(2, 10),
          created_at: iso(new Date()),
        });
        account.members = members;
        sendJSON(response, 200, { ok: true });
      }).catch(function() {
        sendJSON(response, 400, { error: 'Invalid JSON.' });
      });
      return;
    }
    if (request.method === 'PATCH' && resourceID) {
      readJSONBody(request).then(function(body) {
        const nextRole = String(body.role || '').trim();
        const member = (account.members || []).find(function(item) {
          return item.user_id === resourceID;
        });
        if (!member) {
          sendJSON(response, 404, { error: 'Member not found.' });
          return;
        }
        member.role = nextRole || member.role;
        sendJSON(response, 200, { ok: true });
      }).catch(function() {
        sendJSON(response, 400, { error: 'Invalid JSON.' });
      });
      return;
    }
    if (request.method === 'DELETE' && resourceID) {
      account.members = (account.members || []).filter(function(member) {
        return member.user_id !== resourceID;
      });
      sendJSON(response, 200, { ok: true });
      return;
    }
  }

  if (resource === 'tenants') {
    if (request.method === 'POST' && resourceID && action === 'handoff') {
      const workspace = (account.workspaces || []).find(function(item) {
        return item.id === resourceID;
      });
      if (!workspace) {
        sendJSON(response, 404, { error: 'Client workspace not found.' });
        return;
      }
      if (workspace.state !== 'active') {
        sendJSON(response, 409, { error: 'Client workspace is not active.' });
        return;
      }
      response.writeHead(303, {
        Location: previewWorkspaceURL(scenario, resourceID, String(url.searchParams.get('target_path') || '')),
        'Cache-Control': 'no-store',
      });
      response.end();
      return;
    }
    if (action) {
      sendText(response, 405, 'Method not allowed');
      return;
    }
    if (request.method === 'POST' && !resourceID) {
      readJSONBody(request).then(function(body) {
        const displayName = String(body.display_name || '').trim();
        if (!displayName) {
          sendJSON(response, 400, { error: 'Client name is required.' });
          return;
        }
        account.workspaces = account.workspaces || [];
        const workspace = {
          id: 'ws_' + Math.random().toString(36).slice(2, 10),
          display_name: displayName,
          state: 'active',
          healthy: true,
          health_status: 'healthy',
          setup_status: 'install_agents',
          agent_count: 0,
          agent_token_count: 1,
          unused_agent_token_count: 1,
          alert_route_count: 0,
          disabled_alert_route_count: 0,
          report_schedule_count: 0,
          disabled_report_schedule_count: 0,
          created_at: iso(new Date()),
        };
        account.workspaces.push(workspace);
        sendJSON(response, 201, workspace);
      }).catch(function() {
        sendJSON(response, 400, { error: 'Invalid JSON.' });
      });
      return;
    }
    if (request.method === 'PATCH' && resourceID) {
      readJSONBody(request).then(function(body) {
        const workspace = (account.workspaces || []).find(function(item) {
          return item.id === resourceID;
        });
        if (!workspace) {
          sendJSON(response, 404, { error: 'Client workspace not found.' });
          return;
        }
        if (String(body.state || '').trim() === 'suspended') {
          workspace.state = 'suspended';
          workspace.healthy = false;
          workspace.health_status = 'unhealthy';
        }
        sendJSON(response, 200, { ok: true });
      }).catch(function() {
        sendJSON(response, 400, { error: 'Invalid JSON.' });
      });
      return;
    }
    if (request.method === 'DELETE' && resourceID) {
      account.workspaces = (account.workspaces || []).filter(function(workspace) {
        return workspace.id !== resourceID;
      });
      sendJSON(response, 200, { ok: true });
      return;
    }
  }

  sendText(response, 405, 'Method not allowed');
}

function routeCommercialAPI(request, response, url, scenario) {
  const successRedirect = previewReturnURL(request, scenario, 'Preview mode: external billing or email verification would complete here.');
  const route = url.pathname.replace('/__portal_preview/commercial', '');

  if (request.method === 'GET' && route === '/v1/public/pricing-model') {
    sendJSON(response, 200, {
      title: 'Simple self-hosted pricing for Pulse',
      description: 'Community keeps core monitoring free. Relay gives secure remote access to the Pulse web UI, mobile app pairing, push notifications, and 14-day history. Pro adds Patrol control, alert investigation, verified fixes, and 90-day history.',
      explainer:
        'Community keeps core monitoring free. Relay gets your Pulse web UI securely reachable from anywhere. Pro adds Patrol control, alert investigation, verified fixes, and 90-day history.',
      plans: [
        {
          badge: 'Recommended',
          highlight: true,
          tierKicker: 'Relay',
          title: 'Relay',
          price: '$39/year',
          period: 'or $4.99/month',
          blurb: 'Secure remote access to your Pulse web UI, mobile app pairing, and push notifications.',
          features: [
            { tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' },
            { tone: 'check', html: 'Secure remote web access' },
            { tone: 'check', html: 'Mobile app pairing' },
            { tone: 'check', html: 'Push notifications' },
          ],
          buttons: [
            { kind: 'checkout', className: 'btn btn-primary', tier: 'relay', planKey: 'price_relay_annual', billingCycle: 'annual', label: 'Buy Annual' },
            { kind: 'checkout', className: 'btn btn-secondary', tier: 'relay', planKey: 'price_relay_monthly', billingCycle: 'monthly', label: 'Buy Monthly' },
          ],
        },
        {
          tierKicker: 'Pro',
          title: 'Pro',
          price: '$79/year',
          period: 'or $8.99/month',
          blurb: 'The operator tier for Patrol control, alert investigation, verified fixes, and 90-day history.',
          features: [
            { tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' },
            { tone: 'check', html: 'Everything in Relay' },
            { tone: 'check', html: 'Patrol investigates alerts' },
            { tone: 'check', html: 'Patrol fixes safe issues within your control level' },
            { tone: 'check', html: '90-day history' },
          ],
          buttons: [
            { kind: 'checkout', className: 'btn btn-primary', tier: 'pro', planKey: 'price_pro_annual', billingCycle: 'annual', label: 'Buy Annual' },
            { kind: 'checkout', className: 'btn btn-secondary', tier: 'pro', planKey: 'price_pro_monthly', billingCycle: 'monthly', label: 'Buy Monthly' },
          ],
        },
      ],
    });
    return;
  }

  if (request.method === 'GET' && route === '/v1/checkout/portal-handoff') {
    const portalHandoffID = String(url.searchParams.get('portal_handoff_id') || '').trim();
    if (
      portalHandoffID !== 'cph_preview_upgrade' &&
      portalHandoffID !== 'cph_preview_completed'
    ) {
      sendJSON(response, 400, { error: 'portal_handoff_id is invalid' });
      return;
    }
    sendJSON(response, 200, {
      portal_handoff_id: portalHandoffID,
      feature: 'self_hosted_plan',
      status: portalHandoffID === 'cph_preview_completed' ? 'completed' : 'resolved',
      resolved_at: Math.floor(Date.now() / 1000) - 30,
      expires_at: Math.floor(Date.now() / 1000) + 3600,
    });
    return;
  }

  if (request.method === 'GET' && route === '/v1/checkout/session') {
    sendJSON(response, 200, {
      status: 'fulfilled',
      owner_email: 'buyer@example.com',
      tier: 'pro',
      plan_key: 'price_pro_annual',
      activation_key_prefix: 'ppk_live_preview',
      max_monitored_systems: 0,
      current_period_end: iso('2027-03-20T10:00:00Z'),
    });
    return;
  }

  if (request.method !== 'POST') {
    sendText(response, 405, 'Method not allowed');
    return;
  }

  if (
    route === '/v1/manage/request' ||
    route === '/v1/retrieve-license/request' ||
    route === '/v1/gdpr/request-export' ||
    route === '/v1/gdpr/request-delete'
  ) {
    sendJSON(response, 200, { message: 'Preview code sent.' });
    return;
  }

  if (route === '/v1/manage') {
    sendJSON(response, 200, { url: successRedirect });
    return;
  }

  if (route === '/v1/retrieve-license') {
    sendJSON(response, 200, {
      license: {
        token: 'pulse_preview_1234567890',
        tier: 'Cloud Starter',
        issued_at: iso('2026-03-20T10:00:00Z'),
        expires_at: null,
        email: 'buyer@example.com',
        invoice_url: 'https://pulserelay.pro',
      },
    });
    return;
  }

  if (route === '/v1/gdpr/export') {
    sendJSON(response, 200, {
      email: 'buyer@example.com',
      export_generated_at: iso(new Date()),
      records: [
        { type: 'license', id: 'lic_preview' },
        { type: 'billing_profile', id: 'bill_preview' },
      ],
    });
    return;
  }

  if (route === '/v1/gdpr/confirm-delete') {
    sendJSON(response, 200, {
      deleted_count: 2,
      message: 'Preview deletion completed.',
      stripe_reminder: 'Stripe-managed payment data still requires provider-side deletion.',
    });
    return;
  }

  if (route === '/v1/self-refund') {
    sendJSON(response, 200, { ok: true });
    return;
  }

  if (route === '/v1/checkout/session') {
    readJSONBody(request).then(function(body) {
      const portalHandoffID = String(body.portal_handoff_id || '').trim();
      if (portalHandoffID === 'cph_preview_completed') {
        sendJSON(response, 409, { error: 'This secure upgrade handoff already completed. Return to the Plans page in Pulse to review the live plan state.' });
        return;
      }
      if (portalHandoffID !== 'cph_preview_upgrade') {
        sendJSON(response, 400, { error: 'Pulse Account could not verify the secure plan upgrade handoff.' });
        return;
      }
      const successURL = String(body.success_url || '').replace('{CHECKOUT_SESSION_ID}', 'cs_preview_success');
      sendJSON(response, 200, {
        url: successURL || successRedirect,
        plan_key: body.plan_key || 'price_pro_plus_annual',
        tier: body.tier || 'pro_plus',
        billing_cycle: body.billing_cycle || 'annual',
      });
    }).catch(function() {
      sendJSON(response, 400, { error: 'Invalid JSON.' });
    });
    return;
  }

  sendText(response, 404, 'Not found');
}

const server = http.createServer(function(request, response) {
  const url = new URL(request.url || '/', 'http://' + previewHost + ':' + String(previewPort));
  const scenario = resolveScenario(url, request);

  if (url.pathname === '/healthz') {
    sendText(response, 200, 'ok');
    return;
  }

  if (url.pathname === '/favicon.svg') {
    response.writeHead(200, {
      'Content-Type': 'image/svg+xml',
      'Cache-Control': 'no-store',
    });
    response.end(previewFaviconSVG);
    return;
  }

  if (url.pathname === '/favicon.ico') {
    response.writeHead(301, { Location: '/favicon.svg' });
    response.end();
    return;
  }

  if (url.pathname === '/__portal_preview/events') {
    response.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-store',
      Connection: 'keep-alive',
    });
    response.write('event: ready\n');
    response.write('data: {"ok":true}\n\n');
    sseClients.add(response);
    request.on('close', function() {
      sseClients.delete(response);
    });
    return;
  }

  if (url.pathname === '/api/portal/bootstrap') {
    sendJSON(response, 200, getScenarioState(scenario));
    return;
  }

  if (url.pathname === '/api/public/magic-link/request') {
    sendJSON(response, 200, { message: 'Preview mode: magic link skipped.' });
    return;
  }

  if (url.pathname === '/auth/logout') {
    sendJSON(response, 200, { ok: true });
    return;
  }

  if (url.pathname === '/api/portal/billing') {
    sendJSON(response, 200, {
      url: previewReturnURL(request, scenario, 'Preview mode: hosted billing portal would open here.'),
    });
    return;
  }

  if (url.pathname.startsWith('/api/accounts/')) {
    routeAccountAPI(request, response, url, getScenarioState(scenario), scenario);
    return;
  }

  const previewSetupMatch = url.pathname.match(/^\/__portal_preview\/workspaces\/([^/]+)\/setup$/);
  if (previewSetupMatch) {
    if (request.method !== 'POST') {
      sendText(response, 405, 'Method not allowed');
      return;
    }
    const workspaceID = decodeURIComponent(previewSetupMatch[1]);
    const result = applyPreviewWorkspaceSetupAction(
      getScenarioState(scenario),
      workspaceID,
      String(url.searchParams.get('action') || '').trim(),
    );
    if (!result.ok) {
      sendJSON(response, result.status || 400, { error: result.message || 'Preview setup action failed.' });
      return;
    }
    const redirectURL = previewWorkspaceURL(
      scenario,
      workspaceID,
      result.targetPath || String(url.searchParams.get('target_path') || ''),
    );
    const nextURL = new URL(redirectURL);
    nextURL.searchParams.set('preview_toast', result.message || 'Preview setup state updated.');
    response.writeHead(303, {
      Location: nextURL.pathname + nextURL.search,
      'Cache-Control': 'no-store',
    });
    response.end();
    return;
  }

  const previewWorkspaceMatch = url.pathname.match(/^\/__portal_preview\/workspaces\/([^/]+)$/);
  if (previewWorkspaceMatch) {
    response.writeHead(200, {
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'no-store',
    });
    response.end(buildPreviewWorkspaceHTML(
      getScenarioState(scenario),
      decodeURIComponent(previewWorkspaceMatch[1]),
      String(url.searchParams.get('target_path') || ''),
      scenario,
      String(url.searchParams.get('preview_toast') || ''),
    ));
    return;
  }

  if (url.pathname.startsWith('/__portal_preview/commercial/')) {
    routeCommercialAPI(request, response, url, scenario);
    return;
  }

  if (url.pathname === '/' || url.pathname === '/portal') {
    if (url.searchParams.get('reset') === '1') {
      resetScenario(scenario);
    }
    if (buildState.errors.length > 0 && !buildState.assets.js) {
      response.writeHead(500, {
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'no-store',
        'Set-Cookie': scenarioCookieName + '=' + encodeURIComponent(scenario) + '; Path=/; SameSite=Lax',
      });
      response.end('<!DOCTYPE html><html><body><pre>' + escapeHTML(buildErrorText(buildState.errors)) + '</pre></body></html>');
      return;
    }
    response.writeHead(200, {
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'no-store',
      'Set-Cookie': scenarioCookieName + '=' + encodeURIComponent(scenario) + '; Path=/; SameSite=Lax',
    });
    response.end(buildPreviewHTML(buildState.assets, getScenarioState(scenario), url.searchParams.get('preview_toast') || ''));
    return;
  }

  sendText(response, 404, 'Not found');
});

const buildContext = await context(createPortalBuildOptions({
  outfile: path.join(frontendRoot, '.portal-preview', 'portal_app.js'),
  write: false,
  logLevel: 'silent',
  plugins: [previewAssetsPlugin],
}));

await buildContext.rebuild();
await buildContext.watch();

server.listen(previewPort, previewHost, function() {
  console.log('[portal-preview] http://' + previewHost + ':' + String(previewPort));
  console.log('[portal-preview] managed    -> http://' + previewHost + ':' + String(previewPort) + '/?scenario=managed');
  console.log('[portal-preview] readonly   -> http://' + previewHost + ':' + String(previewPort) + '/?scenario=readonly');
  console.log('[portal-preview] selfhosted -> http://' + previewHost + ':' + String(previewPort) + '/?scenario=selfhosted');
  console.log('[portal-preview] empty      -> http://' + previewHost + ':' + String(previewPort) + '/?scenario=empty');
  console.log('[portal-preview] provider   -> http://' + previewHost + ':' + String(previewPort) + '/?scenario=provider');
  console.log('[portal-preview] mixed      -> http://' + previewHost + ':' + String(previewPort) + '/?scenario=mixed');
});

async function shutdown(signal) {
  console.log('[portal-preview] shutting down (' + signal + ')');
  for (const response of sseClients) {
    response.end();
  }
  server.close(function() {});
  await buildContext.dispose();
  process.exit(0);
}

process.on('SIGINT', function() {
  void shutdown('SIGINT');
});
process.on('SIGTERM', function() {
  void shutdown('SIGTERM');
});
