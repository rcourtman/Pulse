import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import {
  resolveHealthCheckStrategy,
  validateBootstrapToken,
  validateBootstrapTokenForFirstRun,
} from './pretest.mjs';

test('resolveHealthCheckStrategy uses plain HTTP mode for HTTP health URLs', async () => {
  const strategy = await resolveHealthCheckStrategy('http://localhost:7655/api/health');

  assert.equal(strategy.mode, 'http');
  assert.equal(strategy.parsedURL.protocol, 'http:');
});

test('resolveHealthCheckStrategy keeps HTTPS verification enabled by default', async () => {
  const strategy = await resolveHealthCheckStrategy('https://pulse.example.com/api/health');

  assert.equal(strategy.mode, 'https');
  assert.equal(strategy.parsedURL.protocol, 'https:');
});

test('resolveHealthCheckStrategy rejects unsupported protocols', async () => {
  await assert.rejects(
    () => resolveHealthCheckStrategy('ftp://pulse.example.com/api/health'),
    /Unsupported health-check protocol/,
  );
});

test('resolveHealthCheckStrategy requires curl for insecure HTTPS opt-in', async () => {
  await assert.rejects(
    () => resolveHealthCheckStrategy(
      'https://pulse.example.com/api/health',
      {
        useInsecureTLS: true,
        hasCurlFn: async () => false,
      },
    ),
    /requires curl/,
  );
});

test('resolveHealthCheckStrategy scopes insecure HTTPS mode to curl when available', async () => {
  const strategy = await resolveHealthCheckStrategy(
    'https://pulse.example.com/api/health',
    {
      useInsecureTLS: true,
      hasCurlFn: async () => true,
    },
  );

  assert.equal(strategy.mode, 'curl');
  assert.equal(strategy.parsedURL.protocol, 'https:');
});

test('validateBootstrapToken posts the deterministic token to the public validation route', async () => {
  const calls = [];
  await validateBootstrapToken('http://localhost:7655/', '  token-123  ', {
    fetchImpl: async (url, options) => {
      calls.push({ url, options });
      return {
        ok: true,
        status: 204,
        text: async () => '',
      };
    },
  });

  assert.deepEqual(calls, [
    {
      url: 'http://localhost:7655/api/security/validate-bootstrap-token',
      options: {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ token: 'token-123' }),
      },
    },
  ]);
});

test('validateBootstrapToken fails with response detail when validation is rejected', async () => {
  await assert.rejects(
    () => validateBootstrapToken('http://localhost:7655', 'bad-token', {
      fetchImpl: async () => ({
        ok: false,
        status: 409,
        text: async () => 'Bootstrap token unavailable',
      }),
    }),
    /HTTP 409 Bootstrap token unavailable/,
  );
});

test('validateBootstrapTokenForFirstRun skips validation once auth is configured', async () => {
  const calls = [];
  await validateBootstrapTokenForFirstRun('http://localhost:7655', 'token-123', {
    fetchImpl: async (url, options) => {
      calls.push({ url, options });
      return {
        ok: true,
        status: 200,
        json: async () => ({ hasAuthentication: true }),
        text: async () => '',
      };
    },
  });

  assert.deepEqual(calls, [
    {
      url: 'http://localhost:7655/api/security/status',
      options: undefined,
    },
  ]);
});

test('validateBootstrapTokenForFirstRun validates the seed token before first-run setup', async () => {
  const calls = [];
  await validateBootstrapTokenForFirstRun('http://localhost:7655', 'token-123', {
    fetchImpl: async (url, options) => {
      calls.push({ url, options });
      if (url.endsWith('/api/security/status')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ hasAuthentication: false }),
          text: async () => '',
        };
      }
      return {
        ok: true,
        status: 204,
        text: async () => '',
      };
    },
  });

  assert.equal(calls.length, 2);
  assert.equal(calls[0].url, 'http://localhost:7655/api/security/status');
  assert.equal(calls[1].url, 'http://localhost:7655/api/security/validate-bootstrap-token');
  assert.equal(calls[1].options.body, JSON.stringify({ token: 'token-123' }));
});

test('docker compose bootstrap seed expands the token inside the seed container shell', () => {
  const compose = readFileSync(new URL('../docker-compose.test.yml', import.meta.url), 'utf8');

  assert.match(compose, /command:\n\s+- sh\n\s+- -c\n\s+- \|/);
  assert.ok(
    compose.includes(': "$${PULSE_E2E_BOOTSTRAP_TOKEN:?PULSE_E2E_BOOTSTRAP_TOKEN is required}"'),
  );
  assert.ok(compose.includes('token="$${PULSE_E2E_BOOTSTRAP_TOKEN}"'));
  assert.ok(compose.includes('if [ "$${#token}" -ne 48 ]; then'));
  assert.ok(
    compose.includes('printf \'%s\\n\' "$${token}" > /data/.bootstrap_token'),
  );
  assert.doesNotMatch(compose, /"\$\$PULSE_E2E_BOOTSTRAP_TOKEN"/);
});
