// Raw single-session diagnostic; optional synthetic production incident component fixture.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createRequire } from 'node:module';
import { chromium } from '@playwright/test';
const incidentUserRefresh = process.argv.includes('--incident-user-refresh');
const incidentConvergence = incidentUserRefresh || process.argv.includes('--incident-convergence');
const headedTab = incidentConvergence || process.argv.includes('--headed-tab');
const headedWindow = headedTab || process.argv.includes('--headed-window');
const checkForeground = headedWindow || process.argv.includes('--foreground');
if (headedWindow && !process.env.DISPLAY) throw new Error('--headed-window requires an owned X display');
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const profile = await mkdtemp(join(tmpdir(), 'pulse-lifecycle-'));
const child = spawn(chromium.executablePath(), [...(headedWindow ? [] : ['--headless']), '--no-sandbox',
  '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank']);
let socket;
let fixtureServer;
try {
  if (incidentConvergence) fixtureServer = await (await import('./incident-convergence-server.mjs')).startIncidentFixture();
  const endpoint = await new Promise((resolve, reject) => {
    let stderr = '';
    const timer = setTimeout(() => reject(new Error('Browser endpoint timeout')), 10000);
    child.once('error', reject);
    child.stderr.on('data', data => {
      stderr += data;
      const match = stderr.match(/DevTools listening on (ws:\/\/\S+)/);
      if (match) { clearTimeout(timer); resolve(match[1]); }
    });
  });
  socket = new WebSocket(endpoint);
  await new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true });
    socket.addEventListener('error', reject, { once: true });
  });
  let id = 0;
  const pending = new Map();
  socket.addEventListener('message', ({ data }) => {
    const message = JSON.parse(data);
    if (!pending.has(message.id)) return;
    const { resolve, reject, timer } = pending.get(message.id);
    pending.delete(message.id); clearTimeout(timer);
    if (message.error) reject(new Error(JSON.stringify(message.error)));
    else resolve(message.result);
  });
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    const requestId = ++id;
    const timer = setTimeout(() => { pending.delete(requestId); reject(new Error(`${method} timeout`)); }, 10000);
    pending.set(requestId, { resolve, reject, timer });
    socket.send(JSON.stringify({ id: requestId, method, params, sessionId }));
  });
  const require = createRequire(import.meta.url);
  console.log(JSON.stringify({ browser: await send('Browser.getVersion'),
    playwright: require('@playwright/test/package.json').version, node: process.version,
    platform: process.platform, arch: process.arch, executable: chromium.executablePath(),
    headedWindow, headedTab, incidentConvergence, incidentUserRefresh, display: process.env.DISPLAY, observationBoundMs: headedWindow ? 10000 : 250 }));
  const results = [];
  // Preselected negative (focus forced) and positive (no forced focus), fresh targets.
  for (const focusEnabled of (headedWindow ? [false] : [true, false])) {
    const { targetId } = await send('Target.createTarget', { url: 'about:blank' });
    try {
      const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
      const commands = [];
      const windowCommand = async (method, params) => {
        const response = await send(method, params);
        commands.push({ method, params, response });
        return response;
      };
      const windowInfo = headedWindow
        ? await windowCommand('Browser.getWindowForTarget', { targetId }) : null;
      const command = async (method, params) => {
        const result = await send(method, params, sessionId);
        commands.push({ method, params, response: result });
        return result;
      };
      const evaluate = async expression => {
        const result = await send('Runtime.evaluate', { expression, returnByValue: true }, sessionId);
        if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails));
        return result.result.value;
      };
      await command('Page.enable', {});
      await command('Emulation.setFocusEmulationEnabled', { enabled: focusEnabled });
      if (incidentConvergence) {
        await command('Page.navigate', { url: 'http://127.0.0.1:5198/qualification' });
        const deadline = Date.now() + 20000;
        while (!await evaluate(`typeof window.snapshot === 'function'`)) {
          if (Date.now() > deadline) throw new Error('Fixture load timeout');
          await wait(100);
        }
      }
      await evaluate(`window.probe = { ticks: 0, events: [] };
        setInterval(() => window.probe.ticks++, 50);
        for (const type of ['freeze', 'resume', 'visibilitychange'])
          document.addEventListener(type, () => window.probe.events.push({
            type, ticks: window.probe.ticks, visibility: document.visibilityState }));`);
      const snapshot = `({...window.probe, visibility: document.visibilityState, focus: document.hasFocus()})`;
      await wait(250);
      const before = await evaluate(snapshot);
      let otherTarget;
      const observe = async (stage, visibility) => {
        const deadline = Date.now() + 10000;
        let sample;
        do {
          await wait(250);
          sample = await evaluate(snapshot);
          console.log(JSON.stringify({ stage, snapshot: sample }));
          if (sample.visibility === visibility && (visibility !== 'visible' || sample.focus)) return sample;
        } while (Date.now() < deadline);
        throw new Error(stage + ': native visibility transition not observed');
      };
      const awaitIncident = async id => {
        const deadline = Date.now() + 10000;
        while (!await evaluate(`window.snapshot().incidents.host?.[0]?.id === ${JSON.stringify(id)} && window.snapshot().loading.host === false`)) {
          if (Date.now() > deadline) throw new Error('Incident settlement timeout: ' + id);
          await wait(100);
        }
      };
      if (incidentUserRefresh) {
        await evaluate(`Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Open row').click()`);
        const deadline = Date.now() + 10000;
        while (fixtureServer.count() !== 1) {
          if (Date.now() > deadline) throw new Error('Initial incident request timeout');
          await wait(100);
        }
        fixtureServer.finish(0, 'Cached incident');
        await awaitIncident('Cached incident');
      }
      if (headedTab) {
        assert.ok(before.visibility === 'visible' && before.focus, 'Tab baseline must be active');
        otherTarget = (await windowCommand('Target.createTarget', { url: 'about:blank' })).targetId;
        await windowCommand('Target.activateTarget', { targetId: otherTarget });
        await observe('tab-background-preflight', 'hidden');
        await windowCommand('Target.activateTarget', { targetId });
        await observe('tab-foreground-preflight', 'visible');
        await windowCommand('Target.activateTarget', { targetId: otherTarget });
        await observe('tab-background-before-freeze', 'hidden');
      } else if (headedWindow) {
        assert.ok(before.visibility === 'visible' && before.focus, 'Headed baseline must be visible and focused');
        await windowCommand('Browser.setWindowBounds', { windowId: windowInfo.windowId, bounds: { windowState: 'minimized' } });
        await wait(250);
        console.log(JSON.stringify({ stage: 'minimized', snapshot: await evaluate(snapshot) }));
      }
      if (incidentConvergence && !incidentUserRefresh) {
        for (const [index, label] of ['Open row', 'Overlap refresh'].entries()) {
          await evaluate(`Array.from(document.querySelectorAll('button')).find(b => b.textContent === ${JSON.stringify(label)}).click()`);
          const deadline = Date.now() + 10000;
          while (fixtureServer.count() !== index + 1) {
            if (Date.now() > deadline) throw new Error('Incident request timeout');
            await wait(100);
          }
        }
        console.log(JSON.stringify({ stage: 'incident-before-freeze', state: await evaluate('window.snapshot()') }));
      }
      await command('Page.setWebLifecycleState', { state: 'frozen' });
      if (incidentConvergence && !incidentUserRefresh) {
        fixtureServer.finish(1, 'Latest incident');
        await wait(100);
        fixtureServer.finish(0, 'Obsolete incident');
      }
      await wait(2100); // Host wait: never evaluate while frozen.
      await command('Page.setWebLifecycleState', { state: 'active' });
      await wait(250);
      const after = await evaluate(snapshot);
      const freeze = after.events.find(e => e.type === 'freeze');
      const resume = after.events.find(e => e.type === 'resume');
      const suspended = Boolean(freeze && resume && freeze.ticks === resume.ticks &&
        after.events.indexOf(freeze) < after.events.indexOf(resume) && (headedWindow || after.ticks > resume.ticks));
      // Resume is not foreground activation. Observe the two transitions separately.
      let foreground;
      if (checkForeground) {
        if (headedTab) {
          await windowCommand('Target.activateTarget', { targetId });
        } else if (headedWindow) {
          await windowCommand('Browser.setWindowBounds', { windowId: windowInfo.windowId, bounds: { windowState: 'normal' } });
        }
        await command('Page.bringToFront', {});
        const deadline = Date.now() + (headedWindow ? 10000 : 250);
        do {
          await wait(250);
          foreground = await evaluate(snapshot);
          if (headedWindow) console.log(JSON.stringify({ stage: 'foreground-observation', snapshot: foreground }));
          if (foreground.visibility === 'visible' && foreground.focus && foreground.ticks > after.ticks) break;
        } while (Date.now() < deadline);
      }
      if (incidentConvergence) {
        assert.ok(suspended, 'Fixture must demonstrably suspend and resume');
        assert.ok(foreground.visibility === 'visible' && foreground.focus && foreground.ticks > after.ticks,
          'Fixture must return to native foreground with timer progress');
        const resumeIndex = foreground.events.findIndex(e => e.type === 'resume');
        assert.ok(foreground.events.some((e, i) => i > resumeIndex && e.type === 'visibilitychange' && e.visibility === 'visible'));
        if (incidentUserRefresh) {
          assert.equal(fixtureServer.count(), 1, 'Resume alone must not be mistaken for an incident fetch');
          await awaitIncident('Cached incident');
          const clicked = await evaluate(`(() => {
            const button = Array.from(document.querySelectorAll('button')).find(b => b.textContent.trim() === 'Refresh');
            if (!button || button.disabled) return false;
            button.click(); return true;
          })()`);
          assert.ok(clicked, 'Use the enabled production Refresh control');
          const deadline = Date.now() + 10000;
          while (fixtureServer.count() !== 2) {
            if (Date.now() > deadline) throw new Error('User refresh request timeout');
            await wait(100);
          }
          assert.equal(await evaluate('window.snapshot().loading.host'), true);
          fixtureServer.finish(1, 'Latest incident');
        }
        const deadline = Date.now() + 10000;
        let state;
        do {
          state = await evaluate('window.snapshot()');
          if (state.incidents.host?.[0]?.id === 'Latest incident' && state.loading.host === false) break;
          await wait(250);
        } while (Date.now() < deadline);
        const rendered = await evaluate(`({text: document.querySelector('section').innerText, errors: document.querySelector('[data-testid="errors"]').textContent})`);
        console.log(JSON.stringify({ stage: incidentUserRefresh ? 'incident-user-refresh' : 'incident-convergence', state, rendered }));
        assert.equal(state.incidents.host?.[0]?.id, 'Latest incident');
        assert.equal(state.loading.host, false);
        assert.equal(state.error.host, false);
        assert.ok(rendered.text.includes('Latest incident'));
        assert.ok(!rendered.text.includes('Obsolete incident'));
        if (incidentUserRefresh) assert.ok(!rendered.text.includes('Cached incident'));
        assert.equal(rendered.errors, '0');
      }
      const result = { focusEnabled, before, after, foreground, commands, suspended };
      results.push(result); console.log(JSON.stringify(result));
      if (otherTarget) await send('Target.closeTarget', { targetId: otherTarget });
    } finally { await send('Target.closeTarget', { targetId }); }
  }
  if (!headedWindow) assert.ok(!results[0].suspended && results[0].after.ticks - results[0].before.ticks >= 20,
    'Negative control must continue ticking');
  const positive = results.at(-1);
  assert.ok(positive.suspended, 'Positive control must suspend and resume timers');
  if (headedWindow) {
    const events = positive.foreground.events;
    const resumeIndex = events.findIndex(e => e.type === 'resume');
    assert.ok(events.some((e, i) => i > resumeIndex && e.type === 'visibilitychange' && e.visibility === 'visible'),
      'Must observe visible transition after resume');
  }
  if (checkForeground) {
    assert.ok(positive.foreground.visibility === 'visible' && positive.foreground.focus,
      'Positive control must return to visible and focused after tab activation');
    assert.ok(positive.foreground.ticks > positive.after.ticks,
      'Foreground timer must continue advancing');
  }
} finally {
  await fixtureServer?.close();
  socket?.close();
  const exited = new Promise(resolve => child.once('exit', resolve));
  if (child.exitCode === null) { child.kill(); await exited; }
  await rm(profile, { recursive: true });
}
