import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { context } from 'esbuild';

import { createPortalBuildOptions, frontendRoot } from './build_config.mjs';

const scenarioCookieName = 'pulse_portal_preview_scenario';
const previewHost = process.env.PULSE_PORTAL_PREVIEW_HOST || '127.0.0.1';
const previewPort = Number(process.env.PULSE_PORTAL_PREVIEW_PORT || '8765');
const previewScenarios = ['managed', 'readonly', 'selfhosted', 'empty'];
const previewFaviconSVG = fs.readFileSync(path.join(frontendRoot, '..', '..', 'favicon.svg'), 'utf8');
const previewFaviconHref = '/favicon.svg?v=' + createHash('sha256').update(previewFaviconSVG).digest('hex').slice(0, 16);

function iso(value) {
  return new Date(value).toISOString();
}

function deepClone(value) {
  return JSON.parse(JSON.stringify(value));
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
        workspaces: [
          {
            id: 'ws_alpha',
            display_name: 'MSP Test Workspace A',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
            created_at: iso('2026-03-20T10:00:00Z'),
          },
          {
            id: 'ws_beta',
            display_name: 'MSP Test Workspace B',
            state: 'active',
            healthy: true,
            health_status: 'healthy',
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
        workspaces: [],
        members: [
          { email: 'owner@example.com', role: 'owner', user_id: 'u_owner' },
        ],
      }],
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
      workspaces: [
        {
          id: 'ws_alpha',
          display_name: 'MSP Test Workspace A',
          state: 'suspended',
          healthy: false,
          health_status: 'unhealthy',
          created_at: iso('2026-03-20T10:00:00Z'),
        },
        {
          id: 'ws_beta',
          display_name: 'MSP Test Workspace B',
          state: 'active',
          healthy: true,
          health_status: 'healthy',
          created_at: iso('2026-03-21T10:00:00Z'),
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
    .replace(/>/g, '&gt;');
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

function previewTargetLabel(targetPath) {
  if (targetPath === '/settings/infrastructure?add=linux-host') {
    return 'Settings -> Infrastructure, with the agent install drawer open';
  }
  if (targetPath === '/settings/support/reporting') {
    return 'Settings -> Reporting for this client workspace';
  }
  return 'the workspace dashboard';
}

function buildPreviewWorkspaceHTML(bootstrap, workspaceID, targetPath, scenario) {
  const entry = findWorkspaceByID(bootstrap, workspaceID);
  const title = entry ? entry.workspace.display_name : workspaceID;
  const accountName = entry ? entry.account.name : 'Pulse Account';
  const targetLabel = previewTargetLabel(targetPath);
  const portalURL = '/?scenario=' + encodeURIComponent(scenario);
  return '<!DOCTYPE html>' +
    '<html lang="en">' +
      '<head>' +
        '<meta charset="utf-8">' +
        '<meta name="viewport" content="width=device-width, initial-scale=1">' +
        '<title>' + escapeHTML(title) + ' - Preview workspace</title>' +
        '<style>' +
          'body{margin:0;font-family:Inter,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f6f7f9;color:#101828}' +
          'main{max-width:760px;margin:0 auto;padding:48px 20px}' +
          '.panel{background:#fff;border:1px solid #d0d5dd;border-radius:8px;padding:24px;box-shadow:0 1px 2px rgba(16,24,40,.06)}' +
          'h1{margin:0 0 8px;font-size:24px;letter-spacing:0}' +
          'p{margin:0 0 16px;color:#475467;line-height:1.5}' +
          '.facts{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:20px 0}' +
          '.fact{border:1px solid #eaecf0;border-radius:6px;padding:10px;background:#fcfcfd}' +
          '.fact span{display:block;font-size:12px;color:#667085}' +
          '.fact strong{display:block;margin-top:2px;font-size:13px}' +
          'a{display:inline-flex;align-items:center;min-height:34px;padding:0 12px;border-radius:6px;background:#155eef;color:#fff;text-decoration:none;font-size:14px;font-weight:600}' +
          '@media(max-width:640px){.facts{grid-template-columns:1fr}}' +
        '</style>' +
      '</head>' +
      '<body>' +
        '<main>' +
          '<section class="panel">' +
            '<p>Preview workspace handoff</p>' +
            '<h1>' + escapeHTML(title) + '</h1>' +
            '<p>This preview page stands in for the hosted client workspace. In production the signed handoff creates a session inside the client workspace and opens ' + escapeHTML(targetLabel) + '.</p>' +
            '<div class="facts">' +
              '<div class="fact"><span>Account</span><strong>' + escapeHTML(accountName) + '</strong></div>' +
              '<div class="fact"><span>Workspace</span><strong>' + escapeHTML(workspaceID) + '</strong></div>' +
              '<div class="fact"><span>Target</span><strong>' + escapeHTML(targetPath || '/') + '</strong></div>' +
            '</div>' +
            '<p>Agents, alerts, and reports are scoped to this workspace after the handoff.</p>' +
            '<a href="' + escapeHTML(portalURL) + '">Back to Pulse Account</a>' +
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
        sendJSON(response, 404, { error: 'Workspace not found.' });
        return;
      }
      if (workspace.state !== 'active') {
        sendJSON(response, 409, { error: 'Workspace is not active.' });
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
          sendJSON(response, 400, { error: 'Workspace name is required.' });
          return;
        }
        account.workspaces = account.workspaces || [];
        account.workspaces.push({
          id: 'ws_' + Math.random().toString(36).slice(2, 10),
          display_name: displayName,
          state: 'active',
          healthy: true,
          health_status: 'healthy',
          created_at: iso(new Date()),
        });
        sendJSON(response, 200, { ok: true });
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
          sendJSON(response, 404, { error: 'Workspace not found.' });
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
      description: 'Community keeps core monitoring free. Relay gives secure remote access to the Pulse web UI, mobile app pairing, push notifications, and 14-day history. Pro adds root-cause analysis, safe remediation workflows, and 90-day history.',
      explainer:
        'Community keeps core monitoring free. Relay gets your Pulse web UI securely reachable from anywhere. Pro adds root-cause analysis, safe remediation workflows, and 90-day history.',
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
          blurb: 'The operator tier for root-cause analysis, safe remediation workflows, and 90-day history.',
          features: [
            { tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' },
            { tone: 'check', html: 'Everything in Relay' },
            { tone: 'check', html: 'Alert-triggered root-cause analysis' },
            { tone: 'check', html: 'Safe remediation workflows with approval or autonomous mode' },
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
