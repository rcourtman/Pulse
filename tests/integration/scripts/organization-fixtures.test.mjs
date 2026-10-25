import assert from 'node:assert/strict';
import fs from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { runInNewContext } from 'node:vm';

const integration = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = fs.readFileSync(process.env.PULSE_ORG_FIXTURE_SOURCE || path.join(integration, 'tests/helpers.ts'), 'utf8');
const start = source.indexOf('export async function activateOfflineOrganization(');
const end = source.indexOf('export async function switchOrg(', start);
assert.ok(start >= 0 && end > start, 'missing public organization fixture helpers');
const helpers = stripTypeScriptTypes(`${source.slice(start, end).replace(/^export /gm, '')}\n({ activateOfflineOrganization, createOrg, deleteOrg, deleteOrgs });`);
const response = (status, data = {}) => ({
  ok: () => status >= 200 && status < 300,
  status: () => status,
  json: async () => data,
  text: async () => JSON.stringify(data),
});

function fixture(request) {
  const calls = [];
  const api = runInNewContext(helpers, {
    process: { env: { PULSE_E2E_OFFLINE_ACTIVATION_KEY: 'ephemeral-fixture-key' } },
    toOrgID: () => 'fixture-org',
    expect: value => ({
      toBe: expected => assert.equal(value, expected),
      toContain: expected => assert.ok(value.includes(expected)),
      not: { toContain: expected => assert.ok(!value.includes(expected)) },
    }),
    apiRequest: async (page, endpoint, options = {}) => {
      calls.push({ endpoint, options });
      if (endpoint === '/api/license/runtime-capabilities') return response(200, { capabilities: ['multi_tenant'] });
      if (endpoint === '/api/security/status') return response(200, { sessionCapabilities: { demoMode: false } });
      return request(endpoint, options);
    },
  });
  return { ...api, calls };
}

// A logical clock keeps this a fast boundary test. These operations are valid
// at the server's existing 15s activation and 30s graceful-shutdown deadlines.
function completeWithinRequestBudget(options, elapsed, status, payload) {
  const timeout = options.timeout ?? 10_000;
  assert.ok(timeout > 0 && timeout <= 35_000, 'operation must retain a bounded request timeout');
  if (elapsed >= timeout) throw new Error('request timed out before the server contract completed');
  return response(status, payload);
}

test('activation receives the server verdict beyond the browser action timeout', async () => {
  const f = fixture((endpoint, options) => {
    assert.equal(endpoint, '/api/license/activate');
    return completeWithinRequestBudget(options, 15_000, 200);
  });
  await f.activateOfflineOrganization({}, 'owned-org');
  assert.equal(f.calls[0].options.headers['X-Pulse-Org-ID'], 'owned-org');
  assert.equal(f.calls.filter(call => call.endpoint === '/api/license/activate').length, 1);
});

test('activation rejection remains a failure without another attempt', async () => {
  const f = fixture(() => response(403, { error: 'rejected' }));
  await assert.rejects(f.activateOfflineOrganization({}), /403/);
  assert.equal(f.calls.length, 1);
});

test('deletion waits for graceful shutdown and uses the default admin scope', async () => {
  const f = fixture((endpoint, options) => {
    assert.equal(endpoint, '/api/orgs/owned%2Forg');
    assert.equal(options.method, 'DELETE');
    assert.equal(options.headers['X-Org-ID'], 'default');
    assert.equal(options.headers['X-Pulse-Org-ID'], 'default');
    return completeWithinRequestBudget(options, 30_000, 204);
  });
  await f.deleteOrg({}, 'owned/org');
  assert.equal(f.calls.length, 1);
});

test('deletion still rejects incomplete shutdown, unexpected success and an exhausted request budget', async () => {
  for (const status of [200, 403, 503]) {
    const f = fixture(() => response(status));
    await assert.rejects(f.deleteOrg({}, 'owned'), new RegExp(String(status)));
    assert.equal(f.calls.length, 1);
  }
  const missing = fixture(() => response(404));
  await missing.deleteOrg({}, 'already-removed');
  const expired = fixture((endpoint, options) => completeWithinRequestBudget(options, 35_001, 204));
  await assert.rejects(expired.deleteOrg({}, 'stuck'), /request timed out/);
});

test('creation owns the returned ID before activation can fail', async () => {
  const activationError = new Error('issuer unavailable');
  const f = fixture((endpoint, options) => {
    if (endpoint === '/api/orgs') return response(201, { id: 'created-org' });
    if (endpoint === '/api/license/activate') throw activationError;
    assert.equal(endpoint, '/api/orgs/created-org');
    assert.equal(options.method, 'DELETE');
    return response(204);
  });
  await assert.rejects(f.createOrg({}, 'Fixture'), error => error === activationError);
  assert.deepEqual(f.calls.map(call => call.endpoint), ['/api/orgs', '/api/license/activate', '/api/orgs/created-org']);
});

test('creation preserves activation and cleanup failures together', async () => {
  const activationError = new Error('issuer unavailable');
  const cleanupError = new Error('shutdown incomplete');
  const f = fixture(endpoint => {
    if (endpoint === '/api/orgs') return response(201, { id: 'created-org' });
    if (endpoint === '/api/license/activate') throw activationError;
    throw cleanupError;
  });
  await assert.rejects(f.createOrg({}, 'Fixture'), error => {
    assert.equal(error.name, 'AggregateError');
    assert.deepEqual([...error.errors], [activationError, cleanupError]);
    return true;
  });
});

test('successful creation returns ownership without deleting the fixture', async () => {
  const f = fixture(endpoint => endpoint === '/api/orgs' ? response(201, { id: 'created-org' }) : response(200));
  assert.equal((await f.createOrg({}, 'Fixture')).id, 'created-org');
  assert.ok(!f.calls.some(call => call.options.method === 'DELETE'));
});

test('cleanup attempts every owned ID in reverse order and reports every failure', async () => {
  const f = fixture(endpoint => endpoint.endsWith('/middle') ? response(503) : response(204));
  await assert.rejects(f.deleteOrgs({}, ['first', 'middle', 'last']), error => {
    assert.equal(error.name, 'AggregateError');
    assert.equal(error.errors.length, 1);
    assert.match(error.errors[0].message, /middle.*503/);
    return true;
  });
  assert.deepEqual(f.calls.map(call => call.endpoint), ['/api/orgs/last', '/api/orgs/middle', '/api/orgs/first']);
});
