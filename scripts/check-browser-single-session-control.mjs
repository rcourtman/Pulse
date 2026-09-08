// Owned blank-page diagnostic; no Playwright page session or application loaded.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createRequire } from 'node:module';
import { chromium } from '@playwright/test';
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const profile = await mkdtemp(join(tmpdir(), 'pulse-lifecycle-'));
const child = spawn(chromium.executablePath(), ['--headless', '--no-sandbox',
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
    platform: process.platform, arch: process.arch, executable: chromium.executablePath() }));
  const results = [];
  // Preselected negative (focus forced) and positive (no forced focus), fresh targets.
  for (const focusEnabled of [true, false]) {
    const { targetId } = await send('Target.createTarget', { url: 'about:blank' });
    try {
      const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
      const commands = [];
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
      await command('Page.setWebLifecycleState', { state: 'frozen' });
      await wait(2100); // Host wait: never evaluate while frozen.
      await command('Page.setWebLifecycleState', { state: 'active' });
      await wait(250);
      const after = await evaluate(snapshot);
      const freeze = after.events.find(e => e.type === 'freeze');
      const resume = after.events.find(e => e.type === 'resume');
      const suspended = Boolean(freeze && resume && freeze.ticks === resume.ticks &&
        after.events.indexOf(freeze) < after.events.indexOf(resume) && after.ticks > resume.ticks);
      const result = { focusEnabled, before, after, commands, suspended };
      results.push(result); console.log(JSON.stringify(result));
    } finally { await send('Target.closeTarget', { targetId }); }
  }
  assert.ok(!results[0].suspended && results[0].after.ticks - results[0].before.ticks >= 20,
    'Negative control must continue ticking');
  assert.ok(results[1].suspended, 'Positive control must suspend and resume timers');
} finally {
  socket?.close();
  const exited = new Promise(resolve => child.once('exit', resolve));
  if (child.exitCode === null) { child.kill(); await exited; }
  await rm(profile, { recursive: true });
}
