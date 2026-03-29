import assert from 'node:assert/strict';
import test from 'node:test';

import { resolveHealthCheckStrategy } from './pretest.mjs';

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
