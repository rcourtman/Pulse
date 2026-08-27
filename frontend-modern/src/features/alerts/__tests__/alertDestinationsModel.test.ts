import { describe, expect, it } from 'vitest';

import {
  buildAppriseConfigPayload,
  buildEmailConfigPayload,
  normalizeAppriseConfig,
} from '../alertDestinationsModel';

describe('alertDestinationsModel', () => {
  it('normalizes apprise config into the UI model', () => {
    expect(
      normalizeAppriseConfig({
        enabled: true,
        mode: 'http',
        targets: ['https://notify.example.test'],
        serverUrl: 'https://apprise.example.test',
        timeoutSeconds: 30,
        minimumSeverity: 'critical',
      }),
    ).toEqual(
      expect.objectContaining({
        enabled: true,
        mode: 'http',
        targetsText: 'https://notify.example.test',
        serverUrl: 'https://apprise.example.test',
        timeoutSeconds: 30,
        minimumSeverity: 'critical',
      }),
    );
  });

  it('trims and drops empty recipient lines when building the email payload', () => {
    expect(
      buildEmailConfigPayload({
        enabled: true,
        provider: 'smtp',
        server: 'smtp.internal',
        port: 587,
        username: 'ops@example.com',
        password: '',
        from: 'pulse@example.com',
        // Raw lines as they would be while the user is mid-edit in the
        // textarea — whitespace, empties, trailing newline.
        to: ['alice@test.com', '', '  charlie@test.com  ', ''],
        tls: true,
        startTLS: true,
        replyTo: '',
        maxRetries: 3,
        retryDelay: 60,
        rateLimit: 0,
      }).to,
    ).toEqual(['alice@test.com', 'charlie@test.com']);
  });

  it('includes explicit email tag routing, including an empty filter used to clear routing', () => {
    expect(
      buildEmailConfigPayload({
        enabled: true,
        provider: 'smtp',
        server: 'smtp.internal',
        port: 587,
        username: 'ops@example.com',
        password: '',
        from: 'pulse@example.com',
        to: ['alerts@example.com'],
        tls: true,
        startTLS: true,
        replyTo: '',
        maxRetries: 3,
        retryDelay: 60,
        rateLimit: 0,
        tagFilter: [],
        tagFilterMode: 'any',
        minimumSeverity: 'critical',
      }),
    ).toEqual(
      expect.objectContaining({
        tagFilter: [],
        tagFilterMode: 'any',
        minimumSeverity: 'critical',
      }),
    );
  });

  it('builds outbound email and apprise payloads from UI state', () => {
    expect(
      buildEmailConfigPayload({
        enabled: true,
        provider: 'smtp',
        server: 'smtp.internal',
        port: 587,
        username: 'ops@example.com',
        password: '',
        from: 'pulse@example.com',
        to: ['alerts@example.com'],
        tls: true,
        startTLS: true,
        replyTo: '',
        maxRetries: 3,
        retryDelay: 60,
        rateLimit: 0,
        minimumSeverity: 'critical',
      }),
    ).toEqual(
      expect.objectContaining({
        server: 'smtp.internal',
        to: ['alerts@example.com'],
        minimumSeverity: 'critical',
      }),
    );

    expect(
      buildAppriseConfigPayload({
        enabled: true,
        mode: 'http',
        targetsText: 'https://notify.internal',
        cliPath: 'apprise',
        timeoutSeconds: 15,
        serverUrl: 'https://apprise.internal',
        configKey: '',
        apiKey: '',
        apiKeyHeader: 'X-API-KEY',
        skipTlsVerify: false,
        hasApiKey: false,
        minimumSeverity: 'critical',
      }),
    ).toEqual(
      expect.objectContaining({
        mode: 'http',
        targets: ['https://notify.internal'],
        serverUrl: 'https://apprise.internal',
        minimumSeverity: 'critical',
      }),
    );
  });
});
