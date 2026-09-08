// Owned blank-page diagnostic; no Playwright page session or application loaded.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createRequire } from 'node:module';
import { chromium } from '@playwright/test';
const headedWindow = process.argv.includes('--headed-window');
const checkForeground = headedWindow || process.argv.includes('--foreground');
if (headedWindow && !process.env.DISPLAY) throw new Error('--headed-window requires an owned X display');
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const profile = await mkdtemp(join(tmpdir(), 'pulse-lifecycle-'));
const child = spawn(chromium.executablePath(), [...(headedWindow ? [] : ['--headless']), '--no-sandbox',
  '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank']);
let socket;
try {
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
    headedWindow, display: process.env.DISPLAY, observationBoundMs: headedWindow ? 10000 : 250 }));
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
      await evaluate(`window.probe = { ticks: 0, events: [] };
        setInterval(() => window.probe.ticks++, 50);
        for (const type of ['freeze', 'resume', 'visibilitychange'])
          document.addEventListener(type, () => window.probe.events.push({
            type, ticks: window.probe.ticks, visibility: document.visibilityState }));`);
      const snapshot = `({...window.probe, visibility: document.visibilityState, focus: document.hasFocus()})`;
      await wait(250);
      const before = await evaluate(snapshot);
      if (headedWindow) {
        assert.ok(before.visibility === 'visible' && before.focus, 'Headed baseline must be visible and focused');
        await windowCommand('Browser.setWindowBounds', { windowId: windowInfo.windowId, bounds: { windowState: 'minimized' } });
        await wait(250);
        console.log(JSON.stringify({ stage: 'minimized', snapshot: await evaluate(snapshot) }));
      }
      await command('Page.setWebLifecycleState', { state: 'frozen' });
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
        if (headedWindow) {
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
      const result = { focusEnabled, before, after, foreground, commands, suspended };
      results.push(result); console.log(JSON.stringify(result));
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
  socket?.close();
  const exited = new Promise(resolve => child.once('exit', resolve));
  if (child.exitCode === null) { child.kill(); await exited; }
  await rm(profile, { recursive: true });
}
