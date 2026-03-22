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
      }),
    ).toEqual(
      expect.objectContaining({
        enabled: true,
        mode: 'http',
        targetsText: 'https://notify.example.test',
        serverUrl: 'https://apprise.example.test',
        timeoutSeconds: 30,
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
      }),
    ).toEqual(
      expect.objectContaining({
        server: 'smtp.internal',
        to: ['alerts@example.com'],
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
      }),
    ).toEqual(
      expect.objectContaining({
        mode: 'http',
        targets: ['https://notify.internal'],
        serverUrl: 'https://apprise.internal',
      }),
    );
  });
});
