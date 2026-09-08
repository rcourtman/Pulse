import { test as base, expect } from '@playwright/test';
import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import WebSocket, { WebSocketServer } from 'ws';

// Playwright 1.56 enables focus emulation on its own CDP session. Disabling
// it on a second session does not clear that first session's override. This
// opt-in transport changes only that emulation command, on the owning session;
// visibility observations, input, HTTP routing and assertions remain untouched.
const nativeVisibilityTest = base.extend({
  browser: async ({ playwright, browserName, headless }, use) => {
    if (browserName !== 'chromium' || headless || !process.env.DISPLAY) {
      throw new Error('Native visibility requires headed Chromium on an owned display');
    }
    const profile = await mkdtemp(join(tmpdir(), 'pulse-native-visibility-'));
    const child = spawn(playwright.chromium.executablePath(), [
      '--no-sandbox', '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank',
    ], { stdio: ['ignore', 'ignore', 'pipe'] });
    const proxy = new WebSocketServer({ host: '127.0.0.1', port: 0 });
    const sockets = new Set<WebSocket>();
    let overrides = 0;
    try {
      const endpoint = await new Promise<string>((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('Chromium endpoint timeout')), 15000);
        let stderr = '';
        child.once('error', error => { clearTimeout(timer); reject(error); });
        child.once('exit', () => { clearTimeout(timer); reject(new Error('Chromium exited before connection')); });
        child.stderr!.on('data', data => {
          stderr += data;
          const match = stderr.match(/DevTools listening on (ws:\/\/\S+)/);
          if (match) { clearTimeout(timer); resolve(match[1]); }
        });
      });
      proxy.on('connection', downstream => {
        const upstream = new WebSocket(endpoint);
        sockets.add(downstream); sockets.add(upstream);
        const queued: string[] = [];
        downstream.on('message', data => {
          const message = JSON.parse(data.toString());
          if (message.method === 'Emulation.setFocusEmulationEnabled' && message.params?.enabled === true) {
            message.params.enabled = false;
            overrides++;
          }
          const wire = JSON.stringify(message);
          if (upstream.readyState === WebSocket.OPEN) upstream.send(wire);
          else queued.push(wire);
        });
        upstream.on('open', () => { for (const wire of queued) upstream.send(wire); queued.length = 0; });
        upstream.on('message', data => {
          if (downstream.readyState === WebSocket.OPEN) downstream.send(data.toString());
        });
        upstream.on('error', () => downstream.close());
        downstream.on('error', () => upstream.close());
        downstream.on('close', () => upstream.close());
        upstream.on('close', () => downstream.close());
      });
      if (!proxy.address()) await new Promise<void>(resolve => proxy.once('listening', resolve));
      const address = proxy.address();
      if (!address || typeof address === 'string') throw new Error('Missing proxy address');
      const browser = await playwright.chromium.connectOverCDP(`ws://127.0.0.1:${address.port}`);
      try {
        console.log(JSON.stringify({ nativeVisibilityTransport: 'owning-session focus emulation disabled', browser: browser.version() }));
        await use(browser);
        expect(overrides, 'Expected Playwright owning-session focus command').toBeGreaterThan(0);
      } finally { await browser.close(); }
    } finally {
      for (const socket of sockets) socket.terminate();
      await new Promise<void>(resolve => proxy.close(() => resolve()));
      if (child.exitCode === null && child.signalCode === null) {
        const stopped = new Promise<void>(resolve => child.once('exit', () => resolve()));
        child.kill('SIGTERM');
        const timer = setTimeout(() => child.kill('SIGKILL'), 5000);
        try { await stopped; } finally { clearTimeout(timer); }
      }
      await rm(profile, { recursive: true, force: true });
    }
  },
});

// Keep the ordinary journey on Playwright's unmodified default fixtures.
export const test = process.env.PULSE_E2E_INCIDENT_FOREGROUND === '1'
  ? nativeVisibilityTest : base;
