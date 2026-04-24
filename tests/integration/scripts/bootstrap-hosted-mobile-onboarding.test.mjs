import assert from 'node:assert/strict';
import test from 'node:test';

import {
  deriveTenantBaseUrl,
  fetchOnboardingPayload,
  fetchOnboardingPayloadViaCloudHost,
  redactBearerTokens,
} from './bootstrap-hosted-mobile-onboarding.mjs';

test('deriveTenantBaseUrl maps the control-plane host to the tenant subdomain', () => {
  assert.equal(
    deriveTenantBaseUrl('https://cloud.pulserelay.pro/', 't-CANARY123'),
    'https://t-canary123.cloud.pulserelay.pro',
  );
});

test('fetchOnboardingPayload uses bearer auth without leaking it in failures', async () => {
  const token = 'pulse_secret_token.value/with+chars';
  const calls = [];
  const fetchImpl = async (url, options) => {
    calls.push({ options, url });
    return {
      ok: false,
      status: 401,
      text: async () => `bad Authorization: Bearer ${token}`,
    };
  };

  await assert.rejects(
    () => fetchOnboardingPayload({
      fetchImpl,
      rawToken: token,
      tenantBaseUrl: 'https://t-canary.cloud.pulserelay.pro',
    }),
    (error) => {
      assert.match(error.message, /Bearer \[REDACTED\]/);
      assert.doesNotMatch(error.message, new RegExp(token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
      return true;
    },
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, 'https://t-canary.cloud.pulserelay.pro/api/onboarding/qr');
  assert.equal(calls[0].options.headers.Authorization, `Bearer ${token}`);
});

test('redactBearerTokens redacts bearer values in arbitrary command text', () => {
  assert.equal(
    redactBearerTokens('Command failed: curl -H Authorization: Bearer abc.def/ghi'),
    'Command failed: curl -H Authorization: Bearer [REDACTED]',
  );
});

test('fetchOnboardingPayloadViaCloudHost passes the bearer token over stdin only', () => {
  const token = 'pulse_secret_token.value/with+chars';
  const calls = [];
  const payload = fetchOnboardingPayloadViaCloudHost({
    cloudHost: 'root@pulse-cloud',
    rawToken: token,
    tenantBaseUrl: 'https://t-canary.cloud.pulserelay.pro',
    runner: (command, args, options) => {
      calls.push({ args, command, options });
      return JSON.stringify({
        deep_link: 'pulse://pair',
        instance_id: 'relay_123',
        relay: { enabled: true, url: 'wss://relay.example/ws' },
      });
    },
  });

  assert.equal(payload.instance_id, 'relay_123');
  assert.equal(calls.length, 1);
  assert.equal(calls[0].command, 'ssh');
  assert.equal(calls[0].options.input, token);
  assert.doesNotMatch(calls[0].args.join(' '), new RegExp(token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
});
