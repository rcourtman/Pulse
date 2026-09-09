import assert from 'node:assert/strict';
import { createPublicKey, verify } from 'node:crypto';
import test from 'node:test';
import { startOfflineLicenseIssuer } from './offline-license-issuer.mjs';

const post = (issuer, route, data, token = '') => fetch(issuer.url + route, {
  method: 'POST', headers: { Authorization: `Bearer ${token}` }, body: JSON.stringify(data),
});

test('offline issuer signs scoped grants and binds status/refresh to each installation', async t => {
  const issuer = await startOfflineLicenseIssuer();
  t.after(() => issuer.close());
  const activate = fingerprint => post(issuer, '/v1/activate', {
    activation_key: issuer.activationKey, instance_fingerprint: fingerprint, runtime: { build: 'community' },
  });
  const responses = await Promise.all(['default', 'org-a', 'org-b'].map(activate));
  assert.ok(responses.every(r => r.status === 201));
  const states = await Promise.all(responses.map(r => r.json()));
  assert.equal(new Set(states.map(s => s.installation.installation_id)).size, 3);
  assert.equal(new Set(states.map(s => s.installation.installation_token)).size, 3);
  const pub = createPublicKey({ format: 'jwk', key: { kty: 'OKP', crv: 'Ed25519',
    x: Buffer.from(issuer.publicKey, 'base64').toString('base64url') } });
  for (const [i, state] of states.entries()) {
    const [head, body, signature] = state.grant.jwt.split('.');
    assert.ok(verify(null, Buffer.from(`${head}.${body}`), pub, Buffer.from(signature, 'base64url')));
    const claims = JSON.parse(Buffer.from(body, 'base64url'));
    assert.equal(claims.iid, state.installation.installation_id);
    assert.equal(claims.lid, state.license.license_id);
    assert.equal(claims.st, 'active'); assert.equal(claims.tier, 'msp');
    assert.equal(claims.feat, undefined); // Runtime derives features; no capability injection.
    const data = { installation_id: claims.iid, instance_fingerprint: ['default', 'org-a', 'org-b'][i] };
    for (const route of ['/v1/grants/status', '/v1/grants/refresh']) {
      assert.equal((await post(issuer, route, data, state.installation.installation_token)).status, 200);
      assert.equal((await post(issuer, route, data, 'wrong')).status, 401);
      assert.equal((await post(issuer, route, { ...data, instance_fingerprint: 'wrong' }, state.installation.installation_token)).status, 401);
      assert.equal((await post(issuer, route, data, states[(i + 1) % 3].installation.installation_token)).status, 401);
    }
  }
  const repeated = await (await activate('org-a')).json();
  assert.deepEqual(repeated.installation, states[1].installation);
});

test('offline issuer rejects unsupported routes, bad activation and malformed input', async t => {
  const issuer = await startOfflineLicenseIssuer(); t.after(() => issuer.close());
  const data = { activation_key: issuer.activationKey, instance_fingerprint: 'default', runtime: { build: 'community' } };
  for (const mutation of [{ activation_key: 'wrong' }, { runtime: { build: 'enterprise' } }, { instance_fingerprint: '' }]) {
    assert.ok((await post(issuer, '/v1/activate', { ...data, ...mutation })).status >= 400);
  }
  assert.equal((await post(issuer, '/v1/licenses/exchange', data)).status, 404);
  assert.equal((await fetch(issuer.url + '/v1/activate')).status, 404);
  assert.equal((await fetch(issuer.url + '/v1/activate', { method: 'POST', body: '{' })).status, 400);
  assert.equal((await post(issuer, '/v1/activate', { ...data, extra: 'x'.repeat(17000) })).status, 413);
  const other = await startOfflineLicenseIssuer(); t.after(() => other.close());
  assert.notEqual(other.publicKey, issuer.publicKey);
  assert.notEqual(other.activationKey, issuer.activationKey);
});
