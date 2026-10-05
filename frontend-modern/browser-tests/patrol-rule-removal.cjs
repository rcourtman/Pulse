// Full production Patrol surface. Only local API responses are synthetic.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const files = [
  'src/api/patrol.ts',
  'src/utils/apiClient.ts',
  'src/components/AI/FindingsPanel.tsx',
  'src/features/patrol/PatrolIntelligenceSurface.tsx',
  'src/features/patrol/PatrolAttentionWorkbench.tsx',
  'src/features/patrol/PatrolSuppressionRules.tsx',
];
const output =
  require.main === module
    ? '/workspace/tmp/patrol-rule-proof/browser'
    : '/workspace/tmp/patrol-rule-proof/visual';
const report = {
  playwright: require('playwright/package.json').version,
  content_sha256: {},
  cases: [],
  captures: [],
  cleanup: {},
};
fs.mkdirSync(output, { recursive: true });
for (const file of files)
  report.content_sha256[`frontend-modern/${file}`] = hash(`/workspace/frontend-modern/${file}`);
assert.equal(
  report.playwright,
  JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);

async function journey(root, engine, width, parent = false, resume = false, visualOnly = false) {
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, `vite-${parent ? 'parent' : engine}-${width}`),
    server: { host: '127.0.0.1', port: 5297, strictPort: true, watch: null },
  });
  let browser;
  let page;
  const result = {
    parent,
    engine,
    width,
    checks: [],
    requests: [],
    errors: [],
    offOrigin: [],
    cleanup: {},
  };
  report.cases.push(result);
  const now = '2026-10-05T12:00:00Z';
  const broad = {
    id: 'rule_any_any_2',
    resource_id: '',
    category: '',
    description: 'Intentional maintenance',
    created_from: 'manual',
    created_at: now,
  };
  const other = {
    id: 'rule_keep_capacity_3',
    resource_id: 'fixture-keep',
    resource_name: 'Keep VM',
    category: 'capacity',
    description:
      visualOnly === 'long'
        ? 'Intentional maintenance on this resource; keep the documented scope.\n'.repeat(30)
        : 'Keep this unrelated rule',
    created_from: 'manual',
    created_at: now,
  };
  const historical = {
    id: 'finding_history',
    finding_id: 'history',
    resource_id: 'fixture-db',
    resource_name: 'Database VM',
    category: 'backup',
    description: 'Retain this dismissal and note',
    created_from: 'dismissed',
    created_at: now,
  };
  const legacy = { ...other, id: 'rule_legacy', created_from: 'suppress' };
  const rules = [broad, other, historical, legacy];
  const initialUntouched = JSON.stringify(rules);
  const item = {
    id: 'record-1',
    operationalRecordId: 'record-1',
    subjectResourceId: 'fixture-db',
    subjectResourceName: 'Database VM',
    subjectResourceType: 'vm',
    kind: 'backup',
    title: 'Backup age on Database VM',
    plainLanguageSummary: 'The off-site backup is older than expected.',
    severity: 'warning',
    state: 'open',
    firstObservedAt: now,
    lastObservedAt: now,
    evidenceFreshness: 'fresh',
    evidenceCompleteness: 'complete',
    impact: 'Check the backup schedule.',
    relatedResources: [],
    recommendedNextStep: 'Review the off-site backup.',
    availableActions: [],
    verificationState: 'not_available',
  };
  const finding = {
    id: 'fixture-finding',
    resource_id: 'fixture-db',
    resource_name: 'Database VM',
    resource_type: 'vm',
    category: 'backup',
    severity: 'warning',
    title: 'Off-site backup age',
    description: 'The off-site backup is older than expected.',
    detected_at: now,
    last_seen_at: now,
    times_raised: 1,
    suppressed: false,
    auto_resolved: false,
    mirrors_alert_id: 'fixture-db::backup',
    mirrors_alert_type: 'backup',
  };
  const summary = {
    activeCount: 1,
    openCount: 1,
    acknowledgedCount: 0,
    suppressedCount: 0,
    uncertainCount: 0,
    resolvedCount: 0,
    calm: false,
    coverageState: 'current',
    evaluatedAt: now,
  };
  let loseResponse = false;
  let denied = false;
  let created;
  try {
    await server.listen();
    browser =
      engine === 'webkit'
        ? await webkit.launch({ headless: true })
        : await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.browser = browser.version();
    page = await browser.newPage({
      viewport: { width, height: 900 },
      colorScheme: width === 320 ? 'dark' : 'light',
      isMobile: width === 320,
      hasTouch: width === 320,
    });
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => result.errors.push(error.message));
    const origin = 'http://127.0.0.1:5297';
    await page.context().addCookies([
      { name: 'pulse_csrf', value: 'synthetic-csrf', url: origin },
      { name: 'pulse_session', value: 'synthetic-session', url: origin },
    ]);
    await page.addInitScript(() => {
      if (matchMedia('(prefers-color-scheme: dark)').matches)
        document.documentElement.classList.add('dark');
    });
    await page.routeWebSocket('**/ws*', () => {});
    await page.route('**/*', async (route) => {
      const request = route.request(),
        url = new URL(request.url()),
        method = request.method(),
        p = url.pathname;
      if (url.origin !== origin) {
        result.offOrigin.push(url.origin);
        return route.abort();
      }
      if (!p.startsWith('/api/')) return route.continue();
      const org = request.headers()['x-pulse-org-id'];
      result.requests.push({ path: p, method, org });
      if (p === '/api/ai/patrol/suppressions') {
        assert.equal(org, 'fixture-tenant-a');
        if (method === 'GET') {
          if (denied)
            return route.fulfill({
              status: 403,
              json: { error: 'access_revoked' },
              headers: { 'X-CSRF-Token': 'synthetic-replacement' },
            });
          return route.fulfill({ json: rules });
        }
        assert.equal(method, 'POST');
        assert.equal(request.headers()['x-csrf-token'], 'synthetic-csrf');
        const input = request.postDataJSON();
        assert.equal(input.resource_id, 'fixture-db');
        assert.equal(input.category, 'backup');
        assert.equal(input.allow_broad_scope, undefined);
        created = {
          ...input,
          id: 'rule_fixture-db_backup_1',
          created_from: 'manual',
          created_at: now,
        };
        rules.push(created);
        return route.fulfill({ json: { success: true, rule: created } });
      }
      if (p.startsWith('/api/ai/patrol/suppressions/')) {
        assert.equal(method, 'DELETE');
        assert.equal(org, 'fixture-tenant-a');
        assert.equal(request.headers()['x-csrf-token'], 'synthetic-csrf');
        const id = decodeURIComponent(p.slice('/api/ai/patrol/suppressions/'.length));
        const index = rules.findIndex((r) => r.id === id);
        assert(index >= 0);
        assert.equal(rules[index].created_from, 'manual');
        rules.splice(index, 1);
        if (loseResponse) {
          loseResponse = false;
          return route.abort('failed');
        }
        return route.fulfill({ json: { success: true } });
      }
      assert.equal(method, 'GET', `unexpected mutation ${p}`);
      let json = {};
      if (p === '/api/settings/ai')
        json = {
          patrol_enabled: true,
          patrol_autonomy_level: 'monitor',
          patrol_readiness: { status: 'ready', ready: true, cause: 'none' },
        };
      else if (p === '/api/ai/patrol/status')
        json = {
          enabled: true,
          runtime_state: 'idle',
          running: false,
          readiness: { status: 'ready', ready: true, cause: 'none' },
          summary: { active: 1, critical: 0, warning: 1, info: 0 },
        };
      else if (p === '/api/ai/patrol/attention')
        json = { data: [item], summary, meta: { page: 1, limit: 50, total: 1, totalPages: 1 } };
      else if (p === '/api/ai/patrol/attention/summary') json = summary;
      else if (p === '/api/ai/patrol/attention/record-1')
        json = {
          item,
          operationalRecord: {
            id: item.id,
            kind: item.kind,
            state: item.state,
            severity: item.severity,
            title: item.title,
            subjectResourceId: item.subjectResourceId,
            subjectResourceName: item.subjectResourceName,
            createdAt: now,
            updatedAt: now,
          },
          timeline: [],
          evidence: [],
        };
      else if (p === '/api/ai/patrol/autonomy')
        json = { autonomy_level: 'monitor', full_mode_unlocked: false };
      else if (p === '/api/ai/patrol/findings')
        json = [
          finding,
          {
            ...finding,
            id: 'fixture-independent',
            title: 'Independent backup check',
            mirrors_alert_id: '',
            mirrors_alert_type: '',
          },
        ];
      else if (p === '/api/ai/patrol/digest')
        json = {
          generated_at: now,
          window: { start: now, end: now, days: 7, history_complete: true },
          mode: 'monitor',
          runs: {
            total: 0,
            scheduled: 0,
            event_triggered: 0,
            manual: 0,
            failed: 0,
            checks: 0,
            resources_covered: 0,
          },
          findings: {
            new: 0,
            open_by_severity: { critical: 0, warning: 0, watch: 0, info: 0 },
            resolved: 0,
            auto_resolved: 0,
            dismissed: 0,
            suppressed: 0,
          },
          investigations: { total: 0, by_outcome: {} },
          actions: {
            proposed: 0,
            approved: 0,
            rejected: 0,
            executed: 0,
            verified: 0,
            failed: 0,
            pending: 0,
          },
          alerts: { reviewed: 0 },
          spend: {
            estimated_usd: 0,
            pricing_known: true,
            input_tokens: 0,
            output_tokens: 0,
            calls: 0,
          },
        };
      else if (p === '/api/ai/patrol/runs') json = [];
      else if (p === '/api/ai/unified/findings') json = { findings: [] };
      else if (p === '/api/ai/approvals') json = { approvals: [] };
      else if (p === '/api/ai/models') json = { models: [] };
      else if (p === '/api/license/runtime-capabilities')
        json = {
          capabilities: ['ai_patrol'],
          limits: [],
          hosted_mode: false,
          max_history_days: 7,
          runtime: { build: 'community', label: 'Pulse Community runtime' },
          blocked_capabilities: [],
        };
      return route.fulfill({ json });
    });
    const capture = async (name) => {
      const file = path.join(
        output,
        `${parent ? 'parent' : engine}-${width}-${visualOnly === 'long' ? 'long-' : ''}${name}.png`,
      );
      await page.screenshot({
        path: file,
        fullPage: !visualOnly,
        animations: visualOnly ? 'disabled' : 'allow',
      });
      report.captures.push({ path: file.replace('/workspace/', ''), sha256: hash(file) });
      const dimensions = await page.evaluate(() => ({
        inner: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      assert(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
    };
    await page.goto(`${origin}/browser-tests/patrol-rule-removal.html`, {
      waitUntil: 'domcontentloaded',
      timeout: 60000,
    });
    let dialog;
    if (visualOnly) {
      await page.getByRole('tab', { name: 'Activity' }).click();
      await page.locator('#patrol-suppression-rules > summary').click();
      await page.getByRole('button', { name: 'Remove rule for Keep VM, capacity' }).waitFor();
      // App theme initialisation can override an early init script. Verify the
      // actual rendered theme after it has mounted, not just colour preference.
      if (width === 320) await page.evaluate(() => document.documentElement.classList.add('dark'));
      result.renderedTheme = await page.evaluate(() => ({
        dark: document.documentElement.classList.contains('dark'),
        panelBackground: getComputedStyle(document.querySelector('#patrol-suppression-rules'))
          .backgroundColor,
      }));
      assert.equal(result.renderedTheme.dark, width === 320);
      result.dialogs = [];
      const visualCases = [
        ['Remove rule for Keep VM, capacity', 'rule_keep_capacity_3', 'specific-viewport'],
        ['Remove rule for All resources, All categories', 'rule_any_any_2', 'wildcard-viewport'],
      ];
      for (const [label, expected, name] of visualOnly === 'long'
        ? visualCases.slice(0, 1)
        : visualCases) {
        await page.getByRole('button', { name: label }).click();
        dialog = page.getByRole('dialog', { name: 'Remove suppression rule?' });
        await dialog.getByText(expected, { exact: true }).waitFor();
        // Complete the finite entrance animation before measuring and capture
        // only the user's viewport; full-page capture reflows fixed overlays.
        await dialog.evaluate((e) =>
          e.getAnimations({ subtree: true }).forEach((a) => {
            try {
              a.finish();
            } catch {}
          }),
        );
        await page.screenshot({
          path: path.join(output, `intermediate-${engine}.png`),
          animations: 'disabled',
        });
        const box = await dialog.boundingBox();
        assert(box, 'Dialog has a rendered box');
        assert(
          box.x >= -1 && box.y >= -1 && box.x + box.width <= width + 1 && box.y + box.height <= 901,
          JSON.stringify(box),
        );
        assert(
          await dialog
            .getByRole('button', { name: 'Cancel' })
            .evaluate((e) => e === document.activeElement),
        );
        await capture(name);
        const actions = await dialog
          .getByRole('button', { name: 'Remove this rule' })
          .boundingBox();
        assert(
          actions &&
            actions.y >= box.y &&
            actions.y + actions.height <= box.y + box.height + 1 &&
            actions.y + actions.height <= 901,
          'Confirmation action must not be clipped: ' + JSON.stringify({ box, actions }),
        );
        if (visualOnly === 'long') {
          const region = dialog.getByRole('region', { name: 'Rule scope and reason' });
          const initial = await region.evaluate((e) => ({
            top: e.scrollTop,
            height: e.clientHeight,
            scrollHeight: e.scrollHeight,
          }));
          assert.equal(initial.top, 0, 'Safe autofocus must not scroll past the rule scope');
          assert(
            initial.scrollHeight > initial.height,
            'Long reason needs a real scrolling region',
          );
          for (const text of ['Keep VM', 'fixture-keep', 'capacity', expected]) {
            const scopeBox = await region.getByText(text, { exact: true }).boundingBox();
            const regionBox = await region.boundingBox();
            assert(
              scopeBox &&
                regionBox &&
                scopeBox.y >= regionBox.y &&
                scopeBox.y + scopeBox.height <= regionBox.y + regionBox.height + 1,
              'The scope and exact ID are initially in view: ' + text,
            );
          }
          const input =
            engine === 'webkit' && width === 320 ? 'keyboard PageDown' : 'pointer wheel';
          if (input === 'keyboard PageDown') {
            // Playwright mobile WebKit does not implement mouse wheel. Use
            // its supported native keyboard scroll on the focusable region.
            await region.focus();
            await page.keyboard.press('PageDown');
          } else {
            await region.hover();
            await page.mouse.wheel(0, 3000);
          }
          await page.waitForFunction(
            () => document.querySelector('[aria-label="Rule scope and reason"]').scrollTop > 0,
          );
          const scrolled = await region.evaluate((e) => e.scrollTop);
          await capture('long-reason-scrolled');
          const footerAfter = await dialog
            .getByRole('button', { name: 'Remove this rule' })
            .boundingBox();
          assert.deepEqual(footerAfter, actions, 'The action footer must not move with the reason');
          await region.focus();
          await page.keyboard.press('Home');
          await page.waitForFunction(
            () => document.querySelector('[aria-label="Rule scope and reason"]').scrollTop === 0,
          );
          result.longReason = {
            initial,
            scrollInput: input,
            scrollPosition: scrolled,
            keyboardHomeReturned: true,
            stableFooter: true,
          };
          result.checks.push(
            'Long reason supports the recorded scroll input and keyboard Home, with initial scope/ID visible and a stable safe footer.',
          );
        }
        result.dialogs.push({ name, ruleId: expected, box, viewport: { width, height: 900 } });
        await page.keyboard.press('Escape');
        await dialog.waitFor({ state: 'detached' });
      }
      assert.equal(result.requests.filter((r) => r.method !== 'GET').length, 0);
      assert.deepEqual(result.errors, []);
      assert.deepEqual(result.offOrigin, []);
      result.checks.push(
        'Exact-ID and wildcard confirmation panels and their cancel/remove controls are inside the actual viewport; no POST or DELETE was replayed.',
      );
      return;
    }
    if (!resume) {
      await page.getByRole('button', { name: 'Open Database VM · Backup age' }).click();
      await page
        .getByRole('list', { name: 'Lasting decisions' })
        .getByRole('button', { name: 'Create rule', exact: true })
        .click();
      const form = page.getByRole('form', { name: 'Confirm Create rule' });
      assert(await form.getByRole('button', { name: 'Confirm: Create rule' }).isDisabled());
      await form.getByLabel(/Why this rule/).fill('Backups are deliberately held off-site');
      await form.getByRole('button', { name: 'Confirm: Create rule' }).click();
      await page.getByRole('status').filter({ hasText: 'Rule created.' }).waitFor();
      assert(created);
      result.checks.push(
        'Production Inbox Create rule sends one narrow scoped POST with a written reason.',
      );
      if (parent) {
        assert.equal(await page.getByRole('link', { name: 'Manage suppression rules' }).count(), 0);
        await page.getByRole('tab', { name: 'Activity' }).click();
        assert.equal(await page.locator('#patrol-suppression-rules').count(), 0);
        assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, 0);
        await capture('no-reversal');
        result.checks.push(
          'Exact assigned parent creates a permanent rule but has no manual-rule reversal control.',
        );
        return;
      }
      await page.getByRole('link', { name: 'Manage suppression rules' }).click();
      await page.getByRole('button', { name: 'Remove rule for Database VM, backup' }).waitFor();
      assert.equal(await page.locator('#patrol-suppression-rules').getAttribute('open'), '');
      await capture('manual-list');
      const select = () =>
        page.getByRole('button', { name: 'Remove rule for Database VM, backup' });
      await select().click();
      dialog = page.getByRole('dialog', { name: 'Remove suppression rule?' });
      await dialog.getByText(created.id, { exact: true }).waitFor();
      assert(
        await dialog
          .getByRole('button', { name: 'Cancel' })
          .evaluate((e) => e === document.activeElement),
      );
      await capture('exact-confirmation');
      await page.keyboard.press('Escape');
      await dialog.waitFor({ state: 'detached' });
      assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, 0);
      await select().click();
      await page.getByRole('dialog').getByRole('button', { name: 'Remove this rule' }).click();
      await page.getByRole('status').filter({ hasText: 'Rule removed.' }).waitFor();
      assert.equal(JSON.stringify(rules), initialUntouched);
      assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, 1);
      assert.equal(await select().count(), 0);
      await capture('removed-readback');
      result.checks.push(
        'Cancellation is non-mutating; exact manual deletion requires pre-read and post-read and preserves unrelated rules and finding-backed history.',
      );
    } else {
      await page.getByRole('tab', { name: 'Activity' }).click();
      await page.locator('#patrol-suppression-rules > summary').click();
      await page.getByRole('button', { name: 'Remove rule for Keep VM, capacity' }).waitFor();
      result.checks.push(
        'Continue completed desktop creation/removal proof at the remaining controls; no replay of its POST or exact-ID removal.',
      );
    }
    // The second existing creation control points at the same reversal flow.
    await page.getByRole('button', { name: 'Finding options and history' }).click();
    await page
      .getByRole('button', { name: 'Finding options for Independent backup check' })
      .click();
    await page.getByRole('button', { name: 'Create rule from this' }).click();
    await page.getByRole('textbox', { name: 'Reason for suppression rule' }).waitFor();
    assert.equal(
      await page.getByRole('link', { name: 'Manage suppression rules' }).getAttribute('href'),
      '/patrol/activity#patrol-suppression-rules',
    );
    await page.getByText(/Reopen finding only undoes an individual dismissal/).waitFor();
    await capture('finding-create-link');
    await page.getByRole('button', { name: 'Cancel', exact: true }).click();
    result.checks.push(
      'Production Finding options create control has an accessible reason field and the same Activity reversal link; no second POST.',
    );

    await page
      .getByRole('button', { name: 'Remove rule for All resources, All categories' })
      .click();
    dialog = page.getByRole('dialog');
    await dialog.getByText('This broad rule covers all resources in all categories.').waitFor();
    await capture('wildcard-confirmation');
    loseResponse = true;
    await dialog.getByRole('button', { name: 'Remove this rule' }).click();
    await page.getByRole('alert').filter({ hasText: 'Removal could not be confirmed.' }).waitFor();
    assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, resume ? 1 : 2);
    await capture('uncertain-not-success');
    await page.getByRole('button', { name: 'Reload rules' }).click();
    await page.getByRole('button', { name: 'Remove rule for Keep VM, capacity' }).waitFor();
    assert.equal(
      await page
        .getByRole('button', { name: 'Remove rule for All resources, All categories' })
        .count(),
      0,
    );
    assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, resume ? 1 : 2);
    result.checks.push(
      'A lost DELETE response never retries or claims removal; explicit reload reconciles absence and keeps other decisions.',
    );

    await page.getByRole('button', { name: 'Remove rule for Keep VM, capacity' }).click();
    denied = true;
    await page.getByRole('dialog').getByRole('button', { name: 'Remove this rule' }).click();
    await page.getByRole('alert').filter({ hasText: 'Removal could not be confirmed.' }).waitFor();
    assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, resume ? 1 : 2);
    denied = false;
    await page.getByRole('button', { name: 'Reload rules' }).click();
    await page.getByRole('button', { name: 'Remove rule for Keep VM, capacity' }).click();
    await page.evaluate(() => window.__patrolRuleRemoval.switchOrg());
    await page.getByRole('alert').filter({ hasText: 'Organisation or access changed.' }).waitFor();
    assert.equal(await page.getByRole('dialog').count(), 0);
    assert.equal(await page.getByRole('button', { name: /^Remove rule for/ }).count(), 0);
    assert.equal(result.requests.filter((r) => r.method === 'DELETE').length, resume ? 1 : 2);
    await capture('scope-invalidated');
    result.checks.push(
      'Revoked access blocks before DELETE, then organisation switch discards pending confirmation and old rule data. No cross-tenant action or automatic retry.',
    );
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.offOrigin, []);
  } catch (error) {
    result.failure = error.message;
    if (page) {
      result.pageState = await page
        .evaluate(() => ({ url: location.href, text: document.body.innerText.slice(0, 12000) }))
        .catch(() => null);
      const file = path.join(output, `${engine}-${width}-failure.png`);
      await page.screenshot({ path: file, fullPage: true }).catch(() => {});
      if (fs.existsSync(file))
        report.captures.push({ path: file.replace('/workspace/', ''), sha256: hash(file) });
    }
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
    result.cleanup = { browser_closed: Boolean(browser), server_closed: true };
  }
}
module.exports = { journey, report, output, hash };
if (require.main === module)
  (async () => {
    try {
      // Completed parent and desktop creation/removal proof are retained; do not replay them.
      await journey('/workspace/frontend-modern', 'chromium', 1365, false, true);
      await journey('/workspace/frontend-modern', 'webkit', 320);
      report.result = 'passed';
    } catch (error) {
      report.result = 'failed';
      report.failure = error.message;
      throw error;
    } finally {
      report.cleanup = {
        all_cases_closed: report.cases.every(
          (c) => c.cleanup.browser_closed && c.cleanup.server_closed,
        ),
      };
      fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
    }
    console.log(
      JSON.stringify({
        result: report.result,
        cases: report.cases.length,
        captures: report.captures.length,
        cleanup: report.cleanup,
      }),
    );
  })().catch((error) => {
    console.error(error.stack);
    process.exitCode = 1;
  });
