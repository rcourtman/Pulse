import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import test from 'node:test';
import { observeBootstrapTiming } from './bootstrap-timing.mjs';

const request = (path) => ({ url: () => `http://localhost${path}`, method: () => 'GET' });
test('pairs each allowed browser path once, with bounded cookie-context HTTP and no payload', async () => {
  const page = new EventEmitter();
  const calls = [];
  let disposed = 0;
  page.request = { get: async (...args) => {
    calls.push(args);
    return { status: () => 200, dispose: async () => { disposed++; } };
  } };
  let time = 100;
  const finish = observeBootstrapTiming(page, () => time++);
  page.emit('request', request('/api/state?secret=private'));
  const summary = request('/api/state/summary?secret=private');
  page.emit('request', summary);
  page.emit('request', summary);
  page.emit('response', { request: () => summary, status: () => 200 });
  page.emit('request', request('/api/license/runtime-capabilities'));
  const rows = await finish();
  assert.equal(calls.length, 2);
  assert.deepEqual(calls[0], ['/api/state/summary', { timeout: 5000, maxRedirects: 0 }]);
  assert.equal(disposed, 2);
  assert.equal(rows.length, 4);
  assert.equal(rows[0].status, 200);
  assert.equal(rows[2].status, null); // censored browser request
  assert.equal(rows[2].duration, null);
  assert.doesNotMatch(JSON.stringify(rows), /secret|private/);
  assert.equal(page.listenerCount('request'), 0);
  assert.equal(page.listenerCount('response'), 0);
});
test('transport failure is censored and never retains exception text', async () => {
  const page = new EventEmitter();
  page.request = { get: async () => { throw new Error('credential=private'); } };
  const finish = observeBootstrapTiming(page);
  page.emit('request', request('/api/state/summary'));
  const rows = await finish();
  assert.equal(rows[1].status, null);
  assert.equal(typeof rows[1].duration, 'number');
  assert.doesNotMatch(JSON.stringify(rows), /credential|private/);
});
