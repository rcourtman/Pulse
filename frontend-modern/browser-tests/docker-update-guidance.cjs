// Production Docs viewer and shipped Docker guide; no container action is run.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'docker-update-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-docker-update-guidance'),
    server: { host: '127.0.0.1', port: 5250, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    for (const [engine, width, dark] of [
      ['chromium', 1280, false],
      ['webkit', 390, true],
    ]) {
      browser = await { chromium, webkit }[engine].launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5250/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/DOCKER#before-updating-a-workload');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const waitForHeading = async (id) => {
        await page.waitForFunction((target) => {
          const heading = document.getElementById(target);
          if (!heading) return false;
          const rect = heading.getBoundingClientRect();
          return document.activeElement === heading && rect.top >= 0 && rect.top < innerHeight;
        }, id);
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'page must not overflow the viewport',
        );
      };
      await waitForHeading('before-updating-a-workload');
      // Theme colours transition after the root class changes. Capture settled
      // content, not an intermediate grey background on the phone fixture.
      await page.evaluate(
        () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
      );
      await page.waitForFunction(() => document.getAnimations().length === 0);
      const article = await page.locator('article').innerText();
      for (const text of [
        'independent, consistent backup',
        'not a backup of mounted data',
        'five minutes after a successful update',
        'Removal, rename or restart can fail too',
        'check the current state before retrying',
        'not a monitoring-only security boundary',
      ])
        assert.ok(article.replace(/\s+/g, ' ').includes(text), text);
      await page.screenshot({ path: path.join(artifacts, `${engine}-precautions.png`) });
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/DOCKER#safety-features');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      await waitForHeading('safety-features');
      await page.screenshot({ path: path.join(artifacts, `${engine}-limits.png`) });
      const link = page.getByRole('link', {
        name: 'container lifecycle requirements',
        exact: true,
      });
      const target = decodeURIComponent((await link.getAttribute('href')).split('#')[1]);
      await link.focus();
      await page.keyboard.press('Enter');
      await waitForHeading(target);
      assert.equal(
        await page
          .getByRole('heading', { name: '▶️ Container Lifecycle Actions', exact: true })
          .textContent(),
        '▶️ Container Lifecycle Actions',
      );

      // Follow the existing troubleshooting entry, then distinguish a refused
      // plan from an already-sent action before reaching any shell recipe.
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/TROUBLESHOOTING#choose-the-affected-workflow');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      await waitForHeading('choose-the-affected-workflow');
      await page.getByRole('link', { name: 'Docker workload update checks', exact: true }).focus();
      await page.keyboard.press('Enter');
      await waitForHeading('check-a-failed-or-pending-workload-update');
      assert.ok(page.url().endsWith('/docs/DOCKER#check-a-failed-or-pending-workload-update'));
      const stages = page.locator('#check-a-failed-or-pending-workload-update + p + ul');
      assert.equal(await stages.locator('li').count(), 3);
      const stageText = (await stages.innerText()).replace(/\s+/g, ' ');
      for (const phrase of [
        'refused that plan before dispatch',
        '409 alone has other causes',
        'does not establish what an earlier action did',
        'outcome is uncertain, even beside a failed banner',
        'do not assume nothing changed',
      ])
        assert.ok(stageText.includes(phrase), phrase);
      assert.ok(
        await page.evaluate(() => {
          const commands = document.getElementById('command-agent-is-not-connected');
          const uncertain = document.getElementById('action-was-sent-or-its-outcome-is-unknown');
          const firstCode = uncertain.nextElementSibling.nextElementSibling.nextElementSibling;
          return (
            commands.compareDocumentPosition(uncertain) & Node.DOCUMENT_POSITION_FOLLOWING &&
            firstCode.tagName === 'PRE'
          );
        }),
      );
      await page.screenshot({ path: path.join(artifacts, `${engine}-update-stages.png`) });
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/DOCKER#command-agent-is-not-connected');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      await waitForHeading('command-agent-is-not-connected');
      const authorityGuide = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of [
        "root identifies the collector's OS user, not its server-side permission",
        'commands legacy identifies an older, unmarked compatibility profile',
        'cannot authenticate the legacy command channel',
        'An absent runner alone does not rule out an authorised legacy command connection',
        'not proof of a broken collector',
        'your usual Docker or Podman administration tool',
        'Neither is a diagnostic prerequisite',
        'unknown is valid',
      ])
        assert.ok(authorityGuide.includes(phrase), phrase);
      await page.screenshot({ path: path.join(artifacts, `${engine}-authority.png`) });
      // The existing profile choices are optional operator decisions. Both
      // links must be usable by keyboard; do not exercise any install action.
      for (const [name, id] of [
        ['agent privilege model', 'agent-privilege-model'],
        ['least-privilege profile', 'least-privilege-agent-profile'],
      ]) {
        await page.getByRole('link', { name, exact: true }).focus();
        if (id === 'agent-privilege-model') {
          await page.screenshot({ path: path.join(artifacts, `${engine}-profile-choice.png`) });
        }
        await page.keyboard.press('Enter');
        await waitForHeading(id);
        assert.ok(page.url().endsWith(`/docs/AGENT_SECURITY#${id}`));
        await page.evaluate(() => {
          history.pushState({}, '', '/docs/DOCKER#command-agent-is-not-connected');
          dispatchEvent(new PopStateEvent('popstate'));
        });
        await waitForHeading('command-agent-is-not-connected');
      }
      await page
        .getByRole('link', {
          name: 'command-channel connection and admission checks',
          exact: true,
        })
        .focus();
      await page.keyboard.press('Enter');
      await waitForHeading('commands-enabled-but-remote-control-blocked');
      assert.ok(
        page.url().endsWith('/docs/UNIFIED_AGENT#commands-enabled-but-remote-control-blocked'),
      );
      const connectionGuide = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      assert.ok(connectionGuide.includes('not a connected session'));
      assert.ok(connectionGuide.includes('Keep saved identity and credentials intact'));
      assert.deepEqual(errors, []);
      results.push({
        engine,
        version: browser.version(),
        width,
        dark,
        precautionsAndLimitsRendered: true,
        keyboardRequirementsLink: true,
        refusedSentUnknownStages: true,
        commandGuidanceBeforeShellChecks: true,
        keyboardTroubleshootingAndCommandChannelLinks: true,
        legacyCredentialAndIndependentRunnerDistinguished: true,
        keyboardOptionalProfileLinks: true,
        headingsFocusedAndVisible: true,
        noHorizontalOverflow: true,
        errors,
      });
      await browser.close();
      browser = undefined;
    }
    const result = {
      result: 'passed',
      playwright: require('playwright/package.json').version,
      scope: 'shipped help rendering and keyboard navigation, not native updates or rollback',
      documents: Object.fromEntries(
        ['DOCKER', 'UNIFIED_AGENT', 'TROUBLESHOOTING'].map((name) => [
          name,
          crypto
            .createHash('sha256')
            .update(fs.readFileSync(path.join(root, 'public/docs', `${name}.md`)))
            .digest('hex'),
        ]),
      ),
      results,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
