// Source-bound offline consumer checks; no live connection test or guest request.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = '/workspace/tmp/collection-browser';
  fs.mkdirSync(artifacts, { recursive: true });
  const inputs = [
    'frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx',
    'frontend-modern/src/components/Settings/useNodeModalState.ts',
    'frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx',
    'frontend-modern/src/utils/nodeModalPresentation.ts',
    'frontend-modern/src/components/shared/InlineNotice.tsx',
    'frontend-modern/src/components/shared/Toggle.tsx',
    'frontend-modern/src/components/shared/useToggleState.ts',
    'frontend-modern/src/index.css',
    'frontend-modern/package.json',
    'frontend-modern/package-lock.json',
    'frontend-modern/browser-tests/proxmox-collection-safety.tsx',
    'frontend-modern/browser-tests/proxmox-collection-safety.html',
    'frontend-modern/browser-tests/proxmox-collection-safety.cjs',
  ];
  const hash = (raw) => crypto.createHash('sha256').update(raw).digest('hex');
  const inputHashes = Object.fromEntries(
    inputs.map((file) => [file, hash(fs.readFileSync(path.join('/workspace', file)))]),
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5243, strictPort: true },
  });
  const cases = [];
  const accessibleDescription = async (locator) =>
    locator.evaluate((node) =>
      (node.getAttribute('aria-describedby') || '')
        .split(/\s+/)
        .filter(Boolean)
        .map((id) => {
          const help = document.getElementById(id);
          if (!help) throw new Error(`Missing description ${id}`);
          return help.textContent.trim().replace(/\s+/g, ' ');
        })
        .join(' '),
    );
  const observedAX = async (page, locator) => {
    const selector = await locator.evaluate((node) => {
      const attr = node.hasAttribute('aria-labelledby') ? 'aria-labelledby' : 'aria-describedby';
      return `[${attr}="${node.getAttribute(attr)}"]`;
    });
    const session = await page.context().newCDPSession(page);
    try {
      const { root } = await session.send('DOM.getDocument');
      const { nodeId } = await session.send('DOM.querySelector', { nodeId: root.nodeId, selector });
      const { nodes } = await session.send('Accessibility.getPartialAXTree', {
        nodeId,
        fetchRelatives: false,
      });
      assert.equal(nodes.length, 1);
      return {
        role: nodes[0].role?.value,
        name: nodes[0].name?.value,
        description: nodes[0].description?.value,
        properties: nodes[0].properties,
      };
    } finally {
      await session.detach();
    }
  };
  const result = {
    inputHashes,
    playwright: require('playwright/package.json').version,
    graph: {
      solid: JSON.parse(fs.readFileSync(path.join(root, 'node_modules/solid-js/package.json')))
        .version,
      seroval: JSON.parse(fs.readFileSync(path.join(root, 'node_modules/seroval/package.json')))
        .version,
    },
    cases,
  };
  let browser;
  try {
    await server.listen();
    for (const engine of ['chromium', 'webkit']) {
      browser = await { chromium, webkit }[engine].launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const phone = engine === 'webkit';
      const viewport = phone ? { width: 390, height: 844 } : { width: 1280, height: 900 };
      for (const type of ['pve', 'pbs', 'pmg']) {
        const page = await browser.newPage({
          viewport,
          ...(phone ? { isMobile: true, hasTouch: true } : {}),
        });
        const errors = [];
        const requests = [];
        const row = {
          engine,
          type,
          viewport,
          phase: 'initial-render',
          errors,
          requests,
          consoleErrors: [],
        };
        cases.push(row);
        page.on('console', (message) => {
          if (message.type() === 'error') row.consoleErrors.push(message.text());
        });
        page.on('pageerror', (error) => errors.push(error.message));
        await page.route('**/api/**', async (route) => {
          // Vite also serves /src/api/*.ts modules; those are source, not live APIs.
          if (!new URL(route.request().url()).pathname.startsWith('/api/')) return route.continue();
          requests.push({
            method: route.request().method(),
            path: new URL(route.request().url()).pathname,
          });
          await route.abort();
        });
        await page.goto(
          `http://127.0.0.1:5243/browser-tests/proxmox-collection-safety.html?type=${type}`,
        );
        await page
          .getByRole('heading', { name: `Edit ${type.toUpperCase()} connection` })
          .waitFor()
          .catch(async (error) => {
            row.body = await page
              .locator('body')
              .innerText()
              .catch(() => 'unavailable');
            row.failureCapture = `${engine}-${type}-failure.png`;
            await page.screenshot({
              path: path.join(artifacts, row.failureCapture),
              fullPage: true,
            });
            throw error;
          });
        if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
        const ssl = page.getByRole('checkbox', { name: 'Verify SSL certificate', exact: true });
        assert.equal(await ssl.isChecked(), true);
        const precaution = page.getByRole('note', {
          name: 'Guest-agent backup precaution',
          exact: true,
        });
        const scope =
          type === 'pve'
            ? page.getByRole('region', { name: 'Collection scope', exact: true })
            : page.locator('div.space-y-3').filter({
                has: page.getByRole('heading', { name: 'Collection scope', exact: true }),
              });
        assert.equal(await scope.count(), 1);
        if (type === 'pve') {
          await precaution.waitFor();
          const text = await scope.innerText();
          for (const sentence of [
            'Guest details depend on guest-agent availability and permissions.',
            'Some readings can be unavailable or temporarily deferred.',
            'stop Pulse monitoring and leave it stopped',
            'independent checks confirm thaw and writes to every filesystem covered by the backup',
            'Do not run live diagnostics or restart the guest agent during the incident.',
            'An OK backup or a running VM does not prove recovery.',
          ])
            assert.ok(text.includes(sentence), sentence);
          assert.ok(!text.includes('full visibility without extra configuration'));
        } else {
          assert.equal(await precaution.count(), 0);
          assert.ok(!(await scope.innerText()).includes('guest-agent reads'));
        }
        const label =
          type === 'pve'
            ? /^Virtual machines/
            : type === 'pbs'
              ? /^Datastores/
              : /^Queue health insights/;
        const collector = page.getByRole('checkbox', { name: label });
        assert.equal(await collector.isChecked(), true);
        if (type === 'pve') {
          const description = await accessibleDescription(collector);
          row.vmDescription = description;
          assert.ok(description.startsWith('VM state and resource usage.'));
          assert.ok(
            description.includes(
              'independent checks confirm thaw and writes to every filesystem covered by the backup',
            ),
          );
          assert.ok(
            description.includes(
              'Do not run live diagnostics or restart the guest agent during the incident.',
            ),
          );
          assert.ok(description.includes('An OK backup or a running VM does not prove recovery.'));
          assert.equal(
            await page.getByRole('checkbox', { name: 'Virtual machines', exact: true }).count(),
            1,
          );
          if (engine === 'chromium') {
            row.vmAXChecked = await observedAX(page, collector);
            assert.equal(row.vmAXChecked.name, 'Virtual machines');
            assert.equal(row.vmAXChecked.description, description);
          }
          row.independentCollectors = [];
          for (const [name, help] of [
            ['Containers', 'LXC container state and resource usage.'],
            ['Storage', 'Per-storage capacity, status, and content type breakdown.'],
            ['Backups', 'vzdump schedules and recent backup activity.'],
          ]) {
            const control = page.getByRole('checkbox', { name, exact: true });
            assert.equal(await accessibleDescription(control), help);
            if (phone) {
              await page.getByText(name, { exact: true }).tap();
              assert.equal(await control.isChecked(), false);
              await page.getByText(name, { exact: true }).tap();
            } else {
              await control.focus();
              await page.keyboard.press('Space');
              assert.equal(await control.isChecked(), false);
              await page.keyboard.press('Space');
            }
            assert.equal(await control.isChecked(), true);
            const ax = engine === 'chromium' ? await observedAX(page, control) : undefined;
            if (ax) {
              assert.equal(ax.name, name);
              assert.equal(ax.description, help);
            }
            row.independentCollectors.push({ name, description: help, ax });
          }
          const warning =
            'This will spin up idle HDDs. Leave disabled if you rely on drive standby.';
          const smart = page.getByRole('button', {
            name: 'Enable physical disk monitoring',
            exact: true,
          });
          assert.equal(await smart.getAttribute('aria-pressed'), 'false');
          assert.equal(await accessibleDescription(smart), warning);
          if (engine === 'chromium') {
            row.smartAXDisabled = await observedAX(page, smart);
            assert.equal(row.smartAXDisabled.description, warning);
          }
          if (phone) await smart.tap();
          else {
            await smart.focus();
            await page.keyboard.press('Space');
          }
          const enabledSmart = page.getByRole('button', {
            name: 'Disable physical disk monitoring',
            exact: true,
          });
          assert.equal(await enabledSmart.getAttribute('aria-pressed'), 'true');
          assert.equal(await accessibleDescription(enabledSmart), warning);
          await page
            .getByRole('combobox', { name: 'Physical disk health polling interval', exact: true })
            .waitFor();
          if (engine === 'chromium') {
            row.smartAXEnabled = await observedAX(page, enabledSmart);
            assert.equal(row.smartAXEnabled.description, warning);
          }
          if (phone) await enabledSmart.tap();
          else await page.keyboard.press('Space');
          assert.equal(await smart.getAttribute('aria-pressed'), 'false');
          row.smartCapture = `${engine}-pve-smart.png`;
          await smart
            .locator('..')
            .locator('..')
            .screenshot({ path: path.join(artifacts, row.smartCapture) });
        }
        if (phone) await collector.tap();
        else {
          await collector.focus();
          await page.keyboard.press('Space');
        }
        assert.equal(await collector.isChecked(), false);
        if (type === 'pve') {
          assert.equal(await accessibleDescription(collector), row.vmDescription);
          if (engine === 'chromium') {
            row.vmAXUnchecked = await observedAX(page, collector);
            assert.equal(row.vmAXUnchecked.name, 'Virtual machines');
            assert.equal(row.vmAXUnchecked.description, row.vmDescription);
          }
        }
        const geometry = await scope.evaluate((node) => {
          const rect = node.getBoundingClientRect();
          return {
            x: rect.x,
            right: rect.right,
            width: rect.width,
            scrollWidth: document.documentElement.scrollWidth,
            viewportWidth: innerWidth,
          };
        });
        assert.ok(geometry.x >= 0 && geometry.right <= viewport.width + 1, 'scope fits viewport');
        assert.ok(
          geometry.scrollWidth <= geometry.viewportWidth + 1,
          'no horizontal page overflow',
        );
        const capture = path.join(artifacts, `${engine}-${type}-collection.png`);
        await scope.screenshot({ path: capture });
        const save = page.getByRole('button', { name: 'Save changes', exact: true });
        if (phone) await save.tap();
        else await save.click();
        await page.waitForFunction(
          () => document.querySelector('[data-testid="save-finished"]').textContent === 'true',
        );
        const saved = JSON.parse(await page.getByTestId('save-result').innerText());
        assert.equal(saved.targetId, `${type}-fixture`, 'save keeps the existing edit target');
        assert.equal(saved.data.type, type);
        assert.equal(saved.data.verifySSL, true);
        assert.ok(
          !('tokenValue' in saved.data) && !('password' in saved.data),
          'existing credential is not replaced',
        );
        if (type === 'pve') {
          assert.equal(saved.data.monitorVMs, false);
          for (const key of ['monitorContainers', 'monitorStorage', 'monitorBackups'])
            assert.equal(saved.data[key], true);
          assert.equal(saved.data.monitorPhysicalDisks, false);
          assert.equal(
            await precaution.isVisible(),
            true,
            'collector choice is not recovery evidence',
          );
        } else if (type === 'pbs') {
          assert.equal(saved.data.monitorDatastores, false);
          assert.equal(saved.data.monitorSyncJobs, true);
        } else {
          assert.equal(saved.data.monitorQueues, false);
          assert.equal(saved.data.monitorMailStats, true);
        }
        await page.waitForTimeout(200);
        assert.deepEqual(requests, [], 'no probe, token, collector, or other API request');
        assert.deepEqual(errors, [], 'no page exceptions');
        assert.deepEqual(row.consoleErrors, [], 'no console errors');
        const bytes = fs.readFileSync(capture);
        Object.assign(row, {
          phase: 'complete',
          engine,
          browser: browser.version(),
          viewport,
          type,
          theme: phone ? 'dark' : 'light',
          geometry,
          saved,
          requests,
          errors,
          capture: path.basename(capture),
          bytes: bytes.length,
          sha256: hash(bytes),
          passed: true,
        });
        await page.close();
      }
      await browser.close();
      browser = null;
    }
    result.passed = true;
  } catch (error) {
    result.passed = false;
    result.error = error.stack;
    throw error;
  } finally {
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
    console.log(
      JSON.stringify({
        result: '/workspace/tmp/collection-browser/result.json',
        passed: result.passed,
        cases: cases.length,
      }),
    );
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
